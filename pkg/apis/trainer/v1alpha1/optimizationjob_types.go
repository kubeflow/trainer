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

// OptimizationJobSpec defines the desired state of OptimizationJob.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="OptimizationJobSpec is immutable and cannot be updated after creation"
// +kubebuilder:validation:XValidation:rule="self.parallelTrials <= self.numTrials",message="parallelTrials cannot exceed numTrials"
// +kubebuilder:validation:XValidation:rule="!has(self.searchAlgorithm.grid) || self.parameters.all(p, has(p.searchSpace.categorical))",message="Grid search requires all parameters to be Categorical; Uniform and LogUniform are not supported."
type OptimizationJobSpec struct {
	// objectives is the list of objectives to optimize.
	// +listType=map
	// +listMapKey=metric
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +required
	Objectives []Objective `json:"objectives,omitempty"`

	// searchAlgorithm is the algorithm to use for searching over the hyperparameters.
	// +kubebuilder:default={random: {}}
	// +optional
	SearchAlgorithm *SearchAlgorithm `json:"searchAlgorithm,omitempty"`

	// parameters is the list of hyperparameters to search over.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +required
	Parameters []Parameter `json:"parameters,omitempty"`

	// numTrials is the total number of trials to run.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	NumTrials int32 `json:"numTrials,omitempty"`

	// parallelTrials is the number of trials to run in parallel. Defaults to 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	ParallelTrials int32 `json:"parallelTrials,omitempty"`

	// trainJobTemplate is the template for the train job to run.
	// +required
	TrainJobTemplate TrainJobTemplateSpec `json:"trainJobTemplate,omitzero"`
}

type Objective struct {
	// metric specifies the name of the objective metric to track.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Metric string `json:"metric,omitempty"`

	// direction specifies the optimization goal. Defaults to "Minimize".
	// +kubebuilder:default=Minimize
	// +optional
	Direction ObjectiveDirection `json:"direction,omitempty"`
}

// +kubebuilder:validation:ExactlyOneOf=random;grid
type SearchAlgorithm struct {
	// random is the random search algorithm.
	// +optional
	Random *RandomAlgorithm `json:"random,omitempty"`

	// grid is the grid search algorithm.
	// +optional
	Grid *GridAlgorithm `json:"grid,omitempty"`
}

// RandomAlgorithm is the random search algorithm.
type RandomAlgorithm struct {
	// seed is the seed for the random search algorithm.
	// +optional
	Seed *int64 `json:"seed,omitempty"`
}

// GridAlgorithm is the grid search algorithm.
type GridAlgorithm struct{}

// SearchSpace acts as a Discriminated Union (OneOf) supporting flexible statistical distributions.
// +kubebuilder:validation:ExactlyOneOf=uniform;logUniform;categorical
type SearchSpace struct {
	// uniform is the uniform search space.
	// +optional
	Uniform UniformSpace `json:"uniform,omitempty,omitzero"`

	// logUniform is the log-uniform search space.
	// +optional
	LogUniform LogUniformSpace `json:"logUniform,omitempty,omitzero"`

	// categorical is the categorical search space.
	// +optional
	Categorical CategoricalSpace `json:"categorical,omitempty,omitzero"`
}

// +kubebuilder:validation:Pattern="^-?(0|[1-9][0-9]*)(\\.[0-9]+)?([eE][+-]?[0-9]+)?$"
// +kubebuilder:validation:MaxLength=64
type Double string

// UniformSpace defines a continuous uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type UniformSpace struct {
	// min is the minimum value of the uniform search space.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +required
	Min Double `json:"min,omitempty"`

	// max is the maximum value of the uniform search space.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +required
	Max Double `json:"max,omitempty"`

	// type specifies the underlying data type. Defaults to "Float".
	// +kubebuilder:default=Float
	// +optional
	Type ParameterType `json:"type,omitempty"`
}

// LogUniformSpace defines a continuous log-uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) > 0.0",message="min must be strictly greater than 0"
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type LogUniformSpace struct {
	// min is the minimum value of the log-uniform search space.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +required
	Min Double `json:"min,omitempty"`

	// max is the maximum value of the log-uniform search space.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +required
	Max Double `json:"max,omitempty"`

	// type specifies the underlying data type. Defaults to "Float".
	// +kubebuilder:default=Float
	// +optional
	Type ParameterType `json:"type,omitempty"`
}

// ParameterType is the type of the parameter.
// +kubebuilder:validation:Enum=Int;Float
type ParameterType string

const (
	ParameterTypeInt   ParameterType = "Int"
	ParameterTypeFloat ParameterType = "Float"
)

// CategoricalSpace defines a search space over a discrete set of unordered strings.
type CategoricalSpace struct {
	// choices is the set of strings to sample from.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:items:MaxLength=64
	// +listType=set
	// +required
	Choices []string `json:"choices,omitempty"`
}

type Parameter struct {
	// name is the name of the hyperparameter.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name,omitempty"`

	// searchSpace is the search space for the hyperparameter.
	// +required
	SearchSpace *SearchSpace `json:"searchSpace,omitempty"`
}

// ParameterAssignment represents a single hyperparameter and its assigned value.
type ParameterAssignment struct {
	// name is the name of the hyperparameter.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name,omitempty"`

	// value is the value of the hyperparameter.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Value string `json:"value,omitempty"`
}

// TrainJobTemplateSpec is the template for the train job to run.
type TrainJobTemplateSpec struct {
	// metadata is the metadata for the train job.
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.name) && !has(self.namespace)", message="name and namespace cannot be set in a template."
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the spec for the train job.
	// +required
	Spec TrainJobSpec `json:"spec,omitzero"`
}

// ObjectiveMetricValue represents a single objective metric and its value.
type ObjectiveMetricValue struct {
	// metric is the name of the objective metric.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Metric string `json:"metric,omitempty"`

	// value is the reported value for the objective metric.
	// +required
	Value string `json:"value,omitempty"`
}

// OptimalTrial represents a trial on the optimal Pareto front (supporting single and multi-objective optimization).
type OptimalTrial struct {
	// trainJobName is the name of the underlying TrainJob that achieved this result.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +required
	TrainJobName string `json:"trainJobName,omitempty"`

	// metrics achieved by this optimal trial (multi-metric support).
	// +listType=map
	// +listMapKey=metric
	// +optional
	Metrics []ObjectiveMetricValue `json:"metrics,omitempty"`

	// parameters assigned to this trial.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	// +optional
	Parameters []ParameterAssignment `json:"parameters,omitempty"`
}

// OptimizationJobStatus is the status of the optimization job.
type OptimizationJobStatus struct {
	// conditions is the list of conditions for the optimization job.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=100
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// results tracks optimal trials (Pareto front for multi-objective optimization).
	// +optional
	Results []OptimalTrial `json:"results,omitempty,omitzero"`
}
