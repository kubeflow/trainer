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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v2alpha1

import (
	v1 "k8s.io/api/core/v1"
)

// InputModelApplyConfiguration represents an declarative configuration of the InputModel type for use
// with apply.
type InputModelApplyConfiguration struct {
	StorageUri *string             `json:"storageUri,omitempty"`
	Env        []v1.EnvVar         `json:"env,omitempty"`
	SecretRef  *v1.SecretReference `json:"secretRef,omitempty"`
}

// InputModelApplyConfiguration constructs an declarative configuration of the InputModel type for use with
// apply.
func InputModel() *InputModelApplyConfiguration {
	return &InputModelApplyConfiguration{}
}

// WithStorageUri sets the StorageUri field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the StorageUri field is set to the value of the last call.
func (b *InputModelApplyConfiguration) WithStorageUri(value string) *InputModelApplyConfiguration {
	b.StorageUri = &value
	return b
}

// WithEnv adds the given value to the Env field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Env field.
func (b *InputModelApplyConfiguration) WithEnv(values ...v1.EnvVar) *InputModelApplyConfiguration {
	for i := range values {
		b.Env = append(b.Env, values[i])
	}
	return b
}

// WithSecretRef sets the SecretRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SecretRef field is set to the value of the last call.
func (b *InputModelApplyConfiguration) WithSecretRef(value v1.SecretReference) *InputModelApplyConfiguration {
	b.SecretRef = &value
	return b
}
