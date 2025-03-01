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

package jobset

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
)

type JobSet struct {
	client     client.Client
	restMapper meta.RESTMapper
	scheme     *apiruntime.Scheme
	logger     logr.Logger
}

var _ framework.WatchExtensionPlugin = (*JobSet)(nil)
var _ framework.ComponentBuilderPlugin = (*JobSet)(nil)
var _ framework.TerminalConditionPlugin = (*JobSet)(nil)

const Name = constants.JobSetKind

// +kubebuilder:rbac:groups=jobset.x-k8s.io,resources=jobsets,verbs=create;get;list;watch;update;patch

func New(ctx context.Context, client client.Client, _ client.FieldIndexer) (framework.Plugin, error) {
	return &JobSet{
		client:     client,
		restMapper: client.RESTMapper(),
		scheme:     client.Scheme(),
		logger:     ctrl.LoggerFrom(ctx).WithValues("pluginName", constants.JobSetKind),
	}, nil
}

func (j *JobSet) Name() string {
	return Name
}

func (j *JobSet) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	if _, err := j.restMapper.RESTMapping(
		schema.GroupKind{Group: jobsetv1alpha2.GroupVersion.Group, Kind: constants.JobSetKind},
		jobsetv1alpha2.SchemeGroupVersion.Version,
	); err != nil {
		// TODO (tenzen-y): After we provide the Configuration API, we should return errors based on the enabled plugins.
		j.logger.Error(err, "JobSet CRDs must be installed in advance")
	}
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Owns(&jobsetv1alpha2.JobSet{})
		},
	}
}

func (j *JobSet) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]any, error) {
	if info == nil || trainJob == nil {
		return nil, fmt.Errorf("runtime info or object is missing")
	}

	// Do not update the JobSet if it already exists and is not suspended
	oldJobSet := &jobsetv1alpha2.JobSet{}
	if err := j.client.Get(ctx, client.ObjectKeyFromObject(trainJob), oldJobSet); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		oldJobSet = nil
	}
	if oldJobSet != nil &&
		!ptr.Deref(trainJob.Spec.Suspend, false) &&
		!ptr.Deref(oldJobSet.Spec.Suspend, false) {
		return nil, nil
	}

	// Get the runtime as unstructured from the TrainJob ref
	runtimeJobTemplate := &unstructured.Unstructured{}
	runtimeJobTemplate.SetAPIVersion(trainer.GroupVersion.String())

	if kind := trainJob.Spec.RuntimeRef.Kind; kind != nil {
		runtimeJobTemplate.SetKind(*kind)
	} else {
		runtimeJobTemplate.SetKind(trainer.ClusterTrainingRuntimeKind)
	}

	key := client.ObjectKey{Name: trainJob.Spec.RuntimeRef.Name}
	if runtimeJobTemplate.GetKind() == trainer.TrainingRuntimeKind {
		key.Namespace = trainJob.Namespace
	}

	if err := j.client.Get(ctx, key, runtimeJobTemplate); err != nil {
		return nil, err
	}

	// Populate the JobSet template spec apply configuration
	jobSetTemplateSpec := &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{}
	if jobSetSpec, ok, err := unstructured.NestedFieldCopy(runtimeJobTemplate.Object, "spec", "template", "spec"); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("trainJob runtime %s does not have a spec.template.spec field", trainJob.Spec.RuntimeRef.Name)
	} else {
		if raw, err := json.Marshal(jobSetSpec); err != nil {
			return nil, err
		} else if err := json.Unmarshal(raw, jobSetTemplateSpec); err != nil {
			return nil, err
		}
	}

	// Init the JobSet apply configuration from the runtime template spec
	jobSetBuilder := NewBuilder(jobsetv1alpha2ac.JobSet(trainJob.Name, trainJob.Namespace).
		WithLabels(maps.Clone(info.Labels)).
		WithAnnotations(maps.Clone(info.Annotations)).
		WithSpec(jobSetTemplateSpec))

	// TODO (andreyvelich): Add support for the PodSpecOverride.
	// TODO (andreyvelich): Refactor the builder with wrappers for PodSpec.
	// Apply the runtime info
	jobSet := jobSetBuilder.
		Initializer(trainJob).
		Launcher(info, trainJob).
		Trainer(info, trainJob).
		PodLabels(info.PodLabels).
		Suspend(trainJob.Spec.Suspend).
		Build()

	// Set the TrainJob as owner
	jobSet.WithOwnerReferences(metav1ac.OwnerReference().
		WithAPIVersion(trainer.GroupVersion.String()).
		WithKind(trainer.TrainJobKind).
		WithName(trainJob.Name).
		WithUID(trainJob.UID).
		WithController(true).
		WithBlockOwnerDeletion(true))

	return []any{jobSet}, nil
}

func (j *JobSet) TerminalCondition(ctx context.Context, trainJob *trainer.TrainJob) (*metav1.Condition, error) {
	jobSet := &jobsetv1alpha2.JobSet{}
	if err := j.client.Get(ctx, client.ObjectKeyFromObject(trainJob), jobSet); err != nil {
		return nil, err
	}
	if completed := meta.FindStatusCondition(jobSet.Status.Conditions, string(jobsetv1alpha2.JobSetCompleted)); completed != nil && completed.Status == metav1.ConditionTrue {
		completed.Type = trainer.TrainJobComplete
		return completed, nil
	}
	if failed := meta.FindStatusCondition(jobSet.Status.Conditions, string(jobsetv1alpha2.JobSetFailed)); failed != nil && failed.Status == metav1.ConditionTrue {
		failed.Type = trainer.TrainJobFailed
		return failed, nil
	}
	return nil, nil
}
