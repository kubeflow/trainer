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

package trainjob

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestRuntimeRefIsTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		ref  trainer.RuntimeRef
		want bool
	}{
		"runtimeRef is TrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: true,
		},
		"runtimeRef is not TrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has wrong APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: ptr.To("other.group.io"),
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: nil,
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil Kind": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     nil,
			},
			want: false,
		},
		"runtimeRef has both nil": {
			ref:  trainer.RuntimeRef{},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RuntimeRefIsTrainingRuntime(tc.ref)
			if got != tc.want {
				t.Errorf("RuntimeRefIsTrainingRuntime(%v) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestRuntimeRefIsClusterTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		ref  trainer.RuntimeRef
		want bool
	}{
		"runtimeRef is ClusterTrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: true,
		},
		"runtimeRef is TrainingRuntime not ClusterTrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has wrong APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: ptr.To("other.group.io"),
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: nil,
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil Kind": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     nil,
			},
			want: false,
		},
		"runtimeRef has both nil": {
			ref:  trainer.RuntimeRef{},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RuntimeRefIsClusterTrainingRuntime(tc.ref)
			if got != tc.want {
				t.Errorf("RuntimeRefIsClusterTrainingRuntime(%v) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestIsManagedByExternalController(t *testing.T) {
	cases := map[string]struct {
		trainJob *trainer.TrainJob
		want     bool
	}{
		"managedBy is unset": {
			trainJob: &trainer.TrainJob{},
			want:     false,
		},
		"managedBy is the built-in TrainJob controller": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ManagedBy: ptr.To(TrainJobControllerName),
				},
			},
			want: false,
		},
		"managedBy is MultiKueue": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ManagedBy: ptr.To("kueue.x-k8s.io/multikueue"),
				},
			},
			want: true,
		},
		"managedBy is an unrecognized controller": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ManagedBy: ptr.To("foo"),
				},
			},
			want: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsManagedByExternalController(tc.trainJob)
			if got != tc.want {
				t.Errorf("IsManagedByExternalController(%v) = %v, want %v", tc.trainJob, got, tc.want)
			}
		})
	}
}

func TestIsTrainJobFinished(t *testing.T) {
	cases := map[string]struct {
		trainJob *trainer.TrainJob
		want     bool
	}{
		"completed TrainJob is finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobComplete,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		"failed TrainJob is finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobFailed,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		"running TrainJob with no conditions is not finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{},
				},
			},
			want: false,
		},
		"TrainJob with Complete=False is not finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobComplete,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		"TrainJob with Failed=False is not finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobFailed,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		"TrainJob with both Complete and Failed true is finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobComplete,
							Status: metav1.ConditionTrue,
						},
						{
							Type:   trainer.TrainJobFailed,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsTrainJobFinished(tc.trainJob)
			if got != tc.want {
				t.Errorf("IsTrainJobFinished(%v) = %v, want %v", tc.trainJob, got, tc.want)
			}
		})
	}
}

func TestResourceClaimsPaths(t *testing.T) {
	cases := map[string]struct {
		trainJob  *trainer.TrainJob
		wantPaths []string
	}{
		"TrainJob without resource claims": {
			trainJob: &trainer.TrainJob{},
		},
		"resourceClaimsPerNode is set": {
			trainJob: &trainer.TrainJob{Spec: trainer.TrainJobSpec{
				Trainer: &trainer.Trainer{
					ResourceClaimsPerNode: []trainer.TrainerResourceClaim{{Name: "gpu", ResourceClaimTemplateName: "gpu-template"}},
				},
			}},
			wantPaths: []string{"spec.trainer.resourceClaimsPerNode"},
		},
		"runtimePatches set Pod resourceClaims": {
			trainJob: &trainer.TrainJob{Spec: trainer.TrainJobSpec{
				RuntimePatches: []trainer.RuntimePatch{
					{Manager: "first.io/manager"},
					runtimePatch("second.io/manager", trainer.PodSpecPatch{}, trainer.PodSpecPatch{
						ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimTemplateName: ptr.To("gpu-template")}},
					}),
				},
			}},
			wantPaths: []string{"spec.runtimePatches[1].trainingRuntimeSpec.template.spec.replicatedJobs[1].template.spec.template.spec.resourceClaims"},
		},
		"runtimePatches only set container resources.claims": {
			trainJob: &trainer.TrainJob{Spec: trainer.TrainJobSpec{
				RuntimePatches: []trainer.RuntimePatch{
					runtimePatch("first.io/manager", trainer.PodSpecPatch{
						Containers: []trainer.ContainerPatch{{Name: "node", Resources: &corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu"}}}}},
					}),
				},
			}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.wantPaths, pathStrings(ResourceClaimsPaths(tc.trainJob))); len(diff) != 0 {
				t.Errorf("Unexpected ResourceClaimsPaths (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestContainerResourceClaimsPaths(t *testing.T) {
	cases := map[string]struct {
		trainJob  *trainer.TrainJob
		wantPaths []string
	}{
		"TrainJob without resource claims": {
			trainJob: &trainer.TrainJob{},
		},
		"runtimePatches only set Pod resourceClaims": {
			trainJob: &trainer.TrainJob{Spec: trainer.TrainJobSpec{
				RuntimePatches: []trainer.RuntimePatch{
					runtimePatch("first.io/manager", trainer.PodSpecPatch{
						ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimTemplateName: ptr.To("gpu-template")}},
					}),
				},
			}},
		},
		"runtimePatches set container and initContainer resources.claims": {
			trainJob: &trainer.TrainJob{Spec: trainer.TrainJobSpec{
				RuntimePatches: []trainer.RuntimePatch{
					runtimePatch("first.io/manager", trainer.PodSpecPatch{
						Containers: []trainer.ContainerPatch{
							{Name: "sidecar"},
							{Name: "node", Resources: &corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu"}}}},
						},
						InitContainers: []trainer.ContainerPatch{
							{Name: "init", Resources: &corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu"}}}},
						},
					}),
				},
			}},
			wantPaths: []string{
				"spec.runtimePatches[0].trainingRuntimeSpec.template.spec.replicatedJobs[0].template.spec.template.spec.containers[1].resources.claims",
				"spec.runtimePatches[0].trainingRuntimeSpec.template.spec.replicatedJobs[0].template.spec.template.spec.initContainers[0].resources.claims",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.wantPaths, pathStrings(ContainerResourceClaimsPaths(tc.trainJob))); len(diff) != 0 {
				t.Errorf("Unexpected ContainerResourceClaimsPaths (-want,+got):\n%s", diff)
			}
		})
	}
}

// runtimePatch returns a RuntimePatch which patches one ReplicatedJob Pod spec per given podSpec.
func runtimePatch(manager string, podSpecs ...trainer.PodSpecPatch) trainer.RuntimePatch {
	var rJobs []trainer.ReplicatedJobPatch
	for _, podSpec := range podSpecs {
		rJobs = append(rJobs, trainer.ReplicatedJobPatch{
			Name: "node",
			Template: &trainer.JobTemplatePatch{
				Spec: &trainer.JobSpecPatch{
					Template: &trainer.PodTemplatePatch{Spec: &podSpec},
				},
			},
		})
	}
	return trainer.RuntimePatch{
		Manager: manager,
		TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
			Template: &trainer.JobSetTemplatePatch{
				Spec: &trainer.JobSetSpecPatch{ReplicatedJobs: rJobs},
			},
		},
	}
}

func pathStrings(paths []*field.Path) []string {
	var out []string
	for _, path := range paths {
		out = append(out, path.String())
	}
	return out
}

func TestForEachRuntimePatchPodSpec(t *testing.T) {
	type visit struct {
		Path     string
		Manager  string
		RJobName string
	}
	trainJob := &trainer.TrainJob{Spec: trainer.TrainJobSpec{
		RuntimePatches: []trainer.RuntimePatch{
			{Manager: "first.io/manager"},
			runtimePatch("second.io/manager", trainer.PodSpecPatch{}),
			{
				Manager: "third.io/manager",
				TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
					Template: &trainer.JobSetTemplatePatch{
						Spec: &trainer.JobSetSpecPatch{
							ReplicatedJobs: []trainer.ReplicatedJobPatch{{Name: "launcher"}},
						},
					},
				},
			},
		},
	}}
	wantVisits := []visit{{
		Path:     "spec.runtimePatches[1].trainingRuntimeSpec.template.spec.replicatedJobs[0].template.spec.template.spec",
		Manager:  "second.io/manager",
		RJobName: "node",
	}}

	var gotVisits []visit
	ForEachRuntimePatchPodSpec(trainJob, func(podSpecPath *field.Path, manager, rJobName string, _ *trainer.PodSpecPatch) {
		gotVisits = append(gotVisits, visit{Path: podSpecPath.String(), Manager: manager, RJobName: rJobName})
	})
	if diff := cmp.Diff(wantVisits, gotVisits); len(diff) != 0 {
		t.Errorf("Unexpected ForEachRuntimePatchPodSpec visits (-want,+got):\n%s", diff)
	}
}
