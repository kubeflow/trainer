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
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
)

type Builder struct {
	jobsetv1alpha2.JobSet
}

func NewBuilder(objectKey client.ObjectKey, jobSetTemplateSpec trainer.JobSetTemplateSpec) *Builder {
	return &Builder{
		JobSet: jobsetv1alpha2.JobSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: jobsetv1alpha2.SchemeGroupVersion.String(),
				Kind:       constants.JobSetKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   objectKey.Namespace,
				Name:        objectKey.Name,
				Labels:      maps.Clone(jobSetTemplateSpec.Labels),
				Annotations: maps.Clone(jobSetTemplateSpec.Annotations),
			},
			Spec: *jobSetTemplateSpec.Spec.DeepCopy(),
		},
	}
}

// mergeInitializerEnvs merges the TrainJob and Runtime Pod envs.
func mergeInitializerEnvs(storageUri *string, trainJobEnvs, containerEnv []corev1.EnvVar) []corev1.EnvVar {
	envNames := sets.New[string]()
	var envs []corev1.EnvVar
	// Add the Storage URI env.
	if storageUri != nil {
		envNames.Insert(InitializerEnvStorageUri)
		envs = append(envs, corev1.EnvVar{
			Name:  InitializerEnvStorageUri,
			Value: *storageUri,
		})
	}
	// Add the rest TrainJob envs.
	// TODO (andreyvelich): Validate that TrainJob dataset and model envs don't have the STORAGE_URI env.
	for _, e := range trainJobEnvs {
		envNames.Insert(e.Name)
		envs = append(envs, e)
	}

	// TrainJob envs take precedence over the TrainingRuntime envs.
	for _, e := range containerEnv {
		if !envNames.Has(e.Name) {
			envs = append(envs, e)
		}
	}
	return envs
}

// Initializer updates JobSet values for the initializer Job.
func (b *Builder) Initializer(trainJob *trainer.TrainJob) *Builder {
	for i, rJob := range b.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			// TODO (andreyvelich): Currently, we use initContainers for the initializers.
			// Once JobSet supports execution policy for the ReplicatedJobs, we should migrate to containers.
			// Ref: https://github.com/kubernetes-sigs/jobset/issues/672
			for j, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				// Update values for the dataset initializer container.
				if container.Name == constants.ContainerDatasetInitializer && trainJob.Spec.DatasetConfig != nil {
					// Update the dataset initializer envs.
					b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = mergeInitializerEnvs(
						trainJob.Spec.DatasetConfig.StorageUri,
						trainJob.Spec.DatasetConfig.Env,
						container.Env,
					)
					// Update the dataset initializer secret reference.
					if trainJob.Spec.DatasetConfig.SecretRef != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].EnvFrom = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].EnvFrom,
							corev1.EnvFromSource{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: *trainJob.Spec.DatasetConfig.SecretRef,
								},
							},
						)
					}
				}
				// TODO (andreyvelich): Add the model exporter when we support it.
				// Update values for the model initializer container.
				if container.Name == constants.ContainerModelInitializer && trainJob.Spec.ModelConfig != nil && trainJob.Spec.ModelConfig.Input != nil {
					// Update the model initializer envs.
					b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = mergeInitializerEnvs(
						trainJob.Spec.ModelConfig.Input.StorageUri,
						trainJob.Spec.ModelConfig.Input.Env,
						container.Env,
					)
					// Update the model initializer secret reference.
					if trainJob.Spec.ModelConfig.Input.SecretRef != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].EnvFrom = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].EnvFrom,
							corev1.EnvFromSource{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: *trainJob.Spec.ModelConfig.Input.SecretRef,
								},
							},
						)
					}

				}
			}
		}
	}
	return b
}

// Launcher updates JobSet values for the launcher Job.
func (b *Builder) Launcher(info *runtime.Info, trainJob *trainer.TrainJob) *Builder {
	for i, rJob := range b.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobLauncher {

			// Update the volumes for the launcher Job.
			b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes = append(
				b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes, info.Trainer.Volumes...)

			// Update values for the launcher container.
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerLauncher {
					// Update values from the Info object.
					if info.Trainer.Env != nil {
						// Update JobSet envs from the Info.
						envNames := sets.New[string]()
						for _, env := range info.Trainer.Env {
							envNames.Insert(env.Name)
						}
						trainerEnvs := info.Trainer.Env
						// Info envs take precedence over the TrainingRuntime envs.
						for _, env := range container.Env {
							if !envNames.Has(env.Name) {
								trainerEnvs = append(trainerEnvs, env)
							}
						}
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env = trainerEnvs
					}
					// Update the launcher container port.
					if info.Trainer.ContainerPort != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports, *info.Trainer.ContainerPort)
					}
					// Update the launcher container volume mounts.
					if info.Trainer.VolumeMounts != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts, info.Trainer.VolumeMounts...)
					}
				}
			}
		}
	}
	return b
}

// Trainer updates JobSet values for the trainer Job.
func (b *Builder) Trainer(info *runtime.Info, trainJob *trainer.TrainJob) *Builder {
	for i, rJob := range b.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			// Update the Parallelism and Completions values for the Trainer Job.
			b.Spec.ReplicatedJobs[i].Template.Spec.Parallelism = info.Trainer.NumNodes
			b.Spec.ReplicatedJobs[i].Template.Spec.Completions = info.Trainer.NumNodes

			// Update the volumes for the Trainer Job.
			b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes = append(
				b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes, info.Trainer.Volumes...)

			// Update values for the Trainer container.
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerTrainer {
					// Update values from the TrainJob trainer.
					if trainJob.Spec.Trainer != nil {
						if trainJob.Spec.Trainer.Image != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Image = *trainJob.Spec.Trainer.Image
						}
						if trainJob.Spec.Trainer.Command != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Command = trainJob.Spec.Trainer.Command
						}
						if trainJob.Spec.Trainer.Args != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Args = trainJob.Spec.Trainer.Args
						}
						if trainJob.Spec.Trainer.ResourcesPerNode != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Resources = *trainJob.Spec.Trainer.ResourcesPerNode
						}
					}
					// Update values from the Info object.
					if info.Trainer.Env != nil {
						// Update JobSet envs from the Info.
						envNames := sets.New[string]()
						for _, env := range info.Trainer.Env {
							envNames.Insert(env.Name)
						}
						trainerEnvs := info.Trainer.Env
						// Info envs take precedence over the TrainingRuntime envs.
						for _, env := range container.Env {
							if !envNames.Has(env.Name) {
								trainerEnvs = append(trainerEnvs, env)
							}
						}
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env = trainerEnvs
					}
					// Update the Trainer container port.
					if info.Trainer.ContainerPort != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports, *info.Trainer.ContainerPort)
					}
					// Update the Trainer container volume mounts.
					if info.Trainer.VolumeMounts != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts, info.Trainer.VolumeMounts...)
					}
				}
			}
		}
	}
	return b
}

// TODO: Supporting merge labels would be great.
func (b *Builder) PodLabels(labels map[string]string) *Builder {
	for i := range b.Spec.ReplicatedJobs {
		b.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels = labels
	}
	return b
}

func (b *Builder) Suspend(suspend *bool) *Builder {
	b.Spec.Suspend = suspend
	return b
}

// TODO: Need to support all TrainJob fields.

func (b *Builder) Build() *jobsetv1alpha2.JobSet {
	return &b.JobSet
}
