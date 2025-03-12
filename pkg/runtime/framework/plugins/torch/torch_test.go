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

package torch

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestTorch(t *testing.T) {
	cases := map[string]struct {
		info               *runtime.Info
		trainJob           *trainer.TrainJob
		wantInfo           *runtime.Info
		wantMLPolicyError  error
		wantNumProcPerNode string // For validating numProcPerNode value
	}{
		"no action when info is nil": {},
		"no action when mlPolicy is nil": {
			info: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
		},
		"no action when mlPolicy torch is null": {
			info: runtime.NewInfo(
				runtime.WithMLPolicy(utiltesting.MakeMLPolicyWrapper().
					Obj()),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithMLPolicy(utiltesting.MakeMLPolicyWrapper().
					Obj()),
			),
		},
		"trainJob numNodes is respected rather than mlPolicy one": {
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				Trainer: runtime.Trainer{
					NumNodes:       ptr.To[int32](2),
					NumProcPerNode: "",
					Env: []corev1ac.EnvVarApplyConfiguration{
						{
							Name:  ptr.To(constants.TorchEnvNumNodes),
							Value: ptr.To("2"),
						},
						{
							Name:  ptr.To(constants.TorchEnvNumProcPerNode),
							Value: ptr.To("auto"),
						},
						{
							Name: ptr.To(constants.TorchEnvNodeRank),
							ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
								FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
									FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
								},
							},
						},
						{
							Name:  ptr.To(constants.TorchEnvMasterAddr),
							Value: ptr.To("trainJob-trainer-node-0-0.trainJob"),
						},
						{
							Name:  ptr.To(constants.TorchEnvMasterPort),
							Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
						},
					},
					ContainerPort: &corev1ac.ContainerPortApplyConfiguration{
						ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
					},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: map[string]runtime.TotalResourceRequest{}},
			},
		},
		// Issue #2407 test case - nproc_per_node=auto with CPU limit
		"nproc_per_node=auto with CPU limit": {
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
			wantNumProcPerNode: "4", // Should be capped to CPU limit
		},
		// Issue #2407 test case - nproc_per_node=auto with no CPU resources
		"nproc_per_node=auto with no CPU resources": {
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
			wantNumProcPerNode: "1", // Default to 1 when no CPU resources specified
		},
		// Issue #2407 test case - nproc_per_node=auto with low CPU limit
		"nproc_per_node=auto with low CPU limit": {
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
			wantNumProcPerNode: "2", // Should be capped to CPU limit (2) even if actual CPU count is higher
		},
		// Issue #2407 test case - nproc_per_node=auto with CPU request but no limit
		"nproc_per_node=auto with CPU request but no limit": {
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
			wantNumProcPerNode: "3", // Should use CPU request when no limit is set
		},
		// Issue #2407 test case - nproc_per_node=auto with millicore CPU limit
		"nproc_per_node=auto with millicore CPU limit": {
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
			wantNumProcPerNode: "3", // Should round up to 3 for 2.5 cores
		},
		// Issue #2407 test case - nproc_per_node=auto with fractional CPU limit
		"nproc_per_node=auto with fractional CPU limit": {
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
			wantNumProcPerNode: "1", // Should round up to 1 for 0.7 cores
		},
		// Issue #2407 test case - nproc_per_node=auto with GPU request should remain auto
		"nproc_per_node=auto with GPU request should remain auto": {
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
			wantNumProcPerNode: "auto", // Keep auto when GPU is requested
		},
		// Issue #2407 test case - explicitly set nproc_per_node should be preserved
		"explicitly set nproc_per_node should be preserved": {
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
			wantNumProcPerNode: "3", // Explicit value should be preserved
		},
		// Issue #2407 test case - nproc_per_node=auto with millicore CPU limit in m format
		"nproc_per_node=auto with millicore CPU limit in m format": {
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
			wantNumProcPerNode: "3", // Should round up to 3 for 2500m (2.5) cores
		},
		// Test case - nproc_per_node=cpu with CPU limit
		"nproc_per_node=cpu with CPU limit": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "cpu-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("cpu")).
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
			wantNumProcPerNode: "4", // Should use CPU limit (4)
		},
		// Test case - nproc_per_node=cpu with GPU resources (should still use CPU resources)
		"nproc_per_node=cpu with GPU resources": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "cpu-gpu-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("cpu")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("6"),
							"nvidia.com/gpu":   resource.MustParse("2"),
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
			wantNumProcPerNode: "6", // Should use CPU limit (6) even with GPU resources
		},
		// Test case - nproc_per_node=cpu with fractional CPU
		"nproc_per_node=cpu with fractional CPU": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "cpu-frac-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromString("cpu")).
						Container("test:image", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("3.7"),
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
			wantNumProcPerNode: "4", // Should round up to 4 for 3.7 cores
		},
		// New test case - Complete test with multiple GPU resources
		"multi-node multi-GPU training with complete info": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "gpu-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(4).
						NumProcPerNode(intstr.FromString("auto")).
						Container("pytorch/pytorch:2.0.0-cuda11.7-cudnn8-runtime", []string{}, []string{}, corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("16Gi"),
							"nvidia.com/gpu":      resource.MustParse("4"), // 4 GPUs per node
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(2). // This value should be overridden by the value in trainJob
						TorchPolicy("auto", nil).
						Obj(),
				),
				runtime.WithLabels(map[string]string{
					"app": "pytorch-training",
					"env": "production",
				}),
			),
			wantInfo: &runtime.Info{
				Labels: map[string]string{
					"app": "pytorch-training",
					"env": "production",
				},
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(2).
						TorchPolicy("auto", nil).
						Obj(),
				},
				Trainer: runtime.Trainer{
					NumNodes:       ptr.To[int32](4),
					NumProcPerNode: "",
					Env: []corev1ac.EnvVarApplyConfiguration{
						{
							Name:  ptr.To(constants.TorchEnvNumNodes),
							Value: ptr.To("4"),
						},
						{
							Name:  ptr.To(constants.TorchEnvNumProcPerNode),
							Value: ptr.To("auto"),
						},
						{
							Name: ptr.To(constants.TorchEnvNodeRank),
							ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
								FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
									FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
								},
							},
						},
						{
							Name:  ptr.To(constants.TorchEnvMasterAddr),
							Value: ptr.To("gpu-job-trainer-node-0-0.gpu-job"),
						},
						{
							Name:  ptr.To(constants.TorchEnvMasterPort),
							Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
						},
					},
					ContainerPort: &corev1ac.ContainerPortApplyConfiguration{
						ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
					},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: map[string]runtime.TotalResourceRequest{}},
			},
			wantNumProcPerNode: "auto", // Should keep auto when GPU is present
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cliBuilder := utiltesting.NewClientBuilder()
			p, err := New(ctx, cliBuilder.Build(), nil)
			if err != nil {
				t.Fatalf("Failed to initialize Torch plugin: %v", err)
			}

			// Test EnforceMLPolicy
			err = p.(framework.EnforceMLPolicyPlugin).EnforceMLPolicy(tc.info, tc.trainJob)
			if diff := cmp.Diff(tc.wantMLPolicyError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforceMLPolicy (-want,+got):\n%s", diff)
			}

			// If need to validate numProcPerNode
			if tc.wantNumProcPerNode != "" && tc.info != nil {
				// Find PET_NPROC_PER_NODE environment variable
				var numProcPerNodeValue string
				for _, env := range tc.info.Trainer.Env {
					if env.Name != nil && *env.Name == constants.TorchEnvNumProcPerNode {
						if env.Value != nil {
							numProcPerNodeValue = *env.Value
						}
						break
					}
				}

				if diff := cmp.Diff(tc.wantNumProcPerNode, numProcPerNodeValue); diff != "" {
					t.Errorf("Torch.EnforceMLPolicy() numProcPerNode mismatch (-want +got):\n%s", diff)
				}
			}

			// Validate the entire info object (if wantInfo is provided)
			if tc.wantInfo != nil {
				if diff := cmp.Diff(tc.wantInfo, tc.info,
					cmpopts.SortSlices(func(a, b string) bool { return a < b }),
					cmpopts.SortMaps(func(a, b string) bool { return a < b }),
				); len(diff) != 0 {
					t.Errorf("Unexpected RuntimeInfo (-want,+got):\n%s", diff)
				}
			}
		})
	}
}
