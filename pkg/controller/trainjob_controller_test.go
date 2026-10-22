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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2/ktesting"
	clocktesting "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/pkg/util/trainjob"
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
				UID("deadline-job-uid").
				CreationTimestamp(created).
				ActiveDeadlineSeconds(60).
				Obj(),
			jobSet: utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").
				ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "deadline-job", "deadline-job-uid").
				Obj(),
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
		"zero creation timestamp starts a fresh deadline window": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").
				ActiveDeadlineSeconds(60).
				Obj(),
			now:        created.Add(time.Hour),
			wantResult: ctrl.Result{RequeueAfter: time.Minute},
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

			r.reconcileStartTime(tc.trainJob)
			gotResult, err := r.reconcileDeadline(ctx, tc.trainJob, tc.trainJob.Spec.ActiveDeadlineSeconds)
			if err != nil {
				t.Fatalf("Unexpected deadline reconciliation error: %v", err)
			}

			if diff := cmp.Diff(tc.wantResult, gotResult); len(diff) != 0 {
				t.Errorf("Unexpected ctrl.Result (-want, +got): \n%s", diff)
			}
			if diff := cmp.Diff(tc.wantConditions, tc.trainJob.Status.Conditions,
				cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime"),
			); len(diff) != 0 {
				t.Errorf("Unexpected TrainJob conditions (-want, +got): \n%s", diff)
			}

			// reconcileDeadline deletes the child JobSet it controls, so absence is the
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

func TestResolveRunPolicy(t *testing.T) {
	cases := map[string]struct {
		spec     trainer.TrainJobSpec
		defaults *trainer.RunPolicy
		want     trainer.RunPolicy
	}{
		"no policy": {},
		"runtime defaults": {
			defaults: &trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](60), TTLSecondsAfterFinished: ptr.To[int32](30)},
			want:     trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](60), TTLSecondsAfterFinished: ptr.To[int32](30)},
		},
		"TrainJob overrides each default": {
			spec:     trainer.TrainJobSpec{ActiveDeadlineSeconds: 120, TTLSecondsAfterFinished: ptr.To[int32](90)},
			defaults: &trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](60), TTLSecondsAfterFinished: ptr.To[int32](30)},
			want:     trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](120), TTLSecondsAfterFinished: ptr.To[int32](90)},
		},
		"zero TTL overrides without disabling inherited deadline": {
			spec:     trainer.TrainJobSpec{TTLSecondsAfterFinished: ptr.To[int32](0)},
			defaults: &trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](60), TTLSecondsAfterFinished: ptr.To[int32](30)},
			want:     trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](60), TTLSecondsAfterFinished: ptr.To[int32](0)},
		},
		"deadline override preserves inherited TTL": {
			spec:     trainer.TrainJobSpec{ActiveDeadlineSeconds: 120},
			defaults: &trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](60), TTLSecondsAfterFinished: ptr.To[int32](30)},
			want:     trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](120), TTLSecondsAfterFinished: ptr.To[int32](30)},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := resolveRunPolicy(&trainer.TrainJob{Spec: tc.spec}, tc.defaults)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected effective policy (-want,+got): %s", diff)
			}
		})
	}
}

func TestReconcileStartTime(t *testing.T) {
	created := metav1.NewTime(time.Now().Truncate(time.Second))
	resumed := metav1.NewTime(created.Add(time.Hour))
	cases := map[string]struct {
		job  *trainer.TrainJob
		want *metav1.Time
	}{
		"initial active run": {
			job:  &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: created}},
			want: &created,
		},
		"suspended clears timestamp": {
			job: &trainer.TrainJob{Spec: trainer.TrainJobSpec{Suspend: ptr.To(true)}, Status: trainer.TrainJobStatus{StartTime: &created}},
		},
		"resume starts new active window": {
			job: &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: created}, Status: trainer.TrainJobStatus{Conditions: []metav1.Condition{{
				Type: trainer.TrainJobSuspended, Status: metav1.ConditionFalse, LastTransitionTime: resumed,
			}}}},
			want: &resumed,
		},
		"existing window survives controller restart": {
			job:  &trainer.TrainJob{Status: trainer.TrainJobStatus{StartTime: &created}},
			want: &created,
		},
		"finished run keeps timestamp": {
			job: &trainer.TrainJob{Spec: trainer.TrainJobSpec{Suspend: ptr.To(true)}, Status: trainer.TrainJobStatus{StartTime: &created, Conditions: []metav1.Condition{{
				Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue,
			}}}},
			want: &created,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &TrainJobReconciler{clock: clocktesting.NewFakePassiveClock(resumed.Time)}
			r.reconcileStartTime(tc.job)
			if diff := cmp.Diff(tc.want, tc.job.Status.StartTime); diff != "" {
				t.Errorf("Unexpected start time (-want,+got): %s", diff)
			}
		})
	}
}

func TestReconcileDeadlineRetriesChildDeletion(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	started := metav1.NewTime(time.Now().Truncate(time.Second))
	now := started.Add(61 * time.Second)
	job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-retry").UID("deadline-retry-uid").ActiveDeadlineSeconds(60).Obj()
	job.Status.StartTime = &started
	jobSet := utiltesting.MakeJobSetWrapper(job.Namespace, job.Name).
		ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), job.Name, string(job.UID)).
		Obj()
	deleteErr := errors.New("delete failed")
	deletions := 0
	cli := utiltesting.NewClientBuilder().WithObjects(jobSet).WithInterceptorFuncs(interceptor.Funcs{
		Delete: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			deletions++
			if deletions == 1 {
				return deleteErr
			}
			return cli.Delete(ctx, obj, opts...)
		},
	}).Build()
	r := &TrainJobReconciler{client: cli, clock: clocktesting.NewFakePassiveClock(now)}
	if _, err := r.reconcileDeadline(ctx, job, job.Spec.ActiveDeadlineSeconds); !errors.Is(err, deleteErr) {
		t.Fatalf("Expected child deletion error, got %v", err)
	}
	condition := meta.FindStatusCondition(job.Status.Conditions, trainer.TrainJobFailed)
	if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != trainer.TrainJobDeadlineExceededReason {
		t.Fatalf("Expected terminal deadline failure, got %v", condition)
	}
	finished := condition.LastTransitionTime
	if err := cli.Get(ctx, client.ObjectKeyFromObject(jobSet), &jobsetv1alpha2.JobSet{}); err != nil {
		t.Fatalf("Expected child retained after failed deletion: %v", err)
	}
	r.clock = clocktesting.NewFakePassiveClock(now.Add(time.Minute))
	if _, err := r.reconcileDeadline(ctx, job, job.Spec.ActiveDeadlineSeconds); err != nil {
		t.Fatalf("Unexpected retry error: %v", err)
	}
	if deletions != 2 || !meta.FindStatusCondition(job.Status.Conditions, trainer.TrainJobFailed).LastTransitionTime.Equal(&finished) {
		t.Fatalf("Retry did not preserve terminal time or retry deletion: %d deletions", deletions)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(jobSet), &jobsetv1alpha2.JobSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("Expected child deleted on retry, got %v", err)
	}
	if _, err := r.reconcileDeadline(ctx, job, job.Spec.ActiveDeadlineSeconds); err != nil || deletions != 2 {
		t.Fatalf("Absent child triggered another deletion: error %v, deletions %d", err, deletions)
	}
}

func TestDeleteDeadlineJobSet(t *testing.T) {
	trainJobGVK := trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind)
	cases := map[string]struct {
		jobSet *jobsetv1alpha2.JobSet
		// cachedUID, when set, is the UID the cached read returns instead of the stored one.
		cachedUID   string
		wantDeletes int
		wantGone    bool
		wantErr     func(error) bool
	}{
		"owned JobSet is deleted": {
			jobSet:      utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").ControllerReference(trainJobGVK, "deadline-job", "deadline-job-uid").Obj(),
			wantDeletes: 1,
			wantGone:    true,
		},
		"absent JobSet is not deleted": {},
		"terminating JobSet is not deleted again": {
			jobSet: func() *jobsetv1alpha2.JobSet {
				jobSet := utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").ControllerReference(trainJobGVK, "deadline-job", "deadline-job-uid").Obj()
				jobSet.Finalizers = []string{"test.kubeflow.org/finalizer"}
				jobSet.DeletionTimestamp = ptr.To(metav1.Now())
				return jobSet
			}(),
		},
		"JobSet without a controller reference is retained": {
			jobSet: utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").Obj(),
		},
		"JobSet controlled by a previous TrainJob is retained": {
			jobSet: utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").ControllerReference(trainJobGVK, "deadline-job", "previous-uid").Obj(),
		},
		"JobSet replaced after the cached read is retained": {
			jobSet:      utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "deadline-job").ControllerReference(trainJobGVK, "deadline-job", "deadline-job-uid").Obj(),
			cachedUID:   "replaced-jobset-uid",
			wantDeletes: 1,
			wantErr:     apierrors.IsConflict,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "deadline-job").UID("deadline-job-uid").Obj()
			builder := utiltesting.NewClientBuilder()
			if tc.jobSet != nil {
				tc.jobSet.UID = "jobset-uid"
				builder = builder.WithObjects(tc.jobSet)
			}
			deletes := 0
			cli := builder.WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if err := cli.Get(ctx, key, obj, opts...); err != nil {
						return err
					}
					if tc.cachedUID != "" {
						obj.SetUID(types.UID(tc.cachedUID))
					}
					return nil
				},
				Delete: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					deletes++
					deleteOpts := &client.DeleteOptions{}
					deleteOpts.ApplyOptions(opts)
					stored := &jobsetv1alpha2.JobSet{}
					if err := cli.Get(ctx, client.ObjectKeyFromObject(obj), stored); err != nil {
						return err
					}
					// The fake client ignores UID preconditions, so enforce them as the apiserver does.
					if deleteOpts.Preconditions != nil && deleteOpts.Preconditions.UID != nil && *deleteOpts.Preconditions.UID != stored.UID {
						return apierrors.NewConflict(jobsetv1alpha2.Resource("jobsets"), obj.GetName(), errors.New("UID precondition failed"))
					}
					return cli.Delete(ctx, obj, opts...)
				},
			}).Build()
			r := &TrainJobReconciler{client: cli}

			err := r.deleteDeadlineJobSet(ctx, job)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tc.wantErr != nil && !tc.wantErr(err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			if deletes != tc.wantDeletes {
				t.Errorf("Delete requests=%d, want %d", deletes, tc.wantDeletes)
			}
			err = cli.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: job.Name}, &jobsetv1alpha2.JobSet{})
			if tc.jobSet != nil && apierrors.IsNotFound(err) != tc.wantGone {
				t.Errorf("Unexpected JobSet deletion: %v", err)
			}
		})
	}
}

func TestReconcileTTL(t *testing.T) {
	finished := metav1.NewTime(time.Now().Truncate(time.Second))
	cases := map[string]struct {
		ttl       *int32
		condition *metav1.Condition
		owner     *metav1.OwnerReference
		deleting  bool
		now       time.Time
		wantDelay time.Duration
		wantGone  bool
		wantErr   bool
	}{
		"unset disables cleanup": {now: finished.Add(time.Hour)},
		"terminating job is not deleted again": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Time, deleting: true,
		},
		"unfinished job retained": {ttl: ptr.To[int32](0), now: finished.Add(time.Hour)},
		"false terminal condition retained": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionFalse, LastTransitionTime: finished}, now: finished.Add(time.Hour),
		},
		"successful job expires": {
			ttl: ptr.To[int32](60), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Add(time.Minute), wantGone: true,
		},
		"failed job expires": {
			ttl: ptr.To[int32](60), condition: &metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Add(time.Minute), wantGone: true,
		},
		"TTL zero deletes immediately": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Time, wantGone: true,
		},
		"successful OptimizationJob trial retained": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Time,
			owner: &metav1.OwnerReference{APIVersion: trainer.GroupVersion.String(), Kind: "OptimizationJob", Name: "optimization", UID: "optimization-uid", Controller: ptr.To(true)},
		},
		"failed OptimizationJob trial retained": {
			ttl: ptr.To[int32](60), condition: &metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Add(time.Minute),
			owner: &metav1.OwnerReference{APIVersion: trainer.GroupVersion.String(), Kind: "OptimizationJob", Name: "optimization", UID: "optimization-uid", Controller: ptr.To(true)},
		},
		"OptimizationJob non-controller owner does not prevent cleanup": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Time, wantGone: true,
			owner: &metav1.OwnerReference{APIVersion: trainer.GroupVersion.String(), Kind: "OptimizationJob", Name: "optimization", UID: "optimization-uid", Controller: ptr.To(false)},
		},
		"unrelated API group does not prevent cleanup": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Time, wantGone: true,
			owner: &metav1.OwnerReference{APIVersion: "example.com/v1alpha1", Kind: "OptimizationJob", Name: "optimization", UID: "optimization-uid", Controller: ptr.To(true)},
		},
		"unrelated controller kind does not prevent cleanup": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Time, wantGone: true,
			owner: &metav1.OwnerReference{APIVersion: trainer.GroupVersion.String(), Kind: "OtherJob", Name: "other", UID: "other-uid", Controller: ptr.To(true)},
		},
		"pending expiry requeues remaining time": {
			ttl: ptr.To[int32](60), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Add(20 * time.Second), wantDelay: 40 * time.Second,
		},
		"longer TTL uses original finish time": {
			ttl: ptr.To[int32](120), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Add(90 * time.Second), wantDelay: 30 * time.Second,
		},
		"future finish time defers cleanup": {
			ttl: ptr.To[int32](60), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}, now: finished.Add(-time.Minute), wantDelay: 2 * time.Minute,
		},
		"missing finish time reports error": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue}, now: finished.Time, wantErr: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "ttl-job").UID("ttl-job-uid").Obj()
			if tc.owner != nil {
				job.OwnerReferences = []metav1.OwnerReference{*tc.owner}
			}
			if tc.condition != nil {
				job.Status.Conditions = []metav1.Condition{*tc.condition}
			}
			if tc.deleting {
				job.Finalizers = []string{metav1.FinalizerDeleteDependents}
				job.DeletionTimestamp = ptr.To(metav1.Now())
			}
			deletes := 0
			cli := utiltesting.NewClientBuilder().WithObjects(job).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					deletes++
					return cli.Delete(ctx, obj, opts...)
				},
			}).Build()
			if err := cli.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
				t.Fatal(err)
			}
			r := &TrainJobReconciler{client: cli, clock: clocktesting.NewFakePassiveClock(tc.now)}
			result, err := r.reconcileTTL(ctx, job, tc.ttl)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Unexpected TTL error: %v", err)
			}
			if result.RequeueAfter != tc.wantDelay {
				t.Errorf("RequeueAfter=%s, want %s", result.RequeueAfter, tc.wantDelay)
			}
			wantDeletes := 0
			if tc.wantGone {
				wantDeletes = 1
			}
			if deletes != wantDeletes {
				t.Errorf("Delete requests=%d, want %d", deletes, wantDeletes)
			}
			err = cli.Get(ctx, client.ObjectKeyFromObject(job), &trainer.TrainJob{})
			if apierrors.IsNotFound(err) != tc.wantGone {
				t.Errorf("Unexpected TrainJob deletion: %v", err)
			}
		})
	}
}

type lifecycleRuntime struct {
	jobruntimes.Runtime
	status *trainer.TrainJobStatus
}

func (r *lifecycleRuntime) NewObjects(context.Context, *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	return nil, nil
}

func (r *lifecycleRuntime) DeleteObjects(context.Context, *trainer.TrainJob) ([]client.Object, error) {
	return nil, nil
}

func (r *lifecycleRuntime) TrainJobStatus(context.Context, *trainer.TrainJob) (*trainer.TrainJobStatus, error) {
	return r.status, nil
}

func (r *lifecycleRuntime) RunPolicy(context.Context, *trainer.TrainJob) (*trainer.RunPolicy, error) {
	return nil, nil
}

func TestReconcileExternallyManagedTTL(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	finished := metav1.NewTime(now.Add(-time.Minute))
	cases := map[string]struct {
		ttl       *int32
		condition *metav1.Condition
		wantGone  bool
		wantDelay time.Duration
	}{
		"completed manager expires": {
			ttl: ptr.To[int32](30), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}, wantGone: true,
		},
		"failed manager expires": {
			ttl: ptr.To[int32](30), condition: &metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue}, wantGone: true,
		},
		"deadline failure expires": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue, Reason: trainer.TrainJobDeadlineExceededReason}, wantGone: true,
		},
		"pending manager cleanup requeues": {
			ttl: ptr.To[int32](120), condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}, wantDelay: time.Minute,
		},
		"unset manager TTL retains the job": {
			condition: &metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
		},
		"unfinished manager with zero TTL is retained": {ttl: ptr.To[int32](0)},
		"false terminal condition is retained": {
			ttl: ptr.To[int32](0), condition: &metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionFalse},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "manager-job").UID("manager-job-uid").Obj()
			job.Spec.ManagedBy = ptr.To("kueue.x-k8s.io/multikueue")
			job.Spec.TTLSecondsAfterFinished = tc.ttl
			if tc.condition != nil {
				job.Status.Conditions = []metav1.Condition{*tc.condition}
				job.Status.Conditions[0].LastTransitionTime = finished
			}
			cli := utiltesting.NewClientBuilder().WithObjects(job).WithStatusSubresource(job).Build()
			r := &TrainJobReconciler{
				client: cli, clock: clocktesting.NewFakePassiveClock(now),
				runtimes: map[string]jobruntimes.Runtime{
					jobruntimes.RuntimeRefToRuntimeRegistryKey(job.Spec.RuntimeRef): &struct{ jobruntimes.Runtime }{},
				},
			}
			result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(job)})
			if err != nil || result.RequeueAfter != tc.wantDelay {
				t.Fatalf("Unexpected result %v, error %v; want delay %s", result, err, tc.wantDelay)
			}
			got := &trainer.TrainJob{}
			err = cli.Get(ctx, client.ObjectKeyFromObject(job), got)
			if tc.wantGone {
				if !apierrors.IsNotFound(err) {
					t.Fatalf("Expected expired manager TrainJob deleted, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(job.Status, got.Status); diff != "" {
				t.Errorf("External TrainJob status changed (-want,+got): %s", diff)
			}
		})
	}
}

func TestReconcileTTLWaitsForTerminalStatusPersistence(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	finished := metav1.NewTime(time.Now().Truncate(time.Second))
	job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "status-retry").UID("status-retry-uid").Obj()
	job.Spec.TTLSecondsAfterFinished = ptr.To[int32](0)
	status := &trainer.TrainJobStatus{Conditions: []metav1.Condition{{
		Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished,
	}}}
	patchErr := errors.New("status patch failed")
	deletions := 0
	cli := utiltesting.NewClientBuilder().WithObjects(job).WithStatusSubresource(job).WithInterceptorFuncs(interceptor.Funcs{
		SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
			return patchErr
		},
		Delete: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			deletions++
			return cli.Delete(ctx, obj, opts...)
		},
	}).Build()
	r := &TrainJobReconciler{
		client: cli, clock: clocktesting.NewFakePassiveClock(finished.Time), recorder: events.NewFakeRecorder(1),
		runtimes: map[string]jobruntimes.Runtime{jobruntimes.RuntimeRefToRuntimeRegistryKey(job.Spec.RuntimeRef): &lifecycleRuntime{status: status}},
	}
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(job)}); !errors.Is(err, patchErr) {
		t.Fatalf("Expected status patch error, got %v", err)
	}
	got := &trainer.TrainJob{}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(job), got); err != nil {
		t.Fatalf("Expected TrainJob retained after status patch failure: %v", err)
	}
	if deletions != 0 || trainjob.IsTrainJobFinished(got) {
		t.Fatalf("Unpersisted terminal status triggered cleanup: %d deletions, status %v", deletions, got.Status)
	}
}

func TestReconcileTTLRejectsStaleObject(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	finished := metav1.NewTime(time.Now().Truncate(time.Second))
	job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "stale-job").UID("stale-job-uid").Obj()
	job.Spec.TTLSecondsAfterFinished = ptr.To[int32](0)
	job.Status.Conditions = []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}}
	cli := utiltesting.NewClientBuilder().WithObjects(job).Build()
	if err := cli.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
		t.Fatal(err)
	}
	latest := job.DeepCopy()
	latest.Spec.TTLSecondsAfterFinished = ptr.To[int32](120)
	if err := cli.Update(ctx, latest); err != nil {
		t.Fatal(err)
	}
	r := &TrainJobReconciler{client: cli, clock: clocktesting.NewFakePassiveClock(finished.Time)}
	if _, err := r.reconcileTTL(ctx, job, job.Spec.TTLSecondsAfterFinished); !apierrors.IsConflict(err) {
		t.Fatalf("Expected stale deletion rejected by its resource-version precondition, got %v", err)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(job), &trainer.TrainJob{}); err != nil {
		t.Fatalf("Expected current TrainJob retained: %v", err)
	}
}

func TestReconcileTTLDeleteConflict(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	finished := metav1.NewTime(time.Now().Truncate(time.Second))
	job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "conflict-job").UID("conflict-job-uid").Obj()
	job.Status.Conditions = []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}}
	deletions := 0
	cli := utiltesting.NewClientBuilder().WithObjects(job).WithInterceptorFuncs(interceptor.Funcs{
		Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			deletions++
			options := &client.DeleteOptions{}
			options.ApplyOptions(opts)
			if ptr.Deref(options.PropagationPolicy, "") != metav1.DeletePropagationForeground {
				t.Error("TTL deletion does not use foreground propagation")
			}
			if options.Preconditions == nil || ptr.Deref(options.Preconditions.UID, "") != job.UID || ptr.Deref(options.Preconditions.ResourceVersion, "") != job.ResourceVersion {
				t.Error("TTL deletion does not fence concurrent changes with UID and resource version")
			}
			return apierrors.NewConflict(trainer.Resource("trainjobs"), obj.GetName(), errors.New("TTL changed during deletion"))
		},
	}).Build()
	if err := cli.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
		t.Fatal(err)
	}
	r := &TrainJobReconciler{client: cli, clock: clocktesting.NewFakePassiveClock(finished.Time)}
	if _, err := r.reconcileTTL(ctx, job, ptr.To[int32](0)); !apierrors.IsConflict(err) || deletions != 1 {
		t.Fatalf("Expected retryable deletion conflict, got %v, deletions %d", err, deletions)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(job), &trainer.TrainJob{}); err != nil {
		t.Fatalf("Expected TrainJob retained after conflict: %v", err)
	}
}

func TestReconcileTTLUpdatedDuringStatusPatch(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	finished := metav1.NewTime(time.Now().Truncate(time.Second))
	job := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "retained-job").UID("retained-job-uid").Obj()
	job.Spec.TTLSecondsAfterFinished = ptr.To[int32](0)
	job.Status.Conditions = []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, LastTransitionTime: finished}}
	status := job.Status.DeepCopy()
	status.StartTime = &finished
	cli := utiltesting.NewClientBuilder().WithObjects(job).WithStatusSubresource(job).WithInterceptorFuncs(interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, cli client.Client, subResource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			latest := &trainer.TrainJob{}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(obj), latest); err != nil {
				return err
			}
			latest.Spec.TTLSecondsAfterFinished = ptr.To[int32](120)
			if err := cli.Update(ctx, latest); err != nil {
				return err
			}
			return cli.SubResource(subResource).Patch(ctx, obj, patch, opts...)
		},
	}).Build()
	r := &TrainJobReconciler{
		client: cli, clock: clocktesting.NewFakePassiveClock(finished.Time), recorder: events.NewFakeRecorder(1),
		runtimes: map[string]jobruntimes.Runtime{jobruntimes.RuntimeRefToRuntimeRegistryKey(job.Spec.RuntimeRef): &lifecycleRuntime{status: status}},
	}
	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(job)})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != 120*time.Second {
		t.Errorf("RequeueAfter=%s, want 120s", result.RequeueAfter)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(job), &trainer.TrainJob{}); err != nil {
		t.Errorf("TTL update did not retain TrainJob: %v", err)
	}
}

// TestReconcileUnsupportedRuntime covers the branch where spec.runtimeRef resolves to no
// registered runtime. The reconciler reports that with a failed condition, so it must not
// also derive the status from the runtime it did not find.
func TestReconcileUnsupportedRuntime(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)

	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "unsupported-job").
		RuntimeRef(schema.GroupVersionKind{
			Group: "example.com",
			Kind:  "NoSuchRuntime",
		}, "no-such-runtime").
		Obj()

	cli := utiltesting.NewClientBuilder().
		WithObjects(trainJob).
		WithStatusSubresource(trainJob).
		Build()

	r := &TrainJobReconciler{
		client:   cli,
		recorder: events.NewFakeRecorder(1),
		clock:    clocktesting.NewFakePassiveClock(time.Now()),
		// Empty registry: nothing matches the runtimeRef above.
		runtimes: map[string]jobruntimes.Runtime{},
	}

	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(trainJob)})
	if err == nil {
		t.Fatal("Expected an error for the unsupported runtime, got nil")
	}

	var got trainer.TrainJob
	if getErr := cli.Get(ctx, client.ObjectKeyFromObject(trainJob), &got); getErr != nil {
		t.Fatalf("Unexpected error getting the TrainJob: %v", getErr)
	}
	wantConditions := []metav1.Condition{{
		Type:    trainer.TrainJobFailed,
		Status:  metav1.ConditionTrue,
		Message: "unsupported runtime: NoSuchRuntime.example.com",
		Reason:  trainer.TrainJobRuntimeNotSupportedReason,
	}}
	if diff := cmp.Diff(wantConditions, got.Status.Conditions,
		cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime", "ObservedGeneration"),
	); len(diff) != 0 {
		t.Errorf("Unexpected TrainJob conditions (-want, +got): \n%s", diff)
	}
}

// fakeRuntime is a minimal jobruntimes.Runtime stub that records whether NewObjects and
// DeleteObjects were invoked, without needing a real TrainingRuntime/ClusterTrainingRuntime
// or plugin registry. newObjectsErr/deleteObjectsErr let tests exercise error propagation
// from each without a real plugin returning one.
type fakeRuntime struct {
	// Embedded so only the methods reconcileObjects calls are implemented; any other call panics.
	jobruntimes.Runtime

	objsToApply      []apiruntime.ApplyConfiguration
	objsToDelete     []client.Object
	newObjectsErr    error
	deleteObjectsErr error

	newObjectsCalled    bool
	deleteObjectsCalled bool
}

func (f *fakeRuntime) NewObjects(context.Context, *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	f.newObjectsCalled = true
	return f.objsToApply, f.newObjectsErr
}

func (f *fakeRuntime) DeleteObjects(context.Context, *trainer.TrainJob) ([]client.Object, error) {
	f.deleteObjectsCalled = true
	return f.objsToDelete, f.deleteObjectsErr
}

// TestReconcileObjects covers the branch between materializing new objects and cleaning up
// stale ones in isolation, independently of any concrete plugin.
func TestReconcileObjects(t *testing.T) {
	runningTrainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-trainjob").Obj()
	suspendedTrainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-trainjob").
		Suspend(true).
		Obj()
	failedTrainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-trainjob").
		Conditions(metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue}).
		Obj()

	errNewObjects := errors.New("newObjects failed")
	errApply := errors.New("apply failed")
	errDeleteObjects := errors.New("deleteObjects failed")
	errClientDelete := errors.New("client delete failed")

	toDeleteObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "to-delete", Namespace: metav1.NamespaceDefault}}
	toApplyObjs := []apiruntime.ApplyConfiguration{corev1ac.ConfigMap("to-apply", metav1.NamespaceDefault)}

	cases := map[string]struct {
		trainJob         *trainer.TrainJob
		objsToApply      []apiruntime.ApplyConfiguration
		objsToDelete     []client.Object
		seedObjs         []client.Object
		newObjectsErr    error
		deleteObjectsErr error
		applyErr         error
		clientDeleteErr  error

		wantNewObjsCalled bool
		wantApplyCalled   bool
		wantDeleteCalled  bool
		wantDeleted       bool
		wantErr           error
	}{
		"running TrainJob applies new objects": {
			trainJob:          runningTrainJob,
			objsToApply:       toApplyObjs,
			wantNewObjsCalled: true,
			wantApplyCalled:   true,
		},
		"suspended TrainJob is not finished, so it still materializes new objects alongside cleanup": {
			trainJob:          suspendedTrainJob,
			objsToApply:       toApplyObjs,
			objsToDelete:      []client.Object{toDeleteObj},
			seedObjs:          []client.Object{toDeleteObj},
			wantNewObjsCalled: true,
			wantApplyCalled:   true,
			wantDeleteCalled:  true,
			wantDeleted:       true,
		},
		"failed TrainJob skips materializing new objects but still cleans up": {
			trainJob:         failedTrainJob,
			objsToApply:      toApplyObjs,
			objsToDelete:     []client.Object{toDeleteObj},
			seedObjs:         []client.Object{toDeleteObj},
			wantDeleteCalled: true,
			wantDeleted:      true,
		},
		"cleanup tolerates an object that is already gone": {
			trainJob: failedTrainJob,
			objsToDelete: []client.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "already-gone", Namespace: metav1.NamespaceDefault}},
			},
			wantDeleteCalled: true,
		},
		"NewObjects error does not block cleanup, and is still returned": {
			trainJob:          runningTrainJob,
			newObjectsErr:     errNewObjects,
			objsToDelete:      []client.Object{toDeleteObj},
			seedObjs:          []client.Object{toDeleteObj},
			wantNewObjsCalled: true,
			wantDeleteCalled:  true,
			wantDeleted:       true,
			wantErr:           errNewObjects,
		},
		"Apply error does not block cleanup, and is still returned": {
			trainJob:          runningTrainJob,
			objsToApply:       toApplyObjs,
			applyErr:          errApply,
			objsToDelete:      []client.Object{toDeleteObj},
			seedObjs:          []client.Object{toDeleteObj},
			wantNewObjsCalled: true,
			wantApplyCalled:   true,
			wantDeleteCalled:  true,
			wantDeleted:       true,
			wantErr:           errApply,
		},
		"DeleteObjects error is propagated": {
			trainJob:          runningTrainJob,
			deleteObjectsErr:  errDeleteObjects,
			wantNewObjsCalled: true,
			wantErr:           errDeleteObjects,
		},
		"client.Delete error is propagated": {
			trainJob:         failedTrainJob,
			objsToDelete:     []client.Object{toDeleteObj},
			seedObjs:         []client.Object{toDeleteObj},
			clientDeleteErr:  errClientDelete,
			wantDeleteCalled: true,
			wantErr:          errClientDelete,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			var applyCalled, deleteCalled bool
			cli := utiltesting.NewClientBuilder().
				WithObjects(tc.seedObjs...).
				WithInterceptorFuncs(interceptor.Funcs{
					Apply: func(_ context.Context, _ client.WithWatch, _ apiruntime.ApplyConfiguration, _ ...client.ApplyOption) error {
						applyCalled = true
						return tc.applyErr
					},
					Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						deleteCalled = true
						if tc.clientDeleteErr != nil {
							return tc.clientDeleteErr
						}
						return c.Delete(ctx, obj, opts...)
					},
				}).
				Build()
			fr := &fakeRuntime{
				objsToApply:      tc.objsToApply,
				objsToDelete:     tc.objsToDelete,
				newObjectsErr:    tc.newObjectsErr,
				deleteObjectsErr: tc.deleteObjectsErr,
			}
			r := &TrainJobReconciler{client: cli}

			err := r.reconcileObjects(ctx, fr, tc.trainJob)
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want, +got): %s", diff)
			}

			// DeleteObjects must run on every call, regardless of the TrainJob's status.
			if !fr.deleteObjectsCalled {
				t.Error("Expected DeleteObjects to be called")
			}
			if fr.newObjectsCalled != tc.wantNewObjsCalled {
				t.Errorf("NewObjects called = %v, want %v", fr.newObjectsCalled, tc.wantNewObjsCalled)
			}
			if applyCalled != tc.wantApplyCalled {
				t.Errorf("client.Apply called = %v, want %v", applyCalled, tc.wantApplyCalled)
			}
			if deleteCalled != tc.wantDeleteCalled {
				t.Errorf("client.Delete called = %v, want %v", deleteCalled, tc.wantDeleteCalled)
			}
			for _, obj := range tc.objsToDelete {
				getErr := cli.Get(ctx, client.ObjectKeyFromObject(obj), &corev1.ConfigMap{})
				if tc.wantDeleted && !apierrors.IsNotFound(getErr) {
					t.Errorf("Expected %s to be deleted, got error: %v", obj.GetName(), getErr)
				}
			}
		})
	}
}
