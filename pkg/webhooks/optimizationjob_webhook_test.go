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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/klog/v2/ktesting"

	"github.com/kubeflow/trainer/v2/pkg/features"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestOptimizationJobValidateCreate(t *testing.T) {
	cases := map[string]struct {
		featureGates featuregatetesting.FeatureOverrides
		wantError    field.ErrorList
	}{
		"OptimizationJob feature gate is disabled": {
			featureGates: featuregatetesting.FeatureOverrides{features.OptimizationJob: false},
			wantError: field.ErrorList{
				field.Forbidden(field.NewPath("spec"), ""),
			},
		},
		"OptimizationJob feature gate is enabled": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGatesDuringTest mutates global state.
			featuregatetesting.SetFeatureGatesDuringTest(t, utilfeature.DefaultFeatureGate, tc.featureGates)
			_, ctx := ktesting.NewTestContext(t)

			validator := &OptimizationJobValidator{}
			_, err := validator.ValidateCreate(ctx, testingutil.MakeOptimizationJobWrapper("default", "test").Obj())
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateCreate (-want, +got): %s", diff)
			}
		})
	}
}
