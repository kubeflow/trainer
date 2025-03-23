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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
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

func TestJobSetValidate(t *testing.T) {
	cases := map[string]struct {
		runtimeJobTemplate client.Object
		runtimeInfo        *runtime.Info
		oldObj             *trainer.TrainJob
		newObj             *trainer.TrainJob
		wantError          field.ErrorList
		wantWarnings       admission.Warnings
	}{
		"no jobset runtime template": {
			runtimeJobTemplate: &utiltesting.JobSetWrapper{},
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"no initializer job": {
			runtimeJobTemplate: utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "test"),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"no dataset intializer job": {
			runtimeJobTemplate: utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "test"),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Initializer(&trainer.Initializer{Dataset: nil}).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("runtimeRef"),
					utiltesting.MakeTrainJobWrapper("default", "test").Obj().Spec.RuntimeRef,
					fmt.Sprintf("must have %s job when trainJob is configured with input modelConfig", constants.DatasetInitializer)),
			},
		},
		// assert that we get a error  here
		"dataset initializer job exists but container missing": {
			runtimeJobTemplate: func() client.Object {
				js := utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "test")
				for i, rJob := range js.Spec.ReplicatedJobs {
					if rJob.Name == constants.DatasetInitializer {
						js.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers = []v1.Container{}
					}
				}
				return js.Obj()
			}(),
			newObj: utiltesting.MakeTrainJobWrapper("default", "test").
				Initializer(&trainer.Initializer{
					Dataset: &trainer.DatasetInitializer{},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("runtimeRef"),
					utiltesting.MakeTrainJobWrapper("default", "test").Obj().Spec.RuntimeRef,
					fmt.Sprintf("must have container with name - %s in the %s job", constants.DatasetInitializer, constants.DatasetInitializer)),
			},
		},
		"no model intializer job": {
			runtimeJobTemplate: utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "test"),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Initializer(&trainer.Initializer{Model: nil}).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("runtimeRef"),
					utiltesting.MakeTrainJobWrapper("default", "test").Obj().Spec.RuntimeRef,
					fmt.Sprintf("must have %s job when trainJob is configured with input modelConfig", constants.ModelInitializer)),
			},
		},
		"model intializer job exists but container missing ": {
			runtimeJobTemplate: func() client.Object {
				js := utiltesting.MakeJobSetWrapper(metav1.NamespaceDefault, "test")
				for i, rjob := range js.Spec.ReplicatedJobs {
					if rjob.Name == constants.ModelInitializer {
						js.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers = []v1.Container{}
					}
				}
				return js.Obj()
			}(),
			runtimeInfo: runtime.NewInfo(),
			newObj: utiltesting.MakeTrainJobWrapper("default", "test").
				Initializer(&trainer.Initializer{
					Model: &trainer.ModelInitializer{},
				}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("runtimeRef"),
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
			warnings, errs := p.(framework.CustomValidationPlugin).Validate(tc.runtimeJobTemplate, nil, tc.oldObj, tc.newObj)
			if diff := cmp.Diff(tc.wantError, errs); len(diff) != 0 {
				t.Errorf("Unexpected error from Validate (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantWarnings, warnings); len(diff) != 0 {
				t.Errorf("Unexpected warnings from Validate (-want, +got): %s", diff)
			}
		})
	}
}
