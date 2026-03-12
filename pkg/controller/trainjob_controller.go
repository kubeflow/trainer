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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/features"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/util/preemption"
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

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
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
	// Keep track of the origin TrainJob status
	prevTrainJob := trainJob.DeepCopy()

	// Let's clear the failed condition that could have been set previously.
	// An external change to the TrainJob spec may transition it out of the Failed state.
	removeFailedCondition(&trainJob)

	runtimeRefGK := jobruntimes.RuntimeRefToRuntimeRegistryKey(trainJob.Spec.RuntimeRef)
	runtime, ok := r.runtimes[runtimeRefGK]
	if !ok {
		err = fmt.Errorf("unsupported runtime: %s", runtimeRefGK)
		setFailedCondition(&trainJob, fmt.Sprintf("unsupported runtime: %s", runtimeRefGK), trainer.TrainJobRuntimeNotSupportedReason)
	} else if !trainjob.IsTrainJobFinished(&trainJob) {
		// Handle preempted pods: when the PreemptionRestart feature gate is enabled,
		// detect pods that have been preempted by the scheduler and delete them so
		// they can be recreated by the JobSet controller.
		if features.Enabled(features.PreemptionRestart) {
			if preemptErr := r.handlePreemptedPods(ctx, &trainJob); preemptErr != nil {
				log.Error(preemptErr, "Failed to handle preempted pods")
				err = errors.Join(err, preemptErr)
			}
		}

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

	setSuspendedCondition(&trainJob)

	if statusErr := setTrainJobStatus(ctx, runtime, &trainJob); statusErr != nil {
		err = errors.Join(err, statusErr)
	}

	if !equality.Semantic.DeepEqual(&trainJob.Status, prevTrainJob.Status) {
		// TODO(astefanutti): Consider using SSA once controller-runtime client has SSA support
		// for sub-resources. See: https://github.com/kubernetes-sigs/controller-runtime/issues/3183
		return ctrl.Result{}, errors.Join(err, r.client.Status().Patch(ctx, &trainJob, client.MergeFrom(prevTrainJob)))
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

func removeFailedCondition(trainJob *trainer.TrainJob) {
	meta.RemoveStatusCondition(&trainJob.Status.Conditions, trainer.TrainJobFailed)
}

func setTrainJobStatus(ctx context.Context, runtime jobruntimes.Runtime, trainJob *trainer.TrainJob) error {
	status, err := runtime.TrainJobStatus(ctx, trainJob)
	if err != nil {
		return err
	}
	if status != nil {
		trainJob.Status = *status
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

// handlePreemptedPods lists all pods owned by the TrainJob and deletes any that have been
// preempted by the scheduler (indicated by DisruptionTarget condition with reason
// PreemptionByScheduler). This allows the JobSet controller to recreate the pods,
// effectively restarting the preempted replicas instead of marking the job as failed.
//
// The max preemption restart limit is enforced via the PreemptionRestartCountAnnotation
// on the TrainJob. If the limit is exceeded, preempted pods are not deleted and the
// job is allowed to fail naturally.
func (r *TrainJobReconciler) handlePreemptedPods(ctx context.Context, trainJob *trainer.TrainJob) error {
	log := ctrl.LoggerFrom(ctx)

	// List all pods owned by this TrainJob.
	var podList corev1.PodList
	if err := r.client.List(ctx, &podList,
		client.InNamespace(trainJob.Namespace),
		client.MatchingLabels{
			constants.LabelTrainJobName: trainJob.Name,
		},
	); err != nil {
		return fmt.Errorf("listing pods for TrainJob: %w", err)
	}

	preemptedPods := preemption.FilterPreemptedPods(podList.Items)
	if len(preemptedPods) == 0 {
		return nil
	}

	log.V(1).Info("Detected preempted pods", "count", len(preemptedPods))

	// Check the preemption restart count against the max limit.
	currentRestartCount := getTrainJobPreemptionRestartCount(trainJob)
	maxRestarts := int32(preemption.DefaultMaxPreemptionRestarts)
	if maxRestarts > 0 && currentRestartCount >= maxRestarts {
		log.Info("TrainJob has exceeded max preemption restarts, allowing failure",
			"currentRestarts", currentRestartCount, "maxRestarts", maxRestarts)
		r.recorder.Eventf(trainJob, nil, corev1.EventTypeWarning, "PreemptionRestartLimitExceeded", "Reconciling",
			"TrainJob %s/%s exceeded max preemption restarts (%d), allowing job to fail",
			trainJob.Namespace, trainJob.Name, maxRestarts)
		return nil
	}

	// Delete preempted pods so they can be recreated by the JobSet controller.
	var errs []error
	for i := range preemptedPods {
		pod := &preemptedPods[i]
		log.Info("Deleting preempted pod for recreation",
			"pod", klog.KObj(pod), "restartCount", currentRestartCount+1)
		if err := r.client.Delete(ctx, pod); client.IgnoreNotFound(err) != nil {
			errs = append(errs, fmt.Errorf("deleting preempted pod %s: %w", pod.Name, err))
			continue
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	// Update the preemption restart count annotation on the TrainJob.
	if err := incrementTrainJobPreemptionRestartCount(ctx, r.client, trainJob); err != nil {
		return fmt.Errorf("updating preemption restart count: %w", err)
	}

	r.recorder.Eventf(trainJob, nil, corev1.EventTypeWarning, "PreemptedPodsDeleted", "Reconciling",
		"Deleted %d preempted pod(s) for TrainJob %s/%s (restart %d/%d)",
		len(preemptedPods), trainJob.Namespace, trainJob.Name, currentRestartCount+1, maxRestarts)

	return nil
}

// getTrainJobPreemptionRestartCount returns the current preemption restart count
// from the TrainJob's annotations.
func getTrainJobPreemptionRestartCount(trainJob *trainer.TrainJob) int32 {
	if trainJob.Annotations == nil {
		return 0
	}
	val, ok := trainJob.Annotations[preemption.PreemptionRestartCountAnnotation]
	if !ok {
		return 0
	}
	count, err := fmt.Sscanf(val, "%d")
	if err != nil || count == 0 {
		return 0
	}
	var result int32
	fmt.Sscanf(val, "%d", &result)
	return result
}

// incrementTrainJobPreemptionRestartCount increments the preemption restart count
// annotation on the TrainJob.
func incrementTrainJobPreemptionRestartCount(ctx context.Context, c client.Client, trainJob *trainer.TrainJob) error {
	patch := client.MergeFrom(trainJob.DeepCopy())
	currentCount := getTrainJobPreemptionRestartCount(trainJob)
	if trainJob.Annotations == nil {
		trainJob.Annotations = make(map[string]string)
	}
	trainJob.Annotations[preemption.PreemptionRestartCountAnnotation] = fmt.Sprintf("%d", currentCount+1)
	return c.Patch(ctx, trainJob, patch)
}
