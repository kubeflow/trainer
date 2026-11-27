/*
Copyright 2025 The Kubeflow Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/features"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestClusterTrainingRuntimeValidateCreate(t *testing.T) {
	// Do not use t.Parallel() — SetFeatureGateDuringTest mutates global state.
	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.TrainJobWorkloadAwareScheduling, true)
	cases := map[string]struct {
		labels     map[string]string
		scheduling *trainer.TrainingRuntimeScheduling
		warnings   admission.Warnings
		wantError  field.ErrorList
	}{
		"no deprecation label": {
			labels:   nil,
			warnings: nil,
		},
		"support=deprecated": {
			labels: map[string]string{constants.LabelSupport: constants.SupportDeprecated},
			warnings: admission.Warnings{
				"ClusterTrainingRuntime \"test-runtime\" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: " + constants.RuntimeDeprecationPolicyURL,
			},
		},
		"scheduling leaving a ReplicatedJob untargeted is rejected": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{{TargetReplicatedJobs: []string{constants.Node}}},
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "template", "spec", "replicatedJobs").Index(0).Child("name"), constants.DatasetInitializer, ""),
				field.Invalid(field.NewPath("spec", "template", "spec", "replicatedJobs").Index(1).Child("name"), constants.ModelInitializer, ""),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			obj := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: func() jobsetv1alpha2.JobSetSpec {
							js := testingutil.MakeJobSetWrapper("", "")
							js.Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node)
							return js.Obj().Spec
						}(),
					},
				}).Obj()

			if len(tc.labels) != 0 {
				obj.Labels = tc.labels
			}
			obj.Spec.Scheduling = tc.scheduling

			validator := &ClusterTrainingRuntimeValidator{}
			warnings, err := validator.ValidateCreate(ctx, obj)
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateCreate (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.warnings, warnings); diff != "" {
				t.Fatalf("unexpected warnings (-want, +got): %s", diff)
			}
		})
	}
}

func TestClusterTrainingRuntimeValidateUpdate(t *testing.T) {
	// Do not use t.Parallel() — SetFeatureGateDuringTest mutates global state.
	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.TrainJobWorkloadAwareScheduling, true)
	cases := map[string]struct {
		oldObj    *trainer.ClusterTrainingRuntime
		newObj    *trainer.ClusterTrainingRuntime
		wantError field.ErrorList
	}{
		"valid update succeeds": {
			oldObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Obj(),
			newObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: func() jobsetv1alpha2.JobSetSpec {
							js := testingutil.MakeJobSetWrapper("", "")
							js.Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node)
							return js.Obj().Spec
						}(),
					},
				}).Obj(),
		},
		"invalid update with invalid replicas fails": {
			oldObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Obj(),
			newObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: func() jobsetv1alpha2.JobSetSpec {
							js := testingutil.MakeJobSetWrapper("", "")
							js.Replicas(0, constants.DatasetInitializer, constants.ModelInitializer, constants.Node)
							return js.Obj().Spec
						}(),
					},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(0).Child("replicas"), int32(0), ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(1).Child("replicas"), int32(0), ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(2).Child("replicas"), int32(0), ""),
			},
		},
		"update from invalid old object succeeds": {
			oldObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: func() jobsetv1alpha2.JobSetSpec {
							js := testingutil.MakeJobSetWrapper("", "")
							js.Replicas(0, constants.DatasetInitializer, constants.ModelInitializer, constants.Node)
							return js.Obj().Spec
						}(),
					},
				}).Obj(),
			newObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: func() jobsetv1alpha2.JobSetSpec {
							js := testingutil.MakeJobSetWrapper("", "")
							js.Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node)
							return js.Obj().Spec
						}(),
					},
				}).Obj(),
		},
		"leaving a ReplicatedJob untargeted is rejected": {
			oldObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Obj().Spec).
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Scheduling(&trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer, constants.Node}},
						},
					}).
					Obj()).
				Obj(),
			newObj: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Obj().Spec).
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Scheduling(&trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{{TargetReplicatedJobs: []string{constants.Node}}},
					}).
					Obj()).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "template", "spec", "replicatedJobs").Index(0).Child("name"), constants.DatasetInitializer, ""),
				field.Invalid(field.NewPath("spec", "template", "spec", "replicatedJobs").Index(1).Child("name"), constants.ModelInitializer, ""),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			validator := &ClusterTrainingRuntimeValidator{}
			warnings, err := validator.ValidateUpdate(ctx, tc.oldObj, tc.newObj)
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateUpdate (-want, +got): %s", diff)
			}
			if len(warnings) != 0 {
				t.Errorf("ValidateUpdate() warnings = %v, want nil", warnings)
			}
		})
	}
}

func TestClusterTrainingRuntimeValidateDelete(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	validator := &ClusterTrainingRuntimeValidator{}
	obj := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Obj()
	warnings, err := validator.ValidateDelete(ctx, obj)
	if err != nil {
		t.Fatalf("ValidateDelete() unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("ValidateDelete() warnings = %v, want nil", warnings)
	}
}
