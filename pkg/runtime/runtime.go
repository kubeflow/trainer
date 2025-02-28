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
	"maps"

	corev1 "k8s.io/api/core/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	resourcehelpers "k8s.io/component-helpers/resource"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
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
}

type RuntimePolicy struct {
	MLPolicy       *trainer.MLPolicy
	PodGroupPolicy *trainer.PodGroupPolicy
}

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

type TotalResourceRequest struct {
	Replicas    int32
	PodRequests corev1.ResourceList
}

type InfoOptions struct {
	labels          map[string]string
	annotations     map[string]string
	runtimePolicy   RuntimePolicy
	podSpecReplicas []podSpecReplica
}

type InfoOption func(options *InfoOptions)

var defaultOptions = InfoOptions{}

type podSpecReplica struct {
	replicas int32
	name     string
	podSpec  corev1.PodSpec
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

func WithPodSpecReplicas(replicaName string, replicas int32, podSpec corev1.PodSpec) InfoOption {
	return func(o *InfoOptions) {
		o.podSpecReplicas = append(o.podSpecReplicas, podSpecReplica{
			name:     replicaName,
			replicas: replicas,
			podSpec:  *podSpec.DeepCopy(),
		})
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
	}

	for _, spec := range options.podSpecReplicas {
		info.TotalRequests[spec.name] = TotalResourceRequest{
			Replicas:    spec.replicas,
			PodRequests: resourcehelpers.PodRequests(&corev1.Pod{Spec: spec.podSpec}, resourcehelpers.PodResourcesOptions{}),
		}
	}
	if options.labels != nil {
		info.Labels = options.labels
	}
	if options.annotations != nil {
		info.Annotations = options.annotations
	}

	return info
}
