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
	"github.com/kubeflow/trainer/v2/pkg/constants"
	trainingruntime "github.com/kubeflow/trainer/v2/pkg/util/trainingruntime"
)

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-clustertrainingruntime,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=clustertrainingruntimes,verbs=create;update,versions=v1alpha1,name=validator.clustertrainingruntime.trainer.kubeflow.org,admissionReviewVersions=v1

// ClusterTrainingRuntimeValidator validates ClusterTrainingRuntimes
type ClusterTrainingRuntimeValidator struct{}

var _ admission.Validator[*trainer.ClusterTrainingRuntime] = (*ClusterTrainingRuntimeValidator)(nil)

func setupWebhookForClusterTrainingRuntime(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.ClusterTrainingRuntime{}).
		WithValidator(&ClusterTrainingRuntimeValidator{}).
		Complete()
}

func (w *ClusterTrainingRuntimeValidator) ValidateCreate(ctx context.Context, obj *trainer.ClusterTrainingRuntime) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("clustertrainingruntime-webhook")
	log.V(5).Info("Validating create", "clusterTrainingRuntime", klog.KObj(obj))
	var warnings admission.Warnings
	if trainingruntime.IsSupportDeprecated(obj.Labels) {
		warnings = append(warnings, fmt.Sprintf(
			"ClusterTrainingRuntime \"%s\" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: %s",
			obj.Name,
			constants.RuntimeDeprecationPolicyURL,
		))
	}
	if obj.Spec.TTLSecondsAfterFinished != nil && *obj.Spec.TTLSecondsAfterFinished < 60 {
		warnings = append(warnings, "ttlSecondsAfterFinished is less than 60s; completed TrainJobs will be deleted very quickly")
	}
	return warnings, validateReplicatedJobs(obj.Spec.Template.Spec.ReplicatedJobs).ToAggregate()
}

func (w *ClusterTrainingRuntimeValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.ClusterTrainingRuntime) (admission.Warnings, error) {
	log := ctrl.LoggerFrom(ctx).WithName("clustertrainingruntime-webhook")
	log.V(5).Info("Validating update", "clusterTrainingRuntime", klog.KObj(newObj))
	var warnings admission.Warnings
	if newObj.Spec.TTLSecondsAfterFinished != nil && *newObj.Spec.TTLSecondsAfterFinished < 60 {
		warnings = append(warnings, "ttlSecondsAfterFinished is less than 60s; completed TrainJobs will be deleted very quickly")
	}
	// Deprecation warning is not required on updates to avoid spam.
	return warnings, validateReplicatedJobs(newObj.Spec.Template.Spec.ReplicatedJobs).ToAggregate()
}

func (w *ClusterTrainingRuntimeValidator) ValidateDelete(ctx context.Context, obj *trainer.ClusterTrainingRuntime) (admission.Warnings, error) {
	return nil, nil
}
