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

	"k8s.io/klog/v2/ktesting"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func gridParam(name string, choices ...string) trainer.Parameter {
	return trainer.Parameter{
		Name: name,
		SearchSpace: &trainer.SearchSpace{
			Categorical: trainer.CategoricalSpace{Choices: choices},
		},
	}
}

// manyWideParams builds numParams categorical parameters, each with
// choicesPerParam distinct choices. The full product (choicesPerParam ^
// numParams) is astronomically larger than int64 can hold, which is exactly
// the overflow case the budget check must survive.
func manyWideParams(numParams, choicesPerParam int) []trainer.Parameter {
	params := make([]trainer.Parameter, 0, numParams)
	for p := range numParams {
		choices := make([]string, 0, choicesPerParam)
		for c := range choicesPerParam {
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
					NumTrials:       6,
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
					NumTrials:       4,
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
					NumTrials:       7,
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd", "adam"),
						gridParam("lr", "0.01", "0.1", "0.001"),
					},
				},
			},
			wantErr: true,
		},
		"Grid: defaulted numTrials of 1 is allowed": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       1,
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
					NumTrials:       100,
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
					NumTrials: 100,
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
					NumTrials:       2,
					Parameters:      manyWideParams(40, 90),
				},
			},
			wantErr: false,
		},
		"Grid: single-choice parameters allow only numTrials=1": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       2,
					Parameters: []trainer.Parameter{
						gridParam("optimizer", "sgd"), // 1 combination
					},
				},
			},
			wantErr: true,
		},
		"Grid: a non-categorical parameter defers to the CEL rule": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}},
					NumTrials:       100,
					Parameters: []trainer.Parameter{
						{
							Name: "lr",
							SearchSpace: &trainer.SearchSpace{
								Uniform: trainer.UniformSpace{Min: "0.001", Max: "0.1"},
							},
						},
					},
				},
			},
			wantErr: false,
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
