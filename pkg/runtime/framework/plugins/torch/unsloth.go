/*
Copyright 2026 The Kubeflow Authors.

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

package torch

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

// validateUnsloth performs validation for TrainJobs using the Unsloth backend.
// Currently essentially a stub for the skeletal PR.
func validateUnsloth(_ *runtime.Info, _ *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	var allErrs field.ErrorList

	// TODO: Implement proper validation for supported models, GPU configurations, etc.
	// For now, this skeletal backend assumes the user's config is valid.

	return nil, allErrs
}

// buildUnslothCommand constructs the correct CLI arguments for the Unsloth trainer.
// Currently returns the user's provided command unmodified.
func buildUnslothCommand(_ *runtime.Info, trainJob *trainer.TrainJob) []string {
	// TODO: Parse the Trainer.args and construct the Unsloth FastLanguageModel compatible command.
	return trainJob.Spec.Trainer.Command
}
