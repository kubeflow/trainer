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

package webhooks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/util/optimizationjob"
)

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-optimizationjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=create,versions=v1alpha1,name=validator.optimizationjob.trainer.kubeflow.org,admissionReviewVersions=v1

// OptimizationJobValidator validates OptimizationJobs.
type OptimizationJobValidator struct {
	runtimes map[string]runtime.Runtime
}

var _ admission.Validator[*trainer.OptimizationJob] = (*OptimizationJobValidator)(nil)

func setupWebhookForOptimizationJob(mgr ctrl.Manager, runtimes map[string]runtime.Runtime) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.OptimizationJob{}).
		WithValidator(&OptimizationJobValidator{runtimes: runtimes}).
		Complete()
}

// ValidateCreate rejects new OptimizationJobs while the OptimizationJob feature gate is disabled,
// because no controller reconciles them. Updates and deletions are not validated, so existing
// OptimizationJobs can still be removed after the feature gate is disabled.
func (w *OptimizationJobValidator) ValidateCreate(ctx context.Context, obj *trainer.OptimizationJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationJob-webhook")
	log.V(5).Info("Validating create", "OptimizationJob", klog.KObj(obj))

	if !features.Enabled(features.OptimizationJob) {
		return nil, field.ErrorList{
			field.Forbidden(field.NewPath("spec"), "OptimizationJob feature gate is disabled"),
		}.ToAggregate()
	}

	var allErrs field.ErrorList

	for _, err := range validation.IsDNS1035Label(obj.Name) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "name"), obj.Name, err))
	}
	if len(allErrs) > 0 {
		return nil, allErrs.ToAggregate()
	}

	trialName := optimizationjob.GenerateTrialName(obj.Name)
	syntheticTrainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   obj.Namespace,
			Name:        trialName,
			Labels:      obj.Spec.TrainJobTemplate.Labels,
			Annotations: obj.Spec.TrainJobTemplate.Annotations,
		},
		Spec: obj.Spec.TrainJobTemplate.Spec,
	}

	runtimeRefGK := runtime.RuntimeRefToRuntimeRegistryKey(syntheticTrainJob.Spec.RuntimeRef)
	r, ok := w.runtimes[runtimeRefGK]
	if !ok {
		return nil, append(allErrs, field.Invalid(
			field.NewPath("spec", "trainJobTemplate", "spec", "runtimeRef"),
			syntheticTrainJob.Spec.RuntimeRef,
			fmt.Sprintf("unsupported runtime: %s", runtimeRefGK),
		)).ToAggregate()
	}

	warnings, trialErrs := r.ValidateObjects(ctx, nil, syntheticTrainJob)
	for _, err := range trialErrs {
		if err.Field == "metadata.name" {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("metadata", "name"),
				obj.Name,
				fmt.Sprintf("OptimizationJob name is too long: trial TrainJob %q exceeds runtime limits: %s", trialName, err.Detail),
			))
		} else {
			cloneErr := *err
			cloneErr.Field = field.NewPath("spec", "trainJobTemplate").Child(err.Field).String()
			allErrs = append(allErrs, &cloneErr)
		}
	}

	return warnings, allErrs.ToAggregate()
}

func (w *OptimizationJobValidator) ValidateUpdate(context.Context, *trainer.OptimizationJob, *trainer.OptimizationJob) (admission.Warnings, error) {
	return nil, nil
}

func (w *OptimizationJobValidator) ValidateDelete(context.Context, *trainer.OptimizationJob) (admission.Warnings, error) {
	return nil, nil
}
