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
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/apply"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
)

type Builder struct {
	*jobsetv1alpha2ac.JobSetApplyConfiguration
}

func NewBuilder(jobSet *jobsetv1alpha2ac.JobSetApplyConfiguration) *Builder {
	return &Builder{
		JobSetApplyConfiguration: jobSet,
	}
}

// Initializer updates JobSet values for the initializer Job.
func (b *Builder) Initializer(trainJob *trainer.TrainJob) *Builder {
	for i, rJob := range b.Spec.ReplicatedJobs {
		if *rJob.Name == constants.JobInitializer {
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				// Update values for the dataset initializer container.
				if *container.Name == constants.ContainerDatasetInitializer && trainJob.Spec.DatasetConfig != nil {
					env := &b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env
					// Update the dataset initializer envs.
					if storageUri := trainJob.Spec.DatasetConfig.StorageUri; storageUri != nil {
						apply.UpsertEnvVar(env, corev1ac.EnvVar().
							WithName(InitializerEnvStorageUri).
							WithValue(*storageUri))
					}
					apply.UpsertEnvVars(env, apply.EnvVars(trainJob.Spec.DatasetConfig.Env...))
					// Update the dataset initializer secret reference.
					if trainJob.Spec.DatasetConfig.SecretRef != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].
							WithEnvFrom(corev1ac.EnvFromSource().
								WithSecretRef(corev1ac.SecretEnvSource().
									WithName(trainJob.Spec.DatasetConfig.SecretRef.Name)))
					}
				}
				// TODO (andreyvelich): Add the model exporter when we support it.
				// Update values for the model initializer container.
				if *container.Name == constants.ContainerModelInitializer &&
					trainJob.Spec.ModelConfig != nil &&
					trainJob.Spec.ModelConfig.Input != nil {
					// Update the model initializer envs.
					env := &b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env
					if storageUri := trainJob.Spec.ModelConfig.Input.StorageUri; storageUri != nil {
						apply.UpsertEnvVar(env, corev1ac.EnvVar().
							WithName(InitializerEnvStorageUri).
							WithValue(*storageUri))
					}
					apply.UpsertEnvVars(env, apply.EnvVars(trainJob.Spec.ModelConfig.Input.Env...))
					// Update the model initializer secret reference.
					if trainJob.Spec.ModelConfig.Input.SecretRef != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].
							WithEnvFrom(corev1ac.EnvFromSource().
								WithSecretRef(corev1ac.SecretEnvSource().
									WithName(trainJob.Spec.ModelConfig.Input.SecretRef.Name)))
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
		if *rJob.Name == constants.JobLauncher {

			// Update the volumes for the Trainer Job.
			apply.UpsertVolumes(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes, info.Trainer.Volumes)

			// Update values for the launcher container.
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if *container.Name == constants.ContainerLauncher {
					// Update values from the Info object.
					if env := info.Trainer.Env; env != nil {
						// Update JobSet envs from the Info.
						apply.UpsertEnvVars(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env, env)
					}

					// Update the launcher container port.
					if port := info.Trainer.ContainerPort; port != nil {
						apply.UpsertPort(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports, port)
					}
					// Update the launcher container volume mounts.
					if mounts := info.Trainer.VolumeMounts; mounts != nil {
						apply.UpsertVolumeMounts(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts, mounts)
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
		if *rJob.Name == constants.JobTrainerNode {
			// Update the Parallelism and Completions values for the Trainer Job.
			b.Spec.ReplicatedJobs[i].Template.Spec.Parallelism = info.Trainer.NumNodes
			b.Spec.ReplicatedJobs[i].Template.Spec.Completions = info.Trainer.NumNodes

			// Update the volumes for the Trainer Job.
			apply.UpsertVolumes(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes, info.Trainer.Volumes)

			// Update values for the Trainer container.
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if *container.Name == constants.ContainerTrainer {
					// Update values from the TrainJob trainer.
					if trainJob.Spec.Trainer != nil {
						if image := trainJob.Spec.Trainer.Image; image != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Image = image
						}
						if command := trainJob.Spec.Trainer.Command; command != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Command = command
						}
						if args := trainJob.Spec.Trainer.Args; args != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Args = args
						}
						if resourcesPerNode := trainJob.Spec.Trainer.ResourcesPerNode; resourcesPerNode != nil &&
							(resourcesPerNode.Limits != nil || resourcesPerNode.Requests != nil) {
							requirements := corev1ac.ResourceRequirements()
							if limits := resourcesPerNode.Limits; limits != nil {
								requirements.WithLimits(limits)
							}
							if requests := resourcesPerNode.Requests; requests != nil {
								requirements.WithRequests(requests)
							}
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].
								WithResources(requirements)
						}
					}
					// Update values from the Info object.
					if env := info.Trainer.Env; env != nil {
						// Update JobSet envs from the Info.
						apply.UpsertEnvVars(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env, env)
					}
					// Update the Trainer container port.
					if port := info.Trainer.ContainerPort; port != nil {
						apply.UpsertPort(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports, port)
					}
					// Update the Trainer container volume mounts.
					if mounts := info.Trainer.VolumeMounts; mounts != nil {
						apply.UpsertVolumeMounts(&b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts, mounts)
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
		b.Spec.ReplicatedJobs[i].Template.Spec.Template.WithLabels(labels)
	}
	return b
}

func (b *Builder) Suspend(suspend *bool) *Builder {
	b.Spec.Suspend = suspend
	return b
}

// TODO: Need to support all TrainJob fields.

func (b *Builder) Build() *jobsetv1alpha2ac.JobSetApplyConfiguration {
	return b.JobSetApplyConfiguration
}
