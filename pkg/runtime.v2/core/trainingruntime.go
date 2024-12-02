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

package core

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	kubeflowv2 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v2alpha1"
	runtime "github.com/kubeflow/training-operator/pkg/runtime.v2"
	fwkcore "github.com/kubeflow/training-operator/pkg/runtime.v2/framework/core"
	fwkplugins "github.com/kubeflow/training-operator/pkg/runtime.v2/framework/plugins"
	idxer "github.com/kubeflow/training-operator/pkg/runtime.v2/indexer"
)

var (
	errorNotFoundSpecifiedTrainingRuntime = errors.New("TrainingRuntime specified in TrainJob is not found")
)

type TrainingRuntime struct {
	framework *fwkcore.Framework
	client    client.Client
}

var TrainingRuntimeGroupKind = schema.GroupKind{
	Group: kubeflowv2.GroupVersion.Group,
	Kind:  kubeflowv2.TrainingRuntimeKind,
}.String()

var _ runtime.Runtime = (*TrainingRuntime)(nil)

var trainingRuntimeFactory *TrainingRuntime

func NewTrainingRuntime(ctx context.Context, c client.Client, indexer client.FieldIndexer) (runtime.Runtime, error) {
	if err := indexer.IndexField(ctx, &kubeflowv2.TrainJob{}, idxer.TrainJobRuntimeRefKey, idxer.IndexTrainJobTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on TrainingRuntime for TrainJob: %w", err)
	}
	if err := indexer.IndexField(ctx, &kubeflowv2.TrainJob{}, idxer.TrainJobClusterRuntimeRefKey, idxer.IndexTrainJobClusterTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on ClusterTrainingRuntime for TrainJob: %w", err)
	}
	fwk, err := fwkcore.New(ctx, c, fwkplugins.NewRegistry(), indexer)
	if err != nil {
		return nil, err
	}
	trainingRuntimeFactory = &TrainingRuntime{
		framework: fwk,
		client:    c,
	}
	return trainingRuntimeFactory, nil
}

func (r *TrainingRuntime) NewObjects(ctx context.Context, trainJob *kubeflowv2.TrainJob) ([]client.Object, error) {
	var trainingRuntime kubeflowv2.TrainingRuntime
	err := r.client.Get(ctx, client.ObjectKey{Namespace: trainJob.Namespace, Name: trainJob.Spec.RuntimeRef.Name}, &trainingRuntime)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errorNotFoundSpecifiedTrainingRuntime, err)
	}
	return r.buildObjects(ctx, trainJob, trainingRuntime.Spec.Template, trainingRuntime.Spec.MLPolicy, trainingRuntime.Spec.PodGroupPolicy)
}

func (r *TrainingRuntime) buildObjects(
	ctx context.Context, trainJob *kubeflowv2.TrainJob, jobSetTemplateSpec kubeflowv2.JobSetTemplateSpec, mlPolicy *kubeflowv2.MLPolicy, podGroupPolicy *kubeflowv2.PodGroupPolicy,
) ([]client.Object, error) {
	propagationLabels := jobSetTemplateSpec.Labels
	if propagationLabels == nil && trainJob.Spec.Labels != nil {
		propagationLabels = make(map[string]string, len(trainJob.Spec.Labels))
	}
	for k, v := range trainJob.Spec.Labels {
		// The JobSetTemplateSpec labels are overridden by the TrainJob Labels (.spec.labels).
		propagationLabels[k] = v
	}
	propagationAnnotations := jobSetTemplateSpec.Annotations
	if propagationAnnotations == nil && trainJob.Spec.Annotations != nil {
		propagationAnnotations = make(map[string]string, len(trainJob.Spec.Annotations))
	}
	for k, v := range trainJob.Spec.Annotations {
		// The JobSetTemplateSpec annotations are overridden by the TrainJob Annotations (.spec.annotations).
		propagationAnnotations[k] = v
	}
	opts := []runtime.InfoOption{
		runtime.WithLabels(propagationLabels),
		runtime.WithAnnotations(propagationAnnotations),
		runtime.WithMLPolicy(mlPolicy),
		runtime.WithPodGroupPolicy(podGroupPolicy),
	}

	for _, rJob := range jobSetTemplateSpec.Spec.ReplicatedJobs {
		// By default every ReplicatedJob has only 1 replica.
		opts = append(opts, runtime.WithPodSpecReplicas(rJob.Name, 1, rJob.Template.Spec.Template.Spec))
	}

	info := runtime.NewInfo(opts...)

	if err := r.framework.RunEnforceMLPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}

	if err := r.framework.RunEnforcePodGroupPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}

	jobSetTemplate := jobsetv1alpha2.JobSet{
		Spec: jobSetTemplateSpec.Spec,
	}

	return r.framework.RunComponentBuilderPlugins(ctx, jobSetTemplate.DeepCopy(), info, trainJob)
}

func (r *TrainingRuntime) EventHandlerRegistrars() []runtime.ReconcilerBuilder {
	var builders []runtime.ReconcilerBuilder
	for _, ex := range r.framework.WatchExtensionPlugins() {
		builders = append(builders, ex.ReconcilerBuilders()...)
	}
	return builders
}

func (r *TrainingRuntime) ValidateObjects(ctx context.Context, old, new *kubeflowv2.TrainJob) (admission.Warnings, field.ErrorList) {
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: old.Namespace,
		Name:      old.Spec.RuntimeRef.Name,
	}, &kubeflowv2.TrainingRuntime{}); err != nil {
		return nil, field.ErrorList{
			field.Invalid(field.NewPath("spec", "runtimeRef"), old.Spec.RuntimeRef,
				fmt.Sprintf("%v: specified trainingRuntime must be created before the TrainJob is created", err)),
		}
	}
	return r.framework.RunCustomValidationPlugins(old, new)
}
