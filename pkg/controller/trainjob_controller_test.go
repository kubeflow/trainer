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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2/ktesting"
	clocktesting "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

// TestReconcileDeadline covers the activeDeadlineSeconds arithmetic in isolation.
// The integration suite already asserts the resulting conditions, but it polls with
// Eventually/Consistently and never inspects the returned ctrl.Result, so the requeue
// delay itself is only observable here, where the clock is fake and the result exact.
func TestReconcileDeadline(t *testing.T) {
	// created is the TrainJob creation timestamp every case is anchored on. Truncated to
	// the second because metav1.Time serializes at second granularity.
	created := metav1.NewTime(time.Now().Truncate(time.Second))
	resumed := metav1.NewTime(created.Add(time.Hour))

	failedCondition := metav1.Condition{
		Type:    trainer.TrainJobFailed,
		Status:  metav1.ConditionTrue,
		Message: constants.TrainJobDeadlineExceededMessage,
		Reason:  trainer.TrainJobDeadlineExceededReason,
	}
	resumedCondition := metav1.Condition{
		Type:               trainer.TrainJobSuspended,
		Status:             metav1.ConditionFalse,
		Message:            constants.TrainJobResumedMessage,
		Reason:             trainer.TrainJobResumedReason,
		LastTransitionTime: resumed,
	}

	cases := map[string]struct {
		trainJob *trainer.TrainJob
		// jobSet, when set, is the child JobSet seeded into the fake client.
		jobSet *jobsetv1alpha2.JobSet
		// now is the instant the fake clock reports.
		now time.Time

		wantResult     ctrl.Result
		wantConditions []metav1.Condition
		// wantJobSet is true when the child JobSet must survive the reconciliation.
		wantJobSet bool
	}{
		"deadline exceeded fails the TrainJob and deletes the child JobSet": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Obj(),
			jobSet:         utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").Obj(),
			now:            created.Add(61 * time.Second),
			wantResult:     ctrl.Result{},
			wantConditions: []metav1.Condition{failedCondition},
		},
		"deadline exceeded still fails the TrainJob when the child JobSet is already gone": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Obj(),
			now:            created.Add(61 * time.Second),
			wantResult:     ctrl.Result{},
			wantConditions: []metav1.Condition{failedCondition},
		},
		"active deadline requeues after exactly the remaining duration": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Obj(),
			jobSet:     utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").Obj(),
			now:        created.Add(20 * time.Second),
			wantResult: ctrl.Result{RequeueAfter: 40 * time.Second},
			wantJobSet: true,
		},
		"requeue is floored at one second when the deadline is exactly now": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Obj(),
			// now.After(deadline) is false on equality, so the remaining duration is
			// zero and must be clamped rather than requeued immediately.
			now:        created.Add(60 * time.Second),
			wantResult: ctrl.Result{RequeueAfter: time.Second},
		},
		"deadline is measured from the resume transition rather than the creation timestamp": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Conditions(resumedCondition).
				Obj(),
			// An hour past creation, but only 20s past the resume, so the job is still
			// well within its deadline.
			now:            resumed.Add(20 * time.Second),
			wantResult:     ctrl.Result{RequeueAfter: 40 * time.Second},
			wantConditions: []metav1.Condition{resumedCondition},
		},
		"zero creation timestamp leaves the TrainJob untouched": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				ActiveDeadlineSeconds(60).
				Obj(),
			now:        created.Add(time.Hour),
			wantResult: ctrl.Result{},
		},
		"no deadline configured leaves the TrainJob untouched": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				Obj(),
			now:        created.Add(time.Hour),
			wantResult: ctrl.Result{},
		},
		"suspended TrainJob does not start the deadline timer": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Suspend(true).
				Obj(),
			jobSet:     utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").Obj(),
			now:        created.Add(time.Hour),
			wantResult: ctrl.Result{},
			wantJobSet: true,
		},
		"finished TrainJob is not failed again once the deadline passes": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Conditions(metav1.Condition{
					Type:   trainer.TrainJobComplete,
					Status: metav1.ConditionTrue,
					Reason: "Complete",
				}).
				Obj(),
			jobSet: utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").Obj(),
			now:    created.Add(time.Hour),
			wantConditions: []metav1.Condition{{
				Type:   trainer.TrainJobComplete,
				Status: metav1.ConditionTrue,
				Reason: "Complete",
			}},
			wantResult: ctrl.Result{},
			wantJobSet: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			builder := utiltesting.NewClientBuilder().WithObjects(tc.trainJob)
			if tc.jobSet != nil {
				builder = builder.WithObjects(tc.jobSet)
			}
			cli := builder.Build()

			r := &TrainJobReconciler{
				client: cli,
				clock:  clocktesting.NewFakePassiveClock(tc.now),
			}

			gotResult := r.reconcileDeadline(ctx, tc.trainJob)

			if diff := cmp.Diff(tc.wantResult, gotResult); len(diff) != 0 {
				t.Errorf("Unexpected ctrl.Result (-want, +got): \n%s", diff)
			}
			if diff := cmp.Diff(tc.wantConditions, tc.trainJob.Status.Conditions,
				cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime"),
			); len(diff) != 0 {
				t.Errorf("Unexpected TrainJob conditions (-want, +got): \n%s", diff)
			}

			// reconcileDeadline deletes the child JobSet by name, so absence is the
			// signal that the deadline path ran to completion.
			gotJobSetErr := cli.Get(ctx, client.ObjectKey{
				Namespace: metav1.NamespaceDefault,
				Name:      tc.trainJob.Name,
			}, &jobsetv1alpha2.JobSet{})
			switch {
			case tc.wantJobSet && gotJobSetErr != nil:
				t.Errorf("Expected the child JobSet to be preserved, got error: %v", gotJobSetErr)
			case !tc.wantJobSet && !apierrors.IsNotFound(gotJobSetErr):
				t.Errorf("Expected the child JobSet to be absent, got error: %v", gotJobSetErr)
			}
		})
	}
}

// fakeRuntime is a minimal jobruntimes.Runtime stub that records whether NewObjects and
// DeleteObjects were invoked, without needing a real TrainingRuntime/ClusterTrainingRuntime
// or plugin registry.
type fakeRuntime struct {
	objsToDelete []client.Object

	newObjectsCalled    bool
	deleteObjectsCalled bool
}

func (f *fakeRuntime) NewObjects(context.Context, *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	f.newObjectsCalled = true
	return nil, nil
}

func (f *fakeRuntime) DeleteObjects(context.Context, *trainer.TrainJob) ([]client.Object, error) {
	f.deleteObjectsCalled = true
	return f.objsToDelete, nil
}

func (f *fakeRuntime) RuntimeInfo(*trainer.TrainJob, any, *trainer.MLPolicy, *trainer.PodGroupPolicy) (*jobruntimes.Info, error) {
	return nil, nil
}

func (f *fakeRuntime) TrainJobStatus(context.Context, *trainer.TrainJob) (*trainer.TrainJobStatus, error) {
	return nil, nil
}

func (f *fakeRuntime) EventHandlerRegistrars() []jobruntimes.ReconcilerBuilder {
	return nil
}

func (f *fakeRuntime) ValidateObjects(context.Context, *trainer.TrainJob, *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}

var _ jobruntimes.Runtime = (*fakeRuntime)(nil)

// TestReconcileObjects covers the branch between materializing new objects and cleaning up
// stale ones in isolation, independently of any concrete plugin.
func TestReconcileObjects(t *testing.T) {
	runningTrainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-trainjob").Obj()
	failedTrainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-trainjob").
		Conditions(metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue}).
		Obj()

	cases := map[string]struct {
		trainJob          *trainer.TrainJob
		objsToDelete      []client.Object
		seedObjs          []client.Object
		wantNewObjsCalled bool
		wantDeleted       bool
	}{
		"running TrainJob still materializes new objects": {
			trainJob:          runningTrainJob,
			wantNewObjsCalled: true,
		},
		"failed TrainJob skips materializing new objects but still cleans up": {
			trainJob: failedTrainJob,
			objsToDelete: []client.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "to-delete", Namespace: metav1.NamespaceDefault}},
			},
			seedObjs: []client.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "to-delete", Namespace: metav1.NamespaceDefault}},
			},
			wantDeleted: true,
		},
		"cleanup tolerates an object that is already gone": {
			trainJob: failedTrainJob,
			objsToDelete: []client.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "already-gone", Namespace: metav1.NamespaceDefault}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			cli := utiltesting.NewClientBuilder().WithObjects(tc.seedObjs...).Build()
			fr := &fakeRuntime{objsToDelete: tc.objsToDelete}
			r := &TrainJobReconciler{client: cli}

			if err := r.reconcileObjects(ctx, fr, tc.trainJob); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// DeleteObjects must run on every call, regardless of the TrainJob's status.
			if !fr.deleteObjectsCalled {
				t.Error("Expected DeleteObjects to be called")
			}
			if fr.newObjectsCalled != tc.wantNewObjsCalled {
				t.Errorf("NewObjects called = %v, want %v", fr.newObjectsCalled, tc.wantNewObjsCalled)
			}
			for _, obj := range tc.objsToDelete {
				err := cli.Get(ctx, client.ObjectKeyFromObject(obj), &corev1.ConfigMap{})
				if tc.wantDeleted && !apierrors.IsNotFound(err) {
					t.Errorf("Expected %s to be deleted, got error: %v", obj.GetName(), err)
				}
			}
		})
	}
}
