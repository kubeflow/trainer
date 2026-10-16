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
	"slices"

	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-helpers/scheduling/schedulingv1/workloadbuilder"
	"k8s.io/utils/ptr"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

// buildWorkload compiles the Workload of the TrainJob, or returns nil when the runtime has no
// ReplicatedJobs to schedule. Every PodGroupTemplate is compiled by its own Builder, since a Builder
// compiles a single PodGroupTemplate, and the PodGroupTemplates are merged into a single Workload.
func buildWorkload(info *runtime.Info, trainJob *trainer.TrainJob) (*schedulingv1beta1.Workload, error) {
	scheduling := info.RuntimePolicy.Scheduling
	podSets := info.TemplateSpec.PodSets
	if len(podSets) == 0 {
		return nil, nil
	}
	jobSetSpec, _ := runtime.TemplateSpecApply[jobsetv1alpha2ac.JobSetSpecApplyConfiguration](info)

	var items []*workloadbuilder.WorkloadItem
	if len(scheduling.ReplicatedJobs) == 0 {
		// The TrainJob-level (level 1) model builds a single PodGroupTemplate for every ReplicatedJob.
		rJobNames := make([]string, 0, len(podSets))
		for _, ps := range podSets {
			rJobNames = append(rJobNames, ps.Name)
		}
		items = append(items, newItem(trainJob.Name, minCount(podSets, jobSetSpec, rJobNames), priorityClassName(jobSetSpec, rJobNames[0]),
			workloadbuilder.WorkloadInput{
				Policy:         compositePolicyInput(scheduling.SchedulingPolicy),
				Constraints:    compositeConstraintsInput(scheduling.SchedulingConstraints),
				DisruptionMode: compositeDisruptionModeInput(scheduling.DisruptionMode),
			}))
	}
	for _, entry := range scheduling.ReplicatedJobs {
		templateName, _ := podGroupTemplateName(trainJob, scheduling, entry.TargetReplicatedJobs[0])
		if entry.Job != nil {
			// The Job-level (level 3) gang size is the number of Pods of a single Job.
			target := entry.TargetReplicatedJobs[0]
			var jobPodCount int32
			if ps := podSets.FindByName(target); ps != nil {
				jobPodCount = ptr.Deref(ps.Count, 1)
			}
			items = append(items, newItem(templateName, jobPodCount, priorityClassName(jobSetSpec, target), workloadbuilder.WorkloadInput{
				Policy:         workloadbuilder.PolicyInput{PodGroupData: entry.Job.SchedulingPolicy},
				Constraints:    workloadbuilder.ConstraintsInput{PodGroupData: entry.Job.SchedulingConstraints},
				DisruptionMode: workloadbuilder.DisruptionModeInput{PodGroupData: entry.Job.DisruptionMode},
				ResourceClaims: workloadbuilder.ResourceClaimsInput{PodGroupData: entry.Job.ResourceClaims},
			}))
			continue
		}
		// The runtime webhook ensures that the targets sharing the PodGroup use the same priorityClassName.
		items = append(items, newItem(templateName, minCount(podSets, jobSetSpec, entry.TargetReplicatedJobs),
			priorityClassName(jobSetSpec, entry.TargetReplicatedJobs[0]), workloadbuilder.WorkloadInput{
				Policy:         compositePolicyInput(entry.SchedulingPolicy),
				Constraints:    compositeConstraintsInput(entry.SchedulingConstraints),
				DisruptionMode: compositeDisruptionModeInput(entry.DisruptionMode),
			}))
	}

	var workload *schedulingv1beta1.Workload
	for _, item := range items {
		wl, err := workloadbuilder.NewBuilder(item, buildOptions(trainJob)).BuildWorkload()
		if err != nil {
			return nil, err
		}
		if workload == nil {
			workload = wl
			continue
		}
		workload.Spec.PodGroupTemplates = append(workload.Spec.PodGroupTemplates, wl.Spec.PodGroupTemplates...)
	}
	return workload, nil
}

// newItem returns the WorkloadItem of a PodGroupTemplate, which defaults to the Gang scheduling
// policy with the given minCount.
func newItem(name string, minCount int32, priorityClassName string, input workloadbuilder.WorkloadInput) *workloadbuilder.WorkloadItem {
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
					cfg.Policy.Gang.MinCount = &minCount
				}
			},
		},
	}
}

// minCount returns the gang size of a PodGroupTemplate for the given ReplicatedJobs, which is the
// number of Pods of the ReplicatedJobs. The ReplicatedJobs which depend on another ReplicatedJob of
// the same PodGroupTemplate are excluded.
func minCount(podSets runtime.PodSets, jobSetSpec *jobsetv1alpha2ac.JobSetSpecApplyConfiguration, rJobNames []string) int32 {
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
	return podCount
}

// priorityClassName returns the priorityClassName of the ReplicatedJob Pod template.
func priorityClassName(jobSetSpec *jobsetv1alpha2ac.JobSetSpecApplyConfiguration, rJobName string) string {
	if rJob := findReplicatedJob(jobSetSpec, rJobName); rJob != nil {
		return ptr.Deref(rJob.Template.Spec.Template.Spec.PriorityClassName, "")
	}
	return ""
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

// compositePolicyInput maps the composite scheduling policy to the PodGroup scheduling policy,
// since CompositePodGroups are not created yet. The gang size is computed from the Pod count.
// TODO (andreyvelich): This needs to be refactored once we support CPG.
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
			Topology: constraints.Topology,
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
