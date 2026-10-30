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

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/util/optimizationjob"
	"github.com/kubeflow/trainer/v2/pkg/util/trainjob"
)

// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-optimizationjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=create,versions=v1alpha1,name=validator.optimizationjob.trainer.kubeflow.org,admissionReviewVersions=v1

// OptimizationJobValidator validates OptimizationJobs.
type OptimizationJobValidator struct {
	client client.Client
}

var _ admission.Validator[*trainer.OptimizationJob] = (*OptimizationJobValidator)(nil)

func setupWebhookForOptimizationJob(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainer.OptimizationJob{}).
		WithValidator(&OptimizationJobValidator{client: mgr.GetClient()}).
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

	runtimeRef := obj.Spec.TrainJobTemplate.Spec.RuntimeRef
	if w.client != nil && runtimeRef.Name != "" {
		var replicatedJobs []jobsetv1alpha2.ReplicatedJob
		var mlPolicy *trainer.MLPolicy
		if trainjob.RuntimeRefIsTrainingRuntime(runtimeRef) {
			trainingRuntime := &trainer.TrainingRuntime{}
			if err := w.client.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: runtimeRef.Name}, trainingRuntime); err == nil {
				replicatedJobs = trainingRuntime.Spec.Template.Spec.ReplicatedJobs
				mlPolicy = trainingRuntime.Spec.MLPolicy
			}
		} else if trainjob.RuntimeRefIsClusterTrainingRuntime(runtimeRef) || runtimeRef.Kind == nil || *runtimeRef.Kind == trainer.ClusterTrainingRuntimeKind {
			clusterTrainingRuntime := &trainer.ClusterTrainingRuntime{}
			if err := w.client.Get(ctx, client.ObjectKey{Name: runtimeRef.Name}, clusterTrainingRuntime); err == nil {
				replicatedJobs = clusterTrainingRuntime.Spec.Template.Spec.ReplicatedJobs
				mlPolicy = clusterTrainingRuntime.Spec.MLPolicy
			}
		}
		if len(replicatedJobs) > 0 {
			minMaxLen := 63
			limitingRJob := ""
			for _, rJob := range replicatedJobs {
				if rJob.Name == "" {
					continue
				}
				replicas := rJob.Replicas
				if replicas < 1 {
					replicas = 1
				}
				completions := int32(1)
				ancestor := ""
				if rJob.Template.Labels != nil {
					ancestor = rJob.Template.Labels[constants.LabelTrainJobAncestor]
				}
				if ancestor == constants.AncestorTrainer {
					numNodes := int32(1)
					if mlPolicy != nil && mlPolicy.NumNodes != nil {
						numNodes = *mlPolicy.NumNodes
					}
					if obj.Spec.TrainJobTemplate.Spec.Trainer != nil && obj.Spec.TrainJobTemplate.Spec.Trainer.NumNodes != nil {
						numNodes = *obj.Spec.TrainJobTemplate.Spec.Trainer.NumNodes
					}
					completions = numNodes / replicas
				} else {
					if rJob.Template.Spec.Completions != nil {
						completions = *rJob.Template.Spec.Completions
					} else if rJob.Template.Spec.Parallelism != nil {
						completions = *rJob.Template.Spec.Parallelism
					}
				}
				if completions < 1 {
					completions = 1
				}
				maxLen := trainjob.MaxTrainJobNameLength(rJob.Name, replicas, completions)
				if maxLen < minMaxLen {
					minMaxLen = maxLen
					limitingRJob = rJob.Name
				}
			}
			maxOptJobNameLen := optimizationjob.MaxOptimizationJobNameLength(minMaxLen)
			if len(obj.Name) > maxOptJobNameLen && limitingRJob != "" {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("metadata", "name"),
					obj.Name,
					fmt.Sprintf("OptimizationJob name is too long, trial TrainJob names generated for replicatedJob %q will exceed maximum allowed length of %d (maximum OptimizationJob name length is %d)", limitingRJob, minMaxLen, maxOptJobNameLen),
				))
			}
		}
	}

	return nil, allErrs.ToAggregate()
}

func (w *OptimizationJobValidator) ValidateUpdate(context.Context, *trainer.OptimizationJob, *trainer.OptimizationJob) (admission.Warnings, error) {
	return nil, nil
}

func (w *OptimizationJobValidator) ValidateDelete(context.Context, *trainer.OptimizationJob) (admission.Warnings, error) {
	return nil, nil
}
