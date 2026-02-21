/*
Copyright 2026 The Kubeflow Authors.

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

package xgboost

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
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestXGBoostValidate(t *testing.T) {
	cases := map[string]struct {
		trainJob *trainer.TrainJob
		wantErrs field.ErrorList
	}{
		"no error when trainer is nil": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
		},
		"no error when env does not contain reserved names": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						Obj(),
				).
				Obj(),
		},
		"error when using reserved DMLC_TRACKER_URI env": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						Env(corev1.EnvVar{Name: constants.XGBoostEnvTrackerURI, Value: "custom-tracker"}).
						Obj(),
				).
				Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "trainer", "env").Index(0),
					fmt.Sprintf("%s is reserved for the XGBoost runtime", constants.XGBoostEnvTrackerURI),
				),
			},
		},
		"error when using reserved DMLC_NUM_WORKER env": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						Env(corev1.EnvVar{Name: constants.XGBoostEnvNumWorker, Value: "10"}).
						Obj(),
				).
				Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "trainer", "env").Index(0),
					fmt.Sprintf("%s is reserved for the XGBoost runtime", constants.XGBoostEnvNumWorker),
				),
			},
		},
		"multiple errors when using multiple reserved envs": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						Env(
							corev1.EnvVar{Name: constants.XGBoostEnvTrackerURI, Value: "custom"},
							corev1.EnvVar{Name: constants.XGBoostEnvTaskID, Value: "0"},
						).
						Obj(),
				).
				Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "trainer", "env").Index(0),
					fmt.Sprintf("%s is reserved for the XGBoost runtime", constants.XGBoostEnvTrackerURI),
				),
				field.Forbidden(
					field.NewPath("spec", "trainer", "env").Index(1),
					fmt.Sprintf("%s is reserved for the XGBoost runtime", constants.XGBoostEnvTaskID),
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
			cliBuilder := utiltesting.NewClientBuilder()
			p, err := New(ctx, cliBuilder.Build(), nil)
			if err != nil {
				t.Fatalf("Failed to initialize XGBoost plugin: %v", err)
			}

			// Test Validate
			_, errs := p.(framework.CustomValidationPlugin).Validate(ctx, nil, nil, tc.trainJob)
			if diff := cmp.Diff(tc.wantErrs, errs, cmpopts.IgnoreFields(field.Error{}, "Detail")); len(diff) != 0 {
				t.Errorf("Unexpected validation errors (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestXGBoostEnforceMLPolicy(t *testing.T) {
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
		"no action when mlPolicySource XGBoost is null": {
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
		"single node XGBoost training (CPU)": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							XGBoostPolicy().
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
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("test-job-%s-0-0.test-job", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("1"),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"multi-node XGBoost training (CPU)": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							XGBoostPolicy().
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
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("multi-node-job-%s-0-0.multi-node-job", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("4"),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"XGBoost training with GPU resources": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							XGBoostPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "gpu-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Container("xgboost/xgboost:latest", nil, nil, corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("32Gi"),
							"example.com/gpu":     resource.MustParse("4"),
						}).
						Obj(),
				).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("gpu-job-%s-0-0.gpu-job", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("8"),
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
							XGBoostPolicy().
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
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("trainJob-%s-0-0.trainJob", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("3"),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"resources not set - defaults to 1 worker per node": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							XGBoostPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 2, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "no-res-job").
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
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("no-res-job-%s-0-0.no-res-job", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("2"),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"resources set in Runtime only": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							XGBoostPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
				runtime.WithTemplateSpecObjApply(
					jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
														WithResources(corev1ac.ResourceRequirements().
															WithRequests(corev1.ResourceList{
																"example.com/gpu": resource.MustParse("2"),
															}),
														),
												),
											),
										),
									),
								),
						),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "runtime-res-job").
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
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("runtime-res-job-%s-0-0.runtime-res-job", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("6"),
								},
							},
						}},
					}},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
														WithResources(corev1ac.ResourceRequirements().
															WithRequests(corev1.ResourceList{
																"example.com/gpu": resource.MustParse("2"),
															}),
														),
												),
											),
										),
									),
								),
						),
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"resources set in TrainJob only": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							XGBoostPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainjob-res").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Container("xgboost/xgboost:latest", nil, nil, corev1.ResourceList{
							"example.com/gpu": resource.MustParse("4"),
						}).
						Obj(),
				).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("trainjob-res-%s-0-0.trainjob-res", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("8"),
								},
							},
						}},
					}},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
		},
		"resources set in both Runtime and TrainJob": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(
					utiltesting.MakeMLPolicyWrapper().
						WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
							XGBoostPolicy().
							Obj(),
						).
						Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
				runtime.WithTemplateSpecObjApply(
					jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
														WithResources(corev1ac.ResourceRequirements().
															WithRequests(corev1.ResourceList{
																"example.com/gpu": resource.MustParse("1"),
															}),
														),
												),
											),
										),
									),
								),
						),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "both-res-job").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Container("xgboost/xgboost:latest", nil, nil, corev1.ResourceList{
							"example.com/gpu": resource.MustParse("3"),
						}).
						Obj(),
				).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						XGBoostPolicy().
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
								ContainerPort: ptr.To(constants.ContainerTrainerPort),
							}},
							Env: []corev1ac.EnvVarApplyConfiguration{
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerURI),
									Value: ptr.To(fmt.Sprintf("both-res-job-%s-0-0.both-res-job", constants.Node)),
								},
								{
									Name:  ptr.To(constants.XGBoostEnvTrackerPort),
									Value: ptr.To(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
								},
								{
									Name: ptr.To(constants.XGBoostEnvTaskID),
									ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
										FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
											FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
										},
									},
								},
								{
									Name:  ptr.To(constants.XGBoostEnvNumWorker),
									Value: ptr.To("6"),
								},
							},
						}},
					}},
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(
							jobsetv1alpha2ac.ReplicatedJob().
								WithName(constants.Node).
								WithTemplate(batchv1ac.JobTemplateSpec().
									WithSpec(batchv1ac.JobSpec().
										WithTemplate(corev1ac.PodTemplateSpec().
											WithSpec(corev1ac.PodSpec().
												WithContainers(
													corev1ac.Container().
														WithName(constants.Node).
														WithResources(corev1ac.ResourceRequirements().
															WithRequests(corev1.ResourceList{
																"example.com/gpu": resource.MustParse("1"),
															}),
														),
												),
											),
										),
									),
								),
						),
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
				t.Fatalf("Failed to initialize XGBoost plugin: %v", err)
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
