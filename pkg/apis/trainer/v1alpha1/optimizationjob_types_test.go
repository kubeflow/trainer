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

package v1alpha1

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestOptimizationJobSerialization(t *testing.T) {
	job := &OptimizationJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       OptimizationJobKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "random-tuning-test",
			Namespace: "default",
		},
		Spec: OptimizationJobSpec{
			Objectives: []Objective{
				{
					Metric:    ptr.To("val_loss"),
					Direction: ptr.To(ObjectiveDirectionMinimize),
				},
			},
			SearchAlgorithm: &SearchAlgorithm{
				Random: &RandomAlgorithm{
					Seed: ptr.To(int64(42)),
				},
			},
			Parameters: []Parameter{
				{
					Name: "learning_rate",
					SearchSpace: SearchSpace{
						LogUniform: &LogUniformSpace{
							Min:  "0.0001",
							Max:  "0.1",
							Type: ParameterTypeFloat,
						},
					},
				},
				{
					Name: "batch_size",
					SearchSpace: SearchSpace{
						Categorical: &CategoricalSpace{
							Choices: []string{"16", "32", "64"},
						},
					},
				},
			},
			NumTrials:      ptr.To(int32(20)),
			ParallelTrials: ptr.To(int32(4)),
			TrainJobTemplate: TrainJobTemplateSpec{
				Spec: TrainJobSpec{
					RuntimeRef: RuntimeRef{
						Name: "pytorch-distributed",
					},
				},
			},
		},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to marshal OptimizationJob: %v", err)
	}

	var unmarshaled OptimizationJob
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal OptimizationJob: %v", err)
	}

	if unmarshaled.Name != job.Name {
		t.Errorf("Expected name %s, got %s", job.Name, unmarshaled.Name)
	}

	if len(unmarshaled.Spec.Parameters) != 2 {
		t.Fatalf("Expected 2 parameters, got %d", len(unmarshaled.Spec.Parameters))
	}

	if unmarshaled.Spec.Parameters[0].Name != "learning_rate" {
		t.Errorf("Expected first parameter name 'learning_rate', got %s", unmarshaled.Spec.Parameters[0].Name)
	}

	if unmarshaled.Spec.Parameters[0].SearchSpace.LogUniform == nil {
		t.Error("Expected LogUniform search space to be non-nil")
	} else if unmarshaled.Spec.Parameters[0].SearchSpace.LogUniform.Min != "0.0001" {
		t.Errorf("Expected LogUniform min '0.0001', got %s", unmarshaled.Spec.Parameters[0].SearchSpace.LogUniform.Min)
	}
}

func TestOptimizationJobDeepCopy(t *testing.T) {
	job := &OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deepcopy-test",
			Namespace: "default",
		},
		Spec: OptimizationJobSpec{
			NumTrials: ptr.To(int32(10)),
			SearchAlgorithm: &SearchAlgorithm{
				Grid: &GridAlgorithm{},
			},
			Parameters: []Parameter{
				{
					Name: "epochs",
					SearchSpace: SearchSpace{
						Uniform: &UniformSpace{
							Min:  "1",
							Max:  "10",
							Type: ParameterTypeInt,
						},
					},
				},
			},
		},
		Status: OptimizationJobStatus{
			Result: &Result{
				TrainJobName: "trial-1",
				Parameters: []ParameterAssignment{
					{Name: "epochs", Value: "5"},
				},
			},
		},
	}

	copied := job.DeepCopy()
	if copied == nil {
		t.Fatal("Expected non-nil copied object")
	}

	if copied.Name != job.Name {
		t.Errorf("Expected name %s, got %s", job.Name, copied.Name)
	}

	// Modify copied object and ensure original is untouched
	copied.Name = "modified-name"
	*copied.Spec.NumTrials = 20

	if job.Name == "modified-name" {
		t.Error("Original object name was modified after DeepCopy")
	}
	if *job.Spec.NumTrials == 20 {
		t.Error("Original NumTrials was modified after DeepCopy")
	}
}
