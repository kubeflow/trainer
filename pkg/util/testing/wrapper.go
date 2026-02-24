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

package testing

import (
	"slices"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobsetplgconsts "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jobset/constants"
)

type JobSetWrapper struct {
	jobsetv1alpha2.JobSet
}

func MakeJobSetWrapper(namespace, name string) *JobSetWrapper {
	return &JobSetWrapper{
		JobSet: jobsetv1alpha2.JobSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: jobsetv1alpha2.SchemeGroupVersion.String(),
				Kind:       constants.JobSetKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: jobsetv1alpha2.JobSetSpec{
				ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
					{
						Name:      constants.DatasetInitializer,
						GroupName: "default",
						Template: batchv1.JobTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									constants.LabelTrainJobAncestor: constants.DatasetInitializer,
								},
							},
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: constants.DatasetInitializer,
												VolumeMounts: []corev1.VolumeMount{{
													Name:      jobsetplgconsts.VolumeNameInitializer,
													MountPath: constants.DatasetMountPath,
												}},
											},
										},
										Volumes: []corev1.Volume{{
											Name: jobsetplgconsts.VolumeNameInitializer,
											VolumeSource: corev1.VolumeSource{
												PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
													ClaimName: jobsetplgconsts.VolumeNameInitializer,
												},
											},
										}},
									},
								},
							},
						},
					},
					{
						Name:      constants.ModelInitializer,
						GroupName: "default",
						Template: batchv1.JobTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									constants.LabelTrainJobAncestor: constants.ModelInitializer,
								},
							},
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: constants.ModelInitializer,
												VolumeMounts: []corev1.VolumeMount{{
													Name:      jobsetplgconsts.VolumeNameInitializer,
													MountPath: constants.ModelMountPath,
												}},
											},
										},
										Volumes: []corev1.Volume{{
											Name: jobsetplgconsts.VolumeNameInitializer,
											VolumeSource: corev1.VolumeSource{
												PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
													ClaimName: jobsetplgconsts.VolumeNameInitializer,
												},
											},
										}},
									},
								},
							},
						},
					},
					{
						Name:      constants.Node,
						GroupName: "default",
						Template: batchv1.JobTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									constants.LabelTrainJobAncestor: constants.AncestorTrainer,
								},
							},
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: constants.Node,
												VolumeMounts: []corev1.VolumeMount{
													{
														Name:      jobsetplgconsts.VolumeNameInitializer,
														MountPath: constants.DatasetMountPath,
													},
													{
														Name:      jobsetplgconsts.VolumeNameInitializer,
														MountPath: constants.ModelMountPath,
													},
												},
											},
										},
										Volumes: []corev1.Volume{{
											Name: jobsetplgconsts.VolumeNameInitializer,
											VolumeSource: corev1.VolumeSource{
												PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
													ClaimName: jobsetplgconsts.VolumeNameInitializer,
												},
											},
										}},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (j *JobSetWrapper) Replicas(replicas int32, rJobNames ...string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if slices.Contains(rJobNames, rJob.Name) {
			j.Spec.ReplicatedJobs[i].Replicas = replicas
		}
	}
	return j
}

func (j *JobSetWrapper) NumNodes(numNodes int32) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.Node {
			j.Spec.ReplicatedJobs[i].Template.Spec.Parallelism = &numNodes
			j.Spec.ReplicatedJobs[i].Template.Spec.Completions = &numNodes
		}
	}
	return j
}

func (j *JobSetWrapper) Parallelism(p int32, rJobNames ...string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if slices.Contains(rJobNames, rJob.Name) {
			j.Spec.ReplicatedJobs[i].Template.Spec.Parallelism = &p
		}
	}
	return j
}

func (j *JobSetWrapper) Completions(c int32, rJobNames ...string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if slices.Contains(rJobNames, rJob.Name) {
			j.Spec.ReplicatedJobs[i].Template.Spec.Completions = &c
		}
	}
	return j
}

func (j *JobSetWrapper) LauncherReplica() *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.Node {
			j.Spec.ReplicatedJobs = append(j.Spec.ReplicatedJobs, jobsetv1alpha2.ReplicatedJob{})
			copy(j.Spec.ReplicatedJobs[i+1:], j.Spec.ReplicatedJobs[i:])
			j.Spec.ReplicatedJobs[i] = jobsetv1alpha2.ReplicatedJob{
				Name:      constants.Launcher,
				GroupName: "default",
				Template: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:    constants.Node,
									Command: []string{"mpirun"},
									Args:    []string{"echo.sh"},
								}},
							},
						},
					},
				},
			}
		}
	}
	return j
}

func (j *JobSetWrapper) InitContainer(rJobName, containerName, image string, envs ...corev1.EnvVar) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers = []corev1.Container{
				{
					Name:  containerName,
					Image: image,
					Env:   envs,
				},
			}
		}
	}
	return j
}

func (j *JobSetWrapper) Container(rJobName, containerName, image string, command []string, args []string, res corev1.ResourceList) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for k, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Image = image
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Command = command
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Args = args
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Resources.Requests = res
					return j
				}
			}

			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers = append(
				j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers,
				[]corev1.Container{
					{
						Name:      containerName,
						Image:     image,
						Command:   command,
						Args:      args,
						Resources: corev1.ResourceRequirements{Requests: res},
					},
				}...,
			)
		}
	}
	return j
}

func (j *JobSetWrapper) ReplaceContainer(rJobName, containerName, newContainerName, image string, command, args []string, res corev1.ResourceList) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for k, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k] = corev1.Container{
						Name:      newContainerName,
						Image:     image,
						Command:   command,
						Args:      args,
						Resources: corev1.ResourceRequirements{Requests: res},
					}
					return j
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) ContainerTrainerPorts(ports []corev1.ContainerPort) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.Node {
			for k, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.Node {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Ports = ports
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) NodeSelector(rJobName string, selector map[string]string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			// NodeSelector field is atomic
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.NodeSelector = selector
		}
	}
	return j
}

func (j *JobSetWrapper) Affinity(rJobName string, affinity corev1.Affinity) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Affinity = &affinity
		}
	}
	return j
}

func (j *JobSetWrapper) SchedulingGates(rJobName string, schedulingGates ...corev1.PodSchedulingGate) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.SchedulingGates = append(
				j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.SchedulingGates,
				schedulingGates...,
			)
		}
	}
	return j
}

func (j *JobSetWrapper) ImagePullSecrets(rJobName string, imagePullSecrets ...corev1.LocalObjectReference) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.ImagePullSecrets = append(
				j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.ImagePullSecrets,
				imagePullSecrets...,
			)
		}
	}
	return j
}

func (j *JobSetWrapper) Tolerations(rJobName string, tolerations ...corev1.Toleration) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Tolerations = append(
				j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Tolerations,
				tolerations...,
			)
		}
	}
	return j
}

func (j *JobSetWrapper) PodSecurityContext(rJobName string, securityContext corev1.PodSecurityContext) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.SecurityContext = &securityContext
		}
	}
	return j
}

func (j *JobSetWrapper) Volumes(rJobName string, v ...corev1.Volume) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes = append(
				j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes,
				v...,
			)
		}
	}
	return j
}

func (j *JobSetWrapper) VolumeMounts(rJobName, containerName string, vms ...corev1.VolumeMount) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for k, container := range j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].VolumeMounts = append(
						j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].VolumeMounts,
						vms...,
					)
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) Env(rJobName, containerName string, envs ...corev1.EnvVar) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for k, container := range j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Env = append(
						j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Env,
						envs...,
					)
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) ContainerSecurityContext(rJobName, containerName string, securityContext corev1.SecurityContext) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for k, container := range j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].SecurityContext = &securityContext
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) EnvFrom(rJobName, containerName string, envFrom ...corev1.EnvFromSource) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for k, container := range j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].EnvFrom = append(
						j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].EnvFrom,
						envFrom...,
					)
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) ServiceAccountName(rJobName string, serviceAccountName string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.ServiceAccountName = serviceAccountName

		}
	}
	return j
}

func (j *JobSetWrapper) Suspend(suspend bool) *JobSetWrapper {
	j.Spec.Suspend = &suspend
	return j
}

func (j *JobSetWrapper) ControllerReference(gvk schema.GroupVersionKind, name, uid string) *JobSetWrapper {
	j.OwnerReferences = append(j.OwnerReferences, metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                types.UID(uid),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	})
	return j
}

func (j *JobSetWrapper) ReplicatedJobLabel(key, value string, rJobNames ...string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if !slices.Contains(rJobNames, rJob.Name) {
			continue
		}

		if rJob.Template.Labels == nil {
			j.Spec.ReplicatedJobs[i].Template.Labels = make(map[string]string, 1)
		}
		j.Spec.ReplicatedJobs[i].Template.Labels[key] = value
	}
	return j
}

func (j *JobSetWrapper) PodLabel(key, value string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Template.Spec.Template.Labels == nil {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels = make(map[string]string, 1)
		}
		j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels[key] = value
	}
	return j
}

func (j *JobSetWrapper) PodAnnotation(key, value string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Template.Spec.Template.Annotations == nil {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations = make(map[string]string, 1)
		}
		j.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations[key] = value
	}
	return j
}

func (j *JobSetWrapper) PodLabelForJobs(key, value string, rJobNames ...string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		for _, rJobName := range rJobNames {
			if rJob.Name == rJobName {
				if rJob.Template.Spec.Template.Labels == nil {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels = make(map[string]string, 1)
				}
				j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels[key] = value
			}
		}
	}
	return j
}

func (j *JobSetWrapper) PodAnnotationForJobs(key, value string, rJobNames ...string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		for _, rJobName := range rJobNames {
			if rJob.Name == rJobName {
				if rJob.Template.Spec.Template.Annotations == nil {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations = make(map[string]string, 1)
				}
				j.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations[key] = value
			}
		}
	}
	return j
}

func (j *JobSetWrapper) PodPriorityClassName(value string) *JobSetWrapper {
	for i := range j.Spec.ReplicatedJobs {
		j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.PriorityClassName = value
	}
	return j
}

func (j *JobSetWrapper) Label(key, value string) *JobSetWrapper {
	if j.Labels == nil {
		j.Labels = make(map[string]string, 1)
	}
	j.Labels[key] = value
	return j
}

func (j *JobSetWrapper) Annotation(key, value string) *JobSetWrapper {
	if j.Annotations == nil {
		j.Annotations = make(map[string]string, 1)
	}
	j.Annotations[key] = value
	return j
}

func (j *JobSetWrapper) Conditions(conditions ...metav1.Condition) *JobSetWrapper {
	if len(conditions) != 0 {
		j.Status.Conditions = append(j.Status.Conditions, conditions...)
	}
	return j
}

func (j *JobSetWrapper) DependsOn(rJobName string, dependsOn ...jobsetv1alpha2.DependsOn) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			j.Spec.ReplicatedJobs[i].DependsOn = append(j.Spec.ReplicatedJobs[i].DependsOn, dependsOn...)
		}
	}
	return j
}

func (j *JobSetWrapper) ReplicatedJobsStatuses(statuses []jobsetv1alpha2.ReplicatedJobStatus) *JobSetWrapper {
	j.Status.ReplicatedJobsStatus = statuses
	return j
}

func (j *JobSetWrapper) Obj() *jobsetv1alpha2.JobSet {
	return &j.JobSet
}

type TrainJobWrapper struct {
	trainer.TrainJob
}

func MakeTrainJobWrapper(namespace, name string) *TrainJobWrapper {
	return &TrainJobWrapper{
		TrainJob: trainer.TrainJob{
			TypeMeta: metav1.TypeMeta{
				APIVersion: trainer.SchemeGroupVersion.Version,
				Kind:       trainer.TrainJobKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: trainer.TrainJobSpec{},
		},
	}
}

func (t *TrainJobWrapper) Suspend(suspend bool) *TrainJobWrapper {
	t.Spec.Suspend = &suspend
	return t
}

func (t *TrainJobWrapper) UID(uid string) *TrainJobWrapper {
	t.ObjectMeta.UID = types.UID(uid)
	return t
}

func (t *TrainJobWrapper) SpecLabel(key, value string) *TrainJobWrapper {
	if t.Spec.Labels == nil {
		t.Spec.Labels = make(map[string]string, 1)
	}
	t.Spec.Labels[key] = value
	return t
}

func (t *TrainJobWrapper) SpecAnnotation(key, value string) *TrainJobWrapper {
	if t.Spec.Annotations == nil {
		t.Spec.Annotations = make(map[string]string, 1)
	}
	t.Spec.Annotations[key] = value
	return t
}

func (t *TrainJobWrapper) RuntimeRef(gvk schema.GroupVersionKind, name string) *TrainJobWrapper {
	runtimeRef := trainer.RuntimeRef{
		Name: name,
	}
	if gvk.Group != "" {
		runtimeRef.APIGroup = &gvk.Group
	}
	if gvk.Kind != "" {
		runtimeRef.Kind = &gvk.Kind
	}
	t.Spec.RuntimeRef = runtimeRef
	return t
}

func (t *TrainJobWrapper) Initializer(initializer *trainer.Initializer) *TrainJobWrapper {
	t.Spec.Initializer = initializer
	return t
}

func (t *TrainJobWrapper) Trainer(trainer *trainer.Trainer) *TrainJobWrapper {
	t.Spec.Trainer = trainer
	return t
}

func (t *TrainJobWrapper) PodTemplateOverrides(podTemplateOverrides []trainer.PodTemplateOverride) *TrainJobWrapper {
	t.Spec.PodTemplateOverrides = podTemplateOverrides
	return t
}

func (t *TrainJobWrapper) ManagedBy(m string) *TrainJobWrapper {
	t.Spec.ManagedBy = &m
	return t
}

func (t *TrainJobWrapper) Obj() *trainer.TrainJob {
	return &t.TrainJob
}

type TrainJobTrainerWrapper struct {
	trainer.Trainer
}

func MakeTrainJobTrainerWrapper() *TrainJobTrainerWrapper {
	return &TrainJobTrainerWrapper{
		Trainer: trainer.Trainer{},
	}
}

func (t *TrainJobTrainerWrapper) NumNodes(numNodes int32) *TrainJobTrainerWrapper {
	t.Trainer.NumNodes = &numNodes
	return t
}

func (t *TrainJobTrainerWrapper) NumProcPerNode(numProcPerNode intstr.IntOrString) *TrainJobTrainerWrapper {
	t.Trainer.NumProcPerNode = &numProcPerNode
	return t
}

func (t *TrainJobTrainerWrapper) Container(image string, command []string, args []string, resRequests corev1.ResourceList) *TrainJobTrainerWrapper {
	t.Image = &image
	t.Command = command
	t.Args = args
	t.ResourcesPerNode = &corev1.ResourceRequirements{
		Requests: resRequests,
	}
	return t
}

func (t *TrainJobTrainerWrapper) Env(env ...corev1.EnvVar) *TrainJobTrainerWrapper {
	t.Trainer.Env = env
	return t
}

func (t *TrainJobTrainerWrapper) Obj() *trainer.Trainer {
	return &t.Trainer
}

type TrainJobInitializerWrapper struct {
	trainer.Initializer
}

func MakeTrainJobInitializerWrapper() *TrainJobInitializerWrapper {
	return &TrainJobInitializerWrapper{
		Initializer: trainer.Initializer{},
	}
}

func (t *TrainJobInitializerWrapper) DatasetInitializer(datasetInitializer *trainer.DatasetInitializer) *TrainJobInitializerWrapper {
	t.Dataset = datasetInitializer
	return t
}

func (t *TrainJobInitializerWrapper) ModelInitializer(modelInitializer *trainer.ModelInitializer) *TrainJobInitializerWrapper {
	t.Model = modelInitializer
	return t
}

func (t *TrainJobInitializerWrapper) Obj() *trainer.Initializer {
	return &t.Initializer
}

type TrainJobDatasetInitializerWrapper struct {
	trainer.DatasetInitializer
}

func MakeTrainJobDatasetInitializerWrapper() *TrainJobDatasetInitializerWrapper {
	return &TrainJobDatasetInitializerWrapper{
		DatasetInitializer: trainer.DatasetInitializer{},
	}
}

func (t *TrainJobDatasetInitializerWrapper) StorageUri(storageUri string) *TrainJobDatasetInitializerWrapper {
	t.DatasetInitializer.StorageUri = &storageUri
	return t
}

func (t *TrainJobDatasetInitializerWrapper) Env(env ...corev1.EnvVar) *TrainJobDatasetInitializerWrapper {
	t.DatasetInitializer.Env = env
	return t
}

func (t *TrainJobDatasetInitializerWrapper) SecretRef(secretRef corev1.LocalObjectReference) *TrainJobDatasetInitializerWrapper {
	t.DatasetInitializer.SecretRef = &secretRef
	return t
}

func (t *TrainJobDatasetInitializerWrapper) Obj() *trainer.DatasetInitializer {
	return &t.DatasetInitializer
}

type TrainJobModelInitializerWrapper struct {
	trainer.ModelInitializer
}

func MakeTrainJobModelInitializerWrapper() *TrainJobModelInitializerWrapper {
	return &TrainJobModelInitializerWrapper{
		ModelInitializer: trainer.ModelInitializer{},
	}
}

func (t *TrainJobModelInitializerWrapper) StorageUri(storageUri string) *TrainJobModelInitializerWrapper {
	t.ModelInitializer.StorageUri = &storageUri
	return t
}

func (t *TrainJobModelInitializerWrapper) Env(env ...corev1.EnvVar) *TrainJobModelInitializerWrapper {
	t.ModelInitializer.Env = env
	return t
}

func (t *TrainJobModelInitializerWrapper) SecretRef(secretRef corev1.LocalObjectReference) *TrainJobModelInitializerWrapper {
	t.ModelInitializer.SecretRef = &secretRef
	return t
}

func (t *TrainJobModelInitializerWrapper) Obj() *trainer.ModelInitializer {
	return &t.ModelInitializer
}

type TrainingRuntimeWrapper struct {
	trainer.TrainingRuntime
}

func MakeTrainingRuntimeWrapper(namespace, name string) *TrainingRuntimeWrapper {
	return &TrainingRuntimeWrapper{
		TrainingRuntime: trainer.TrainingRuntime{
			TypeMeta: metav1.TypeMeta{
				APIVersion: trainer.SchemeGroupVersion.String(),
				Kind:       trainer.TrainingRuntimeKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: trainer.TrainingRuntimeSpec{
				Template: trainer.JobSetTemplateSpec{
					Spec: jobsetv1alpha2.JobSetSpec{
						ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
							{
								Name:      constants.DatasetInitializer,
								GroupName: "default",
								Template: batchv1.JobTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											constants.LabelTrainJobAncestor: constants.DatasetInitializer,
										},
									},
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.DatasetInitializer,
														VolumeMounts: []corev1.VolumeMount{{
															Name:      jobsetplgconsts.VolumeNameInitializer,
															MountPath: constants.DatasetMountPath,
														}},
													},
												},
												Volumes: []corev1.Volume{{
													Name: jobsetplgconsts.VolumeNameInitializer,
													VolumeSource: corev1.VolumeSource{
														PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
															ClaimName: jobsetplgconsts.VolumeNameInitializer,
														},
													},
												}},
											},
										},
									},
								},
							},
							{
								Name:      constants.ModelInitializer,
								GroupName: "default",
								Template: batchv1.JobTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											constants.LabelTrainJobAncestor: constants.ModelInitializer,
										},
									},
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.ModelInitializer,
														VolumeMounts: []corev1.VolumeMount{{
															Name:      jobsetplgconsts.VolumeNameInitializer,
															MountPath: constants.ModelMountPath,
														}},
													},
												},
												Volumes: []corev1.Volume{{
													Name: jobsetplgconsts.VolumeNameInitializer,
													VolumeSource: corev1.VolumeSource{
														PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
															ClaimName: jobsetplgconsts.VolumeNameInitializer,
														},
													},
												}},
											},
										},
									},
								},
							},
							{
								Name:      constants.Node,
								GroupName: "default",
								Template: batchv1.JobTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											constants.LabelTrainJobAncestor: constants.AncestorTrainer,
										},
									},
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.Node,
														VolumeMounts: []corev1.VolumeMount{
															{
																Name:      jobsetplgconsts.VolumeNameInitializer,
																MountPath: constants.DatasetMountPath,
															},
															{
																Name:      jobsetplgconsts.VolumeNameInitializer,
																MountPath: constants.ModelMountPath,
															},
														},
													},
												},
												Volumes: []corev1.Volume{{
													Name: jobsetplgconsts.VolumeNameInitializer,
													VolumeSource: corev1.VolumeSource{
														PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
															ClaimName: jobsetplgconsts.VolumeNameInitializer,
														},
													},
												}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *TrainingRuntimeWrapper) Label(key, value string) *TrainingRuntimeWrapper {
	if r.Labels == nil {
		r.Labels = make(map[string]string, 1)
	}
	r.Labels[key] = value
	return r
}

func (r *TrainingRuntimeWrapper) Annotation(key, value string) *TrainingRuntimeWrapper {
	if r.Annotations == nil {
		r.Annotations = make(map[string]string, 1)
	}
	r.Annotations[key] = value
	return r
}

func (r *TrainingRuntimeWrapper) Finalizers(f ...string) *TrainingRuntimeWrapper {
	r.ObjectMeta.Finalizers = append(r.ObjectMeta.Finalizers, f...)
	return r
}

func (r *TrainingRuntimeWrapper) DeletionTimestamp(t metav1.Time) *TrainingRuntimeWrapper {
	r.ObjectMeta.DeletionTimestamp = &t
	return r
}

func (r *TrainingRuntimeWrapper) RuntimeSpec(spec trainer.TrainingRuntimeSpec) *TrainingRuntimeWrapper {
	r.Spec = spec
	return r
}

func (r *TrainingRuntimeWrapper) Obj() *trainer.TrainingRuntime {
	return &r.TrainingRuntime
}

type ClusterTrainingRuntimeWrapper struct {
	trainer.ClusterTrainingRuntime
}

func MakeClusterTrainingRuntimeWrapper(name string) *ClusterTrainingRuntimeWrapper {
	return &ClusterTrainingRuntimeWrapper{
		ClusterTrainingRuntime: trainer.ClusterTrainingRuntime{
			TypeMeta: metav1.TypeMeta{
				APIVersion: trainer.SchemeGroupVersion.String(),
				Kind:       trainer.ClusterTrainingRuntimeKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: trainer.TrainingRuntimeSpec{
				Template: trainer.JobSetTemplateSpec{
					Spec: jobsetv1alpha2.JobSetSpec{
						ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
							{
								Name:      constants.DatasetInitializer,
								GroupName: "default",
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.DatasetInitializer,
														VolumeMounts: []corev1.VolumeMount{{
															Name:      jobsetplgconsts.VolumeNameInitializer,
															MountPath: constants.DatasetMountPath,
														}},
													},
												},
												Volumes: []corev1.Volume{{
													Name: jobsetplgconsts.VolumeNameInitializer,
													VolumeSource: corev1.VolumeSource{
														PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
															ClaimName: jobsetplgconsts.VolumeNameInitializer,
														},
													},
												}},
											},
										},
									},
								},
							},
							{
								Name:      constants.ModelInitializer,
								GroupName: "default",
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.ModelInitializer,
														VolumeMounts: []corev1.VolumeMount{{
															Name:      jobsetplgconsts.VolumeNameInitializer,
															MountPath: constants.ModelMountPath,
														}},
													},
												},
												Volumes: []corev1.Volume{{
													Name: jobsetplgconsts.VolumeNameInitializer,
													VolumeSource: corev1.VolumeSource{
														PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
															ClaimName: jobsetplgconsts.VolumeNameInitializer,
														},
													},
												}},
											},
										},
									},
								},
							},
							{
								Name:      constants.Node,
								GroupName: "default",
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.Node,
														VolumeMounts: []corev1.VolumeMount{
															{
																Name:      jobsetplgconsts.VolumeNameInitializer,
																MountPath: constants.DatasetMountPath,
															},
															{
																Name:      jobsetplgconsts.VolumeNameInitializer,
																MountPath: constants.ModelMountPath,
															},
														},
													},
												},
												Volumes: []corev1.Volume{{
													Name: jobsetplgconsts.VolumeNameInitializer,
													VolumeSource: corev1.VolumeSource{
														PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
															ClaimName: jobsetplgconsts.VolumeNameInitializer,
														},
													},
												}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *ClusterTrainingRuntimeWrapper) Finalizers(f ...string) *ClusterTrainingRuntimeWrapper {
	r.ObjectMeta.Finalizers = append(r.ObjectMeta.Finalizers, f...)
	return r
}

func (r *ClusterTrainingRuntimeWrapper) DeletionTimestamp(t metav1.Time) *ClusterTrainingRuntimeWrapper {
	r.ObjectMeta.DeletionTimestamp = &t
	return r
}

func (r *ClusterTrainingRuntimeWrapper) RuntimeSpec(spec trainer.TrainingRuntimeSpec) *ClusterTrainingRuntimeWrapper {
	r.Spec = spec
	return r
}

func (r *ClusterTrainingRuntimeWrapper) Obj() *trainer.ClusterTrainingRuntime {
	return &r.ClusterTrainingRuntime
}

type TrainingRuntimeSpecWrapper struct {
	trainer.TrainingRuntimeSpec
}

func MakeTrainingRuntimeSpecWrapper(spec trainer.TrainingRuntimeSpec) *TrainingRuntimeSpecWrapper {
	return &TrainingRuntimeSpecWrapper{
		TrainingRuntimeSpec: spec,
	}
}

func (s *TrainingRuntimeSpecWrapper) JobSetSpec(spec jobsetv1alpha2.JobSetSpec) *TrainingRuntimeSpecWrapper {
	s.Template.Spec = spec
	return s
}

func (s *TrainingRuntimeSpecWrapper) WithMLPolicy(mlPolicy *trainer.MLPolicy) *TrainingRuntimeSpecWrapper {
	s.MLPolicy = mlPolicy
	return s
}

func (s *TrainingRuntimeSpecWrapper) LauncherReplica() *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if rJob.Name == constants.Node {
			s.Template.Spec.ReplicatedJobs = append(s.Template.Spec.ReplicatedJobs, jobsetv1alpha2.ReplicatedJob{})
			copy(s.Template.Spec.ReplicatedJobs[i+1:], s.Template.Spec.ReplicatedJobs[i:])
			s.Template.Spec.ReplicatedJobs[i] = jobsetv1alpha2.ReplicatedJob{
				Name:      constants.Launcher,
				GroupName: "default",
				Template: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:    constants.Node,
									Command: []string{"mpirun"},
									Args:    []string{"echo.sh"},
								}},
							},
						},
					},
				},
			}
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) Replicas(replicas int32, rJobNames ...string) *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if slices.Contains(rJobNames, rJob.Name) {
			s.Template.Spec.ReplicatedJobs[i].Replicas = replicas
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) InitContainer(rJobName, containerName, image string, envs ...corev1.EnvVar) *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers = []corev1.Container{
				{
					Name:  containerName,
					Image: image,
					Env:   envs,
				},
			}
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) Container(rJobName, containerName, image string, command []string, args []string, res corev1.ResourceList) *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Image = image
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Command = command
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Args = args
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Resources.Requests = res
					return s
				}
			}
			s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers = append(
				s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers,
				[]corev1.Container{
					{
						Name:      containerName,
						Image:     image,
						Command:   command,
						Args:      args,
						Resources: corev1.ResourceRequirements{Requests: res},
					},
				}...,
			)
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) Env(rJobName, containerName string, envs ...corev1.EnvVar) *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if rJob.Name == rJobName {
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env = append(
						s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env,
						envs...)
				}
			}
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) PodGroupPolicyCoscheduling(src *trainer.CoschedulingPodGroupPolicySource) *TrainingRuntimeSpecWrapper {
	s.PodGroupPolicy = &trainer.PodGroupPolicy{
		PodGroupPolicySource: trainer.PodGroupPolicySource{
			Coscheduling: src,
		},
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) PodGroupPolicyCoschedulingSchedulingTimeout(timeout int32) *TrainingRuntimeSpecWrapper {
	if s.PodGroupPolicy == nil || s.PodGroupPolicy.Coscheduling == nil {
		return s.PodGroupPolicyCoscheduling(&trainer.CoschedulingPodGroupPolicySource{
			ScheduleTimeoutSeconds: &timeout,
		})
	}
	s.PodGroupPolicy.Coscheduling.ScheduleTimeoutSeconds = &timeout
	return s
}

func (s *TrainingRuntimeSpecWrapper) Obj() trainer.TrainingRuntimeSpec {
	return s.TrainingRuntimeSpec
}

type MLPolicyWrapper struct {
	trainer.MLPolicy
}

func MakeMLPolicyWrapper() *MLPolicyWrapper {
	return &MLPolicyWrapper{
		MLPolicy: trainer.MLPolicy{},
	}
}

func (m *MLPolicyWrapper) WithNumNodes(numNodes int32) *MLPolicyWrapper {
	m.NumNodes = &numNodes
	return m
}

func (m *MLPolicyWrapper) WithMLPolicySource(source trainer.MLPolicySource) *MLPolicyWrapper {
	m.MLPolicySource = source
	return m
}

func (m *MLPolicyWrapper) Obj() *trainer.MLPolicy {
	return &m.MLPolicy
}

type MLPolicySourceWrapper struct {
	trainer.MLPolicySource
}

func MakeMLPolicySourceWrapper() *MLPolicySourceWrapper {
	return &MLPolicySourceWrapper{}
}

func (m *MLPolicySourceWrapper) TorchPolicy(numProcPerNode *intstr.IntOrString) *MLPolicySourceWrapper {
	if m.Torch == nil {
		m.Torch = &trainer.TorchMLPolicySource{}
	}
	m.Torch = &trainer.TorchMLPolicySource{
		NumProcPerNode: numProcPerNode,
	}
	return m
}

func (m *MLPolicySourceWrapper) MPIPolicy(numProcPerNode *int32, MPImplementation trainer.MPIImplementation, sshAuthMountPath *string, runLauncherAsNode *bool) *MLPolicySourceWrapper {
	if m.MPI == nil {
		m.MPI = &trainer.MPIMLPolicySource{}
	}
	m.MPI.NumProcPerNode = numProcPerNode
	m.MPI.MPIImplementation = &MPImplementation
	m.MPI.SSHAuthMountPath = sshAuthMountPath
	m.MPI.RunLauncherAsNode = runLauncherAsNode
	return m
}

func (m *MLPolicySourceWrapper) Obj() *trainer.MLPolicySource {
	return &m.MLPolicySource
}

type SchedulerPluginsPodGroupWrapper struct {
	schedulerpluginsv1alpha1.PodGroup
}

func MakeSchedulerPluginsPodGroup(namespace, name string) *SchedulerPluginsPodGroupWrapper {
	return &SchedulerPluginsPodGroupWrapper{
		PodGroup: schedulerpluginsv1alpha1.PodGroup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: schedulerpluginsv1alpha1.SchemeGroupVersion.String(),
				Kind:       constants.PodGroupKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (p *SchedulerPluginsPodGroupWrapper) MinMember(members int32) *SchedulerPluginsPodGroupWrapper {
	p.Spec.MinMember = members
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) MinResources(resources corev1.ResourceList) *SchedulerPluginsPodGroupWrapper {
	p.Spec.MinResources = resources
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) SchedulingTimeout(timeout int32) *SchedulerPluginsPodGroupWrapper {
	p.Spec.ScheduleTimeoutSeconds = &timeout
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) ControllerReference(gvk schema.GroupVersionKind, name, uid string) *SchedulerPluginsPodGroupWrapper {
	p.OwnerReferences = append(p.OwnerReferences, metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                types.UID(uid),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	})
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) Obj() *schedulerpluginsv1alpha1.PodGroup {
	return &p.PodGroup
}

type VolcanoPodGroupWrapper struct {
	volcanov1beta1.PodGroup
}

func MakeVolcanoPodGroup(namespace, name string) *VolcanoPodGroupWrapper {
	return &VolcanoPodGroupWrapper{
		PodGroup: volcanov1beta1.PodGroup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: volcanov1beta1.SchemeGroupVersion.String(),
				Kind:       "PodGroup",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (p *VolcanoPodGroupWrapper) MinMember(members int32) *VolcanoPodGroupWrapper {
	p.Spec.MinMember = members
	return p
}

func (p *VolcanoPodGroupWrapper) MinResources(resources *corev1.ResourceList) *VolcanoPodGroupWrapper {
	p.Spec.MinResources = resources
	return p
}

func (p *VolcanoPodGroupWrapper) Queue(queue string) *VolcanoPodGroupWrapper {
	p.Spec.Queue = queue
	return p
}

func (p *VolcanoPodGroupWrapper) PriorityClassName(pc string) *VolcanoPodGroupWrapper {
	p.Spec.PriorityClassName = pc
	return p
}

func (p *VolcanoPodGroupWrapper) NetworkTopology(mode volcanov1beta1.NetworkTopologyMode, highestTier int) *VolcanoPodGroupWrapper {
	p.Spec.NetworkTopology = &volcanov1beta1.NetworkTopologySpec{
		Mode:               mode,
		HighestTierAllowed: &highestTier,
	}
	return p
}

func (p *VolcanoPodGroupWrapper) ControllerReference(gvk schema.GroupVersionKind, name, uid string) *VolcanoPodGroupWrapper {
	owner := *metav1.NewControllerRef(&trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      name,
			UID:       types.UID(uid),
		},
	}, gvk)
	p.OwnerReferences = append(p.OwnerReferences, owner)
	return p
}

func (p *VolcanoPodGroupWrapper) Obj() *volcanov1beta1.PodGroup {
	return &p.PodGroup
}

type ConfigMapWrapper struct {
	corev1.ConfigMap
}

func MakeConfigMapWrapper(name, ns string) *ConfigMapWrapper {
	return &ConfigMapWrapper{
		ConfigMap: corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

func (c *ConfigMapWrapper) WithData(data map[string]string) *ConfigMapWrapper {
	if c.Data == nil {
		c.Data = make(map[string]string, len(data))
	}
	for k, v := range data {
		c.Data[k] = v
	}
	return c
}

func (c *ConfigMapWrapper) ControllerReference(gvk schema.GroupVersionKind, name, uid string) *ConfigMapWrapper {
	c.OwnerReferences = append(c.OwnerReferences, metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                types.UID(uid),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	})
	return c
}

func (c *ConfigMapWrapper) Obj() *corev1.ConfigMap {
	return &c.ConfigMap
}

type SecretWrapper struct {
	corev1.Secret
}

func MakeSecretWrapper(name, ns string) *SecretWrapper {
	return &SecretWrapper{
		Secret: corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

func (s *SecretWrapper) WithType(t corev1.SecretType) *SecretWrapper {
	s.Type = t
	return s
}

func (s *SecretWrapper) WithData(data map[string][]byte) *SecretWrapper {
	if s.Data == nil {
		s.Data = make(map[string][]byte, len(data))
	}
	for k, v := range data {
		s.Data[k] = v
	}
	return s
}

func (s *SecretWrapper) WithImmutable(immutable bool) *SecretWrapper {
	s.Immutable = &immutable
	return s
}

func (s *SecretWrapper) ControllerReference(gvk schema.GroupVersionKind, name, uid string) *SecretWrapper {
	s.OwnerReferences = append(s.OwnerReferences, metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                types.UID(uid),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	})
	return s
}

func (s *SecretWrapper) Obj() *corev1.Secret {
	return &s.Secret
}

// JAXPolicy sets the JAX policy source
func (w *MLPolicySourceWrapper) JAXPolicy() *MLPolicySourceWrapper {
	w.JAX = &trainer.JAXMLPolicySource{}
	return w
}
