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
	"k8s.io/utils/ptr"

	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/trainjobstatus"
)

func TestNewRegistry_TrainJobStatus(t *testing.T) {
	cases := map[string]struct {
		gateVal    *bool
		wantPlugin bool
	}{
		"TrainJobStatus registered by default": {
			gateVal:    nil,
			wantPlugin: true,
		},
		"TrainJobStatus registered when explicitly enabled": {
			gateVal:    ptr.To(true),
			wantPlugin: true,
		},
		"TrainJobStatus not registered when explicitly disabled": {
			gateVal:    ptr.To(false),
			wantPlugin: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.gateVal != nil {
				featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.TrainJobStatus, *tc.gateVal)
			}
			registry := NewRegistry()
			_, isRegistered := registry[trainjobstatus.Name]
			if isRegistered != tc.wantPlugin {
				t.Errorf("NewRegistry() plugin %q registered = %v, want %v", trainjobstatus.Name, isRegistered, tc.wantPlugin)
			}
		})
	}
}
