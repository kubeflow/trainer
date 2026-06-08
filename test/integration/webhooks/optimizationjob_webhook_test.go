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
			ginkgo.Entry("Should succeed to default Provider, ParallelTrials, and NumTrials",
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
								{Name: "learning_rate", SearchSpace: trainer.SearchSpace{Type: "double", Min: "0.01", Max: "0.1"}},
							},
							Algorithm: trainer.Algorithm{
								Name: "random",
								// Provider is nil
							},
							TrialConfig: trainer.TrialConfig{
								// ParallelTrials and NumTrials are nil
							},
							TrainJobTemplate: trainer.TrainJobTemplateSpec{
								Spec: trainer.TrainJobSpec{
									RuntimeRef: trainer.RuntimeRef{
										Name:     "dummy-runtime",
										APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
										Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
									},
									Trainer: &trainer.Trainer{
										Image: ptr.To("my-training-image:{{.learning_rate}}"),
									},
								},
							},
						},
					}
				},
				func(got *trainer.OptimizationJob) {
					gomega.Expect(got.Spec.Algorithm.Provider).ToNot(gomega.BeNil())
					gomega.Expect(*got.Spec.Algorithm.Provider).To(gomega.Equal("optuna"))

					gomega.Expect(got.Spec.TrialConfig.ParallelTrials).ToNot(gomega.BeNil())
					gomega.Expect(*got.Spec.TrialConfig.ParallelTrials).To(gomega.Equal(int32(1)))

					gomega.Expect(got.Spec.TrialConfig.NumTrials).ToNot(gomega.BeNil())
					gomega.Expect(*got.Spec.TrialConfig.NumTrials).To(gomega.Equal(int32(1)))
				}),
		)

		// =====================================================================
		// 2. VALIDATION INTEGRATION TESTS (String Templating)
		// =====================================================================
		ginkgo.DescribeTable("Validate OptimizationJob on creation", func(job func() *trainer.OptimizationJob, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, job())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed when string template placeholders match parameters",
				func() *trainer.OptimizationJob {
					return &trainer.OptimizationJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "valid-template",
							Namespace: ns.Name,
						},
						Spec: trainer.OptimizationJobSpec{
							Objectives: []trainer.Objective{
								{Metric: "accuracy", Direction: "maximize"},
							},
							Algorithm: trainer.Algorithm{Name: "random", Provider: ptr.To("optuna")},
							Parameters: []trainer.Parameter{
								{Name: "learning_rate", SearchSpace: trainer.SearchSpace{Type: "double", Min: "0.01", Max: "0.1"}},
							},
							TrainJobTemplate: trainer.TrainJobTemplateSpec{
								Spec: trainer.TrainJobSpec{
									RuntimeRef: trainer.RuntimeRef{
										Name:     "dummy-runtime",
										APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
										Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
									},
									Trainer: &trainer.Trainer{
										Image: ptr.To("my-training-image:{{.learning_rate}}"),
									},
								},
							},
						},
					}
				},
				gomega.Succeed()),

			ginkgo.Entry("Should fail to create when string template placeholders are missing",
				func() *trainer.OptimizationJob {
					return &trainer.OptimizationJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "invalid-template-missing-placeholder",
							Namespace: ns.Name,
						},
						Spec: trainer.OptimizationJobSpec{
							Objectives: []trainer.Objective{
								{Metric: "accuracy", Direction: "maximize"},
							},
							Algorithm: trainer.Algorithm{Name: "random", Provider: ptr.To("optuna")},
							Parameters: []trainer.Parameter{
								{Name: "learning_rate", SearchSpace: trainer.SearchSpace{Type: "double", Min: "0.01", Max: "0.1"}},
							},
							TrainJobTemplate: trainer.TrainJobTemplateSpec{
								Spec: trainer.TrainJobSpec{
									RuntimeRef: trainer.RuntimeRef{
										Name:     "dummy-runtime",
										APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
										Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
									},
									Trainer: &trainer.Trainer{
										Image: ptr.To("my-training-image"), // MISSING: :{{.learning_rate}}
									},
								},
							},
						},
					}
				},
				gomega.SatisfyAll(
					gomega.HaveOccurred(),
					gomega.WithTransform(func(err error) string {
						return err.Error()
					}, gomega.ContainSubstring("Parameter 'learning_rate' is defined")),
				)),
		)
	})
})
