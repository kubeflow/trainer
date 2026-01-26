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

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-trainjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=trainjobs,verbs=create;update,versions=v1alpha1,name=validator.trainjob.trainer.kubeflow.org,admissionReviewVersions=v1

type TrainJobValidator struct {
	runtimes map[string]runtime.Runtime
}

func setupWebhookForTrainJob(mgr ctrl.Manager, run map[string]runtime.Runtime) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.TrainJob{}).
		WithValidator(&TrainJobValidator{runtimes: run}).
		Complete()
}

func (w *TrainJobValidator) ValidateCreate(ctx context.Context, obj *trainer.TrainJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainJob-webhook")
	log.V(5).Info("Validating create", "TrainJob", klog.KObj(obj))

	runtimeRefGK := runtime.RuntimeRefToRuntimeRegistryKey(obj.Spec.RuntimeRef)
	runtime, ok := w.runtimes[runtimeRefGK]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
	}
	warnings, errors := runtime.ValidateObjects(ctx, nil, obj)
	return warnings, errors.ToAggregate()
}

func (w *TrainJobValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.TrainJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainJob-webhook")
	log.V(5).Info("Validating update", "TrainJob", klog.KObj(newObj))

	runtimeRefGK := runtime.RuntimeRefToRuntimeRegistryKey(newObj.Spec.RuntimeRef)
	runtime, ok := w.runtimes[runtimeRefGK]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
	}
	warnings, errors := runtime.ValidateObjects(ctx, oldObj, newObj)
	return warnings, errors.ToAggregate()
}

func (w *TrainJobValidator) ValidateDelete(ctx context.Context, obj *trainer.TrainJob) (admission.Warnings, error) {
	return nil, nil
}
