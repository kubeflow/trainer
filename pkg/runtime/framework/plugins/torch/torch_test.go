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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestTorch(t *testing.T) {
	cases := map[string]struct {
		info              *runtime.Info
		trainJob          *trainer.TrainJob
		wantInfo          *runtime.Info
		wantMLPolicyError error
	}{
		"no action when info is nil": {},
		"no action when mlPolicySource is nil": {
			info: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
		},
		"no action when mlPolicySource torch is null": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().Obj(),
				),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().Obj(),
				),
			),
		},
		"trainJob numNodes is respected rather than mlPolicy one": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(
						corev1ac.Container().WithName(constants.Node),
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
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](2),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("trainJob-node-0-0.trainJob"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, nil).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"), // Low CPU limit
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("3"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2.5"), // 2.5 CPU cores
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("0.7"), // 0.7 CPU cores
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							"example.com/gpu": resource.MustParse("2"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("8"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2500m"), // 2.5 CPU cores in millicore format
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("test-job-node-0-0.test-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("4"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("cpu-job-node-0-0.cpu-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("6"),
							"example.com/gpu":  resource.MustParse("2"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("cpu-gpu-job-node-0-0.cpu-gpu-job"),
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
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("3.7"),
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("cpu-frac-job-node-0-0.cpu-frac-job"),
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
						Container("pytorch/pytorch:2.0.0-cuda11.7-cudnn8-runtime", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("16Gi"),
							"example.com/gpu":     resource.MustParse("4"), // 4 GPUs per node
						}).
						Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithLabels(map[string]string{
					"app": "pytorch-training",
					"env": "production",
				}),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 2, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels: map[string]string{
					"app": "pytorch-training",
					"env": "production",
				},
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](4),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
									Value: ptr.To("gpu-job-node-0-0.gpu-job"),
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
		"multi-devices full fine-tuning with torchtune": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "torchtune-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						NumProcPerNode(intstr.FromString("auto")).
						Container(
							"ghcr.io/kubeflow/trainer/torchtune-trainer",
							[]string{"tune", "run"},
							[]string{
								"dtype=fp16",
								"batch_size=32",
								"epochs=10",
								"loss=torchtune.modules.loss.CEWithChunkedOutputLoss",
							},
							corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("8"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
								"example.com/gpu":     resource.MustParse("4"), // 4 GPUs per node
							},
						).
						Obj(),
				).
				RuntimeRef(
					trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind),
					"torchtune-llama3.2-1b",
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
							},
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"single-device full fine-tuning with torchtune": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "torchtune-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						NumProcPerNode(intstr.FromInt32(1)).
						Container(
							"ghcr.io/kubeflow/trainer/torchtune-trainer",
							[]string{"tune", "run"},
							[]string{
								"dtype=fp16",
								"batch_size=32",
								"epochs=10",
								"loss=torchtune.modules.loss.CEWithChunkedOutputLoss",
							},
							corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("8"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
								"example.com/gpu":     resource.MustParse("1"), // 1 GPU per node
							},
						).
						Obj(),
				).
				RuntimeRef(
					trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind),
					"torchtune-llama3.2-1b",
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](1),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
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
							},
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"multi-nodes full fine-tuning with torchtune": {
			trainJob: utiltesting.MakeTrainJobWrapper("default", "torchtune-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						NumProcPerNode(intstr.FromInt32(8)).
						Container(
							"ghcr.io/kubeflow/trainer/torchtune-trainer",
							[]string{"tune", "run"},
							[]string{
								"dtype=fp16",
								"batch_size=32",
								"epochs=10",
								"loss=torchtune.modules.loss.CEWithChunkedOutputLoss",
							},
							corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("8"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
								"example.com/gpu":     resource.MustParse("8"), // 8 GPUs per node
							},
						).
						Obj(),
				).
				RuntimeRef(
					trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind),
					"torchtune-llama3.3-70b",
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							TorchPolicy(ptr.To(intstr.FromString("auto"))).
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 2, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](2),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.TorchEnvNumNodes),
									Value: ptr.To("2"),
								},
								{
									Name:  ptr.To(constants.TorchEnvNumProcPerNode),
									Value: ptr.To("8"),
								},
								{
									Name: ptr.To(constants.TorchEnvNodeRank),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
							},
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
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

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		info         *runtime.Info
		oldObj       *trainer.TrainJob
		newObj       *trainer.TrainJob
		wantError    field.ErrorList
		wantWarnings admission.Warnings
	}{
		"no action when info is nil": {
			info: nil,
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"no action when info does not have MLPolicySource": {
			info: runtime.NewInfo(),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"no action when info has MLPolicySource but no Torch policy": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().Obj()),
			),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"no action when info does not have numProcPerNode": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(nil).
						Obj(),
					).
					Obj(),
				),
			),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Obj(),
				).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Obj(),
				).
				Obj(),
		},
		"numProcPerNode is string and invalid": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("npu"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("npu")).
					Obj(),
				).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("trainer").Child("numProcPerNode"),
					intstr.FromString("npu"),
					fmt.Sprintf("must have an int value or %v", []string{"auto", "cpu", "gpu"}),
				),
			},
		},
		"numProcPerNode is valid string": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("auto")).
					Obj(),
				).
				Obj(),
		},
		"valid environment variable present": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Env(
						[]corev1.EnvVar{
							{
								Name:  "test",
								Value: "value",
							},
							{
								Name:  "test2",
								Value: "value",
							},
						}...,
					).
					Obj(),
				).
				Obj(),
		},
		"reserved environment variable present": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("auto")).
					Env(
						[]corev1.EnvVar{
							{
								Name:  "test",
								Value: "value",
							},
							{
								Name:  constants.TorchEnvNumProcPerNode,
								Value: "value",
							},
						}...,
					).
					Obj(),
				).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("trainer").Child("env"),
					[]corev1.EnvVar{
						{
							Name:  "test",
							Value: "value",
						},
						{
							Name:  constants.TorchEnvNumProcPerNode,
							Value: "value",
						},
					},
					fmt.Sprintf("must not have reserved envs, invalid envs configured: %v", func() []string {
						torchEnvs := sets.New[string]()
						torchEnvs.Insert(constants.TorchEnvNumProcPerNode)
						return sets.List(torchEnvs)
					}()),
				),
			},
		},
		"unsupported pretrained model": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("auto")).
					NumNodes(int32(1)).
					Container(
						"ghcr.io/kubeflow/trainer/torchtune-trainer",
						[]string{"tune", "run"},
						nil, corev1.ResourceList{},
					).
					Obj(),
				).
				RuntimeRef(
					trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind),
					"torchtune-llama3.1-70b",
				).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("runtimeRef").Child("name"),
					"torchtune-llama3.1-70b",
					fmt.Sprintf("must have a supported pretrained model, invalid model configured: %s", "llama3_1/70B"),
				),
			},
		},
		"multi-node mode is only enabled for Llama-3.3-70b": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("auto")).
					NumNodes(int32(2)).
					Container(
						"ghcr.io/kubeflow/trainer/torchtune-trainer",
						[]string{"tune", "run"},
						nil, corev1.ResourceList{},
					).
					Obj(),
				).
				RuntimeRef(
					trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind),
					"torchtune-llama3.2-3b",
				).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("trainer").Child("numNodes"),
					int32(2),
					fmt.Sprintf("must be 1 for %v model", "llama3_2/3B"),
				),
			},
		},
		"qlora is not supported for qwen in torchtune": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("auto")).
					NumNodes(int32(1)).
					Container(
						"ghcr.io/kubeflow/trainer/torchtune-trainer",
						[]string{"tune", "run"},
						[]string{
							"model.lora_attn_modules=[q_proj, v_proj, output_proj]",
							"model.quantize_base=True",
						},
						corev1.ResourceList{},
					).
					Obj(),
				).
				RuntimeRef(
					trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind),
					"torchtune-qwen2.5-1.5b",
				).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("runtimeRef").Child("name"),
					"torchtune-qwen2.5-1.5b",
					fmt.Sprintf("QLoRA is not supported for %v model", "qwen2_5/1.5B"),
				),
			},
		},
		"muti-devices qlora is not supported for llama3.2-1b in torchtune": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						TorchPolicy(ptr.To(intstr.FromString("auto"))).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("auto")).
					NumNodes(int32(1)).
					Container(
						"ghcr.io/kubeflow/trainer/torchtune-trainer",
						[]string{"tune", "run"},
						[]string{
							"model.lora_attn_modules=[q_proj, v_proj, output_proj]",
							"model.quantize_base=True",
						},
						corev1.ResourceList{
							"example.com/gpu": resource.MustParse("2"),
						},
					).
					Obj(),
				).
				RuntimeRef(
					trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind),
					"torchtune-llama3.2-1b",
				).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("trainer").Child("numProcPerNode"),
					intstr.FromString("auto"),
					fmt.Sprintf("must be auto or 1 for %v model when using QLoRA", "llama3_2/1B"),
				),
				field.Invalid(
					field.NewPath("spec").Child("trainer").Child("resourcesPerNode"),
					&corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"example.com/gpu": resource.MustParse("2"),
						},
					},
					fmt.Sprintf("must have gpu resource with value 1 for %v model when using QLoRA", "llama3_2/1B"),
				),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			p, err := New(ctx, utiltesting.NewClientBuilder().Build(), nil)
			if err != nil {
				t.Fatalf("Failed to initialize Torch plugin: %v", err)
			}
			warnings, errs := p.(framework.CustomValidationPlugin).Validate(ctx, tc.info, tc.oldObj, tc.newObj)
			if diff := cmp.Diff(tc.wantError, errs); len(diff) != 0 {
				t.Errorf("Unexpected error from Validate (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantWarnings, warnings); len(diff) != 0 {
				t.Errorf("Unexpected warnings from Validate (-want, +got): %s", diff)
			}
		})
	}
}
