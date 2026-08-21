/*
Copyright 2025 The Kubeflow Authors.

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
)

// makeIntBoundsOptimizationJob builds a minimal valid OptimizationJob whose
// single parameter carries the given search space, so each table entry below
// only has to describe the distribution under test.
func makeIntBoundsOptimizationJob(namespace string, searchSpace *trainer.SearchSpace) *trainer.OptimizationJob {
	return &trainer.OptimizationJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: trainer.GroupVersion.String(),
			Kind:       "OptimizationJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "optimizationjob-",
			Namespace:    namespace,
		},
		Spec: trainer.OptimizationJobSpec{
			Objectives: []trainer.Objective{{
				Metric:    "accuracy",
				Direction: trainer.ObjectiveDirectionMaximize,
			}},
			Parameters: []trainer.Parameter{{
				Name:        "batch_size",
				SearchSpace: searchSpace,
			}},
			TrainJobTemplate: trainer.TrainJobTemplateSpec{
				Spec: trainer.TrainJobSpec{
					RuntimeRef: trainer.RuntimeRef{
						Name: "torch-distributed",
					},
				},
			},
		},
	}
}

var _ = ginkgo.Describe("OptimizationJob Int search space bounds validation", ginkgo.Ordered, func() {
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
				GenerateName: "optimizationjob-int-bounds-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
	})

	ginkgo.DescribeTable("Validate integral bounds for Int-typed uniform and logUniform search spaces on creation",
		func(searchSpace func() *trainer.SearchSpace, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, makeIntBoundsOptimizationJob(ns.Name, searchSpace()))).Should(errorMatcher)
		},

		// uniform, type: Int - rejected.
		ginkgo.Entry("Should fail to create OptimizationJob with a fractional Int uniform range",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1.5", Max: "8.7", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with an Int uniform range containing no integer",
			// [1.2, 1.8] holds no integer at all, so the search space is
			// unsatisfiable rather than merely imprecise.
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1.2", Max: "1.8", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with a fractional Int uniform min only",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "0.5", Max: "10", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with a fractional Int uniform max only",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1", Max: "10.5", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with a fractional scientific-notation Int uniform bound",
			// 1e-3 is pattern-valid and finite, but not a whole number.
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1e-3", Max: "1e2", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with an Int uniform bound beyond 2^53",
			// Above 2^53 a float64 cannot represent consecutive integers, so
			// integrality is no longer a meaningful property of the value.
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1", Max: "1e300", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),

		// uniform, type: Int - accepted.
		ginkgo.Entry("Should succeed to create OptimizationJob with a whole-number Int uniform range",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1", Max: "8", Type: trainer.ParameterTypeInt},
				}
			},
			gomega.Succeed()),
		ginkgo.Entry("Should succeed to create OptimizationJob with integral Int uniform bounds written as decimals",
			// "1.0" is a whole number even though it is spelled with a
			// fractional part, so it must not be rejected.
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1.0", Max: "8.0", Type: trainer.ParameterTypeInt},
				}
			},
			gomega.Succeed()),
		ginkgo.Entry("Should succeed to create OptimizationJob with integral Int uniform bounds in scientific notation",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "1e2", Max: "1e3", Type: trainer.ParameterTypeInt},
				}
			},
			gomega.Succeed()),
		ginkgo.Entry("Should succeed to create OptimizationJob with a negative whole-number Int uniform range",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "-8", Max: "-1", Type: trainer.ParameterTypeInt},
				}
			},
			gomega.Succeed()),

		// uniform, type: Float and defaulted type - untouched by the new rule.
		ginkgo.Entry("Should succeed to create OptimizationJob with a fractional Float uniform range",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "0.001", Max: "0.1", Type: trainer.ParameterTypeFloat},
				}
			},
			gomega.Succeed()),
		ginkgo.Entry("Should succeed to create OptimizationJob with a fractional uniform range and a defaulted type",
			// type defaults to Float, and defaulting runs before CEL, so the
			// guard is evaluable and the rule must not fire here.
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Uniform: trainer.UniformSpace{Min: "0.001", Max: "0.1"},
				}
			},
			gomega.Succeed()),

		// logUniform, type: Int - rejected.
		ginkgo.Entry("Should fail to create OptimizationJob with a fractional Int logUniform range",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					LogUniform: trainer.LogUniformSpace{Min: "1.5", Max: "8.7", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with an Int logUniform range containing no integer",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					LogUniform: trainer.LogUniformSpace{Min: "1.2", Max: "1.8", Type: trainer.ParameterTypeInt},
				}
			},
			testingutil.BeInvalidError()),

		// logUniform, type: Int - accepted.
		ginkgo.Entry("Should succeed to create OptimizationJob with a whole-number Int logUniform range",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					LogUniform: trainer.LogUniformSpace{Min: "1", Max: "1024", Type: trainer.ParameterTypeInt},
				}
			},
			gomega.Succeed()),

		// logUniform, type: Float - untouched by the new rule.
		ginkgo.Entry("Should succeed to create OptimizationJob with a fractional Float logUniform range",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					LogUniform: trainer.LogUniformSpace{Min: "0.0001", Max: "0.1", Type: trainer.ParameterTypeFloat},
				}
			},
			gomega.Succeed()),
	)
})
