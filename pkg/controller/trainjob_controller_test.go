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
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/ktesting"
	clocktesting "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
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

			gotResult, gotErr := r.reconcileDeadline(ctx, tc.trainJob)
			if gotErr != nil {
				t.Fatalf("reconcileDeadline() error = %v", gotErr)
			}

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

func TestReconcileDeadline_RetryJobSetDeletion(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "train-job").
		ActiveDeadlineSeconds(1).
		Obj()
	trainJob.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
	jobSet := &jobsetv1alpha2.JobSet{
		ObjectMeta: metav1.ObjectMeta{Name: trainJob.Name, Namespace: trainJob.Namespace},
	}

	deleteErr := errors.New("error deleting JobSet")
	deleteCalls := 0
	clientBuilder := utiltesting.NewClientBuilder().WithObjects(jobSet)
	clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
		Delete: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			deleteCalls++
			if deleteCalls == 1 {
				return deleteErr
			}
			return cli.Delete(ctx, obj, opts...)
		},
	})
	cli := clientBuilder.Build()
	r := NewTrainJobReconciler(cli, nil, nil)

	if _, err := r.reconcileDeadline(ctx, trainJob); !errors.Is(err, deleteErr) {
		t.Fatalf("reconcileDeadline() error = %v, want %v", err, deleteErr)
	}
	deadlineCond := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed)
	if deadlineCond == nil || deadlineCond.Reason != trainer.TrainJobDeadlineExceededReason {
		t.Fatalf("reconcileDeadline() condition = %v, want reason %q", deadlineCond, trainer.TrainJobDeadlineExceededReason)
	}

	if _, err := r.reconcileDeadline(ctx, trainJob); err != nil {
		t.Fatalf("reconcileDeadline() retry returned error: %v", err)
	}
	if deleteCalls != 2 {
		t.Errorf("reconcileDeadline() delete calls = %d, want 2", deleteCalls)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(jobSet), &jobsetv1alpha2.JobSet{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get() error = %v, want NotFound", err)
	}
}
