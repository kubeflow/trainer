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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// Objective defines the metric and goal for the OptimizationJob.
type Objective struct {
	// +kubebuilder:validation:MinLength=1
	Metric string `json:"metric"`

	// +kubebuilder:validation:Enum=maximize;minimize
	Direction string `json:"direction"`

	// deferred to phase 2
	// Goal      *float64 `json:"goal,omitempty"`
}

// Algorithm defines the optimization algorithm configuration.
type Algorithm struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +listType=map
	// +listMapKey=name
	Settings []SettingKV `json:"settings,omitempty"`
}

// SettingKV is a key-value pair for algorithm settings.
type SettingKV struct {
	// +kubebuilder:validation:MinLength=1
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SearchSpace defines the type and exact boundaries for the algorithm to search.
// +kubebuilder:validation:XValidation:rule="self.type != 'categorical' || has(self.list)",message="list must be provided when type is categorical"
// +kubebuilder:validation:XValidation:rule="self.type == 'categorical' || (has(self.min) && has(self.max))",message="min and max must be provided for int or double types"
type SearchSpace struct {
	// +kubebuilder:validation:Enum=int;double;categorical
	Type string `json:"type"` // e.g., int, double, categorical

	Max string `json:"max,omitempty"`
	Min string `json:"min,omitempty"`

	// +listType=atomic
	// +optional
	List []string `json:"list,omitempty"`
}

// Parameter defines a single hyperparameter and its search space.
type Parameter struct {
	// +kubebuilder:validation:MinLength=1
	Name        string      `json:"name"`
	SearchSpace SearchSpace `json:"searchSpace"`
}

// ParameterAssignment represents a single hyperparameter and its assigned value.
type ParameterAssignment struct {
	// name is the user-defined label for the parameter (e.g., "learning_rate").
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name"`

	// value of the parameter. Values must be serialized as a string
	// to avoid float precision issues and align with Trainer v2 patterns.
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

// BestTrial tracks the best performing trial.
// Kept minimal for MVP; users can inspect the specific Trial/TrainJob for exact metrics and settings.
type BestTrial struct {
	Name string `json:"name"`

	// optimalParameters is a list of the hyperparameter assignments that won.
	// +listType=atomic
	// +optional
	OptimalParameters []ParameterAssignment `json:"optimalParameters,omitempty"`
}

// OptimizationJobSpec defines the desired state of OptimizationJob.
type OptimizationJobSpec struct {
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	Objectives []Objective `json:"objectives"`

	Algorithm Algorithm `json:"algorithm"`

	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Parameters []Parameter `json:"parameters"`

	TrialConfig TrialConfig `json:"trialConfig"`

	// TrialTemplate acts as a generic wrapper for the underlying workload.
	// Parameters are injected via native Kubernetes Environment Variables, replacing regex.
	// +kubebuilder:pruning:PreserveUnknownFields
	TrialTemplate runtime.RawExtension `json:"trialTemplate"`
}

// OptimizationJobStatus defines the observed state of OptimizationJob.
type OptimizationJobStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Counters for Trial states
	// +kubebuilder:validation:Minimum=0
	Active int32 `json:"active,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Succeeded int32 `json:"succeeded,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Failed int32 `json:"failed,omitempty"`

	// BestTrial caches the highest performing trial based on the Objective.
	BestTrial *BestTrial `json:"bestTrial,omitempty"`

	// Note: SchedulerState and SuggestionState are deferred to Phase 2
	// to support advanced stateful algorithms (Hyperband, PBT)
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
