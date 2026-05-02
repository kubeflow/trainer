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

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/metrics"
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

	runtimeRefGK := runtime.RuntimeRefToRuntimeRegistryKey(obj.Spec.RuntimeRef)
	rt, ok := w.runtimes[runtimeRefGK]
	if !ok {
		metrics.WebhookValidationTotal.WithLabelValues("TrainJob", "create", "error").Inc()
		return nil, fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
	}
	warnings, errs := rt.ValidateObjects(ctx, nil, obj)
	result := "success"
	if errs != nil {
		result = "error"
	}
	metrics.WebhookValidationTotal.WithLabelValues("TrainJob", "create", result).Inc()
	return warnings, errs.ToAggregate()
}

func (w *TrainJobValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.TrainJob) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("trainJob-webhook")
	log.V(5).Info("Validating update", "TrainJob", klog.KObj(newObj))

	runtimeRefGK := runtime.RuntimeRefToRuntimeRegistryKey(newObj.Spec.RuntimeRef)
	rt, ok := w.runtimes[runtimeRefGK]
	if !ok {
		metrics.WebhookValidationTotal.WithLabelValues("TrainJob", "update", "error").Inc()
		return nil, fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
	}
	warnings, errs := rt.ValidateObjects(ctx, oldObj, newObj)
	result := "success"
	if errs != nil {
		result = "error"
	}
	metrics.WebhookValidationTotal.WithLabelValues("TrainJob", "update", result).Inc()
	return warnings, errs.ToAggregate()
}

func (w *TrainJobValidator) ValidateDelete(ctx context.Context, obj *trainer.TrainJob) (admission.Warnings, error) {
	return nil, nil
}
