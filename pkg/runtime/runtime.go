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

package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/apply"
	"github.com/kubeflow/trainer/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	resourcehelpers "k8s.io/component-helpers/resource"
)

var (
	errorTemplateSpecPathNotFound = errors.New("template spec path not found")

	defaultPodSetsSyncer = func(*Info) {}
	syncPodSets          = defaultPodSetsSyncer
)

type Info struct {
	// Labels and Annotations to add to the RuntimeJobTemplate.
	Labels      map[string]string
	Annotations map[string]string
	// Original policy values from the runtime.
	RuntimePolicy RuntimePolicy
	// Trainer parameters to add to the RuntimeJobTemplate.
	Trainer
	// Scheduler parameters to add to the RuntimeJobTemplate.
	*Scheduler
	// TemplateSpec is TrainingRuntime Template object.
	// ObjApply podSpecs and this PodSets should be kept in sync by info.SyncPodSetsToTemplateSpec().
	TemplateSpec TemplateSpec
}

type RuntimePolicy struct {
	MLPolicy       *trainer.MLPolicy
	PodGroupPolicy *trainer.PodGroupPolicy
}

type TemplateSpec struct {
	// ObjApply is ApplyConfiguration for the TrainingRuntimes Template field.
	ObjApply any
	// PodSets is a set of Pod extracted from ObjApply.
	PodSets []PodSet
}

type PodSet struct {
	Name string
	// If Name is trainer-node, CountForNonTrainer is null.
	// For Trainer, PodSet Count should be stored in Info.RuntimePolicy.MLPolicy.NumNodes.
	CountForNonTrainer *int32
	Containers         []Container
	Volumes            []corev1ac.VolumeApplyConfiguration
	Endpoints          iter.Seq[string]
}

type Container struct {
	Name         string
	Env          []corev1ac.EnvVarApplyConfiguration
	Ports        []corev1ac.ContainerPortApplyConfiguration
	VolumeMounts []corev1ac.VolumeMountApplyConfiguration
}

// DEPRECATED: Replace all Trainer usage with RuntimePolicy and PodSet.

type Trainer struct {
	NumNodes       *int32
	NumProcPerNode string
	Env            []corev1ac.EnvVarApplyConfiguration
	ContainerPort  *corev1ac.ContainerPortApplyConfiguration
	Volumes        []corev1ac.VolumeApplyConfiguration
	VolumeMounts   []corev1ac.VolumeMountApplyConfiguration
}

// TODO (andreyvelich): Potentially, we can add ScheduleTimeoutSeconds to the Scheduler for consistency.
type Scheduler struct {
	PodLabels     map[string]string
	TotalRequests map[string]TotalResourceRequest
}

// DEPRECATED: Replace all TotalResourceRequest usage with PodSet.

type TotalResourceRequest struct {
	Replicas    int32
	PodRequests corev1.ResourceList
}

type InfoOptions struct {
	labels          map[string]string
	annotations     map[string]string
	runtimePolicy   RuntimePolicy
	podSpecReplicas []podSpecReplica
	templateSpec    TemplateSpec
}

type InfoOption func(options *InfoOptions) error

var defaultOptions = InfoOptions{}

type podSpecReplica struct {
	count   int32
	name    string
	podSpec corev1.PodSpec
}

func WithLabels(labels map[string]string) InfoOption {
	return func(o *InfoOptions) error {
		o.labels = maps.Clone(labels)
		return nil
	}
}

func WithAnnotations(annotations map[string]string) InfoOption {
	return func(o *InfoOptions) error {
		o.annotations = maps.Clone(annotations)
		return nil
	}
}

func WithMLPolicy(mlPolicy *trainer.MLPolicy) InfoOption {
	return func(o *InfoOptions) error {
		o.runtimePolicy.MLPolicy = mlPolicy
		return nil
	}
}

func WithPodGroupPolicy(pgPolicy *trainer.PodGroupPolicy) InfoOption {
	return func(o *InfoOptions) error {
		o.runtimePolicy.PodGroupPolicy = pgPolicy
		return nil
	}
}

func WithPodSpecReplicas(replicaName string, count int32, podSpec corev1.PodSpec) InfoOption {
	return func(o *InfoOptions) error {
		o.podSpecReplicas = append(o.podSpecReplicas, podSpecReplica{
			name:    replicaName,
			count:   max(count, 1),
			podSpec: *podSpec.DeepCopy(),
		})
		return nil
	}
}

func WithTemplateSpecObjApply[A any](obj client.Object, fields ...string) InfoOption {
	return func(o *InfoOptions) error {
		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return err
		}
		templateSpec, ok, err := unstructured.NestedFieldCopy(u, fields...)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: '.%s'", errorTemplateSpecPathNotFound, strings.Join(fields, "."))
		}
		raw, err := json.Marshal(templateSpec)
		if err != nil {
			return err
		}
		var objApply *A
		if err = json.Unmarshal(raw, &objApply); err != nil {
			return err
		}
		o.templateSpec.ObjApply = objApply
		return nil
	}
}

func WithPodSetSyncer(syncer func(*Info)) InfoOption {
	return func(o *InfoOptions) error {
		syncPodSets = syncer
		return nil
	}
}

func NewInfo(opts ...InfoOption) (*Info, error) {
	options := defaultOptions
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	info := &Info{
		Labels:        make(map[string]string),
		Annotations:   make(map[string]string),
		RuntimePolicy: options.runtimePolicy,
		Scheduler: &Scheduler{
			TotalRequests: make(map[string]TotalResourceRequest, len(options.podSpecReplicas)),
		},
		TemplateSpec: options.templateSpec,
	}

	for _, spec := range options.podSpecReplicas {
		info.TotalRequests[spec.name] = TotalResourceRequest{
			Replicas:    spec.count,
			PodRequests: resourcehelpers.PodRequests(&corev1.Pod{Spec: spec.podSpec}, resourcehelpers.PodResourcesOptions{}),
		}
		ps := PodSet{
			Name:    spec.name,
			Volumes: apply.Volumes(spec.podSpec.Volumes...),
		}
		if spec.name != constants.JobTrainerNode {
			ps.CountForNonTrainer = &spec.count
		}
		for _, container := range spec.podSpec.Containers {
			ps.Containers = append(ps.Containers, Container{
				Name:         container.Name,
				Env:          apply.EnvVars(container.Env...),
				Ports:        apply.ContainerPorts(container.Ports...),
				VolumeMounts: apply.VolumeMounts(container.VolumeMounts...),
			})
		}
		info.TemplateSpec.PodSets = append(info.TemplateSpec.PodSets, ps)
	}
	if options.labels != nil {
		info.Labels = options.labels
	}
	if options.annotations != nil {
		info.Annotations = options.annotations
	}
	return info, nil
}

func (i *Info) SyncPodSetsToTemplateSpec() {
	syncPodSets(i)
}

func TemplateSpecApply[A any](info *Info) (*A, bool) {
	spec, ok := info.TemplateSpec.ObjApply.(*A)
	return spec, ok
}
