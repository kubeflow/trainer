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

package pdb

import (
	"cmp"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestPodDisruptionBudget(t *testing.T) {
	objCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) int {
			return cmp.Compare(a.GetObjectKind().GroupVersionKind().String(), b.GetObjectKind().GroupVersionKind().String())
		}),
	}

	cases := map[string]struct {
		info     *runtime.Info
		trainJob *trainer.TrainJob
		wantObjs []apiruntime.Object
	}{
		"no action when pod group policy is nil": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{Name: constants.Node, Count: ptr.To[int32](2)},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Obj(),
		},
		"no action for single trainer-replica gang job": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{Name: constants.Node, Ancestor: ptr.To(constants.AncestorTrainer), Count: ptr.To[int32](1)},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Obj(),
		},
		"pdb counts only trainer replicas, excluding initializers": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{Name: constants.DatasetInitializer, Ancestor: ptr.To(constants.DatasetInitializer), Count: ptr.To[int32](1)},
						{Name: constants.Node, Ancestor: ptr.To(constants.AncestorTrainer), Count: ptr.To[int32](3)},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Obj(),
			wantObjs: []apiruntime.Object{
				utiltesting.MakePodDisruptionBudgetWrapper("test-job", metav1.NamespaceDefault).
					MinAvailable(3).
					MatchLabels(map[string]string{jobsetv1alpha2.JobSetNameKey: "test-job"}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "test-uid").
					Obj(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			cli := utiltesting.NewClientBuilder().Build()
			p, err := New(ctx, cli, nil, nil)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			objs, err := p.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			typedObjs, err := utiltesting.ToObject(cli.Scheme(), objs...)
			if err != nil {
				t.Fatalf("ToObject failed: %v", err)
			}
			if diff := gocmp.Diff(tc.wantObjs, typedObjs, objCmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects from Build (-want, +got): %s", diff)
			}
		})
	}
}
