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

package controllerv2

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	runtime "github.com/kubeflow/training-operator/pkg/runtime.v2"
)

func SetupControllers(mgr ctrl.Manager, runtimes map[string]runtime.Runtime, options controller.Options) (string, error) {
	if err := NewTrainJobReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("training-operator-trainjob-controller"),
		runtimes,
	).SetupWithManager(mgr, options); err != nil {
		return "TrainJob", err
	}
	return "", nil
}
