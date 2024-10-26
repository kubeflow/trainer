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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TrainJobStatusApplyConfiguration represents an declarative configuration of the TrainJobStatus type for use
// with apply.
type TrainJobStatusApplyConfiguration struct {
	Conditions []v1.Condition                `json:"conditions,omitempty"`
	JobsStatus []JobStatusApplyConfiguration `json:"jobsStatus,omitempty"`
}

// TrainJobStatusApplyConfiguration constructs an declarative configuration of the TrainJobStatus type for use with
// apply.
func TrainJobStatus() *TrainJobStatusApplyConfiguration {
	return &TrainJobStatusApplyConfiguration{}
}

// WithConditions adds the given value to the Conditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Conditions field.
func (b *TrainJobStatusApplyConfiguration) WithConditions(values ...v1.Condition) *TrainJobStatusApplyConfiguration {
	for i := range values {
		b.Conditions = append(b.Conditions, values[i])
	}
	return b
}

// WithJobsStatus adds the given value to the JobsStatus field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the JobsStatus field.
func (b *TrainJobStatusApplyConfiguration) WithJobsStatus(values ...*JobStatusApplyConfiguration) *TrainJobStatusApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithJobsStatus")
		}
		b.JobsStatus = append(b.JobsStatus, *values[i])
	}
	return b
}
