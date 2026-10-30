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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/features"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestOptimizationJobValidateCreate(t *testing.T) {
	cases := map[string]struct {
		featureGates featuregatetesting.FeatureOverrides
		client       client.Client
		obj          *trainer.OptimizationJob
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
		"OptimizationJob with name within limit for runtime passes": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			client: testingutil.NewClientBuilder().WithRuntimeObjects(
				testingutil.MakeClusterTrainingRuntimeWrapper("runtime").Obj(),
			).Build(),
			obj: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 18)).
				Obj(),
		},
		"OptimizationJob with name exceeding limit for runtime fails": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			client: testingutil.NewClientBuilder().WithRuntimeObjects(
				testingutil.MakeClusterTrainingRuntimeWrapper("runtime").Obj(),
			).Build(),
			obj: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 19)).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), strings.Repeat("a", 19),
					"OptimizationJob name is too long, trial TrainJob names generated for replicatedJob \"dataset-initializer\" will exceed maximum allowed length of 33 (maximum OptimizationJob name length is 18)"),
			},
		},
		"OptimizationJob with name exceeding 63 characters fails": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			obj: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 64)).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), strings.Repeat("a", 64),
					"must be no more than 63 characters"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGatesDuringTest mutates global state.
			featuregatetesting.SetFeatureGatesDuringTest(t, utilfeature.DefaultFeatureGate, tc.featureGates)
			_, ctx := ktesting.NewTestContext(t)

			validator := &OptimizationJobValidator{client: tc.client}
			obj := tc.obj
			if obj == nil {
				obj = testingutil.MakeOptimizationJobWrapper("default", "test").Obj()
			}
			_, err := validator.ValidateCreate(ctx, obj)
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateCreate (-want, +got): %s", diff)
			}
		})
	}
}
