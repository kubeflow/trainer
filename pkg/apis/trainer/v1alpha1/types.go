// Copyright 2024 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +k8s:defaulter-gen=TypeMeta
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package

// Package v2alpha1 is the v2alpha1 version of the API.
// +groupName=trainer.kubeflow.org

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TrainerControllerConfig represents the configuration for the training operator.
type TrainerControllerConfig struct {
	metav1.TypeMeta `json:",inline"`

	ControllerManager *ControllerManager `json:"controllerManager,omitempty"`
	CertGeneration   *CertGeneration   `json:"certGeneration,omitempty"`
}

// ControllerManager holds configuration related to the controller manager.
type ControllerManager struct {
	MetricsBindAddress      string `json:"metricsBindAddress,omitempty"`
	HealthProbeBindAddress  string `json:"healthProbeBindAddress,omitempty"`
	LeaderElect             bool   `json:"leaderElect,omitempty"`
	MetricsSecure           bool   `json:"metricsSecure,omitempty"`
	EnableHTTP2             bool   `json:"enableHttp2,omitempty"`
}

// CertGeneration holds configuration related to webhook server certificate generation.
type CertGeneration struct {
	WebhookServerPort  int    `json:"webhookServerPort,omitempty"`
	WebhookServiceName string `json:"webhookServiceName,omitempty"`
	WebhookSecretName  string `json:"webhookSecretName,omitempty"`
}
