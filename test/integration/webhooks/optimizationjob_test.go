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
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
)

var _ = ginkgo.Describe("OptimizationJob Webhook", ginkgo.Ordered, func() {
	optimizationJobName := "optimization-job"
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
		ginkgo.DescribeTable("Validate OptimizationJob on creation", func(optimizationJob func() *trainer.OptimizationJob, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, optimizationJob())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed in creating optimizationJob without a random search seed",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed in creating optimizationJob with a zero random search seed",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						SearchAlgorithm(&trainer.SearchAlgorithm{
							Random: &trainer.RandomAlgorithm{Seed: ptr.To[int64](0)},
						}).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed in creating optimizationJob with the maximum random search seed",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						SearchAlgorithm(&trainer.SearchAlgorithm{
							Random: &trainer.RandomAlgorithm{Seed: ptr.To[int64](4294967295)},
						}).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail in creating optimizationJob with a negative random search seed",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						SearchAlgorithm(&trainer.SearchAlgorithm{
							Random: &trainer.RandomAlgorithm{Seed: ptr.To[int64](-1)},
						}).
						Obj()
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with a random search seed above the maximum",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						SearchAlgorithm(&trainer.SearchAlgorithm{
							Random: &trainer.RandomAlgorithm{Seed: ptr.To[int64](4294967296)},
						}).
						Obj()
				},
				testingutil.BeInvalidError()),
		)
	})
})
