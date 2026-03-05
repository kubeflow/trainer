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

package status

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	trainerv1alpha1ac "github.com/kubeflow/trainer/v2/pkg/client/applyconfiguration/trainer/v1alpha1"
)

const (
	shutdownTimeout = 5 * time.Second

	// HTTP Server timeouts to prevent resource exhaustion
	readTimeout  = 10 * time.Second
	writeTimeout = 10 * time.Second
	idleTimeout  = 120 * time.Second

	// Maximum request body size (64kB)
	maxBodySize = 1 << 16
)

// Server for collecting runtime status updates.
type Server struct {
	log          logr.Logger
	httpServer   *http.Server
	client       client.Client
	oidcProvider *oidc.Provider
}

var (
	_ manager.Runnable               = &Server{}
	_ manager.LeaderElectionRunnable = &Server{}
)

// NewServer creates a new Server for collecting runtime status updates.
// oidcProvider may be nil for testing purposes, but will panic if authorization is attempted.
func NewServer(c client.Client, cfg *configapi.TrainJobStatusServer, tlsConfig *tls.Config, oidcProvider *oidc.Provider) (*Server, error) {
	if cfg == nil || cfg.Port == nil {
		return nil, fmt.Errorf("cfg info is required")
	}
	if tlsConfig == nil {
		return nil, fmt.Errorf("tlsConfig is required")
	}

	log := ctrl.Log.WithName("runtime-status")

	s := &Server{
		log:          log,
		client:       c,
		oidcProvider: oidcProvider,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+StatusUrl("{namespace}", "{name}"), s.handleTrainJobRuntimeStatus)
	mux.HandleFunc("/", s.handleDefault)

	// Apply middleware (authentication happens in handler)
	handler := chain(mux,
		recoveryMiddleware(log),
		loggingMiddleware(log),
		bodySizeLimitMiddleware(log, maxBodySize),
	)

	httpServer := http.Server{
		Addr:         fmt.Sprintf(":%d", *cfg.Port),
		Handler:      handler,
		TLSConfig:    tlsConfig,
		ErrorLog:     slog.NewLogLogger(logr.ToSlogHandler(log), slog.LevelInfo),
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
		s.log.Info("Shutting down runtime status server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.log.Error(err, "Error shutting down runtime status server")
		}
	}()

	s.log.Info("Starting runtime status server with TLS", "address", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("runtime status server failed: %w", err)
	}
	return nil
}

func (s *Server) NeedLeaderElection() bool {
	// server needs to run on all replicas
	return false
}

// handleTrainJobRuntimeStatus handles POST requests to update TrainJob status.
// Expected URL format: /apis/trainer.kubeflow.org/v1alpha1/namespaces/{namespace}/trainjobs/{name}/status
func (s *Server) handleTrainJobRuntimeStatus(w http.ResponseWriter, r *http.Request) {

	namespace := r.PathValue("namespace")
	trainJobName := r.PathValue("name")

	if !s.authorizeRequest(r, namespace, trainJobName) {
		badRequest(w, s.log, "Forbidden", metav1.StatusReasonForbidden, http.StatusForbidden)
		return
	}

	// Parse request body
	var runtimeStatus trainer.UpdateTrainJobStatusRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&runtimeStatus); err != nil {
		s.log.V(5).Error(err, "Failed to parse runtime status", "namespace", namespace, "trainJobName", trainJobName)
		badRequest(w, s.log, "Invalid payload", metav1.StatusReasonInvalid, http.StatusUnprocessableEntity)
		return
	}

	var trainJob = trainerv1alpha1ac.TrainJob(trainJobName, namespace).
		WithStatus(trainerv1alpha1ac.TrainJobStatus().
			WithTrainerStatus(toApplyConfig(runtimeStatus.TrainerStatus)),
		)

	if err := s.client.Status().Apply(r.Context(), trainJob, client.ForceOwnership, client.FieldOwner("trainer-status")); err != nil {
		s.log.Error(err, "Failed to update TrainJob", "namespace", namespace, "name", trainJobName)

		// Check if the error is due to validation failure
		if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
			// Extract the validation error message for the user
			statusErr, ok := err.(*apierrors.StatusError)
			if ok && statusErr.ErrStatus.Message != "" {
				badRequest(w, s.log, statusErr.ErrStatus.Message, metav1.StatusReasonInvalid, http.StatusUnprocessableEntity)
			} else {
				badRequest(w, s.log, "Validation failed: "+err.Error(), metav1.StatusReasonInvalid, http.StatusUnprocessableEntity)
			}
			return
		}

		// For other errors, return internal server error
		badRequest(w, s.log, "Internal error", metav1.StatusReasonInternalError, http.StatusInternalServerError)
		return
	}

	// Return the parsed payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(runtimeStatus); err != nil {
		s.log.Error(err, "Failed to write TrainJob status", "namespace", namespace, "name", trainJobName)
	}
}

// handleDefault is the default handler for unknown requests.
func (s *Server) handleDefault(w http.ResponseWriter, _ *http.Request) {
	badRequest(w, s.log, "Not found", metav1.StatusReasonNotFound, http.StatusNotFound)
}

// authorizeRequest verifies the bearer token has the correct audience for the TrainJob.
// Authorization is based on token audience matching the TrainJob-specific endpoint.
func (s *Server) authorizeRequest(r *http.Request, namespace, trainJobName string) bool {
	// Extract bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		s.log.V(5).Info("Missing Authorization header", "namespace", namespace, "trainJob", trainJobName)
		return false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		s.log.V(5).Info("Invalid Authorization header format", "namespace", namespace, "trainJob", trainJobName)
		return false
	}

	rawToken := parts[1]
	if rawToken == "" {
		s.log.V(5).Info("Empty bearer token", "namespace", namespace, "trainJob", trainJobName)
		return false
	}

	// Create verifier with TrainJob-specific audience
	expectedAudience := TokenAudience(namespace, trainJobName)
	verifier := s.oidcProvider.Verifier(&oidc.Config{
		ClientID: expectedAudience,
	})

	// Verify token signature, expiry, and audience
	idToken, err := verifier.Verify(r.Context(), rawToken)
	if err != nil {
		s.log.V(5).Error(err, "Token verification failed",
			"namespace", namespace,
			"trainJob", trainJobName,
			"expectedAudience", expectedAudience)
		return false
	}

	// Log successful authentication for observability
	s.log.V(3).Info("Authenticated request",
		"subject", idToken.Subject,
		"namespace", namespace,
		"trainJob", trainJobName)

	return true
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

func toApplyConfig(trainerStatus *trainer.TrainerStatus) *trainerv1alpha1ac.TrainerStatusApplyConfiguration {
	var status = trainerv1alpha1ac.TrainerStatus()
	if trainerStatus.ProgressPercentage != nil {
		status = status.WithProgressPercentage(*trainerStatus.ProgressPercentage)
	}
	if trainerStatus.EstimatedRemainingSeconds != nil {
		status = status.WithEstimatedRemainingSeconds(*trainerStatus.EstimatedRemainingSeconds)
	}
	for _, m := range trainerStatus.Metrics {
		status.WithMetrics(
			trainerv1alpha1ac.Metric().
				WithName(m.Name).
				WithValue(m.Value),
		)
	}

	status = status.WithLastUpdatedTime(trainerStatus.LastUpdatedTime)

	return status
}
