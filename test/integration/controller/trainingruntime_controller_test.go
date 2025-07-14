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

package controller

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

var _ = ginkgo.Describe("TrainingRuntime controller", ginkgo.Ordered, func() {
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
				GenerateName: "trainingruntime-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).Should(gomega.Succeed())
	})

	ginkgo.When("Reconcile TrainingRuntime", func() {
		var (
			trainJob           *trainer.TrainJob
			trainingRuntime    *trainer.TrainingRuntime
			trainingRuntimeKey client.ObjectKey
		)
		ginkgo.BeforeEach(func() {
			trainingRuntime = utiltesting.MakeTrainingRuntimeWrapper(ns.Name, "alpha").
				Obj()
			trainingRuntimeKey = client.ObjectKeyFromObject(trainingRuntime)
			trainJob = utiltesting.MakeTrainJobWrapper(ns.Name, "alpha").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), trainingRuntime.Name).
				Obj()
		})
		ginkgo.AfterEach(func() {
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainingRuntime{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		})

		ginkgo.It("TrainingRuntime obtains finalizer when TrainJob with TrainingRuntime is created", func() {
			ginkgo.By("Creating a TrainingRuntime")
			gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())

			ginkgo.By("Checking if the TrainingRuntime does not keep obtaining finalizers")
			gomega.Consistently(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainingRuntimeKey, trainingRuntime)).Should(gomega.Succeed())
				g.Expect(trainingRuntime.ObjectMeta.Finalizers).Should(gomega.HaveLen(0))
			}, util.ConsistentDuration, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Creating a TrainJob")
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			ginkgo.By("Checking if the TrainingRuntime obtains finalizers")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainingRuntimeKey, trainingRuntime)).Should(gomega.Succeed())
				g.Expect(trainingRuntime.ObjectMeta.Finalizers).Should(gomega.BeComparableTo(
					[]string{constants.ResourceInUseFinalizer},
				))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})

		ginkgo.It("TrainingRuntime can be deleted once referenced TrainJob is deleted", func() {
			ginkgo.By("Creating a TrainingRuntime and TrainJob")
			gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			ginkgo.By("Checking if the TrainingRuntime obtains finalizers")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainingRuntimeKey, trainingRuntime)).Should(gomega.Succeed())
				g.Expect(trainingRuntime.ObjectMeta.Finalizers).Should(gomega.BeComparableTo(
					[]string{constants.ResourceInUseFinalizer},
				))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Delete a TrainJob")
			gomega.Expect(k8sClient.Delete(ctx, trainJob)).Should(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), trainJob)).Should(utiltesting.BeNotFoundError())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Checking if the TrainingRuntime keep having finalizers")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainingRuntimeKey, trainingRuntime)).Should(gomega.Succeed())
				g.Expect(trainingRuntime.ObjectMeta.Finalizers).Should(gomega.BeComparableTo(
					[]string{constants.ResourceInUseFinalizer},
				))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Checking if the TrainingRuntime can be deleted")
			gomega.Expect(k8sClient.Delete(ctx, trainingRuntime)).Should(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainingRuntimeKey, trainingRuntime)).Should(utiltesting.BeNotFoundError())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})
})
