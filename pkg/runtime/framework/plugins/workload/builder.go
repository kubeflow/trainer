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

package workload

import (
	"errors"
	"fmt"
	"slices"

	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-helpers/scheduling/schedulingv1/workloadbuilder"
	"k8s.io/utils/ptr"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

// newBuilders returns one Builder per PodGroupTemplate of the TrainJob Workload.
func newBuilders(info *runtime.Info, trainJob *trainer.TrainJob) []*workloadbuilder.Builder {
	scheduling := info.RuntimePolicy.Scheduling
	jobSetSpec, _ := runtime.TemplateSpecApply[jobsetv1alpha2ac.JobSetSpecApplyConfiguration](info)
	opts := buildOptions(trainJob)

	// The TrainJob-level (level 1) model builds a single PodGroupTemplate for every ReplicatedJob.
	if len(scheduling.ReplicatedJobs) == 0 {
		rJobNames := make([]string, 0, len(info.TemplateSpec.PodSets))
		for _, ps := range info.TemplateSpec.PodSets {
			rJobNames = append(rJobNames, ps.Name)
		}
		item := newItem(trainJob.Name, info.TemplateSpec.PodSets, jobSetSpec, rJobNames, nil, workloadbuilder.WorkloadInput{
			Policy:         compositePolicyInput(scheduling.SchedulingPolicy),
			Constraints:    compositeConstraintsInput(scheduling.SchedulingConstraints),
			DisruptionMode: compositeDisruptionModeInput(scheduling.DisruptionMode),
		})
		return []*workloadbuilder.Builder{workloadbuilder.NewBuilder(item, opts)}
	}

	builders := make([]*workloadbuilder.Builder, 0, len(scheduling.ReplicatedJobs))
	for _, entry := range scheduling.ReplicatedJobs {
		templateName, _ := podGroupTemplateName(trainJob, scheduling, entry.TargetReplicatedJobs[0])
		var item *workloadbuilder.WorkloadItem
		if entry.Job != nil {
			// The Job-level (level 3) gang size is the number of Pods of a single Job.
			var minCount int32
			if ps := info.TemplateSpec.PodSets.FindByName(entry.TargetReplicatedJobs[0]); ps != nil {
				minCount = ptr.Deref(ps.Count, 1)
			}
			item = newItem(templateName, info.TemplateSpec.PodSets, jobSetSpec, entry.TargetReplicatedJobs, &minCount, workloadbuilder.WorkloadInput{
				Policy:         workloadbuilder.PolicyInput{PodGroupData: entry.Job.SchedulingPolicy.DeepCopy()},
				Constraints:    workloadbuilder.ConstraintsInput{PodGroupData: entry.Job.SchedulingConstraints.DeepCopy()},
				DisruptionMode: workloadbuilder.DisruptionModeInput{PodGroupData: entry.Job.DisruptionMode.DeepCopy()},
				ResourceClaims: workloadbuilder.ResourceClaimsInput{PodGroupData: entry.Job.ResourceClaims},
			})
		} else {
			item = newItem(templateName, info.TemplateSpec.PodSets, jobSetSpec, entry.TargetReplicatedJobs, nil, workloadbuilder.WorkloadInput{
				Policy:         compositePolicyInput(entry.SchedulingPolicy),
				Constraints:    compositeConstraintsInput(entry.SchedulingConstraints),
				DisruptionMode: compositeDisruptionModeInput(entry.DisruptionMode),
			})
		}
		builders = append(builders, workloadbuilder.NewBuilder(item, opts))
	}
	return builders
}

// newItem returns the WorkloadItem of a PodGroupTemplate for the given ReplicatedJobs, which
// defaults to the Gang scheduling policy. The gang size defaults to the number of Pods of the
// ReplicatedJobs, excluding the ReplicatedJobs which depend on another ReplicatedJob of the same
// PodGroupTemplate, since JobSet creates them only after their dependencies are ready.
func newItem(
	name string,
	podSets runtime.PodSets,
	jobSetSpec *jobsetv1alpha2ac.JobSetSpecApplyConfiguration,
	rJobNames []string,
	minCount *int32,
	input workloadbuilder.WorkloadInput,
) *workloadbuilder.WorkloadItem {
	if minCount == nil {
		var podCount int32
		for _, rJobName := range rJobNames {
			ps := podSets.FindByName(rJobName)
			if ps == nil {
				continue
			}
			replicas := int32(1)
			if rJob := findReplicatedJob(jobSetSpec, rJobName); rJob != nil {
				if slices.ContainsFunc(rJob.DependsOn, func(dep jobsetv1alpha2ac.DependsOnApplyConfiguration) bool {
					return slices.Contains(rJobNames, ptr.Deref(dep.Name, ""))
				}) {
					continue
				}
				replicas = ptr.Deref(rJob.Replicas, 1)
			}
			podCount += ptr.Deref(ps.Count, 1) * replicas
		}
		minCount = &podCount
	}

	var priorityClassName string
	if rJob := findReplicatedJob(jobSetSpec, rJobNames[0]); rJob != nil {
		priorityClassName = ptr.Deref(rJob.Template.Spec.Template.Spec.PriorityClassName, "")
	}

	return &workloadbuilder.WorkloadItem{
		Name: name,
		DefaultConfig: &workloadbuilder.SchedulingConfig{
			Policy:            &workloadbuilder.SchedulingPolicy{Gang: &workloadbuilder.GangSchedulingPolicy{}},
			PriorityClassName: priorityClassName,
		},
		Input: input,
		Callbacks: []workloadbuilder.SchedulingConfigFunc{
			func(cfg *workloadbuilder.SchedulingConfig) {
				if cfg.Policy != nil && cfg.Policy.Gang != nil && cfg.Policy.Gang.MinCount == nil {
					cfg.Policy.Gang.MinCount = minCount
				}
			},
		},
	}
}

// findReplicatedJob finds the ReplicatedJob with the given name in the JobSet spec.
func findReplicatedJob(jobSetSpec *jobsetv1alpha2ac.JobSetSpecApplyConfiguration, name string) *jobsetv1alpha2ac.ReplicatedJobApplyConfiguration {
	if jobSetSpec == nil {
		return nil
	}
	if idx := slices.IndexFunc(jobSetSpec.ReplicatedJobs, func(rJob jobsetv1alpha2ac.ReplicatedJobApplyConfiguration) bool {
		return ptr.Deref(rJob.Name, "") == name
	}); idx != -1 {
		return &jobSetSpec.ReplicatedJobs[idx]
	}
	return nil
}

// buildOptions returns the BuildOptions of the Workload owned by the TrainJob.
func buildOptions(trainJob *trainer.TrainJob) workloadbuilder.BuildOptions {
	return workloadbuilder.BuildOptions{
		Name:      workloadName(trainJob),
		Namespace: trainJob.Namespace,
		Owner: &metav1.OwnerReference{
			APIVersion:         trainer.GroupVersion.String(),
			Kind:               trainer.TrainJobKind,
			Name:               trainJob.Name,
			UID:                trainJob.UID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
		AllowedPolicies:        []workloadbuilder.SchedulingPolicyOption{workloadbuilder.BasicPolicy, workloadbuilder.GangPolicy},
		AllowedDisruptionModes: []workloadbuilder.DisruptionModeOption{workloadbuilder.SingleMode, workloadbuilder.AllMode},
	}
}

// buildWorkload compiles the Workload from the Builders. Every Builder compiles a Workload with a
// single PodGroupTemplate, so the PodGroupTemplates are merged into a single Workload.
func buildWorkload(builders []*workloadbuilder.Builder) (*schedulingv1beta1.Workload, error) {
	if len(builders) == 0 {
		return nil, errors.New("no PodGroupTemplates were compiled from spec.scheduling")
	}
	workload, err := builders[0].BuildWorkload()
	if err != nil {
		return nil, err
	}
	workload = workload.DeepCopy()
	for _, b := range builders[1:] {
		wl, err := b.BuildWorkload()
		if err != nil {
			return nil, err
		}
		if !equality.Semantic.DeepEqual(wl.Spec.ControllerRef, workload.Spec.ControllerRef) {
			return nil, fmt.Errorf("cannot merge Workloads with different controller references")
		}
		workload.Spec.PodGroupTemplates = append(workload.Spec.PodGroupTemplates, wl.Spec.PodGroupTemplates...)
	}
	return workload, nil
}

// compositePolicyInput maps the composite scheduling policy to the PodGroup scheduling policy,
// since CompositePodGroups are not created yet. The gang size is computed from the Pod count.
func compositePolicyInput(policy *schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy) workloadbuilder.PolicyInput {
	if policy == nil {
		return workloadbuilder.PolicyInput{}
	}
	podGroupPolicy := &schedulingv1alpha3.WorkloadPodGroupSchedulingPolicy{}
	if policy.Basic != nil {
		podGroupPolicy.Basic = &schedulingv1alpha3.WorkloadPodGroupBasicSchedulingPolicy{}
	}
	if policy.Gang != nil {
		podGroupPolicy.Gang = &schedulingv1alpha3.WorkloadPodGroupGangSchedulingPolicy{}
	}
	return workloadbuilder.PolicyInput{PodGroupData: podGroupPolicy}
}

// compositeConstraintsInput maps the composite scheduling constraints to the PodGroup scheduling
// constraints, since CompositePodGroups are not created yet.
func compositeConstraintsInput(constraints *schedulingv1alpha3.WorkloadCompositePodGroupSchedulingConstraints) workloadbuilder.ConstraintsInput {
	if constraints == nil {
		return workloadbuilder.ConstraintsInput{}
	}
	return workloadbuilder.ConstraintsInput{
		PodGroupData: &schedulingv1alpha3.WorkloadPodGroupSchedulingConstraints{
			Topology: slices.Clone(constraints.Topology),
		},
	}
}

// compositeDisruptionModeInput maps the composite disruption mode to the PodGroup disruption mode,
// since CompositePodGroups are not created yet.
func compositeDisruptionModeInput(disruptionMode *schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode) workloadbuilder.DisruptionModeInput {
	if disruptionMode == nil {
		return workloadbuilder.DisruptionModeInput{}
	}
	podGroupDisruptionMode := &schedulingv1alpha3.WorkloadPodGroupDisruptionMode{}
	if disruptionMode.Single != nil {
		podGroupDisruptionMode.Single = &schedulingv1alpha3.WorkloadPodGroupSingleDisruptionMode{}
	}
	if disruptionMode.All != nil {
		podGroupDisruptionMode.All = &schedulingv1alpha3.WorkloadPodGroupAllDisruptionMode{}
	}
	return workloadbuilder.DisruptionModeInput{PodGroupData: podGroupDisruptionMode}
}
