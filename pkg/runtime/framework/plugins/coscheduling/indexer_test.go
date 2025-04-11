/*
Copyright 2025 The Kubeflow Authors.

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

package coscheduling

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
)

func TestIndexTrainingRuntimeContainerRuntimeClass(t *testing.T) {
	cases := map[string]struct {
		obj        client.Object
		wantResult []string
	}{
		"error type": {
			obj: &trainer.ClusterTrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{},
						},
					},
				},
			},
			wantResult: nil,
		},
		"empty ReplicatedJobs": {
			obj: &trainer.TrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{},
						},
					},
				},
			},
			wantResult: nil,
		},
		"with RuntimeClassName": {
			obj: &trainer.TrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: v1.PodTemplateSpec{
												Spec: v1.PodSpec{
													RuntimeClassName: ptr.To[string]("test-runtime-class"),
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
			wantResult: []string{"test-runtime-class"},
		},
		"multiple RuntimeClassNames": {
			obj: &trainer.TrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: v1.PodTemplateSpec{
												Spec: v1.PodSpec{
													RuntimeClassName: ptr.To[string]("test-runtime-class-1"),
												},
											},
										},
									},
								},
								{
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: v1.PodTemplateSpec{
												Spec: v1.PodSpec{
													RuntimeClassName: ptr.To[string]("test-runtime-class-2"),
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
			wantResult: []string{"test-runtime-class-1", "test-runtime-class-2"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := IndexTrainingRuntimeContainerRuntimeClass(tc.obj)
			if diff := cmp.Diff(tc.wantResult, result); len(diff) != 0 {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestIndexClusterTrainingRuntimeContainerRuntimeClass(t *testing.T) {
	cases := map[string]struct {
		obj        client.Object
		wantResult []string
	}{
		"error type": {
			obj: &trainer.TrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{},
						},
					},
				},
			},
			wantResult: nil,
		},
		"empty ReplicatedJobs": {
			obj: &trainer.ClusterTrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{},
						},
					},
				},
			},
			wantResult: nil,
		},
		"with RuntimeClassName": {
			obj: &trainer.ClusterTrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: v1.PodTemplateSpec{
												Spec: v1.PodSpec{
													RuntimeClassName: ptr.To[string]("cluster-test-runtime-class"),
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
			wantResult: []string{"cluster-test-runtime-class"},
		},
		"multiple RuntimeClassNames": {
			obj: &trainer.ClusterTrainingRuntime{
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: v1.PodTemplateSpec{
												Spec: v1.PodSpec{
													RuntimeClassName: ptr.To[string]("cluster-test-runtime-class-1"),
												},
											},
										},
									},
								},
								{
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: v1.PodTemplateSpec{
												Spec: v1.PodSpec{
													RuntimeClassName: ptr.To[string]("cluster-test-runtime-class-2"),
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
			wantResult: []string{"cluster-test-runtime-class-1", "cluster-test-runtime-class-2"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := IndexClusterTrainingRuntimeContainerRuntimeClass(tc.obj)
			if diff := cmp.Diff(tc.wantResult, result); len(diff) != 0 {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}
