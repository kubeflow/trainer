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

	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
)

type ClusterTrainingRuntimeWebhook struct {
	runtimes map[string]runtime.Runtime
}

func setupWebhookForClusterTrainingRuntime(mgr ctrl.Manager, run map[string]runtime.Runtime) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&trainer.ClusterTrainingRuntime{}).
		WithValidator(&ClusterTrainingRuntimeWebhook{runtimes: run}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-clustertrainingruntime,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=clustertrainingruntimes,verbs=create;update,versions=v1alpha1,name=validator.clustertrainingruntime.trainer.kubeflow.org,admissionReviewVersions=v1

var _ webhook.CustomValidator = (*ClusterTrainingRuntimeWebhook)(nil)

func (w *ClusterTrainingRuntimeWebhook) ValidateCreate(ctx context.Context, obj apiruntime.Object) (admission.Warnings, error) {
	clTrainingRuntime := obj.(*trainer.ClusterTrainingRuntime)
	log := ctrl.LoggerFrom(ctx).WithName("clustertrainingruntime-webhook")
	log.V(5).Info("Validating create", "clusterTrainingRuntime", klog.KObj(clTrainingRuntime))
	return nil, validateReplicatedJobs(clTrainingRuntime.Spec.Template.Spec.ReplicatedJobs).ToAggregate()
}

func (w *ClusterTrainingRuntimeWebhook) ValidateUpdate(context.Context, apiruntime.Object, apiruntime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (w *ClusterTrainingRuntimeWebhook) ValidateDelete(context.Context, apiruntime.Object) (admission.Warnings, error) {
	return nil, nil
}
