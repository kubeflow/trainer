package coscheduling

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestIndexTrainingRuntimeContainerRuntimeClass(t *testing.T) {
	cases := map[string]struct {
		obj  client.Object
		want []string
	}{

		"object is not a TrainingRuntime": {
			obj: utiltesting.MakeClusterTrainingRuntimeWrapper(metav1.NamespaceDefault).Obj(),
		},
		"TrainingRuntime with no ReplicatedJobs": {
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").RuntimeSpec(utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{}).Obj()).Obj(),
		},
		"TrainingRuntime with multiple ReplicatedJobs where all RuntimeClassName are nil": {
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").RuntimeSpec(utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
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
			}).Obj()).Obj(),
		},
		"TrainingRuntime with ReplicatedJobs where all RuntimeClassName are set": {
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").RuntimeSpec(utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
				ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
					{
						Name: constants.DatasetInitializer,
						Template: batchv1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										RuntimeClassName: ptr.To("containerd"),
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
										RuntimeClassName: ptr.To("containerd"),
									},
								},
							},
						},
					},
				},
			},
			).Obj()).Obj(),
			want: []string{"containerd", "containerd"},
		},
		"TrainingRuntime with one ReplicatedJob and RuntimeClassName set": {
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").RuntimeSpec(
				utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
					ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
						{
							Name: constants.ModelInitializer,
							Template: batchv1.JobTemplateSpec{
								Spec: batchv1.JobSpec{
									Template: corev1.PodTemplateSpec{
										Spec: corev1.PodSpec{
											RuntimeClassName: ptr.To("containerd"),
										},
									},
								},
							},
						},
					},
				},
				).Obj()).Obj(),
			want: []string{"containerd"},
		},
		"TrainingRuntime with ReplicatedJobs where some RuntimeClassName are set and others are nil": {
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").RuntimeSpec(
				utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
					ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
						{
							Name: constants.DatasetInitializer,
							Template: batchv1.JobTemplateSpec{
								Spec: batchv1.JobSpec{
									Template: corev1.PodTemplateSpec{
										Spec: corev1.PodSpec{
											RuntimeClassName: ptr.To("containerd"),
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
				}).Obj()).Obj(),
			want: []string{"containerd"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IndexTrainingRuntimeContainerRuntimeClass(tc.obj)
			if diff := cmp.Diff(tc.want, got); len(diff) != 0 {
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
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
		"ClusterTrainingRuntime with no ReplicatedJobs": {
			obj: utiltesting.MakeClusterTrainingRuntimeWrapper(metav1.NamespaceDefault).RuntimeSpec(utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{}).Obj()).Obj(),
		},
		"ClusterTrainingRuntime with multiple ReplicatedJobs where all RuntimeClassName are nil": {
			obj: utiltesting.MakeClusterTrainingRuntimeWrapper(metav1.NamespaceDefault).RuntimeSpec(utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
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
			}).Obj()).Obj(),
		},
		"ClusterTrainingRuntime with ReplicatedJobs where all RuntimeClassName are set": {
			obj: utiltesting.MakeClusterTrainingRuntimeWrapper(metav1.NamespaceDefault).RuntimeSpec(utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
				ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
					{
						Name: constants.DatasetInitializer,
						Template: batchv1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										RuntimeClassName: ptr.To("containerd"),
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
										RuntimeClassName: ptr.To("containerd"),
									},
								},
							},
						},
					},
				},
			},
			).Obj()).Obj(),
			want: []string{"containerd", "containerd"},
		},
		"ClusterTrainingRuntime with one ReplicatedJob and RuntimeClassName set": {
			obj: utiltesting.MakeClusterTrainingRuntimeWrapper(metav1.NamespaceDefault).RuntimeSpec(
				utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
					ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
						{
							Name: constants.ModelInitializer,
							Template: batchv1.JobTemplateSpec{
								Spec: batchv1.JobSpec{
									Template: corev1.PodTemplateSpec{
										Spec: corev1.PodSpec{
											RuntimeClassName: ptr.To("containerd"),
										},
									},
								},
							},
						},
					},
				},
				).Obj()).Obj(),
			want: []string{"containerd"},
		},
		"ClusterTrainingRuntime with ReplicatedJobs where some RuntimeClassName are set and others are nil": {
			obj: utiltesting.MakeClusterTrainingRuntimeWrapper(metav1.NamespaceDefault).RuntimeSpec(
				utiltesting.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{}).JobSetSpec(jobsetv1alpha2.JobSetSpec{
					ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
						{
							Name: constants.DatasetInitializer,
							Template: batchv1.JobTemplateSpec{
								Spec: batchv1.JobSpec{
									Template: corev1.PodTemplateSpec{
										Spec: corev1.PodSpec{
											RuntimeClassName: ptr.To("containerd"),
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
				}).Obj()).Obj(),
			want: []string{"containerd"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IndexClusterTrainingRuntimeContainerRuntimeClass(tc.obj)
			if diff := cmp.Diff(tc.want, got); len(diff) != 0 {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}
