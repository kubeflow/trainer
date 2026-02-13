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

package progress

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

// newTestTLSConfig creates a TLS config with a self-signed certificate for testing.
func newTestTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Create TLS certificate
	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
}

// noopVerifier is a token verifier for testing.
type noopVerifier struct {
	validToken string
}

func (noopVerifier) Verify(context.Context, string) (*oidc.IDToken, error) {
	return nil, nil
}

func newTestServer(t *testing.T, cfg *configapi.ProgressServer, objs ...client.Object) *httptest.Server {
	t.Helper()

	fakeClient := utiltesting.NewClientBuilder().
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()

	srv, err := NewServer(fakeClient, cfg, newTestTLSConfig(t), noopVerifier{})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	return httptest.NewServer(srv.httpServer.Handler)
}

func TestHandleProgressStatus(t *testing.T) {
	validProgressStatus := trainer.ProgressStatus{
		TrainerStatus: &trainer.TrainJobTrainerStatus{
			ProgressPercentage:        ptr.To[int32](75),
			EstimatedRemainingSeconds: ptr.To[int32](300),
			Metrics: []trainer.Metric{
				{Name: "loss", Value: "0.123"},
				{Name: "accuracy", Value: "0.95"},
			},
		},
	}

	validBody, _ := json.Marshal(validProgressStatus)

	// TrainJob that exists in the cluster
	existingTrainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
	}

	cases := map[string]struct {
		url          string
		body         string
		wantResponse *trainer.ProgressStatus
	}{
		"successful POST request with full progress status": {
			url:          "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			body:         string(validBody),
			wantResponse: &validProgressStatus,
		},
		"successful POST request with empty progress status": {
			url:          "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			body:         "{}",
			wantResponse: &trainer.ProgressStatus{},
		},
		"successful POST request with only metrics": {
			url: "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			body: `{
				"trainerStatus": {
					"metrics": [{"name": "loss", "value": "0.5"}]
				}
			}`,
			wantResponse: &trainer.ProgressStatus{
				TrainerStatus: &trainer.TrainJobTrainerStatus{
					Metrics: []trainer.Metric{{Name: "loss", Value: "0.5"}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t, &configapi.ProgressServer{Port: ptr.To[int32](8080)}, existingTrainJob)
			defer ts.Close()

			// Make actual HTTP request
			req, err := http.NewRequest("POST", ts.URL+tc.url, bytes.NewReader([]byte(tc.body)))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("HTTP POST failed: %v", err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %v, want %v", resp.StatusCode, http.StatusOK)
			}

			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %v, want application/json", resp.Header.Get("Content-Type"))
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var got trainer.ProgressStatus
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if diff := cmp.Diff(tc.wantResponse, &got); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestServerErrorResponses(t *testing.T) {
	// TrainJob that exists in the cluster
	existingTrainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
	}

	cases := map[string]struct {
		url          string
		body         string
		authHeader   string
		wantResponse *metav1.Status
	}{
		"missing Authorization header fails with 401": {
			url:        "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			authHeader: "",
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Missing Authorization header",
				Reason:  metav1.StatusReasonUnauthorized,
				Code:    http.StatusUnauthorized,
			},
		},
		"invalid Authorization header format triggers unauthorized": {
			url:        "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			authHeader: "Basic dXNlcjpwYXNz",
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Invalid Authorization header format",
				Reason:  metav1.StatusReasonUnauthorized,
				Code:    http.StatusUnauthorized,
			},
		},
		"empty bearer token triggers unauthorized": {
			url:        "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			authHeader: "Bearer ",
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Invalid Authorization header format",
				Reason:  metav1.StatusReasonUnauthorized,
				Code:    http.StatusUnauthorized,
			},
		},
		"invalid JSON triggers invalid payload error": {
			url:        "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			body:       "{invalid json}",
			authHeader: "Bearer test-token",
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Invalid payload",
				Reason:  metav1.StatusReasonInvalid,
				Code:    http.StatusUnprocessableEntity,
			},
		},
		"malformed data triggers invalid payload error": {
			url:        "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			body:       "not json at all",
			authHeader: "Bearer test-token",
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Invalid payload",
				Reason:  metav1.StatusReasonInvalid,
				Code:    http.StatusUnprocessableEntity,
			},
		},
		"oversized body triggers payload too large error": {
			url: "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			// Generate ~1MB payload (exceeds 64kB limit)
			body:       `{"trainerStatus": {"metrics": [` + strings.Repeat(`{"name":"m","value":"0.5"},`, 40000) + `]}}`,
			authHeader: "Bearer test-token",
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Payload too large",
				Reason:  metav1.StatusReasonRequestEntityTooLarge,
				Code:    http.StatusRequestEntityTooLarge,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t, &configapi.ProgressServer{Port: ptr.To[int32](8080)}, existingTrainJob)
			defer ts.Close()

			// Make actual HTTP request
			req, err := http.NewRequest("POST", ts.URL+tc.url, bytes.NewReader([]byte(tc.body)))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", tc.authHeader)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("HTTP POST failed: %v", err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })

			if resp.StatusCode != int(tc.wantResponse.Code) {
				t.Errorf("status = %v, want %v", resp.StatusCode, tc.wantResponse.Code)
			}

			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %v, want application/json", resp.Header.Get("Content-Type"))
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var got metav1.Status
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if diff := cmp.Diff(tc.wantResponse, &got); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
