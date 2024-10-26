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
	v2alpha1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v2alpha1"
)

// MPIMLPolicySourceApplyConfiguration represents an declarative configuration of the MPIMLPolicySource type for use
// with apply.
type MPIMLPolicySourceApplyConfiguration struct {
	NumProcPerNode    *int32                      `json:"numProcPerNode,omitempty"`
	MPIImplementation *v2alpha1.MPIImplementation `json:"mpiImplementation,omitempty"`
	SSHAuthMountPath  *string                     `json:"SSHAuthMountPath,omitempty"`
	RunLauncherAsNode *bool                       `json:"runLauncherAsNode,omitempty"`
}

// MPIMLPolicySourceApplyConfiguration constructs an declarative configuration of the MPIMLPolicySource type for use with
// apply.
func MPIMLPolicySource() *MPIMLPolicySourceApplyConfiguration {
	return &MPIMLPolicySourceApplyConfiguration{}
}

// WithNumProcPerNode sets the NumProcPerNode field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the NumProcPerNode field is set to the value of the last call.
func (b *MPIMLPolicySourceApplyConfiguration) WithNumProcPerNode(value int32) *MPIMLPolicySourceApplyConfiguration {
	b.NumProcPerNode = &value
	return b
}

// WithMPIImplementation sets the MPIImplementation field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the MPIImplementation field is set to the value of the last call.
func (b *MPIMLPolicySourceApplyConfiguration) WithMPIImplementation(value v2alpha1.MPIImplementation) *MPIMLPolicySourceApplyConfiguration {
	b.MPIImplementation = &value
	return b
}

// WithSSHAuthMountPath sets the SSHAuthMountPath field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SSHAuthMountPath field is set to the value of the last call.
func (b *MPIMLPolicySourceApplyConfiguration) WithSSHAuthMountPath(value string) *MPIMLPolicySourceApplyConfiguration {
	b.SSHAuthMountPath = &value
	return b
}

// WithRunLauncherAsNode sets the RunLauncherAsNode field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the RunLauncherAsNode field is set to the value of the last call.
func (b *MPIMLPolicySourceApplyConfiguration) WithRunLauncherAsNode(value bool) *MPIMLPolicySourceApplyConfiguration {
	b.RunLauncherAsNode = &value
	return b
}
