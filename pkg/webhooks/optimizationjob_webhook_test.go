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
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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
		clientInterceptor      *interceptor.Funcs
		obj                    *trainer.OptimizationJob
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
			obj: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 25)).
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
			obj: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 25)).
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
			obj: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 26)).
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
				field.Invalid(field.NewPath("metadata", "name"), strings.Repeat("a", 26), ""),
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
			// 25 chars (25 + 8 = 33) passes; 26 chars (26 + 8 = 34) fails.
			obj: testingutil.MakeOptimizationJobWrapper("default", strings.Repeat("a", 26)).
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
				field.Invalid(field.NewPath("metadata", "name"), strings.Repeat("a", 26), ""),
			},
		},
		"OptimizationJob referencing missing runtime returns invalid error": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			obj: testingutil.MakeOptimizationJobWrapper("default", "valid-name").
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
		"OptimizationJob runtime lookup error returns internal error": {
			featureGates: featuregatetesting.FeatureOverrides{features.TrainJobStatus: true, features.OptimizationJob: true},
			clientInterceptor: &interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.New("simulated lookup error")
				},
			},
			obj: testingutil.MakeOptimizationJobWrapper("default", "valid-name").
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
				field.InternalError(field.NewPath("spec", "trainJobTemplate", "spec", "runtimeRef"), errors.New("simulated lookup error")),
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
			featuregatetesting.SetFeatureGatesDuringTest(t, utilfeature.DefaultFeatureGate, tc.featureGates)
			_, ctx := ktesting.NewTestContext(t)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.clusterTrainingRuntime)
			}
			if tc.trainingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.trainingRuntime)
			}
			if tc.clientInterceptor != nil {
				clientBuilder = clientBuilder.WithInterceptorFuncs(*tc.clientInterceptor)
			}

			runtimes, err := runtimecore.New(ctx, clientBuilder.Build(), testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}

			validator := &OptimizationJobValidator{runtimes: runtimes}
			obj := tc.obj
			if obj == nil {
				obj = testingutil.MakeOptimizationJobWrapper("default", "test").Obj()
			}
			_, err = validator.ValidateCreate(ctx, obj)
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateCreate (-want, +got): %s", diff)
			}
		})
	}
}
