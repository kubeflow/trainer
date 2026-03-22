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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/statusserver"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

type mockAuthorizer struct{}

func (m *mockAuthorizer) Init(_ context.Context) error { return nil }
func (m *mockAuthorizer) Authorize(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

func generateTestTLSConfig() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-status-server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	)
	if err != nil {
		return nil, fmt.Errorf("build TLS certificate: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

// findFreePort asks the OS for an available TCP port on loopback.
func findFreePort() (int32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return int32(l.Addr().(*net.TCPAddr).Port), nil
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

		port, err := findFreePort()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		server, err := statusserver.NewServer(
			k8sClient,
			&configapi.StatusServer{Port: ptr.To(port)},
			tlsConfig,
			&mockAuthorizer{},
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		var serverCtx context.Context
		serverCtx, serverCancel = context.WithCancel(ctx)
		go func() {
			defer ginkgo.GinkgoRecover()
			_ = server.Start(serverCtx)
		}()

		serverAddr = fmt.Sprintf("https://127.0.0.1:%d", port)
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test only
			},
		}

		gomega.Eventually(func(g gomega.Gomega) {
			resp, err := httpClient.Get(serverAddr + "/")
			g.Expect(err).NotTo(gomega.HaveOccurred())
			defer resp.Body.Close()
		}, 5*time.Second, 100*time.Millisecond).Should(gomega.Succeed())

		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-statusserver-"},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		serverCancel()
		gomega.Expect(k8sClient.Delete(context.Background(), ns)).To(gomega.Succeed())
	})


	ginkgo.It("should update the TrainJob status when the request is valid", func() {
		const jobName = "valid-trainjob"
		lastUpdatedTime := metav1.Now()

		trainingRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, jobName).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())

		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, jobName).
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), jobName).
			Obj()
		gomega.Expect(k8sClient.Create(ctx, trainJob)).To(gomega.Succeed())

		resp := postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{
			TrainerStatus: &trainer.TrainerStatus{
				ProgressPercentage: ptr.To(int32(50)),
				LastUpdatedTime:    lastUpdatedTime,
			},
		})
		defer resp.Body.Close()

		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

		gomega.Eventually(func(g gomega.Gomega) {
			updated := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: ns.Name,
			}, updated)).To(gomega.Succeed())
			g.Expect(updated.Status.TrainerStatus).NotTo(gomega.BeNil())
			g.Expect(updated.Status.TrainerStatus.ProgressPercentage).NotTo(gomega.BeNil())
			g.Expect(*updated.Status.TrainerStatus.ProgressPercentage).To(gomega.Equal(int32(50)))
		}, 5*time.Second, 100*time.Millisecond).Should(gomega.Succeed())
	})


	ginkgo.It("should reject a request whose progressPercentage exceeds 100 with a useful error message", func() {
		const jobName = "invalid-progress-trainjob"
		lastUpdatedTime := metav1.Now()

		trainingRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, jobName).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())

		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, jobName).
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), jobName).
			Obj()
		gomega.Expect(k8sClient.Create(ctx, trainJob)).To(gomega.Succeed())

		resp := postStatus(httpClient, serverAddr, ns.Name, jobName, trainer.UpdateTrainJobStatusRequest{
			TrainerStatus: &trainer.TrainerStatus{
				ProgressPercentage: ptr.To(int32(150)), // invalid: > 100
				LastUpdatedTime:    lastUpdatedTime,
			},
		})
		defer resp.Body.Close()

		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusUnprocessableEntity))

		var status metav1.Status
		gomega.Expect(json.NewDecoder(resp.Body).Decode(&status)).To(gomega.Succeed())
		gomega.Expect(status.Message).NotTo(gomega.BeEmpty(),
			"response body should contain a human-readable error message")
	})


	ginkgo.It("should reject an update to a non-existing TrainJob with a useful error message", func() {
		lastUpdatedTime := metav1.Now()

		resp := postStatus(httpClient, serverAddr, ns.Name, "does-not-exist",
			trainer.UpdateTrainJobStatusRequest{
				TrainerStatus: &trainer.TrainerStatus{
					ProgressPercentage: ptr.To(int32(50)),
					LastUpdatedTime:    lastUpdatedTime,
				},
			},
		)
		defer resp.Body.Close()

		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusNotFound))

		var status metav1.Status
		gomega.Expect(json.NewDecoder(resp.Body).Decode(&status)).To(gomega.Succeed())
		gomega.Expect(status.Message).NotTo(gomega.BeEmpty(),
			"response body should contain a human-readable error message")
	})
})