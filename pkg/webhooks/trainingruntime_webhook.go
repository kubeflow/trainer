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

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	trainingruntime "github.com/kubeflow/trainer/v2/pkg/util/trainingruntime"
)

const (
	rJobReplicasErrorMsg          = "always must be 1"
	rJobContainerNamesErrorMsg    = "must contain the required container for the ancestor: %s"
	rJobPriorityClassNameErrorMsg = "must not be set on this replicatedJob: every Pod in a gang-scheduled TrainJob is scheduled at a single priority, so only the replicatedJob labelled with the %q ancestor may set it"
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
	allErrs = append(allErrs, validatePriorityClassName(obj.Spec.Template.Spec.ReplicatedJobs)...)
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
		if rJob.Template.Labels == nil {
			continue
		}

		if labelAncestor, ok := rJob.Template.Labels[constants.LabelTrainJobAncestor]; ok && ancestors.Has(labelAncestor) {
			if rJob.Replicas != 1 {
				allErrs = append(allErrs, field.Invalid(rJobsPath.Index(idx).Child("replicas"), rJob.Replicas, rJobReplicasErrorMsg))
			}

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

// validatePriorityClassName ensures that at most one replicatedJob declares a priorityClassName,
// and that it is the one labelled as the trainer ancestor.
//
// A gang-scheduled TrainJob is scheduled as a single unit: the Volcano PodGroup built from these
// replicatedJobs carries exactly one priorityClassName for the whole gang. Allowing several
// replicatedJobs to declare different values leaves no well-defined priority for the gang, so the
// trainer ancestor is taken as the single source of truth and the plugin propagates its value to
// every Pod template.
func validatePriorityClassName(rJobs []jobsetv1alpha2.ReplicatedJob) field.ErrorList {
	rJobsPath := field.NewPath("spec").
		Child("template").
		Child("spec").
		Child("replicatedJobs")
	var allErrs field.ErrorList
	for idx, rJob := range rJobs {
		if rJob.Template.Spec.Template.Spec.PriorityClassName == "" {
			continue
		}
		if rJob.Template.Labels[constants.LabelTrainJobAncestor] == constants.AncestorTrainer {
			continue
		}
		allErrs = append(allErrs, field.Invalid(
			rJobsPath.Index(idx).Child("template").Child("spec").Child("template").Child("spec").Child("priorityClassName"),
			rJob.Template.Spec.Template.Spec.PriorityClassName,
			fmt.Sprintf(rJobPriorityClassNameErrorMsg, constants.AncestorTrainer),
		))
	}
	return allErrs
}

func (w *TrainingRuntimeValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.TrainingRuntime) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainingruntime-webhook")
	log.V(5).Info("Validating update", "trainingRuntime", klog.KObj(newObj))
	return nil, validateReplicatedJobs(newObj.Spec.Template.Spec.ReplicatedJobs).ToAggregate()
}

func (w *TrainingRuntimeValidator) ValidateDelete(ctx context.Context, obj *trainer.TrainingRuntime) (admission.Warnings, error) {
	return nil, nil
}
