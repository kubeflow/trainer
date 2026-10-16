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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

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
