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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/util/trainjob"
)

type TrainJobReconciler struct {
	log      logr.Logger
	client   client.Client
	recorder events.EventRecorder
	runtimes map[string]jobruntimes.Runtime
	// clock is the time source for the activeDeadlineSeconds arithmetic. Tests
	// substitute a fake clock so the requeue delay can be asserted exactly.
	clock clock.PassiveClock
}

var _ reconcile.Reconciler = (*TrainJobReconciler)(nil)
var _ predicate.TypedPredicate[*trainer.TrainJob] = (*TrainJobReconciler)(nil)

func NewTrainJobReconciler(
	client client.Client,
	recorder events.EventRecorder,
	runtimes map[string]jobruntimes.Runtime,
) *TrainJobReconciler {
	return &TrainJobReconciler{
		log:      ctrl.Log.WithName("trainjob-controller"),
		client:   client,
		recorder: recorder,
		runtimes: runtimes,
		clock:    clock.RealClock{},
	}
}

// +kubebuilder:rbac:groups="",resources=events,verbs=create;watch;update;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;watch;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create;get;list;update

func (r *TrainJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var trainJob trainer.TrainJob
	if err := r.client.Get(ctx, req.NamespacedName, &trainJob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log := ctrl.LoggerFrom(ctx).WithValues("trainJob", klog.KObj(&trainJob))
	ctx = ctrl.LoggerInto(ctx, log)
	log.V(2).Info("Reconciling TrainJob")

	if trainjob.IsManagedByExternalController(&trainJob) {
		log.V(2).Info("Skipping TrainJob managed by a custom controller", "managedBy", ptr.Deref(trainJob.Spec.ManagedBy, ""))
		if trainjob.IsTrainJobFinished(&trainJob) {
			return r.reconcileTTL(ctx, &trainJob, trainJob.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	var err error
	var defaults *trainer.RunPolicy
	// Keep track of the origin TrainJob status
	prevTrainJob := trainJob.DeepCopy()

	runtimeRefGK := jobruntimes.RuntimeRefToRuntimeRegistryKey(trainJob.Spec.RuntimeRef)
	runtime, ok := r.runtimes[runtimeRefGK]
	if !ok {
		err = fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
		setFailedCondition(&trainJob, fmt.Sprintf("unsupported runtime: %s", runtimeRefGK), trainer.TrainJobRuntimeNotSupportedReason)
	} else {
		err = r.reconcileObjects(ctx, runtime, &trainJob)
		if err != nil {
			// TODO (astefanutti): the error should be surfaced in the TrainJob status to indicate
			//  the creation of the runtime resources failed and the TrainJob is backed off until
			//  the next retry attempt.
			// The event message is truncated to stay within the maximum length limit (1024 chars).
			message := fmt.Sprintf("TrainJob resources reconciliation failed: %.950v", err.Error())
			if len(err.Error()) > 950 {
				message = fmt.Sprintf("%s ...", message)
			}
			r.recorder.Eventf(&trainJob, nil, corev1.EventTypeWarning, "TrainJobResourcesCreationFailed", "Reconciling", message)
		}
		if statusErr := setTrainJobStatus(ctx, runtime, &trainJob); statusErr != nil {
			err = errors.Join(err, statusErr)
		}
		var policyErr error
		defaults, policyErr = runtime.RunPolicy(ctx, &trainJob)
		err = errors.Join(err, policyErr)
	}

	setSuspendedCondition(&trainJob)
	r.reconcileStartTime(&trainJob)

	policy := resolveRunPolicy(&trainJob, defaults)
	deadlineResult, deadlineErr := r.reconcileDeadline(ctx, &trainJob, ptr.Deref(policy.ActiveDeadlineSeconds, 0))
	err = errors.Join(err, deadlineErr)

	if !equality.Semantic.DeepEqual(trainJob.Status, prevTrainJob.Status) {
		// TODO(astefanutti): Consider using SSA once controller-runtime client has SSA support
		// for sub-resources. See: https://github.com/kubernetes-sigs/controller-runtime/issues/3183
		err = errors.Join(err, r.client.Status().Patch(ctx, &trainJob, client.MergeFrom(prevTrainJob)))
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	policy = resolveRunPolicy(&trainJob, defaults)
	ttlResult, ttlErr := r.reconcileTTL(ctx, &trainJob, policy.TTLSecondsAfterFinished)
	if ttlErr != nil || ttlResult.RequeueAfter > 0 {
		return ttlResult, ttlErr
	}

	if deadlineResult.RequeueAfter > 0 {
		return deadlineResult, err
	}
	return ctrl.Result{}, err
}

func (r *TrainJobReconciler) reconcileObjects(ctx context.Context, runtime jobruntimes.Runtime, trainJob *trainer.TrainJob) error {
	var applyErr error
	if !trainjob.IsTrainJobFinished(trainJob) {
		objects, err := runtime.NewObjects(ctx, trainJob)
		if err != nil {
			applyErr = err
		}
		for _, object := range objects {
			if err := r.client.Apply(ctx, object, client.FieldOwner("trainer"), client.ForceOwnership); err != nil {
				applyErr = err
				break
			}
		}
	}

	// Runs even for finished TrainJobs, so plugins can clean up objects of finished or
	// suspended TrainJobs (e.g. a gang-scheduling PodGroup, which would otherwise keep
	// reserving scheduler queue capacity indefinitely).
	toDelete, err := runtime.DeleteObjects(ctx, trainJob)
	if err != nil {
		return errors.Join(applyErr, fmt.Errorf("resolving objects to delete: %w", err))
	}
	for _, object := range toDelete {
		if err := client.IgnoreNotFound(r.client.Delete(ctx, object)); err != nil {
			return errors.Join(applyErr, fmt.Errorf("deleting %s: %w", klog.KObj(object), err))
		}
	}
	return applyErr
}

func resolveRunPolicy(trainJob *trainer.TrainJob, defaults *trainer.RunPolicy) trainer.RunPolicy {
	var policy trainer.RunPolicy
	if defaults != nil {
		policy = *defaults
	}
	if trainJob.Spec.ActiveDeadlineSeconds != 0 {
		policy.ActiveDeadlineSeconds = ptr.To(trainJob.Spec.ActiveDeadlineSeconds)
	}
	if trainJob.Spec.TTLSecondsAfterFinished != nil {
		policy.TTLSecondsAfterFinished = trainJob.Spec.TTLSecondsAfterFinished
	}
	return policy
}

func (r *TrainJobReconciler) reconcileStartTime(trainJob *trainer.TrainJob) {
	if trainjob.IsTrainJobFinished(trainJob) {
		return
	}
	if ptr.Deref(trainJob.Spec.Suspend, false) {
		trainJob.Status.StartTime = nil
		return
	}
	if trainJob.Status.StartTime != nil {
		return
	}
	startTime := trainJob.CreationTimestamp
	if cond := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobSuspended); cond != nil && cond.Status == metav1.ConditionFalse {
		startTime = cond.LastTransitionTime
	}
	if startTime.IsZero() {
		startTime = metav1.NewTime(r.clock.Now())
	}
	trainJob.Status.StartTime = &startTime
}

func (r *TrainJobReconciler) reconcileDeadline(ctx context.Context, trainJob *trainer.TrainJob, activeDeadlineSeconds int64) (ctrl.Result, error) {
	if condition := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed); condition != nil && condition.Status == metav1.ConditionTrue && condition.Reason == trainer.TrainJobDeadlineExceededReason {
		return ctrl.Result{}, r.deleteDeadlineJobSet(ctx, trainJob)
	}
	if activeDeadlineSeconds == 0 || trainjob.IsTrainJobFinished(trainJob) || ptr.Deref(trainJob.Spec.Suspend, false) {
		return ctrl.Result{}, nil
	}
	if trainJob.Status.StartTime == nil {
		return ctrl.Result{}, nil
	}
	startTime := trainJob.Status.StartTime.Time
	deadline := startTime.Add(time.Duration(activeDeadlineSeconds) * time.Second)
	now := r.clock.Now()
	if now.After(deadline) {
		ctrl.LoggerFrom(ctx).V(2).Info("TrainJob deadline exceeded, marking as failed",
			"activeDeadlineSeconds", activeDeadlineSeconds,
			"startTime", startTime,
			"deadline", deadline)
		setFailedCondition(trainJob, constants.TrainJobDeadlineExceededMessage, trainer.TrainJobDeadlineExceededReason)
		meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed).LastTransitionTime = metav1.NewTime(now)
		return ctrl.Result{}, r.deleteDeadlineJobSet(ctx, trainJob)
	}
	requeueAfter := deadline.Sub(now)
	if requeueAfter <= 0 {
		requeueAfter = 1 * time.Second
	}
	ctrl.LoggerFrom(ctx).V(2).Info("Scheduling deadline check",
		"activeDeadlineSeconds", activeDeadlineSeconds,
		"requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *TrainJobReconciler) deleteDeadlineJobSet(ctx context.Context, trainJob *trainer.TrainJob) error {
	jobSet := &jobsetv1alpha2.JobSet{}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(trainJob), jobSet); err != nil {
		return client.IgnoreNotFound(err)
	}
	if jobSet.DeletionTimestamp != nil || !metav1.IsControlledBy(jobSet, trainJob) {
		return nil
	}
	// The UID precondition stops a delete of a JobSet that replaced the cached one.
	return client.IgnoreNotFound(r.client.Delete(ctx, jobSet, client.Preconditions{UID: &jobSet.UID}))
}

func (r *TrainJobReconciler) reconcileTTL(ctx context.Context, trainJob *trainer.TrainJob, ttl *int32) (ctrl.Result, error) {
	if ttl == nil || trainJob.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}
	if owner := metav1.GetControllerOf(trainJob); owner != nil && owner.APIVersion == trainer.GroupVersion.String() && owner.Kind == trainer.OptimizationJobKind {
		return ctrl.Result{}, nil
	}
	for _, condition := range trainJob.Status.Conditions {
		if (condition.Type != trainer.TrainJobComplete && condition.Type != trainer.TrainJobFailed) || condition.Status != metav1.ConditionTrue {
			continue
		}
		if condition.LastTransitionTime.IsZero() {
			return ctrl.Result{}, fmt.Errorf("TrainJob %s/%s has no terminal transition time", trainJob.Namespace, trainJob.Name)
		}
		expiry := condition.LastTransitionTime.Add(time.Duration(*ttl) * time.Second)
		remaining := expiry.Sub(r.clock.Now())
		if remaining > 0 {
			ctrl.LoggerFrom(ctx).V(2).Info("Scheduling TTL cleanup", "ttlSecondsAfterFinished", *ttl, "requeueAfter", remaining)
			return ctrl.Result{RequeueAfter: remaining}, nil
		}
		err := r.client.Delete(ctx, trainJob,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
			client.Preconditions{UID: &trainJob.UID, ResourceVersion: &trainJob.ResourceVersion})
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		if err == nil {
			ctrl.LoggerFrom(ctx).V(2).Info("Requested TrainJob deletion after TTL expired", "ttlSecondsAfterFinished", *ttl)
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TrainJobReconciler) Create(e event.TypedCreateEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.Object)).Info("TrainJob create event")
	return true
}

func (r *TrainJobReconciler) Delete(e event.TypedDeleteEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.Object)).Info("TrainJob delete event")
	return true
}

func (r *TrainJobReconciler) Update(e event.TypedUpdateEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.ObjectNew)).Info("TrainJob update event")
	return true
}

func (r *TrainJobReconciler) Generic(e event.TypedGenericEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.Object)).Info("TrainJob generic event")
	return true
}

func setSuspendedCondition(trainJob *trainer.TrainJob) {
	var newCond metav1.Condition
	switch {
	case ptr.Deref(trainJob.Spec.Suspend, false):
		newCond = metav1.Condition{
			Type:    trainer.TrainJobSuspended,
			Status:  metav1.ConditionTrue,
			Message: constants.TrainJobSuspendedMessage,
			Reason:  trainer.TrainJobSuspendedReason,
		}
	case meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobSuspended):
		newCond = metav1.Condition{
			Type:    trainer.TrainJobSuspended,
			Status:  metav1.ConditionFalse,
			Message: constants.TrainJobResumedMessage,
			Reason:  trainer.TrainJobResumedReason,
		}
	default:
		return
	}
	meta.SetStatusCondition(&trainJob.Status.Conditions, newCond)
}

func setFailedCondition(trainJob *trainer.TrainJob, message, reason string) {
	newCond := metav1.Condition{
		Type:    trainer.TrainJobFailed,
		Status:  metav1.ConditionTrue,
		Message: message,
		Reason:  reason,
	}
	meta.SetStatusCondition(&trainJob.Status.Conditions, newCond)
}

func setTrainJobStatus(ctx context.Context, runtime jobruntimes.Runtime, trainJob *trainer.TrainJob) error {
	deadlineCond := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed)
	if deadlineCond != nil && deadlineCond.Reason != trainer.TrainJobDeadlineExceededReason {
		deadlineCond = nil
	}

	status, err := runtime.TrainJobStatus(ctx, trainJob)
	if err != nil {
		return err
	}
	if status != nil {
		trainJob.Status = *status
	}
	if deadlineCond != nil {
		meta.SetStatusCondition(&trainJob.Status.Conditions, *deadlineCond)
	}
	return nil
}

func (r *TrainJobReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	b := builder.TypedControllerManagedBy[reconcile.Request](mgr).
		Named("trainjob_controller").
		WithOptions(options).
		WatchesRawSource(source.TypedKind(
			mgr.GetCache(),
			&trainer.TrainJob{},
			&handler.TypedEnqueueRequestForObject[*trainer.TrainJob]{},
			r,
		))
	for _, runtime := range r.runtimes {
		for _, registrar := range runtime.EventHandlerRegistrars() {
			if registrar != nil {
				b = registrar(b, mgr.GetClient(), mgr.GetCache())
			}
		}
	}
	return b.Complete(r)
}
