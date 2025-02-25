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
	"fmt"
	"maps"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

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

// +kubebuilder:rbac:groups=jobset.x-k8s.io,resources=jobsets,verbs=get;list;watch;create

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

func (j *JobSet) Build(ctx context.Context, runtimeJobTemplate client.Object, info *runtime.Info, trainJob *trainer.TrainJob) ([]client.Object, error) {
	if runtimeJobTemplate == nil || info == nil || trainJob == nil {
		return nil, fmt.Errorf("runtime info or object is missing")
	}

	raw, ok := runtimeJobTemplate.(*jobsetv1alpha2.JobSet)
	if !ok {
		return nil, nil
	}

	var jobSetBuilder *Builder
	oldJobSet := &jobsetv1alpha2.JobSet{}
	if err := j.client.Get(ctx, client.ObjectKeyFromObject(trainJob), oldJobSet); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		jobSetBuilder = NewBuilder(client.ObjectKeyFromObject(trainJob), trainer.JobSetTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      info.Labels,
				Annotations: info.Annotations,
			},
			Spec: raw.Spec,
		})
		oldJobSet = nil
	} else {
		jobSetBuilder = &Builder{
			JobSet: *oldJobSet.DeepCopy(),
		}
	}

	// TODO (andreyvelich): Add support for the PodSpecOverride.
	// TODO (andreyvelich): Refactor the builder with wrappers for PodSpec.
	jobSet := jobSetBuilder.
		Initializer(trainJob).
		Launcher(info, trainJob).
		Trainer(info, trainJob).
		PodLabels(info.PodLabels).
		Suspend(trainJob.Spec.Suspend).
		Build()
	if err := ctrlutil.SetControllerReference(trainJob, jobSet, j.scheme); err != nil {
		return nil, err
	}

	if needsCreateOrUpdate(oldJobSet, jobSet, ptr.Deref(trainJob.Spec.Suspend, false)) {
		return []client.Object{jobSet}, nil
	}
	return nil, nil
}

func needsCreateOrUpdate(old, new *jobsetv1alpha2.JobSet, trainJobIsSuspended bool) bool {
	return old == nil ||
		(!trainJobIsSuspended && jobSetIsSuspended(old) && !jobSetIsSuspended(new)) ||
		(trainJobIsSuspended && (!equality.Semantic.DeepEqual(old.Spec, new.Spec) || !maps.Equal(old.Labels, new.Labels) || !maps.Equal(old.Annotations, new.Annotations)))
}

func jobSetIsSuspended(jobSet *jobsetv1alpha2.JobSet) bool {
	return ptr.Deref(jobSet.Spec.Suspend, false)
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
