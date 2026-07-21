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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
// +kubebuilder:validation:XValidation:rule="self.parallelTrials <= self.numTrials",message="parallelTrials cannot exceed numTrials"
type OptimizationJobSpec struct {
	// +listType=map
	// +listMapKey=metric
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1
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

// SearchSpace acts as a Discriminated Union (OneOf) supporting flexible statistical distributions.
// +kubebuilder:validation:XValidation:rule="[has(self.uniform), has(self.logUniform), has(self.categorical)].filter(x, x).size() == 1",message="Exactly one search space distribution configuration must be provided"
type SearchSpace struct {
	// +optional
	Uniform *UniformSpace `json:"uniform,omitempty"`

	// +optional
	LogUniform *LogUniformSpace `json:"logUniform,omitempty"`

	// +optional
	Categorical *CategoricalSpace `json:"categorical,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.matches('^-?(0|[1-9][0-9]*)(\\.[0-9]+)?([eE][+-]?[0-9]+)?$')",message="value must be a valid numeric value"
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

type TrainJobTemplateSpec struct {
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.name) && !has(self.namespace)", message="name and namespace cannot be set in a template."
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec TrainJobSpec `json:"spec,omitzero"`
}

type OptimizationJobStatus struct {
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Result *Result `json:"result,omitempty"`
}

// Result tracks the parameters of the highest performing trial.
type Result struct {
	// TrainJobName is the name of the underlying TrainJob that achieved this result.
	// +kubebuilder:validation:MinLength=1
	// +required
	TrainJobName string `json:"trainJobName"`

	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	// +optional
	Parameters []ParameterAssignment `json:"parameters,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// OptimizationJob is the Schema for the optimizationjobs API.
type OptimizationJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OptimizationJobSpec   `json:"spec,omitempty"`
	Status OptimizationJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// OptimizationJobList contains a list of OptimizationJob.
type OptimizationJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OptimizationJob `json:"items"`
}
