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

package workload

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestWorkload(t *testing.T) {
	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj()

	cases := map[string]struct {
		trainJob   *trainer.TrainJob
		info       *runtime.Info
		expectInfo *runtime.Info
	}{
		"no action when info is nil": {
			trainJob: trainJob,
		},
		"no action when trainJob is nil": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node}},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node}},
				},
			},
		},
		"no action when the runtime does not set scheduling": {
			trainJob: trainJob,
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node}},
				},
			},
			expectInfo: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node}},
				},
			},
		},
		"TrainJob-level scheduling assigns every PodSet to one PodGroup": {
			trainJob: trainJob,
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{Name: constants.DatasetInitializer},
						{Name: constants.Node},
					},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:            constants.DatasetInitializer,
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName(podGroupName(trainJob, "test-job")),
						},
						{
							Name:            constants.Node,
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName(podGroupName(trainJob, "test-job")),
						},
					},
				},
			},
		},
		"per-ReplicatedJob scheduling assigns every entry to its own PodGroup": {
			trainJob: trainJob,
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
							{TargetReplicatedJobs: []string{constants.Node}},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{Name: constants.DatasetInitializer},
						{Name: constants.ModelInitializer},
						{Name: constants.Node},
					},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
							{TargetReplicatedJobs: []string{constants.Node}},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:            constants.DatasetInitializer,
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName(podGroupName(trainJob, "0-dataset-initializer-model-initializer")),
						},
						{
							Name:            constants.ModelInitializer,
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName(podGroupName(trainJob, "0-dataset-initializer-model-initializer")),
						},
						{
							Name:            constants.Node,
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName(podGroupName(trainJob, "1-node")),
						},
					},
				},
			},
		},
		"Job-level scheduling assigns the Job of the ReplicatedJob to its own PodGroup": {
			trainJob: trainJob,
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{}},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node}},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{}},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:            constants.Node,
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName(podGroupName(trainJob, "node-0")),
						},
					},
				},
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			plugin, err := New(ctx, utiltesting.NewClientBuilder().Build(), nil, nil)
			if err != nil {
				t.Fatalf("Failed to initialize Workload plugin: %v", err)
			}

			if err := plugin.(framework.EnforceSchedulingPlugin).EnforceScheduling(c.info, c.trainJob); err != nil {
				t.Fatalf("Unexpected error from EnforceScheduling: %v", err)
			}
			if diff := cmp.Diff(c.expectInfo, c.info); len(diff) != 0 {
				t.Errorf("Unexpected info from EnforceScheduling (-want, +got): %s", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	jobClaimsScheduling := &trainer.TrainingRuntimeScheduling{
		ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
			TargetReplicatedJobs: []string{constants.Node},
			Job: &trainer.JobScheduling{
				ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
					Name:                      "tpu-slice",
					ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
				}},
			},
		}},
	}
	jobScheduling := &trainer.TrainingRuntimeScheduling{
		ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
			TargetReplicatedJobs: []string{constants.Node},
			Job:                  &trainer.JobScheduling{},
		}},
	}
	trainerWithClaims := utiltesting.MakeTrainJobTrainerWrapper().
		ResourceClaimsPerNode(trainer.TrainerResourceClaim{Name: "gpu", ResourceClaimTemplateName: "gpu-template"}).
		Obj()
	podClaimsPatch := trainer.RuntimePatch{
		Manager: "test.io/manager",
		TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
			Template: &trainer.JobSetTemplatePatch{
				Spec: &trainer.JobSetSpecPatch{
					ReplicatedJobs: []trainer.ReplicatedJobPatch{{
						Name: constants.Node,
						Template: &trainer.JobTemplatePatch{
							Spec: &trainer.JobSpecPatch{
								Template: &trainer.PodTemplatePatch{
									Spec: &trainer.PodSpecPatch{
										ResourceClaims: []corev1.PodResourceClaim{{
											Name:                      "gpu",
											ResourceClaimTemplateName: ptr.To("gpu-template"),
										}},
									},
								},
							},
						},
					}},
				},
			},
		},
	}
	nodeSelectorPatch := trainer.RuntimePatch{
		Manager: "test.io/manager",
		TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
			Template: &trainer.JobSetTemplatePatch{
				Spec: &trainer.JobSetSpecPatch{
					ReplicatedJobs: []trainer.ReplicatedJobPatch{{
						Name: constants.Node,
						Template: &trainer.JobTemplatePatch{
							Spec: &trainer.JobSpecPatch{
								Template: &trainer.PodTemplatePatch{
									Spec: &trainer.PodSpecPatch{
										NodeSelector: map[string]string{"accelerator": "tpu"},
									},
								},
							},
						},
					}},
				},
			},
		},
	}
	// newClaimsPatch returns a RuntimePatch from the given manager which sets Pod resourceClaims
	// on the given ReplicatedJob.
	newClaimsPatch := func(manager, rJobName string) trainer.RuntimePatch {
		patch := *podClaimsPatch.DeepCopy()
		patch.Manager = manager
		patch.TrainingRuntimeSpec.Template.Spec.ReplicatedJobs[0].Name = rJobName
		return patch
	}
	podClaimsPathAt := func(patchIdx, rJobIdx int) *field.Path {
		return field.NewPath("spec", "runtimePatches").Index(patchIdx).
			Child("trainingRuntimeSpec", "template", "spec", "replicatedJobs").Index(rJobIdx).
			Child("template", "spec", "template", "spec", "resourceClaims")
	}
	trainerClaimsPath := field.NewPath("spec", "trainer", "resourceClaimsPerNode")
	patchClaimsPath := field.NewPath("spec", "runtimePatches").Index(0).
		Child("trainingRuntimeSpec", "template", "spec", "replicatedJobs").Index(0).
		Child("template", "spec", "template", "spec", "resourceClaims")

	cases := map[string]struct {
		info      *runtime.Info
		oldObj    *trainer.TrainJob
		newObj    *trainer.TrainJob
		wantError field.ErrorList
	}{
		"no action when info is nil": {
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
		"no action when the runtime does not set scheduling": {
			info: runtime.NewInfo(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
		},
		"resource claims are allowed when the runtime sets Job-level scheduling without resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
		},
		"runtimePatches without resource claims are allowed when the runtime sets Job-level resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{nodeSelectorPatch}).
				Obj(),
		},
		"resourceClaimsPerNode is rejected when the runtime sets Job-level resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(trainerClaimsPath, ""),
			},
		},
		"runtimePatches Pod resourceClaims are rejected when the runtime sets Job-level resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(patchClaimsPath, ""),
			},
		},
		"update is allowed when the resource claims were already set on the old TrainJob": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				Suspend(true).
				Obj(),
		},
		"update adding runtimePatches Pod resourceClaims is rejected when the runtime sets Job-level resourceClaims": {
			info:   runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(patchClaimsPath, ""),
			},
		},
		"update reordering runtimePatches rejects only the added Pod resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{newClaimsPatch("existing.io/manager", constants.Node)}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{
					newClaimsPatch("added.io/manager", constants.Node),
					newClaimsPatch("existing.io/manager", constants.Node),
				}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(podClaimsPathAt(0, 0), ""),
			},
		},
		"update adding Pod resourceClaims to an existing runtimePatch is rejected": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{nodeSelectorPatch}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{newClaimsPatch(nodeSelectorPatch.Manager, constants.Node)}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(podClaimsPathAt(0, 0), ""),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := New(context.Background(), nil, nil, nil)
			if err != nil {
				t.Fatalf("Failed to initialize Workload plugin: %v", err)
			}
			_, gotError := p.(*Workload).Validate(context.Background(), tc.info, tc.oldObj, tc.newObj)
			if diff := cmp.Diff(tc.wantError, gotError, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("Unexpected error from Validate (-want, +got): %s", diff)
			}
		})
	}
}

func TestWorkloadName(t *testing.T) {
	cases := map[string]struct {
		trainJob *trainer.TrainJob
		wantName string
	}{
		"TrainJob name is used as the prefix": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			wantName: "test-job-e2577dd78b",
		},
		"long TrainJob name is truncated": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, strings.Repeat("a", 63)).UID("uid").Obj(),
			wantName: strings.Repeat("a", 52) + "-e2577dd78b",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := workloadName(tc.trainJob); got != tc.wantName {
				t.Errorf("Unexpected Workload name: got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestPodGroupTemplateName(t *testing.T) {
	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj()
	longTargets := []string{strings.Repeat("a", 40), strings.Repeat("b", 40)}
	cases := map[string]struct {
		scheduling *trainer.TrainingRuntimeScheduling
		rJobName   string
		wantName   string
		wantOK     bool
	}{
		"TrainJob-level scheduling uses the TrainJob name": {
			scheduling: &trainer.TrainingRuntimeScheduling{},
			rJobName:   constants.Node,
			wantName:   "test-job",
			wantOK:     true,
		},
		"per-ReplicatedJob scheduling joins the entry index and targets": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node}},
				},
			},
			rJobName: constants.ModelInitializer,
			wantName: "0-dataset-initializer-model-initializer",
			wantOK:   true,
		},
		"per-ReplicatedJob scheduling with a single target": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node}},
				},
			},
			rJobName: constants.Node,
			wantName: "1-node",
			wantOK:   true,
		},
		"long per-ReplicatedJob name is truncated": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{{TargetReplicatedJobs: longTargets}},
			},
			rJobName: longTargets[1],
			wantName: "0-" + strings.Repeat("a", 40) + "-" + strings.Repeat("b", 9) + "-4dced6bb4a",
			wantOK:   true,
		},
		"Job-level scheduling uses the ReplicatedJob name and Job index": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{}},
				},
			},
			rJobName: constants.Node,
			wantName: "node-0",
			wantOK:   true,
		},
		"untargeted ReplicatedJob has no PodGroupTemplate": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.Node}},
				},
			},
			rJobName: constants.Launcher,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotName, gotOK := podGroupTemplateName(trainJob, tc.scheduling, tc.rJobName)
			if gotName != tc.wantName || gotOK != tc.wantOK {
				t.Errorf("Unexpected PodGroupTemplate name: got (%q, %v), want (%q, %v)", gotName, gotOK, tc.wantName, tc.wantOK)
			}
		})
	}
}

func TestPodGroupName(t *testing.T) {
	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj()
	cases := map[string]struct {
		templateName string
		wantName     string
	}{
		"TrainJob-level PodGroupTemplate omits the redundant template segment": {
			templateName: "test-job",
			wantName:     "test-job-e2577dd78b-b6ff97ef4e",
		},
		"per-ReplicatedJob PodGroupTemplate is appended to the Workload name": {
			templateName: "1-node",
			wantName:     "test-job-e2577dd78b-1-node-cc90ebe740",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := podGroupName(trainJob, tc.templateName); got != tc.wantName {
				t.Errorf("Unexpected PodGroup name: got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestHashedName(t *testing.T) {
	cases := map[string]struct {
		prefix   string
		hash     string
		wantName string
	}{
		"hash is appended to the prefix": {
			prefix:   "test-job",
			hash:     "0123456789",
			wantName: "test-job-0123456789",
		},
		"long prefix is truncated to fit into a DNS label": {
			prefix:   strings.Repeat("a", 63),
			hash:     "0123456789",
			wantName: strings.Repeat("a", 52) + "-0123456789",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := hashedName(tc.prefix, tc.hash); got != tc.wantName {
				t.Errorf("Unexpected hashed name: got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestNameHash(t *testing.T) {
	if got, want := nameHash("uid"), "e2577dd78b"; got != want {
		t.Errorf("Unexpected name hash: got %q, want %q", got, want)
	}
}
