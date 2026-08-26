// /*
// Copyright 2026 The Kubeflow Authors.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

// package controller

// import (
// 	"context"
// 	"fmt"

// 	"github.com/onsi/ginkgo/v2"
// 	"github.com/onsi/gomega"
// 	appsv1 "k8s.io/api/apps/v1"
// 	corev1 "k8s.io/api/core/v1"
// 	"k8s.io/apimachinery/pkg/api/meta"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/utils/ptr"
// 	"sigs.k8s.io/controller-runtime/pkg/client"

// 	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
// 	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
// 	"github.com/kubeflow/trainer/v2/pkg/constants"
// 	"github.com/kubeflow/trainer/v2/test/integration/framework"
// 	"github.com/kubeflow/trainer/v2/test/util"
// )

// type mockIntegrationSuggestionClient struct {
// 	MockedAssignments [][]trainer.ParameterAssignment
// 	MockErr           error
// }

// func (m *mockIntegrationSuggestionClient) GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error) {
// 	return m.MockedAssignments, m.MockErr
// }

// var mockClient = &mockIntegrationSuggestionClient{}

// var _ = ginkgo.Describe("OptimizationJob Controller", ginkgo.Ordered, func() {
// 	var ns *corev1.Namespace

// 	ginkgo.BeforeAll(func() {
// 		fwk = &framework.Framework{
// 			SuggestionClient: mockClient,
// 		}
// 		cfg = fwk.Init()
// 		ctx, k8sClient = fwk.RunManager(cfg, true)

// 		// Note: Ensure that in your test suite's RunManager/SetupWithManager,
// 		// OptimizationJobReconciler is initialized with a mock SuggestionClient
// 		// that returns standard parameters (e.g., {"lr": "0.01"}) to bypass gRPC calls.
// 	})

// 	ginkgo.AfterAll(func() {
// 		fwk.Teardown()
// 	})

// 	ginkgo.BeforeEach(func() {
// 		ns = &corev1.Namespace{
// 			TypeMeta: metav1.TypeMeta{
// 				APIVersion: corev1.SchemeGroupVersion.String(),
// 				Kind:       "Namespace",
// 			},
// 			ObjectMeta: metav1.ObjectMeta{
// 				GenerateName: "optjob-test-",
// 			},
// 		}
// 		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
// 		mockClient.MockErr = nil
// 		mockClient.MockedAssignments = [][]trainer.ParameterAssignment{
// 			{{Name: "lr", Value: "0.01"}},
// 		}
// 	})

// 	ginkgo.When("Reconciling OptimizationJob", func() {
// 		var (
// 			optJob    *trainer.OptimizationJob
// 			optJobKey client.ObjectKey
// 		)

// 		ginkgo.AfterEach(func() {
// 			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
// 			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
// 		})

// 		ginkgo.BeforeEach(func() {
// 			optJob = &trainer.OptimizationJob{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-optjob",
// 					Namespace: ns.Name,
// 				},
// 				Spec: trainer.OptimizationJobSpec{
// 					NumTrials:      2,
// 					ParallelTrials: 1,
// 					Parameters: []trainer.Parameter{
// 						{
// 							Name: "lr",
// 							SearchSpace: &trainer.SearchSpace{
// 								Uniform: trainer.UniformSpace{
// 									Min: "0.01",
// 									Max: "0.1",
// 								},
// 							},
// 						},
// 					},
// 					Objectives: []trainer.Objective{
// 						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
// 					},
// 					TrainJobTemplate: trainer.TrainJobTemplateSpec{
// 						Spec: trainer.TrainJobSpec{
// 							RuntimeRef: trainer.RuntimeRef{
// 								Name:     "dummy-runtime",
// 								APIGroup: ptr.To(trainer.GroupVersion.Group),
// 								Kind:     ptr.To(trainer.TrainingRuntimeKind),
// 							},
// 							Trainer: &trainer.Trainer{
// 								Image: ptr.To("docker.io/kubeflow/trainer:latest"),
// 							},
// 						},
// 					},
// 				},
// 			}
// 			optJobKey = client.ObjectKeyFromObject(optJob)
// 		})

// 		ginkgo.Context("Algorithm Service Provisioning", func() {
// 			ginkgo.It("Should create Optuna Deployment and Service, and wait for readiness", func() {
// 				ginkgo.By("Creating OptimizationJob")
// 				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

// 				deployKey := client.ObjectKey{Name: "test-optjob-optuna", Namespace: ns.Name}
// 				svcKey := client.ObjectKey{Name: "test-optjob-optuna", Namespace: ns.Name}

// 				ginkgo.By("Waiting for the Deployment to be created")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					deploy := &appsv1.Deployment{}

// 					err := k8sClient.Get(ctx, deployKey, deploy)
// 					g.Expect(err).Should(gomega.Succeed())

// 					g.Expect(deploy.OwnerReferences).Should(gomega.Not(gomega.BeEmpty()))
// 					g.Expect(deploy.OwnerReferences[0].UID).Should(gomega.Equal(optJob.UID))
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				// Run assertions synchronously outside the poller to avoid Gomega panics!
// 				gomega.Expect(deploy.OwnerReferences).Should(gomega.HaveLen(1))
// 				gomega.Expect(deploy.OwnerReferences[0].UID).Should(gomega.Equal(optJob.UID))

// 				ginkgo.By("Waiting for the Service to be created")
// 				svc := &corev1.Service{}
// 				gomega.Eventually(func() error {
// 					return k8sClient.Get(ctx, svcKey, svc)
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				// Synchronous assertions
// 				gomega.Expect(svc.Spec.Ports).ShouldNot(gomega.BeEmpty())
// 				gomega.Expect(svc.Spec.Ports[0].Port).Should(gomega.Equal(int32(6789)))

// 				ginkgo.By("Simulating Optuna Pod Readiness")
// 				gomega.Eventually(func() error {
// 					// Always fetch fresh copy before updating status
// 					if err := k8sClient.Get(ctx, deployKey, deploy); err != nil {
// 						return err
// 					}

// 					// Set all replica fields to satisfy API validation
// 					deploy.Status.Replicas = 1
// 					deploy.Status.ReadyReplicas = 1
// 					deploy.Status.AvailableReplicas = 1

// 					return k8sClient.Status().Update(ctx, deploy)
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())
// 				ginkgo.By("Checking that OptimizationJob transitions to Created=True")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					gotJob := &trainer.OptimizationJob{}
// 					g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())

// 					g.Expect(gotJob.Status.Conditions).Should(gomega.BeComparableTo([]metav1.Condition{
// 						{
// 							Type:   constants.OptimizationJobCreated,
// 							Status: metav1.ConditionTrue,
// 							Reason: "AlgorithmServiceCreated",
// 						},
// 					}, util.IgnoreConditions))
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())
// 			})
// 		})

// 		ginkgo.Context("Trial Synchronization and Completion", func() {
// 			ginkgo.It("Should spawn TrainJobs, sync Active state, and complete successfully with best metric", func() {
// 				ginkgo.By("Creating OptimizationJob")
// 				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

// 				ginkgo.By("Simulating Optuna Pod Readiness")
// 				deployKey := client.ObjectKey{Name: "test-optjob-optuna", Namespace: ns.Name}
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					deploy := &appsv1.Deployment{}
// 					if err := k8sClient.Get(ctx, deployKey, deploy); err == nil {
// 						deploy.Status.AvailableReplicas = 1
// 						g.Expect(k8sClient.Status().Update(ctx, deploy)).Should(gomega.Succeed())
// 					} else {
// 						g.Expect(err).ShouldNot(gomega.HaveOccurred())
// 					}
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				ginkgo.By("Waiting for the first TrainJob (Trial 0) to be spawned")
// 				var trainJobs trainer.TrainJobList
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					err := k8sClient.List(ctx, &trainJobs, client.InNamespace(ns.Name))
// 					g.Expect(err).Should(gomega.Succeed())
// 					if err != nil {
// 						return
// 					}

// 					g.Expect(trainJobs.Items).Should(gomega.Not(gomega.BeEmpty()))
// 					if len(trainJobs.Items) > 0 {
// 						g.Expect(trainJobs.Items[0].Name).Should(gomega.Equal("test-optjob-trial-0"))
// 					}
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				ginkgo.By("Checking that OptimizationJob Status indicates 1 Running trial")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					gotJob := &trainer.OptimizationJob{}
// 					g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())
// 					g.Expect(gotJob.Status.Conditions).Should(gomega.ContainElement(gomega.SatisfyAll(
// 						gomega.HaveField("Type", "Running"),
// 						gomega.HaveField("Status", metav1.ConditionTrue),
// 					)))
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				ginkgo.By("Simulating completion of Trial 0 (Accuracy: 0.85)")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					err := k8sClient.List(ctx, &trainJobs, client.InNamespace(ns.Name))
// 					g.Expect(err).Should(gomega.Succeed())
// 					if err != nil || len(trainJobs.Items) == 0 {
// 						return
// 					} // STOP IF EMPTY

// 					tj := &trainJobs.Items[0]
// 					err = k8sClient.Get(ctx, client.ObjectKeyFromObject(tj), tj)
// 					g.Expect(err).Should(gomega.Succeed())
// 					if err != nil {
// 						return
// 					}

// 					meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
// 						Type:   trainer.TrainJobComplete,
// 						Status: metav1.ConditionTrue,
// 						Reason: "JobFinished",
// 					})
// 					tj.Status.TrainerStatus = &trainer.TrainerStatus{
// 						Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.85"}},
// 					}
// 					g.Expect(k8sClient.Status().Update(ctx, tj)).Should(gomega.Succeed())
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				ginkgo.By("Simulating completion of Trial 1 with a BETTER metric (Accuracy: 0.95)")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					var trainJobs trainer.TrainJobList

// 					g.Expect(k8sClient.List(
// 						ctx,
// 						&trainJobs,
// 						client.InNamespace(ns.Name),
// 					)).Should(gomega.Succeed())

// 					var tj *trainer.TrainJob

// 					for i := range trainJobs.Items {
// 						if trainJobs.Items[i].Name == "test-optjob-trial-1" {
// 							tj = &trainJobs.Items[i]
// 							break
// 						}
// 					}

// 					g.Expect(tj).ShouldNot(gomega.BeNil())

// 					meta.SetStatusCondition(
// 						&tj.Status.Conditions,
// 						metav1.Condition{
// 							Type:   trainer.TrainJobComplete,
// 							Status: metav1.ConditionTrue,
// 							Reason: "JobFinished",
// 						},
// 					)

// 					tj.Status.TrainerStatus = &trainer.TrainerStatus{
// 						Metrics: []trainer.Metric{
// 							{
// 								Name:  "accuracy",
// 								Value: "0.95",
// 							},
// 						},
// 					}

// 					g.Expect(k8sClient.Status().Update(ctx, tj)).Should(gomega.Succeed())
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())
// 			})

// 			ginkgo.It("Should fail the OptimizationJob if all TrainJobs crash", func() {
// 				ginkgo.By("Creating OptimizationJob")
// 				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

// 				ginkgo.By("Simulating Optuna Pod Readiness")
// 				deployKey := client.ObjectKey{Name: "test-optjob-optuna", Namespace: ns.Name}
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					deploy := &appsv1.Deployment{}
// 					if err := k8sClient.Get(ctx, deployKey, deploy); err == nil {
// 						deploy.Status.AvailableReplicas = 1
// 						g.Expect(k8sClient.Status().Update(ctx, deploy)).Should(gomega.Succeed())
// 					}
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				var trainJobs trainer.TrainJobList
// 				ginkgo.By("Waiting for the first TrainJob (Trial 0) and simulating Failure")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					err := k8sClient.List(ctx, &trainJobs, client.InNamespace(ns.Name))
// 					g.Expect(err).Should(gomega.Succeed())
// 					if err != nil || len(trainJobs.Items) == 0 {
// 						return
// 					}

// 					tj := &trainJobs.Items[0]
// 					meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
// 						Type:   trainer.TrainJobFailed,
// 						Status: metav1.ConditionTrue,
// 						Reason: "CrashLoopBackOff",
// 					})
// 					g.Expect(k8sClient.Status().Update(ctx, tj)).Should(gomega.Succeed())
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				ginkgo.By("Waiting for the second TrainJob (Trial 1) and simulating Failure")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					err := k8sClient.List(ctx, &trainJobs, client.InNamespace(ns.Name))
// 					g.Expect(err).Should(gomega.Succeed())
// 					if err != nil || len(trainJobs.Items) < 2 {
// 						return
// 					}

// 					var tj *trainer.TrainJob
// 					for i := range trainJobs.Items {
// 						if trainJobs.Items[i].Name == "test-optjob-trial-1" {
// 							tj = &trainJobs.Items[i]
// 						}
// 					}
// 					g.Expect(tj).ShouldNot(gomega.BeNil())
// 					if tj == nil {
// 						return
// 					}

// 					meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
// 						Type:   trainer.TrainJobFailed,
// 						Status: metav1.ConditionTrue,
// 						Reason: "CrashLoopBackOff",
// 					})
// 					g.Expect(k8sClient.Status().Update(ctx, tj)).Should(gomega.Succeed())
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				ginkgo.By("Verifying OptimizationJob marks itself as FAILED due to complete Trial failure")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					gotJob := &trainer.OptimizationJob{}
// 					g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())

// 					g.Expect(gotJob.Status.Conditions).Should(gomega.ContainElement(gomega.SatisfyAll(
// 						gomega.HaveField("Type", constants.OptimizationJobFailed),
// 						gomega.HaveField("Status", metav1.ConditionTrue),
// 						gomega.HaveField("Reason", "AllTrialsFailed"),
// 					)))
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())
// 			})

// 			ginkgo.It("Should fail the OptimizationJob if the gRPC Suggestion Service fails", func() {
// 				ginkgo.By("Injecting a simulated gRPC connection error into the framework mock")
// 				mockClient.MockErr = fmt.Errorf("connection refused")

// 				ginkgo.By("Creating OptimizationJob")
// 				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

// 				ginkgo.By("Simulating Optuna Pod Readiness")
// 				deployKey := client.ObjectKey{Name: "test-optjob-optuna", Namespace: ns.Name}
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					deploy := &appsv1.Deployment{}
// 					if err := k8sClient.Get(ctx, deployKey, deploy); err == nil {
// 						deploy.Status.AvailableReplicas = 1
// 						g.Expect(k8sClient.Status().Update(ctx, deploy)).Should(gomega.Succeed())
// 					}
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())

// 				ginkgo.By("Verifying OptimizationJob marks itself as FAILED due to SuggestionServiceFailed")
// 				gomega.Eventually(func(g gomega.Gomega) {
// 					gotJob := &trainer.OptimizationJob{}
// 					g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())

// 					g.Expect(gotJob.Status.Conditions).Should(gomega.ContainElement(gomega.SatisfyAll(
// 						gomega.HaveField("Type", constants.OptimizationJobFailed),
// 						gomega.HaveField("Status", metav1.ConditionTrue),
// 						gomega.HaveField("Reason", "SuggestionServiceFailed"),
// 					)))
// 				}, util.Timeout, util.Interval).Should(gomega.Succeed())
// 			})
// 		})
// 	})
// })

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

package controller

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

type mockIntegrationSuggestionClient struct {
	MockedAssignments [][]trainer.ParameterAssignment
	MockErr           error
}

func (m *mockIntegrationSuggestionClient) GetSuggestions(
	ctx context.Context,
	addr string,
	req *katibapi.GetSuggestionsRequest,
) ([][]trainer.ParameterAssignment, error) {
	return m.MockedAssignments, m.MockErr
}

var mockClient = &mockIntegrationSuggestionClient{}

var _ = ginkgo.Describe("OptimizationJob Controller", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{
			SuggestionClient: mockClient,
		}
		cfg = fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg, true)

		// RunManager/SetupWithManager must initialize OptimizationJobReconciler
		// with the framework's SuggestionClient so these tests do not make
		// real gRPC calls to the Optuna suggestion service.
	})

	ginkgo.AfterAll(func() {
		fwk.Teardown()
	})

	ginkgo.BeforeEach(func() {
		ns = &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "optjob-test-",
			},
		}

		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())

		mockClient.MockErr = nil
		mockClient.MockedAssignments = [][]trainer.ParameterAssignment{
			{{Name: "lr", Value: "0.01"}},
		}
	})

	ginkgo.When("Reconciling OptimizationJob", func() {
		var (
			optJob    *trainer.OptimizationJob
			optJobKey client.ObjectKey
		)

		ginkgo.AfterEach(func() {
			gomega.Expect(
				k8sClient.DeleteAllOf(
					ctx,
					&trainer.OptimizationJob{},
					client.InNamespace(ns.Name),
				),
			).Should(gomega.Succeed())

			gomega.Expect(
				k8sClient.DeleteAllOf(
					ctx,
					&trainer.TrainJob{},
					client.InNamespace(ns.Name),
				),
			).Should(gomega.Succeed())
		})

		ginkgo.BeforeEach(func() {
			optJob = &trainer.OptimizationJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-optjob",
					Namespace: ns.Name,
				},
				Spec: trainer.OptimizationJobSpec{
					NumTrials:      2,
					ParallelTrials: 1,
					Parameters: []trainer.Parameter{
						{
							Name: "lr",
							SearchSpace: &trainer.SearchSpace{
								Uniform: trainer.UniformSpace{
									Min: "0.01",
									Max: "0.1",
								},
							},
						},
					},
					Objectives: []trainer.Objective{
						{
							Metric:    "accuracy",
							Direction: trainer.ObjectiveDirectionMaximize,
						},
					},
					TrainJobTemplate: trainer.TrainJobTemplateSpec{
						Spec: trainer.TrainJobSpec{
							RuntimeRef: trainer.RuntimeRef{
								Name:     "dummy-runtime",
								APIGroup: ptr.To(trainer.GroupVersion.Group),
								Kind:     ptr.To(trainer.TrainingRuntimeKind),
							},
							Trainer: &trainer.Trainer{
								Image: ptr.To("docker.io/kubeflow/trainer:latest"),
							},
						},
					},
				},
			}

			optJobKey = client.ObjectKeyFromObject(optJob)
		})

		ginkgo.Context("Algorithm Service Provisioning", func() {
			ginkgo.It("Should create Optuna Deployment and Service, and wait for readiness", func() {
				ginkgo.By("Creating OptimizationJob")
				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

				deployKey := client.ObjectKey{
					Name:      "test-optjob-optuna",
					Namespace: ns.Name,
				}
				svcKey := client.ObjectKey{
					Name:      "test-optjob-optuna",
					Namespace: ns.Name,
				}

				ginkgo.By("Waiting for the Optuna Deployment to be created and owned by the OptimizationJob")
				gomega.Eventually(func(g gomega.Gomega) {
					deploy := &appsv1.Deployment{}

					g.Expect(k8sClient.Get(ctx, deployKey, deploy)).Should(gomega.Succeed())
					g.Expect(deploy.OwnerReferences).Should(gomega.HaveLen(1))
					g.Expect(deploy.OwnerReferences[0].UID).Should(gomega.Equal(optJob.UID))
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Waiting for the Optuna Service to be created")
				gomega.Eventually(func(g gomega.Gomega) {
					svc := &corev1.Service{}

					g.Expect(k8sClient.Get(ctx, svcKey, svc)).Should(gomega.Succeed())
					g.Expect(svc.Spec.Ports).ShouldNot(gomega.BeEmpty())
					g.Expect(svc.Spec.Ports[0].Port).Should(gomega.Equal(int32(6789)))
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Simulating Optuna Pod Readiness")
				gomega.Eventually(func(g gomega.Gomega) {
					deploy := &appsv1.Deployment{}

					g.Expect(k8sClient.Get(ctx, deployKey, deploy)).Should(gomega.Succeed())

					deploy.Status.Replicas = 1
					deploy.Status.ReadyReplicas = 1
					deploy.Status.AvailableReplicas = 1

					g.Expect(k8sClient.Status().Update(ctx, deploy)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Checking that OptimizationJob transitions to Created=True")
				gomega.Eventually(func() bool {
					gotJob := &trainer.OptimizationJob{}

					if err := k8sClient.Get(ctx, optJobKey, gotJob); err != nil {
						return false
					}

					if gotJob.Status == nil {
						return false
					}

					return meta.IsStatusConditionTrue(
						gotJob.Status.Conditions,
						constants.OptimizationJobCreated,
					)
				}, util.Timeout, util.Interval).Should(gomega.BeTrue())
			})
		})

		ginkgo.Context("Trial Synchronization and Completion", func() {
			ginkgo.It("Should spawn TrainJobs, sync Active state, and complete successfully with best metric", func() {
				ginkgo.By("Creating OptimizationJob")

				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

				deployKey := client.ObjectKey{
					Name:      "test-optjob-optuna",
					Namespace: ns.Name,
				}

				svcKey := client.ObjectKey{
					Name:      "test-optjob-optuna",
					Namespace: ns.Name,
				}

				ginkgo.By("Waiting for the Optuna Deployment to be created")

				gomega.Eventually(func() error {
					deploy := &appsv1.Deployment{}
					return k8sClient.Get(ctx, deployKey, deploy)
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Waiting for the Optuna Service to be created")

				gomega.Eventually(func() error {
					svc := &corev1.Service{}
					return k8sClient.Get(ctx, svcKey, svc)
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Simulating Optuna Pod Readiness")

				deploy := &appsv1.Deployment{}
				gomega.Expect(
					k8sClient.Get(ctx, deployKey, deploy),
				).Should(gomega.Succeed())

				deploy.Status.Replicas = 1
				deploy.Status.ReadyReplicas = 1
				deploy.Status.AvailableReplicas = 1

				gomega.Expect(
					k8sClient.Status().Update(ctx, deploy),
				).Should(gomega.Succeed())

				ginkgo.By("Waiting for OptimizationJob Created=True")

				gomega.Eventually(func() bool {
					gotJob := &trainer.OptimizationJob{}

					if err := k8sClient.Get(ctx, optJobKey, gotJob); err != nil {
						return false
					}

					if gotJob.Status == nil {
						return false
					}

					return meta.IsStatusConditionTrue(
						gotJob.Status.Conditions,
						constants.OptimizationJobCreated,
					)
				}, util.Timeout, util.Interval).Should(gomega.BeTrue())

				ginkgo.By("Waiting for the first TrainJob (Trial 0) to be spawned")

				trial0Key := client.ObjectKey{
					Name:      "test-optjob-trial-0",
					Namespace: ns.Name,
				}

				gomega.Eventually(func(g gomega.Gomega) {
					trial0 := &trainer.TrainJob{}

					g.Expect(
						k8sClient.Get(ctx, trial0Key, trial0),
					).Should(gomega.Succeed())

					g.Expect(trial0.OwnerReferences).Should(gomega.HaveLen(1))
					g.Expect(trial0.OwnerReferences[0].UID).
						Should(gomega.Equal(optJob.UID))

					g.Expect(
						trial0.Annotations["trainer.kubeflow.org/opt-param-lr"],
					).Should(gomega.Equal("0.01"))
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Checking that OptimizationJob Status indicates 1 Running trial")
				gomega.Eventually(func() bool {
					gotJob := &trainer.OptimizationJob{}

					if err := k8sClient.Get(ctx, optJobKey, gotJob); err != nil {
						return false
					}

					if gotJob.Status == nil {
						return false
					}

					return meta.IsStatusConditionTrue(
						gotJob.Status.Conditions,
						"Running",
					)
				}, util.Timeout, util.Interval).Should(gomega.BeTrue())

				ginkgo.By("Simulating completion of Trial 0 (Accuracy: 0.85)")
				trial0 := &trainer.TrainJob{}

				gomega.Expect(
					k8sClient.Get(ctx, trial0Key, trial0),
				).Should(gomega.Succeed())

				meta.SetStatusCondition(
					&trial0.Status.Conditions,
					metav1.Condition{
						Type:   trainer.TrainJobComplete,
						Status: metav1.ConditionTrue,
						Reason: "JobFinished",
					},
				)

				trial0.Status.TrainerStatus = &trainer.TrainerStatus{
					Metrics: []trainer.Metric{
						{
							Name:  "accuracy",
							Value: "0.85",
						},
					},
				}

				gomega.Expect(
					k8sClient.Status().Update(ctx, trial0),
				).Should(gomega.Succeed())

				ginkgo.By("Waiting for Trial 1 to be spawned")
				trial1Key := client.ObjectKey{
					Name:      "test-optjob-trial-1",
					Namespace: ns.Name,
				}

				gomega.Eventually(func(g gomega.Gomega) {
					trial1 := &trainer.TrainJob{}

					g.Expect(k8sClient.Get(ctx, trial1Key, trial1)).Should(gomega.Succeed())
					g.Expect(trial1.OwnerReferences).Should(gomega.HaveLen(1))
					g.Expect(trial1.OwnerReferences[0].UID).Should(gomega.Equal(optJob.UID))
					g.Expect(trial1.Annotations["trainer.kubeflow.org/opt-param-lr"]).
						Should(gomega.Equal("0.01"))
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Simulating completion of Trial 1 with a BETTER metric (Accuracy: 0.95)")
				trial1 := &trainer.TrainJob{}

				gomega.Expect(
					k8sClient.Get(ctx, trial1Key, trial1),
				).Should(gomega.Succeed())

				meta.SetStatusCondition(
					&trial1.Status.Conditions,
					metav1.Condition{
						Type:   trainer.TrainJobComplete,
						Status: metav1.ConditionTrue,
						Reason: "JobFinished",
					},
				)

				trial1.Status.TrainerStatus = &trainer.TrainerStatus{
					Metrics: []trainer.Metric{
						{
							Name:  "accuracy",
							Value: "0.95",
						},
					},
				}

				gomega.Expect(
					k8sClient.Status().Update(ctx, trial1),
				).Should(gomega.Succeed())

				ginkgo.By("Checking that OptimizationJob completes with the best result from Trial 1")
				gomega.Eventually(func(g gomega.Gomega) {
					gotJob := &trainer.OptimizationJob{}

					g.Expect(
						k8sClient.Get(ctx, optJobKey, gotJob),
					).Should(gomega.Succeed())

					g.Expect(gotJob.Status).ShouldNot(gomega.BeNil())

					g.Expect(
						meta.IsStatusConditionTrue(
							gotJob.Status.Conditions,
							constants.OptimizationJobComplete,
						),
					).Should(gomega.BeTrue())

					g.Expect(gotJob.Status.Result).ShouldNot(gomega.BeNil())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.It("Should fail the OptimizationJob if all TrainJobs crash", func() {
				ginkgo.By("Creating OptimizationJob")
				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

				deployKey := client.ObjectKey{
					Name:      "test-optjob-optuna",
					Namespace: ns.Name,
				}

				ginkgo.By("Simulating Optuna Pod Readiness")
				gomega.Eventually(func(g gomega.Gomega) {
					deploy := &appsv1.Deployment{}

					g.Expect(k8sClient.Get(ctx, deployKey, deploy)).Should(gomega.Succeed())

					deploy.Status.Replicas = 1
					deploy.Status.ReadyReplicas = 1
					deploy.Status.AvailableReplicas = 1

					g.Expect(k8sClient.Status().Update(ctx, deploy)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				trial0Key := client.ObjectKey{
					Name:      "test-optjob-trial-0",
					Namespace: ns.Name,
				}

				ginkgo.By("Waiting for the first TrainJob (Trial 0)")
				gomega.Eventually(func(g gomega.Gomega) {
					trial0 := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, trial0Key, trial0)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Simulating Trial 0 Failure")
				trial0 := &trainer.TrainJob{}
				gomega.Expect(k8sClient.Get(ctx, trial0Key, trial0)).Should(gomega.Succeed())

				meta.SetStatusCondition(&trial0.Status.Conditions, metav1.Condition{
					Type:   trainer.TrainJobFailed,
					Status: metav1.ConditionTrue,
					Reason: "CrashLoopBackOff",
				})
				gomega.Expect(k8sClient.Status().Update(ctx, trial0)).Should(gomega.Succeed())

				trial1Key := client.ObjectKey{
					Name:      "test-optjob-trial-1",
					Namespace: ns.Name,
				}

				ginkgo.By("Waiting for the second TrainJob (Trial 1)")
				gomega.Eventually(func(g gomega.Gomega) {
					trial1 := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, trial1Key, trial1)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Simulating Trial 1 Failure")
				trial1 := &trainer.TrainJob{}
				gomega.Expect(k8sClient.Get(ctx, trial1Key, trial1)).Should(gomega.Succeed())

				meta.SetStatusCondition(&trial1.Status.Conditions, metav1.Condition{
					Type:   trainer.TrainJobFailed,
					Status: metav1.ConditionTrue,
					Reason: "CrashLoopBackOff",
				})
				gomega.Expect(k8sClient.Status().Update(ctx, trial1)).Should(gomega.Succeed())

				ginkgo.By("Verifying OptimizationJob marks itself as FAILED because all trials failed")
				gomega.Eventually(func() bool {
					gotJob := &trainer.OptimizationJob{}

					if err := k8sClient.Get(ctx, optJobKey, gotJob); err != nil {
						return false
					}

					if gotJob.Status == nil {
						return false
					}

					return meta.IsStatusConditionTrue(gotJob.Status.Conditions, constants.OptimizationJobFailed)
				}, util.Timeout, util.Interval).Should(gomega.BeTrue())
			})

			ginkgo.It("Should fail the OptimizationJob if the gRPC Suggestion Service fails", func() {
				ginkgo.By("Injecting a simulated gRPC connection error into the framework mock")
				mockClient.MockErr = fmt.Errorf("connection refused")

				ginkgo.By("Creating OptimizationJob")
				gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

				deployKey := client.ObjectKey{
					Name:      "test-optjob-optuna",
					Namespace: ns.Name,
				}

				ginkgo.By("Simulating Optuna Pod Readiness")
				gomega.Eventually(func(g gomega.Gomega) {
					deploy := &appsv1.Deployment{}

					g.Expect(k8sClient.Get(ctx, deployKey, deploy)).Should(gomega.Succeed())

					deploy.Status.Replicas = 1
					deploy.Status.ReadyReplicas = 1
					deploy.Status.AvailableReplicas = 1

					g.Expect(k8sClient.Status().Update(ctx, deploy)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())

				ginkgo.By("Verifying OptimizationJob marks itself as FAILED due to SuggestionServiceFailed")
				gomega.Eventually(func() bool {
					gotJob := &trainer.OptimizationJob{}

					if err := k8sClient.Get(ctx, optJobKey, gotJob); err != nil {
						return false
					}

					if gotJob.Status == nil {
						return false
					}

					return meta.IsStatusConditionTrue(gotJob.Status.Conditions, constants.OptimizationJobFailed)
				}, util.Timeout, util.Interval).Should(gomega.BeTrue())
			})
		})
	})
})
