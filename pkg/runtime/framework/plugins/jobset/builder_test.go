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

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	jobsetplgconsts "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jobset/constants"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/applyconfigurations/batch/v1"
	corev "k8s.io/client-go/applyconfigurations/core/v1"
	metav1 "k8s.io/client-go/applyconfigurations/meta/v1"

	"k8s.io/utils/ptr"

	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"
)

func TestInitializer(t *testing.T) {
	tests := []struct {
		name     string
		jobset   *jobsetv1alpha2ac.JobSetApplyConfiguration
		trainJob *trainer.TrainJob
	}{
		{
			name: "ancestor is dataset initializer",
			jobset: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &v1.JobTemplateSpecApplyConfiguration{
								Spec: &v1.JobSpecApplyConfiguration{
									Completions: ptr.To(int32(2)),
									Template: &corev.PodTemplateSpecApplyConfiguration{
										Spec: &corev.PodSpecApplyConfiguration{
											Containers: []corev.ContainerApplyConfiguration{
												{
													Env: []corev.EnvVarApplyConfiguration{
														{
															Name: ptr.To(jobsetplgconsts.InitializerEnvStorageUri),
														},
													},
													EnvFrom: []corev.EnvFromSourceApplyConfiguration{
														{
															SecretRef: &corev.SecretEnvSourceApplyConfiguration{
																Optional: ptr.To(false),
															},
														},
													},
													Name: ptr.To(constants.DatasetInitializer),
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1.ObjectMetaApplyConfiguration{
									GenerateName: ptr.To(""),
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.DatasetInitializer,
									},
								},
							},
							Name:      ptr.To("test_replica_name_1"),
							GroupName: ptr.To("test_group_name_1"),
							Replicas:  ptr.To(int32(2)),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Labels: map[string]string{
						"hello_world":   "1234",
						"hello_world_1": "3456",
					},
					RuntimeRef: trainer.RuntimeRef{
						Name:     "",
						APIGroup: ptr.To(""),
						Kind:     ptr.To(""),
					},
					Initializer: &trainer.Initializer{
						Model: &trainer.ModelInitializer{
							StorageUri: ptr.To("uri_example"),
						},
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To("uri_example"),
							SecretRef: &corev1.LocalObjectReference{
								Name: "",
							},
						},
					},
					Trainer: &trainer.Trainer{
						Image: ptr.To(""),
					},
				},
			},
		},
		{
			name: "ancestor is model initializer",
			jobset: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &v1.JobTemplateSpecApplyConfiguration{
								Spec: &v1.JobSpecApplyConfiguration{
									Completions: ptr.To(int32(2)),
									Template: &corev.PodTemplateSpecApplyConfiguration{
										Spec: &corev.PodSpecApplyConfiguration{
											Containers: []corev.ContainerApplyConfiguration{
												{
													Env: []corev.EnvVarApplyConfiguration{
														{
															Name: ptr.To("example_1"),
														},
													},
													EnvFrom: []corev.EnvFromSourceApplyConfiguration{
														{
															SecretRef: &corev.SecretEnvSourceApplyConfiguration{
																Optional: ptr.To(false),
															},
														},
													},
													Name: ptr.To(constants.ModelInitializer),
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1.ObjectMetaApplyConfiguration{
									GenerateName: ptr.To(""),
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.ModelInitializer,
									},
								},
							},
							Name:      ptr.To("test_replica_name_1"),
							GroupName: ptr.To("test_group_name_1"),
							Replicas:  ptr.To(int32(2)),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Labels: map[string]string{
						"hello_world":   "1234",
						"hello_world_1": "3456",
					},
					RuntimeRef: trainer.RuntimeRef{
						Name:     "",
						APIGroup: ptr.To(""),
						Kind:     ptr.To(""),
					},
					Initializer: &trainer.Initializer{
						Model: &trainer.ModelInitializer{
							StorageUri: ptr.To("example_1"),
							SecretRef: &corev1.LocalObjectReference{
								Name: "",
							},
						},
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To("example_1"),
							SecretRef: &corev1.LocalObjectReference{
								Name: "",
							},
						},
					},
					Trainer: &trainer.Trainer{
						Image: ptr.To(""),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := NewBuilder(test.jobset)
			builder.Initializer(test.trainJob)
			rj := builder.Spec.ReplicatedJobs[0]

			if *rj.Replicas != 1 {
				t.Fatalf("expected replicas=1, got %d", *rj.Replicas)
			}

			containers := rj.Template.Spec.Template.Spec.Containers
			env := containers[0].Env

			found := false
			for _, e := range env {
				if *e.Name == jobsetplgconsts.InitializerEnvStorageUri {
					found = true
				}
			}

			if !found {
				t.Fatalf("storage uri env not injected")
			}

			envFrom := containers[0].EnvFrom
			if len(envFrom) == 0 {
				t.Fatalf("expected secret envFrom injected")
			}

			builder.isRunLauncherAsNode(&runtime.Info{
				Labels: map[string]string{
					"hello_world":   "1234",
					"hello_world_1": "3456",
				},
			})
		})

	}
}

func TestTrainer(t *testing.T) {
	tests := []struct {
		name     string
		jobset   *jobsetv1alpha2ac.JobSetApplyConfiguration
		trainJob *trainer.TrainJob
	}{
		{
			name: "job metadata constant is Ancestor Trainer",
			jobset: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &v1.JobTemplateSpecApplyConfiguration{
								Spec: &v1.JobSpecApplyConfiguration{
									Completions: ptr.To(int32(2)),
									Template: &corev.PodTemplateSpecApplyConfiguration{
										Spec: &corev.PodSpecApplyConfiguration{
											Containers: []corev.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.DatasetInitializer),
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1.ObjectMetaApplyConfiguration{
									GenerateName: ptr.To(""),
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.DatasetInitializer,
									},
								},
							},
							Name:      ptr.To("test_replica_name_1"),
							GroupName: ptr.To("test_group_name_1"),
							Replicas:  ptr.To(int32(2)),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Labels: map[string]string{
						"hello_world":   "1234",
						"hello_world_1": "3456",
					},
					RuntimeRef: trainer.RuntimeRef{
						Name:     "runtime_example",
						APIGroup: ptr.To("example_api_group"),
						Kind:     ptr.To("pods"),
					},
					Initializer: &trainer.Initializer{
						Model: &trainer.ModelInitializer{
							StorageUri: ptr.To("model_storage_uri"),
						},
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To("dataset_storage_uri"),
							SecretRef: &corev1.LocalObjectReference{
								Name: "",
							},
						},
					},
					Trainer: &trainer.Trainer{
						Image: ptr.To(""),
					},
				},
			},
		},
		{
			name: "job meta data constant is ancestor trainer or job name is node",
			jobset: &jobsetv1alpha2ac.JobSetApplyConfiguration{
				Spec: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
					ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
						{
							Template: &v1.JobTemplateSpecApplyConfiguration{
								Spec: &v1.JobSpecApplyConfiguration{
									Completions: ptr.To(int32(2)),
									Template: &corev.PodTemplateSpecApplyConfiguration{
										Spec: &corev.PodSpecApplyConfiguration{
											Containers: []corev.ContainerApplyConfiguration{
												{
													Name: ptr.To(constants.Node),
												},
											},
										},
									},
								},
								ObjectMetaApplyConfiguration: &metav1.ObjectMetaApplyConfiguration{
									GenerateName: ptr.To(""),
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									},
								},
							},
							Name:      ptr.To("test_replica_name_1"),
							GroupName: ptr.To("test_group_name_1"),
							Replicas:  ptr.To(int32(2)),
						},
					},
				},
			},
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					Labels: map[string]string{
						"hello_world":   "1234",
						"hello_world_1": "3456",
					},
					RuntimeRef: trainer.RuntimeRef{
						Name:     "",
						APIGroup: ptr.To(""),
						Kind:     ptr.To(""),
					},
					Initializer: &trainer.Initializer{
						Model: &trainer.ModelInitializer{
							StorageUri: ptr.To(""),
							SecretRef: &corev1.LocalObjectReference{
								Name: "",
							},
						},
						Dataset: &trainer.DatasetInitializer{
							StorageUri: ptr.To(""),
							SecretRef: &corev1.LocalObjectReference{
								Name: "",
							},
						},
					},
					Trainer: &trainer.Trainer{
						Image:   ptr.To(""),
						Command: []string{"cmd1", "cmd2", "cmd3"},
						Args:    []string{"args1", "args2", "arg3"},
						ResourcesPerNode: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			builder := NewBuilder(test.jobset)

			builder.Trainer(&runtime.Info{}, test.trainJob)
			builder.Build()

			c := builder.Spec.ReplicatedJobs[0].
				Template.Spec.Template.Spec.Containers[0]

			ancestor := test.jobset.Spec.ReplicatedJobs[0].
				Template.ObjectMetaApplyConfiguration.Labels[constants.LabelTrainJobAncestor]

			if ancestor == constants.AncestorTrainer {
				if c.Image == nil {
					t.Fatalf("trainer image not injected")
				}

				if *c.Image != *test.trainJob.Spec.Trainer.Image {
					t.Fatalf("image mismatch")
				}

				if len(c.Command) != len(test.trainJob.Spec.Trainer.Command) {
					t.Fatalf("command not applied")
				}
			} else {
				if c.Image != nil {
					t.Fatalf("image should not be set for non-trainer job")
				}
			}
		})
	}
}
