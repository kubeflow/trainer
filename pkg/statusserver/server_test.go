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

package statusserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

type fakeAuthorizer struct {
	authorized bool
}

func (f fakeAuthorizer) Init(_ context.Context) error {
	return nil
}

func (f fakeAuthorizer) Authorize(_ context.Context, _, _, _ string) (bool, error) {
	return f.authorized, nil
}

func newTestServer(t *testing.T, cfg *configapi.StatusServer, authorizer TokenAuthorizer, objs ...client.Object) *httptest.Server {
	t.Helper()

	fakeClient := utiltesting.NewClientBuilder().
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()

	srv, err := NewServer(fakeClient, cfg, &tls.Config{}, authorizer)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	return httptest.NewServer(srv.httpServer.Handler)
}

func TestServerErrorResponses(t *testing.T) {
	cases := map[string]struct {
		url          string
		body         string
		authorized   bool
		wantResponse *metav1.Status
	}{
		"unauthorized fails with 403 unauthorized": {
			url:        "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			authorized: false,
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Forbidden",
				Reason:  metav1.StatusReasonForbidden,
				Code:    http.StatusForbidden,
			},
		},
		"invalid payload fails with 422 unprocessable entity": {
			url:        "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			body:       `invalid payload`,
			authorized: true,
			wantResponse: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Invalid payload",
				Reason:  metav1.StatusReasonInvalid,
				Code:    http.StatusUnprocessableEntity,
			},
		},
		"oversized payload fails with 413 payload too large error": {
			url: "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
			// Generate ~1MB payload (exceeds 64kB limit)
			body:       `{"trainerStatus": {"metrics": [` + strings.Repeat(`{"name":"m","value":"0.5"},`, 40000) + `]}}`,
			authorized: true,
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
			existingTrainJob := &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			ts := newTestServer(
				t,
				&configapi.StatusServer{Port: ptr.To[int32](8080)},
				fakeAuthorizer{authorized: tc.authorized},
				existingTrainJob,
			)
			defer ts.Close()

			// Make actual HTTP request
			req, err := http.NewRequest("POST", ts.URL+tc.url, bytes.NewReader([]byte(tc.body)))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("HTTP POST failed: %v", err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })

			if resp.StatusCode != int(tc.wantResponse.Code) {
				t.Errorf("status = %v, want %v", resp.StatusCode, tc.wantResponse.Code)
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

func TestServerCheck(t *testing.T) {
	srv, err := NewServer(
		nil,
		&configapi.StatusServer{Port: ptr.To[int32](8080)},
		&tls.Config{},
		fakeAuthorizer{},
	)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	// Before ready: Check should return error
	if err := srv.Check(nil); err == nil {
		t.Error("Check() = nil, want error before server is ready")
	}

	// After marking ready: Check should return nil
	srv.ready.Store(true)
	if err := srv.Check(nil); err != nil {
		t.Errorf("Check() = %v, want nil after server is ready", err)
	}

	// After marking not ready: Check should return error again
	srv.ready.Store(false)
	if err := srv.Check(nil); err == nil {
		t.Error("Check() = nil, want error after server marked not ready")
	}
}
