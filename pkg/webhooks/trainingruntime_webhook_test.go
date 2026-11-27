/*
Copyright 2024 The Kubeflow Authors.

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
	corev1 "k8s.io/api/core/v1"
	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/features"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestTrainingRuntimeValidateCreate(t *testing.T) {
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
				"TrainingRuntime \"test-runtime\" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: " + constants.RuntimeDeprecationPolicyURL,
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

			obj := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj()
			// Set valid replicas to avoid validation errors
			for i := range obj.Spec.Template.Spec.ReplicatedJobs {
				obj.Spec.Template.Spec.ReplicatedJobs[i].Replicas = 1
			}

			if len(tc.labels) != 0 {
				obj.Labels = tc.labels
			}
			obj.Spec.Scheduling = tc.scheduling

			validator := &TrainingRuntimeValidator{}
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

func TestValidateReplicatedJobs(t *testing.T) {
	cases := map[string]struct {
		rJobs     []jobsetv1alpha2.ReplicatedJob
		wantError field.ErrorList
	}{
		"valid replicatedJobs": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				LauncherReplica().
				Replicas(1, constants.Launcher, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				Obj().Spec.ReplicatedJobs,
		},
		"valid replicatedJobs with unknown user-specified ancestor": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				ReplicatedJobLabel(constants.LabelTrainJobAncestor, "user-specified", constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				Replicas(2, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				Obj().Spec.ReplicatedJobs,
		},
		"valid replicas greater than one": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				LauncherReplica().
				Replicas(2, constants.Launcher, constants.DatasetInitializer, constants.ModelInitializer).
				Replicas(4, constants.Node).
				Obj().Spec.ReplicatedJobs,
		},
		"invalid replicas: zero is not allowed": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				ReplicatedJobLabel(constants.LabelTrainJobAncestor, "user-specified", constants.ModelInitializer).
				Replicas(1, constants.DatasetInitializer).
				Replicas(0, constants.ModelInitializer, constants.Node).
				Obj().Spec.ReplicatedJobs,
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(1).Child("replicas"),
					int32(0), ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(2).Child("replicas"),
					int32(0), ""),
			},
		},
		"missing required container in replicatedJobs": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				Replicas(1, constants.Launcher, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				ReplaceContainer(constants.DatasetInitializer, constants.DatasetInitializer, "test", "", []string{}, []string{}, corev1.ResourceList{}).
				ReplaceContainer(constants.ModelInitializer, constants.ModelInitializer, "test", "", []string{}, []string{}, corev1.ResourceList{}).
				ReplaceContainer(constants.Node, constants.Node, "test", "", []string{}, []string{}, corev1.ResourceList{}).
				Obj().Spec.ReplicatedJobs,
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(0).Child("template").Child("spec").Child("template").Child("spec").Child("containers"),
					[]corev1.Container{{Name: "test"}}, ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(1).Child("template").Child("spec").Child("template").Child("spec").Child("containers"),
					[]corev1.Container{{Name: "test"}}, ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(2).Child("template").Child("spec").Child("template").Child("spec").Child("containers"),
					[]corev1.Container{{Name: "test"}}, ""),
			},
		},
		"container claim referencing a pod resourceClaim": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				PodResourceClaims(constants.Node, corev1.PodResourceClaim{Name: "gpu", ResourceClaimTemplateName: ptr.To("gpu-template")}).
				ContainerResourceClaims(constants.Node, constants.Node, corev1.ResourceClaim{Name: "gpu"}).
				Obj().Spec.ReplicatedJobs,
		},
		"container claim without a matching pod resourceClaim": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				ContainerResourceClaims(constants.Node, constants.Node, corev1.ResourceClaim{Name: "gpu"}).
				Obj().Spec.ReplicatedJobs,
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(2).
					Child("template").Child("spec").Child("template").Child("spec").Child("containers").Index(0).
					Child("resources").Child("claims").Index(0),
					"gpu", ""),
			},
		},
		"initContainer claim without a matching pod resourceClaim": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				InitContainer(constants.Node, "init", "test").
				ContainerResourceClaims(constants.Node, "init", corev1.ResourceClaim{Name: "gpu"}).
				Obj().Spec.ReplicatedJobs,
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(2).
					Child("template").Child("spec").Child("template").Child("spec").Child("initContainers").Index(0).
					Child("resources").Child("claims").Index(0),
					"gpu", ""),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateReplicatedJobs(tc.rJobs)
			if diff := cmp.Diff(tc.wantError, gotErr, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("validateReplicateJobs() mismatch (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestTrainingRuntimeValidateUpdate(t *testing.T) {
	// Do not use t.Parallel() — SetFeatureGateDuringTest mutates global state.
	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.TrainJobWorkloadAwareScheduling, true)
	cases := map[string]struct {
		oldObj    *trainer.TrainingRuntime
		newObj    *trainer.TrainingRuntime
		wantError field.ErrorList
	}{
		"valid update succeeds": {
			oldObj: testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj(),
			newObj: func() *trainer.TrainingRuntime {
				obj := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj()
				for i := range obj.Spec.Template.Spec.ReplicatedJobs {
					obj.Spec.Template.Spec.ReplicatedJobs[i].Replicas = 1
				}
				return obj
			}(),
		},
		"invalid update with invalid replicas fails": {
			oldObj: testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj(),
			newObj: func() *trainer.TrainingRuntime {
				obj := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj()
				for i := range obj.Spec.Template.Spec.ReplicatedJobs {
					obj.Spec.Template.Spec.ReplicatedJobs[i].Replicas = 0
				}
				return obj
			}(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(0).Child("replicas"), int32(0), ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(1).Child("replicas"), int32(0), ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(2).Child("replicas"), int32(0), ""),
			},
		},
		"update from invalid old object succeeds": {
			oldObj: func() *trainer.TrainingRuntime {
				obj := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj()
				for i := range obj.Spec.Template.Spec.ReplicatedJobs {
					obj.Spec.Template.Spec.ReplicatedJobs[i].Replicas = 0
				}
				return obj
			}(),
			newObj: func() *trainer.TrainingRuntime {
				obj := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj()
				for i := range obj.Spec.Template.Spec.ReplicatedJobs {
					obj.Spec.Template.Spec.ReplicatedJobs[i].Replicas = 1
				}
				return obj
			}(),
		},
		"leaving a ReplicatedJob untargeted is rejected": {
			oldObj: testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").
				RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj().Spec).
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Scheduling(&trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer, constants.Node}},
						},
					}).
					Obj()).
				Obj(),
			newObj: testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").
				RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj().Spec).
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

			validator := &TrainingRuntimeValidator{}
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

func TestTrainingRuntimeValidateDelete(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	validator := &TrainingRuntimeValidator{}
	obj := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").Obj()
	warnings, err := validator.ValidateDelete(ctx, obj)
	if err != nil {
		t.Fatalf("ValidateDelete() unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("ValidateDelete() warnings = %v, want nil", warnings)
	}
}

func TestValidateScheduling(t *testing.T) {
	gang := &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
		Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
	}
	runtimeSpec := func(scheduling *trainer.TrainingRuntimeScheduling) *trainer.TrainingRuntimeSpec {
		spec := testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper("ns", "runtime").Obj().Spec).
			Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
			Scheduling(scheduling).
			Obj()
		return &spec
	}
	schedulingPath := field.NewPath("spec", "scheduling")
	rJobsPath := field.NewPath("spec", "template", "spec", "replicatedJobs")

	cases := map[string]struct {
		enableWAS bool
		oldSpec   *trainer.TrainingRuntimeSpec
		newSpec   *trainer.TrainingRuntimeSpec
		wantError field.ErrorList
	}{
		"scheduling is not set and feature gate is disabled": {
			newSpec: runtimeSpec(nil),
		},
		"scheduling is set on create and feature gate is disabled": {
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{SchedulingPolicy: gang}),
			wantError: field.ErrorList{
				field.Forbidden(schedulingPath, ""),
			},
		},
		"unchanged scheduling on update is allowed when feature gate is disabled": {
			oldSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{SchedulingPolicy: gang}),
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{SchedulingPolicy: gang}),
		},
		"changed scheduling on update is rejected when feature gate is disabled": {
			oldSpec: runtimeSpec(nil),
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{SchedulingPolicy: gang}),
			wantError: field.ErrorList{
				field.Forbidden(schedulingPath, ""),
			},
		},
		"valid TrainJob-level scheduling": {
			enableWAS: true,
			newSpec:   runtimeSpec(&trainer.TrainingRuntimeScheduling{SchedulingPolicy: gang}),
		},
		"valid per-ReplicatedJob scheduling targeting every ReplicatedJob": {
			enableWAS: true,
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node}},
				},
			}),
		},
		"targetReplicatedJobs references a ReplicatedJob which does not exist": {
			enableWAS: true,
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node, "invalid"}},
				},
			}),
			wantError: field.ErrorList{
				field.Invalid(schedulingPath.Child("replicatedJobs").Index(1).Child("targetReplicatedJobs").Index(1), "", ""),
			},
		},
		"per-ReplicatedJob scheduling leaves a ReplicatedJob untargeted": {
			enableWAS: true,
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
				},
			}),
			wantError: field.ErrorList{
				field.Invalid(rJobsPath.Index(2).Child("name"), "", ""),
			},
		},
		"Job-level scheduling targeting a ReplicatedJob with one replica": {
			enableWAS: true,
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{}},
				},
			}),
		},
		"Job-level scheduling targeting a ReplicatedJob with more than one replica": {
			enableWAS: true,
			newSpec: func() *trainer.TrainingRuntimeSpec {
				spec := runtimeSpec(&trainer.TrainingRuntimeScheduling{
					ReplicatedJobs: []trainer.ReplicatedJobScheduling{
						{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
						{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{}},
					},
				})
				spec.Template.Spec.ReplicatedJobs[2].Replicas = 2
				return spec
			}(),
			wantError: field.ErrorList{
				field.Invalid(schedulingPath.Child("replicatedJobs").Index(1).Child("job"), "", ""),
			},
		},
		"scheduling is rejected by the workloadbuilder validation": {
			enableWAS: true,
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{
				SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
					Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
				},
				DisruptionMode: &schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode{
					All: &schedulingv1alpha3.WorkloadCompositePodGroupAllDisruptionMode{},
				},
			}),
			wantError: field.ErrorList{
				field.Invalid(schedulingPath.Child("disruptionMode"), "", ""),
			},
		},
		"TrainJob-level scheduling with the same priorityClassName for every ReplicatedJob": {
			enableWAS: true,
			newSpec: func() *trainer.TrainingRuntimeSpec {
				spec := runtimeSpec(&trainer.TrainingRuntimeScheduling{SchedulingPolicy: gang})
				for i := range spec.Template.Spec.ReplicatedJobs {
					spec.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.PriorityClassName = "high-priority"
				}
				return spec
			}(),
		},
		"TrainJob-level scheduling with a different priorityClassName for a ReplicatedJob": {
			enableWAS: true,
			newSpec: func() *trainer.TrainingRuntimeSpec {
				spec := runtimeSpec(&trainer.TrainingRuntimeScheduling{SchedulingPolicy: gang})
				spec.Template.Spec.ReplicatedJobs[2].Template.Spec.Template.Spec.PriorityClassName = "high-priority"
				return spec
			}(),
			wantError: field.ErrorList{
				field.Invalid(rJobsPath.Index(2).Child("template", "spec", "template", "spec", "priorityClassName"), "", ""),
			},
		},
		"per-ReplicatedJob scheduling entry with a different priorityClassName for a target": {
			enableWAS: true,
			newSpec: func() *trainer.TrainingRuntimeSpec {
				spec := runtimeSpec(&trainer.TrainingRuntimeScheduling{
					ReplicatedJobs: []trainer.ReplicatedJobScheduling{
						{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
						{TargetReplicatedJobs: []string{constants.Node}},
					},
				})
				spec.Template.Spec.ReplicatedJobs[1].Template.Spec.Template.Spec.PriorityClassName = "high-priority"
				return spec
			}(),
			wantError: field.ErrorList{
				field.Invalid(schedulingPath.Child("replicatedJobs").Index(0).Child("targetReplicatedJobs").Index(1), "", ""),
			},
		},
		"per-ReplicatedJob scheduling entries with a single target can use different priorityClassNames": {
			enableWAS: true,
			newSpec: func() *trainer.TrainingRuntimeSpec {
				spec := runtimeSpec(&trainer.TrainingRuntimeScheduling{
					ReplicatedJobs: []trainer.ReplicatedJobScheduling{
						{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
						{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{}},
					},
				})
				spec.Template.Spec.ReplicatedJobs[2].Template.Spec.Template.Spec.PriorityClassName = "high-priority"
				return spec
			}(),
		},
		"Job-level resourceClaims with a matching Pod resourceClaim": {
			enableWAS: true,
			newSpec: func() *trainer.TrainingRuntimeSpec {
				spec := runtimeSpec(&trainer.TrainingRuntimeScheduling{
					ReplicatedJobs: []trainer.ReplicatedJobScheduling{
						{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
						{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{
							ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
								Name:                      "tpu-slice",
								ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
							}},
						}},
					},
				})
				spec.Template.Spec.ReplicatedJobs[2].Template.Spec.Template.Spec.ResourceClaims = []corev1.PodResourceClaim{{
					Name:                      "tpu-slice",
					ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
				}}
				return spec
			}(),
		},
		"Job-level resourceClaims without a matching Pod resourceClaim": {
			enableWAS: true,
			newSpec: runtimeSpec(&trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{
						ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
							Name:                      "tpu-slice",
							ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
						}},
					}},
				},
			}),
			wantError: field.ErrorList{
				field.Invalid(schedulingPath.Child("replicatedJobs").Index(1).Child("job", "resourceClaims").Index(0), "", ""),
			},
		},
		"Job-level resourceClaims with a Pod resourceClaim referencing a different ResourceClaimTemplate": {
			enableWAS: true,
			newSpec: func() *trainer.TrainingRuntimeSpec {
				spec := runtimeSpec(&trainer.TrainingRuntimeScheduling{
					ReplicatedJobs: []trainer.ReplicatedJobScheduling{
						{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
						{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{
							ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
								Name:                      "tpu-slice",
								ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
							}},
						}},
					},
				})
				spec.Template.Spec.ReplicatedJobs[2].Template.Spec.Template.Spec.ResourceClaims = []corev1.PodResourceClaim{{
					Name:                      "tpu-slice",
					ResourceClaimTemplateName: ptr.To("other-template"),
				}}
				return spec
			}(),
			wantError: field.ErrorList{
				field.Invalid(schedulingPath.Child("replicatedJobs").Index(1).Child("job", "resourceClaims").Index(0), "", ""),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGateDuringTest mutates global state.
			featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.TrainJobWorkloadAwareScheduling, tc.enableWAS)
			gotErr := validateScheduling(context.Background(), tc.newSpec, tc.oldSpec)
			if diff := cmp.Diff(tc.wantError, gotErr, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("validateScheduling() mismatch (-want,+got):\n%s", diff)
			}
		})
	}
}
