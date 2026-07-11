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

// +kubebuilder:validation:Enum=maximize;minimize
type ObjectiveDirection string

// Objective defines the metric and goal for the OptimizationJob.
type Objective struct {
	// +kubebuilder:validation:MinLength=1
	Metric string `json:"metric"`

	Direction ObjectiveDirection `json:"direction"`
}

// SearchAlgorithm defines the hyperparameter sampling configuration.
// +kubebuilder:validation:XValidation:rule="[has(self.random), has(self.grid), has(self.bayesian)].filter(x, x).size() == 1",message="Exactly one search algorithm configuration must be provided"
type SearchAlgorithm struct {
	// +optional
	Random *RandomAlgorithm `json:"random,omitempty"`
	// +optional
	Grid *GridAlgorithm `json:"grid,omitempty"`
	// +optional
	Bayesian *BayesianAlgorithm `json:"bayesian,omitempty"`
}

type RandomAlgorithm struct {
	// +optional
	RandomState *int64 `json:"randomState,omitempty"`
}

// GridAlgorithm is intentionally empty; step-intervals are derived from SearchSpace.Int.Step.
type GridAlgorithm struct{}

type BayesianAlgorithm struct {
	// +kubebuilder:validation:Minimum=1
	// +optional
	InitialTrials *int32 `json:"initialTrials,omitempty"`

	// +kubebuilder:validation:Enum=ucb;ei;pi
	// +optional
	AcquisitionFunction *string `json:"acquisitionFunction,omitempty"`
}

type SettingKV struct {
	// +kubebuilder:validation:MinLength=1
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SearchSpace acts as a Discriminated Union (OneOf) supporting flexible statistical distributions.
// +kubebuilder:validation:XValidation:rule="(has(self.uniform) ? 1 : 0) + (has(self.logUniform) ? 1 : 0) + (has(self.categorical) ? 1 : 0) == 1",message="Exactly one search space distribution configuration must be provided"
type SearchSpace struct {
	// +optional
	Uniform *UniformSpace `json:"uniform,omitempty"`

	// +optional
	LogUniform *LogUniformSpace `json:"logUniform,omitempty"`

	// +optional
	Categorical *CategoricalSpace `json:"categorical,omitempty"`
}

// UniformSpace defines a continuous uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type UniformSpace struct {
	// +kubebuilder:validation:MinLength=1
	Min string `json:"min"`

	// +kubebuilder:validation:MinLength=1
	Max string `json:"max"`
}

// LogUniformSpace defines a continuous log-uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) > 0.0",message="min must be strictly greater than 0 for a log-uniform distribution"
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type LogUniformSpace struct {
	// +kubebuilder:validation:MinLength=1
	Min string `json:"min"`

	// +kubebuilder:validation:MinLength=1
	Max string `json:"max"`

	// Type specifies the underlying data type. Defaults to "float".
	// +optional
	Type *string `json:"type,omitempty"`
}

// CategoricalSpace defines a search space over a discrete set of unordered strings.
type CategoricalSpace struct {
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	Choices []string `json:"choices"`
}

// Parameter defines a single hyperparameter and its search space.
type Parameter struct {
	// +kubebuilder:validation:MinLength=1
	Name        string      `json:"name"`
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

// TrialConfig controls the orchestration of the trials.
// +kubebuilder:validation:XValidation:rule="!has(self.parallelTrials) || !has(self.numTrials) || self.parallelTrials <= self.numTrials",message="parallelTrials cannot exceed numTrials"
type TrialConfig struct {
	// +kubebuilder:validation:Minimum=1
	NumTrials *int32 `json:"numTrials,omitempty"`

	// +kubebuilder:validation:Minimum=1
	ParallelTrials *int32 `json:"parallelTrials,omitempty"`

	// +kubebuilder:validation:Minimum=0
	MaxFailedTrials *int32 `json:"maxFailedTrials,omitempty"`
}

// Result tracks the parameters of the highest performing trial.
type Result struct {
	// TrainJobName is the name of the underlying TrainJob that achieved this result.
	// +kubebuilder:validation:MinLength=1
	// +required
	TrainJobName string `json:"trainJobName"`
	// +listType=map
	// +listMapKey=name
	// +optional
	Parameters []ParameterAssignment `json:"parameters,omitempty"`
}

// TrainJobTemplateSpec describes the metadata and spec of the TrainJobs created by the OptimizationJob.
type TrainJobTemplateSpec struct {
	// Standard object's metadata.
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.name) && !has(self.namespace)", message="name and namespace cannot be set in a template."
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the TrainJob.
	// Hyperparameters are injected into this template dynamically by the controller
	// via prefixed environment variables (KUBEFLOW_OPT_*) and metadata annotations.
	Spec TrainJobSpec `json:"spec"`
}

// OptimizationJobSpec defines the desired state of OptimizationJob.
type OptimizationJobSpec struct {
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	Objectives []Objective `json:"objectives"`

	// SearchAlgorithm explicitly separates initial sampling from mid-run pruning.
	SearchAlgorithm SearchAlgorithm `json:"searchAlgorithm"`

	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Parameters []Parameter `json:"parameters"`

	TrialConfig TrialConfig `json:"trialConfig"`

	// TrainJobTemplate wraps the underlying TrainJob workload and its metadata.
	TrainJobTemplate TrainJobTemplateSpec `json:"trainJobTemplate"`
}

// OptimizationJobPhase represents the current phase of the OptimizationJob.
type OptimizationJobPhase string

const (
	OptimizationJobScheduling OptimizationJobPhase = "Scheduling"
	OptimizationJobRunning    OptimizationJobPhase = "Running"
	OptimizationJobSucceeded  OptimizationJobPhase = "Succeeded"
	OptimizationJobFailed     OptimizationJobPhase = "Failed"
)

// OptimizationJobStatus defines the observed state of OptimizationJob.
type OptimizationJobStatus struct {
	// +optional
	Phase OptimizationJobPhase `json:"phase,omitempty"`

	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Active int32 `json:"active,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Succeeded int32 `json:"succeeded,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Failed int32 `json:"failed,omitempty"`

	// Result caches the highest performing parameters based on the Objective.
	Result *Result `json:"result,omitempty"`
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
