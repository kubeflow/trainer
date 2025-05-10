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
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	idxer "github.com/kubeflow/trainer/pkg/runtime/indexer"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestReconcile_ClusterTrainingRuntimeReconciler(t *testing.T) {
	errorFailedGetClusterTrainingRuntime := errors.New("TEST: failed to get ClusterTrainingRuntime")
	cases := map[string]struct {
		trainJobs             trainer.TrainJobList
		clTrainingRuntime     *trainer.ClusterTrainingRuntime
		wantClTrainingRuntime *trainer.ClusterTrainingRuntime
		wantError             error
	}{
		"no action when clusterTrainingRuntime with finalizer does not being deleted": {
			clTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Finalizers(constants.ResourceInUseFinalizer).
				Obj(),
			wantClTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Finalizers(constants.ResourceInUseFinalizer).
				Obj(),
		},
		"remove clusterTrainingRuntime due to removed finalizers when runtime without finalizer is deleting": {
			clTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Finalizers(constants.ResourceInUseFinalizer).
				DeletionTimestamp(metav1.Now()).
				Obj(),
			wantError:             errorFailedGetClusterTrainingRuntime,
			wantClTrainingRuntime: &trainer.ClusterTrainingRuntime{},
		},
		"add finalizer when clusterTrainingRuntime is used by trainJob": {
			clTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),
			trainJobs: trainer.TrainJobList{
				Items: []trainer.TrainJob{
					*utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "runtime").
						Obj(),
				},
			},
			wantClTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Finalizers(constants.ResourceInUseFinalizer).
				Obj(),
		},
		"no action when all TrainJobs use another ClusterTrainingRuntime": {
			clTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),
			trainJobs: trainer.TrainJobList{
				Items: []trainer.TrainJob{
					*utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "another").
						Obj(),
				},
			},
			wantClTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),
		},
		"no action when runtime without finalizer is not used by any TrainJob": {
			clTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),
			wantClTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cli := utiltesting.NewClientBuilder().
				WithObjects(tc.clTrainingRuntime).
				WithLists(&tc.trainJobs).
				WithIndex(&trainer.TrainJob{}, idxer.TrainJobClusterRuntimeRefKey, idxer.IndexTrainJobClusterTrainingRuntime).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*trainer.TrainJob); !ok && errors.Is(tc.wantError, errorFailedGetClusterTrainingRuntime) {
							return errorFailedGetClusterTrainingRuntime
						}
						return cli.Get(ctx, key, obj, opts...)
					},
				}).
				Build()
			r := NewClusterTrainingRuntimeReconciler(cli, nil)
			clRuntimeKey := client.ObjectKeyFromObject(tc.clTrainingRuntime)
			_, gotError := r.Reconcile(ctx, reconcile.Request{NamespacedName: clRuntimeKey})
			if diff := cmp.Diff(tc.wantError, gotError, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected Reconcile error (-want, +got): \n%s", diff)
			}
			var gotClRuntime trainer.ClusterTrainingRuntime
			gotError = cli.Get(ctx, clRuntimeKey, &gotClRuntime)
			if diff := cmp.Diff(tc.wantError, gotError, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected GET error (-want, +got): \n%s", diff)
			}
			if diff := cmp.Diff(tc.wantClTrainingRuntime, &gotClRuntime,
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
			); len(diff) != 0 {
				t.Errorf("Unexpected ClusterTrainingRuntime: (-want, +got): \n%s", diff)
			}
		})
	}
}

func TestNotifyTrainJobUpdate_ClusterTrainingRuntimeReconciler(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		oldJob    *trainer.TrainJob
		newJob    *trainer.TrainJob
		wantEvent event.TypedGenericEvent[iter.Seq[types.NamespacedName]]
	}{
		"UPDATE Event: runtimeRef is ClusterTrainingRuntime": {
			oldJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			newJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				SpecLabel("key", "value").
				Obj(),
			wantEvent: event.TypedGenericEvent[iter.Seq[types.NamespacedName]]{
				Object: func(yield func(types.NamespacedName) bool) {
					yield(types.NamespacedName{Name: "test-runtime"})
				},
			},
		},
		"UPDATE Event: runtimeRef is not ClusterTrainingRuntime": {
			oldJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			newJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				SpecLabel("key", "value").
				Obj(),
		},
		"CREATE Event: runtimeRef is ClusterTrainingRuntime": {
			newJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			wantEvent: event.TypedGenericEvent[iter.Seq[types.NamespacedName]]{
				Object: func(yield func(types.NamespacedName) bool) {
					yield(types.NamespacedName{Name: "test-runtime"})
				},
			},
		},
		"CREATE Event: runtimeRef is not ClusterTrainingRuntime": {
			newJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
		},
		"DELETE Event: runtimeRef is ClusterTrainingRuntime": {
			oldJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			wantEvent: event.TypedGenericEvent[iter.Seq[types.NamespacedName]]{
				Object: func(yield func(types.NamespacedName) bool) {
					yield(types.NamespacedName{Name: "test-runtime"})
				},
			},
		},
		"DELETE Event: runtimeRef is not ClusterTrainingRuntime": {
			oldJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			logger, _ := ktesting.NewTestContext(t)
			updateCh := make(chan event.TypedGenericEvent[iter.Seq[types.NamespacedName]], 1)
			t.Cleanup(func() {
				close(updateCh)
			})
			r := &ClusterTrainingRuntimeReconciler{
				log:                        logger,
				nonClRuntimeObjectUpdateCh: updateCh,
			}
			r.NotifyTrainJobUpdate(tc.oldJob, tc.newJob)
			var got event.TypedGenericEvent[iter.Seq[types.NamespacedName]]
			select {
			case got = <-updateCh:
			case <-time.After(time.Second):
			}
			if diff := cmp.Diff(tc.wantEvent, got, utiltesting.TrainJobUpdateReconcileRequestCmpOpts); len(diff) != 0 {
				t.Errorf("Unexpected GenericEvent (-want, +got):\n%s", diff)
			}
		})
	}
}
