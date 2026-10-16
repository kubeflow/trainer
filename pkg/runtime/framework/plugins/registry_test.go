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

package plugins

import (
	"testing"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/workload"
)

func TestNewRegistryWorkloadPlugin(t *testing.T) {
	cases := map[string]struct {
		enableWAS      bool
		wantRegistered bool
	}{
		"Workload plugin is not registered when the feature gate is disabled": {},
		"Workload plugin is registered when the feature gate is enabled": {
			enableWAS:      true,
			wantRegistered: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGateDuringTest mutates global state.
			featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.TrainJobWorkloadAwareScheduling, tc.enableWAS)
			_, gotRegistered := NewRegistry()[workload.Name]
			if gotRegistered != tc.wantRegistered {
				t.Errorf("Unexpected Workload plugin registration: got %v, want %v", gotRegistered, tc.wantRegistered)
			}
		})
	}
}
