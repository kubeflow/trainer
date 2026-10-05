/*
Copyright The Kubeflow Authors.

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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/ktesting"
)

func TestRecoveryMiddleware(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with recovery middleware
	handler := recoveryMiddleware(logger)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic, should return 500
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %v, want %v", rec.Code, http.StatusInternalServerError)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := loggingMiddleware(logger)(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected inner handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rec.Code, http.StatusOK)
	}
}

func TestBodySizeLimitMiddleware(t *testing.T) {
	cases := map[string]struct {
		contentLength int64
		body          string
		maxBytes      int64
		wantStatus    int
		wantReason    metav1.StatusReason
	}{
		"rejects when Content-Length exceeds maxBytes": {
			contentLength: 20,
			body:          "01234567890123456789",
			maxBytes:      10,
			wantStatus:    http.StatusRequestEntityTooLarge,
			wantReason:    metav1.StatusReasonRequestEntityTooLarge,
		},
		"allows when Content-Length is within limit": {
			contentLength: 5,
			body:          "hello",
			maxBytes:      10,
			wantStatus:    http.StatusOK,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			logger, _ := ktesting.NewTestContext(t)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := bodySizeLimitMiddleware(logger, tc.maxBytes)(next)
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(tc.body))
			req.ContentLength = tc.contentLength
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %v, want %v", rec.Code, tc.wantStatus)
			}

			if tc.wantReason != "" {
				var status metav1.Status
				if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
					t.Fatalf("failed to unmarshal status: %v", err)
				}
				if status.Reason != tc.wantReason {
					t.Errorf("reason = %v, want %v", status.Reason, tc.wantReason)
				}
			}
		})
	}
}

func TestBodySizeLimitMiddlewareStreaming(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	readErr := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			readErr = true
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := bodySizeLimitMiddleware(logger, 5)(next)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString("longer-than-five-bytes"))
	req.ContentLength = -1 // streaming or chunked without Content-Length
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !readErr {
		t.Error("expected reading oversized streaming body to produce an error")
	}
}

func TestChain(t *testing.T) {
	var trace []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trace = append(trace, "mw1-pre")
			next.ServeHTTP(w, r)
			trace = append(trace, "mw1-post")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trace = append(trace, "mw2-pre")
			next.ServeHTTP(w, r)
			trace = append(trace, "mw2-post")
		})
	}

	endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace = append(trace, "endpoint")
		w.WriteHeader(http.StatusOK)
	})

	chained := chain(endpoint, mw1, mw2)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	chained.ServeHTTP(rec, req)

	wantTrace := []string{"mw1-pre", "mw2-pre", "endpoint", "mw2-post", "mw1-post"}
	if diff := cmp.Diff(wantTrace, trace); diff != "" {
		t.Errorf("chain execution order mismatch (-want,+got):\n%s", diff)
	}
}
