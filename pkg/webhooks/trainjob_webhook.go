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
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

// +kubebuilder:webhook:path=/mutate-trainer-kubeflow-org-v1alpha1-trainjob,mutating=true,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=trainjobs,verbs=create;update,versions=v1alpha1,name=defaulter.trainjob.trainer.kubeflow.org,admissionReviewVersions=v1

// TrainJobDefaulter defaults TrainJobs.
type TrainJobDefaulter struct {
	clock clock.PassiveClock
}

var _ admission.Defaulter[*trainer.TrainJob] = (*TrainJobDefaulter)(nil)

func (d *TrainJobDefaulter) Default(ctx context.Context, trainJob *trainer.TrainJob) error {
	log := ctrl.LoggerFrom(ctx).WithName("trainJob-webhook")
	log.V(5).Info("Defaulting", "TrainJob", klog.KObj(trainJob))

	now := metav1.NewTime(d.clock.Now())

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	var oldObj *trainer.TrainJob
	if req.Operation == admissionv1.Update {
		oldObj = &trainer.TrainJob{}
		if err := json.Unmarshal(req.OldObject.Raw, oldObj); err != nil {
			return err
		}
	}

	if oldObj == nil {
		for i := range trainJob.Spec.RuntimePatches {
			if trainJob.Spec.RuntimePatches[i].Time == nil {
				trainJob.Spec.RuntimePatches[i].Time = &now
			}
		}
		return nil
	}

	oldByManager := make(map[string]trainer.RuntimePatch, len(oldObj.Spec.RuntimePatches))
	for _, p := range oldObj.Spec.RuntimePatches {
		oldByManager[p.Manager] = p
	}
	for i := range trainJob.Spec.RuntimePatches {
		patch := &trainJob.Spec.RuntimePatches[i]
		if old, ok := oldByManager[patch.Manager]; ok {
			oldCmp, newCmp := old, *patch
			oldCmp.Time, newCmp.Time = nil, nil
			if equality.Semantic.DeepEqual(oldCmp, newCmp) {
				patch.Time = old.Time
			} else {
				patch.Time = &now
			}
		} else if patch.Time == nil {
			patch.Time = &now
		}
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-trainjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=trainjobs,verbs=create;update,versions=v1alpha1,name=validator.trainjob.trainer.kubeflow.org,admissionReviewVersions=v1

// TrainJobValidator validates TrainJobs
type TrainJobValidator struct {
	runtimes map[string]runtime.Runtime
}

var _ admission.Validator[*trainer.TrainJob] = (*TrainJobValidator)(nil)

func setupWebhookForTrainJob(mgr ctrl.Manager, run map[string]runtime.Runtime) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.TrainJob{}).
		WithDefaulter(&TrainJobDefaulter{clock: clock.RealClock{}}).
		WithValidator(&TrainJobValidator{runtimes: run}).
		Complete()
}

func (w *TrainJobValidator) ValidateCreate(ctx context.Context, obj *trainer.TrainJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainJob-webhook")
	log.V(5).Info("Validating create", "TrainJob", klog.KObj(obj))

	if !features.Enabled(features.TrainJobDynamicResourceAllocation) && usesDRA(obj) {
		return nil, draFieldPaths(obj).ToAggregate()
	}

	runtimeRefGK := runtime.RuntimeRefToRuntimeRegistryKey(obj.Spec.RuntimeRef)
	runtime, ok := w.runtimes[runtimeRefGK]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
	}
	warnings, errors := runtime.ValidateObjects(ctx, nil, obj)
	return warnings, append(validatePodFields(obj), errors...).ToAggregate()
}

func (w *TrainJobValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.TrainJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainJob-webhook")
	log.V(5).Info("Validating update", "TrainJob", klog.KObj(newObj))

	// spec.trainer is immutable, but spec.runtimePatches entries can still be appended on update,
	// so only reject an update that introduces DRA fields; a TrainJob created with them while the
	// gate was enabled stays updatable (e.g. suspend) after the gate is disabled.
	if !features.Enabled(features.TrainJobDynamicResourceAllocation) && usesDRA(newObj) && !usesDRA(oldObj) {
		return nil, draFieldPaths(newObj).ToAggregate()
	}

	runtimeRefGK := runtime.RuntimeRefToRuntimeRegistryKey(newObj.Spec.RuntimeRef)
	runtime, ok := w.runtimes[runtimeRefGK]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
	}
	warnings, errors := runtime.ValidateObjects(ctx, oldObj, newObj)
	// Only reject the update when it introduces invalid pod fields, so a TrainJob admitted
	// before this check existed stays updatable (e.g. suspend or delete its finalizers).
	if len(validatePodFields(oldObj)) == 0 {
		errors = append(validatePodFields(newObj), errors...)
	}
	return warnings, errors.ToAggregate()
}

func (w *TrainJobValidator) ValidateDelete(ctx context.Context, obj *trainer.TrainJob) (admission.Warnings, error) {
	return nil, nil
}

// usesDRA reports whether the TrainJob references any DRA fields.
func usesDRA(trainJob *trainer.TrainJob) bool {
	if trainJob.Spec.Trainer != nil && len(trainJob.Spec.Trainer.ResourceClaimsPerNode) > 0 {
		return true
	}
	for _, patch := range trainJob.Spec.RuntimePatches {
		if patch.TrainingRuntimeSpec == nil || patch.TrainingRuntimeSpec.Template == nil ||
			patch.TrainingRuntimeSpec.Template.Spec == nil {
			continue
		}
		for _, rJob := range patch.TrainingRuntimeSpec.Template.Spec.ReplicatedJobs {
			if rJob.Template == nil || rJob.Template.Spec == nil ||
				rJob.Template.Spec.Template == nil || rJob.Template.Spec.Template.Spec == nil {
				continue
			}
			podSpec := rJob.Template.Spec.Template.Spec
			if len(podSpec.ResourceClaims) > 0 {
				return true
			}
			for _, c := range podSpec.Containers {
				if c.Resources != nil && len(c.Resources.Claims) > 0 {
					return true
				}
			}
			for _, c := range podSpec.InitContainers {
				if c.Resources != nil && len(c.Resources.Claims) > 0 {
					return true
				}
			}
		}
	}
	return false
}

// draFieldPaths returns field.Forbidden errors for every DRA field set on the TrainJob.
func draFieldPaths(trainJob *trainer.TrainJob) field.ErrorList {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")
	msg := "TrainJobDynamicResourceAllocation feature gate is disabled"

	if trainJob.Spec.Trainer != nil && len(trainJob.Spec.Trainer.ResourceClaimsPerNode) > 0 {
		allErrs = append(allErrs, field.Forbidden(
			specPath.Child("trainer", "resourceClaimsPerNode"), msg,
		))
	}

	for i, patch := range trainJob.Spec.RuntimePatches {
		if patch.TrainingRuntimeSpec == nil || patch.TrainingRuntimeSpec.Template == nil ||
			patch.TrainingRuntimeSpec.Template.Spec == nil {
			continue
		}
		for j, rJob := range patch.TrainingRuntimeSpec.Template.Spec.ReplicatedJobs {
			if rJob.Template == nil || rJob.Template.Spec == nil ||
				rJob.Template.Spec.Template == nil || rJob.Template.Spec.Template.Spec == nil {
				continue
			}
			podSpec := rJob.Template.Spec.Template.Spec
			rJobPath := specPath.Child("runtimePatches").Index(i).
				Child("trainingRuntimeSpec", "template", "spec", "replicatedJobs").Index(j).
				Child("template", "spec", "template", "spec")

			if len(podSpec.ResourceClaims) > 0 {
				allErrs = append(allErrs, field.Forbidden(
					rJobPath.Child("resourceClaims"), msg,
				))
			}
			for k, c := range podSpec.Containers {
				if c.Resources != nil && len(c.Resources.Claims) > 0 {
					allErrs = append(allErrs, field.Forbidden(
						rJobPath.Child("containers").Index(k).Child("resources", "claims"), msg,
					))
				}
			}
			for k, c := range podSpec.InitContainers {
				if c.Resources != nil && len(c.Resources.Claims) > 0 {
					allErrs = append(allErrs, field.Forbidden(
						rJobPath.Child("initContainers").Index(k).Child("resources", "claims"), msg,
					))
				}
			}
		}
	}
	return allErrs
}

// validatePodFields rejects env names and resource requirements that the API server would
// reject on the Pods. Without this check the TrainJob is admitted, but JobSet fails to create
// its Jobs and the TrainJob never starts.
func validatePodFields(trainJob *trainer.TrainJob) field.ErrorList {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	if trainJob.Spec.Trainer != nil {
		trainerPath := specPath.Child("trainer")
		allErrs = append(allErrs, validateEnvNames(trainJob.Spec.Trainer.Env, trainerPath.Child("env"))...)
		allErrs = append(allErrs, validateResourceRequirements(trainJob.Spec.Trainer.ResourcesPerNode, trainerPath.Child("resourcesPerNode"))...)
	}
	if trainJob.Spec.Initializer != nil {
		initializerPath := specPath.Child("initializer")
		if trainJob.Spec.Initializer.Dataset != nil {
			allErrs = append(allErrs, validateEnvNames(trainJob.Spec.Initializer.Dataset.Env, initializerPath.Child("dataset", "env"))...)
		}
		if trainJob.Spec.Initializer.Model != nil {
			allErrs = append(allErrs, validateEnvNames(trainJob.Spec.Initializer.Model.Env, initializerPath.Child("model", "env"))...)
		}
	}

	for i, patch := range trainJob.Spec.RuntimePatches {
		if patch.TrainingRuntimeSpec == nil || patch.TrainingRuntimeSpec.Template == nil ||
			patch.TrainingRuntimeSpec.Template.Spec == nil {
			continue
		}
		for j, rJob := range patch.TrainingRuntimeSpec.Template.Spec.ReplicatedJobs {
			if rJob.Template == nil || rJob.Template.Spec == nil ||
				rJob.Template.Spec.Template == nil || rJob.Template.Spec.Template.Spec == nil {
				continue
			}
			podSpec := rJob.Template.Spec.Template.Spec
			rJobPath := specPath.Child("runtimePatches").Index(i).
				Child("trainingRuntimeSpec", "template", "spec", "replicatedJobs").Index(j).
				Child("template", "spec", "template", "spec")

			for k, c := range podSpec.Containers {
				containerPath := rJobPath.Child("containers").Index(k)
				allErrs = append(allErrs, validateEnvNames(c.Env, containerPath.Child("env"))...)
				allErrs = append(allErrs, validateResourceRequirements(c.Resources, containerPath.Child("resources"))...)
			}
			for k, c := range podSpec.InitContainers {
				containerPath := rJobPath.Child("initContainers").Index(k)
				allErrs = append(allErrs, validateEnvNames(c.Env, containerPath.Child("env"))...)
				allErrs = append(allErrs, validateResourceRequirements(c.Resources, containerPath.Child("resources"))...)
			}
		}
	}
	return allErrs
}

// validateEnvNames uses the relaxed env name rule, so it only rejects names that are invalid
// on every supported Kubernetes version.
func validateEnvNames(envs []corev1.EnvVar, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for i, env := range envs {
		for _, msg := range validation.IsRelaxedEnvVarName(env.Name) {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("name"), env.Name, msg))
		}
	}
	return allErrs
}

// validateResourceRequirements rejects negative quantities and requests above their limit,
// which the API server rejects on Pods. A negative request also reaches the PodGroup that the
// gang-scheduling plugins build from the per-Pod requests.
func validateResourceRequirements(resources *corev1.ResourceRequirements, fldPath *field.Path) field.ErrorList {
	if resources == nil {
		return nil
	}
	var allErrs field.ErrorList
	allErrs = append(allErrs, validateNonNegativeQuantities(resources.Requests, fldPath.Child("requests"))...)
	allErrs = append(allErrs, validateNonNegativeQuantities(resources.Limits, fldPath.Child("limits"))...)
	for name, limit := range resources.Limits {
		if request, ok := resources.Requests[name]; ok && request.Cmp(limit) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("requests").Key(string(name)), request.String(),
				fmt.Sprintf("must be less than or equal to %s limit of %s", name, limit.String())))
		}
	}
	return allErrs
}

func validateNonNegativeQuantities(resources corev1.ResourceList, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for _, name := range slices.Sorted(maps.Keys(resources)) {
		if quantity := resources[name]; quantity.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(string(name)), quantity.String(), "must be greater than or equal to 0"))
		}
	}
	return allErrs
}
