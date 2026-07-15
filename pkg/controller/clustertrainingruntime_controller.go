/*
Copyright 2025 The Kubeflow Authors.

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

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
)

type ClusterTrainingRuntimeReconciler struct {
	log      logr.Logger
	client   client.Client
	recorder events.EventRecorder
}

var _ reconcile.Reconciler = (*ClusterTrainingRuntimeReconciler)(nil)

func NewClusterTrainingRuntimeReconciler(cli client.Client, recorder events.EventRecorder) *ClusterTrainingRuntimeReconciler {
	return &ClusterTrainingRuntimeReconciler{
		log:      ctrl.Log.WithName("clustertrainingruntime-controller"),
		client:   cli,
		recorder: recorder,
	}
}

// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=clustertrainingruntimes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=clustertrainingruntimes/finalizers,verbs=get;update;patch

func (r *ClusterTrainingRuntimeReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var clRuntime trainer.ClusterTrainingRuntime
	if err := r.client.Get(ctx, request.NamespacedName, &clRuntime); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log := r.log.WithValues("clusterTrainingRuntime", klog.KObj(&clRuntime))
	ctrl.LoggerInto(ctx, log)
	log.V(2).Info("Reconciling ClusterTrainingRuntime")

	prevClRuntime := clRuntime.DeepCopy()

	if ctrlutil.ContainsFinalizer(&clRuntime, constants.ResourceInUseFinalizer) {
		ctrlutil.RemoveFinalizer(&clRuntime, constants.ResourceInUseFinalizer)
		return ctrl.Result{}, r.client.Patch(ctx, &clRuntime, client.MergeFrom(prevClRuntime))
	}

	return ctrl.Result{}, nil
}

func (r *ClusterTrainingRuntimeReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return builder.TypedControllerManagedBy[reconcile.Request](mgr).
		Named("clustertrainingruntime_controller").
		WithOptions(options).
		WatchesRawSource(source.TypedKind(
			mgr.GetCache(),
			&trainer.ClusterTrainingRuntime{},
			&handler.TypedEnqueueRequestForObject[*trainer.ClusterTrainingRuntime]{},
		)).
		Complete(r)
}
