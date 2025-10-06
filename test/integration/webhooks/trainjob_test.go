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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

var _ = ginkgo.Describe("TrainJob Webhook", ginkgo.Ordered, func() {
	var ns *corev1.Namespace
	var trainingRuntime *trainer.TrainingRuntime
	var clusterTrainingRuntime *trainer.ClusterTrainingRuntime
	runtimeName := "training-runtime"
	jobName := "train-job"

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
				GenerateName: "trainjob-webhook-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())

		baseRuntimeWrapper := testingutil.MakeTrainingRuntimeWrapper(ns.Name, runtimeName)
		baseClusterRuntimeWrapper := testingutil.MakeClusterTrainingRuntimeWrapper(runtimeName)
		trainingRuntime = baseRuntimeWrapper.RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(
				testingutil.MakeTrainingRuntimeWrapper(ns.Name, runtimeName).Spec).Obj()).Obj()
		clusterTrainingRuntime = baseClusterRuntimeWrapper.RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(
				testingutil.MakeClusterTrainingRuntimeWrapper(runtimeName).Spec).Obj()).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())
		gomega.Expect(k8sClient.Create(ctx, clusterTrainingRuntime)).To(gomega.Succeed())
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainingRuntime), trainingRuntime)).Should(gomega.Succeed())
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterTrainingRuntime), clusterTrainingRuntime)).Should(gomega.Succeed())
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainingRuntime{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.ClusterTrainingRuntime{})).To(gomega.Succeed())
	})

	ginkgo.When("Creating TrainJob", func() {
		ginkgo.DescribeTable("Validate TrainJob on creation", func(trainJob func() *trainer.TrainJob, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, trainJob())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed in creating trainJob with namespace scoped trainingRuntime",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), runtimeName).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail in creating trainJob referencing trainingRuntime not present in the namespace",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "invalid").
						Obj()
				},
				testingutil.BeForbiddenError()),
			ginkgo.Entry("Should succeed in creating trainJob with namespace scoped trainingRuntime",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail in creating trainJob with pre-trained model config when referencing a trainingRuntime without an initializer",
				func() *trainer.TrainJob {
					newContainers := []corev1.Container{}
					// TODO (andreyvelich): Refactor this test to check ancestor label.
					job := &trainingRuntime.Spec.Template.Spec.ReplicatedJobs[1]
					for _, container := range job.Template.Spec.Template.Spec.Containers {
						if container.Name != constants.ModelInitializer {
							newContainers = append(newContainers, container)
						}
					}
					job.Template.Spec.Template.Spec.Containers = newContainers
					gomega.Expect(k8sClient.Update(ctx, trainingRuntime)).To(gomega.Succeed())
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), runtimeName).
						Initializer(
							testingutil.MakeTrainJobInitializerWrapper().
								ModelInitializer(
									testingutil.MakeTrainJobModelInitializerWrapper().
										StorageUri("hf://trainjob-model").
										Obj(),
								).
								Obj(),
						).
						Obj()
				},
				testingutil.BeForbiddenError()),
			ginkgo.Entry("Should fail in creating trainJob with dataset initializer when referencing a trainingRuntime without an initializer",
				func() *trainer.TrainJob {
					newContainers := []corev1.Container{}
					job := &trainingRuntime.Spec.Template.Spec.ReplicatedJobs[0]
					for _, container := range job.Template.Spec.Template.Spec.Containers {
						if container.Name != constants.DatasetInitializer {
							newContainers = append(newContainers, container)
						}
					}
					job.Template.Spec.Template.Spec.Containers = newContainers
					gomega.Expect(k8sClient.Update(ctx, trainingRuntime)).To(gomega.Succeed())
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), runtimeName).
						Initializer(
							testingutil.MakeTrainJobInitializerWrapper().
								DatasetInitializer(
									testingutil.MakeTrainJobDatasetInitializerWrapper().
										StorageUri("hf://trainjob-model").
										Obj(),
								).
								Obj(),
						).
						Obj()
				},
				testingutil.BeForbiddenError()),
			ginkgo.Entry("Should succeed in creating trainJob with dataset initializer when referencing a ClusterTrainingRuntime with an initializer",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Initializer(
							testingutil.MakeTrainJobInitializerWrapper().
								DatasetInitializer(
									testingutil.MakeTrainJobDatasetInitializerWrapper().
										StorageUri("hf://trainjob-model").
										Obj(),
								).
								Obj(),
						).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail in creating trainJob with invalid trainer config for mpi runtime",
				func() *trainer.TrainJob {
					trainingRuntime.Spec.MLPolicy = &trainer.MLPolicy{MLPolicySource: trainer.MLPolicySource{MPI: &trainer.MPIMLPolicySource{}}}
					gomega.Expect(k8sClient.Update(ctx, trainingRuntime)).To(gomega.Succeed())
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), runtimeName).
						Trainer(&trainer.Trainer{NumProcPerNode: ptr.To(intstr.FromString("invalid"))}).
						Obj()
				},
				testingutil.BeForbiddenError()),
			ginkgo.Entry("Should fail in creating trainJob with invalid trainer config for torch runtime",
				func() *trainer.TrainJob {
					trainingRuntime.Spec.MLPolicy = &trainer.MLPolicy{MLPolicySource: trainer.MLPolicySource{Torch: &trainer.TorchMLPolicySource{}}}
					gomega.Expect(k8sClient.Update(ctx, trainingRuntime)).To(gomega.Succeed())
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), runtimeName).
						Trainer(&trainer.Trainer{NumProcPerNode: ptr.To(intstr.FromString("invalid"))}).
						Obj()
				},
				testingutil.BeForbiddenError()),
			ginkgo.Entry("Should succeed in creating trainJob with valid trainer config for torch runtime",
				func() *trainer.TrainJob {
					trainingRuntime.Spec.MLPolicy = &trainer.MLPolicy{MLPolicySource: trainer.MLPolicySource{Torch: &trainer.TorchMLPolicySource{}}}
					gomega.Expect(k8sClient.Update(ctx, trainingRuntime)).To(gomega.Succeed())
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), runtimeName).
						Trainer(&trainer.Trainer{NumProcPerNode: ptr.To(intstr.FromString("auto"))}).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail in creating trainJob with trainer config having envs with reserved env names",
				func() *trainer.TrainJob {
					trainingRuntime.Spec.MLPolicy = &trainer.MLPolicy{MLPolicySource: trainer.MLPolicySource{Torch: &trainer.TorchMLPolicySource{}}}
					gomega.Expect(k8sClient.Update(ctx, trainingRuntime)).To(gomega.Succeed())
					return testingutil.MakeTrainJobWrapper(ns.Name, jobName).
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), runtimeName).
						Trainer(&trainer.Trainer{NumProcPerNode: ptr.To(intstr.FromString("auto")), Env: []corev1.EnvVar{{Name: "PET_NODE_RANK", Value: "1"}}}).
						Obj()
				},
				testingutil.BeForbiddenError()),
		)
		ginkgo.DescribeTable("RFC1035-compliant TrainJob name validation", func(trainJob func() *trainer.TrainJob, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, trainJob())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed to create TrainJob with valid RFC1035-compliant name",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "valid-job-name").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed to create TrainJob with name exactly 63 characters",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name,
						"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk"). // 63 chars
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail to create TrainJob with uppercase letters in name",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "Invalid-job-name").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Obj()
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create TrainJob starting with a digit",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "1jobname").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Obj()
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create TrainJob ending with a hyphen",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "jobname-").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Obj()
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create TrainJob with name exceeding 63 characters",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name,
						"this-name-is-way-too-long-for-a-rfc1035-label-and-should-fail-validation").
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
						Obj()
				},
				testingutil.BeInvalidError()),
		)
	})
})

var _ = ginkgo.Describe("TrainJob marker validations and defaulting", ginkgo.Ordered, func() {
	var ns *corev1.Namespace
	var (
		trainingRuntime        *trainer.TrainingRuntime
		clusterTrainingRuntime *trainer.ClusterTrainingRuntime
	)

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
				GenerateName: "trainjob-marker-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
		baseRuntimeWrapper := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "testing")
		baseClusterRuntimeWrapper := testingutil.MakeClusterTrainingRuntimeWrapper("testing")
		trainingRuntime = baseRuntimeWrapper.RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(
				testingutil.MakeTrainingRuntimeWrapper(ns.Name, "testing").Spec).Obj()).Obj()
		clusterTrainingRuntime = baseClusterRuntimeWrapper.RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(
				testingutil.MakeClusterTrainingRuntimeWrapper("testing").Spec).Obj()).Obj()
		gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).To(gomega.Succeed())
		gomega.Expect(k8sClient.Create(ctx, clusterTrainingRuntime)).To(gomega.Succeed())
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainingRuntime), trainingRuntime)).Should(gomega.Succeed())
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterTrainingRuntime), clusterTrainingRuntime)).Should(gomega.Succeed())
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
	})
	ginkgo.AfterAll(func() {
		fwk.Teardown()
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainingRuntime{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.ClusterTrainingRuntime{})).To(gomega.Succeed())
	})

	ginkgo.When("Creating TrainJob", func() {
		ginkgo.DescribeTable("Validate TrainJob on creation", func(trainJob func() *trainer.TrainJob, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, trainJob())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed to create TrainJob with 'managedBy: trainer.kubeflow.org/trainjob-controller'",
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
			ginkgo.Entry("Should succeed to create trainJob with podSpecOverrides containing duplicate targetJob",
				func() *trainer.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "duplicated-podspecoverrides-target-jobs").
						RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
						PodSpecOverrides([]trainer.PodSpecOverride{
							{
								TargetJobs:         []trainer.PodSpecOverrideTargetJob{{Name: "node"}},
								ServiceAccountName: ptr.To("custom-sa"),
							},
							{
								TargetJobs:         []trainer.PodSpecOverrideTargetJob{{Name: "node"}},
								ServiceAccountName: ptr.To("custom-sa-two"),
							},
						}).
						Obj()
				},
				gomega.Succeed()),
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
						RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "testing").
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
