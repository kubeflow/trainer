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
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/features"
	runtimecore "github.com/kubeflow/trainer/v2/pkg/runtime/core"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestOptimizationJobValidateCreate(t *testing.T) {
	cases := map[string]struct {
		featureGates           featuregatetesting.FeatureOverrides
		clusterTrainingRuntime *trainer.ClusterTrainingRuntime
		trainingRuntime        *trainer.TrainingRuntime
		optimizationJob        *trainer.OptimizationJob
		wantError              field.ErrorList
	}{
		"OptimizationJob feature gate is disabled": {
			featureGates: featuregatetesting.FeatureOverrides{features.OptimizationJob: false},
			wantError: field.ErrorList{
				field.Forbidden(field.NewPath("spec"), ""),
			},
		},
		"OptimizationJob with name within limit for ClusterTrainingRuntime passes": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			clusterTrainingRuntime: testingutil.MakeClusterTrainingRuntimeWrapper("runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
					},
				}).Obj(),
			optimizationJob: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 27)).
				TrainJobTemplate(trainer.TrainJobTemplateSpec{
					Spec: trainer.TrainJobSpec{
						RuntimeRef: trainer.RuntimeRef{
							APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
							Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
							Name:     "runtime",
						},
					},
				}).
				Obj(),
		},
		"OptimizationJob with name within limit for namespaced TrainingRuntime passes": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper("default", "runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
					},
				}).Obj(),
			optimizationJob: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 27)).
				TrainJobTemplate(trainer.TrainJobTemplateSpec{
					Spec: trainer.TrainJobSpec{
						RuntimeRef: trainer.RuntimeRef{
							APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
							Kind:     ptr.To(trainer.TrainingRuntimeKind),
							Name:     "runtime",
						},
					},
				}).
				Obj(),
		},
		"OptimizationJob with name exceeding limit for runtime fails": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			clusterTrainingRuntime: testingutil.MakeClusterTrainingRuntimeWrapper("runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
					},
				}).Obj(),
			optimizationJob: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 28)).
				TrainJobTemplate(trainer.TrainJobTemplateSpec{
					Spec: trainer.TrainJobSpec{
						RuntimeRef: trainer.RuntimeRef{
							APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
							Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
							Name:     "runtime",
						},
					},
				}).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), strings.Repeat("a", 28), ""),
			},
		},
		"OptimizationJob with numNodes override producing 2-digit index reduces allowed name length": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			clusterTrainingRuntime: testingutil.MakeClusterTrainingRuntimeWrapper("runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
					},
				}).Obj(),
			// dataset-initializer has max TrainJob length 33, but with numNodes 11 on trainer,
			// node has completions 11, giving max TrainJob length 47. dataset-initializer (33) remains limiting.
			// 27 chars (27 + 6 = 33) passes; 28 chars (28 + 6 = 34) fails.
			optimizationJob: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 28)).
				TrainJobTemplate(trainer.TrainJobTemplateSpec{
					Spec: trainer.TrainJobSpec{
						RuntimeRef: trainer.RuntimeRef{
							APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
							Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
							Name:     "runtime",
						},
						Trainer: &trainer.Trainer{
							NumNodes: ptr.To[int32](11),
						},
					},
				}).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), strings.Repeat("a", 28), ""),
			},
		},
		"OptimizationJob referencing missing runtime returns invalid error": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			optimizationJob: testingutil.MakeOptimizationJobWrapper("default", "valid-name").
				TrainJobTemplate(trainer.TrainJobTemplateSpec{
					Spec: trainer.TrainJobSpec{
						RuntimeRef: trainer.RuntimeRef{
							APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
							Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
							Name:     "missing-runtime",
						},
					},
				}).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "trainJobTemplate", "spec", "runtimeRef"), "", ""),
			},
		},
		"OptimizationJob with name exceeding 63 characters fails": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			optimizationJob: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 64)).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), strings.Repeat("a", 64),
					"must be no more than 63 characters"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			featuregatetesting.SetFeatureGatesDuringTest(t, utilfeature.DefaultFeatureGate, tc.featureGates)
			_, ctx := ktesting.NewTestContext(t)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.clusterTrainingRuntime)
			}
			if tc.trainingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.trainingRuntime)
			}

			runtimes, err := runtimecore.New(ctx, clientBuilder.Build(), testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}

			validator := &OptimizationJobValidator{runtimes: runtimes}
			optimizationJob := tc.optimizationJob
			if optimizationJob == nil {
				optimizationJob = testingutil.MakeOptimizationJobWrapper("default", "test").Obj()
			}
			_, err = validator.ValidateCreate(ctx, optimizationJob)
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateCreate (-want, +got): %s", diff)
			}
		})
	}
}
