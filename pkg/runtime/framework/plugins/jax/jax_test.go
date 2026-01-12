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

package jax

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestJax(t *testing.T) {
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
		"no action when mlPolicySource JAX is null": {
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
		"single node JAX training": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							JAXPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						JAXPolicy().
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
									Name:  ptr.To("JAX_NUM_PROCESSES"),
									Value: ptr.To("1"),
								},
								{
									Name: ptr.To("JAX_PROCESS_ID"),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_ADDRESS"),
									Value: ptr.To(fmt.Sprintf("test-job-%s-0-0.test-job:%d", constants.Node, constants.ContainerTrainerPort)),
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_PORT"),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"multi-node JAX training": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							JAXPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 2, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "multi-node-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(4).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						JAXPolicy().
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
									Name:  ptr.To("JAX_NUM_PROCESSES"),
									Value: ptr.To("4"),
								},
								{
									Name: ptr.To("JAX_PROCESS_ID"),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_ADDRESS"),
									Value: ptr.To(fmt.Sprintf("multi-node-job-%s-0-0.multi-node-job:%d", constants.Node, constants.ContainerTrainerPort)),
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_PORT"),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"trainJob numNodes is respected": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							JAXPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(3).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						JAXPolicy().
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{
						Name:              constants.Node,
						Ancestor:          ptr.To(constants.AncestorTrainer),
						Count:             ptr.To[int32](3),
						SinglePodRequests: make(corev1.ResourceList),
						Containers: []runtime.Container{{
							Name: constants.Node,
							Ports: []corev1ac.ContainerPortApplyConfiguration{{
								ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To("JAX_NUM_PROCESSES"),
									Value: ptr.To("3"),
								},
								{
									Name: ptr.To("JAX_PROCESS_ID"),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_ADDRESS"),
									Value: ptr.To(fmt.Sprintf("trainJob-%s-0-0.trainJob:%d", constants.Node, constants.ContainerTrainerPort)),
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_PORT"),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"JAX training with GPU resources": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							JAXPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithLabels(map[string]string{
					"app": "jax-training",
				}),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "gpu-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Container("jax/jax:latest", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("32Gi"),
							"nvidia.com/gpu":      resource.MustParse("4"),
						}).
						Obj(),
				).
				Obj(),
			wantInfo: &runtime.Info{
				Labels: map[string]string{
					"app": "jax-training",
				},
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						JAXPolicy().
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
									Name:  ptr.To("JAX_NUM_PROCESSES"),
									Value: ptr.To("2"),
								},
								{
									Name: ptr.To("JAX_PROCESS_ID"),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_ADDRESS"),
									Value: ptr.To(fmt.Sprintf("gpu-job-%s-0-0.gpu-job:%d", constants.Node, constants.ContainerTrainerPort)),
								},
								{
									Name:  ptr.To("JAX_COORDINATOR_PORT"),
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
				t.Fatalf("Failed to initialize JAX plugin: %v", err)
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
		"no action when info has MLPolicySource but no JAX policy": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().Obj()),
			),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"valid JAX configuration": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						JAXPolicy().
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumNodes(4).
					Obj(),
				).
				Obj(),
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
				t.Fatalf("Failed to initialize JAX plugin: %v", err)
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
