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

package flux

import (
	"strings"

	trainerapi "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

// getOriginalCommand derives the original Kubeflow command we need to wrap / handoff to Flux
func getOriginalCommand(trainJob *trainerapi.TrainJob) string {
	var command []string
	var args []string

	// Check high-level Trainer fields?
	if trainJob.Spec.Trainer != nil {
		command = trainJob.Spec.Trainer.Command
		args = trainJob.Spec.Trainer.Args
	}

	// Combine into a single string for the shell script
	fullCommand := strings.Join(append(command, args...), " ")
	return strings.TrimSpace(fullCommand)
}
