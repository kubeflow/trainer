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
	"iter"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	resourcehelpers "k8s.io/component-helpers/resource"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
)

var (
	defaultPodSetsSyncer = func(*Info) {}
	syncPodSets          = defaultPodSetsSyncer
)

type Info struct {
	// Labels and Annotations to add to the RuntimeJobTemplate.
	Labels      map[string]string
	Annotations map[string]string
	// Original policy values from the runtime.
	RuntimePolicy RuntimePolicy
	// Scheduler parameters to add to the RuntimeJobTemplate.
	Scheduler *Scheduler
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
	// This is abstract concept to represent multiple PodSpec as a unit.
	PodSets []PodSet
}

type PodSet struct {
	// PodSet name is the name to identify PodSpec.
	// This typically has the name stored in each PodSpec.
	Name string
	// If Name is trainer-node, CountForNonTrainer is null.
	// For Trainer, PodSet Count should be stored in Info.RuntimePolicy.MLPolicy.NumNodes.
	CountForNonTrainer *int32
	InitContainers     []Container
	Containers         []Container
	Volumes            []corev1ac.VolumeApplyConfiguration
	Endpoints          iter.Seq[string]
	// The total PodSet requests can be calculated with
	// SinglePodRequests x [CountForNonTrainer|RuntimePolicy.MLPolicy.NumNodes].
	SinglePodRequests corev1.ResourceList
}

type Container struct {
	Name         string
	Env          []corev1ac.EnvVarApplyConfiguration
	Ports        []corev1ac.ContainerPortApplyConfiguration
	VolumeMounts []corev1ac.VolumeMountApplyConfiguration
}

// TODO (andreyvelich): Potentially, we can add ScheduleTimeoutSeconds to the Scheduler for consistency.
type Scheduler struct {
	PodLabels map[string]string
}

type InfoOptions struct {
	labels        map[string]string
	annotations   map[string]string
	runtimePolicy RuntimePolicy
	templateSpec  TemplateSpec
}

type InfoOption func(options *InfoOptions)

var defaultOptions = InfoOptions{}

func WithLabels(labels map[string]string) InfoOption {
	return func(o *InfoOptions) {
		o.labels = maps.Clone(labels)
	}
}

func WithAnnotations(annotations map[string]string) InfoOption {
	return func(o *InfoOptions) {
		o.annotations = maps.Clone(annotations)
	}
}

func WithMLPolicy(mlPolicy *trainer.MLPolicy) InfoOption {
	return func(o *InfoOptions) {
		o.runtimePolicy.MLPolicy = mlPolicy
	}
}

func WithPodGroupPolicy(pgPolicy *trainer.PodGroupPolicy) InfoOption {
	return func(o *InfoOptions) {
		o.runtimePolicy.PodGroupPolicy = pgPolicy
	}
}

func WithTemplateSpecObjApply(objApply any) InfoOption {
	return func(o *InfoOptions) {
		o.templateSpec.ObjApply = objApply
	}
}

// WithPodSet construct Info.TemplateSpec.PodSet from PodSpec.
// The third argument, 'typedPodSpec' is used only to calculate requested resources.
func WithPodSet(
	psName string, count int32, typedPodSpec corev1.PodSpec, podSpecApply *corev1ac.PodSpecApplyConfiguration,
) InfoOption {
	return func(o *InfoOptions) {
		ps := PodSet{
			Name:              psName,
			Volumes:           podSpecApply.Volumes,
			SinglePodRequests: resourcehelpers.PodRequests(&corev1.Pod{Spec: typedPodSpec}, resourcehelpers.PodResourcesOptions{}),
			InitContainers:    slices.Collect(toPodSetContainer(podSpecApply.InitContainers...)),
			Containers:        slices.Collect(toPodSetContainer(podSpecApply.Containers...)),
		}
		if psName != constants.JobTrainerNode {
			ps.CountForNonTrainer = ptr.To(max(count, 1))
		}
		o.templateSpec.PodSets = append(o.templateSpec.PodSets, ps)
	}
}

func toPodSetContainer(containerApply ...corev1ac.ContainerApplyConfiguration) iter.Seq[Container] {
	return func(yield func(Container) bool) {
		for _, cApply := range containerApply {
			container := Container{
				Name:         ptr.Deref(cApply.Name, ""),
				Env:          cApply.Env,
				Ports:        cApply.Ports,
				VolumeMounts: cApply.VolumeMounts,
			}
			if !yield(container) {
				return
			}
		}
	}
}

func WithPodSetSyncer(syncer func(*Info)) InfoOption {
	return func(o *InfoOptions) {
		syncPodSets = syncer
	}
}

func NewInfo(opts ...InfoOption) *Info {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}

	info := &Info{
		Labels:        make(map[string]string),
		Annotations:   make(map[string]string),
		RuntimePolicy: options.runtimePolicy,
		Scheduler: &Scheduler{
			PodLabels: make(map[string]string),
		},
		TemplateSpec: options.templateSpec,
	}
	if options.labels != nil {
		info.Labels = options.labels
	}
	if options.annotations != nil {
		info.Annotations = options.annotations
	}
	return info
}

func (i *Info) SyncPodSetsToTemplateSpec() {
	syncPodSets(i)
}

func TemplateSpecApply[A any](info *Info) (*A, bool) {
	spec, ok := info.TemplateSpec.ObjApply.(*A)
	return spec, ok
}

// FindContainerByPodSetContainerName finds runtime.Container from Info.TemplateSpec.PodSet by PodSet and Container name.
func (i *Info) FindContainerByPodSetContainerName(psName, containerName string) *Container {
	for psIdx, ps := range i.TemplateSpec.PodSets {
		if ps.Name == psName {
			for containerIdx, container := range ps.Containers {
				if container.Name == containerName {
					return &i.TemplateSpec.PodSets[psIdx].Containers[containerIdx]
				}
			}
		}
	}
	return nil
}

func RuntimeRefToRuntimeRegistryKey(runtimeRef trainer.RuntimeRef) string {
	return schema.GroupKind{
		Group: ptr.Deref(runtimeRef.APIGroup, ""),
		Kind:  ptr.Deref(runtimeRef.Kind, ""),
	}.String()
}
