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

package jobset

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	v1 "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// TODO: Add tests for all Interfaces.
// REF: https://github.com/kubeflow/trainer/issues/2468

func TestJobSet(t *testing.T) {
	cases := map[string]struct {
		trainJob  *trainer.TrainJob
		info      *runtime.Info
		wantInfo  *runtime.Info
		wantError error
	}{
		"no action when info is nil": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Obj(),
		},
		"no action when trainJob is not nil": {
			info: &runtime.Info{
				Labels: map[string]string{"key": "value"},
			},
			wantInfo: &runtime.Info{
				Labels: map[string]string{"key": "value"},
			},
		},
		"no action when template.spec is not JobSet": {
			info: &runtime.Info{
				Labels: map[string]string{"key": "value"},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: batchv1ac.JobSpec(),
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Obj(),
			wantInfo: &runtime.Info{
				Labels: map[string]string{"key": "value"},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: batchv1ac.JobSpec(),
				},
			},
		},
		"trainer numNodes is respected rather than parallelism when replicatedJob name is node": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(nil, ptr.To(trainer.MPIImplementationOpenMPI), nil, nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Containers: make([]runtime.Container, 1),
						},
						{
							Name:       constants.Node,
							Count:      ptr.To[int32](2),
							Containers: make([]runtime.Container, 1),
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName("sidecar"),
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(2).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
						),
				},
			},
			wantInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(nil, ptr.To(trainer.MPIImplementationOpenMPI), nil, nil).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Containers: make([]runtime.Container, 1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:       constants.Node,
							Count:      ptr.To[int32](2),
							Containers: make([]runtime.Container, 1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-0-0.trainJob")
								yield("trainJob-node-0-1.trainJob")
							},
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName("sidecar"),
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(2).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
						),
				},
			},
		},
		"subDomain in jobSetSpec is used to endpoint": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Containers: make([]runtime.Container, 1),
						},
						{
							Name:       constants.Node,
							Containers: make([]runtime.Container, 1),
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithNetwork(jobsetv1alpha2ac.Network().
							WithSubdomain("kubeflow.org")).
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
						),
				},
			},
			wantInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Containers: make([]runtime.Container, 1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.kubeflow.org")
							},
						},
						{
							Name:       constants.Node,
							Containers: make([]runtime.Container, 1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-0-0.kubeflow.org")
							},
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithNetwork(jobsetv1alpha2ac.Network().
							WithSubdomain("kubeflow.org")).
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
						),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cli := utiltesting.NewClientBuilder().Build()
			p, err := New(ctx, cli, nil)
			if err != nil {
				t.Fatalf("Failed to initialize JobSet plugin: %v", err)
			}
			err = p.(framework.PodNetworkPlugin).IdentifyPodNetwork(tc.info, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantInfo, tc.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b string) bool { return a < b }),
				utiltesting.PodSetEndpointsCmpOpts,
			); len(diff) != 0 {
				t.Errorf("Unexpected Info from IdentifyPodNetwork (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		info         *runtime.Info
		newObj       *trainer.TrainJob
		wantError    field.ErrorList
		wantWarnings admission.Warnings
	}{
		"no initializer job": {
			info: &runtime.Info{TemplateSpec: runtime.TemplateSpec{
				ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{},
			}},
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Initializer(nil).
				Obj(),
		},
		"no dataset initializer job": {
			info: &runtime.Info{TemplateSpec: runtime.TemplateSpec{
				ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{},
			}},
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Initializer(&trainer.Initializer{Dataset: nil}).
				Obj(),
		},
		"must have dataset initializer job when trainJob is configured with input datasetConfig": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To("random"),
								Template: &v1.JobTemplateSpecApplyConfiguration{
									Spec: &v1.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To("random"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newObj: utiltesting.MakeTrainJobWrapper("default", "test").
				Initializer(&trainer.Initializer{
					Dataset: &trainer.DatasetInitializer{},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(runtimeRefPath,
					utiltesting.MakeTrainJobWrapper("default", "test").Obj().Spec.RuntimeRef,
					fmt.Sprintf("must have %s job when trainJob is configured with input datasetConfig", constants.DatasetInitializer)),
			},
		},
		"must have container with name - dataset initializer in the dataset initializer job": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.DatasetInitializer),
								Template: &v1.JobTemplateSpecApplyConfiguration{
									Spec: &v1.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newObj: utiltesting.MakeTrainJobWrapper("default", "test").
				Initializer(&trainer.Initializer{
					Dataset: &trainer.DatasetInitializer{},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(runtimeRefPath,
					utiltesting.MakeTrainJobWrapper("default", "test").Obj().Spec.RuntimeRef,
					fmt.Sprintf("must have container with name - %s in the %s job", constants.DatasetInitializer, constants.DatasetInitializer)),
			},
		},
		"no model initializer job": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.DatasetInitializer),
								Template: &v1.JobTemplateSpecApplyConfiguration{
									Spec: &v1.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.DatasetInitializer),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Initializer(&trainer.Initializer{Dataset: nil}).
				Obj(),
		},
		"must have model initializer job when trainJob is configured with input modelConfig": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To("random"),
								Template: &v1.JobTemplateSpecApplyConfiguration{
									Spec: &v1.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To("random"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newObj: utiltesting.MakeTrainJobWrapper("default", "test").
				Initializer(&trainer.Initializer{
					Model: &trainer.ModelInitializer{},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(runtimeRefPath,
					utiltesting.MakeTrainJobWrapper("default", "test").Obj().Spec.RuntimeRef,
					fmt.Sprintf("must have %s job when trainJob is configured with input modelConfig", constants.ModelInitializer)),
			},
		},
		"must have container with name - model initializer in the model initializer job": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.ModelInitializer),
								Template: &v1.JobTemplateSpecApplyConfiguration{
									Spec: &v1.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newObj: utiltesting.MakeTrainJobWrapper("default", "test").
				Initializer(&trainer.Initializer{
					Model: &trainer.ModelInitializer{},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(runtimeRefPath,
					utiltesting.MakeTrainJobWrapper("default", "test").Obj().Spec.RuntimeRef,
					fmt.Sprintf("must have container with name - %s in the %s job", constants.ModelInitializer, constants.ModelInitializer)),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cli := utiltesting.NewClientBuilder().Build()
			p, err := New(ctx, cli, nil)
			if err != nil {
				t.Fatalf("Failed to initialize JobSet plugin: %v", err)
			}
			warnings, errs := p.(framework.CustomValidationPlugin).Validate(tc.info, nil, tc.newObj)
			if diff := cmp.Diff(tc.wantError, errs); len(diff) != 0 {
				t.Errorf("Unexpected error from Validate (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantWarnings, warnings); len(diff) != 0 {
				t.Errorf("Unexpected warnings from Validate (-want, +got): %s", diff)
			}
		})
	}
}

func TestTrainer(t *testing.T) {
	cases := map[string]struct {
		trainJob  *trainer.TrainJob
		info      *runtime.Info
		wantInfo  *runtime.Info
		wantError error
	}{
		"resources per node should be applied to both launcher and node pods": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("5"),
							corev1.ResourceMemory: resource.MustParse("16Gi"),
							"nvidia.com/gpu":      resource.MustParse("1"),
						}).
						Obj(),
				).
				Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(
							ptr.To[int32](2),                         // numNodes
							ptr.To(trainer.MPIImplementationOpenMPI), // implementation
							nil,                                      // customArgs
							ptr.To(true),                             // runLauncherAsNode
						).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Count:      ptr.To[int32](1),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.AncestorTrainer),
						},
						{
							Name:       constants.Node,
							Count:      ptr.To[int32](2),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.Node),
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Launcher),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.Node,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node),
												),
											),
										),
									),
								),
						),
				},
			},
			wantInfo: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Count:      ptr.To[int32](1),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.AncestorTrainer),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:       constants.Node,
							Count:      ptr.To[int32](2),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.Node),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-0-0.trainJob")
								yield("trainJob-node-0-1.trainJob")
							},
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithReplicas(1).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Launcher).
														WithImage("test:image").
														WithResources(corev1ac.ResourceRequirements().
															WithRequests(corev1.ResourceList{
																corev1.ResourceCPU:    resource.MustParse("5"),
																corev1.ResourceMemory: resource.MustParse("16Gi"),
																"nvidia.com/gpu":      resource.MustParse("1"),
															}),
														),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.Node,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
														WithImage("test:image").
														WithResources(corev1ac.ResourceRequirements().
															WithRequests(corev1.ResourceList{
																corev1.ResourceCPU:    resource.MustParse("5"),
																corev1.ResourceMemory: resource.MustParse("16Gi"),
																"nvidia.com/gpu":      resource.MustParse("1"),
															}),
														),
												),
											),
										),
									),
								),
						),
				},
			},
		},
		"resources should only be applied to node when runLauncherAsNode is false": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Container("test:image", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
							"nvidia.com/gpu":      resource.MustParse("1"),
						}).
						Obj(),
				).
				Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(
							ptr.To[int32](2),                         // numNodes
							ptr.To(trainer.MPIImplementationOpenMPI), // implementation
							nil,                                      // customArgs
							ptr.To(false),                            // runLauncherAsNode
						).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Count:      ptr.To[int32](1),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.AncestorTrainer),
						},
						{
							Name:       constants.Node,
							Count:      ptr.To[int32](2),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.Node),
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Launcher).
														WithImage("launcher:latest"),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.Node,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Node).
														WithImage("node:latest"),
												),
											),
										),
									),
								),
						),
				},
			},
			wantInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(
							ptr.To[int32](2),                         // numNodes
							ptr.To(trainer.MPIImplementationOpenMPI), // implementation
							nil,                                      // customArgs
							ptr.To(false),                            // runLauncherAsNode
						).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:       constants.Launcher,
							Count:      ptr.To[int32](1),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.AncestorTrainer),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:       constants.Node,
							Count:      ptr.To[int32](2),
							Containers: make([]runtime.Container, 1),
							Ancestor:   ptr.To(constants.Node),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-0-0.trainJob")
								yield("trainJob-node-0-1.trainJob")
							},
						},
					},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Launcher).
								WithReplicas(1).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithParallelism(1).
										WithCompletions(1).
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().WithName(constants.Launcher).
														WithImage("launcher:latest"),
												),
											),
										),
									),
								),
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithLabels(map[string]string{
										constants.LabelTrainJobAncestor: constants.Node,
									}).
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
														WithImage("test:image").
														WithResources(corev1ac.ResourceRequirements().
															WithRequests(corev1.ResourceList{
																corev1.ResourceCPU:    resource.MustParse("1"),
																corev1.ResourceMemory: resource.MustParse("1Gi"),
																"nvidia.com/gpu":      resource.MustParse("1"),
															}),
														),
												),
											),
										),
									),
								),
						),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cli := utiltesting.NewClientBuilder().Build()
			_, err := New(ctx, cli, nil)
			if err != nil {
				t.Fatalf("Failed to initialize JobSet plugin: %v", err)
			}

			// Ensure all required fields are initialized
			if tc.info == nil {
				tc.info = &runtime.Info{}
			}
			if tc.info.TemplateSpec.ObjApply == nil {
				tc.info.TemplateSpec.ObjApply = jobsetv1alpha2ac.JobSetSpec()
			}
			if tc.info.RuntimePolicy.MLPolicySource == nil {
				tc.info.RuntimePolicy.MLPolicySource = utiltesting.MakeMLPolicySourceWrapper().Obj()
			}

			jobSetSpec := tc.info.TemplateSpec.ObjApply.(*jobsetv1alpha2ac.JobSetSpecApplyConfiguration)
			builder := NewBuilder(jobsetv1alpha2ac.JobSet("test", "default").WithSpec(jobSetSpec))
			builder.Trainer(tc.info, tc.trainJob)
			if diff := cmp.Diff(tc.wantInfo.TemplateSpec.ObjApply, builder.Build().Spec,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b string) bool { return a < b }),
			); len(diff) != 0 {
				t.Errorf("Unexpected JobSet from Trainer (-want,+got):\n%s", diff)
			}
		})
	}
}
