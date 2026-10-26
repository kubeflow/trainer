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

package features

import (
	"testing"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
)

func TestValidateDependencies(t *testing.T) {
	cases := map[string]struct {
		featureGates featuregatetesting.FeatureOverrides
		wantError    bool
	}{
		"OptimizationJob without TrainJobStatus is rejected": {
			featureGates: featuregatetesting.FeatureOverrides{OptimizationJob: true},
			wantError:    true,
		},
		"OptimizationJob with TrainJobStatus is accepted": {
			featureGates: featuregatetesting.FeatureOverrides{OptimizationJob: true, TrainJobStatus: true},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGatesDuringTest mutates global state.
			featuregatetesting.SetFeatureGatesDuringTest(t, utilfeature.DefaultFeatureGate, tc.featureGates)
			err := ValidateDependencies()
			if gotError := err != nil; gotError != tc.wantError {
				t.Errorf("ValidateDependencies() error = %v, want error: %t", err, tc.wantError)
			}
		})
	}
}
