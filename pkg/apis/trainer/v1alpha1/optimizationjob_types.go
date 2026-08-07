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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// OptimizationJobKind is the Kind name for the OptimizationJob.
	OptimizationJobKind string = "OptimizationJob"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.conditions[-1:].type`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.metadata.name.matches('^[a-z]([-a-z0-9]*[a-z0-9])?$')", message="metadata.name must match RFC 1035 DNS label format"
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 63", message="metadata.name must be no more than 63 characters"

// OptimizationJob represents configuration of a hyperparameter optimization job.
type OptimizationJob struct {
	metav1.TypeMeta `json:",inline"`

	// metadata of the OptimizationJob.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec of the OptimizationJob.
	// +optional
	Spec OptimizationJobSpec `json:"spec,omitzero"`

	// status of OptimizationJob.
	// +optional
	Status OptimizationJobStatus `json:"status,omitzero"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// OptimizationJobList is a list of OptimizationJobs.
type OptimizationJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OptimizationJob `json:"items"`
}

// +kubebuilder:validation:Enum=Maximize;Minimize
type ObjectiveDirection string

const (
	ObjectiveDirectionMaximize ObjectiveDirection = "Maximize"
	ObjectiveDirectionMinimize ObjectiveDirection = "Minimize"
)

type Objective struct {
	// Metric specifies the name of the objective metric to track. Defaults to "loss".
	// +kubebuilder:default=loss
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +optional
	Metric *string `json:"metric,omitempty"`

	// Direction specifies the optimization goal. Defaults to "Minimize".
	// +kubebuilder:default=Minimize
	// +optional
	Direction *ObjectiveDirection `json:"direction,omitempty"`
}

// OptimizationJobSpec defines the desired state of OptimizationJob.
// +kubebuilder:validation:XValidation:rule="!has(self.parallelTrials) || !has(self.numTrials) || self.parallelTrials <= self.numTrials",message="parallelTrials cannot exceed numTrials"
type OptimizationJobSpec struct {
	// Objectives configures the target metrics for optimization.
	// Supports multi-objective optimization (KEP-3562 / issue #3799).
	// +listType=map
	// +listMapKey=metric
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +required
	Objectives []Objective `json:"objectives"`

	// +optional
	SearchAlgorithm *SearchAlgorithm `json:"searchAlgorithm,omitempty"`

	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +required
	Parameters []Parameter `json:"parameters"`

	// NumTrials is the total number of trials to run.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	NumTrials *int32 `json:"numTrials,omitempty"`

	// ParallelTrials is the number of trials to run in parallel. Defaults to 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	ParallelTrials *int32 `json:"parallelTrials,omitempty"`

	// +required
	TrainJobTemplate TrainJobTemplateSpec `json:"trainJobTemplate"`
}

// +kubebuilder:validation:XValidation:rule="[has(self.random), has(self.grid)].filter(x, x).size() == 1",message="Exactly one search algorithm configuration must be provided"
type SearchAlgorithm struct {
	// +optional
	Random *RandomAlgorithm `json:"random,omitempty"`
	// +optional
	Grid *GridAlgorithm `json:"grid,omitempty"`
}

type RandomAlgorithm struct {
	// +optional
	Seed *int64 `json:"seed,omitempty"`
}

type GridAlgorithm struct{}

// +kubebuilder:validation:Enum=Int;Float
type ParameterType string

const (
	ParameterTypeInt   ParameterType = "Int"
	ParameterTypeFloat ParameterType = "Float"
)

// SearchSpace acts as a Discriminated Union supporting flexible statistical distributions.
// +kubebuilder:validation:XValidation:rule="[has(self.uniform), has(self.logUniform), has(self.categorical)].filter(x, x).size() == 1",message="Exactly one search space distribution configuration must be provided"
type SearchSpace struct {
	// +optional
	Uniform *UniformSpace `json:"uniform,omitempty"`

	// +optional
	LogUniform *LogUniformSpace `json:"logUniform,omitempty"`

	// +optional
	Categorical *CategoricalSpace `json:"categorical,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.matches('^-?(0|[1-9][0-9]*)(\\\\.[0-9]+)?([eE][+-]?[0-9]+)?$')",message="value must be a valid numeric value"
// +kubebuilder:validation:MaxLength=64
type Double string

// UniformSpace defines a continuous uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type UniformSpace struct {
	// +required
	Min Double `json:"min"`

	// +required
	Max Double `json:"max"`

	// Type specifies the underlying data type. Defaults to "Float".
	// +kubebuilder:default=Float
	// +required
	Type ParameterType `json:"type"`
}

// LogUniformSpace defines a continuous log-uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) > 0.0",message="min must be strictly greater than 0"
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type LogUniformSpace struct {
	// +required
	Min Double `json:"min"`

	// +required
	Max Double `json:"max"`

	// Type specifies the underlying data type. Defaults to "Float".
	// +kubebuilder:default=Float
	// +required
	Type ParameterType `json:"type"`
}

// CategoricalSpace defines a search space over a discrete set of unordered strings.
type CategoricalSpace struct {
	// Choices is the set of strings to sample from.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +listType=set
	// +required
	Choices []string `json:"choices"`
}

type Parameter struct {
	// Name is the name of the hyperparameter.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name"`

	// +required
	SearchSpace SearchSpace `json:"searchSpace"`
}

// ParameterAssignment represents a single hyperparameter and its assigned value.
type ParameterAssignment struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Value string `json:"value"`
}

type ObjectiveMetricValue struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Metric string `json:"metric"`

	// +required
	Value string `json:"value"`
}

type TrainJobTemplateSpec struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec TrainJobSpec `json:"spec,omitzero"`
}

// OptimalTrial represents a trial on the optimal Pareto front (supporting single and multi-objective optimization).
type OptimalTrial struct {
	// TrainJobName is the name of the underlying TrainJob that achieved this result.
	// +kubebuilder:validation:MinLength=1
	// +required
	TrainJobName string `json:"trainJobName"`

	// Metrics achieved by this optimal trial (multi-metric support).
	// +listType=map
	// +listMapKey=metric
	// +optional
	Metrics []ObjectiveMetricValue `json:"metrics,omitempty"`

	// Parameters assigned to this trial.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	// +optional
	Parameters []ParameterAssignment `json:"parameters,omitempty"`
}

type OptimizationJobStatus struct {
	// Conditions of the OptimizationJob.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Results tracks optimal trials (Pareto front for multi-objective optimization).
	// +optional
	Results []OptimalTrial `json:"results,omitempty"`
}
