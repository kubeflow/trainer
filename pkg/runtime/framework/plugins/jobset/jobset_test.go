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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
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
						MPIPolicy(nil, trainer.MPIImplementationOpenMPI, nil, nil).
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
						MPIPolicy(nil, trainer.MPIImplementationOpenMPI, nil, nil).
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
		oldObj       *trainer.TrainJob
		newObj       *trainer.TrainJob
		jobSet       *jobsetv1alpha2.JobSet
		clientErr    error
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
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
		"podTemplateOverrides contain invalid targetJob": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: "invalid"}},
					},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(podTemplateOverridePath,
					[]trainer.PodTemplateOverride{
						{
							TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: "invalid"}},
						},
					},
					"must not have targetJob that doesn't exist in the runtime job template"),
			},
		},
		"podTemplateOverrides contain invalid initContainer": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												InitContainers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To("custom-init"),
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
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							InitContainers: []trainer.ContainerOverride{
								{
									Name: "invalid",
								},
							},
						},
					},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(podTemplateOverridePath,
					[]trainer.PodTemplateOverride{
						{
							TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
							Spec: &trainer.PodTemplateSpecOverride{
								InitContainers: []trainer.ContainerOverride{
									{
										Name: "invalid",
									},
								},
							},
						},
					},
					fmt.Sprintf("must not have initContainer that doesn't exist in the runtime job %s", constants.Node)),
			},
		},
		"podTemplateOverrides contain invalid container": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							Containers: []trainer.ContainerOverride{
								{
									Name: "invalid",
								},
							},
						},
					},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(podTemplateOverridePath,
					[]trainer.PodTemplateOverride{
						{
							TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
							Spec: &trainer.PodTemplateSpecOverride{
								Containers: []trainer.ContainerOverride{
									{
										Name: "invalid",
									},
								},
							},
						},
					},
					fmt.Sprintf("must not have container that doesn't exist in the runtime job %s", constants.Node)),
			},
		},
		"podTemplateOverrides contain envs for reserved container": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							Containers: []trainer.ContainerOverride{
								{
									Name: constants.Node,
									Env: []corev1.EnvVar{
										{
											Name:  "ENV_NAME",
											Value: "OVERRIDE",
										},
									},
								},
							},
						},
					},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(podTemplateOverridePath,
					[]trainer.PodTemplateOverride{
						{
							TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
							Spec: &trainer.PodTemplateSpecOverride{
								Containers: []trainer.ContainerOverride{
									{
										Name: constants.Node,
										Env: []corev1.EnvVar{
											{
												Name:  "ENV_NAME",
												Value: "OVERRIDE",
											},
										},
									},
								},
							},
						},
					},
					fmt.Sprintf("must not have envs for the %s, %s, %s containers", constants.DatasetInitializer, constants.ModelInitializer, constants.Node)),
			},
		},
		"allow podTemplateOverrides when creating a new trainJob": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: nil,
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			wantError: nil,
		},
		"allow updates to trainJob with no changes to podTemplateOverrides": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			wantError: nil,
		},
		"forbid changes to podTemplateOverrides when trainJob is not suspended": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(false).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(false).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account-updated"),
						},
					},
				}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(podTemplateOverridePath, "PodTemplateOverrides can only be modified when the TrainJob is suspended"),
			},
		},
		"allow changes to podTemplateOverrides when trainJob is suspended and jobSet does not exists": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account-updated"),
						},
					},
				}).
				Obj(),
			clientErr: apierrors.NewNotFound(jobsetv1alpha2.Resource("jobset"), ""),
			wantError: nil,
		},
		"allow changes to podTemplateOverrides when trainJob is suspended and jobSet exists but is inactive": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.DatasetInitializer),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account-updated"),
						},
					},
				}).
				Obj(),
			jobSet: &jobsetv1alpha2.JobSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: metav1.NamespaceDefault,
				},
				Status: jobsetv1alpha2.JobSetStatus{
					ReplicatedJobsStatus: []jobsetv1alpha2.ReplicatedJobStatus{
						{
							Name:   constants.DatasetInitializer,
							Active: 0,
						},
						{
							Name:   constants.Node,
							Active: 0,
						},
					},
				},
			},
			wantError: nil,
		},
		"forbid changes to podTemplateOverrides when trainJob is suspended but jobSet has an active replicatedJob": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.DatasetInitializer),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account-updated"),
						},
					},
				}).
				Obj(),
			jobSet: &jobsetv1alpha2.JobSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: metav1.NamespaceDefault,
				},
				Status: jobsetv1alpha2.JobSetStatus{
					ReplicatedJobsStatus: []jobsetv1alpha2.ReplicatedJobStatus{
						{
							Name:   constants.DatasetInitializer,
							Active: 0,
						},
						{
							Name:   constants.Node,
							Active: 2,
						},
					},
				},
			},
			wantError: field.ErrorList{
				field.Forbidden(podTemplateOverridePath, "PodTemplateOverrides cannot be modified when the JobSet's ReplicatedJob node is still active"),
			},
		},
		"forbid changes to podTemplateOverrides when trainJob is suspended but has multiple active replicatedJobs": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.DatasetInitializer),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
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
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account-updated"),
						},
					},
				}).
				Obj(),
			jobSet: &jobsetv1alpha2.JobSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: metav1.NamespaceDefault,
				},
				Status: jobsetv1alpha2.JobSetStatus{
					ReplicatedJobsStatus: []jobsetv1alpha2.ReplicatedJobStatus{
						{
							Name:   constants.DatasetInitializer,
							Active: 1,
						},
						{
							Name:   constants.Node,
							Active: 2,
						},
					},
				},
			},
			wantError: field.ErrorList{
				field.Forbidden(podTemplateOverridePath, "PodTemplateOverrides cannot be modified when the JobSet's ReplicatedJob dataset-initializer is still active"),
				field.Forbidden(podTemplateOverridePath, "PodTemplateOverrides cannot be modified when the JobSet's ReplicatedJob node is still active"),
			},
		},
		"forbid changes to podTemplateOverrides when trainJob is suspended but jobSet cannot be checked due to a client error": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account-updated"),
						},
					},
				}).
				Obj(),
			clientErr: fmt.Errorf("client error"),
			wantError: field.ErrorList{
				field.InternalError(podTemplateOverridePath, fmt.Errorf("client error")),
			},
		},
		"allow atomic update: unsuspend and modify podTemplateOverrides in single request": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: &jobsetv1alpha2ac.JobSetSpecApplyConfiguration{
						ReplicatedJobs: []jobsetv1alpha2ac.ReplicatedJobApplyConfiguration{
							{
								Name: ptr.To(constants.Node),
								Template: &batchv1ac.JobTemplateSpecApplyConfiguration{
									Spec: &batchv1ac.JobSpecApplyConfiguration{
										Template: &corev1ac.PodTemplateSpecApplyConfiguration{
											Spec: &corev1ac.PodSpecApplyConfiguration{
												Containers: []corev1ac.ContainerApplyConfiguration{
													{
														Name: ptr.To(constants.Node),
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
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(true).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("service-account"),
						},
					},
				}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Suspend(false).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{
							{
								Name: constants.Node,
							},
						},
						Spec: &trainer.PodTemplateSpecOverride{
							NodeSelector: map[string]string{
								"accelerator": "nvidia-h100",
							},
						},
					},
				}).
				Obj(),
			clientErr: apierrors.NewNotFound(jobsetv1alpha2.Resource("jobset"), ""),
			wantError: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := utiltesting.NewClientBuilder()
			if tc.jobSet != nil {
				clientBuilder = clientBuilder.WithObjects(tc.jobSet)
			}
			if tc.clientErr != nil {
				clientBuilder = clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*jobsetv1alpha2.JobSet); ok {
							return tc.clientErr
						}
						return cli.Get(ctx, key, obj, opts...)
					},
				})
			}
			cli := clientBuilder.Build()

			p, err := New(ctx, cli, nil)
			if err != nil {
				t.Fatalf("Failed to initialize JobSet plugin: %v", err)
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
