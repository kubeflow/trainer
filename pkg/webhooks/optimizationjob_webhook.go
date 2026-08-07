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

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-optimizationjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=create;update,versions=v1alpha1,name=validator.optimizationjob.trainer.kubeflow.org,admissionReviewVersions=v1

// OptimizationJobValidator validates OptimizationJobs.
type OptimizationJobValidator struct{}

var _ admission.Validator[*trainer.OptimizationJob] = (*OptimizationJobValidator)(nil)

func setupWebhookForOptimizationJob(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.OptimizationJob{}).
		WithValidator(&OptimizationJobValidator{}).
		Complete()
}

func (w *OptimizationJobValidator) ValidateCreate(ctx context.Context, obj *trainer.OptimizationJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationjob-webhook")
	log.V(5).Info("Validating create", "optimizationJob", klog.KObj(obj))

	// Structural constraints (exactly-one algorithm, exactly-one search space
	// distribution, grid requires categorical parameters) are enforced by CEL.
	// The trial-budget check below is a fold over the parameter list, which CEL
	// cannot express, so it lives here.
	return nil, validateGridTrialBudget(&obj.Spec).ToAggregate()
}

// validateGridTrialBudget rejects a grid search whose numTrials exceeds the
// total number of parameter combinations. Grid enumerates the full cartesian
// product up front, so requesting more trials than combinations is
// unsatisfiable. This mirrors Katib's ValidateAlgorithmSettings, which rejects
// the Experiment before any trial is scheduled instead of letting the
// suggestion container fail later. It only runs for grid; random is unbounded.
func validateGridTrialBudget(spec *trainer.OptimizationJobSpec) field.ErrorList {
	var allErrs field.ErrorList

	if spec.SearchAlgorithm == nil || spec.SearchAlgorithm.Grid == nil {
		return allErrs
	}

	// CEL guarantees every parameter is categorical when grid is set, so the
	// number of combinations is the product of the choice-set sizes.
	//
	// numTrials is bounded (kubebuilder Maximum), so once the running product
	// reaches it the config is already satisfiable and we stop multiplying.
	// This is not just an optimization: the parameter list and each choice set
	// are only bounded by MaxItems (100 each today), so a naive product can be
	// as large as 100^100 and overflow int64, wrapping to a value that could
	// spuriously trip the check below. Stopping early keeps the product small.
	numTrials := int64(spec.NumTrials)
	combinations := int64(1)
	for i := range spec.Parameters {
		searchSpace := spec.Parameters[i].SearchSpace
		if searchSpace == nil || len(searchSpace.Categorical.Choices) == 0 {
			// A non-categorical parameter under grid should have been rejected
			// by CEL already; defer to that layer rather than guessing a count.
			return allErrs
		}
		combinations *= int64(len(searchSpace.Categorical.Choices))
		if combinations >= numTrials {
			// Enough combinations to satisfy the budget; further multiplication
			// can only grow the product (choice sets have MinItems=1), so it is
			// safe to stop and avoid any overflow.
			return allErrs
		}
	}

	if numTrials > combinations {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "numTrials"),
			numTrials,
			fmt.Sprintf("numTrials (%d) cannot exceed the %d possible grid combinations across %s; reduce numTrials or widen the search space", numTrials, combinations, field.NewPath("spec", "parameters").String()),
		))
	}

	return allErrs
}

func (w *OptimizationJobValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.OptimizationJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationjob-webhook")
	log.V(5).Info("Validating update", "optimizationJob", klog.KObj(newObj))

	// The spec is immutable, which the CEL rule on OptimizationJobSpec already
	// enforces, so a create-time budget check cannot be bypassed on update.
	return nil, nil
}

func (w *OptimizationJobValidator) ValidateDelete(ctx context.Context, obj *trainer.OptimizationJob) (admission.Warnings, error) {
	return nil, nil
}
