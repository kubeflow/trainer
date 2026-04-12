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
	"reflect"
	"testing"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

func TestValidateUnsloth(t *testing.T) {
	trainJob := &trainer.TrainJob{
		Spec: trainer.TrainJobSpec{
			Trainer: &trainer.Trainer{
				Command: []string{"unsloth", "finetune", "--model", "llama3"},
			},
		},
	}
	info := &runtime.Info{}

	warnings, errs := validateUnsloth(info, trainJob)
	if len(warnings) != 0 {
		t.Errorf("validateUnsloth returned unexpected warnings: %v", warnings)
	}
	if len(errs) != 0 {
		t.Errorf("validateUnsloth returned unexpected errors: %v", errs)
	}
}

func TestBuildUnslothCommand(t *testing.T) {
	cmd := []string{"unsloth", "finetune", "--model", "llama3"}
	trainJob := &trainer.TrainJob{
		Spec: trainer.TrainJobSpec{
			Trainer: &trainer.Trainer{
				Command: cmd,
			},
		},
	}
	info := &runtime.Info{}

	result := buildUnslothCommand(info, trainJob)
	if !reflect.DeepEqual(result, cmd) {
		t.Errorf("buildUnslothCommand = %v, want %v", result, cmd)
	}
}
