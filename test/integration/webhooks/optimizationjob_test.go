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
	"strings"

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
			// Parameter names become environment variables; reject unusable names at admission.
			ginkgo.Entry("Should succeed in creating optimizationJob with underscore-separated parameters",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "learning_rate"
					return optimizationJob
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed in creating optimizationJob with parameters beginning with underscore",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "_lr"
					return optimizationJob
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed in creating optimizationJob with alphanumeric parameters",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "batch_size2"
					return optimizationJob
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed in creating optimizationJob with uppercase parameters",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "LearningRate"
					return optimizationJob
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail in creating optimizationJob with parameters containing slash",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "weight/decay"
					return optimizationJob
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with parameters containing spaces",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "n layers"
					return optimizationJob
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with parameters beginning with a digit",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "1learning_rate"
					return optimizationJob
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with parameters containing hyphen",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "learning-rate"
					return optimizationJob
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with parameters containing equals sign",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "a=b"
					return optimizationJob
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with non-ASCII parameters",
				func() *trainer.OptimizationJob {
					optimizationJob := testingutil.MakeOptimizationJobWrapper(ns.Name, optimizationJobName).
						Obj()
					optimizationJob.Spec.Parameters[0].Name = "lré"
					return optimizationJob
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should succeed in creating optimizationJob with a valid RFC 1035 name",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, "valid-job-name").Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed in creating optimizationJob with a 63-character name",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, strings.Repeat("a", 63)).Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail in creating optimizationJob with uppercase letters in the name",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, "Invalid-job-name").Obj()
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with a name starting with a digit",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, "1jobname").Obj()
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with a dot in the name",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, "job.name").Obj()
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail in creating optimizationJob with a name ending with a hyphen",
				func() *trainer.OptimizationJob {
					return testingutil.MakeOptimizationJobWrapper(ns.Name, "jobname-").Obj()
				},
				testingutil.BeInvalidError()),
		)
	})
})
