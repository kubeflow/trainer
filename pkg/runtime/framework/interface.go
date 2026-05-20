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

package framework

import (
	"context"

	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

type Plugin interface {
	Name() string
}

type CustomValidationPlugin interface {
	Plugin
	Validate(ctx context.Context, info *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList)
}

type WatchExtensionPlugin interface {
	Plugin
	ReconcilerBuilders() []runtime.ReconcilerBuilder
}

type EnforcePodGroupPolicyPlugin interface {
	Plugin
	EnforcePodGroupPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error
}

type EnforceMLPolicyPlugin interface {
	Plugin
	EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error
}

// EnforceRuntimeInfoPlugin is a generic plugin interface for plugins that
// update runtime.Info independently of any specific policy field on
// TrainingRuntime (for example, .spec.mlPolicy or .spec.podGroupPolicy).
// For policy-driven plugins that activate based on .mlPolicy or
// .podGroupPolicy, use EnforceMLPolicyPlugin or
// EnforcePodGroupPolicyPlugin respectively.
// This interface runs after EnforceMLPolicyPlugin and
// EnforcePodGroupPolicyPlugin, so implementations may rely on runtime.Info
// fields populated by earlier phases.
type EnforceRuntimeInfoPlugin interface {
	Plugin
	EnforceRuntimeInfo(info *runtime.Info, trainJob *trainer.TrainJob) error
}

type ComponentBuilderPlugin interface {
	Plugin
	Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error)
}

type TrainJobStatusPlugin interface {
	Plugin
	Status(ctx context.Context, trainJob *trainer.TrainJob) (*trainer.TrainJobStatus, error)
}
