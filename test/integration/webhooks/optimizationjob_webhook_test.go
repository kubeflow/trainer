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
	"github.com/kubeflow/trainer/v2/test/util"
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

		// =====================================================================
		// 1. DEFAULTING INTEGRATION TESTS
		// =====================================================================
		ginkgo.DescribeTable("Defaulting OptimizationJob on creation", func(job func() *trainer.OptimizationJob, validateFunc func(*trainer.OptimizationJob)) {
			created := job()
			gomega.Expect(k8sClient.Create(ctx, created)).Should(gomega.Succeed())

			gomega.Eventually(func(g gomega.Gomega) {
				got := &trainer.OptimizationJob{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(created), got)).Should(gomega.Succeed())
				validateFunc(got)
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		},
			ginkgo.Entry("Should succeed to default ParallelTrials and NumTrials",
				func() *trainer.OptimizationJob {
					return &trainer.OptimizationJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-defaulting",
							Namespace: ns.Name,
						},
						Spec: trainer.OptimizationJobSpec{
							Objectives: []trainer.Objective{
								{Metric: "accuracy", Direction: "maximize"},
							},
							Parameters: []trainer.Parameter{
								{
									Name: "learning_rate",
									SearchSpace: trainer.SearchSpace{
										Uniform: &trainer.UniformSpace{Min: "0.01", Max: "0.1"},
									},
								},
							},
							SearchAlgorithm: trainer.SearchAlgorithm{
								Random: &trainer.RandomAlgorithm{},
							},
							TrialConfig: trainer.TrialConfig{
								// ParallelTrials and NumTrials are explicitly left nil to test defaulting
							},
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
				},
				func(got *trainer.OptimizationJob) {
					// Assert ParallelTrials defaults to 1
					gomega.Expect(got.Spec.TrialConfig.ParallelTrials).ToNot(gomega.BeNil())
					gomega.Expect(*got.Spec.TrialConfig.ParallelTrials).To(gomega.Equal(int32(1)))

					// Assert NumTrials defaults to 1
					gomega.Expect(got.Spec.TrialConfig.NumTrials).ToNot(gomega.BeNil())
					gomega.Expect(*got.Spec.TrialConfig.NumTrials).To(gomega.Equal(int32(1)))
				}),
		)
	})
})
