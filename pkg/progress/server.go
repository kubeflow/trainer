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
	"log/slog"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
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

// Server for collecting progress status updates.
type Server struct {
	log        logr.Logger
	httpServer *http.Server
	client     client.Client
}

// NewServer creates a new Server for collecting progress updates.
func NewServer(c client.Client, cfg *configapi.ProgressServer, tlsConfig *tls.Config, verifier TokenVerifier) (*Server, error) {
	if cfg == nil || cfg.Port == nil {
		return nil, fmt.Errorf("cfg info is required")
	}
	if tlsConfig == nil {
		return nil, fmt.Errorf("tlsConfig is required")
	}
	if verifier == nil {
		return nil, fmt.Errorf("verifier is required")
	}

	log := ctrl.Log.WithName("progress")

	s := &Server{
		log:    log,
		client: c,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+statusUrl("{namespace}", "{name}"), s.handleProgressStatus)
	mux.HandleFunc("/", s.handleDefault)

	// Apply middleware
	handler := chain(mux,
		recoveryMiddleware(log),
		loggingMiddleware(log),
		authenticationMiddleware(log, verifier),
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
	trainJobName := r.PathValue("name")

	if !s.authoriseRequest(r, namespace, trainJobName) {
		badRequest(w, s.log, "Forbidden", metav1.StatusReasonForbidden, http.StatusForbidden)
		return
	}

	// Parse request body
	var progressStatus trainer.ProgressStatus
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&progressStatus); err != nil {
		s.log.V(5).Error(err, "Failed to parse progress status", "namespace", namespace, "trainJobName", trainJobName)
		badRequest(w, s.log, "Invalid payload", metav1.StatusReasonInvalid, http.StatusUnprocessableEntity)
		return
	}

	var trainJob = trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trainJobName,
			Namespace: namespace,
		},
		Status: trainer.TrainJobStatus{
			TrainerStatus: progressStatus.TrainerStatus,
		},
	}
	if err := s.client.Status().Patch(r.Context(), &trainJob, client.Merge); err != nil {
		s.log.Error(err, "Failed to update train job", "namespace", namespace, "name", trainJobName)

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
	if err := json.NewEncoder(w).Encode(progressStatus); err != nil {
		s.log.Error(err, "Failed to write progress status", "namespace", namespace, "trainJobName", trainJobName)
	}
}

// handleDefault is the default handler for unknown requests.
func (s *Server) handleDefault(w http.ResponseWriter, _ *http.Request) {
	badRequest(w, s.log, "Not found", metav1.StatusReasonNotFound, http.StatusNotFound)
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get

// authoriseRequest checks whether the service account token bearer token used by this request comes from
// a pod that is part of the TrainJob that is being updated.
func (s *Server) authoriseRequest(r *http.Request, namespace, trainJobName string) bool {
	token, ok := serviceAccountTokenFromContext(r.Context())
	if !ok {
		s.log.V(5).Info("Unauthorized request", "namespace", namespace, "trainJob", trainJobName)
		return false
	}

	// Check namespace matches
	if token.Kubernetes.Namespace != namespace {
		s.log.V(5).Info("Namespace mismatch",
			"expected", namespace,
			"got", token.Kubernetes.Namespace)
		return false
	}

	// Check token is bound to a pod
	if token.Kubernetes.Pod == nil {
		s.log.V(5).Info("Token not bound to a pod")
		return false
	}

	// Look up the pod from the Kubernetes API to verify it has the correct label
	pod := &corev1.Pod{}
	podKey := client.ObjectKey{
		Namespace: token.Kubernetes.Namespace,
		Name:      token.Kubernetes.Pod.Name,
	}

	if err := s.client.Get(r.Context(), podKey, pod); err != nil {
		s.log.V(5).Error(err, "Failed to get pod",
			"namespace", podKey.Namespace,
			"pod", podKey.Name)
		return false
	}

	if token.Kubernetes.Pod.UID != pod.UID {
		s.log.V(5).Info("Pod UID does not match",
			"namespace", pod.Namespace,
			"pod", podKey.Name)
		return false
	}

	// Verify the pod has the label identifying it belongs to this TrainJob
	trainJobNameFromLabel, ok := pod.Labels[LabelTrainJobName]
	if !ok {
		s.log.V(5).Info("Pod missing TrainJob label",
			"namespace", pod.Namespace, "pod", pod.Name)
		return false
	}

	if trainJobName != trainJobNameFromLabel {
		s.log.V(5).Info("Pod TrainJob label does not match",
			"expected", trainJobName,
			"got", trainJobName,
			"namespace", pod.Namespace,
			"pod", pod.Name)
		return false
	}

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
