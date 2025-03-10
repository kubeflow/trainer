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

// Package torch contains tests for the Torch plugin, including validation of the fix for Issue #2407
// which caps nproc_per_node based on CPU resources when set to "auto" and no GPU is requested.
package torch

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestTorch_EnforceMLPolicy_NumProcPerNode_Issue2407(t *testing.T) {
	tests := []struct {
		name     string
		trainJob *trainer.TrainJob
		info     *runtime.Info
		want     string
	}{
		{
			name: "nproc_per_node=auto with CPU limit",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "4", // Should be capped to CPU limit
		},
		{
			name: "nproc_per_node=auto with no CPU resources",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, nil).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "1", // Default to 1 when no CPU resources specified
		},
		{
			name: "nproc_per_node=auto with low CPU limit",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"), // Low CPU limit
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "2", // Should be capped to CPU limit (2) even if actual CPU count is higher
		},
		{
			name: "nproc_per_node=auto with CPU request but no limit",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("3"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "3", // Should use CPU request when no limit is set
		},
		{
			name: "nproc_per_node=auto with millicore CPU limit",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2.5"), // 2.5 CPU cores
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "3", // Should round up to 3 for 2.5 cores
		},
		{
			name: "nproc_per_node=auto with fractional CPU limit",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("0.7"), // 0.7 CPU cores
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "1", // Should round up to 1 for 0.7 cores
		},
		{
			name: "nproc_per_node=auto with GPU request should remain auto",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("2"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "auto", // Keep auto when GPU is requested
		},
		{
			name: "explicitly set nproc_per_node should be preserved",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromInt(3)).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("8"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "3", // Explicit value should be preserved
		},
		{
			name: "nproc_per_node=auto with millicore CPU limit in m format",
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("auto")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2500m"), // 2.5 CPU cores in millicore format
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			want: "3", // Should round up to 3 for 2500m (2.5) cores
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			torch := &Torch{}
			err := torch.EnforceMLPolicy(tt.info, tt.trainJob)
			if err != nil {
				t.Errorf("Torch.EnforceMLPolicy() error = %v", err)
				return
			}

			// Find the PET_NPROC_PER_NODE env var
			var numProcPerNodeValue string
			for _, env := range tt.info.Trainer.Env {
				if env.Name != nil && *env.Name == constants.TorchEnvNumProcPerNode {
					if env.Value != nil {
						numProcPerNodeValue = *env.Value
					}
					break
				}
			}

			if diff := cmp.Diff(tt.want, numProcPerNodeValue); diff != "" {
				t.Errorf("Torch.EnforceMLPolicy() numProcPerNode mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
