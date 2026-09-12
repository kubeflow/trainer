/*
Copyright The Kubeflow Authors.

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
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	optutil "github.com/kubeflow/trainer/v2/pkg/util/optimizationjob"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

type mockIntegrationSuggestionClient struct {
	mu        sync.RWMutex
	responses [][][]trainer.ParameterAssignment
	respIndex int
	mockErr   error
}

func (m *mockIntegrationSuggestionClient) SetResponses(responses [][][]trainer.ParameterAssignment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = responses
	m.respIndex = 0
}

func (m *mockIntegrationSuggestionClient) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockErr = err
}

func (m *mockIntegrationSuggestionClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = nil
	m.respIndex = 0
	m.mockErr = nil
}

func (m *mockIntegrationSuggestionClient) GetSuggestions(
	ctx context.Context,
	addr string,
	req *katibapi.GetSuggestionsRequest,
) ([][]trainer.ParameterAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mockErr != nil {
		return nil, m.mockErr
	}
	if len(m.responses) > 0 {
		idx := m.respIndex
		if idx >= len(m.responses) {
			idx = len(m.responses) - 1
		}
		m.respIndex++
		return m.responses[idx], nil
	}

	var result [][]trainer.ParameterAssignment
	for i := int32(0); i < req.CurrentRequestNumber; i++ {
		result = append(result, []trainer.ParameterAssignment{
			{Name: "lr", Value: "0.01"},
		})
	}
	return result, nil
}

var mockClient = &mockIntegrationSuggestionClient{}

func newTestOptimizationJob(namespace, name string, numTrials, parallelTrials int32) *trainer.OptimizationJob {
	return &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: trainer.OptimizationJobSpec{
			NumTrials:      numTrials,
			ParallelTrials: parallelTrials,
			SearchAlgorithm: &trainer.SearchAlgorithm{
				Random: &trainer.RandomAlgorithm{},
			},
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
						Name:     "alpha",
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
}

func makeAlgorithmDeploymentReady(optJob *trainer.OptimizationJob) {
	deployKey := client.ObjectKey{
		Name:      optutil.GetAlgorithmServiceName(optJob),
		Namespace: optJob.Namespace,
	}
	gomega.Eventually(func(g gomega.Gomega) {
		deploy := &appsv1.Deployment{}
		g.Expect(k8sClient.Get(ctx, deployKey, deploy)).Should(gomega.Succeed())
		deploy.Status.Replicas = 1
		deploy.Status.ReadyReplicas = 1
		deploy.Status.AvailableReplicas = 1
		g.Expect(k8sClient.Status().Update(ctx, deploy)).Should(gomega.Succeed())
	}, util.Timeout, util.Interval).Should(gomega.Succeed())
}

func getChildTrainJobs(optJob *trainer.OptimizationJob) ([]trainer.TrainJob, error) {
	var trainJobs trainer.TrainJobList
	if err := k8sClient.List(ctx, &trainJobs, client.InNamespace(optJob.Namespace), client.MatchingLabels{
		constants.OptimizationJobNameLabel: optJob.Name,
	}); err != nil {
		return nil, err
	}
	var owned []trainer.TrainJob
	for _, tj := range trainJobs.Items {
		if owner := metav1.GetControllerOf(&tj); owner != nil && owner.UID == optJob.UID {
			owned = append(owned, tj)
		}
	}
	return owned, nil
}

var _ = ginkgo.Describe("OptimizationJob Controller", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{
			SuggestionClient: mockClient,
		}
		cfg = fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg, true)
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

		trainingRuntime := utiltesting.MakeTrainingRuntimeWrapper(ns.Name, "alpha").Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())

		mockClient.Reset()
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainingRuntime{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &corev1.Service{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
	})

	ginkgo.Context("Algorithm Service Provisioning", func() {
		ginkgo.It("Should create search algorithm Deployment and Service, and wait for readiness", func() {
			optJob := newTestOptimizationJob(ns.Name, "test-provisioning", 2, 1)

			ginkgo.By("Creating OptimizationJob")
			gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

			serviceName := optutil.GetAlgorithmServiceName(optJob)
			deployKey := client.ObjectKey{Name: serviceName, Namespace: ns.Name}
			svcKey := client.ObjectKey{Name: serviceName, Namespace: ns.Name}

			ginkgo.By("Waiting for the Search Algorithm Deployment to be created with correct OwnerReference")
			gomega.Eventually(func(g gomega.Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, deployKey, deploy)).Should(gomega.Succeed())
				g.Expect(deploy.OwnerReferences).Should(gomega.HaveLen(1))
				g.Expect(deploy.OwnerReferences[0].UID).Should(gomega.Equal(optJob.UID))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Waiting for the Search Algorithm Service to be created with correct port")
			gomega.Eventually(func(g gomega.Gomega) {
				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, svcKey, svc)).Should(gomega.Succeed())
				g.Expect(svc.OwnerReferences).Should(gomega.HaveLen(1))
				g.Expect(svc.OwnerReferences[0].UID).Should(gomega.Equal(optJob.UID))
				g.Expect(svc.Spec.Ports).ShouldNot(gomega.BeEmpty())
				g.Expect(svc.Spec.Ports[0].Port).Should(gomega.Equal(int32(constants.SearchAlgorithmServicePort)))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Simulating algorithm pod readiness")
			makeAlgorithmDeploymentReady(optJob)

			ginkgo.By("Checking that OptimizationJob transitions to Created=True")
			gomega.Eventually(func(g gomega.Gomega) {
				gotJob := &trainer.OptimizationJob{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optJob), gotJob)).Should(gomega.Succeed())
				g.Expect(gotJob.Status).ShouldNot(gomega.BeNil())
				g.Expect(meta.IsStatusConditionTrue(gotJob.Status.Conditions, constants.OptimizationJobCreated)).Should(gomega.BeTrue())
				cond := meta.FindStatusCondition(gotJob.Status.Conditions, constants.OptimizationJobCreated)
				g.Expect(cond.Reason).Should(gomega.Equal("AlgorithmServiceCreated"))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})

	ginkgo.Context("Trial Spawning and Parameter Assignment", func() {
		ginkgo.It("Should spawn a TrainJob with parameters injected as environment variables", func() {
			mockClient.SetResponses([][][]trainer.ParameterAssignment{
				{
					{
						{Name: "lr", Value: "0.01"},
						{Name: "batch_size", Value: "32"},
					},
				},
			})

			optJob := newTestOptimizationJob(ns.Name, "test-spawn", 1, 1)

			ginkgo.By("Creating OptimizationJob")
			gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

			ginkgo.By("Making algorithm service ready")
			makeAlgorithmDeploymentReady(optJob)

			ginkgo.By("Waiting for child TrainJob to be spawned")
			var childJob trainer.TrainJob
			gomega.Eventually(func(g gomega.Gomega) {
				jobs, err := getChildTrainJobs(optJob)
				g.Expect(err).Should(gomega.Succeed())
				g.Expect(jobs).Should(gomega.HaveLen(1))
				childJob = jobs[0]
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Verifying child TrainJob metadata and environment variables")
			gomega.Expect(childJob.OwnerReferences).Should(gomega.HaveLen(1))
			gomega.Expect(childJob.OwnerReferences[0].UID).Should(gomega.Equal(optJob.UID))
			gomega.Expect(childJob.Labels[constants.OptimizationJobNameLabel]).Should(gomega.Equal(optJob.Name))

			gomega.Expect(childJob.Spec.Trainer).ShouldNot(gomega.BeNil())
			envMap := make(map[string]string)
			for _, env := range childJob.Spec.Trainer.Env {
				envMap[env.Name] = env.Value
			}
			gomega.Expect(envMap[fmt.Sprintf("%slr", constants.EnvVarPrefix)]).Should(gomega.Equal("0.01"))
			gomega.Expect(envMap[fmt.Sprintf("%sbatch_size", constants.EnvVarPrefix)]).Should(gomega.Equal("32"))
		})
	})

	ginkgo.Context("Sequential Execution, Best Result Tracking and Automated Cleanup", func() {
		ginkgo.It("Should respect concurrency, update best result, and clean up algorithm service upon completion", func() {
			mockClient.SetResponses([][][]trainer.ParameterAssignment{
				{{{Name: "lr", Value: "0.01"}}},
				{{{Name: "lr", Value: "0.05"}}},
			})

			optJob := newTestOptimizationJob(ns.Name, "test-lifecycle", 2, 1)

			ginkgo.By("Creating OptimizationJob with NumTrials=2, ParallelTrials=1")
			gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

			ginkgo.By("Making algorithm service ready")
			makeAlgorithmDeploymentReady(optJob)

			ginkgo.By("Waiting for the first TrainJob (Trial 1) to be spawned")
			var trial1 trainer.TrainJob
			gomega.Eventually(func(g gomega.Gomega) {
				jobs, err := getChildTrainJobs(optJob)
				g.Expect(err).Should(gomega.Succeed())
				g.Expect(jobs).Should(gomega.HaveLen(1))
				trial1 = jobs[0]
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Ensuring no second trial is spawned while Trial 1 is still active (parallelTrials=1)")
			gomega.Consistently(func(g gomega.Gomega) {
				jobs, err := getChildTrainJobs(optJob)
				g.Expect(err).Should(gomega.Succeed())
				g.Expect(jobs).Should(gomega.HaveLen(1))
			}, 1*time.Second, 200*time.Millisecond).Should(gomega.Succeed())

			ginkgo.By("Simulating completion of Trial 1 with accuracy=0.85")
			gomega.Eventually(func(g gomega.Gomega) {
				freshTrial := &trainer.TrainJob{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&trial1), freshTrial)).Should(gomega.Succeed())
				meta.SetStatusCondition(&freshTrial.Status.Conditions, metav1.Condition{
					Type:    trainer.TrainJobComplete,
					Status:  metav1.ConditionTrue,
					Reason:  "JobFinished",
					Message: "Trial 1 finished successfully",
				})
				freshTrial.Status.TrainerStatus = &trainer.TrainerStatus{
					LastUpdatedTime: metav1.Now(),
					Metrics:         []trainer.Metric{{Name: "accuracy", Value: "0.85"}},
				}
				g.Expect(k8sClient.Status().Update(ctx, freshTrial)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Waiting for the second TrainJob (Trial 2) to be spawned")
			var trial2 trainer.TrainJob
			gomega.Eventually(func(g gomega.Gomega) {
				jobs, err := getChildTrainJobs(optJob)
				g.Expect(err).Should(gomega.Succeed())
				g.Expect(jobs).Should(gomega.HaveLen(2))
				for _, j := range jobs {
					if j.Name != trial1.Name {
						trial2 = j
						break
					}
				}
				g.Expect(trial2.Name).ShouldNot(gomega.BeEmpty())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Simulating completion of Trial 2 with a better accuracy=0.95")
			gomega.Eventually(func(g gomega.Gomega) {
				freshTrial := &trainer.TrainJob{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&trial2), freshTrial)).Should(gomega.Succeed())
				meta.SetStatusCondition(&freshTrial.Status.Conditions, metav1.Condition{
					Type:    trainer.TrainJobComplete,
					Status:  metav1.ConditionTrue,
					Reason:  "JobFinished",
					Message: "Trial 2 finished successfully",
				})
				freshTrial.Status.TrainerStatus = &trainer.TrainerStatus{
					LastUpdatedTime: metav1.Now(),
					Metrics:         []trainer.Metric{{Name: "accuracy", Value: "0.95"}},
				}
				g.Expect(k8sClient.Status().Update(ctx, freshTrial)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Verifying OptimizationJob reaches Complete status condition")
			optJobKey := client.ObjectKeyFromObject(optJob)
			gomega.Eventually(func(g gomega.Gomega) {
				gotJob := &trainer.OptimizationJob{}
				g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())
				g.Expect(gotJob.Status).ShouldNot(gomega.BeNil())
				g.Expect(meta.IsStatusConditionTrue(gotJob.Status.Conditions, constants.OptimizationJobComplete)).Should(gomega.BeTrue())
				cond := meta.FindStatusCondition(gotJob.Status.Conditions, constants.OptimizationJobComplete)
				g.Expect(cond.Reason).Should(gomega.Equal("OptimizationJobCompleted"))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Verifying Status.Result points to Trial 2 (accuracy=0.95)")
			gotJob := &trainer.OptimizationJob{}
			gomega.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())
			gomega.Expect(gotJob.Status.Result.TrainJobName).Should(gomega.Equal(trial2.Name))
			gomega.Expect(gotJob.Status.Result.Parameters).Should(gomega.ContainElement(trainer.ParameterAssignment{
				Name:  "lr",
				Value: "0.05",
			}))

			ginkgo.By("Verifying automated cleanup: Search Algorithm Deployment and Service are deleted")
			serviceName := optutil.GetAlgorithmServiceName(optJob)
			gomega.Eventually(func(g gomega.Gomega) {
				deploy := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: ns.Name}, deploy)
				g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())

				svc := &corev1.Service{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: ns.Name}, svc)
				g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})

	ginkgo.Context("Failure Handling", func() {
		ginkgo.It("Should mark OptimizationJob as Failed and cleanup when a trial fails", func() {
			optJob := newTestOptimizationJob(ns.Name, "test-trial-failure", 2, 1)

			ginkgo.By("Creating OptimizationJob")
			gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

			ginkgo.By("Making algorithm service ready")
			makeAlgorithmDeploymentReady(optJob)

			ginkgo.By("Waiting for the first TrainJob to be spawned")
			var trial1 trainer.TrainJob
			gomega.Eventually(func(g gomega.Gomega) {
				jobs, err := getChildTrainJobs(optJob)
				g.Expect(err).Should(gomega.Succeed())
				g.Expect(jobs).Should(gomega.HaveLen(1))
				trial1 = jobs[0]
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Simulating Trial 1 failure")
			gomega.Eventually(func(g gomega.Gomega) {
				freshTrial := &trainer.TrainJob{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&trial1), freshTrial)).Should(gomega.Succeed())
				meta.SetStatusCondition(&freshTrial.Status.Conditions, metav1.Condition{
					Type:    trainer.TrainJobFailed,
					Status:  metav1.ConditionTrue,
					Reason:  "PodFailed",
					Message: "Trial pod terminated with error",
				})
				g.Expect(k8sClient.Status().Update(ctx, freshTrial)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Verifying OptimizationJob transitions to Failed=True with Reason TrialFailed")
			optJobKey := client.ObjectKeyFromObject(optJob)
			gomega.Eventually(func(g gomega.Gomega) {
				gotJob := &trainer.OptimizationJob{}
				g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())
				g.Expect(gotJob.Status).ShouldNot(gomega.BeNil())
				g.Expect(meta.IsStatusConditionTrue(gotJob.Status.Conditions, constants.OptimizationJobFailed)).Should(gomega.BeTrue())
				cond := meta.FindStatusCondition(gotJob.Status.Conditions, constants.OptimizationJobFailed)
				g.Expect(cond.Reason).Should(gomega.Equal("TrialFailed"))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Verifying automated cleanup deletes algorithm Deployment and Service")
			serviceName := optutil.GetAlgorithmServiceName(optJob)
			gomega.Eventually(func(g gomega.Gomega) {
				deploy := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: ns.Name}, deploy)
				g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())

				svc := &corev1.Service{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: ns.Name}, svc)
				g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})

		ginkgo.It("Should mark OptimizationJob as Failed when SearchAlgorithm is unsupported", func() {
			optJob := newTestOptimizationJob(ns.Name, "test-unsupported-algo", 2, 1)
			optJob.Spec.SearchAlgorithm = &trainer.SearchAlgorithm{
				Grid: &trainer.GridAlgorithm{},
			}
			optJob.Spec.Parameters = []trainer.Parameter{
				{
					Name: "model_type",
					SearchSpace: &trainer.SearchSpace{
						Categorical: trainer.CategoricalSpace{
							Choices: []string{"resnet", "vit"},
						},
					},
				},
			}

			ginkgo.By("Creating OptimizationJob with unsupported search algorithm (grid)")
			gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

			ginkgo.By("Verifying OptimizationJob transitions to Failed=True with Reason UnsupportedSearchAlgorithm")
			optJobKey := client.ObjectKeyFromObject(optJob)
			gomega.Eventually(func(g gomega.Gomega) {
				gotJob := &trainer.OptimizationJob{}
				g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())
				g.Expect(gotJob.Status).ShouldNot(gomega.BeNil())
				g.Expect(meta.IsStatusConditionTrue(gotJob.Status.Conditions, constants.OptimizationJobFailed)).Should(gomega.BeTrue())
				cond := meta.FindStatusCondition(gotJob.Status.Conditions, constants.OptimizationJobFailed)
				g.Expect(cond.Reason).Should(gomega.Equal("UnsupportedSearchAlgorithm"))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Verifying that no algorithm Deployment or Service was created")
			serviceName := optutil.GetAlgorithmServiceName(optJob)
			deploy := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: ns.Name}, deploy)
			gomega.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
		})

		ginkgo.It("Should retry on transient suggestion service errors without failing prematurely", func() {
			mockClient.SetError(fmt.Errorf("simulated transient gRPC failure"))

			optJob := newTestOptimizationJob(ns.Name, "test-transient-error", 1, 1)

			ginkgo.By("Creating OptimizationJob")
			gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

			ginkgo.By("Making algorithm service ready")
			makeAlgorithmDeploymentReady(optJob)

			ginkgo.By("Verifying OptimizationJob does not enter Failed state during transient error")
			optJobKey := client.ObjectKeyFromObject(optJob)
			gomega.Consistently(func(g gomega.Gomega) {
				gotJob := &trainer.OptimizationJob{}
				g.Expect(k8sClient.Get(ctx, optJobKey, gotJob)).Should(gomega.Succeed())
				if gotJob.Status != nil {
					g.Expect(meta.IsStatusConditionTrue(gotJob.Status.Conditions, constants.OptimizationJobFailed)).Should(gomega.BeFalse())
				}
			}, 1*time.Second, 200*time.Millisecond).Should(gomega.Succeed())

			ginkgo.By("Clearing transient error")
			mockClient.SetError(nil)

			ginkgo.By("Waiting for child TrainJob to be spawned after retry")
			gomega.Eventually(func(g gomega.Gomega) {
				jobs, err := getChildTrainJobs(optJob)
				g.Expect(err).Should(gomega.Succeed())
				g.Expect(jobs).Should(gomega.HaveLen(1))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})
})
