/*
Copyright 2026 The Kubeflow Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	jobsetplgconsts "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jobset/constants"
)

func makeJobSetWithInitializer(ancestor string, containerName string, replicas int32) *jobsetv1alpha2ac.JobSetApplyConfiguration {
	return &jobsetv1alpha2ac.JobSetApplyConfiguration{
		Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
			ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
				{
					Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
						Spec: &batchv1ac.JobSpecApplyConfiguration{
							Template: &corev1ac.PodTemplateSpecApplyConfiguration{
								Spec: &corev1ac.PodSpecApplyConfiguration{
									Containers: []corev1ac.ContainerApplyConfiguration{
										*corev1ac.Container().WithName(containerName),
									},
								},
							},
						},
						ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
							Labels: map[string]string{
								constants.LabelTrainJobAncestor: ancestor,
							},
						},
					},
					Name:     ptr.To("initializer-job"),
					Replicas: ptr.To(replicas),
				},
			},
		},
	}
}

func makeJobSetWithTrainer(ancestor string, containerName string, replicas int32) *jobsetv1alpha2ac.JobSetApplyConfiguration {
	return &jobsetv1alpha2ac.JobSetApplyConfiguration{
		Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
			ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
				{
					Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
						Spec: &batchv1ac.JobSpecApplyConfiguration{
							Template: &corev1ac.PodTemplateSpecApplyConfiguration{
								Spec: &corev1ac.PodSpecApplyConfiguration{
									Containers: []corev1ac.ContainerApplyConfiguration{
										*corev1ac.Container().WithName(containerName),
									},
								},
							},
						},
						ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
							Labels: map[string]string{
								constants.LabelTrainJobAncestor: ancestor,
							},
						},
					},
					Name:     ptr.To(constants.Node),
					Replicas: ptr.To(replicas),
				},
			},
		},
	}
}

func TestBuilderInitializer(t *testing.T) {
	cases := map[string]struct {
		jobSet     *jobsetv1alpha2ac.JobSetApplyConfiguration
		trainJob   *trainer.TrainJob
		wantJobSet *jobsetv1alpha2ac.JobSetApplyConfiguration
	}{
		"dataset initializer with storageUri and secretRef": {
			jobSet: makeJobSetWithInitializer(constants.DatasetInitializer, constants.DatasetInitializer, 3),
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Initializer: &trainer.Initializer{
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To("hf://my-org/my-dataset"),
							SecretRef: &corev1.LocalObjectReference{
								Name: "hf-token-secret",
							},
						},
					},
				},
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.DatasetInitializer),
													Env: []corev1ac.EnvVarApplyConfiguration{
														{
															Name:  ptr.To(jobsetplgconsts.InitializerEnvStorageUri),
															Value: ptr.To("hf://my-org/my-dataset"),
														},
													},
													EnvFrom: []corev1ac.EnvFromSourceApplyConfiguration{
														{
															SecretRef: &corev1ac.SecretEnvSourceApplyConfiguration{
																LocalObjectReferenceApplyConfiguration: corev1ac.LocalObjectReferenceApplyConfiguration{
																	Name: ptr.To("hf-token-secret"),
																},
															},
														},
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.DatasetInitializer,
									},
								},
							},
							Name:     ptr.To("initializer-job"),
							Replicas: ptr.To[int32](1),
						},
					},
				},
			},
		},
		"model initializer with storageUri and secretRef": {
			jobSet: makeJobSetWithInitializer(constants.ModelInitializer, constants.ModelInitializer, 2),
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Initializer: &trainer.Initializer{
						Model: &trainer.ModelInitializer{
							StorageUri: ptr.To("hf://meta-llama/Llama-3"),
							SecretRef: &corev1.LocalObjectReference{
								Name: "model-secret",
							},
						},
					},
				},
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.ModelInitializer),
													Env: []corev1ac.EnvVarApplyConfiguration{
														{
															Name:  ptr.To(jobsetplgconsts.InitializerEnvStorageUri),
															Value: ptr.To("hf://meta-llama/Llama-3"),
														},
													},
													EnvFrom: []corev1ac.EnvFromSourceApplyConfiguration{
														{
															SecretRef: &corev1ac.SecretEnvSourceApplyConfiguration{
																LocalObjectReferenceApplyConfiguration: corev1ac.LocalObjectReferenceApplyConfiguration{
																	Name: ptr.To("model-secret"),
																},
															},
														},
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.ModelInitializer,
									},
								},
							},
							Name:     ptr.To("initializer-job"),
							Replicas: ptr.To[int32](1),
						},
					},
				},
			},
		},
		"dataset initializer merges Dataset.Env with storageUri and upserts pre-existing container env": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.DatasetInitializer),
													Env: []corev1ac.EnvVarApplyConfiguration{
														*corev1ac.EnvVar().WithName("PRE_EXISTING").WithValue("from-runtime-template"),
														*corev1ac.EnvVar().WithName("OTHER").WithValue("keep-me"),
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.DatasetInitializer,
									},
								},
							},
							Name:     ptr.To("initializer-job"),
							Replicas: ptr.To[int32](3),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Initializer: &trainer.Initializer{
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To("hf://my-org/my-dataset"),
							Env: []corev1.EnvVar{
								{Name: "CUSTOM_FROM_USER", Value: "user-value"},
								{Name: "PRE_EXISTING", Value: "overridden-by-trainjob"},
							},
						},
					},
				},
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.DatasetInitializer),
													Env: []corev1ac.EnvVarApplyConfiguration{
														{
															Name:  ptr.To("PRE_EXISTING"),
															Value: ptr.To("overridden-by-trainjob"),
														},
														{
															Name:  ptr.To("OTHER"),
															Value: ptr.To("keep-me"),
														},
														{
															Name:  ptr.To(jobsetplgconsts.InitializerEnvStorageUri),
															Value: ptr.To("hf://my-org/my-dataset"),
														},
														{
															Name:  ptr.To("CUSTOM_FROM_USER"),
															Value: ptr.To("user-value"),
														},
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.DatasetInitializer,
									},
								},
							},
							Name:     ptr.To("initializer-job"),
							Replicas: ptr.To[int32](1),
						},
					},
				},
			},
		},
		"dataset initializer without secretRef only injects storageUri": {
			jobSet: makeJobSetWithInitializer(constants.DatasetInitializer, constants.DatasetInitializer, 2),
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Initializer: &trainer.Initializer{
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To("s3://bucket/dataset"),
						},
					},
				},
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.DatasetInitializer),
													Env: []corev1ac.EnvVarApplyConfiguration{
														{
															Name:  ptr.To(jobsetplgconsts.InitializerEnvStorageUri),
															Value: ptr.To("s3://bucket/dataset"),
														},
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.DatasetInitializer,
									},
								},
							},
							Name:     ptr.To("initializer-job"),
							Replicas: ptr.To[int32](1),
						},
					},
				},
			},
		},
		"no ancestor label skips modification": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												*corev1ac.Container().WithName("some-container"),
											},
										},
									},
								},
							},
							Name:     ptr.To("worker"),
							Replicas: ptr.To[int32](4),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Initializer: &trainer.Initializer{
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To("hf://dataset"),
						},
					},
				},
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To("some-container"),
												},
											},
										},
									},
								},
							},
							Name:     ptr.To("worker"),
							Replicas: ptr.To[int32](4),
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := NewBuilder(tc.jobSet)
			got := builder.Initializer(tc.trainJob).Build()
			if diff := cmp.Diff(tc.wantJobSet, got); len(diff) != 0 {
				t.Errorf("Unexpected JobSet from Initializer (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestBuilderTrainer(t *testing.T) {
	cases := map[string]struct {
		jobSet     *jobsetv1alpha2ac.JobSetApplyConfiguration
		trainJob   *trainer.TrainJob
		info       *runtime.Info
		wantJobSet *jobsetv1alpha2ac.JobSetApplyConfiguration
	}{
		"trainer ancestor with image command and args": {
			jobSet: makeJobSetWithTrainer(constants.AncestorTrainer, constants.Node, 4),
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						Image:   ptr.To("docker.io/my-org/train:latest"),
						Command: []string{"torchrun", "--nproc_per_node=4"},
						Args:    []string{"train.py", "--epochs=10"},
					},
				},
			},
			info: &runtime.Info{},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name:    ptr.To(constants.Node),
													Image:   ptr.To("docker.io/my-org/train:latest"),
													Command: []string{"torchrun", "--nproc_per_node=4"},
													Args:    []string{"train.py", "--epochs=10"},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									},
								},
							},
							Name:     ptr.To(constants.Node),
							Replicas: ptr.To[int32](1),
						},
					},
				},
			},
		},
		"trainer ancestor merges Trainer.Env and upserts duplicate container env keys": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.Node),
													Env: []corev1ac.EnvVarApplyConfiguration{
														*corev1ac.EnvVar().WithName("FROM_TEMPLATE").WithValue("template-value"),
														*corev1ac.EnvVar().WithName("OVERRIDE_ME").WithValue("old-value"),
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									},
								},
							},
							Name:     ptr.To(constants.Node),
							Replicas: ptr.To[int32](4),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						Env: []corev1.EnvVar{
							{Name: "OVERRIDE_ME", Value: "new-value"},
							{Name: "FROM_TRAINJOB", Value: "extra"},
						},
					},
				},
			},
			info: &runtime.Info{},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.Node),
													Env: []corev1ac.EnvVarApplyConfiguration{
														{
															Name:  ptr.To("FROM_TEMPLATE"),
															Value: ptr.To("template-value"),
														},
														{
															Name:  ptr.To("OVERRIDE_ME"),
															Value: ptr.To("new-value"),
														},
														{
															Name:  ptr.To("FROM_TRAINJOB"),
															Value: ptr.To("extra"),
														},
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									},
								},
							},
							Name:     ptr.To(constants.Node),
							Replicas: ptr.To[int32](1),
						},
					},
				},
			},
		},
		"trainer ancestor with resourcesPerNode": {
			jobSet: makeJobSetWithTrainer(constants.AncestorTrainer, constants.Node, 2),
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						ResourcesPerNode: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"nvidia.com/gpu":      resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
						},
					},
				},
			},
			info: &runtime.Info{},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.Node),
													Resources: &corev1ac.ResourceRequirementsApplyConfiguration{
														Limits: &corev1.ResourceList{
															"nvidia.com/gpu":      resource.MustParse("2"),
															corev1.ResourceMemory: resource.MustParse("16Gi"),
														},
														Requests: &corev1.ResourceList{
															corev1.ResourceCPU:    resource.MustParse("4"),
															corev1.ResourceMemory: resource.MustParse("8Gi"),
														},
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									},
								},
							},
							Name:     ptr.To(constants.Node),
							Replicas: ptr.To[int32](1),
						},
					},
				},
			},
		},
		"non-trainer ancestor is not modified": {
			jobSet: makeJobSetWithTrainer(constants.DatasetInitializer, constants.Node, 2),
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						Image:   ptr.To("should-not-be-applied"),
						Command: []string{"should-not-be-applied"},
					},
				},
			},
			info:       &runtime.Info{},
			wantJobSet: makeJobSetWithTrainer(constants.DatasetInitializer, constants.Node, 2),
		},
		"MPI runLauncherAsNode applies resources to node job": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												*corev1ac.Container().WithName(constants.Node),
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{},
								},
							},
							Name:     ptr.To(constants.Node),
							Replicas: ptr.To[int32](2),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						ResourcesPerNode: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("32Gi"),
							},
						},
					},
				},
			},
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: &trainer.MLPolicySource{
						MPI: &trainer.MPIMLPolicySource{
							RunLauncherAsNode: ptr.To(true),
						},
					},
				},
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										Spec: &corev1ac.PodSpecApplyConfiguration{
											Containers: []corev1ac.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.Node),
													Resources: &corev1ac.ResourceRequirementsApplyConfiguration{
														Limits: &corev1.ResourceList{
															corev1.ResourceMemory: resource.MustParse("32Gi"),
														},
													},
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
									Labels: map[string]string{},
								},
							},
							Name:     ptr.To(constants.Node),
							Replicas: ptr.To[int32](2),
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := NewBuilder(tc.jobSet)
			got := builder.Trainer(tc.info, tc.trainJob).Build()
			if diff := cmp.Diff(tc.wantJobSet, got); len(diff) != 0 {
				t.Errorf("Unexpected JobSet from Trainer (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestBuilderPodLabels(t *testing.T) {
	cases := map[string]struct {
		jobSet     *jobsetv1alpha2ac.JobSetApplyConfiguration
		labels     map[string]string
		wantJobSet *jobsetv1alpha2ac.JobSetApplyConfiguration
	}{
		"labels applied to all replicated jobs": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{},
								},
							},
							Name: ptr.To("worker"),
						},
					},
				},
			},
			labels: map[string]string{
				"app.kubernetes.io/name": "my-training",
				"team":                   "ml-platform",
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
											Labels: map[string]string{
												"app.kubernetes.io/name": "my-training",
												"team":                   "ml-platform",
											},
										},
									},
								},
							},
							Name: ptr.To("worker"),
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := NewBuilder(tc.jobSet)
			got := builder.PodLabels(tc.labels).Build()
			if diff := cmp.Diff(tc.wantJobSet, got); len(diff) != 0 {
				t.Errorf("Unexpected JobSet from PodLabels (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestBuilderPodAnnotations(t *testing.T) {
	cases := map[string]struct {
		jobSet      *jobsetv1alpha2ac.JobSetApplyConfiguration
		annotations map[string]string
		wantJobSet  *jobsetv1alpha2ac.JobSetApplyConfiguration
	}{
		"annotations applied to all replicated jobs": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{},
								},
							},
							Name: ptr.To("worker"),
						},
					},
				},
			},
			annotations: map[string]string{
				"prometheus.io/scrape": "true",
			},
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
								Spec: &batchv1ac.JobSpecApplyConfiguration{
									Template: &corev1ac.PodTemplateSpecApplyConfiguration{
										ObjectMetaApplyConfiguration: &metav1ac.ObjectMetaApplyConfiguration{
											Annotations: map[string]string{
												"prometheus.io/scrape": "true",
											},
										},
									},
								},
							},
							Name: ptr.To("worker"),
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := NewBuilder(tc.jobSet)
			got := builder.PodAnnotations(tc.annotations).Build()
			if diff := cmp.Diff(tc.wantJobSet, got); len(diff) != 0 {
				t.Errorf("Unexpected JobSet from PodAnnotations (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestBuilderSuspend(t *testing.T) {
	cases := map[string]struct {
		jobSet     *jobsetv1alpha2ac.JobSetApplyConfiguration
		suspend    *bool
		wantJobSet *jobsetv1alpha2ac.JobSetApplyConfiguration
	}{
		"suspend set to true": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{},
			},
			suspend: ptr.To(true),
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					Suspend: ptr.To(true),
				},
			},
		},
		"suspend set to false": {
			jobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{},
			},
			suspend: ptr.To(false),
			wantJobSet: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					Suspend: ptr.To(false),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := NewBuilder(tc.jobSet)
			got := builder.Suspend(tc.suspend).Build()
			if diff := cmp.Diff(tc.wantJobSet, got); len(diff) != 0 {
				t.Errorf("Unexpected JobSet from Suspend (-want,+got):\n%s", diff)
			}
		})
	}
}
