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

var _ = ginkgo.Describe("OptimizationJob random search seed validation", ginkgo.Ordered, func() {
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
				GenerateName: "optimizationjob-seed-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
	})

	ginkgo.DescribeTable("validates the random search seed at admission",
		func(seed *int64, errorMatcher gomega.OmegaMatcher) {
			job := testingutil.MakeOptimizationJobWrapper(ns.Name, "optimization-job").
				Objectives(trainer.Objective{
					Metric:    "accuracy",
					Direction: trainer.ObjectiveDirectionMaximize,
				}).
				SearchAlgorithm(&trainer.SearchAlgorithm{
					Random: &trainer.RandomAlgorithm{Seed: seed},
				}).
				Parameters(trainer.Parameter{
					Name: "lr",
					SearchSpace: &trainer.SearchSpace{
						Uniform: trainer.UniformSpace{
							Min: "0.001",
							Max: "0.1",
						},
					},
				}).
				NumTrials(1).
				ParallelTrials(1).
				TrainJobTemplate(trainer.TrainJobTemplateSpec{
					Spec: trainer.TrainJobSpec{
						RuntimeRef: trainer.RuntimeRef{Name: "runtime"},
					},
				}).
				Obj()
			gomega.Expect(k8sClient.Create(ctx, job)).Should(errorMatcher)
		},
		ginkgo.Entry("Should succeed without a seed", nil, gomega.Succeed()),
		ginkgo.Entry("Should succeed with a zero seed", ptr.To[int64](0), gomega.Succeed()),
		ginkgo.Entry("Should succeed with the maximum seed", ptr.To[int64](4294967295), gomega.Succeed()),
		ginkgo.Entry("Should fail with a negative seed", ptr.To[int64](-1), testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail with a seed above the maximum", ptr.To[int64](4294967296), testingutil.BeInvalidError()),
	)
})
