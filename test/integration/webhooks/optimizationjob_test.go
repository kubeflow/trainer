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

// makeOptimizationJob builds a minimal valid OptimizationJob whose single
// parameter carries the given search space, so each table entry below only has
// to describe the distribution under test.
func makeOptimizationJob(namespace string, searchSpace *trainer.SearchSpace) *trainer.OptimizationJob {
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
				Name:        "lr",
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

var _ = ginkgo.Describe("OptimizationJob SearchSpace marker validations", ginkgo.Ordered, func() {
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
				GenerateName: "optimizationjob-searchspace-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.OptimizationJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
	})

	ginkgo.DescribeTable("Validate normal and logNormal search spaces on creation",
		func(searchSpace func() *trainer.SearchSpace, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, makeOptimizationJob(ns.Name, searchSpace()))).Should(errorMatcher)
		},
		ginkgo.Entry("Should succeed to create OptimizationJob with a normal search space",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Normal: trainer.NormalSpace{Mean: "0.1", StdDev: "0.05"},
				}
			},
			gomega.Succeed()),
		ginkgo.Entry("Should succeed to create OptimizationJob with a negative normal mean",
			// Mean is unconstrained: a Gaussian may legitimately be centered
			// below zero.
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Normal: trainer.NormalSpace{Mean: "-2.5", StdDev: "1"},
				}
			},
			gomega.Succeed()),
		ginkgo.Entry("Should fail to create OptimizationJob with a zero normal stdDev",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Normal: trainer.NormalSpace{Mean: "0.1", StdDev: "0"},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with a negative normal stdDev",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Normal: trainer.NormalSpace{Mean: "0.1", StdDev: "-0.05"},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should succeed to create OptimizationJob with a logNormal search space",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					LogNormal: trainer.LogNormalSpace{Mean: "-3", StdDev: "1"},
				}
			},
			gomega.Succeed()),
		ginkgo.Entry("Should fail to create OptimizationJob with a zero logNormal stdDev",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					LogNormal: trainer.LogNormalSpace{Mean: "-3", StdDev: "0"},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with a negative logNormal stdDev",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					LogNormal: trainer.LogNormalSpace{Mean: "-3", StdDev: "-1"},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with both normal and logNormal set",
			// The union is ExactlyOneOf, so the new members have to be mutually
			// exclusive with each other as well as with the existing ones.
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Normal:    trainer.NormalSpace{Mean: "0.1", StdDev: "0.05"},
					LogNormal: trainer.LogNormalSpace{Mean: "-3", StdDev: "1"},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with both normal and uniform set",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Normal:  trainer.NormalSpace{Mean: "0.1", StdDev: "0.05"},
					Uniform: trainer.UniformSpace{Min: "0", Max: "1"},
				}
			},
			testingutil.BeInvalidError()),
		ginkgo.Entry("Should fail to create OptimizationJob with a non-numeric normal stdDev",
			func() *trainer.SearchSpace {
				return &trainer.SearchSpace{
					Normal: trainer.NormalSpace{Mean: "0.1", StdDev: "abc"},
				}
			},
			testingutil.BeInvalidError()),
	)

	ginkgo.It("Should fail to create a grid search OptimizationJob with a normal parameter", func() {
		// Grid enumerates a finite cartesian product, which a continuous
		// distribution cannot provide. The pre-existing CEL rule on
		// OptimizationJobSpec covers the new distributions for free.
		job := makeOptimizationJob(ns.Name, &trainer.SearchSpace{
			Normal: trainer.NormalSpace{Mean: "0.1", StdDev: "0.05"},
		})
		job.Spec.SearchAlgorithm = &trainer.SearchAlgorithm{Grid: &trainer.GridAlgorithm{}}
		gomega.Expect(k8sClient.Create(ctx, job)).Should(testingutil.BeInvalidError())
	})
})
