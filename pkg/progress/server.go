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
	"crypto/tls"
	"encoding/json"
	"errors"
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

// NewServer creates a new Server for collecting progress updates.
func NewServer(cfg *configapi.ProgressServer, tlsConfig *tls.Config) (*Server, error) {
	if cfg == nil || cfg.Port == nil {
		return nil, fmt.Errorf("cfg info is required")
	}
	if tlsConfig == nil {
		return nil, fmt.Errorf("tlsConfig is required")
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
		TLSConfig:    tlsConfig,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	s.httpServer = &httpServer

	return s, nil
}

// Start implements manager.Runnable and starts the HTTPS Server.
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

	s.log.Info("Starting progress Server with TLS", "address", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
