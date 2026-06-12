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
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestOptimizationJobDefault(t *testing.T) {
	cases := map[string]struct {
		inputObj *trainer.OptimizationJob
		wantObj  *trainer.OptimizationJob
	}{
		"Empty fields get defaulted": {
			inputObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Algorithm:   trainer.Algorithm{Name: "random"},
					TrialConfig: trainer.TrialConfig{},
				},
			},
			wantObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Algorithm: trainer.Algorithm{
						Name:     "random",
						Provider: ptr.To("optuna"), // Defaulted
					},
					TrialConfig: trainer.TrialConfig{
						ParallelTrials: ptr.To(int32(1)), // Defaulted
						NumTrials:      ptr.To(int32(1)), // Defaulted
					},
				},
			},
		},
		"Existing fields are preserved": {
			inputObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Algorithm: trainer.Algorithm{
						Name:     "bayesian",
						Provider: ptr.To("vizier"),
					},
					TrialConfig: trainer.TrialConfig{
						ParallelTrials: ptr.To(int32(5)),
						NumTrials:      ptr.To(int32(20)),
					},
				},
			},
			wantObj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Algorithm: trainer.Algorithm{
						Name:     "bayesian",
						Provider: ptr.To("vizier"), // Preserved
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

func TestOptimizationJobValidateCreate(t *testing.T) {
	cases := map[string]struct {
		obj       *trainer.OptimizationJob
		wantError field.ErrorList
	}{
		"Valid template with all placeholders": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Parameters: []trainer.Parameter{
						{Name: "learning_rate"},
						{Name: "batch_size"},
					},
					TrainJobTemplate: trainer.TrainJobTemplateSpec{
						Spec: trainer.TrainJobSpec{
							// Simulating a raw string inside the spec that contains the placeholders
							ManagedBy: ptr.To("some-controller --lr={{.learning_rate}} --bs={{ .batch_size }}"),
						},
					},
				},
			},
			wantError: nil,
		},
		"Invalid template missing placeholders": {
			obj: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Parameters: []trainer.Parameter{
						{Name: "learning_rate"}, // This one is missing from the template
						{Name: "batch_size"},    // This one is present
					},
					TrainJobTemplate: trainer.TrainJobTemplateSpec{
						Spec: trainer.TrainJobSpec{
							ManagedBy: ptr.To("some-controller --bs={{.batch_size}}"),
						},
					},
				},
			},
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "parameters").Index(0).Child("name"),
					"learning_rate",
					"Parameter 'learning_rate' is defined, but no placeholder ({{.learning_rate}}) was found in the trainJobTemplate. The controller will not be able to inject this value.",
				),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			validator := &OptimizationJobValidator{}

			_, err := validator.ValidateCreate(ctx, tc.obj)

			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateCreate (-want, +got): %s", diff)
			}
		})
	}
}
