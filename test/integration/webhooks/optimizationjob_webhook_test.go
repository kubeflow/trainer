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

package webhooks

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
)

var _ = ginkgo.Describe("OptimizationJob Webhook", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{}
		cfg = fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg, false)
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
				GenerateName: "optimizationjob-webhook-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
	})

	ginkgo.When("Creating OptimizationJob", func() {
		// makeJob builds a valid grid OptimizationJob: one categorical parameter
		// with 2 choices and numTrials=2, so the trial budget exactly fits the
		// grid. Each entry mutates one aspect to exercise a single rule.
		makeJob := func(mutate func(*trainer.OptimizationJob)) *trainer.OptimizationJob {
			job := &trainer.OptimizationJob{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-validation-",
					Namespace:    ns.Name,
				},
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{
							Metric:    "accuracy",
							Direction: trainer.ObjectiveDirectionMaximize,
						},
					},
					Parameters: []trainer.Parameter{
						{
							Name: "optimizer",
							SearchSpace: &trainer.SearchSpace{
								Categorical: trainer.CategoricalSpace{
									Choices: []string{"sgd", "adam"},
								},
							},
						},
					},
					SearchAlgorithm: &trainer.SearchAlgorithm{
						Grid: &trainer.GridAlgorithm{},
					},
					NumTrials:      2,
					ParallelTrials: 1,
					TrainJobTemplate: trainer.TrainJobTemplateSpec{
						Spec: trainer.TrainJobSpec{
							RuntimeRef: trainer.RuntimeRef{
								Name:     "dummy-runtime",
								APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
								Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
							},
							Trainer: &trainer.Trainer{
								Image: ptr.To("my-training-image:latest"),
							},
						},
					},
				},
			}
			if mutate != nil {
				mutate(job)
			}
			return job
		}

		ginkgo.DescribeTable("Validating OptimizationJob on creation", func(job func() *trainer.OptimizationJob, wantErr bool) {
			err := k8sClient.Create(ctx, job())
			if wantErr {
				gomega.Expect(err).To(gomega.HaveOccurred())
			} else {
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
		},
			ginkgo.Entry("Should accept grid search over categorical parameters",
				func() *trainer.OptimizationJob { return makeJob(nil) },
				false),
			ginkgo.Entry("Should reject grid search with a uniform parameter (CEL)",
				func() *trainer.OptimizationJob {
					return makeJob(func(j *trainer.OptimizationJob) {
						j.Spec.Parameters[0].SearchSpace = &trainer.SearchSpace{
							Uniform: trainer.UniformSpace{
								Min:  "0.01",
								Max:  "0.1",
								Type: trainer.ParameterTypeFloat,
							},
						}
					})
				},
				true),
			ginkgo.Entry("Should reject grid search with an Int uniform parameter (CEL)",
				func() *trainer.OptimizationJob {
					return makeJob(func(j *trainer.OptimizationJob) {
						j.Spec.Parameters[0].SearchSpace = &trainer.SearchSpace{
							Uniform: trainer.UniformSpace{
								Min:  "1",
								Max:  "10",
								Type: trainer.ParameterTypeInt,
							},
						}
					})
				},
				true),
			ginkgo.Entry("Should reject grid search with a logUniform parameter (CEL)",
				func() *trainer.OptimizationJob {
					return makeJob(func(j *trainer.OptimizationJob) {
						j.Spec.Parameters[0].SearchSpace = &trainer.SearchSpace{
							LogUniform: trainer.LogUniformSpace{
								Min:  "0.001",
								Max:  "0.1",
								Type: trainer.ParameterTypeFloat,
							},
						}
					})
				},
				true),
			ginkgo.Entry("Should reject grid search when numTrials exceeds combinations (webhook)",
				func() *trainer.OptimizationJob {
					return makeJob(func(j *trainer.OptimizationJob) {
						// 2 choices -> at most 2 combinations, ask for 3.
						j.Spec.NumTrials = 3
					})
				},
				true),
			ginkgo.Entry("Should accept random search with a continuous parameter",
				func() *trainer.OptimizationJob {
					return makeJob(func(j *trainer.OptimizationJob) {
						j.Spec.SearchAlgorithm = &trainer.SearchAlgorithm{
							Random: &trainer.RandomAlgorithm{},
						}
						j.Spec.Parameters[0].SearchSpace = &trainer.SearchSpace{
							Uniform: trainer.UniformSpace{
								Min:  "0.01",
								Max:  "0.1",
								Type: trainer.ParameterTypeFloat,
							},
						}
					})
				},
				false),
		)
	})
})
