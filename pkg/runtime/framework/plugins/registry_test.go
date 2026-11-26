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

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/coscheduling"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/flux"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jax"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jobset"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/mpi"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/plainml"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/torch"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/trainjobstatus"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/volcano"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/workloadawarescheduling"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/xgboost"
)

func TestNewRegistry(t *testing.T) {
	cases := map[string]struct {
		featureGates featuregatetesting.FeatureOverrides
		wantRegistry sets.Set[string]
	}{
		"TrainJobStatus plugin is registered by default": {
			wantRegistry: sets.New(
				coscheduling.Name, flux.Name, volcano.Name, mpi.Name, plainml.Name, torch.Name, jobset.Name, jax.Name, xgboost.Name,
				trainjobstatus.Name,
			),
		},
		"TrainJobStatus plugin is not registered when the TrainJobStatus feature gate is disabled": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: false},
			wantRegistry: sets.New(
				coscheduling.Name, flux.Name, volcano.Name, mpi.Name, plainml.Name, torch.Name, jobset.Name, jax.Name, xgboost.Name,
			),
		},
		"WorkloadAwareScheduling plugin is registered alongside default TrainJobStatus when TrainJobWorkloadAwareScheduling is enabled": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobWorkloadAwareScheduling: true},
			wantRegistry: sets.New(
				coscheduling.Name, flux.Name, volcano.Name, mpi.Name, plainml.Name, torch.Name, jobset.Name, jax.Name, xgboost.Name,
				trainjobstatus.Name, workloadawarescheduling.Name,
			),
		},
		"WorkloadAwareScheduling plugin is registered without TrainJobStatus when TrainJobStatus is disabled": {
			featureGates: featuregatetesting.FeatureOverrides{
				features.TrainJobStatus:                  false,
				features.TrainJobWorkloadAwareScheduling: true,
			},
			wantRegistry: sets.New(
				coscheduling.Name, flux.Name, volcano.Name, mpi.Name, plainml.Name, torch.Name, jobset.Name, jax.Name, xgboost.Name,
				workloadawarescheduling.Name,
			),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGatesDuringTest mutates global state.
			if len(tc.featureGates) > 0 {
				featuregatetesting.SetFeatureGatesDuringTest(t, utilfeature.DefaultFeatureGate, tc.featureGates)
			}
			if diff := cmp.Diff(tc.wantRegistry, sets.KeySet(NewRegistry())); len(diff) != 0 {
				t.Errorf("Unexpected registry (-want,+got): %s", diff)
			}
		})
	}
}
