package indexer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIndexTrainJobTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		obj  client.Object
		want []string
	}{
		"object is not a TrainJob": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{}, "test runtime"),
			want: nil,
		},
		"TrainJob with matching APIGroup and Kind": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "test runtime").TrainJob,
			want: []string{"test runtime"},
		},
		"TrainJob with non-matching APIGroup": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "trainer.kubeflow", Version: "v1alpha1", Kind: trainer.TrainingRuntimeKind}, "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with non-matching Kind": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind("TrainingRun"), "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with nil APIGroup": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: trainer.TrainingRuntimeKind}, "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with nil Kind": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(""), "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with both APIGroup and Kind nil": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: ""}, "test runtime").TrainJob,
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IndexTrainJobTrainingRuntime(tc.obj)
			if diff := cmp.Diff(tc.want, got); len(diff) != 0 {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestIndexTrainJobClusterTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		obj  client.Object
		want []string
	}{
		"object is not a TrainJob": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{}, "test runtime"),
			want: nil,
		},
		"TrainJob with matching APIGroup and Kind": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test runtime").TrainJob,
			want: []string{"test runtime"},
		},
		"TrainJob with non-matching APIGroup": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "trainer.kubeflow", Version: "v1alpha1", Kind: trainer.ClusterTrainingRuntimeKind}, "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with non-matching Kind": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind("ClusterTrainingRun"), "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with nil APIGroup": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: trainer.ClusterTrainingRuntimeKind}, "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with nil Kind": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(""), "test runtime").TrainJob,
			want: nil,
		},
		"TrainJob with both APIGroup and Kind nil": {
			obj: &utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: ""}, "test runtime").TrainJob,
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IndexTrainJobClusterTrainingRuntime(tc.obj)
			if diff := cmp.Diff(tc.want, got); len(diff) != 0 {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}
