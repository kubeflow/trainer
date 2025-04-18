package coscheduling

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

func TestIndexTrainingRuntimeContainerRuntimeClass(t *testing.T) {
	cases := map[string]struct {
		obj  client.Object
		want []string
	}{

		"object is not a TrainingRuntime": {
			obj:  utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test"),
			want: nil,
		},
		"TrainingRuntime with no ReplicatedJobs": {
			obj: &trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test",
				},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{},
						},
					},
				},
			},
			want: []string{},
		},
		"TrainingRuntime with multiple ReplicatedJobs where all RuntimeClassName are nil": {
			obj: &trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test",
				},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.DatasetInitializer,
									Template: batchv1.JobTemplateSpec{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{
												constants.LabelTrainJobAncestor: constants.DatasetInitializer,
											},
										},
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: nil,
												},
											},
										},
									},
								},
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{
												constants.LabelTrainJobAncestor: constants.ModelInitializer,
											},
										},
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: nil,
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
			want: []string{},
		},
		"TrainingRuntime with ReplicatedJobs where all RuntimeClassName are set": {
			obj: &trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "test"},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.DatasetInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.DatasetInitializer),
												},
											},
										},
									},
								},
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.ModelInitializer),
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
			want: []string{constants.DatasetInitializer, constants.ModelInitializer},
		},
		"TrainingRuntime with one ReplicatedJob and RuntimeClassName set": {
			obj: &trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "test"},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.ModelInitializer),
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
			want: []string{constants.ModelInitializer},
		},
		"TrainingRuntime with ReplicatedJobs where some RuntimeClassName are set and others are nil": {
			obj: &trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "test"},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.DatasetInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.DatasetInitializer),
												},
											},
										},
									},
								},
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: nil,
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
			want: []string{constants.DatasetInitializer},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IndexTrainingRuntimeContainerRuntimeClass(tc.obj)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); len(diff) != 0 {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestIndexClusterTrainingRuntimeContainerRuntimeClass(t *testing.T) {
	cases := map[string]struct {
		obj  client.Object
		want []string
	}{
		"object is not a ClusterTrainingRuntime": {
			obj:  utiltesting.MakeClusterTrainingRuntimeWrapper("test"),
			want: nil,
		},
		"ClusterTrainingRuntime with no ReplicatedJobs": {
			obj: &trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test",
				},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{},
						},
					},
				},
			},
			want: []string{},
		},
		"ClusterTrainingRuntime with multiple ReplicatedJobs where all RuntimeClassName are nil": {
			obj: &trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test",
				},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.DatasetInitializer,
									Template: batchv1.JobTemplateSpec{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{
												constants.LabelTrainJobAncestor: constants.DatasetInitializer,
											},
										},
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: nil,
												},
											},
										},
									},
								},
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{
												constants.LabelTrainJobAncestor: constants.ModelInitializer,
											},
										},
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: nil,
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
			want: []string{},
		},
		"ClusterTrainingRuntime with ReplicatedJobs where all RuntimeClassName are set": {
			obj: &trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "test"},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.DatasetInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.DatasetInitializer),
												},
											},
										},
									},
								},
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.ModelInitializer),
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
			want: []string{constants.DatasetInitializer, constants.ModelInitializer},
		},
		"ClusterTrainingRuntime with one ReplicatedJob and RuntimeClassName set": {
			obj: &trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "test"},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.ModelInitializer),
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
			want: []string{constants.ModelInitializer},
		},

		"ClusterTrainingRuntime with ReplicatedJobs where some RuntimeClassName are set and others are nil": {
			obj: &trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "test"},
				Spec: trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: constants.DatasetInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: ptr.To(constants.DatasetInitializer),
												},
											},
										},
									},
								},
								{
									Name: constants.ModelInitializer,
									Template: batchv1.JobTemplateSpec{
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													RuntimeClassName: nil,
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
			want: []string{constants.DatasetInitializer},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IndexClusterTrainingRuntimeContainerRuntimeClass(tc.obj)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); len(diff) != 0 {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}
