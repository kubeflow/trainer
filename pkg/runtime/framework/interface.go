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

// EnforceInfrastructurePlugin updates runtime.Info with infrastructure concerns
// that every TrainJob needs regardless of how it was configured: things the
// platform must wire up, rather than things the user asked for.
//
// Use this interface when the plugin is not driven by a policy field. The
// sibling interfaces are each scoped to one field of the TrainingRuntime spec
// and activate only when a user sets it:
//
//   - EnforceMLPolicyPlugin        for .spec.mlPolicy (e.g. Torch, JAX)
//   - EnforcePodGroupPolicyPlugin  for .spec.podGroupPolicy (e.g. Coscheduling)
//
// If you cannot name the spec field that switches your plugin on, it belongs
// here. Current implementations are JobSet, which derives Pod-to-Pod network
// endpoints from the PodSets, and TrainJobStatus, which injects status-server
// configuration; neither corresponds to anything the user declares.
//
// Ordering is load-bearing: this phase runs after EnforceMLPolicyPlugin and
// EnforcePodGroupPolicyPlugin, so implementations may rely on runtime.Info
// already being shaped by those plugins. JobSet, for example, reads
// PodSets[].Count after the ML policy has set it.
type EnforceInfrastructurePlugin interface {
	Plugin
	EnforceInfrastructure(info *runtime.Info, trainJob *trainer.TrainJob) error
}

type ComponentBuilderPlugin interface {
	Plugin
	// SyncParallelCount propagates PodSets.Count into template-level Parallelism/Completions.
	SyncParallelCount(info *runtime.Info) error
	Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error)
}

type TrainJobStatusPlugin interface {
	Plugin
	Status(ctx context.Context, trainJob *trainer.TrainJob) (*trainer.TrainJobStatus, error)
}
