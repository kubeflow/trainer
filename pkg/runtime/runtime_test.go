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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	resourcehelpers "k8s.io/component-helpers/resource"
	"k8s.io/utils/ptr"

	"github.com/kubeflow/trainer/pkg/constants"
	jobsetplgconsts "github.com/kubeflow/trainer/pkg/runtime/framework/plugins/jobset/constants"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"
)

func TestNewInfo(t *testing.T) {
	cases := map[string]struct {
		infoOpts []InfoOption
		wantInfo *Info
	}{
		"all arguments are specified": {
			infoOpts: []InfoOption{
				WithLabels(map[string]string{
					"labelKey": "labelValue",
				}),
				WithAnnotations(map[string]string{
					"annotationKey": "annotationValue",
				}),
				WithPodSpecReplicas(constants.JobInitializer, 1, resourcehelpers.PodRequests(&corev1.Pod{
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name: constants.ContainerModelInitializer,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("10"),
							},
						}}}, InitContainers: []corev1.Container{{
						RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("5"),
							},
						}}},
					},
				}, resourcehelpers.PodResourcesOptions{}), corev1ac.PodSpec().
					WithContainers(
						corev1ac.Container().
							WithName(constants.ContainerModelInitializer).
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("10"),
								})),
					).
					WithInitContainers(
						corev1ac.Container().
							WithRestartPolicy(corev1.ContainerRestartPolicyAlways).
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("5"),
								})),
					),
				),
				WithPodSpecReplicas(constants.JobTrainerNode, 10, resourcehelpers.PodRequests(&corev1.Pod{
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name: constants.ContainerTrainer,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							},
						},
					}}, InitContainers: []corev1.Container{{
						Name:          "preparation",
						RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("25"),
							},
						},
					}}},
				}, resourcehelpers.PodResourcesOptions{}), corev1ac.PodSpec().
					WithContainers(
						corev1ac.Container().
							WithName(constants.ContainerTrainer).
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("25"),
								})).
							WithEnv(
								corev1ac.EnvVar().WithName("TEST").WithValue("TEST"),
							).
							WithPorts(corev1ac.ContainerPort().
								WithName("http").
								WithProtocol(corev1.ProtocolTCP).
								WithContainerPort(8080),
							).
							WithVolumeMounts(
								corev1ac.VolumeMount().
									WithName("TEST").
									WithMountPath("/var").
									WithReadOnly(true),
							),
					).
					WithInitContainers(corev1ac.Container().
						WithName("preparation").
						WithResources(corev1ac.ResourceRequirements().
							WithRequests(corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							})).
						WithRestartPolicy(corev1.ContainerRestartPolicyAlways),
					)),
				WithTemplateSpec(
					jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.JobInitializer).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.ContainerDatasetInitializer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.DatasetMountPath),
														).
														WithResources(corev1ac.ResourceRequirements()),
													corev1ac.Container().
														WithName(constants.ContainerModelInitializer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.ModelMountPath),
														).
														WithResources(corev1ac.ResourceRequirements()),
												).
												WithVolumes(corev1ac.Volume().
													WithName(jobsetplgconsts.VolumeNameInitializer).
													WithPersistentVolumeClaim(corev1ac.PersistentVolumeClaimVolumeSource().
														WithClaimName(jobsetplgconsts.VolumeNameInitializer),
													),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.JobTrainerNode).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.ContainerTrainer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.DatasetMountPath),
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.ModelMountPath),
														).
														WithResources(corev1ac.ResourceRequirements()),
												).
												WithVolumes(corev1ac.Volume().
													WithName(jobsetplgconsts.VolumeNameInitializer).
													WithPersistentVolumeClaim(corev1ac.PersistentVolumeClaimVolumeSource().
														WithClaimName(jobsetplgconsts.VolumeNameInitializer),
													),
												),
											),
										),
									),
								),
						),
				),
			},
			wantInfo: &Info{
				Labels: map[string]string{
					"labelKey": "labelValue",
				},
				Annotations: map[string]string{
					"annotationKey": "annotationValue",
				},
				Scheduler: &Scheduler{
					TotalRequests: map[string]TotalResourceRequest{
						constants.JobInitializer: {
							Replicas: 1,
							PodRequests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							},
						},
						constants.JobTrainerNode: {
							Replicas: 10,
							PodRequests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("40"),
							},
						},
					},
				},
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{
						{
							Name:               constants.JobInitializer,
							CountForNonTrainer: ptr.To[int32](1),
							Containers: []Container{{
								Name: constants.ContainerModelInitializer,
							}},
						},
						{
							Name: constants.JobTrainerNode,
							Containers: []Container{{
								Name: constants.ContainerTrainer,
								Env: []corev1ac.EnvVarApplyConfiguration{{
									Name:  ptr.To("TEST"),
									Value: ptr.To("TEST"),
								}},
								Ports: []corev1ac.ContainerPortApplyConfiguration{{
									Name:          ptr.To("http"),
									Protocol:      ptr.To(corev1.ProtocolTCP),
									ContainerPort: ptr.To[int32](8080),
								}},
								VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{{
									Name:      ptr.To("TEST"),
									ReadOnly:  ptr.To(true),
									MountPath: ptr.To("/var"),
								}},
							}},
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.JobInitializer).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.ContainerDatasetInitializer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.DatasetMountPath),
														).
														WithResources(corev1ac.ResourceRequirements()),
													corev1ac.Container().
														WithName(constants.ContainerModelInitializer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.ModelMountPath),
														).
														WithResources(corev1ac.ResourceRequirements()),
												).
												WithVolumes(corev1ac.Volume().
													WithName(jobsetplgconsts.VolumeNameInitializer).
													WithPersistentVolumeClaim(corev1ac.PersistentVolumeClaimVolumeSource().
														WithClaimName(jobsetplgconsts.VolumeNameInitializer),
													),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.JobTrainerNode).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.ContainerTrainer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.DatasetMountPath),
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.ModelMountPath),
														).
														WithResources(corev1ac.ResourceRequirements()),
												).
												WithVolumes(corev1ac.Volume().
													WithName(jobsetplgconsts.VolumeNameInitializer).
													WithPersistentVolumeClaim(corev1ac.PersistentVolumeClaimVolumeSource().
														WithClaimName(jobsetplgconsts.VolumeNameInitializer),
													),
												),
											),
										),
									),
								),
						),
				},
			},
		},
		"all arguments are not specified": {
			wantInfo: &Info{Scheduler: &Scheduler{TotalRequests: map[string]TotalResourceRequest{}}},
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
		cmpopts.EquateEmpty(),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			info := NewInfo(tc.infoOpts...)
			if diff := cmp.Diff(tc.wantInfo, info, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected runtime.Info (-want,+got):\n%s", diff)
			}
		})
	}
}
