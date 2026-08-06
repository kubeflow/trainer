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

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOptimizationJobMultiObjective(t *testing.T) {
	obj := &OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-multi-objective-job",
			Namespace: "default",
		},
		Spec: OptimizationJobSpec{
			Objectives: []Objective{
				{
					Metric:    "loss",
					Direction: ObjectiveDirectionMinimize,
				},
				{
					Metric:    "accuracy",
					Direction: ObjectiveDirectionMaximize,
				},
			},
			SearchAlgorithm: &SearchAlgorithm{
				Random: &RandomAlgorithm{},
			},
			Parameters: []Parameter{
				{
					Name: "learning_rate",
					SearchSpace: &SearchSpace{
						Uniform: UniformSpace{
							Min:  "0.001",
							Max:  "0.1",
							Type: ParameterTypeFloat,
						},
					},
				},
			},
		},
	}

	if len(obj.Spec.Objectives) != 2 {
		t.Fatalf("expected 2 objectives for multi-objective optimization, got %d", len(obj.Spec.Objectives))
	}

	cloned := obj.DeepCopy()
	if len(cloned.Spec.Objectives) != 2 {
		t.Fatalf("expected cloned object to have 2 objectives, got %d", len(cloned.Spec.Objectives))
	}

	if cloned.Spec.Objectives[0].Metric != "loss" || cloned.Spec.Objectives[1].Metric != "accuracy" {
		t.Errorf("objective metrics mismatched after deepcopy")
	}
}
