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

package controllerv2

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	kubeflowv2 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v2alpha1"
	"github.com/kubeflow/training-operator/pkg/constants"
	jobsetplugin "github.com/kubeflow/training-operator/pkg/runtime.v2/framework/plugins/jobset"
	testingutil "github.com/kubeflow/training-operator/pkg/util.v2/testing"
	"github.com/kubeflow/training-operator/test/integration/framework"
	"github.com/kubeflow/training-operator/test/util"
)

var _ = ginkgo.Describe("TrainJob controller", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	resRequests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}

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
				GenerateName: "trainjob-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.When("Reconciling TrainJob", func() {
		var (
			trainJob        *kubeflowv2.TrainJob
			trainJobKey     client.ObjectKey
			trainingRuntime *kubeflowv2.TrainingRuntime
		)

		ginkgo.AfterEach(func() {
			gomega.Expect(k8sClient.DeleteAllOf(ctx, &kubeflowv2.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		})

		ginkgo.BeforeEach(func() {
			trainJob = testingutil.MakeTrainJobWrapper(ns.Name, "alpha").
				Suspend(true).
				RuntimeRef(kubeflowv2.GroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "alpha").
				SpecLabel("testingKey", "testingVal").
				SpecAnnotation("testingKey", "testingVal").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainJob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj()).
				DatasetConfig(
					testingutil.MakeTrainJobDatasetConfigWrapper().
						StorageUri("hf://trainjob-dataset").
						Obj(),
				).
				ModelConfig(
					testingutil.MakeTrainJobModelConfigWrapper().
						StorageUri("hf://trainjob-model").
						Obj(),
				).
				Obj()
			trainJobKey = client.ObjectKeyFromObject(trainJob)

			trainingRuntime = testingutil.MakeTrainingRuntimeWrapper(ns.Name, "alpha").
				RuntimeSpec(
					testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "alpha").Spec).
						NumNodes(100).
						ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						PodGroupPolicyCoscheduling(&kubeflowv2.CoschedulingPodGroupPolicySource{ScheduleTimeoutSeconds: ptr.To[int32](100)}).
						Obj()).
				Obj()
		})

		// Integration tests for the PlainML Runtime.
		ginkgo.It("Should succeed to create TrainJob with TrainingRuntime", func() {
			ginkgo.By("Creating TrainingRuntime and TrainJob")
			gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			ginkgo.By("Checking if the appropriate JobSet and PodGroup are created")
			gomega.Eventually(func(g gomega.Gomega) {
				jobSet := &jobsetv1alpha2.JobSet{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, jobSet)).Should(gomega.Succeed())
				g.Expect(jobSet).Should(gomega.BeComparableTo(
					testingutil.MakeJobSetWrapper(ns.Name, trainJobKey.Name).
						Replicas(1).
						NumNodes(100).
						ContainerTrainer("test:trainJob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						InitContainerDatasetInitializerEnv([]corev1.EnvVar{{Name: jobsetplugin.InitializerEnvStorageUri, Value: "hf://trainjob-dataset"}}).
						InitContainerModelInitializerEnv([]corev1.EnvVar{{Name: jobsetplugin.InitializerEnvStorageUri, Value: "hf://trainjob-model"}}).
						Suspend(true).
						Label("testingKey", "testingVal").
						Annotation("testingKey", "testingVal").
						PodLabel(schedulerpluginsv1alpha1.PodGroupLabel, trainJobKey.Name).
						ControllerReference(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainJobKind), trainJobKey.Name, string(trainJob.UID)).
						Obj(),
					util.IgnoreObjectMetadata))
				pg := &schedulerpluginsv1alpha1.PodGroup{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, pg)).Should(gomega.Succeed())
				g.Expect(pg).Should(gomega.BeComparableTo(
					testingutil.MakeSchedulerPluginsPodGroup(ns.Name, trainJobKey.Name).
						MinMember(101). // 101 replicas = 100 Trainer nodes + 1 Initializer.
						MinResources(corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("101"), // 1 CPU and 4Gi per replica.
							corev1.ResourceMemory: resource.MustParse("404Gi"),
						}).
						SchedulingTimeout(100).
						ControllerReference(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainJobKind), trainJobKey.Name, string(trainJob.UID)).
						Obj(),
					util.IgnoreObjectMetadata))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})

		ginkgo.It("Should succeeded to update JobSet only when TrainJob is suspended", func() {
			ginkgo.By("Creating TrainingRuntime and suspended TrainJob")
			gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			ginkgo.By("Checking if JobSet and PodGroup are created")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainJobKey, &jobsetv1alpha2.JobSet{})).Should(gomega.Succeed())
				g.Expect(k8sClient.Get(ctx, trainJobKey, &schedulerpluginsv1alpha1.PodGroup{})).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Updating suspended TrainJob Trainer image")
			updatedImageName := "updated-trainer-image"
			originImageName := *trainJob.Spec.Trainer.Image
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainJobKey, trainJob)).Should(gomega.Succeed())
				trainJob.Spec.Trainer.Image = &updatedImageName
				g.Expect(k8sClient.Update(ctx, trainJob)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Trainer image should be updated")
			gomega.Eventually(func(g gomega.Gomega) {
				jobSet := &jobsetv1alpha2.JobSet{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, jobSet)).Should(gomega.Succeed())
				g.Expect(jobSet).Should(gomega.BeComparableTo(
					testingutil.MakeJobSetWrapper(ns.Name, trainJobKey.Name).
						Replicas(1).
						NumNodes(100).
						ContainerTrainer(updatedImageName, []string{"trainjob"}, []string{"trainjob"}, resRequests).
						InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						InitContainerDatasetInitializerEnv([]corev1.EnvVar{{Name: jobsetplugin.InitializerEnvStorageUri, Value: "hf://trainjob-dataset"}}).
						InitContainerModelInitializerEnv([]corev1.EnvVar{{Name: jobsetplugin.InitializerEnvStorageUri, Value: "hf://trainjob-model"}}).
						Suspend(true).
						Label("testingKey", "testingVal").
						Annotation("testingKey", "testingVal").
						PodLabel(schedulerpluginsv1alpha1.PodGroupLabel, trainJobKey.Name).
						ControllerReference(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainJobKind), trainJobKey.Name, string(trainJob.UID)).
						Obj(),
					util.IgnoreObjectMetadata))
				pg := &schedulerpluginsv1alpha1.PodGroup{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, pg)).Should(gomega.Succeed())
				g.Expect(pg).Should(gomega.BeComparableTo(
					testingutil.MakeSchedulerPluginsPodGroup(ns.Name, trainJobKey.Name).
						MinMember(101).
						MinResources(corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("101"), // 1 CPU and 4Gi per 101 replica.
							corev1.ResourceMemory: resource.MustParse("404Gi"),
						}).
						SchedulingTimeout(100).
						ControllerReference(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainJobKind), trainJobKey.Name, string(trainJob.UID)).
						Obj(),
					util.IgnoreObjectMetadata))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Unsuspending TrainJob")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainJobKey, trainJob)).Should(gomega.Succeed())
				trainJob.Spec.Suspend = ptr.To(false)
				g.Expect(k8sClient.Update(ctx, trainJob)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				jobSet := &jobsetv1alpha2.JobSet{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, jobSet)).Should(gomega.Succeed())
				g.Expect(ptr.Deref(jobSet.Spec.Suspend, false)).Should(gomega.BeFalse())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Trying to restore Trainer image")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainJobKey, trainJob)).Should(gomega.Succeed())
				trainJob.Spec.Trainer.Image = &originImageName
				g.Expect(k8sClient.Update(ctx, trainJob)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Checking if JobSet keep having updated Trainer image")
			gomega.Consistently(func(g gomega.Gomega) {
				jobSet := &jobsetv1alpha2.JobSet{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, jobSet)).Should(gomega.Succeed())
				for _, rJob := range jobSet.Spec.ReplicatedJobs {
					if rJob.Name == constants.JobTrainerNode {
						g.Expect(rJob.Template.Spec.Template.Spec.Containers[0].Image).Should(gomega.Equal(updatedImageName))
					}
				}
			}, util.ConsistentDuration, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Trying to re-suspend TrainJob and restore Trainer image")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainJobKey, trainJob))
				trainJob.Spec.Suspend = ptr.To(true)
				trainJob.Spec.Trainer.Image = &originImageName
				g.Expect(k8sClient.Update(ctx, trainJob)).Should(gomega.Succeed())
			}, util.Timeout, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Checking if JobSet image is restored")
			gomega.Eventually(func(g gomega.Gomega) {
				jobSet := &jobsetv1alpha2.JobSet{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, jobSet)).Should(gomega.Succeed())
				g.Expect(jobSet.Spec.Suspend).ShouldNot(gomega.BeNil())
				g.Expect(*jobSet.Spec.Suspend).Should(gomega.BeTrue())
				for _, rJob := range jobSet.Spec.ReplicatedJobs {
					if rJob.Name == constants.JobTrainerNode {
						g.Expect(rJob.Template.Spec.Template.Spec.Containers[0].Image).Should(gomega.Equal(originImageName))
					}
				}
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})

		// Integration tests for the Torch Runtime.
		ginkgo.It("Should succeed to create TrainJob with Torch TrainingRuntime", func() {
			ginkgo.By("Creating Torch TrainingRuntime and TrainJob")
			trainJob = testingutil.MakeTrainJobWrapper(ns.Name, "alpha").
				RuntimeRef(kubeflowv2.GroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "alpha").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainJob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						ContainerEnv([]corev1.EnvVar{{Name: "TRAIN_JOB", Value: "value"}}).
						Obj()).
				Obj()
			trainJobKey = client.ObjectKeyFromObject(trainJob)

			trainingRuntime = testingutil.MakeTrainingRuntimeWrapper(ns.Name, "alpha").
				RuntimeSpec(
					testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "alpha").Spec).
						TorchPolicy(100, "auto").
						ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Obj()).
				Obj()
			gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			ginkgo.By("Checking if the appropriate JobSet is created")
			gomega.Eventually(func(g gomega.Gomega) {
				jobSet := &jobsetv1alpha2.JobSet{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, jobSet)).Should(gomega.Succeed())
				g.Expect(jobSet).Should(gomega.BeComparableTo(
					testingutil.MakeJobSetWrapper(ns.Name, trainJobKey.Name).
						Replicas(1).
						NumNodes(100).
						Suspend(false).
						ContainerTrainer("test:trainJob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort, Protocol: "TCP"}}).
						ContainerTrainerEnv(
							[]corev1.EnvVar{
								{
									Name:  "TRAIN_JOB",
									Value: "value",
								},
								{
									Name:  constants.TorchEnvNumNodes,
									Value: "100",
								},
								{
									Name:  constants.TorchEnvNumProcPerNode,
									Value: "auto",
								},
								{
									Name: constants.TorchEnvNodeRank,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: constants.JobCompletionIndexFieldPath,
										},
									},
								},
								{
									Name:  constants.TorchEnvMasterAddr,
									Value: fmt.Sprintf("alpha-%s-0-0.alpha", constants.JobTrainerNode),
								},
								{
									Name:  constants.TorchEnvMasterPort,
									Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
								},
							},
						).
						ControllerReference(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainJobKind), trainJobKey.Name, string(trainJob.UID)).
						Obj(),
					util.IgnoreObjectMetadata))

			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
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
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &kubeflowv2.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
	})

	ginkgo.When("Creating TrainJob", func() {
		ginkgo.DescribeTable("Validate TrainJob on creation", func(trainJob func() *kubeflowv2.TrainJob, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, trainJob())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed to create TrainJob with 'managedBy: kubeflow.org/trainjob-conteroller'",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "managed-by-trainjob-controller").
						ManagedBy("kubeflow.org/trainjob-controller").
						RuntimeRef(kubeflowv2.GroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "testing").
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should succeed to create TrainJob with 'managedBy: kueue.x-k8s.io/multukueue'",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "managed-by-trainjob-controller").
						ManagedBy("kueue.x-k8s.io/multikueue").
						RuntimeRef(kubeflowv2.GroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "testing").
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail to create TrainJob with invalid managedBy",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "invalid-managed-by").
						ManagedBy("invalid").
						RuntimeRef(kubeflowv2.GroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "testing").
						Obj()
				},
				testingutil.BeInvalidError()),
		)
		ginkgo.DescribeTable("Defaulting TrainJob on creation", func(trainJob func() *kubeflowv2.TrainJob, wantTrainJob func() *kubeflowv2.TrainJob) {
			created := trainJob()
			gomega.Expect(k8sClient.Create(ctx, created)).Should(gomega.Succeed())
			gomega.Expect(created).Should(gomega.BeComparableTo(wantTrainJob(), util.IgnoreObjectMetadata))
		},
			ginkgo.Entry("Should succeed to default suspend=false",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-suspend").
						ManagedBy("kueue.x-k8s.io/multikueue").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.ClusterTrainingRuntimeKind), "testing").
						Obj()
				},
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-suspend").
						ManagedBy("kueue.x-k8s.io/multikueue").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.ClusterTrainingRuntimeKind), "testing").
						Suspend(false).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default managedBy=kubeflow.org/trainjob-controller",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-managed-by").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "testing").
						Suspend(true).
						Obj()
				},
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "null-managed-by").
						ManagedBy("kubeflow.org/trainjob-controller").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "testing").
						Suspend(true).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default runtimeRef.apiGroup",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-api-group").
						RuntimeRef(schema.GroupVersionKind{Group: "", Version: "", Kind: kubeflowv2.TrainingRuntimeKind}, "testing").
						Obj()
				},
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-api-group").
						ManagedBy("kubeflow.org/trainjob-controller").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "testing").
						Suspend(false).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default runtimeRef.kind",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-kind").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(""), "testing").
						Obj()
				},
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "empty-kind").
						ManagedBy("kubeflow.org/trainjob-controller").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.ClusterTrainingRuntimeKind), "testing").
						Suspend(false).
						Obj()
				}),
		)
	})

	ginkgo.When("Updating TrainJob", func() {
		ginkgo.DescribeTable("Validate TrainJob on update", func(old func() *kubeflowv2.TrainJob, new func(*kubeflowv2.TrainJob) *kubeflowv2.TrainJob, errorMatcher gomega.OmegaMatcher) {
			oldTrainJob := old()
			gomega.Expect(k8sClient.Create(ctx, oldTrainJob)).Should(gomega.Succeed())
			gomega.Expect(k8sClient.Update(ctx, new(oldTrainJob))).Should(errorMatcher)
		},
			ginkgo.Entry("Should fail to update TrainJob managedBy",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "valid-managed-by").
						ManagedBy("kubeflow.org/trainjob-controller").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainingRuntimeKind), "testing").
						Obj()
				},
				func(job *kubeflowv2.TrainJob) *kubeflowv2.TrainJob {
					job.Spec.ManagedBy = ptr.To("kueue.x-k8s.io/multikueue")
					return job
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to update runtimeRef",
				func() *kubeflowv2.TrainJob {
					return testingutil.MakeTrainJobWrapper(ns.Name, "valid-runtimeref").
						RuntimeRef(kubeflowv2.SchemeGroupVersion.WithKind(kubeflowv2.TrainJobKind), "testing").
						Obj()
				},
				func(job *kubeflowv2.TrainJob) *kubeflowv2.TrainJob {
					job.Spec.RuntimeRef.Name = "forbidden-update"
					return job
				},
				testingutil.BeInvalidError()),
		)
	})
})
