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

package jax

import (
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

type Jax struct{}

var _ framework.EnforceMLPolicyPlugin = (*Jax)(nil)
var _ framework.CustomValidationPlugin = (*Jax)(nil)

const Name = "Jax"

func New(context.Context, client.Client, client.FieldIndexer) (framework.Plugin, error) {
	return &Jax{}, nil
}

func (t *Jax) Name() string {
	return Name
}

func (t *Jax) Validate(_ context.Context, runtimeInfo *runtime.Info, _, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	var allErrs field.ErrorList
	if runtimeInfo == nil || runtimeInfo.RuntimePolicy.MLPolicySource == nil || runtimeInfo.RuntimePolicy.MLPolicySource.Torch == nil || newObj.Spec.Trainer == nil || newObj.Spec.Trainer.NumProcPerNode == nil {
		return nil, allErrs
	}

	specPath := field.NewPath("spec")
}