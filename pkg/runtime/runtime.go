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

	corev1 "k8s.io/api/core/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"

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

type InfoOption func(options *InfoOptions)

var defaultOptions = InfoOptions{}

// DEPRECATED: Replace all podSpecReplica usage with PodSet
// once we remove TotalResourceRequest.

type podSpecReplica struct {
	count             int32
	name              string
	podSpecApply      *corev1ac.PodSpecApplyConfiguration
	singlePodRequests corev1.ResourceList
}

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

// DEPRECATED: Replace WithPodSpecReplicas with WithTemplateSpec
// once we remove TotalResourceRequest.

func WithPodSpecReplicas(
	replicaName string, count int32, singlePodRequest corev1.ResourceList, podSpecApply *corev1ac.PodSpecApplyConfiguration,
) InfoOption {
	return func(o *InfoOptions) {
		o.podSpecReplicas = append(o.podSpecReplicas, podSpecReplica{
			name:              replicaName,
			count:             max(count, 1),
			podSpecApply:      podSpecApply,
			singlePodRequests: singlePodRequest,
		})
	}
}

func WithTemplateSpec(objApply any) InfoOption {
	return func(o *InfoOptions) {
		o.templateSpec.ObjApply = objApply
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
			TotalRequests: make(map[string]TotalResourceRequest, len(options.podSpecReplicas)),
		},
		TemplateSpec: options.templateSpec,
	}

	for _, spec := range options.podSpecReplicas {
		info.TotalRequests[spec.name] = TotalResourceRequest{
			Replicas:    spec.count,
			PodRequests: spec.singlePodRequests,
		}
		ps := PodSet{
			Name:    spec.name,
			Volumes: spec.podSpecApply.Volumes,
		}
		if spec.name != constants.JobTrainerNode {
			ps.CountForNonTrainer = &spec.count
		}
		for _, container := range spec.podSpecApply.Containers {
			ps.Containers = append(ps.Containers, Container{
				Name:         *container.Name,
				Env:          container.Env,
				Ports:        container.Ports,
				VolumeMounts: container.VolumeMounts,
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
	return info
}

func (i *Info) SyncPodSetsToTemplateSpec() {
	syncPodSets(i)
}

func TemplateSpecApply[A any](info *Info) (*A, bool) {
	spec, ok := info.TemplateSpec.ObjApply.(*A)
	return spec, ok
}

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
