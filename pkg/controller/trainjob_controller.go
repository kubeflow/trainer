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
	"iter"
	"slices"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
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
	"github.com/kubeflow/trainer/v2/pkg/apply"
	applyconfig "github.com/kubeflow/trainer/v2/pkg/client/applyconfiguration/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/util/trainjob"
)

type TrainJobWatcher interface {
	NotifyTrainJobUpdate(oldJob, newJob *trainer.TrainJob)
}

type TrainJobReconciler struct {
	log      logr.Logger
	client   client.Client
	recorder events.EventRecorder
	runtimes map[string]jobruntimes.Runtime
	watchers iter.Seq[TrainJobWatcher]
}

type TrainJobReconcilerOptions struct {
	Watchers iter.Seq[TrainJobWatcher]
}

type TrainJobReconcilerOption func(*TrainJobReconcilerOptions)

func WithWatchers(watchers ...TrainJobWatcher) TrainJobReconcilerOption {
	return func(o *TrainJobReconcilerOptions) {
		o.Watchers = slices.Values(watchers)
	}
}

var _ reconcile.Reconciler = (*TrainJobReconciler)(nil)
var _ predicate.TypedPredicate[*trainer.TrainJob] = (*TrainJobReconciler)(nil)

func NewTrainJobReconciler(client client.Client, recorder events.EventRecorder, runtimes map[string]jobruntimes.Runtime, opts ...TrainJobReconcilerOption) *TrainJobReconciler {
	options := &TrainJobReconcilerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return &TrainJobReconciler{
		log:      ctrl.Log.WithName("trainjob-controller"),
		client:   client,
		recorder: recorder,
		runtimes: runtimes,
		watchers: options.Watchers,
	}
}

// +kubebuilder:rbac:groups="",resources=events,verbs=create;watch;update;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;watch;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs,verbs=get;list;watch;update;patch
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

	var err error
	conditions := currentConditions(&trainJob)

	runtimeRefGK := jobruntimes.RuntimeRefToRuntimeRegistryKey(trainJob.Spec.RuntimeRef)
	runtime, ok := r.runtimes[runtimeRefGK]
	if !ok {
		err = fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
		upsertCondition(&trainJob, &conditions, trainer.TrainJobFailed, metav1.ConditionTrue,
			trainer.TrainJobRuntimeNotSupportedReason, fmt.Sprintf("unsupported runtime: %s", runtimeRefGK),
		)
	} else if !trainjob.IsTrainJobFinished(&trainJob) {
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
	}

	suspendedCondition(&trainJob, &conditions)

	deadlineResult, deadlineErr := r.reconcileDeadline(ctx, &trainJob, &conditions)
	if deadlineErr != nil {
		err = errors.Join(err, deadlineErr)
	}

	statusApply, statusErr := trainJobStatus(ctx, runtime, &trainJob, conditions)
	if statusErr != nil {
		err = errors.Join(err, statusErr)
	}

	trainJobApply := applyconfig.TrainJob(trainJob.Name, trainJob.Namespace).WithStatus(statusApply)
	if applyErr := r.client.Status().Apply(ctx, trainJobApply, client.ForceOwnership, client.FieldOwner("trainjob_controller")); applyErr != nil {
		err = errors.Join(err, applyErr)
	}

	if deadlineResult.RequeueAfter > 0 {
		return deadlineResult, err
	}
	return ctrl.Result{}, err
}

func (r *TrainJobReconciler) reconcileObjects(ctx context.Context, runtime jobruntimes.Runtime, trainJob *trainer.TrainJob) error {
	objects, err := runtime.NewObjects(ctx, trainJob)
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := r.client.Apply(ctx, object, client.FieldOwner("trainer"), client.ForceOwnership); err != nil {
			return err
		}
	}
	return nil
}

func (r *TrainJobReconciler) reconcileDeadline(ctx context.Context, trainJob *trainer.TrainJob, conditions *[]metav1ac.ConditionApplyConfiguration) (ctrl.Result, error) {
	if trainJob.Spec.ActiveDeadlineSeconds == 0 || trainjob.IsTrainJobFinished(trainJob) || ptr.Deref(trainJob.Spec.Suspend, false) {
		return ctrl.Result{}, nil
	}
	startTime := trainJob.CreationTimestamp.Time
	suspendedCond := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobSuspended)
	if suspendedCond != nil && suspendedCond.Status == metav1.ConditionFalse {
		startTime = suspendedCond.LastTransitionTime.Time
	}
	if startTime.IsZero() {
		return ctrl.Result{}, nil
	}
	deadline := startTime.Add(time.Duration(trainJob.Spec.ActiveDeadlineSeconds) * time.Second)
	now := time.Now()
	if now.After(deadline) {
		ctrl.LoggerFrom(ctx).V(2).Info("TrainJob deadline exceeded, marking as failed",
			"activeDeadlineSeconds", trainJob.Spec.ActiveDeadlineSeconds,
			"startTime", startTime,
			"deadline", deadline)
		upsertCondition(trainJob, conditions, trainer.TrainJobFailed, metav1.ConditionTrue,
			trainer.TrainJobDeadlineExceededReason, constants.TrainJobDeadlineExceededMessage)
		jobSet := &jobsetv1alpha2.JobSet{
			ObjectMeta: metav1.ObjectMeta{Name: trainJob.Name, Namespace: trainJob.Namespace},
		}
		if err := client.IgnoreNotFound(r.client.Delete(ctx, jobSet)); err != nil {
			ctrl.LoggerFrom(ctx).V(2).Info("Failed to delete JobSet after deadline exceeded", "error", err)
		}
		return ctrl.Result{}, nil
	}
	requeueAfter := time.Until(deadline)
	if requeueAfter <= 0 {
		requeueAfter = 1 * time.Second
	}
	ctrl.LoggerFrom(ctx).V(2).Info("Scheduling deadline check",
		"activeDeadlineSeconds", trainJob.Spec.ActiveDeadlineSeconds,
		"requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *TrainJobReconciler) Create(e event.TypedCreateEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.Object)).Info("TrainJob create event")
	defer r.notifyWatchers(nil, e.Object)
	return true
}

func (r *TrainJobReconciler) Delete(e event.TypedDeleteEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.Object)).Info("TrainJob delete event")
	defer r.notifyWatchers(e.Object, nil)
	return true
}

func (r *TrainJobReconciler) Update(e event.TypedUpdateEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.ObjectNew)).Info("TrainJob update event")
	defer r.notifyWatchers(e.ObjectOld, e.ObjectNew)
	return true
}

func (r *TrainJobReconciler) Generic(e event.TypedGenericEvent[*trainer.TrainJob]) bool {
	r.log.WithValues("trainJob", klog.KObj(e.Object)).Info("TrainJob generic event")
	return true
}

func (r *TrainJobReconciler) notifyWatchers(oldJob, newJob *trainer.TrainJob) {
	for w := range r.watchers {
		w.NotifyTrainJobUpdate(oldJob, newJob)
	}
}

// suspendedCondition appends the Suspended condition apply configuration if the
// TrainJob has been suspended at any point.
func suspendedCondition(trainJob *trainer.TrainJob, conditions *[]metav1ac.ConditionApplyConfiguration) {
	switch {
	case ptr.Deref(trainJob.Spec.Suspend, false):
		upsertCondition(trainJob, conditions, trainer.TrainJobSuspended, metav1.ConditionTrue,
			trainer.TrainJobSuspendedReason, constants.TrainJobSuspendedMessage)
	case meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobSuspended):
		upsertCondition(trainJob, conditions, trainer.TrainJobSuspended, metav1.ConditionFalse,
			trainer.TrainJobResumedReason, constants.TrainJobResumedMessage)
	default:
		if existing := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobSuspended); existing != nil {
			upsertCondition(trainJob, conditions, trainer.TrainJobSuspended, existing.Status,
				existing.Reason, existing.Message)
		}
	}
}

// currentConditions initializes the conditions slice with existing conditions
func currentConditions(trainJob *trainer.TrainJob) []metav1ac.ConditionApplyConfiguration {
	var conditions []metav1ac.ConditionApplyConfiguration
	for _, c := range trainJob.Status.Conditions {
		// This preserves terminal conditions like Complete across reconciles where the runtime
		// plugin may short-circuit (e.g., JobSet deleted after completion).
		if c.Type == trainer.TrainJobSuspended {
			continue
		}
		if c.Type == trainer.TrainJobFailed && c.Reason != trainer.TrainJobDeadlineExceededReason {
			continue
		}
		conditions = append(conditions, *metav1ac.Condition().
			WithType(c.Type).
			WithStatus(c.Status).
			WithReason(c.Reason).
			WithMessage(c.Message).
			WithLastTransitionTime(c.LastTransitionTime))
	}
	return conditions
}

// upsertCondition adds or replaces a condition in the conditions slice, preserving
// LastTransitionTime from the existing TrainJob status when the status has not changed.
func upsertCondition(trainJob *trainer.TrainJob, conditions *[]metav1ac.ConditionApplyConfiguration, condType string, status metav1.ConditionStatus, reason, message string) {
	ac := metav1ac.Condition().
		WithType(condType).
		WithStatus(status).
		WithReason(reason).
		WithMessage(message)
	existing := meta.FindStatusCondition(trainJob.Status.Conditions, condType)
	if existing != nil && existing.Status == status {
		ac.WithLastTransitionTime(existing.LastTransitionTime)
	} else {
		ac.WithLastTransitionTime(metav1.Now())
	}
	apply.UpsertConditions(conditions, *ac)
}

// trainJobStatus builds the complete status apply configuration by merging the runtime status
// with the conditions collected during reconciliation.
func trainJobStatus(ctx context.Context, runtime jobruntimes.Runtime, trainJob *trainer.TrainJob, conditions []metav1ac.ConditionApplyConfiguration) (*applyconfig.TrainJobStatusApplyConfiguration, error) {
	var statusApply *applyconfig.TrainJobStatusApplyConfiguration
	var err error
	if runtime != nil {
		statusApply, err = runtime.TrainJobStatus(ctx, trainJob)
	}
	if statusApply == nil {
		statusApply = applyconfig.TrainJobStatus()
	}

	// Merge conditions into the runtime status, overriding runtime conditions of the same type.
	apply.UpsertConditions(&statusApply.Conditions, conditions...)

	return statusApply, err
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
