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
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
)

func makeOptimizationJob(namespace, name, parameterName string) *trainer.OptimizationJob {
	return &trainer.OptimizationJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: trainer.GroupVersion.String(),
			Kind:       "OptimizationJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: trainer.OptimizationJobSpec{
			Objectives: []trainer.Objective{{
				Metric:    "accuracy",
				Direction: trainer.ObjectiveDirectionMaximize,
			}},
			Parameters: []trainer.Parameter{{
				Name: parameterName,
				SearchSpace: &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{
						Min: "0.001",
						Max: "0.1",
					},
				},
			}},
			NumTrials:      1,
			ParallelTrials: 1,
			TrainJobTemplate: trainer.TrainJobTemplateSpec{
				Spec: trainer.TrainJobSpec{
					RuntimeRef: trainer.RuntimeRef{Name: "runtime"},
				},
			},
		},
	}
}

var _ = ginkgo.Describe("OptimizationJob API validation", ginkgo.Ordered, func() {
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
				GenerateName: "optimizationjob-validation-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
	})

	ginkgo.DescribeTable("validates parameter names at admission",
		func(parameterName string, shouldSucceed bool) {
			job := makeOptimizationJob(ns.Name, fmt.Sprintf("parameter-name-%d", ginkgo.GinkgoRandomSeed()), parameterName)
			err := k8sClient.Create(ctx, job)
			if shouldSucceed {
				gomega.Expect(err).Should(gomega.Succeed())
			} else {
				gomega.Expect(err).Should(testingutil.BeInvalidError())
			}
		},
		ginkgo.Entry("accepts underscore-separated names", "learning_rate", true),
		ginkgo.Entry("accepts names beginning with underscore", "_lr", true),
		ginkgo.Entry("accepts alphanumeric names", "batch_size2", true),
		ginkgo.Entry("rejects names containing slash", "weight/decay", false),
		ginkgo.Entry("rejects names containing spaces", "n layers", false),
		ginkgo.Entry("rejects names beginning with a digit", "1learning_rate", false),
		ginkgo.Entry("rejects names containing hyphen", "learning-rate", false),
	)
})
