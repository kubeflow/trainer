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

package webhooks

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestOptimizationJobDefault(t *testing.T) {
	cases := map[string]struct {
		inputObj *trainer.OptimizationJob
		wantObj  *trainer.OptimizationJob
	}{
		"Empty limits and algorithms get defaulted": {
			inputObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{},
			},
			wantObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					ParallelTrials: ptr.To(int32(1)), // Defaulted
					NumTrials:      ptr.To(int32(1)), // Defaulted
					SearchAlgorithm: &trainer.SearchAlgorithm{
						Random: &trainer.RandomAlgorithm{}, // Defaulted
					},
				},
			},
		},
		"Existing limits and algorithms are preserved": {
			inputObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{
						Grid: &trainer.GridAlgorithm{},
					},
					ParallelTrials: ptr.To(int32(5)),
					NumTrials:      ptr.To(int32(20)),
				},
			},
			wantObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{
						Grid: &trainer.GridAlgorithm{}, // Preserved
					},
					ParallelTrials: ptr.To(int32(5)),  // Preserved
					NumTrials:      ptr.To(int32(20)), // Preserved
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			defaulter := &OptimizationJobDefaulter{}

			if err := defaulter.Default(ctx, tc.inputObj); err != nil {
				t.Fatalf("Default returned unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.wantObj, tc.inputObj); len(diff) != 0 {
				t.Errorf("Unexpected defaulting result (-want, +got): %s", diff)
			}
		})
	}
}

func TestOptimizationJobValidateUpdate(t *testing.T) {
	oldObj := &trainer.OptimizationJob{
		Spec: trainer.OptimizationJobSpec{
			Objectives: []trainer.Objective{
				{Metric: ptr.To("loss")},
			},
			Parameters: []trainer.Parameter{
				{Name: "learning_rate"},
			},
			TrainJobTemplate: trainer.TrainJobTemplateSpec{},
			NumTrials:        ptr.To(int32(10)),
		},
	}

	cases := map[string]struct {
		updateFn func(obj *trainer.OptimizationJob)
		wantErr  bool
	}{
		"Valid update: modifying metadata labels": {
			updateFn: func(obj *trainer.OptimizationJob) {
				if obj.Labels == nil {
					obj.Labels = make(map[string]string)
				}
				obj.Labels["new-label"] = "foo"
			},
			wantErr: false,
		},
		"Invalid update: changing NumTrials": {
			updateFn: func(obj *trainer.OptimizationJob) {
				obj.Spec.NumTrials = ptr.To(int32(20))
			},
			wantErr: true,
		},
		"Invalid update: changing ParallelTrials": {
			updateFn: func(obj *trainer.OptimizationJob) {
				obj.Spec.ParallelTrials = ptr.To(int32(5))
			},
			wantErr: true,
		},
		"Invalid update: changing parameters": {
			updateFn: func(obj *trainer.OptimizationJob) {
				obj.Spec.Parameters = []trainer.Parameter{
					{Name: "batch_size"},
				}
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			validator := &OptimizationJobValidator{}

			// Create a deep copy to mutate
			newObj := oldObj.DeepCopy()
			tc.updateFn(newObj)

			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func gridParam(name string, choices ...string) trainer.Parameter {
	return trainer.Parameter{
		Name: name,
		SearchSpace: trainer.SearchSpace{
			Categorical: &trainer.CategoricalSpace{Choices: choices},
		},
	}
}

// manyWideParams builds numParams categorical parameters, each with
// choicesPerParam distinct choices. The full product (choicesPerParam ^
// numParams) is astronomically larger than int64 can hold, which is exactly
// the overflow case the budget check must survive.
func manyWideParams(numParams, choicesPerParam int) []trainer.Parameter {
	params := make([]trainer.Parameter, 0, numParams)
	for p := 0; p < numParams; p++ {
		choices := make([]string, 0, choicesPerParam)
		for c := 0; c < choicesPerParam; c++ {
			choices = append(choices, fmt.Sprintf("p%d-c%d", p, c))
		}
		params = append(params, gridParam(fmt.Sprintf("param-%d", p), choices...))
	}
	return params
}

func TestOptimizationJobValidateCreate(t *testing.T) {
	cases := map[string]struct {
		obj     *trainer.OptimizationJob
		wantErr bool
	}{
		"Grid: numTrials equal to combinations is allowed": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       ptr.To(int32(6)),
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd", "adam"),   // 2
						gridParam("lr", "0.01", "0.1", "0.001"), // 3 -> product 6
					},
				},
			},
			wantErr: false,
		},
		"Grid: numTrials below combinations is allowed": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       ptr.To(int32(4)),
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd", "adam"),
						gridParam("lr", "0.01", "0.1", "0.001"),
					},
				},
			},
			wantErr: false,
		},
		"Grid: numTrials exceeding combinations is rejected": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       ptr.To(int32(7)),
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd", "adam"),
						gridParam("lr", "0.01", "0.1", "0.001"),
					},
				},
			},
			wantErr: true,
		},
		"Grid: nil numTrials defaults to 1 and is allowed": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					// NumTrials nil -> treated as 1
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd", "adam"),
					},
				},
			},
			wantErr: false,
		},
		"Random: trial budget check is skipped even when numTrials is large": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Random: &trainer.RandomAlgorithm{}},
					NumTrials:       ptr.To(int32(100)),
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd", "adam"),
					},
				},
			},
			wantErr: false,
		},
		"No search algorithm: trial budget check is skipped": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					NumTrials: ptr.To(int32(100)),
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd", "adam"),
					},
				},
			},
			wantErr: false,
		},
		"Grid: many wide parameters do not overflow and are allowed": {
			// Product across these choice sets vastly exceeds int64 if computed
			// naively; the early-return once combinations >= numTrials must keep
			// it correct. numTrials is small, so this is valid.
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       ptr.To(int32(2)),
					Parameters:      manyWideParams(40, 90),
				},
			},
			wantErr: false,
		},
		"Grid: single-choice parameters allow only numTrials=1": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       ptr.To(int32(2)),
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd"), // 1 combination
					},
				},
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			validator := &OptimizationJobValidator{}

			_, err := validator.ValidateCreate(ctx, tc.obj)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
