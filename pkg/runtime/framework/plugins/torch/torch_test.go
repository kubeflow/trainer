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
		info              *runtime.Info
		trainJob          *trainer.TrainJob
		wantInfo          *runtime.Info
		wantMLPolicyError error
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(
						corev1ac.Container().WithName(constants.ContainerTrainer),
					),
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
						WithNumNodes(2).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
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
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("4"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("1"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("2"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("3"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("3"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("1"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"explicitly set nproc_per_node should be preserved": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumProcPerNode(intstr.FromInt32(3)).
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("3"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("3"),
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
									Value: ptr.To("test-job-trainer-node-0-0.test-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("4"),
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
									Value: ptr.To("cpu-job-trainer-node-0-0.cpu-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("6"),
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
									Value: ptr.To("cpu-gpu-job-trainer-node-0-0.cpu-gpu-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("1"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("4"),
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
									Value: ptr.To("cpu-frac-job-trainer-node-0-0.cpu-frac-job"),
								},
								{
									Name:  ptr.To(constants.TorchEnvMasterPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
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
				runtime.WithPodSet(constants.JobTrainerNode, 2, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.ContainerTrainer)),
				),
			),
			wantInfo: &runtime.Info{
				Labels: map[string]string{
					"app": "pytorch-training",
					"env": "production",
				},
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(4).
						TorchPolicy("auto", nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.JobTrainerNode,
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.ContainerTrainer,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
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
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
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

			// Validate the entire info object
			if diff := cmp.Diff(tc.wantInfo, tc.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b string) bool { return a < b }),
			); len(diff) != 0 {
				t.Errorf("Unexpected RuntimeInfo (-want,+got):\n%s", diff)
			}
		})
	}
}
