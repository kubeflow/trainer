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

// Objective defines the metric and goal for the OptimizationJob.
type Objective struct {
	// +kubebuilder:validation:MinLength=1
	Metric string `json:"metric"`

	// +kubebuilder:validation:Enum=maximize;minimize
	Direction string `json:"direction"`
}

// Algorithm defines the optimization algorithm configuration.
type Algorithm struct {
	// Name of the optimization algorithm (e.g., random, grid, bayesian, tpe, hyperband).
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Provider specifies the backend suggestion engine executing the math (e.g., optuna, vizier).
	// If omitted, the controller will route to a cluster-default provider.
	// +optional
	Provider *string `json:"provider,omitempty"`

	// +listType=map
	// +listMapKey=name
	Settings []SettingKV `json:"settings,omitempty"`
}

// EarlyStopping defines the configuration for pruning unpromising trials.
type EarlyStopping struct {
	// Name of the early stopping algorithm (e.g., median, asha).
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +listType=map
	// +listMapKey=name
	// +optional
	Settings []SettingKV `json:"settings,omitempty"`
}

// SettingKV is a key-value pair for algorithm settings.
type SettingKV struct {
	// +kubebuilder:validation:MinLength=1
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SearchSpace defines the type and exact boundaries for the algorithm to search.
// +kubebuilder:validation:XValidation:rule="self.type != 'categorical' || (has(self.list) && size(self.list) > 0)",message="list must be provided and contain at least one item when type is categorical"
// +kubebuilder:validation:XValidation:rule="self.type == 'categorical' || (has(self.min) && has(self.max) && size(self.min) > 0 && size(self.max) > 0)",message="min and max must be provided and be non-empty for int or double types"
type SearchSpace struct {
	// +kubebuilder:validation:Enum=int;double;categorical
	Type string `json:"type"`

	// +kubebuilder:validation:MinLength=1
	Max string `json:"max,omitempty"`

	// +kubebuilder:validation:MinLength=1
	Min string `json:"min,omitempty"`

	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
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
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Value string `json:"value"`
}

// OptimizationStorage defines the persistent layer for trial checkpoints and state recovery.
type OptimizationStorage struct {
	// StorageUri is the remote object storage path (e.g., s3://my-bucket/experiments).
	// +kubebuilder:validation:Pattern=`^[A-Za-z][A-Za-z0-9+.-]*://.+$`
	// +optional
	StorageUri *string `json:"storageUri,omitempty"`

	// PvcName is the name of an existing PersistentVolumeClaim in the same namespace.
	// +optional
	PvcName *string `json:"pvcName,omitempty"`
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

	// Storage configures where suspended trials persist their checkpoints.
	// +optional
	Storage *OptimizationStorage `json:"storage,omitempty"`
}

// BestTrial tracks the best performing trial.
type BestTrial struct {
	Name string `json:"name"`

	// Value is the actual observed metric value achieved by this trial.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Value string `json:"value"`

	// +listType=atomic
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
	// Hyperparameters are injected into this template via String Templating.
	// Users can place placeholders like {{.parameter_name}} anywhere in this spec
	// (e.g., in args, env values, or annotations) and the controller will render
	// the actual values before applying the TrainJob.
	Spec TrainJobSpec `json:"spec"`
}

// OptimizationJobSpec defines the desired state of OptimizationJob.
type OptimizationJobSpec struct {
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	Objectives []Objective `json:"objectives"`

	Algorithm Algorithm `json:"algorithm"`

	// EarlyStopping separates the pruning logic from the search algorithm.
	// +optional
	EarlyStopping *EarlyStopping `json:"earlyStopping,omitempty"`

	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Parameters []Parameter `json:"parameters"`

	TrialConfig TrialConfig `json:"trialConfig"`

	// TrialTemplate wraps the underlying TrainJob workload and its metadata.
	// Parameter propagation is handled via native string rendering before creation.
	TrainJobTemplate TrainJobTemplateSpec `json:"trialTemplate"`
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

	// Suspended tracks trials that are paused by dynamic algorithms (e.g., PBT).
	// +kubebuilder:validation:Minimum=0
	Suspended int32 `json:"suspended,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Succeeded int32 `json:"succeeded,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Failed int32 `json:"failed,omitempty"`

	// BestTrial caches the highest performing trial based on the Objective.
	BestTrial *BestTrial `json:"bestTrial,omitempty"`
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
