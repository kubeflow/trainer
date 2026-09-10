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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/events"
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

type fakeRuntime struct {
	cleanedUp bool
	status    *trainer.TrainJobStatus
}

var _ jobruntimes.Runtime = (*fakeRuntime)(nil)

func (f *fakeRuntime) NewObjects(context.Context, *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	return nil, nil
}
func (f *fakeRuntime) RuntimeInfo(*trainer.TrainJob, any, *trainer.MLPolicy, *trainer.PodGroupPolicy) (*jobruntimes.Info, error) {
	return nil, nil
}
func (f *fakeRuntime) TrainJobStatus(context.Context, *trainer.TrainJob) (*trainer.TrainJobStatus, error) {
	return f.status, nil
}
func (f *fakeRuntime) EventHandlerRegistrars() []jobruntimes.ReconcilerBuilder {
	return nil
}
func (f *fakeRuntime) ValidateObjects(context.Context, *trainer.TrainJob, *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}
func (f *fakeRuntime) TerminalCleanup(context.Context, *trainer.TrainJob) error {
	f.cleanedUp = true
	return nil
}

func TestReconcile_TerminalCleanup(t *testing.T) {
	runtimeGVK := schema.GroupVersionKind{
		Group:   trainer.GroupVersion.Group,
		Version: trainer.GroupVersion.Version,
		Kind:    trainer.TrainingRuntimeKind,
	}
	runtimeKey := jobruntimes.RuntimeRefToRuntimeRegistryKey(trainer.RuntimeRef{
		APIGroup: &runtimeGVK.Group,
		Kind:     &runtimeGVK.Kind,
		Name:     "test-runtime",
	})

	cases := map[string]struct {
		trainJob      *trainer.TrainJob
		runtimeStatus *trainer.TrainJobStatus
		wantCleanedUp bool
	}{
		"trainJob is failed by condition": {
			trainJob: func() *trainer.TrainJob {
				tj := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "failed-job").
					RuntimeRef(runtimeGVK, "test-runtime").
					Obj()
				tj.Status.Conditions = []metav1.Condition{
					{
						Type:   trainer.TrainJobFailed,
						Status: metav1.ConditionTrue,
					},
				}
				return tj
			}(),
			wantCleanedUp: true,
		},
		"trainJob is complete by condition": {
			trainJob: func() *trainer.TrainJob {
				tj := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "complete-job").
					RuntimeRef(runtimeGVK, "test-runtime").
					Obj()
				tj.Status.Conditions = []metav1.Condition{
					{
						Type:   trainer.TrainJobComplete,
						Status: metav1.ConditionTrue,
					},
				}
				return tj
			}(),
			wantCleanedUp: true,
		},
		"trainJob is active and not finished": {
			trainJob: func() *trainer.TrainJob {
				return utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "active-job").
					RuntimeRef(runtimeGVK, "test-runtime").
					Obj()
			}(),
			wantCleanedUp: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			fakeRT := &fakeRuntime{status: tc.runtimeStatus}
			cli := utiltesting.NewClientBuilder().WithObjects(tc.trainJob).Build()

			r := &TrainJobReconciler{
				client:   cli,
				recorder: events.NewFakeRecorder(10),
				runtimes: map[string]jobruntimes.Runtime{
					runtimeKey: fakeRT,
				},
			}

			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: tc.trainJob.Namespace,
					Name:      tc.trainJob.Name,
				},
			})
			if err != nil {
				t.Fatalf("Unexpected reconcile error: %v", err)
			}

			if fakeRT.cleanedUp != tc.wantCleanedUp {
				t.Errorf("TerminalCleanup called = %v, want %v", fakeRT.cleanedUp, tc.wantCleanedUp)
			}
		})
	}
}

