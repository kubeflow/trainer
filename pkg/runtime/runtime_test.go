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
	"k8s.io/utils/ptr"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobsetplgconsts "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jobset/constants"
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
				WithPodSet(constants.DatasetInitializer, ptr.To(constants.DatasetInitializer), 1, corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: constants.DatasetInitializer,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("10"),
							},
						},
					}},
					InitContainers: []corev1.Container{{
						Name:          "setup-initializer",
						RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("5"),
							},
						},
					}},
				}, corev1ac.PodSpec().
					WithContainers(
						corev1ac.Container().
							WithName(constants.DatasetInitializer).
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("10"),
								})),
					).
					WithInitContainers(
						corev1ac.Container().
							WithName("setup-initializer").
							WithRestartPolicy(corev1.ContainerRestartPolicyAlways).
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("5"),
								})),
					),
				),
				WithPodSet(constants.ModelInitializer, ptr.To(constants.ModelInitializer), 1, corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: constants.ModelInitializer,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("10"),
							},
						},
					}},
					InitContainers: []corev1.Container{{
						Name:          "setup-initializer",
						RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("5"),
							},
						},
					}},
				}, corev1ac.PodSpec().
					WithContainers(
						corev1ac.Container().
							WithName(constants.ModelInitializer).
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("10"),
								})),
					).
					WithInitContainers(
						corev1ac.Container().
							WithName("setup-initializer").
							WithRestartPolicy(corev1.ContainerRestartPolicyAlways).
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("5"),
								})),
					),
				),
				WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 10, corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: constants.Node,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							},
						},
					}},
					InitContainers: []corev1.Container{{
						Name:          "preparation",
						RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("25"),
							},
						},
					}},
				}, corev1ac.PodSpec().
					WithContainers(
						corev1ac.Container().
							WithName(constants.Node).
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
				WithTemplateSpecObjApply(
					jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.DatasetInitializer).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.DatasetInitializer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.DatasetMountPath),
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
								WithName(constants.ModelInitializer).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.ModelInitializer).
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
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
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
				Scheduler: &Scheduler{PodLabels: make(map[string]string)},
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{
						{
							Name:     constants.DatasetInitializer,
							Ancestor: ptr.To(constants.DatasetInitializer),
							Count:    ptr.To[int32](1),
							InitContainers: []Container{{
								Name: "setup-initializer",
							}},
							Containers: []Container{{
								Name: constants.DatasetInitializer,
							}},
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							},
						},
						{
							Name:     constants.ModelInitializer,
							Ancestor: ptr.To(constants.ModelInitializer),
							Count:    ptr.To[int32](1),
							InitContainers: []Container{{
								Name: "setup-initializer",
							}},
							Containers: []Container{{
								Name: constants.ModelInitializer,
							}},
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							},
						},
						{
							Name:     constants.Node,
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](10),
							Containers: []Container{{
								Name: constants.Node,
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
							InitContainers: []Container{{
								Name: "preparation",
							}},
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("40"),
							},
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.DatasetInitializer).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.DatasetInitializer).
														WithVolumeMounts(
															corev1ac.VolumeMount().
																WithName(jobsetplgconsts.VolumeNameInitializer).
																WithMountPath(constants.DatasetMountPath),
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
								WithName(constants.ModelInitializer).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.ModelInitializer).
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
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(nil).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithLabels(nil).
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
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
			wantInfo: &Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Scheduler:   &Scheduler{PodLabels: make(map[string]string)},
			},
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
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

func TestFindContainerByPodSetAncestorContainerName(t *testing.T) {
	cases := map[string]struct {
		info          *Info
		psAncestor    string
		containerName string
		wantContainer *Container
	}{
		"podSet and container exist": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{
						{
							Ancestor: ptr.To("alpha"),
							Containers: []Container{
								{
									Name: "one",
								},
								{
									Name: "two",
								},
							},
						},
						{
							Ancestor:   ptr.To("beta"),
							Containers: []Container{{Name: "one"}},
						},
					},
				},
			},
			psAncestor:    "alpha",
			containerName: "one",
			wantContainer: &Container{
				Name: "one",
			},
		},
		"podSet exists, but container does not exist": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{{
						Ancestor:   ptr.To("alpha"),
						Containers: []Container{{Name: "one"}},
					}},
				},
			},
			psAncestor:    "alpha",
			containerName: "two",
			wantContainer: nil,
		},
		"podSet does not exist": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{{
						Ancestor:   ptr.To("alpha"),
						Containers: []Container{{Name: "one"}},
					}},
				},
			},
			psAncestor:    "beta",
			containerName: "one",
			wantContainer: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.info.FindContainerByPodSetAncestorContainerName(tc.psAncestor, tc.containerName)
			if diff := cmp.Diff(tc.wantContainer, got); len(diff) != 0 {
				t.Errorf("Unexpected Container (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestFindPodSetByName(t *testing.T) {
	cases := map[string]struct {
		info       *Info
		psName     string
		wantPodSet *PodSet
	}{
		"PodSet exists": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{
						{
							Name: "alpha",
						},
						{
							Name: "beta",
						},
					},
				},
			},
			psName: "alpha",
			wantPodSet: &PodSet{
				Name: "alpha",
			},
		},
		"PodSet does not exist": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{
						{
							Name: "alpha",
						},
					},
				},
			},
			psName:     "beta",
			wantPodSet: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.info.FindPodSetByName(tc.psName)
			if diff := cmp.Diff(tc.wantPodSet, got); len(diff) != 0 {
				t.Errorf("Unexpected PodSet (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestFindContainerByName(t *testing.T) {
	cases := map[string]struct {
		info       *Info
		psAncestor string
		want       *PodSet
	}{
		"PodSet exists": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{
						{
							Ancestor: ptr.To("alpha"),
						},
						{
							Ancestor: ptr.To("beta"),
						},
					},
				},
			},
			psAncestor: "alpha",
			want: &PodSet{
				Ancestor: ptr.To("alpha"),
			},
		},
		"PodSet does not exist": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					PodSets: []PodSet{{
						Name: "alpha",
					}},
				},
			},
			psAncestor: "beta",
			want:       nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.info.FindPodSetByAncestor(tc.psAncestor)
			if diff := cmp.Diff(tc.want, got); len(diff) != 0 {
				t.Errorf("Unexpected PodSet (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestSyncPodSetsToTemplateSpec(t *testing.T) {
	cases := map[string]struct {
		info            *Info
		wantParallelism map[int]*int32
		wantCompletions map[int]*int32
	}{
		"sync PodSets.Count to JobSet Parallelism and Completions": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("node").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("worker").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
						),
					PodSets: []PodSet{
						{Name: "node", Count: ptr.To[int32](5)},
						{Name: "worker", Count: ptr.To[int32](10)},
					},
				},
			},
			wantParallelism: map[int]*int32{0: ptr.To[int32](5), 1: ptr.To[int32](10)},
			wantCompletions: map[int]*int32{0: ptr.To[int32](5), 1: ptr.To[int32](10)},
		},
		"sync only PodSets with non-nil Count": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("node").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("worker").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(2).
										WithCompletions(2))),
						),
					PodSets: []PodSet{
						{Name: "node", Count: ptr.To[int32](5)},
						{Name: "worker", Count: nil}, // nil Count should not update
					},
				},
			},
			wantParallelism: map[int]*int32{0: ptr.To[int32](5), 1: ptr.To[int32](2)},
			wantCompletions: map[int]*int32{0: ptr.To[int32](5), 1: ptr.To[int32](2)},
		},
		"nil ObjApply does not panic": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					ObjApply: nil,
					PodSets: []PodSet{
						{Name: "node", Count: ptr.To[int32](5)},
					},
				},
			},
			wantParallelism: nil,
			wantCompletions: nil,
		},
		"more PodSets than ReplicatedJobs does not panic": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("node").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
						),
					PodSets: []PodSet{
						{Name: "node", Count: ptr.To[int32](5)},
						{Name: "worker", Count: ptr.To[int32](10)}, // extra PodSet beyond ReplicatedJobs length
						{Name: "extra", Count: ptr.To[int32](20)},  // another extra PodSet
					},
				},
			},
			// Only the first PodSet should be synced, extras are safely skipped
			wantParallelism: map[int]*int32{0: ptr.To[int32](5)},
			wantCompletions: map[int]*int32{0: ptr.To[int32](5)},
		},
		"fewer PodSets than ReplicatedJobs syncs available PodSets": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("node").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("worker").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(2).
										WithCompletions(2))),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("extra").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(3).
										WithCompletions(3))),
						),
					PodSets: []PodSet{
						{Name: "node", Count: ptr.To[int32](5)},
						// Only one PodSet, but three ReplicatedJobs
					},
				},
			},
			// Only matching ReplicatedJob is updated, others keep original values
			wantParallelism: map[int]*int32{0: ptr.To[int32](5), 1: ptr.To[int32](2), 2: ptr.To[int32](3)},
			wantCompletions: map[int]*int32{0: ptr.To[int32](5), 1: ptr.To[int32](2), 2: ptr.To[int32](3)},
		},
		"PodSets and ReplicatedJobs in different order matches by name": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("worker").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("node").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
						),
					PodSets: []PodSet{
						{Name: "node", Count: ptr.To[int32](5)}, // Different order than ReplicatedJobs
						{Name: "worker", Count: ptr.To[int32](10)},
					},
				},
			},
			// Matching by name: worker is at index 0, node is at index 1
			wantParallelism: map[int]*int32{0: ptr.To[int32](10), 1: ptr.To[int32](5)},
			wantCompletions: map[int]*int32{0: ptr.To[int32](10), 1: ptr.To[int32](5)},
		},
		"PodSet with no matching ReplicatedJob name is skipped": {
			info: &Info{
				TemplateSpec: TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName("node").
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1))),
						),
					PodSets: []PodSet{
						{Name: "nonexistent", Count: ptr.To[int32](5)}, // No matching ReplicatedJob
					},
				},
			},
			// No updates because name doesn't match
			wantParallelism: map[int]*int32{0: ptr.To[int32](1)},
			wantCompletions: map[int]*int32{0: ptr.To[int32](1)},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.info.SyncPodSetsToTemplateSpec()

			if tc.wantParallelism == nil {
				return // nothing to check for nil ObjApply case
			}

			jobSetSpec, ok := TemplateSpecApply[jobsetv1alpha2ac.JobSetSpecApplyConfiguration](tc.info)
			if !ok {
				t.Fatalf("Failed to get JobSetSpecApplyConfiguration")
			}

			for idx, wantP := range tc.wantParallelism {
				gotP := jobSetSpec.ReplicatedJobs[idx].Template.Spec.Parallelism
				if diff := cmp.Diff(wantP, gotP); len(diff) != 0 {
					t.Errorf("Unexpected Parallelism for ReplicatedJob[%d] (-want,+got):\n%s", idx, diff)
				}
			}
			for idx, wantC := range tc.wantCompletions {
				gotC := jobSetSpec.ReplicatedJobs[idx].Template.Spec.Completions
				if diff := cmp.Diff(wantC, gotC); len(diff) != 0 {
					t.Errorf("Unexpected Completions for ReplicatedJob[%d] (-want,+got):\n%s", idx, diff)
				}
			}
		})
	}
}
