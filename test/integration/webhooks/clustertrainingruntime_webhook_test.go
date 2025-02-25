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

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
	"github.com/kubeflow/trainer/test/integration/framework"
)

const clTrainingRuntimeName = "test-clustertrainingruntime"

var _ = ginkgo.Describe("ClusterTrainingRuntime Webhook", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{}
		cfg = fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg)
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
				GenerateName: "clustertrainingruntime-webhook-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.ClusterTrainingRuntime{})).To(gomega.Succeed())
	})

	ginkgo.When("Creating ClusterTrainingRuntime", func() {
		ginkgo.DescribeTable("", func(runtime func() *trainer.ClusterTrainingRuntime) {
			gomega.Expect(k8sClient.Create(ctx, runtime())).Should(gomega.Succeed())
		},
			ginkgo.Entry("Should succeed to create ClusterTrainingRuntime",
				func() *trainer.ClusterTrainingRuntime {
					baseRuntime := testingutil.MakeClusterTrainingRuntimeWrapper(clTrainingRuntimeName)
					return baseRuntime.
						RuntimeSpec(
							testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
								Obj()).
						Obj()
				}),
		)
	})
})
