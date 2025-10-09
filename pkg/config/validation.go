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

package config

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

// validate validates the configuration.
func validate(cfg *configapi.Configuration) field.ErrorList {
	var allErrs field.ErrorList

	// Validate webhook port
	if cfg.Webhook.Port != nil && (*cfg.Webhook.Port < 1 || *cfg.Webhook.Port > 65535) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("webhook", "port"), *cfg.Webhook.Port, "must be between 1 and 65535"))
	}

	// Validate client connection QPS and Burst
	if cfg.ClientConnection != nil {
		if cfg.ClientConnection.QPS != nil && *cfg.ClientConnection.QPS < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("clientConnection", "qps"), *cfg.ClientConnection.QPS, "must be greater than or equal to 0"))
		}
		if cfg.ClientConnection.Burst != nil && *cfg.ClientConnection.Burst < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("clientConnection", "burst"), *cfg.ClientConnection.Burst, "must be greater than or equal to 0"))
		}
	}

	// Validate GroupKindConcurrency values
	if cfg.Controller != nil && cfg.Controller.GroupKindConcurrency != nil {
		for gk, concurrency := range cfg.Controller.GroupKindConcurrency {
			if concurrency < 1 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("controller", "groupKindConcurrency").Key(gk), concurrency, "must be greater than 0"))
			}
		}
	}

	return allErrs
}
