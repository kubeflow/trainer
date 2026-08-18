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

var _ = ginkgo.Describe("OptimizationJob controller", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{}
		cfg = fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg, true)
	})
	ginkgo.AfterAll(func() {
		fwk.Teardown()
	})

	ginkgo.BeforeEach(func() {
		ns = &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       corev1.SchemeGroupVersion.String(),
				APIVersion: "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "optjob-envtest-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).Should(gomega.Succeed())
	})

	ginkgo.When("Reconciling OptimizationJob", func() {
		var (
			optJob    *trainer.OptimizationJob
			optJobKey client.ObjectKey
		)

		ginkgo.BeforeEach(func() {
			optJob = &trainer.OptimizationJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-optjob-e2e",
					Namespace: ns.Name,
				},
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "val_loss"},
					},
					Parameters: []trainer.Parameter{
						{
							Name: "learning_rate",
							SearchSpace: &trainer.SearchSpace{
								Categorical: trainer.CategoricalSpace{
									Choices: []string{"0.01", "0.001"},
								},
							},
						},
					},
					NumTrials:      2,
					ParallelTrials: 1,
					TrainJobTemplate: trainer.TrainJobTemplateSpec{
						Spec: trainer.TrainJobSpec{
							Trainer: &trainer.Trainer{
								Image: ptr.To("docker.io/my-org/model:latest"),
							},
						},
					},
				},
			}
			optJobKey = client.ObjectKeyFromObject(optJob)
		})

		ginkgo.AfterEach(func() {
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		})

		ginkgo.It("Should initialize Created status condition and provision trial TrainJob", func() {
			ginkgo.By("Creating an OptimizationJob")
			gomega.Expect(k8sClient.Create(ctx, optJob)).Should(gomega.Succeed())

			ginkgo.By("Verifying status conditions are initialized")
			gomega.Eventually(func(g gomega.Gomega) {
				var fetchedOptJob trainer.OptimizationJob
				g.Expect(k8sClient.Get(ctx, optJobKey, &fetchedOptJob)).Should(gomega.Succeed())
				g.Expect(fetchedOptJob.Status).ShouldNot(gomega.BeNil())
				g.Expect(fetchedOptJob.Status.Conditions).ShouldNot(gomega.BeEmpty())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Verifying trial TrainJob creation")
			gomega.Eventually(func(g gomega.Gomega) {
				var trainJobList trainer.TrainJobList
				g.Expect(k8sClient.List(ctx, &trainJobList, client.InNamespace(ns.Name))).Should(gomega.Succeed())
				g.Expect(trainJobList.Items).Should(gomega.HaveLen(1))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})
})
