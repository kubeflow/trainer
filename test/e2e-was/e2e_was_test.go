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

package e2ewas

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/util"
)

const (
	// runtimeImage is loaded into the Kind cluster by hack/e2e-setup-cluster-was.sh, so the tests don't pull images.
	runtimeImage = "pytorch/pytorch:2.13.0-cuda13.0-cudnn9-runtime"
	// gangSlotResource is a fake extended resource advertised on the node to control the cluster capacity for
	// the TrainJob Pods.
	gangSlotResource corev1.ResourceName = "example.com/gang-slot"
)

var _ = ginkgo.Describe("TrainJob Workload-Aware Scheduling e2e", func() {
	// Each test runs in a separate namespace.
	var ns *corev1.Namespace

	// Create test namespace before each test.
	ginkgo.BeforeEach(func() {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-was-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())

		// Wait for namespace to exist before proceeding with test.
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)).Should(gomega.Succeed())
		}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
	})

	// Delete test namespace after each test.
	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.Delete(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.When("Creating TrainJob with the TrainJob-level (Level 1) gang scheduling", func() {
		ginkgo.It("should create the Workload and the PodGroup, schedule the TrainJob Pods in the PodGroup, and delete them on completion", func() {
			// The empty scheduling configuration defaults to the TrainJob-level gang.
			trainingRuntime := newTrainingRuntime(ns.Name, "e2e-was-level-1", &trainer.TrainingRuntimeScheduling{},
				newReplicatedJob(constants.Node, constants.AncestorTrainer, constants.Node, "sleep 5"),
			)
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-was-level-1").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), trainingRuntime.Name).
				Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).
				Obj()

			ginkgo.By("Create a TrainingRuntime with spec.scheduling and a TrainJob with 2 nodes", func() {
				gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			var podGroup schedulingv1beta1.PodGroup
			ginkgo.By("Verify the TrainJob-level Workload and the gang PodGroup are created", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					workloads, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(workloads).Should(gomega.HaveLen(1))
					g.Expect(workloads[0].Spec.PodGroupTemplates).Should(gomega.HaveLen(1))
					g.Expect(podGroups).Should(gomega.HaveLen(1))
					podGroup = podGroups[0]
					g.Expect(podGroup.Spec.WorkloadRef).ShouldNot(gomega.BeNil())
					g.Expect(podGroup.Spec.WorkloadRef.WorkloadName).Should(gomega.Equal(workloads[0].Name))
					g.Expect(podGroup.Spec.SchedulingPolicy.Gang).ShouldNot(gomega.BeNil())
					g.Expect(podGroup.Spec.SchedulingPolicy.Gang.MinCount).Should(gomega.Equal(int32(2)))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify the TrainJob Pods are assigned to the PodGroup and the PodGroup is scheduled", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					pods := getReplicatedJobPods(g, trainJob, constants.Node)
					g.Expect(pods).Should(gomega.HaveLen(2))
					for _, pod := range pods {
						g.Expect(pod.Spec.SchedulingGroup).ShouldNot(gomega.BeNil())
						g.Expect(pod.Spec.SchedulingGroup.PodGroupName).Should(gomega.Equal(ptr.To(podGroup.Name)))
					}
					gotPodGroup := &schedulingv1beta1.PodGroup{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&podGroup), gotPodGroup)).Should(gomega.Succeed())
					g.Expect(meta.IsStatusConditionTrue(gotPodGroup.Status.Conditions, schedulingv1beta1.PodGroupInitiallyScheduled)).Should(gomega.BeTrue())
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Wait for TrainJob to be in Succeeded status", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.Conditions).Should(gomega.BeComparableTo([]metav1.Condition{
						{
							Type:    trainer.TrainJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  jobsetconsts.AllJobsCompletedReason,
							Message: jobsetconsts.AllJobsCompletedMessage,
						},
					}, util.IgnoreConditions))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify the Workload and the PodGroup are deleted after the TrainJob completes", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					workloads, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(workloads).Should(gomega.BeEmpty())
					// The completed Pods still reference the PodGroup, so the scheduling.k8s.io/podgroup-protection
					// finalizer may keep the PodGroup until the Pods are removed.
					for _, pg := range podGroups {
						g.Expect(pg.DeletionTimestamp).ShouldNot(gomega.BeNil())
					}
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})

		ginkgo.It("should schedule all TrainJob Pods together only when the cluster has capacity for the whole gang", func() {
			nodes := &corev1.NodeList{}
			gomega.Expect(k8sClient.List(ctx, nodes)).Should(gomega.Succeed())
			gomega.Expect(nodes.Items).ShouldNot(gomega.BeEmpty())
			// Only this node advertises the gang slots, so the TrainJob Pods can't be scheduled to the other nodes.
			nodeName := nodes.Items[0].Name

			ginkgo.DeferCleanup(func() {
				setNodeExtendedResource(nodeName, gangSlotResource, nil)
			})

			ginkgo.By("Advertise capacity for 1 out of 2 TrainJob Pods on the node", func() {
				setNodeExtendedResource(nodeName, gangSlotResource, ptr.To(resource.MustParse("1")))
			})

			trainingRuntime := newTrainingRuntime(ns.Name, "e2e-was-gang", &trainer.TrainingRuntimeScheduling{
				SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
					Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
				},
			},
				newReplicatedJob(constants.Node, constants.AncestorTrainer, constants.Node, "sleep 5"),
			)
			gangSlot := corev1.ResourceList{gangSlotResource: resource.MustParse("1")}
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-was-gang").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), trainingRuntime.Name).
				Trainer(&trainer.Trainer{
					NumNodes: ptr.To(int32(2)),
					// Every node of the TrainJob requires 1 gang slot.
					ResourcesPerNode: &corev1.ResourceRequirements{Requests: gangSlot, Limits: gangSlot},
				}).
				Obj()

			ginkgo.By("Create a TrainingRuntime with the gang scheduling policy and a TrainJob with 2 nodes", func() {
				gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			ginkgo.By("Wait for the TrainJob Pods to be created in the gang PodGroup", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					_, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(podGroups).Should(gomega.HaveLen(1))
					pods := getReplicatedJobPods(g, trainJob, constants.Node)
					g.Expect(pods).Should(gomega.HaveLen(2))
					for _, pod := range pods {
						g.Expect(pod.Spec.SchedulingGroup).ShouldNot(gomega.BeNil())
						g.Expect(pod.Spec.SchedulingGroup.PodGroupName).Should(gomega.Equal(ptr.To(podGroups[0].Name)))
					}
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Ensure none of the TrainJob Pods are scheduled while the gang doesn't fit", func() {
				gomega.Consistently(func(g gomega.Gomega) {
					pods := getReplicatedJobPods(g, trainJob, constants.Node)
					g.Expect(pods).Should(gomega.HaveLen(2))
					for _, pod := range pods {
						g.Expect(pod.Spec.NodeName).Should(gomega.BeEmpty())
					}
				}, 15*time.Second, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Advertise capacity for all TrainJob Pods on the node", func() {
				setNodeExtendedResource(nodeName, gangSlotResource, ptr.To(resource.MustParse("2")))
			})

			ginkgo.By("Wait for TrainJob to be in Succeeded status", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(meta.IsStatusConditionTrue(gotTrainJob.Status.Conditions, trainer.TrainJobComplete)).Should(gomega.BeTrue())
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})

		ginkgo.It("should delete the Workload and the PodGroup when the TrainJob is suspended, and recreate them on resume", func() {
			trainingRuntime := newTrainingRuntime(ns.Name, "e2e-was-suspend", &trainer.TrainingRuntimeScheduling{},
				newReplicatedJob(constants.Node, constants.AncestorTrainer, constants.Node, "sleep 600"),
			)
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-was-suspend").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), trainingRuntime.Name).
				Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).
				Obj()

			ginkgo.By("Create a TrainingRuntime with spec.scheduling and a long-running TrainJob", func() {
				gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			var podGroup schedulingv1beta1.PodGroup
			ginkgo.By("Wait for the TrainJob Pods to be scheduled in the PodGroup", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					_, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(podGroups).Should(gomega.HaveLen(1))
					podGroup = podGroups[0]
					pods := getReplicatedJobPods(g, trainJob, constants.Node)
					g.Expect(pods).Should(gomega.HaveLen(2))
					for _, pod := range pods {
						g.Expect(pod.Spec.NodeName).ShouldNot(gomega.BeEmpty())
					}
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Suspend the TrainJob", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					gotTrainJob.Spec.Suspend = ptr.To(true)
					g.Expect(k8sClient.Update(ctx, gotTrainJob)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify the TrainJob Pods, the Workload, and the PodGroup are deleted", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					g.Expect(getReplicatedJobPods(g, trainJob, constants.Node)).Should(gomega.BeEmpty())
					workloads, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(workloads).Should(gomega.BeEmpty())
					g.Expect(podGroups).Should(gomega.BeEmpty())
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Resume the TrainJob", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					gotTrainJob.Spec.Suspend = ptr.To(false)
					g.Expect(k8sClient.Update(ctx, gotTrainJob)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify the Workload and the PodGroup are recreated and the TrainJob Pods are scheduled in the new PodGroup", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					workloads, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(workloads).Should(gomega.HaveLen(1))
					g.Expect(podGroups).Should(gomega.HaveLen(1))
					g.Expect(podGroups[0].UID).ShouldNot(gomega.Equal(podGroup.UID))
					pods := getReplicatedJobPods(g, trainJob, constants.Node)
					g.Expect(pods).Should(gomega.HaveLen(2))
					for _, pod := range pods {
						g.Expect(pod.Spec.SchedulingGroup).ShouldNot(gomega.BeNil())
						g.Expect(pod.Spec.SchedulingGroup.PodGroupName).Should(gomega.Equal(ptr.To(podGroups[0].Name)))
						g.Expect(pod.Spec.NodeName).ShouldNot(gomega.BeEmpty())
					}
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating TrainJob with the ReplicatedJob-level (Level 2) scheduling", func() {
		ginkgo.It("should schedule the initializer with the basic policy and the trainer nodes with the gang policy", func() {
			initializer := newReplicatedJob(constants.DatasetInitializer, constants.DatasetInitializer, constants.DatasetInitializer, "sleep 1")
			node := newReplicatedJob(constants.Node, constants.AncestorTrainer, constants.Node, "sleep 5")
			node.DependsOn = []jobsetv1alpha2.DependsOn{{Name: constants.DatasetInitializer, Status: jobsetv1alpha2.DependencyComplete}}
			trainingRuntime := newTrainingRuntime(ns.Name, "e2e-was-level-2", &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{
						TargetReplicatedJobs: []string{constants.DatasetInitializer},
						SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
							Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
						},
					},
					{
						TargetReplicatedJobs: []string{constants.Node},
						SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
							Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
						},
					},
				},
			}, initializer, node)
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-was-level-2").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), trainingRuntime.Name).
				Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).
				Obj()

			ginkgo.By("Create a TrainingRuntime with the ReplicatedJob-level scheduling and a TrainJob with 2 nodes", func() {
				gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			podGroupsByName := make(map[string]schedulingv1beta1.PodGroup)
			ginkgo.By("Verify the Workload has a basic PodGroup for the initializer and a gang PodGroup for the nodes", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					workloads, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(workloads).Should(gomega.HaveLen(1))
					g.Expect(workloads[0].Spec.PodGroupTemplates).Should(gomega.HaveLen(2))
					g.Expect(podGroups).Should(gomega.HaveLen(2))
					for _, pg := range podGroups {
						podGroupsByName[pg.Name] = pg
					}
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify the initializer Pod is assigned to the basic PodGroup", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					pods := getReplicatedJobPods(g, trainJob, constants.DatasetInitializer)
					g.Expect(pods).Should(gomega.HaveLen(1))
					g.Expect(pods[0].Spec.SchedulingGroup).ShouldNot(gomega.BeNil())
					pg, ok := podGroupsByName[ptr.Deref(pods[0].Spec.SchedulingGroup.PodGroupName, "")]
					g.Expect(ok).Should(gomega.BeTrue())
					g.Expect(pg.Spec.SchedulingPolicy.Basic).ShouldNot(gomega.BeNil())
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify the node Pods are assigned to the gang PodGroup after the initializer completes", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					pods := getReplicatedJobPods(g, trainJob, constants.Node)
					g.Expect(pods).Should(gomega.HaveLen(2))
					for _, pod := range pods {
						g.Expect(pod.Spec.SchedulingGroup).ShouldNot(gomega.BeNil())
						pg, ok := podGroupsByName[ptr.Deref(pod.Spec.SchedulingGroup.PodGroupName, "")]
						g.Expect(ok).Should(gomega.BeTrue())
						g.Expect(pg.Spec.SchedulingPolicy.Gang).ShouldNot(gomega.BeNil())
						g.Expect(pg.Spec.SchedulingPolicy.Gang.MinCount).Should(gomega.Equal(int32(2)))
					}
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Wait for TrainJob to be in Succeeded status", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(meta.IsStatusConditionTrue(gotTrainJob.Status.Conditions, trainer.TrainJobComplete)).Should(gomega.BeTrue())
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating TrainJob with the Job-level (Level 3) scheduling", func() {
		ginkgo.It("should create a Job-level gang PodGroup sized to the Pods of the node Job", func() {
			// The empty Job-level scheduling configuration defaults to the gang policy.
			trainingRuntime := newTrainingRuntime(ns.Name, "e2e-was-level-3", &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{
						TargetReplicatedJobs: []string{constants.Node},
						Job:                  &trainer.JobScheduling{},
					},
				},
			},
				newReplicatedJob(constants.Node, constants.AncestorTrainer, constants.Node, "sleep 5"),
			)
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-was-level-3").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), trainingRuntime.Name).
				Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).
				Obj()

			ginkgo.By("Create a TrainingRuntime with the Job-level scheduling and a TrainJob with 2 nodes", func() {
				gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			var podGroup schedulingv1beta1.PodGroup
			ginkgo.By("Verify the Workload and the Job-level gang PodGroup are created", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					workloads, podGroups := getSchedulingObjects(g, trainJob)
					g.Expect(workloads).Should(gomega.HaveLen(1))
					g.Expect(workloads[0].Spec.PodGroupTemplates).Should(gomega.HaveLen(1))
					g.Expect(podGroups).Should(gomega.HaveLen(1))
					podGroup = podGroups[0]
					g.Expect(podGroup.Spec.SchedulingPolicy.Gang).ShouldNot(gomega.BeNil())
					// The Job-level gang size is the number of Pods of a single Job.
					g.Expect(podGroup.Spec.SchedulingPolicy.Gang.MinCount).Should(gomega.Equal(int32(2)))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify the node Pods are assigned to the Job-level PodGroup", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					pods := getReplicatedJobPods(g, trainJob, constants.Node)
					g.Expect(pods).Should(gomega.HaveLen(2))
					for _, pod := range pods {
						g.Expect(pod.Spec.SchedulingGroup).ShouldNot(gomega.BeNil())
						g.Expect(pod.Spec.SchedulingGroup.PodGroupName).Should(gomega.Equal(ptr.To(podGroup.Name)))
					}
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Wait for TrainJob to be in Succeeded status", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(meta.IsStatusConditionTrue(gotTrainJob.Status.Conditions, trainer.TrainJobComplete)).Should(gomega.BeTrue())
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})
})

// newTrainingRuntime returns a TrainingRuntime with the given scheduling configuration and ReplicatedJobs.
func newTrainingRuntime(namespace, name string, scheduling *trainer.TrainingRuntimeScheduling, rJobs ...jobsetv1alpha2.ReplicatedJob) *trainer.TrainingRuntime {
	return &trainer.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: trainer.TrainingRuntimeSpec{
			MLPolicy: &trainer.MLPolicy{
				NumNodes: ptr.To(int32(1)),
			},
			Scheduling: scheduling,
			Template: trainer.JobSetTemplateSpec{
				Spec: jobsetv1alpha2.JobSetSpec{
					ReplicatedJobs: rJobs,
				},
			},
		},
	}
}

// newReplicatedJob returns a ReplicatedJob with the given TrainJob ancestor and a single container which runs the
// given shell command.
func newReplicatedJob(name, ancestor, containerName, command string) jobsetv1alpha2.ReplicatedJob {
	return jobsetv1alpha2.ReplicatedJob{
		Name:     name,
		Replicas: 1,
		Template: batchv1.JobTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constants.LabelTrainJobAncestor: ancestor,
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						// Speed up the Pod deletion on suspend and namespace cleanup.
						TerminationGracePeriodSeconds: ptr.To(int64(1)),
						Containers: []corev1.Container{
							{
								Name:    containerName,
								Image:   runtimeImage,
								Command: []string{"sh", "-c", command},
							},
						},
					},
				},
			},
		},
	}
}

// getSchedulingObjects returns the Workloads and PodGroups controlled by the TrainJob.
func getSchedulingObjects(g gomega.Gomega, trainJob *trainer.TrainJob) ([]schedulingv1beta1.Workload, []schedulingv1beta1.PodGroup) {
	var workloadList schedulingv1beta1.WorkloadList
	g.Expect(k8sClient.List(ctx, &workloadList, client.InNamespace(trainJob.Namespace))).Should(gomega.Succeed())
	var podGroupList schedulingv1beta1.PodGroupList
	g.Expect(k8sClient.List(ctx, &podGroupList, client.InNamespace(trainJob.Namespace))).Should(gomega.Succeed())
	var workloads []schedulingv1beta1.Workload
	for _, workload := range workloadList.Items {
		if metav1.IsControlledBy(&workload, trainJob) {
			workloads = append(workloads, workload)
		}
	}
	var podGroups []schedulingv1beta1.PodGroup
	for _, podGroup := range podGroupList.Items {
		if metav1.IsControlledBy(&podGroup, trainJob) {
			podGroups = append(podGroups, podGroup)
		}
	}
	return workloads, podGroups
}

// getReplicatedJobPods returns the Pods of the TrainJob ReplicatedJob.
func getReplicatedJobPods(g gomega.Gomega, trainJob *trainer.TrainJob, rJobName string) []corev1.Pod {
	var podList corev1.PodList
	g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(trainJob.Namespace), client.MatchingLabels{
		jobsetv1alpha2.JobSetNameKey:        trainJob.Name,
		jobsetv1alpha2.ReplicatedJobNameKey: rJobName,
	})).Should(gomega.Succeed())
	return podList.Items
}

// setNodeExtendedResource advertises the extended resource with the given quantity in the node capacity, or removes
// it when the quantity is nil, and waits for the kubelet to reflect the change in the node allocatable.
func setNodeExtendedResource(nodeName string, name corev1.ResourceName, quantity *resource.Quantity) {
	value := "null"
	if quantity != nil {
		value = fmt.Sprintf("%q", quantity.String())
	}
	patch := fmt.Sprintf(`{"status":{"capacity":{%q:%s}}}`, name, value)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	gomega.Expect(k8sClient.Status().Patch(ctx, node, client.RawPatch(types.MergePatchType, []byte(patch)))).Should(gomega.Succeed())

	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(node), node)).Should(gomega.Succeed())
		allocatable, ok := node.Status.Allocatable[name]
		if quantity == nil {
			g.Expect(ok).Should(gomega.BeFalse())
			return
		}
		g.Expect(ok).Should(gomega.BeTrue())
		g.Expect(allocatable.Equal(*quantity)).Should(gomega.BeTrue())
	}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
}
