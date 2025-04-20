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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
)

func SetupControllers(mgr ctrl.Manager, runtimes map[string]runtime.Runtime, options controller.Options) (string, error) {
	runtimeRec := NewTrainingRuntimeReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("trainer-trainingruntime-controller"),
	)
	if err := runtimeRec.SetupWithManager(mgr, options); err != nil {
		return trainer.TrainingRuntimeKind, err
	}
	clRuntimeRec := NewClusterTrainingRuntimeReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("trainer-clustertrainingruntime-controller"),
	)
	if err := clRuntimeRec.SetupWithManager(mgr, options); err != nil {
		return trainer.ClusterTrainingRuntimeKind, err
	}
	if err := NewTrainJobReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("trainer-trainjob-controller"),
		runtimes,
		WithWatchers(runtimeRec, clRuntimeRec),
	).SetupWithManager(mgr, options); err != nil {
		return trainer.TrainJobKind, err
	}
	return "", nil
}
