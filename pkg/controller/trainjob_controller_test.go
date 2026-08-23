/*
Copyright The Kubeflow Authors.

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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	runtimecore "github.com/kubeflow/trainer/v2/pkg/runtime/core"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestReconcile_TrainJobReconciler(t *testing.T) {
	cases := map[string]struct {
		runtimeRefGVK schema.GroupVersionKind
		wantMessage   string
	}{
		"unregistered runtime kind": {
			runtimeRefGVK: trainer.SchemeGroupVersion.WithKind("UnregisteredRuntime"),
			wantMessage:   "unsupported runtime: UnregisteredRuntime.trainer.kubeflow.org",
		},
		"unregistered runtime API group": {
			runtimeRefGVK: schema.GroupVersionKind{Group: "example.com", Kind: trainer.ClusterTrainingRuntimeKind},
			wantMessage:   "unsupported runtime: ClusterTrainingRuntime.example.com",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainjob").
				RuntimeRef(tc.runtimeRefGVK, "runtime").
				Obj()
			runtimeClientBuilder := utiltesting.NewClientBuilder()
			runtimes, err := runtimecore.New(ctx, runtimeClientBuilder.Build(), utiltesting.AsIndex(runtimeClientBuilder), nil)
			if err != nil {
				t.Fatalf("New() returned error: %v", err)
			}
			cli := utiltesting.NewClientBuilder().
				WithObjects(trainJob).
				WithStatusSubresource(trainJob).
				Build()

			r := NewTrainJobReconciler(cli, events.NewFakeRecorder(1), runtimes)
			trainJobKey := client.ObjectKeyFromObject(trainJob)
			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: trainJobKey})
			if err == nil || err.Error() != tc.wantMessage {
				t.Errorf("Reconcile() returned error %v, want %q", err, tc.wantMessage)
			}

			var gotTrainJob trainer.TrainJob
			if err := cli.Get(ctx, trainJobKey, &gotTrainJob); err != nil {
				t.Fatalf("Get() returned error: %v", err)
			}
			wantConditions := []metav1.Condition{{
				Type:    trainer.TrainJobFailed,
				Status:  metav1.ConditionTrue,
				Message: tc.wantMessage,
				Reason:  trainer.TrainJobRuntimeNotSupportedReason,
			}}
			if diff := cmp.Diff(wantConditions, gotTrainJob.Status.Conditions,
				cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime", "ObservedGeneration"),
			); len(diff) != 0 {
				t.Errorf("Unexpected TrainJob conditions (-want, +got): \n%s", diff)
			}
		})
	}
}
