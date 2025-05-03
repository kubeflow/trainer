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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	jobruntimes "github.com/kubeflow/trainer/pkg/runtime"
)

type TrainJobWatcher interface {
	NotifyTrainJobUpdate(oldJob, newJob *trainer.TrainJob)
}

type TrainJobReconciler struct {
	log      logr.Logger
	client   client.Client
	recorder record.EventRecorder
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

func NewTrainJobReconciler(client client.Client, recorder record.EventRecorder, runtimes map[string]jobruntimes.Runtime, opts ...TrainJobReconcilerOption) *TrainJobReconciler {
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

// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs/finalizers,verbs=get;update;patch

func (r *TrainJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var trainJob trainer.TrainJob
	if err := r.client.Get(ctx, req.NamespacedName, &trainJob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log := ctrl.LoggerFrom(ctx).WithValues("trainJob", klog.KObj(&trainJob))
	ctx = ctrl.LoggerInto(ctx, log)
	log.V(2).Info("Reconciling TrainJob")
	if isTrainJobFinished(&trainJob) {
		log.V(5).Info("TrainJob has already been finished")
		return ctrl.Result{}, nil
	}

	var err error
	// Keep track of the origin TrainJob status
	originStatus := trainJob.Status.DeepCopy()

	// Let's clear the failed condition that could have been set previously.
	// An external change to the TrainJob spec may transition it out of the Failed state.
	removeFailedCondition(&trainJob)

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
			r.recorder.Event(&trainJob, corev1.EventTypeWarning, "TrainJobResourcesCreationFailed", message)
		}
	}

	setSuspendedCondition(&trainJob)
	if terminalCondErr := setTerminalCondition(ctx, runtime, &trainJob); terminalCondErr != nil {
		err = errors.Join(err, terminalCondErr)
	}

	if !equality.Semantic.DeepEqual(&trainJob.Status, originStatus) {
		return ctrl.Result{}, errors.Join(err, r.client.Status().Update(ctx, &trainJob))
	}
	return ctrl.Result{}, err
}

func (r *TrainJobReconciler) reconcileObjects(ctx context.Context, runtime jobruntimes.Runtime, trainJob *trainer.TrainJob) error {
	log := ctrl.LoggerFrom(ctx)

	objects, err := runtime.NewObjects(ctx, trainJob)
	if err != nil {
		return err
	}
	for _, object := range objects {
		// TODO (astefanutti): Remove conversion to unstructured when the runtime.ApplyConfiguration
		//  interface becomes available and first-class SSA method is added to the controller-runtime
		// client. See https://github.com/kubernetes/kubernetes/pull/129313
		var obj client.Object
		if o, ok := object.(client.Object); ok {
			return fmt.Errorf("unsupported type client.Object for component: %v", o)
		}

		u, err := apiruntime.DefaultUnstructuredConverter.ToUnstructured(object)
		if err != nil {
			return err
		}
		obj = &unstructured.Unstructured{Object: u}

		if err := r.client.Patch(ctx, obj, client.Apply, client.FieldOwner("trainer"), client.ForceOwnership); err != nil {
			return err
		}

		var gvk schema.GroupVersionKind
		if gvk, err = apiutil.GVKForObject(obj.DeepCopyObject(), r.client.Scheme()); err != nil {
			return err
		}
		logKeysAndValues := []any{
			"groupVersionKind", gvk.String(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
		}

		log.V(5).Info("Succeeded to update object", logKeysAndValues...)
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

func setTerminalCondition(ctx context.Context, runtime jobruntimes.Runtime, trainJob *trainer.TrainJob) error {
	terminalCond, err := runtime.TerminalCondition(ctx, trainJob)
	if err != nil {
		return err
	}
	if terminalCond != nil {
		meta.SetStatusCondition(&trainJob.Status.Conditions, *terminalCond)
	}
	return nil
}

func isTrainJobFinished(trainJob *trainer.TrainJob) bool {
	return meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobComplete) ||
		meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobFailed)
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
