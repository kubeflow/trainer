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

package networkpolicy

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/features"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

var _ = ginkgo.Describe("TrainJob NetworkPolicy", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	resRequests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}

	ginkgo.BeforeAll(func() {
		// The plugin registry is built once while the manager starts, so the feature gate
		// must be enabled before RunManager; toggling it later would come too late for the
		// plugin to be registered.
		gomega.Expect(utilfeature.DefaultMutableFeatureGate.SetFromMap(map[string]bool{
			string(features.TrainJobNetworkPolicy): true,
		})).To(gomega.Succeed())

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
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "trainjob-netpol-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.When("Reconciling a TrainJob with the NetworkPolicy feature enabled", func() {
		var (
			trainJob        *trainer.TrainJob
			trainJobKey     client.ObjectKey
			trainingRuntime *trainer.TrainingRuntime
		)

		ginkgo.BeforeEach(func() {
			trainJob = testingutil.MakeTrainJobWrapper(ns.Name, "alpha").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "alpha").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj()).
				Obj()
			trainJobKey = client.ObjectKeyFromObject(trainJob)

			trainingRuntime = testingutil.MakeTrainingRuntimeWrapper(ns.Name, "alpha").
				RuntimeSpec(
					testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(ns.Name, "alpha").Spec).
						Container(constants.DatasetInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Container(constants.ModelInitializer, constants.ModelInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Obj()).
				Obj()

			gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())
			gomega.Expect(k8sClient.Create(ctx, trainJob)).To(gomega.Succeed())
		})

		ginkgo.AfterEach(func() {
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
		})

		ginkgo.It("Should create a NetworkPolicy that selects every Pod the TrainJob generates", func() {
			ginkgo.By("Checking that the NetworkPolicy is created for the TrainJob")
			networkPolicy := &networkingv1.NetworkPolicy{}
			networkPolicyKey := client.ObjectKey{
				Namespace: ns.Name,
				Name:      trainJobKey.Name + constants.TrainJobNetworkPolicySuffix,
			}
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, networkPolicyKey, networkPolicy)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Checking that it only admits ingress from Pods of the same TrainJob")
			trainJobPods := metav1.LabelSelector{
				MatchLabels: map[string]string{constants.LabelTrainJobName: trainJobKey.Name},
			}
			gomega.Expect(networkPolicy.Spec.PodSelector).To(gomega.BeComparableTo(trainJobPods))
			gomega.Expect(networkPolicy.Spec.PolicyTypes).To(gomega.Equal([]networkingv1.PolicyType{networkingv1.PolicyTypeIngress}))
			gomega.Expect(networkPolicy.Spec.Ingress).To(gomega.HaveLen(1))
			gomega.Expect(networkPolicy.Spec.Ingress[0].From).To(gomega.HaveLen(1))
			gomega.Expect(networkPolicy.Spec.Ingress[0].From[0].PodSelector).To(gomega.BeComparableTo(&trainJobPods))

			ginkgo.By("Checking that it is owned by the TrainJob so it is garbage collected with it")
			gomega.Expect(networkPolicy.OwnerReferences).To(gomega.HaveLen(1))
			gomega.Expect(networkPolicy.OwnerReferences[0].Kind).To(gomega.Equal(trainer.TrainJobKind))
			gomega.Expect(networkPolicy.OwnerReferences[0].Name).To(gomega.Equal(trainJobKey.Name))
			gomega.Expect(*networkPolicy.OwnerReferences[0].Controller).To(gomega.BeTrue())

			ginkgo.By("Checking that every Pod template generated by the TrainJob carries the selected label")
			jobSet := &jobsetv1alpha2.JobSet{}
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainJobKey, jobSet)).Should(gomega.Succeed())
				g.Expect(jobSet.Spec.ReplicatedJobs).ShouldNot(gomega.BeEmpty())
				for _, rJob := range jobSet.Spec.ReplicatedJobs {
					// The Pods inherit these labels, so the NetworkPolicy podSelector above
					// must match them for the policy to isolate anything at all.
					g.Expect(rJob.Template.Spec.Template.Labels).Should(
						gomega.HaveKeyWithValue(constants.LabelTrainJobName, trainJobKey.Name),
						"replicatedJob %q is not selected by the NetworkPolicy", rJob.Name,
					)
				}
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})
})
