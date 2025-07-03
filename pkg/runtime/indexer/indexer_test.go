package indexer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestIndexTrainJobTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		obj  client.Object
		want []string
	}{
		"object is not a TrainJob": {
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
		"TrainJob with matching APIGroup and Kind": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "runtime").Obj(),
			want: []string{"runtime"},
		},
		"TrainJob with non-matching APIGroup": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "trainer.kubeflow", Version: "v1alpha1", Kind: trainer.TrainingRuntimeKind}, "runtime").Obj(),
		},
		"TrainJob with non-matching Kind": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind("TrainingRun"), "runtime").Obj(),
		},
		"TrainJob with nil APIGroup": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: trainer.TrainingRuntimeKind}, "runtime").Obj(),
		},
		"TrainJob with nil Kind": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(""), "runtime").Obj(),
		},
		"TrainJob with both APIGroup and Kind nil": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: ""}, "runtime").Obj(),
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
			obj: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
		"TrainJob with matching APIGroup and Kind": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "runtime").Obj(),
			want: []string{"runtime"},
		},
		"TrainJob with non-matching APIGroup": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "trainer.kubeflow", Version: "v1alpha1", Kind: trainer.ClusterTrainingRuntimeKind}, "runtime").Obj(),
		},
		"TrainJob with non-matching Kind": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind("ClusterTrainingRun"), "runtime").Obj(),
		},
		"TrainJob with nil APIGroup": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: trainer.ClusterTrainingRuntimeKind}, "runtime").Obj(),
		},
		"TrainJob with nil Kind": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.GroupVersion.WithKind(""), "runtime").Obj(),
		},
		"TrainJob with both APIGroup and Kind nil": {
			obj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(schema.GroupVersionKind{Group: "", Version: "v1alpha1", Kind: ""}, "runtime").Obj(),
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
