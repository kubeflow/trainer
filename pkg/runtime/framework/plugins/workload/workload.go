/*
Copyright The Kubeflow Authors.

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

package workload

import (
	"context"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	"github.com/kubeflow/trainer/v2/pkg/util/trainjob"
)

const (
	Name = "Workload"

	resourceClaimsErrorMsg = "must not be set when the runtime sets spec.scheduling.replicatedJobs[].job.resourceClaims"
)

// Workload plugin integrates TrainJob with the Kubernetes Workload-Aware Scheduling APIs.
type Workload struct{}

var _ framework.CustomValidationPlugin = (*Workload)(nil)

func New(context.Context, client.Client, client.FieldIndexer, *configapi.Configuration) (framework.Plugin, error) {
	return &Workload{}, nil
}

func (w *Workload) Name() string {
	return Name
}

func (w *Workload) Validate(_ context.Context, info *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	if info == nil || newObj == nil || !hasJobResourceClaims(info.RuntimePolicy.Scheduling) {
		return nil, nil
	}
	var allErrs field.ErrorList
	if oldObj == nil {
		for _, path := range trainjob.ResourceClaimsPaths(newObj) {
			allErrs = append(allErrs, field.Forbidden(path, resourceClaimsErrorMsg))
		}
		return nil, allErrs
	}
	// On update, spec.trainer and the existing Pod resourceClaims of spec.runtimePatches are
	// immutable, so only Pod resourceClaims added to a runtimePatch need validation. The patched Pod
	// specs are identified by the patch manager and ReplicatedJob name, since list indices may change.
	type patchedPodSpec struct {
		manager  string
		rJobName string
	}
	oldClaims := sets.New[patchedPodSpec]()
	trainjob.ForEachRuntimePatchPodSpec(oldObj, func(_ *field.Path, manager, rJobName string, podSpec *trainer.PodSpecPatch) {
		if len(podSpec.ResourceClaims) > 0 {
			oldClaims.Insert(patchedPodSpec{manager: manager, rJobName: rJobName})
		}
	})
	trainjob.ForEachRuntimePatchPodSpec(newObj, func(podSpecPath *field.Path, manager, rJobName string, podSpec *trainer.PodSpecPatch) {
		if len(podSpec.ResourceClaims) > 0 && !oldClaims.Has(patchedPodSpec{manager: manager, rJobName: rJobName}) {
			allErrs = append(allErrs, field.Forbidden(podSpecPath.Child("resourceClaims"), resourceClaimsErrorMsg))
		}
	})
	return nil, allErrs
}

// hasJobResourceClaims reports whether any Job-level (level 3) scheduling entry sets resourceClaims.
func hasJobResourceClaims(scheduling *trainer.TrainingRuntimeScheduling) bool {
	if scheduling == nil {
		return false
	}
	for _, entry := range scheduling.ReplicatedJobs {
		if entry.Job != nil && len(entry.Job.ResourceClaims) > 0 {
			return true
		}
	}
	return false
}
