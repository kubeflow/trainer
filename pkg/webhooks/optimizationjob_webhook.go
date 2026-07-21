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
	"fmt"
	"reflect"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

// +kubebuilder:webhook:path=/mutate-trainer-kubeflow-org-v1alpha1-optimizationjob,mutating=true,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=create;update,versions=v1alpha1,name=defaulter.optimizationjob.trainer.kubeflow.org,admissionReviewVersions=v1

type OptimizationJobDefaulter struct{}

var _ admission.Defaulter[*trainer.OptimizationJob] = (*OptimizationJobDefaulter)(nil)

func (d *OptimizationJobDefaulter) Default(ctx context.Context, obj *trainer.OptimizationJob) error {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationJob-webhook")
	log.V(5).Info("Defaulting", "OptimizationJob", klog.KObj(obj))

	// 1. Default Budget Limits
	if obj.Spec.ParallelTrials == nil {
		var defaultParallel int32 = 1
		obj.Spec.ParallelTrials = &defaultParallel
	}
	if obj.Spec.NumTrials == nil {
		var defaultNum int32 = 1
		obj.Spec.NumTrials = &defaultNum
	}

	// 2. Default Search Algorithm to Random if entirely omitted
	if obj.Spec.SearchAlgorithm == nil {
		obj.Spec.SearchAlgorithm = &trainer.SearchAlgorithm{
			Random: &trainer.RandomAlgorithm{},
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-optimizationjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=create;update,versions=v1alpha1,name=validator.optimizationjob.trainer.kubeflow.org,admissionReviewVersions=v1

type OptimizationJobValidator struct{}

var _ admission.Validator[*trainer.OptimizationJob] = (*OptimizationJobValidator)(nil)

func SetupWebhookForOptimizationJob(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.OptimizationJob{}).
		WithDefaulter(&OptimizationJobDefaulter{}).
		WithValidator(&OptimizationJobValidator{}).
		Complete()
}

func (w *OptimizationJobValidator) ValidateCreate(ctx context.Context, obj *trainer.OptimizationJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationJob-webhook")
	log.V(5).Info("Validating create", "OptimizationJob", klog.KObj(obj))
	// Delegated to CEL rules
	return nil, nil
}

func (w *OptimizationJobValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.OptimizationJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationJob-webhook")
	log.V(5).Info("Validating update", "OptimizationJob", klog.KObj(newObj))

	// Immutability Checks: Prevent changing the core experiment definition mid-flight
	if !reflect.DeepEqual(oldObj.Spec.TrainJobTemplate, newObj.Spec.TrainJobTemplate) {
		return nil, fmt.Errorf("OptimizationJob.Spec.TrainJobTemplate is immutable")
	}

	if !reflect.DeepEqual(oldObj.Spec.Parameters, newObj.Spec.Parameters) {
		return nil, fmt.Errorf("OptimizationJob.Spec.Parameters are immutable")
	}

	if !reflect.DeepEqual(oldObj.Spec.Objectives, newObj.Spec.Objectives) {
		return nil, fmt.Errorf("OptimizationJob.Spec.Objectives are immutable")
	}

	// Note: Users ARE allowed to update NumTrials and ParallelTrials to scale the job.
	return nil, nil
}

func (w *OptimizationJobValidator) ValidateDelete(ctx context.Context, obj *trainer.OptimizationJob) (admission.Warnings, error) {
	return nil, nil
}
