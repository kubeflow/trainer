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

var _ = ginkgo.Describe("ClusterTrainingRuntime controller", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{}
		cfg := fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg, true)
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
				GenerateName: "clustertrainingruntime-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).Should(gomega.Succeed())
	})

	ginkgo.When("Reconcile ClusterTrainingRuntime", func() {
		var (
			trainJob             *trainer.TrainJob
			clTrainingRuntime    *trainer.ClusterTrainingRuntime
			clTrainingRuntimeKey client.ObjectKey
		)
		ginkgo.BeforeEach(func() {
			clTrainingRuntime = utiltesting.MakeClusterTrainingRuntimeWrapper("alpha").
				Obj()
			clTrainingRuntimeKey = client.ObjectKeyFromObject(clTrainingRuntime)
			trainJob = utiltesting.MakeTrainJobWrapper(ns.Name, "alpha").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), clTrainingRuntime.Name).
				Obj()
		})
		ginkgo.AfterEach(func() {
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.ClusterTrainingRuntime{})).Should(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				trainJobs := &trainer.TrainJobList{}
				g.Expect(k8sClient.List(ctx, trainJobs, client.InNamespace(ns.Name))).Should(gomega.Succeed())
				g.Expect(trainJobs.Items).Should(gomega.HaveLen(0))
				clRuntimes := &trainer.ClusterTrainingRuntimeList{}
				g.Expect(k8sClient.List(ctx, clRuntimes)).Should(gomega.Succeed())
				g.Expect(clRuntimes.Items).Should(gomega.HaveLen(0))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})

		ginkgo.It("ClusterTrainingRuntime obtains finalizer when TrainJob with ClusterTrainingRuntime is created", func() {
			ginkgo.By("Creating a ClusterTrainingRuntime")
			gomega.Expect(k8sClient.Create(ctx, clTrainingRuntime)).Should(gomega.Succeed())

			ginkgo.By("Checking if the ClusterTrainingRuntime does not keep obtaining finalizers")
			gomega.Consistently(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, clTrainingRuntimeKey, clTrainingRuntime)).Should(gomega.Succeed())
				g.Expect(clTrainingRuntime.ObjectMeta.Finalizers).Should(gomega.HaveLen(0))
			}, util.ConsistentDuration, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Creating a TrainJob")
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			ginkgo.By("Checking if the ClusterTrainingRuntime obtains finalizers")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, clTrainingRuntimeKey, clTrainingRuntime)).Should(gomega.Succeed())
				g.Expect(clTrainingRuntime.ObjectMeta.Finalizers).Should(gomega.BeComparableTo(
					[]string{constants.ResourceInUseFinalizer},
				))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})

		ginkgo.It("ClusterTrainingRuntime can be deleted once referenced TrainJob is deleted", func() {
			ginkgo.By("Creating a ClusterTrainingRuntime and TrainJob")
			gomega.Expect(k8sClient.Create(ctx, clTrainingRuntime)).Should(gomega.Succeed())
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			ginkgo.By("Checking if the ClusterTrainingRuntime obtains finalizers")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, clTrainingRuntimeKey, clTrainingRuntime)).Should(gomega.Succeed())
				g.Expect(clTrainingRuntime.ObjectMeta.Finalizers).Should(gomega.BeComparableTo(
					[]string{constants.ResourceInUseFinalizer},
				))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Delete a TrainJob")
			gomega.Expect(k8sClient.Delete(ctx, trainJob)).Should(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), trainJob)).Should(utiltesting.BeNotFoundError())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Checking if the ClusterTrainingRuntime keeps having finalizers")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, clTrainingRuntimeKey, clTrainingRuntime)).Should(gomega.Succeed())
				g.Expect(clTrainingRuntime.ObjectMeta.Finalizers).Should(gomega.BeComparableTo(
					[]string{constants.ResourceInUseFinalizer},
				))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Checking if the ClusterTrainingRuntime can be deleted")
			gomega.Expect(k8sClient.Delete(ctx, clTrainingRuntime)).Should(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, clTrainingRuntimeKey, clTrainingRuntime)).Should(utiltesting.BeNotFoundError())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})
})
