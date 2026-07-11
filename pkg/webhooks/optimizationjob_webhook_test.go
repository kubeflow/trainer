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

package webhooks

import (
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
		"Empty TrialConfig fields get defaulted": {
			inputObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: trainer.SearchAlgorithm{
						Random: &trainer.RandomAlgorithm{},
					},
					TrialConfig: trainer.TrialConfig{},
				},
			},
			wantObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: trainer.SearchAlgorithm{
						Random: &trainer.RandomAlgorithm{},
					},
					TrialConfig: trainer.TrialConfig{
						ParallelTrials: ptr.To(int32(1)), // Defaulted
						NumTrials:      ptr.To(int32(1)), // Defaulted
					},
				},
			},
		},
		"Existing TrialConfig fields are preserved": {
			inputObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: trainer.SearchAlgorithm{
						Bayesian: &trainer.BayesianAlgorithm{},
					},
					TrialConfig: trainer.TrialConfig{
						ParallelTrials: ptr.To(int32(5)),
						NumTrials:      ptr.To(int32(20)),
					},
				},
			},
			wantObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: trainer.SearchAlgorithm{
						Bayesian: &trainer.BayesianAlgorithm{},
					},
					TrialConfig: trainer.TrialConfig{
						ParallelTrials: ptr.To(int32(5)),  // Preserved
						NumTrials:      ptr.To(int32(20)), // Preserved
					},
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
