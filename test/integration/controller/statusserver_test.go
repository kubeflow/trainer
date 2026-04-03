/*
Copyright 2026 The Kubeflow Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/statusserver"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

type mockAuthorizer struct{}

func (m *mockAuthorizer) Init(_ context.Context) error { return nil }
func (m *mockAuthorizer) Authorize(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

// findFreePort asks the OS for an available TCP port.
// The returned port is not held open, so it may be stolen by another process
// before the server binds — callers should retry on failure (see BeforeEach).
func findFreePort() (int32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find free port: %w", err)
	}
	port := int32(l.Addr().(*net.TCPAddr).Port)
	_ = l.Close()
	return port, nil
}

// generateTestTLSConfig generates a fresh self-signed localhost certificate
// and returns a *tls.Config that uses it.  Generating at runtime avoids
// shipping checked-in key material and keeps the cert valid for the duration
// of the test run.
func generateTestTLSConfig() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("key pair: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

// postStatus is a small helper that marshals body and POSTs it to the handler
// URL for the given namespace/name pair.
func postStatus(httpClient *http.Client, serverAddr, namespace, name string, body any) *http.Response {
	ginkgo.GinkgoHelper()
	raw, err := json.Marshal(body)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	url := serverAddr + statusserver.StatusUrl(namespace, name)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(raw))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return resp
}

var _ = ginkgo.Describe("StatusServer", ginkgo.Ordered, func() {
	var (
		serverCancel context.CancelFunc
		serverErr    chan error
		httpClient   *http.Client
		serverAddr   string
		ns           *corev1.Namespace
	)

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{}
		cfg = fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg, true)
	})

	ginkgo.AfterAll(func() {
		fwk.Teardown()
	})

	ginkgo.BeforeEach(func() {
		tlsConfig, err := generateTestTLSConfig()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Retry both port selection and server construction together so that a
		// port stolen between findFreePort and NewServer.Start causes a new
		// attempt rather than a flaky timeout (TOCTOU mitigation).
		const maxAttempts = 5
		var (
			server  *statusserver.Server
			port    int32
			lastErr error
		)
		for attempt := 0; attempt < maxAttempts; attempt++ {
			port, lastErr = findFreePort()
			if lastErr != nil {
				continue
			}
			server, lastErr = statusserver.NewServer(
				k8sClient,
				&configapi.StatusServer{Port: ptr.To(port)},
				tlsConfig,
				&mockAuthorizer{},
			)
			if lastErr == nil {
				break
			}
		}
		gomega.Expect(lastErr).NotTo(gomega.HaveOccurred(),
			"failed to create status server after %d attempts", maxAttempts)

		var serverCtx context.Context
		serverCtx, serverCancel = context.WithCancel(ctx)
		serverErr = make(chan error, 1)
		go func() {
			defer ginkgo.GinkgoRecover()
			serverErr <- server.Start(serverCtx)
		}()

		serverAddr = fmt.Sprintf("https://127.0.0.1:%d", port)
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test only
			},
		}

		gomega.Eventually(func(g gomega.Gomega) {
			resp, err := httpClient.Get(serverAddr + "/")
			g.Expect(err).NotTo(gomega.HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
		}, util.Timeout, util.Interval).Should(gomega.Succeed())

		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-statusserver-"},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		serverCancel()
		// Drain server error — startup/TLS failures surface here.
		select {
		case err := <-serverErr:
			if err != nil {
				ginkgo.GinkgoT().Logf("status server exited with error: %v", err)
			}
		default:
		}
		gomega.Expect(k8sClient.Delete(context.Background(), ns)).To(gomega.Succeed())
	})

	ginkgo.It("Should update the TrainJob status when the request is valid", func() {
		const jobName = "valid-trainjob"

		ginkgo.By("Creating TrainingRuntime and TrainJob")
		trainingRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, jobName).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())

		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, jobName).
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), jobName).
			Obj()
		gomega.Expect(k8sClient.Create(ctx, trainJob)).To(gomega.Succeed())

		ginkgo.By("POSTing a valid status update")
		resp := postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{
			TrainerStatus: &trainer.TrainerStatus{
				ProgressPercentage: ptr.To(int32(50)),
				LastUpdatedTime:    metav1.Now(),
			},
		})
		defer func() { _ = resp.Body.Close() }()
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

		ginkgo.By("Checking that the status is persisted on the TrainJob")
		gomega.Eventually(func(g gomega.Gomega) {
			updated := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: ns.Name,
			}, updated)).To(gomega.Succeed())
			g.Expect(updated.Status.TrainerStatus).NotTo(gomega.BeNil())
			g.Expect(updated.Status.TrainerStatus.ProgressPercentage).NotTo(gomega.BeNil())
			g.Expect(*updated.Status.TrainerStatus.ProgressPercentage).To(gomega.Equal(int32(50)))
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
	})

	ginkgo.It("Should overwrite existing status, not merge, when posting a new update", func() {
		const jobName = "overwrite-trainjob"

		ginkgo.By("Creating TrainingRuntime and TrainJob")
		trainingRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, jobName).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())

		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, jobName).
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), jobName).
			Obj()
		gomega.Expect(k8sClient.Create(ctx, trainJob)).To(gomega.Succeed())

		ginkgo.By("POSTing initial status with progress 30%")
		resp := postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{
			TrainerStatus: &trainer.TrainerStatus{
				ProgressPercentage: ptr.To(int32(30)),
				LastUpdatedTime:    metav1.Now(),
			},
		})
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))
		_ = resp.Body.Close()

		ginkgo.By("Waiting for initial status to be persisted")
		gomega.Eventually(func(g gomega.Gomega) {
			updated := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: ns.Name,
			}, updated)).To(gomega.Succeed())
			g.Expect(updated.Status.TrainerStatus).NotTo(gomega.BeNil())
			g.Expect(updated.Status.TrainerStatus.ProgressPercentage).NotTo(gomega.BeNil())
			g.Expect(*updated.Status.TrainerStatus.ProgressPercentage).To(gomega.Equal(int32(30)))
		}, util.Timeout, util.Interval).Should(gomega.Succeed())

		// Post a second update that deliberately omits ProgressPercentage.
		// If the server merges instead of replacing, the old value (30) would
		// survive — this verifies it does not.
		ginkgo.By("POSTing a second update that omits ProgressPercentage")
		resp = postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{
			TrainerStatus: &trainer.TrainerStatus{
				LastUpdatedTime: metav1.Now(),
				// ProgressPercentage intentionally absent
			},
		})
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))
		_ = resp.Body.Close()

		ginkgo.By("Verifying ProgressPercentage was removed (status replaced, not merged)")
		gomega.Eventually(func(g gomega.Gomega) {
			updated := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: ns.Name,
			}, updated)).To(gomega.Succeed())
			g.Expect(updated.Status.TrainerStatus).NotTo(gomega.BeNil())
			g.Expect(updated.Status.TrainerStatus.ProgressPercentage).To(gomega.BeNil(),
				"ProgressPercentage should be removed when absent from the new update")
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
	})

	ginkgo.It("Should accept an empty update request but not overwrite existing status", func() {
		const jobName = "empty-update-trainjob"

		ginkgo.By("Creating TrainingRuntime and TrainJob")
		trainingRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, jobName).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())

		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, jobName).
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), jobName).
			Obj()
		gomega.Expect(k8sClient.Create(ctx, trainJob)).To(gomega.Succeed())

		ginkgo.By("POSTing initial status with progress 40%")
		resp := postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{
			TrainerStatus: &trainer.TrainerStatus{
				ProgressPercentage: ptr.To(int32(40)),
				LastUpdatedTime:    metav1.Now(),
			},
		})
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))
		_ = resp.Body.Close()

		ginkgo.By("Waiting for initial status to be persisted")
		gomega.Eventually(func(g gomega.Gomega) {
			updated := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: ns.Name,
			}, updated)).To(gomega.Succeed())
			g.Expect(updated.Status.TrainerStatus).NotTo(gomega.BeNil())
			g.Expect(updated.Status.TrainerStatus.ProgressPercentage).NotTo(gomega.BeNil())
			g.Expect(*updated.Status.TrainerStatus.ProgressPercentage).To(gomega.Equal(int32(40)))
		}, util.Timeout, util.Interval).Should(gomega.Succeed())

		ginkgo.By("POSTing an empty update request")
		resp = postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{})
		defer func() { _ = resp.Body.Close() }()
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

		ginkgo.By("Verifying the existing status was not removed")
		// Use Consistently to verify status remains unchanged over a short period
		gomega.Consistently(func(g gomega.Gomega) {
			updated := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: ns.Name,
			}, updated)).To(gomega.Succeed())
			g.Expect(updated.Status.TrainerStatus).NotTo(gomega.BeNil())
			g.Expect(updated.Status.TrainerStatus.ProgressPercentage).NotTo(gomega.BeNil())
			g.Expect(*updated.Status.TrainerStatus.ProgressPercentage).To(gomega.Equal(int32(40)),
				"empty update should not remove existing status")
		}, 2*time.Second, 500*time.Millisecond).Should(gomega.Succeed())
	})

	ginkgo.It("Should reject a request with progressPercentage > 100 with a useful error message", func() {
		const jobName = "invalid-progress-trainjob"

		ginkgo.By("Creating TrainingRuntime and TrainJob")
		trainingRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, jobName).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())

		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, jobName).
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), jobName).
			Obj()
		gomega.Expect(k8sClient.Create(ctx, trainJob)).To(gomega.Succeed())

		ginkgo.By("POSTing a status update with progressPercentage > 100")
		resp := postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{
			TrainerStatus: &trainer.TrainerStatus{
				ProgressPercentage: ptr.To(int32(150)), // invalid: > 100
				LastUpdatedTime:    metav1.Now(),
			},
		})
		defer func() { _ = resp.Body.Close() }()

		// Verify the response body carries a well-formed Status object with all
		// expected fields populated.
		ginkgo.By("Checking the response is 422 with a correctly populated Status object")
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusUnprocessableEntity))
		var apiStatus metav1.Status
		gomega.Expect(json.NewDecoder(resp.Body).Decode(&apiStatus)).To(gomega.Succeed())
		gomega.Expect(apiStatus.Code).To(gomega.Equal(int32(http.StatusUnprocessableEntity)),
			"Status.Code should match the HTTP status")
		gomega.Expect(apiStatus.Reason).To(gomega.Equal(metav1.StatusReasonInvalid),
			"Status.Reason should be Invalid")
		// Message should name the field, the submitted value, and the maximum.
		gomega.Expect(apiStatus.Message).To(gomega.ContainSubstring("progressPercentage"),
			"error message should identify the invalid field")
		gomega.Expect(apiStatus.Message).To(gomega.ContainSubstring("150"),
			"error message should mention the submitted value")
		gomega.Expect(apiStatus.Message).To(gomega.ContainSubstring("100"),
			"error message should mention the maximum allowed value")

		// Verify the TrainJob status was not mutated by the rejected request.
		ginkgo.By("Verifying the TrainJob status was not updated after the rejected request")
		gomega.Consistently(func(g gomega.Gomega) {
			updated := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: ns.Name,
			}, updated)).To(gomega.Succeed())
			g.Expect(updated.Status.TrainerStatus).To(gomega.BeNil(),
				"an invalid request must not mutate the TrainJob status")
		}, 2*time.Second, 500*time.Millisecond).Should(gomega.Succeed())
	})

	ginkgo.It("Should reject an update to a non-existing TrainJob with a useful error message", func() {
		const jobName = "does-not-exist"

		ginkgo.By("POSTing a status update for a non-existing TrainJob")
		resp := postStatus(httpClient, serverAddr, ns.Name, jobName,
			trainer.UpdateTrainJobStatusRequest{
				TrainerStatus: &trainer.TrainerStatus{
					ProgressPercentage: ptr.To(int32(50)),
					LastUpdatedTime:    metav1.Now(),
				},
			},
		)
		defer func() { _ = resp.Body.Close() }()

		ginkgo.By("Checking the response is 404 with a train job not found message")
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusNotFound))
		var apiStatus metav1.Status
		gomega.Expect(json.NewDecoder(resp.Body).Decode(&apiStatus)).To(gomega.Succeed())
		gomega.Expect(apiStatus.Message).To(gomega.Equal("Train job not found"))

		// Confirm the TrainJob truly does not exist — the rejected request must
		// not have created it as a side-effect.
		ginkgo.By("Verifying the TrainJob does not exist in the cluster")
		notFound := &trainer.TrainJob{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      jobName,
			Namespace: ns.Name,
		}, notFound)
		gomega.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue(),
			"the TrainJob should not exist; a rejected request must not create it")
	})
})
