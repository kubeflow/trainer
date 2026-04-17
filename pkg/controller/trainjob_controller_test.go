/*
Copyright 2024 The Kubeflow Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

// fakeRuntime implements jobruntimes.Runtime for unit tests.
type fakeRuntime struct {
	newObjectsErr error
}

func (f *fakeRuntime) NewObjects(_ context.Context, _ *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	return nil, f.newObjectsErr
}

func (f *fakeRuntime) RuntimeInfo(
	_ *trainer.TrainJob, _ any, _ *trainer.MLPolicy, _ *trainer.PodGroupPolicy,
) (*jobruntimes.Info, error) {
	return nil, nil
}

func (f *fakeRuntime) TrainJobStatus(_ context.Context, trainJob *trainer.TrainJob) (*trainer.TrainJobStatus, error) {
	return trainJob.Status.DeepCopy(), nil
}

func (f *fakeRuntime) EventHandlerRegistrars() []jobruntimes.ReconcilerBuilder {
	return nil
}

func (f *fakeRuntime) ValidateObjects(_ context.Context, _, _ *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}

// noopEventRecorder satisfies events.EventRecorder without side effects.
type noopEventRecorder struct{}

func (n *noopEventRecorder) Eventf(_ apiruntime.Object, _ apiruntime.Object, _, _, _, _ string, _ ...interface{}) {
}

func TestReconcile_TrainJobReconciler(t *testing.T) {
	cases := map[string]struct {
		newObjectsErr  error
		wantConditions []metav1.Condition
		wantError      bool
	}{
		"reconcileObjects succeeds: no RuntimeStatus condition set": {
			newObjectsErr:  nil,
			wantConditions: []metav1.Condition{},
		},
		"reconcileObjects fails: RuntimeStatus condition set with Status=False and Reason=BackedOff": {
			newObjectsErr: errors.New("injected reconcile error"),
			wantConditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobRuntimeStatus,
					Status: metav1.ConditionFalse,
					Reason: "BackedOff",
				},
			},
			wantError: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj()

			cli := utiltesting.NewClientBuilder().
				WithObjects(trainJob).
				WithStatusSubresource(&trainer.TrainJob{}).
				Build()

			runtimeKey := jobruntimes.RuntimeRefToRuntimeRegistryKey(trainJob.Spec.RuntimeRef)
			r := NewTrainJobReconciler(
				cli,
				&noopEventRecorder{},
				map[string]jobruntimes.Runtime{
					runtimeKey: &fakeRuntime{newObjectsErr: tc.newObjectsErr},
				},
			)

			_, gotErr := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(trainJob),
			})

			if tc.wantError && gotErr == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tc.wantError && gotErr != nil {
				t.Errorf("Unexpected error: %v", gotErr)
			}

			var gotJob trainer.TrainJob
			if err := cli.Get(ctx, client.ObjectKeyFromObject(trainJob), &gotJob); err != nil {
				t.Fatalf("Failed to get TrainJob after reconcile: %v", err)
			}

			if diff := cmp.Diff(tc.wantConditions, gotJob.Status.Conditions,
				cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime", "Message", "ObservedGeneration"),
				cmpopts.EquateEmpty(),
			); diff != "" {
				t.Errorf("Unexpected conditions (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestReconcile_TrainJobReconciler_RuntimeStatusCleared(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	var cancel func()
	ctx, cancel = context.WithCancel(ctx)
	t.Cleanup(cancel)

	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
		RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
		Obj()

	cli := utiltesting.NewClientBuilder().
		WithObjects(trainJob).
		WithStatusSubresource(&trainer.TrainJob{}).
		Build()

	runtimeKey := jobruntimes.RuntimeRefToRuntimeRegistryKey(trainJob.Spec.RuntimeRef)
	frt := &fakeRuntime{newObjectsErr: errors.New("transient error")}
	r := NewTrainJobReconciler(cli, &noopEventRecorder{}, map[string]jobruntimes.Runtime{
		runtimeKey: frt,
	})

	// First reconcile: error -> RuntimeStatus condition should be set.
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(trainJob)}); err == nil {
		t.Fatal("Expected error on first reconcile, got nil")
	}

	var gotJob trainer.TrainJob
	if err := cli.Get(ctx, client.ObjectKeyFromObject(trainJob), &gotJob); err != nil {
		t.Fatalf("Failed to get TrainJob: %v", err)
	}
	if findCondition(gotJob.Status.Conditions, trainer.TrainJobRuntimeStatus) == nil {
		t.Fatal("Expected RuntimeStatus condition after failed reconcile, got none")
	}

	// Second reconcile: no error -> RuntimeStatus condition should be cleared.
	frt.newObjectsErr = nil
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(trainJob)}); err != nil {
		t.Fatalf("Unexpected error on second reconcile: %v", err)
	}

	if err := cli.Get(ctx, client.ObjectKeyFromObject(trainJob), &gotJob); err != nil {
		t.Fatalf("Failed to get TrainJob: %v", err)
	}
	if c := findCondition(gotJob.Status.Conditions, trainer.TrainJobRuntimeStatus); c != nil {
		t.Errorf("Expected RuntimeStatus condition to be cleared, but found: %+v", c)
	}
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
