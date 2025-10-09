/*
Copyright 2025 The Kubeflow Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +k8s:defaulter-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// Configuration is the Schema for the Kubeflow Trainer controller manager configuration.
type Configuration struct {
	metav1.TypeMeta `json:",inline"`

	// Webhook contains the controllers webhook configuration.
	// +optional
	Webhook ControllerWebhook `json:"webhook,omitempty"`

	// LeaderElection is the LeaderElection config to be used when configuring
	// the manager.Manager leader election.
	// +optional
	LeaderElection *configv1alpha1.LeaderElectionConfiguration `json:"leaderElection,omitempty"`

	// Metrics contains the controller metrics configuration.
	// +optional
	Metrics ControllerMetrics `json:"metrics,omitempty"`

	// Health contains the controller health configuration.
	// +optional
	Health ControllerHealth `json:"health,omitempty"`

	// Controller contains global configuration options for controllers
	// registered within this manager.
	// +optional
	Controller *ControllerConfigurationSpec `json:"controller,omitempty"`

	// CertManagement is configuration for certificate management used by the webhook server.
	// +optional
	CertManagement *CertManagement `json:"certManagement,omitempty"`

	// ClientConnection provides additional configuration options for Kubernetes
	// API server client.
	// +optional
	ClientConnection *ClientConnection `json:"clientConnection,omitempty"`
}

// ControllerWebhook defines the webhook server for the controller.
type ControllerWebhook struct {
	// Port is the port that the webhook server serves at.
	// It is used to set webhook.Server.Port.
	// Defaults to 9443.
	// +optional
	// +kubebuilder:default=9443
	Port *int `json:"port,omitempty"`

	// Host is the hostname that the webhook server binds to.
	// It is used to set webhook.Server.Host.
	// Defaults to "" (all interfaces).
	// +optional
	Host string `json:"host,omitempty"`
}

// ControllerMetrics defines the metrics configs.
type ControllerMetrics struct {
	// BindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8443".
	// +optional
	// +kubebuilder:default=":8443"
	BindAddress string `json:"bindAddress,omitempty"`

	// SecureServing determines if the metrics endpoint should be served securely via HTTPS.
	// Defaults to true.
	// +optional
	// +kubebuilder:default=true
	SecureServing *bool `json:"secureServing,omitempty"`
}

// ControllerHealth defines the health configs.
type ControllerHealth struct {
	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes
	// It can be set to "0" or "" to disable serving the health probe.
	// Defaults to ":8081".
	// +optional
	// +kubebuilder:default=":8081"
	HealthProbeBindAddress string `json:"healthProbeBindAddress,omitempty"`

	// ReadinessEndpointName is the name for the readiness endpoint.
	// Defaults to "readyz".
	// +optional
	// +kubebuilder:default="readyz"
	ReadinessEndpointName string `json:"readinessEndpointName,omitempty"`

	// LivenessEndpointName is the name for the liveness endpoint.
	// Defaults to "healthz".
	// +optional
	// +kubebuilder:default="healthz"
	LivenessEndpointName string `json:"livenessEndpointName,omitempty"`
}

// ControllerConfigurationSpec defines the global configuration for
// controllers registered with the manager.
type ControllerConfigurationSpec struct {
	// GroupKindConcurrency is a map from a Kind to the number of concurrent reconciliation
	// allowed for that controller.
	//
	// When a controller is registered within this manager using the builder utilities,
	// users have to specify the type the controller reconciles in the For(...) call.
	// If the object's kind passed matches one of the keys in this map, the concurrency
	// for that controller is set to the number specified.
	//
	// The key is expected to be consistent in form with GroupKind.String(),
	// e.g. TrainJob.trainer.kubeflow.org.
	//
	// Defaults to empty map, which means no limits.
	// +optional
	GroupKindConcurrency map[string]int `json:"groupKindConcurrency,omitempty"`
}

// CertManagement holds configuration related to webhook server certificate generation.
type CertManagement struct {
	// Enable controls whether the cert management is enabled.
	// If disabled, certificates must be provided externally.
	// Defaults to true.
	// +optional
	// +kubebuilder:default=true
	Enable *bool `json:"enable,omitempty"`

	// WebhookServiceName is the name of the Service used as part of the DNSName
	// when generating the webhook server certificate.
	// Defaults to "kubeflow-trainer-controller-manager".
	// +optional
	// +kubebuilder:default="kubeflow-trainer-controller-manager"
	WebhookServiceName string `json:"webhookServiceName,omitempty"`

	// WebhookSecretName is the name of the Secret used to store the CA and server certificates.
	// Defaults to "kubeflow-trainer-webhook-cert".
	// +optional
	// +kubebuilder:default="kubeflow-trainer-webhook-cert"
	WebhookSecretName string `json:"webhookSecretName,omitempty"`
}

// ClientConnection provides additional configuration options for Kubernetes
// API server client.
type ClientConnection struct {
	// QPS controls the number of queries per second allowed before client-side throttling
	// connection to the API server.
	// Defaults to 50.
	// +optional
	// +kubebuilder:default=50
	QPS *float32 `json:"qps,omitempty"`

	// Burst allows extra queries to accumulate when a client is not using its full QPS allocation.
	// Defaults to 100.
	// +optional
	// +kubebuilder:default=100
	Burst *int32 `json:"burst,omitempty"`
}
