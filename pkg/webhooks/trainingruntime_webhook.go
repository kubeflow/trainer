/*
Copyright 2024 The Kubeflow Authors.

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

package webhooks

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/workloadawarescheduling"
	trainingruntime "github.com/kubeflow/trainer/v2/pkg/util/trainingruntime"
)

const (
	rJobReplicasErrorMsg       = "must be greater than or equal to 1"
	rJobContainerNamesErrorMsg = "must contain the required container for the ancestor: %s"
	rJobContainerClaimErrorMsg = "references resourceClaim %q which is not defined in the Pod's resourceClaims"

	schedulingFeatureGateErrorMsg      = "TrainJobWorkloadAwareScheduling feature gate is disabled"
	schedulingTargetNotFoundErrorMsg   = "must reference a replicatedJob in spec.template.spec.replicatedJobs"
	schedulingUntargetedErrorMsg       = "must be targeted by a spec.scheduling.replicatedJobs entry when spec.scheduling.replicatedJobs is set"
	schedulingJobReplicasErrorMsg      = "must target a ReplicatedJob with at most 1 replicas; a PodGroup per replica is not supported yet"
	schedulingJobClaimErrorMsg         = "must have a Pod resourceClaim with the same name, resourceClaimName, and resourceClaimTemplateName in the targeted ReplicatedJob"
	schedulingTrainJobPriorityErrorMsg = "must match %q of the other ReplicatedJobs sharing the TrainJob-level PodGroup"
	schedulingGroupPriorityErrorMsg    = "priorityClassName %q must match %q of the other ReplicatedJobs sharing this PodGroup"
)

var (
	expectedContainerNames = map[string]string{
		constants.AncestorTrainer:    constants.Node,
		constants.ModelInitializer:   constants.ModelInitializer,
		constants.DatasetInitializer: constants.DatasetInitializer,
	}
)

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-trainingruntime,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=trainingruntimes,verbs=create;update,versions=v1alpha1,name=validator.trainingruntime.trainer.kubeflow.org,admissionReviewVersions=v1

// TrainingRuntimeValidator validates TrainingRuntimes
type TrainingRuntimeValidator struct{}

var _ admission.Validator[*trainer.TrainingRuntime] = (*TrainingRuntimeValidator)(nil)

func setupWebhookForTrainingRuntime(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.TrainingRuntime{}).
		WithValidator(&TrainingRuntimeValidator{}).
		Complete()
}

func (w *TrainingRuntimeValidator) ValidateCreate(ctx context.Context, obj *trainer.TrainingRuntime) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainingruntime-webhook")
	log.V(5).Info("Validating create", "trainingRuntime", klog.KObj(obj))
	var warnings admission.Warnings
	if trainingruntime.IsSupportDeprecated(obj.Labels) {
		warnings = append(warnings, fmt.Sprintf(
			"TrainingRuntime \"%s\" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: %s",
			obj.Name,
			constants.RuntimeDeprecationPolicyURL,
		))
	}
	allErrs := validateReplicatedJobs(obj.Spec.Template.Spec.ReplicatedJobs)
	allErrs = append(allErrs, validateScheduling(ctx, &obj.Spec, nil)...)
	return warnings, allErrs.ToAggregate()
}

func validateReplicatedJobs(rJobs []jobsetv1alpha2.ReplicatedJob) field.ErrorList {
	ancestors := sets.New(constants.AncestorTrainer, constants.ModelInitializer, constants.DatasetInitializer)
	rJobsPath := field.NewPath("spec").
		Child("template").
		Child("spec").
		Child("replicatedJobs")
	var allErrs field.ErrorList
	for idx, rJob := range rJobs {
		// Every container resources.claims entry must reference a Pod-level resourceClaims entry.
		// Otherwise every TrainJob would have to add the Pod-level claim itself to use the runtime.
		podSpec := &rJob.Template.Spec.Template.Spec
		podSpecPath := rJobsPath.Index(idx).Child("template", "spec", "template", "spec")
		podClaims := sets.New[string]()
		for _, pc := range podSpec.ResourceClaims {
			podClaims.Insert(pc.Name)
		}
		validateClaims := func(containersField string, containers []corev1.Container) {
			for k, c := range containers {
				for m, claim := range c.Resources.Claims {
					if !podClaims.Has(claim.Name) {
						allErrs = append(allErrs, field.Invalid(
							podSpecPath.Child(containersField).Index(k).Child("resources", "claims").Index(m),
							claim.Name, fmt.Sprintf(rJobContainerClaimErrorMsg, claim.Name)))
					}
				}
			}
		}
		validateClaims("initContainers", podSpec.InitContainers)
		validateClaims("containers", podSpec.Containers)

		// The CRD schema allows 0 replicas, which would create a JobSet without any Pods for the replicated Job.
		if rJob.Replicas < 1 {
			allErrs = append(allErrs, field.Invalid(rJobsPath.Index(idx).Child("replicas"), rJob.Replicas, rJobReplicasErrorMsg))
		}

		if rJob.Template.Labels == nil {
			continue
		}

		if labelAncestor, ok := rJob.Template.Labels[constants.LabelTrainJobAncestor]; ok && ancestors.Has(labelAncestor) {
			// Validate replicated job contains the required containers.
			// Mapping of the ancestor labels to the containers:
			// 1. dataset-initializer - dataset-initializer
			// 2. model-initializer - model-initializer
			// 3. trainer - node
			hasRequiredContainer := false
			for _, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == expectedContainerNames[labelAncestor] {
					hasRequiredContainer = true
					break
				}
			}
			if !hasRequiredContainer {
				allErrs = append(allErrs, field.Invalid(
					rJobsPath.Index(idx).Child("template").Child("spec").Child("template").Child("spec").Child("containers"),
					rJob.Template.Spec.Template.Spec.Containers,
					fmt.Sprintf(rJobContainerNamesErrorMsg, labelAncestor),
				))
			}
		}
	}
	return allErrs
}

func validateScheduling(ctx context.Context, newSpec, oldSpec *trainer.TrainingRuntimeSpec) field.ErrorList {
	scheduling := newSpec.Scheduling
	if scheduling == nil {
		return nil
	}
	var allErrs field.ErrorList
	schedulingPath := field.NewPath("spec", "scheduling")
	rJobsPath := field.NewPath("spec", "template", "spec", "replicatedJobs")

	// A runtime stored while the feature gate was enabled stays updatable after the gate is
	// disabled, as long as its scheduling configuration doesn't change.
	if !features.Enabled(features.TrainJobWorkloadAwareScheduling) {
		if oldSpec == nil || !equality.Semantic.DeepEqual(oldSpec.Scheduling, scheduling) {
			allErrs = append(allErrs, field.Forbidden(schedulingPath, schedulingFeatureGateErrorMsg))
		}
		return allErrs
	}

	// Run the validation from the workload builder library.
	allErrs = append(allErrs, workloadawarescheduling.ValidateScheduling(ctx, newSpec)...)

	rJobs := newSpec.Template.Spec.ReplicatedJobs
	if len(scheduling.ReplicatedJobs) == 0 {
		// All ReplicatedJobs share the TrainJob-level (level 1) PodGroup, which has a single priority.
		for i := 1; i < len(rJobs); i++ {
			expected := rJobs[0].Template.Spec.Template.Spec.PriorityClassName
			if actual := rJobs[i].Template.Spec.Template.Spec.PriorityClassName; actual != expected {
				allErrs = append(allErrs, field.Invalid(
					rJobsPath.Index(i).Child("template", "spec", "template", "spec", "priorityClassName"),
					actual, fmt.Sprintf(schedulingTrainJobPriorityErrorMsg, expected)))
			}
		}
		return allErrs
	}

	replicas := make(map[string]int32, len(rJobs))
	priorityClassNames := make(map[string]string, len(rJobs))
	podClaims := make(map[string][]corev1.PodResourceClaim, len(rJobs))
	for _, rJob := range rJobs {
		replicas[rJob.Name] = rJob.Replicas
		priorityClassNames[rJob.Name] = rJob.Template.Spec.Template.Spec.PriorityClassName
		podClaims[rJob.Name] = rJob.Template.Spec.Template.Spec.ResourceClaims
	}
	targeted := sets.New[string]()
	for i, entry := range scheduling.ReplicatedJobs {
		targetsPath := schedulingPath.Child("replicatedJobs").Index(i).Child("targetReplicatedJobs")
		for j, name := range entry.TargetReplicatedJobs {
			if _, ok := replicas[name]; !ok {
				allErrs = append(allErrs, field.Invalid(targetsPath.Index(j), name, schedulingTargetNotFoundErrorMsg))
			}
			targeted.Insert(name)
		}
		// A ReplicatedJob has a single Pod template shared by all of its replicas (Jobs), so a
		// distinct PodGroup can't be assigned to the Pods of each replica yet.
		// TODO (andreyvelich): We need to use scheduling.k8s.io/workload-name Job annotation in k8s v1.38
		if entry.Job != nil && len(entry.TargetReplicatedJobs) == 1 && replicas[entry.TargetReplicatedJobs[0]] > 1 {
			allErrs = append(allErrs, field.Invalid(schedulingPath.Child("replicatedJobs").Index(i).Child("job"),
				replicas[entry.TargetReplicatedJobs[0]], schedulingJobReplicasErrorMsg))
		}
		// A Pod uses a PodGroup ResourceClaim only when it declares a Pod resourceClaim referencing the
		// same claim, so every Job-level claim must be declared in the Pod template of its target.
		if entry.Job != nil && len(entry.TargetReplicatedJobs) == 1 {
			for k, claim := range entry.Job.ResourceClaims {
				if !slices.ContainsFunc(podClaims[entry.TargetReplicatedJobs[0]], func(podClaim corev1.PodResourceClaim) bool {
					return podClaim.Name == claim.Name &&
						ptr.Equal(podClaim.ResourceClaimName, claim.ResourceClaimName) &&
						ptr.Equal(podClaim.ResourceClaimTemplateName, claim.ResourceClaimTemplateName)
				}) {
					allErrs = append(allErrs, field.Invalid(schedulingPath.Child("replicatedJobs").Index(i).Child("job", "resourceClaims").Index(k),
						claim.Name, schedulingJobClaimErrorMsg))
				}
			}
		}
		// The targets of a per-ReplicatedJob (level 2) entry share a PodGroup, which has a single priority.
		if entry.Job == nil {
			expected := priorityClassNames[entry.TargetReplicatedJobs[0]]
			for j, name := range entry.TargetReplicatedJobs[1:] {
				if actual, ok := priorityClassNames[name]; ok && actual != expected {
					allErrs = append(allErrs, field.Invalid(targetsPath.Index(j+1), name,
						fmt.Sprintf(schedulingGroupPriorityErrorMsg, actual, expected)))
				}
			}
		}
	}

	// There is no TrainJob-level policy for an untargeted ReplicatedJob to fall back to.
	for i, rJob := range rJobs {
		if !targeted.Has(rJob.Name) {
			allErrs = append(allErrs, field.Invalid(rJobsPath.Index(i).Child("name"), rJob.Name, schedulingUntargetedErrorMsg))
		}
	}
	return allErrs
}

func (w *TrainingRuntimeValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.TrainingRuntime) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainingruntime-webhook")
	log.V(5).Info("Validating update", "trainingRuntime", klog.KObj(newObj))
	allErrs := validateReplicatedJobs(newObj.Spec.Template.Spec.ReplicatedJobs)
	allErrs = append(allErrs, validateScheduling(ctx, &newObj.Spec, &oldObj.Spec)...)
	return nil, allErrs.ToAggregate()
}

func (w *TrainingRuntimeValidator) ValidateDelete(ctx context.Context, obj *trainer.TrainingRuntime) (admission.Warnings, error) {
	return nil, nil
}
