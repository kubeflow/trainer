/*
Copyright The Kubeflow Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
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

	// status of the OptimizationJob.
	// +optional
	Status OptimizationJobStatus `json:"status,omitzero"`
}

const (
	// OptimizationJobCreated means that the OptimizationJob has been accepted and initialized.
	OptimizationJobCreated string = "Created"

	// OptimizationJobComplete means that the OptimizationJob has completed its execution.
	OptimizationJobComplete string = "Complete"

	// OptimizationJobFailed means that the OptimizationJob has failed its execution.
	OptimizationJobFailed string = "Failed"
)

const (
	// OptimizationJobAlgorithmServiceCreationFailedReason is the reason when the suggestion service fails to launch.
	OptimizationJobAlgorithmServiceCreationFailedReason string = "AlgorithmServiceCreationFailed"

	// OptimizationJobTrainJobsCreationFailedReason is the reason when trial TrainJobs fail to launch.
	OptimizationJobTrainJobsCreationFailedReason string = "TrainJobsCreationFailed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=optimizationjobs
// +kubebuilder:object:root=true

// OptimizationJobList is a collection of optimization jobs.
type OptimizationJobList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of OptimizationJobs.
	Items []OptimizationJob `json:"items"`
}

// ObjectiveDirection specifies the optimization goal.
// +kubebuilder:validation:Enum=Maximize;Minimize
type ObjectiveDirection string

const (
	ObjectiveDirectionMaximize ObjectiveDirection = "Maximize"
	ObjectiveDirectionMinimize ObjectiveDirection = "Minimize"
)

// Objective defines the metric to track and optimization direction.
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
	// Objectives is the list of optimization goals.
	// +listType=map
	// +listMapKey=metric
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1
	// +required
	Objectives []Objective `json:"objectives"`

	// SearchAlgorithm specifies the search algorithm configuration.
	// +optional
	SearchAlgorithm *SearchAlgorithm `json:"searchAlgorithm,omitempty"`

	// Parameters specifies the hyperparameter search space definitions.
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

	// TrainJobTemplate is the specification template for trial workloads.
	// +required
	TrainJobTemplate TrainJobTemplateSpec `json:"trainJobTemplate"`
}

// SearchAlgorithm specifies search algorithm configuration (OneOf pattern).
// +kubebuilder:validation:XValidation:rule="[has(self.random), has(self.grid)].filter(x, x).size() == 1",message="Exactly one search algorithm configuration must be provided"
type SearchAlgorithm struct {
	// Random algorithm settings.
	// +optional
	Random *RandomAlgorithm `json:"random,omitempty"`

	// Grid algorithm settings.
	// +optional
	Grid *GridAlgorithm `json:"grid,omitempty"`
}

// RandomAlgorithm defines configuration for Random Search.
type RandomAlgorithm struct {
	// Seed for random number generator reproducibility.
	// +optional
	Seed *int64 `json:"seed,omitempty"`
}

// GridAlgorithm defines configuration for Grid Search.
type GridAlgorithm struct{}

// ParameterType specifies underlying data type for parameter bounds.
// +kubebuilder:validation:Enum=Int;Float
type ParameterType string

const (
	ParameterTypeInt   ParameterType = "Int"
	ParameterTypeFloat ParameterType = "Float"
)

// SearchSpace acts as a Discriminated Union (OneOf) supporting statistical distributions.
// +kubebuilder:validation:XValidation:rule="[has(self.uniform), has(self.logUniform), has(self.categorical)].filter(x, x).size() == 1",message="Exactly one search space distribution configuration must be provided"
type SearchSpace struct {
	// Uniform distribution.
	// +optional
	Uniform *UniformSpace `json:"uniform,omitempty"`

	// LogUniform distribution.
	// +optional
	LogUniform *LogUniformSpace `json:"logUniform,omitempty"`

	// Categorical distribution.
	// +optional
	Categorical *CategoricalSpace `json:"categorical,omitempty"`
}

// Double is a string type representing numeric values without float rounding precision loss.
// +kubebuilder:validation:XValidation:rule="self.matches('^-?(0|[1-9][0-9]*)(\\\\.[0-9]+)?([eE][+-]?[0-9]+)?$')",message="value must be a valid numeric value"
// +kubebuilder:validation:MaxLength=64
type Double string

// UniformSpace defines a continuous uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type UniformSpace struct {
	// Min bound of uniform distribution.
	// +required
	Min Double `json:"min"`

	// Max bound of uniform distribution.
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
	// Min bound of log-uniform distribution.
	// +required
	Min Double `json:"min"`

	// Max bound of log-uniform distribution.
	// +required
	Max Double `json:"max"`

	// Type specifies the underlying data type. Defaults to "Float".
	// +kubebuilder:default=Float
	// +required
	Type ParameterType `json:"type"`
}

// CategoricalSpace defines a search space over discrete string choices.
type CategoricalSpace struct {
	// Choices is the set of strings to sample from.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +listType=set
	// +required
	Choices []string `json:"choices"`
}

// Parameter defines a named hyperparameter and its search space.
type Parameter struct {
	// Name of the hyperparameter.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name"`

	// SearchSpace definition for this parameter.
	// +required
	SearchSpace SearchSpace `json:"searchSpace"`
}

// ParameterAssignment represents a single hyperparameter and its assigned trial value.
type ParameterAssignment struct {
	// Name of the hyperparameter.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name"`

	// Value assigned for this trial.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Value string `json:"value"`
}

// TrainJobTemplateSpec defines the template for creating trial TrainJobs.
type TrainJobTemplateSpec struct {
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.name) && !has(self.namespace)", message="name and namespace cannot be set in a template."
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec of the TrainJob.
	// +required
	Spec TrainJobSpec `json:"spec,omitzero"`
}

// OptimizationJobStatus defines the observed state of OptimizationJob.
type OptimizationJobStatus struct {
	// Conditions of the OptimizationJob.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Result of the highest performing trial.
	// +optional
	Result *Result `json:"result,omitempty"`
}

// Result tracks the parameters of the highest performing trial.
type Result struct {
	// TrainJobName is the name of the underlying TrainJob that achieved this result.
	// +kubebuilder:validation:MinLength=1
	// +required
	TrainJobName string `json:"trainJobName"`

	// Parameters contains the parameter assignments of the best trial.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	// +optional
	Parameters []ParameterAssignment `json:"parameters,omitempty"`
}

// --- DeepCopy implementation methods ---

func (in *OptimizationJob) DeepCopyInto(out *OptimizationJob) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *OptimizationJob) DeepCopy() *OptimizationJob {
	if in == nil {
		return nil
	}
	out := new(OptimizationJob)
	in.DeepCopyInto(out)
	return out
}

func (in *OptimizationJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *OptimizationJobList) DeepCopyInto(out *OptimizationJobList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OptimizationJob, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *OptimizationJobList) DeepCopy() *OptimizationJobList {
	if in == nil {
		return nil
	}
	out := new(OptimizationJobList)
	in.DeepCopyInto(out)
	return out
}

func (in *OptimizationJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *OptimizationJobSpec) DeepCopyInto(out *OptimizationJobSpec) {
	*out = *in
	if in.Objectives != nil {
		in, out := &in.Objectives, &out.Objectives
		*out = make([]Objective, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.SearchAlgorithm != nil {
		in, out := &in.SearchAlgorithm, &out.SearchAlgorithm
		*out = new(SearchAlgorithm)
		(*in).DeepCopyInto(*out)
	}
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make([]Parameter, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NumTrials != nil {
		in, out := &in.NumTrials, &out.NumTrials
		*out = new(int32)
		**out = **in
	}
	if in.ParallelTrials != nil {
		in, out := &in.ParallelTrials, &out.ParallelTrials
		*out = new(int32)
		**out = **in
	}
	in.TrainJobTemplate.DeepCopyInto(&out.TrainJobTemplate)
}

func (in *OptimizationJobSpec) DeepCopy() *OptimizationJobSpec {
	if in == nil {
		return nil
	}
	out := new(OptimizationJobSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *Objective) DeepCopyInto(out *Objective) {
	*out = *in
	if in.Metric != nil {
		in, out := &in.Metric, &out.Metric
		*out = new(string)
		**out = **in
	}
	if in.Direction != nil {
		in, out := &in.Direction, &out.Direction
		*out = new(ObjectiveDirection)
		**out = **in
	}
}

func (in *Objective) DeepCopy() *Objective {
	if in == nil {
		return nil
	}
	out := new(Objective)
	in.DeepCopyInto(out)
	return out
}

func (in *SearchAlgorithm) DeepCopyInto(out *SearchAlgorithm) {
	*out = *in
	if in.Random != nil {
		in, out := &in.Random, &out.Random
		*out = new(RandomAlgorithm)
		(*in).DeepCopyInto(*out)
	}
	if in.Grid != nil {
		in, out := &in.Grid, &out.Grid
		*out = new(GridAlgorithm)
		**out = **in
	}
}

func (in *SearchAlgorithm) DeepCopy() *SearchAlgorithm {
	if in == nil {
		return nil
	}
	out := new(SearchAlgorithm)
	in.DeepCopyInto(out)
	return out
}

func (in *RandomAlgorithm) DeepCopyInto(out *RandomAlgorithm) {
	*out = *in
	if in.Seed != nil {
		in, out := &in.Seed, &out.Seed
		*out = new(int64)
		**out = **in
	}
}

func (in *RandomAlgorithm) DeepCopy() *RandomAlgorithm {
	if in == nil {
		return nil
	}
	out := new(RandomAlgorithm)
	in.DeepCopyInto(out)
	return out
}

func (in *GridAlgorithm) DeepCopyInto(out *GridAlgorithm) {
	*out = *in
}

func (in *GridAlgorithm) DeepCopy() *GridAlgorithm {
	if in == nil {
		return nil
	}
	out := new(GridAlgorithm)
	in.DeepCopyInto(out)
	return out
}

func (in *SearchSpace) DeepCopyInto(out *SearchSpace) {
	*out = *in
	if in.Uniform != nil {
		in, out := &in.Uniform, &out.Uniform
		*out = new(UniformSpace)
		**out = **in
	}
	if in.LogUniform != nil {
		in, out := &in.LogUniform, &out.LogUniform
		*out = new(LogUniformSpace)
		**out = **in
	}
	if in.Categorical != nil {
		in, out := &in.Categorical, &out.Categorical
		*out = new(CategoricalSpace)
		(*in).DeepCopyInto(*out)
	}
}

func (in *SearchSpace) DeepCopy() *SearchSpace {
	if in == nil {
		return nil
	}
	out := new(SearchSpace)
	in.DeepCopyInto(out)
	return out
}

func (in *UniformSpace) DeepCopyInto(out *UniformSpace) {
	*out = *in
}

func (in *UniformSpace) DeepCopy() *UniformSpace {
	if in == nil {
		return nil
	}
	out := new(UniformSpace)
	in.DeepCopyInto(out)
	return out
}

func (in *LogUniformSpace) DeepCopyInto(out *LogUniformSpace) {
	*out = *in
}

func (in *LogUniformSpace) DeepCopy() *LogUniformSpace {
	if in == nil {
		return nil
	}
	out := new(LogUniformSpace)
	in.DeepCopyInto(out)
	return out
}

func (in *CategoricalSpace) DeepCopyInto(out *CategoricalSpace) {
	*out = *in
	if in.Choices != nil {
		in, out := &in.Choices, &out.Choices
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *CategoricalSpace) DeepCopy() *CategoricalSpace {
	if in == nil {
		return nil
	}
	out := new(CategoricalSpace)
	in.DeepCopyInto(out)
	return out
}

func (in *Parameter) DeepCopyInto(out *Parameter) {
	*out = *in
	in.SearchSpace.DeepCopyInto(&out.SearchSpace)
}

func (in *Parameter) DeepCopy() *Parameter {
	if in == nil {
		return nil
	}
	out := new(Parameter)
	in.DeepCopyInto(out)
	return out
}

func (in *ParameterAssignment) DeepCopyInto(out *ParameterAssignment) {
	*out = *in
}

func (in *ParameterAssignment) DeepCopy() *ParameterAssignment {
	if in == nil {
		return nil
	}
	out := new(ParameterAssignment)
	in.DeepCopyInto(out)
	return out
}

func (in *TrainJobTemplateSpec) DeepCopyInto(out *TrainJobTemplateSpec) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

func (in *TrainJobTemplateSpec) DeepCopy() *TrainJobTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(TrainJobTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *OptimizationJobStatus) DeepCopyInto(out *OptimizationJobStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Result != nil {
		in, out := &in.Result, &out.Result
		*out = new(Result)
		(*in).DeepCopyInto(*out)
	}
}

func (in *OptimizationJobStatus) DeepCopy() *OptimizationJobStatus {
	if in == nil {
		return nil
	}
	out := new(OptimizationJobStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *Result) DeepCopyInto(out *Result) {
	*out = *in
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make([]ParameterAssignment, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *Result) DeepCopy() *Result {
	if in == nil {
		return nil
	}
	out := new(Result)
	in.DeepCopyInto(out)
	return out
}
