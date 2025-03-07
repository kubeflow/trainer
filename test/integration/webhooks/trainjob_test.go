/*
Copyright 2024 The Kubeflow Authors.

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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
	"github.com/kubeflow/trainer/test/integration/framework"
	"github.com/kubeflow/trainer/test/util"
)

var _ = ginkgo.Describe("TrainJob Webhook", ginkgo.Ordered, func() {
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
				GenerateName: "trainjob-webhook-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})
})

var _ = ginkgo.Describe("TrainJob marker validations and defaulting", ginkgo.Ordered, func() {
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
				GenerateName: "trainjob-marker-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})
	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
	})

	ginkgo.When("Creating TrainJob", func() {
		ginkgo.DescribeTable("Validate TrainJob on creation", func(trainJob func() *trainer.TrainJob, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, trainJob())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed to create TrainJob with 'managedBy: trainer.kubeflow.org/trainjob-conteroller'",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "managed-by-trainjob-controller").
						ManagedBy("trainer.kubeflow.org/trainjob-controller").
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed to create TrainJob with 'managedBy: kueue.x-k8s.io/multukueue'",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "managed-by-trainjob-controller").
						ManagedBy("kueue.x-k8s.io/multikueue").
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail to create TrainJob with invalid managedBy",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "invalid-managed-by").
						ManagedBy("invalid").
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						Obj()
				},
				testingutil.BeInvalidError()),
		)
		ginkgo.DescribeTable("Defaulting TrainJob on creation", func(trainJob func() *trainer.TrainJob, wantTrainJob func() *trainer.TrainJob) {
			created := trainJob()
			gomega.Expect(k8sClient.Create(ctx, created)).Should(gomega.Succeed())
			gomega.Expect(created).Should(gomega.BeComparableTo(wantTrainJob(), util.IgnoreObjectMetadata))
		},
			ginkgo.Entry("Should succeed to default suspend=false",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-suspend").
						ManagedBy("kueue.x-k8s.io/multikueue").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "testing").
						Obj()
				},
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-suspend").
						ManagedBy("kueue.x-k8s.io/multikueue").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "testing").
						Suspend(false).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default managedBy=trainer.kubeflow.org/trainjob-controller",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-managed-by").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						Suspend(true).
						Obj()
				},
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-managed-by").
						ManagedBy("trainer.kubeflow.org/trainjob-controller").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						Suspend(true).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default runtimeRef.apiGroup",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-api-group").
						RuntimeRef(schema.GroupVersionKind{Group: "", Version: "", Kind: trainer.TrainingRuntimeKind}, "testing").
						Obj()
				},
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-api-group").
						ManagedBy("trainer.kubeflow.org/trainjob-controller").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						Suspend(false).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default runtimeRef.kind",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-kind").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(""), "testing").
						Obj()
				},
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-kind").
						ManagedBy("trainer.kubeflow.org/trainjob-controller").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "testing").
						Suspend(false).
						Obj()
				}),
		)
	})

	ginkgo.When("Updating TrainJob", func() {
		ginkgo.DescribeTable("Validate TrainJob on update", func(old func() *trainer.TrainJob, new func(*trainer.TrainJob) *trainer.TrainJob, errorMatcher gomega.OmegaMatcher) {
			oldTrainJob := old()
			gomega.Expect(k8sClient.Create(ctx, oldTrainJob)).Should(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(oldTrainJob), oldTrainJob)).Should(gomega.Succeed())
				g.Expect(k8sClient.Update(ctx, new(oldTrainJob))).Should(errorMatcher)
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		},
			ginkgo.Entry("Should fail to update TrainJob managedBy",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "valid-managed-by").
						ManagedBy("trainer.kubeflow.org/trainjob-controller").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						Obj()
				},
				func(job *trainer.TrainJob) *trainer.TrainJob {
					job.Spec.ManagedBy = ptr.To("kueue.x-k8s.io/multikueue")
					return job
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to update runtimeRef",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "valid-runtimeref").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "testing").
						Obj()
				},
				func(job *trainer.TrainJob) *trainer.TrainJob {
					job.Spec.RuntimeRef.Name = "forbidden-update"
					return job
				},
				testingutil.BeInvalidError()),
		)
	})
})
