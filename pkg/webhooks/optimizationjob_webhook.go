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

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

// +kubebuilder:webhook:path=/mutate-trainer-kubeflow-org-v1alpha1-optimizationjob,mutating=true,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=create;update,versions=v1alpha1,name=defaulter.optimizationjob.trainer.kubeflow.org,admissionReviewVersions=v1

// OptimizationJobDefaulter defaults OptimizationJobs.
type OptimizationJobDefaulter struct{}

var _ admission.Defaulter[*trainer.OptimizationJob] = (*OptimizationJobDefaulter)(nil)

func (d *OptimizationJobDefaulter) Default(ctx context.Context, obj *trainer.OptimizationJob) error {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationJob-webhook")
	log.V(5).Info("Defaulting", "OptimizationJob", klog.KObj(obj))

	// 1. Default the Provider
	if obj.Spec.Algorithm.Provider == nil || *obj.Spec.Algorithm.Provider == "" {
		defaultProvider := "optuna"
		obj.Spec.Algorithm.Provider = &defaultProvider
	}

	// 2. Default ParallelTrials
	if obj.Spec.TrialConfig.ParallelTrials == nil {
		var defaultParallel int32 = 1
		obj.Spec.TrialConfig.ParallelTrials = &defaultParallel
	}

	// 3. Default NumTrials
	if obj.Spec.TrialConfig.NumTrials == nil {
		var defaultNum int32 = 1
		obj.Spec.TrialConfig.NumTrials = &defaultNum
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-optimizationjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=create;update,versions=v1alpha1,name=validator.optimizationjob.trainer.kubeflow.org,admissionReviewVersions=v1

// OptimizationJobValidator validates OptimizationJobs
type OptimizationJobValidator struct{}

var _ admission.Validator[*trainer.OptimizationJob] = (*OptimizationJobValidator)(nil)

// SetupWebhookForOptimizationJob registers the webhooks with the manager.
func SetupWebhookForOptimizationJob(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.OptimizationJob{}).
		WithDefaulter(&OptimizationJobDefaulter{}).
		WithValidator(&OptimizationJobValidator{}).
		Complete()
}

func (w *OptimizationJobValidator) ValidateCreate(ctx context.Context, obj *trainer.OptimizationJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationJob-webhook")
	log.V(5).Info("Validating create", "OptimizationJob", klog.KObj(obj))

	return nil, validateOptimizationJob(obj).ToAggregate()
}

func (w *OptimizationJobValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.OptimizationJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationJob-webhook")
	log.V(5).Info("Validating update", "OptimizationJob", klog.KObj(newObj))

	// Validation logic applies equally to updates to ensure the spec remains valid
	return nil, validateOptimizationJob(newObj).ToAggregate()
}

func (w *OptimizationJobValidator) ValidateDelete(ctx context.Context, obj *trainer.OptimizationJob) (admission.Warnings, error) {
	return nil, nil // Deletion does not require schema validation
}

// validateOptimizationJob aggregates all business-logic validation failures
func validateOptimizationJob(obj *trainer.OptimizationJob) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateTemplatePlaceholders(obj)...)

	// Add future validations here (e.g., checking early stopping compatibility)

	return allErrs
}

// validateTemplatePlaceholders ensures that any parameter declared in Spec.Parameters
// actually has a corresponding {{.parameter_name}} placeholder inside the TrainJobTemplate.
func validateTemplatePlaceholders(obj *trainer.OptimizationJob) field.ErrorList {
	var allErrs field.ErrorList

	// Serialize the TrainJobTemplate.Spec to easily search the raw text
	templateBytes, err := json.Marshal(obj.Spec.TrainJobTemplate.Spec)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(field.NewPath("spec", "trainJobTemplate"), err))
		return allErrs
	}
	templateStr := string(templateBytes)

	for i, param := range obj.Spec.Parameters {
		// Define the expected string template placeholders
		placeholder1 := fmt.Sprintf("{{.%s}}", param.Name)
		placeholder2 := fmt.Sprintf("{{ .%s }}", param.Name) // Account for spacing

		if !strings.Contains(templateStr, placeholder1) && !strings.Contains(templateStr, placeholder2) {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "parameters").Index(i).Child("name"),
				param.Name,
				fmt.Sprintf("Parameter '%s' is defined, but no placeholder (%s) was found in the trainJobTemplate. The controller will not be able to inject this value.", param.Name, placeholder1),
			))
		}
	}

	return allErrs
}
