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
	"fmt"
	"strings"
	"testing"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

func TestOptimizationJobDependencies(t *testing.T) {
	cases := map[string]struct {
		featureGates map[string]bool
		wantEnabled  bool
		wantError    bool
	}{
		"disabled by default": {},
		"enabled with TrainJobStatus": {
			featureGates: map[string]bool{string(OptimizationJob): true, string(TrainJobStatus): true},
			wantEnabled:  true,
		},
		"requires TrainJobStatus": {
			featureGates: map[string]bool{string(OptimizationJob): true},
			wantError:    true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gate := utilfeature.DefaultMutableFeatureGate.DeepCopy()
			err := gate.SetFromMap(tc.featureGates)
			if tc.wantError {
				wantErr := fmt.Sprintf("%s is enabled, but depends on features that are disabled: [%s]", OptimizationJob, TrainJobStatus)
				if err == nil || !strings.Contains(err.Error(), wantErr) {
					t.Fatalf("SetFromMap() error = %v, want error containing %q", err, wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error from SetFromMap: %v", err)
			}
			if got := gate.Enabled(OptimizationJob); got != tc.wantEnabled {
				t.Errorf("Enabled(%s) = %t, want %t", OptimizationJob, got, tc.wantEnabled)
			}
		})
	}
}
