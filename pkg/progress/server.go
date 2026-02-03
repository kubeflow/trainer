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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

const (
	shutdownTimeout = 5 * time.Second
	progressURL     = "POST /apis/trainer.kubeflow.org/v1alpha1/namespaces/{namespace}/trainjobs/{name}/status"

	// HTTP Server timeouts to prevent resource exhaustion
	readTimeout  = 10 * time.Second
	writeTimeout = 10 * time.Second
	idleTimeout  = 120 * time.Second

	// Maximum request body size (64kB)
	maxBodySize = 1 << 16
)

// Server for collecting progress status updates.
type Server struct {
	log        logr.Logger
	httpServer *http.Server
}

type Middleware func(http.Handler) http.Handler

// chain applies middleware in order: first middleware wraps second, etc.
func chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// recoveryMiddleware recovers from panics in HTTP handlers to prevent Server crashes.
func recoveryMiddleware(log logr.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error(fmt.Errorf("panic: %v", err), "Panic in HTTP handler",
						"path", r.URL.Path, "method", r.Method)
					badRequest(w, log, "Internal Server Error", metav1.StatusReasonInternalError, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware logs incoming HTTP requests.
func loggingMiddleware(log logr.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.V(5).Info("HTTP request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}

// bodySizeLimitMiddleware enforces a maximum request body size.
func bodySizeLimitMiddleware(log logr.Logger, maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reject based on Content-Length header if present
			if r.ContentLength > maxBytes {
				badRequest(w, log, "Payload too large",
					metav1.StatusReasonRequestEntityTooLarge,
					http.StatusRequestEntityTooLarge)
				return
			}

			// Wrap body to enforce limit for chunked/streaming requests
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// NewServer creates a new Server for collecting progress updates.
func NewServer(cfg *configapi.ProgressServer) (*Server, error) {
	if cfg == nil || cfg.Port == nil {
		return nil, fmt.Errorf("cfg info is missing")
	}

	log := ctrl.Log.WithName("progress")

	s := &Server{
		log: log,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(progressURL, s.handleProgressStatus)
	mux.HandleFunc("/", s.handleDefault)

	// Apply middleware (order: recovery -> body size limit -> logging -> handlers)
	handler := chain(mux,
		recoveryMiddleware(log),
		loggingMiddleware(log),
		bodySizeLimitMiddleware(log, maxBodySize),
	)

	httpServer := http.Server{
		Addr:         fmt.Sprintf(":%d", *cfg.Port),
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	s.httpServer = &httpServer

	return s, nil
}

// Start implements manager.Runnable and starts the HTTP Server.
// It blocks until the Server stops, either due to an error or graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	// Handle graceful shutdown in background
	go func() {
		<-ctx.Done()
		s.log.Info("Shutting down progress Server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.log.Error(err, "Error shutting down progress Server")
		}
	}()

	s.log.Info("Starting progress Server", "address", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("progress Server failed: %w", err)
	}
	return nil
}

// handleProgressStatus handles POST requests to update TrainJob progress status.
// Expected URL format: /apis/trainer.kubeflow.org/v1alpha1/namespaces/{namespace}/trainjobs/{name}/status
func (s *Server) handleProgressStatus(w http.ResponseWriter, r *http.Request) {

	namespace := r.PathValue("namespace")
	name := r.PathValue("name")

	// Parse request body
	var progressStatus trainer.ProgressStatus
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&progressStatus); err != nil {
		s.log.V(5).Error(err, "Failed to parse progress status", "namespace", namespace, "name", name)
		badRequest(w, s.log, "Invalid payload", metav1.StatusReasonInvalid, http.StatusUnprocessableEntity)
		return
	}

	// PLACEHOLDER: Log the received progress status
	s.log.Info("Handled progress status update", "namespace", namespace, "name", name, "status", progressStatus)

	// Return the parsed payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(progressStatus); err != nil {
		s.log.Error(err, "Failed to write progress status", "namespace", namespace, "name", name)
	}
}

// handleDefault is the default handler for unknown requests.
func (s *Server) handleDefault(w http.ResponseWriter, _ *http.Request) {
	badRequest(w, s.log, "Not found", metav1.StatusReasonNotFound, http.StatusNotFound)
}

// badRequest sends a kubernetes Status response with the error message
func badRequest(w http.ResponseWriter, log logr.Logger, message string, reason metav1.StatusReason, code int32) {
	status := metav1.Status{
		Status:  metav1.StatusFailure,
		Message: message,
		Reason:  reason,
		Code:    code,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(int(code))
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error(err, "Failed to write bad request details")
	}
}
