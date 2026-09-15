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

package controller

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestReconcile_TrainingRuntimeReconciler(t *testing.T) {
	cases := map[string]struct {
		trainingRuntime     *trainer.TrainingRuntime
		wantTrainingRuntime *trainer.TrainingRuntime
	}{
		"remove existing finalizer during reconciliation": {
			trainingRuntime: utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "runtime").
				Finalizers(constants.ResourceInUseFinalizer).
				Obj(),
			wantTrainingRuntime: utiltesting.MakeTrainingRuntimeWrapper(
				metav1.NamespaceDefault,
				"runtime",
			).Obj(),
		},
		"no action when runtime has no finalizer": {
			trainingRuntime: utiltesting.MakeTrainingRuntimeWrapper(
				metav1.NamespaceDefault,
				"runtime",
			).Obj(),

			wantTrainingRuntime: utiltesting.MakeTrainingRuntimeWrapper(
				metav1.NamespaceDefault,
				"runtime",
			).Obj(),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cli := utiltesting.NewClientBuilder().
				WithObjects(tc.trainingRuntime).
				Build()
			r := NewTrainingRuntimeReconciler(cli, nil)
			runtimeKey := client.ObjectKeyFromObject(tc.trainingRuntime)
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: runtimeKey})
			if err != nil {
				t.Fatalf("Reconcile() returned error: %v", err)
			}
			var gotRuntime trainer.TrainingRuntime
			if err := cli.Get(ctx, runtimeKey, &gotRuntime); err != nil {
				t.Fatalf("Get() returned error: %v", err)
			}
			if diff := cmp.Diff(tc.wantTrainingRuntime, &gotRuntime,
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
			); len(diff) != 0 {
				t.Errorf("Unexpected TrainingRuntime: (-want, +got): \n%s", diff)
			}
		})
	}
}
