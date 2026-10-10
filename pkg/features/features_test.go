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

	"k8s.io/component-base/featuregate"
)

func TestTrainJobStatusFeatureGate(t *testing.T) {
	spec, ok := defaultFeatureGates[TrainJobStatus]
	if !ok {
		t.Fatal("TrainJobStatus feature gate is not registered")
	}

	if !spec.Default {
		t.Error("TrainJobStatus feature gate should be enabled by default")
	}

	if spec.PreRelease != featuregate.Beta {
		t.Errorf("TrainJobStatus feature gate stage = %q, want %q", spec.PreRelease, featuregate.Beta)
	}

	if spec.LockToDefault {
		t.Error("TrainJobStatus feature gate should not be locked to default in Beta")
	}
}
