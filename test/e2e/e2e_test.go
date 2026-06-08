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

package e2e

import (
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/util"

	_ "embed"
)

const (
	torchRuntime     = "torch-distributed"
	deepSpeedRuntime = "deepspeed-distributed"
	jaxRuntime       = "jax-distributed"
	xgboostRuntime   = "xgboost-distributed"
)

//go:embed testdata/status_update.py
var statusUpdateScript string

var _ = ginkgo.Describe("TrainJob e2e", func() {
	// Each test runs in a separate namespace.
	var ns *corev1.Namespace

	// Create test namespace before each test.
	ginkgo.BeforeEach(func() {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-",
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
		// Delete test namespace after each test.
		gomega.Expect(k8sClient.Delete(ctx, ns)).To(gomega.Succeed())
	})

	// These tests create TrainJob that reference supported runtime without any additional changes.
	ginkgo.When("Creating TrainJob to perform the PyTorch workload", func() {
		// Verify the `torch-distributed` ClusterTrainingRuntime.
		ginkgo.It("should create TrainJob with PyTorch runtime reference", func() {
			// Create a TrainJob.
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-torch").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				Obj()

			ginkgo.By("Create a TrainJob with torch-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for jobs to become active
			ginkgo.By("Wait for TrainJob jobs to become active", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(0)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(1)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status with all jobs succeeded.
			ginkgo.By("Wait for TrainJob to be in Succeeded status with all jobs succeeded", func() {
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
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(1)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(0)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating TrainJob to perform OpenMPI workload", func() {
		// Verify the `deepspeed-distributed` ClusterTrainingRuntime.
		ginkgo.It("should create TrainJob with DeepSpeed runtime reference", func() {
			// Create a TrainJob.
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-deepspeed").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), deepSpeedRuntime).
				Obj()

			ginkgo.By("Create a TrainJob with deepspeed-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for jobs to become active
			ginkgo.By("Wait for TrainJob jobs to become active", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Launcher,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(0)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(1)),
							Suspended: ptr.To(int32(0)),
						},
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(0)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(1)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status.
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
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Launcher,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(1)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(0)),
							Suspended: ptr.To(int32(0)),
						},
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(0)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(0)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating TrainJob to perform JAX workload", func() {
		// Verify the `jax-distributed` ClusterTrainingRuntime.
		ginkgo.It("should create TrainJob with JAX runtime reference", func() {
			// Create a TrainJob.
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-jax").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), jaxRuntime).
				Obj()

			ginkgo.By("Create a TrainJob with jax-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for jobs to become active
			ginkgo.By("Wait for TrainJob jobs to become active", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(0)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(1)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status with all jobs succeeded.
			ginkgo.By("Wait for TrainJob to be in Succeeded status with all jobs succeeded", func() {
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
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(1)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(0)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating TrainJob to perform XGBoost workload", func() {
		// Verify the `xgboost-distributed` ClusterTrainingRuntime.
		ginkgo.It("should create TrainJob with XGBoost runtime reference", func() {
			// Create a TrainJob.
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-xgboost").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), xgboostRuntime).
				Obj()

			ginkgo.By("Create a TrainJob with xgboost-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for jobs to become active
			ginkgo.By("Wait for TrainJob jobs to become active", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(0)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(1)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status with all jobs succeeded.
			ginkgo.By("Wait for TrainJob to be in Succeeded status with all jobs succeeded", func() {
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
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     ptr.To(int32(0)),
							Succeeded: ptr.To(int32(1)),
							Failed:    ptr.To(int32(0)),
							Active:    ptr.To(int32(0)),
							Suspended: ptr.To(int32(0)),
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating a TrainJob with RuntimePatches", func() {
		ginkgo.It("should preserve user-provided manager fields", func() {
			userTime := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))

			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				RuntimePatches([]trainer.RuntimePatch{
					{
						Manager: "test.io/manager-one",
						Time:    &userTime,
						TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
							Template: &trainer.JobSetTemplatePatch{
								Spec: &trainer.JobSetSpecPatch{
									ReplicatedJobs: []trainer.ReplicatedJobPatch{{
										Name: constants.Node,
										Template: &trainer.JobTemplatePatch{
											Spec: &trainer.JobSpecPatch{
												Template: &trainer.PodTemplatePatch{
													Spec: &trainer.PodSpecPatch{
														ServiceAccountName: ptr.To("test-sa-1"),
													},
												},
											},
										},
									}},
								},
							},
						},
					},
					{
						Manager: "kueue.k8s.io/manager",
						TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
							Template: &trainer.JobSetTemplatePatch{
								Spec: &trainer.JobSetSpecPatch{
									ReplicatedJobs: []trainer.ReplicatedJobPatch{{
										Name: constants.Node,
										Template: &trainer.JobTemplatePatch{
											Spec: &trainer.JobSpecPatch{
												Template: &trainer.PodTemplatePatch{
													Spec: &trainer.PodSpecPatch{
														ServiceAccountName: ptr.To("test-sa-2"),
													},
												},
											},
										},
									}},
								},
							},
						},
					},
				}).
				Obj()

			ginkgo.By("Create a TrainJob with RuntimePatches", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			ginkgo.By("Verify user-provided Time is preserved for test.io/manager-one and Time is set for kueue.k8s.io/manager", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Spec.RuntimePatches).Should(gomega.HaveLen(2))
					g.Expect(gotTrainJob.Spec.RuntimePatches[0].Manager).To(gomega.Equal("test.io/manager-one"))
					g.Expect(gotTrainJob.Spec.RuntimePatches[0].Time).ShouldNot(gomega.BeNil())
					g.Expect(gotTrainJob.Spec.RuntimePatches[0].Time.Equal(&userTime)).To(gomega.BeTrue())
					g.Expect(gotTrainJob.Spec.RuntimePatches[1].Manager).To(gomega.Equal("kueue.k8s.io/manager"))
					g.Expect(gotTrainJob.Spec.RuntimePatches[1].Time).ShouldNot(gomega.BeNil())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())
			})
		})

		ginkgo.It("should update Time only for the changed patch on update", func() {
			userTime := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
			newPatchTime := metav1.NewTime(time.Now().Add(-30 * time.Minute).Truncate(time.Second))

			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-update-time").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				Suspend(true).
				RuntimePatches([]trainer.RuntimePatch{
					{
						Manager: "test.io/unchanged",
						TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
							Template: &trainer.JobSetTemplatePatch{
								Spec: &trainer.JobSetSpecPatch{
									ReplicatedJobs: []trainer.ReplicatedJobPatch{{
										Name: constants.Node,
										Template: &trainer.JobTemplatePatch{
											Spec: &trainer.JobSpecPatch{
												Template: &trainer.PodTemplatePatch{
													Spec: &trainer.PodSpecPatch{
														NodeSelector: map[string]string{"zone": "keep"},
													},
												},
											},
										},
									}},
								},
							},
						},
					},
					{
						Manager: "test.io/will-change",
						Time:    &userTime,
						TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
							Template: &trainer.JobSetTemplatePatch{
								Spec: &trainer.JobSetSpecPatch{
									ReplicatedJobs: []trainer.ReplicatedJobPatch{{
										Name: constants.Node,
										Template: &trainer.JobTemplatePatch{
											Spec: &trainer.JobSpecPatch{
												Template: &trainer.PodTemplatePatch{
													Spec: &trainer.PodSpecPatch{
														NodeSelector: map[string]string{"zone": "old"},
													},
												},
											},
										},
									}},
								},
							},
						},
					},
				}).
				Obj()

			ginkgo.By("Create a suspended TrainJob with two RuntimePatches", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			var unchangedTime *metav1.Time
			ginkgo.By("Record the original Time values and verify user-provided Time is preserved", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Spec.RuntimePatches).Should(gomega.HaveLen(2))
					g.Expect(gotTrainJob.Spec.RuntimePatches[0].Time).ShouldNot(gomega.BeNil())
					// User-provided Time for test.io/will-change must be preserved on create.
					g.Expect(gotTrainJob.Spec.RuntimePatches[1].Time).ShouldNot(gomega.BeNil())
					g.Expect(gotTrainJob.Spec.RuntimePatches[1].Time.Equal(&userTime)).To(gomega.BeTrue())
					unchangedTime = gotTrainJob.Spec.RuntimePatches[0].Time.DeepCopy()
				}, util.Timeout, util.Interval).Should(gomega.Succeed())
			})

			// metav1.Time serializes to second precision (RFC3339), so we must
			// wait at least one second to guarantee distinguishable timestamps.
			time.Sleep(time.Second)

			ginkgo.By("Update the second RuntimePatch and add two new patches", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					gotTrainJob.Spec.RuntimePatches[1].TrainingRuntimeSpec.Template.Spec.ReplicatedJobs[0].
						Template.Spec.Template.Spec.NodeSelector = map[string]string{"zone": "new"}
					// New patch without Time: webhook should stamp it.
					gotTrainJob.Spec.RuntimePatches = append(gotTrainJob.Spec.RuntimePatches, trainer.RuntimePatch{
						Manager: "test.io/new-no-time",
					})
					// New patch with user-provided Time: should be preserved.
					gotTrainJob.Spec.RuntimePatches = append(gotTrainJob.Spec.RuntimePatches, trainer.RuntimePatch{
						Manager: "test.io/new-with-time",
						Time:    &newPatchTime,
					})
					g.Expect(k8sClient.Update(ctx, gotTrainJob)).Should(gomega.Succeed())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())
			})

			ginkgo.By("Verify Time behaviour for all patches after update", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Spec.RuntimePatches).Should(gomega.HaveLen(4))
					// Unchanged pre-existing patch: Time must be preserved.
					g.Expect(gotTrainJob.Spec.RuntimePatches[0].Time).ShouldNot(gomega.BeNil())
					g.Expect(gotTrainJob.Spec.RuntimePatches[0].Time.Equal(unchangedTime)).To(gomega.BeTrue())
					// Changed pre-existing patch: user-provided Time must be replaced.
					g.Expect(gotTrainJob.Spec.RuntimePatches[1].Time).ShouldNot(gomega.BeNil())
					g.Expect(gotTrainJob.Spec.RuntimePatches[1].Time.Equal(&userTime)).To(gomega.BeFalse())
					// New patch without Time: webhook must stamp it.
					g.Expect(gotTrainJob.Spec.RuntimePatches[2].Time).ShouldNot(gomega.BeNil())
					// New patch with user-provided Time: must be preserved.
					g.Expect(gotTrainJob.Spec.RuntimePatches[3].Time).ShouldNot(gomega.BeNil())
					g.Expect(gotTrainJob.Spec.RuntimePatches[3].Time.Equal(&newPatchTime)).To(gomega.BeTrue())
				}, util.Timeout, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating a TrainJob with Resource Timeouts", func() {
		ginkgo.It("should fail the TrainJob with DeadlineExceeded when active timeout expires", func() {
			deadline := int64(10)
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-deadline-job").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				ActiveDeadlineSeconds(deadline).
				Obj()

			trainJob.Spec.Trainer = &trainer.Trainer{
				Image:   ptr.To("busybox"),
				Command: []string{"/bin/sh", "-c", "sleep 600"},
			}
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			trainJobKey := client.ObjectKeyFromObject(trainJob)

			ginkgo.By("Waiting for TrainJob to fail due to deadline")
			gomega.Eventually(func(g gomega.Gomega) {
				gotTrainJob := &trainer.TrainJob{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, gotTrainJob)).Should(gomega.Succeed())
				g.Expect(gotTrainJob.Status.Conditions).Should(gomega.ContainElement(gomega.HaveField("Reason", trainer.TrainJobDeadlineExceededReason)))
			}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())

			ginkgo.By("Ensuring the underlying JobSet is deleted")
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, trainJobKey, &jobsetv1alpha2.JobSet{})).Should(testingutil.BeNotFoundError())
			}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
		})
	})

	ginkgo.When("Creating TrainJob with runtime status server instrumentation", func() {
		ginkgo.It("should inject runtime configuration which allows the runtime status endpoint to be called", func() {
			// Create a TrainJob that sends a single runtime status update and exits
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-runtime-status").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				Trainer(&trainer.Trainer{
					Command: []string{"python3", "-c"},
					Args:    []string{statusUpdateScript},
				}).
				Obj()

			ginkgo.By("Create a TrainJob that will call the runtime-status endpoint", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			ginkgo.By("Verify trainerStatus is updated with runtime status information", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())

					// Verify trainerStatus is not nil
					g.Expect(gotTrainJob.Status.TrainerStatus).ShouldNot(gomega.BeNil())

					// Verify progress percentage
					g.Expect(gotTrainJob.Status.TrainerStatus.ProgressPercentage).ShouldNot(gomega.BeNil())
					g.Expect(*gotTrainJob.Status.TrainerStatus.ProgressPercentage).Should(gomega.Equal(int32(42)))

					// Verify estimated remaining seconds
					g.Expect(gotTrainJob.Status.TrainerStatus.EstimatedRemainingSeconds).ShouldNot(gomega.BeNil())
					g.Expect(*gotTrainJob.Status.TrainerStatus.EstimatedRemainingSeconds).Should(gomega.Equal(int32(120)))

					// Verify metrics
					g.Expect(gotTrainJob.Status.TrainerStatus.Metrics).Should(gomega.HaveLen(2))
					g.Expect(gotTrainJob.Status.TrainerStatus.Metrics[0].Name).Should(gomega.Equal("loss"))
					g.Expect(gotTrainJob.Status.TrainerStatus.Metrics[0].Value).Should(gomega.Equal("0.123"))
					g.Expect(gotTrainJob.Status.TrainerStatus.Metrics[1].Name).Should(gomega.Equal("accuracy"))
					g.Expect(gotTrainJob.Status.TrainerStatus.Metrics[1].Value).Should(gomega.Equal("0.95"))

					// Verify lastUpdatedTime is set
					g.Expect(gotTrainJob.Status.TrainerStatus.LastUpdatedTime.IsZero()).Should(gomega.BeFalse())
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
		})
	})

	ginkgo.When("Updating a runtime", func() {
		// These tests ensure that a TrainJob is using a "snapshot" of the runtime from creation time, rather than
		// the latest value of the runtime.
		// This is testing the "snapshot" mechanism from docs/proposals/2599-mutable-runtimes/README.md.

		const (
			// These images are arbitrary, but are chosen to be small and to come from
			// the kubernetes registry to avoid Docker Hub rate limits in CI.
			initialImage = "registry.k8s.io/pause:3.9"
			updatedImage = "registry.k8s.io/pause:3.10"
		)

		initialRuntimeSpec := trainer.TrainingRuntimeSpec{
			Template: trainer.JobSetTemplateSpec{
				Spec: jobsetv1alpha2.JobSetSpec{
					ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
						{
							Name: constants.Node,
							Template: batchv1.JobTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										constants.LabelTrainJobAncestor: constants.AncestorTrainer,
									},
								},
								Spec: batchv1.JobSpec{
									Template: corev1.PodTemplateSpec{
										Spec: corev1.PodSpec{
											Containers: []corev1.Container{
												{
													Name:    constants.Node,
													Image:   initialImage,
													Command: []string{"/pause"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		updateRuntimeImage := func(spec *trainer.TrainingRuntimeSpec, newImage string) {
			for i, rJob := range spec.Template.Spec.ReplicatedJobs {
				if rJob.Name == constants.Node {
					spec.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[0].Image = newImage
				}
			}
		}

		ginkgo.DescribeTable("a suspended TrainJob should use the original runtime configuration and not pick up the new configuration",
			func(runtimeFactory func() client.Object, runtimeKind string) {
				runtime := runtimeFactory()

				ginkgo.By("Creating a "+runtimeKind, func() {
					gomega.Expect(k8sClient.Create(ctx, runtime)).Should(gomega.Succeed())
				})

				ginkgo.DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, runtime)
				})

				// Create a TrainJob that references the runtime
				trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-runtime-update").
					RuntimeRef(trainer.SchemeGroupVersion.WithKind(runtimeKind), runtime.GetName()).
					Obj()

				ginkgo.By("Creating a TrainJob that references the "+runtimeKind, func() {
					gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
				})

				ginkgo.By("Waiting for TrainJob jobs to become active", func() {
					gomega.Eventually(func(g gomega.Gomega) {
						gotTrainJob := &trainer.TrainJob{}
						g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
						g.Expect(gotTrainJob.Status.JobsStatus).ShouldNot(gomega.BeEmpty())
						for _, jobStatus := range gotTrainJob.Status.JobsStatus {
							g.Expect(*jobStatus.Active).Should(gomega.BeNumerically(">", 0))
						}
					}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
				})

				ginkgo.By("Pausing the TrainJob", func() {
					gomega.Eventually(func(g gomega.Gomega) {
						gotTrainJob := &trainer.TrainJob{}
						g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
						gotTrainJob.Spec.Suspend = ptr.To(true)
						g.Expect(k8sClient.Update(ctx, gotTrainJob)).Should(gomega.Succeed())
					}, util.Timeout, util.Interval).Should(gomega.Succeed())
				})

				ginkgo.By("Waiting for TrainJob to be suspended", func() {
					gomega.Eventually(func(g gomega.Gomega) {
						gotTrainJob := &trainer.TrainJob{}
						g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
						g.Expect(gotTrainJob.Status.JobsStatus).ShouldNot(gomega.BeEmpty())
						for _, jobStatus := range gotTrainJob.Status.JobsStatus {
							g.Expect(*jobStatus.Suspended).Should(gomega.BeNumerically(">", 0))
						}
					}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
				})

				// Container image is NOT in JobSet's allowed mutable fields for suspended JobSets
				// (only annotations, labels, nodeSelector, tolerations, schedulingGates are allowed)
				ginkgo.By("Updating the runtime with a new container image", func() {
					gomega.Eventually(func(g gomega.Gomega) {
						// Get the runtime with the correct type based on the runtime object
						gotRuntime := runtime.DeepCopyObject().(client.Object)
						g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(runtime), gotRuntime)).Should(gomega.Succeed())

						switch runtimeKind {
						case trainer.TrainingRuntimeKind:
							r := gotRuntime.(*trainer.TrainingRuntime)
							updateRuntimeImage(&r.Spec, updatedImage)
						case trainer.ClusterTrainingRuntimeKind:
							r := gotRuntime.(*trainer.ClusterTrainingRuntime)
							updateRuntimeImage(&r.Spec, updatedImage)
						default:
							ginkgo.Fail("unexpected runtime kind: " + runtimeKind)
						}

						g.Expect(k8sClient.Update(ctx, gotRuntime)).Should(gomega.Succeed())
					}, util.Timeout, util.Interval).Should(gomega.Succeed())
				})

				ginkgo.By("Restarting the TrainJob by unpausing it", func() {
					gomega.Eventually(func(g gomega.Gomega) {
						gotTrainJob := &trainer.TrainJob{}
						g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
						gotTrainJob.Spec.Suspend = ptr.To(false)
						g.Expect(k8sClient.Update(ctx, gotTrainJob)).Should(gomega.Succeed())
					}, util.Timeout, util.Interval).Should(gomega.Succeed())
				})

				// This will fail if the TrainJob uses the updated runtime rather than the snapshot as the controller will
				// try to update the JobSet image which is immutable.
				ginkgo.By("Verifying the TrainJob jobs become active after resuming", func() {
					gomega.Eventually(func(g gomega.Gomega) {
						gotTrainJob := &trainer.TrainJob{}
						g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
						g.Expect(gotTrainJob.Status.JobsStatus).ShouldNot(gomega.BeEmpty())
						// Expect jobs to be active (not suspended)
						for _, jobStatus := range gotTrainJob.Status.JobsStatus {
							g.Expect(*jobStatus.Active).Should(gomega.BeNumerically(">", 0))
						}
					}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
				})
			},
			ginkgo.Entry("TrainingRuntime",
				func() client.Object {
					return &trainer.TrainingRuntime{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: ns.Name,
							Name:      "test-runtime",
						},
						Spec: *initialRuntimeSpec.DeepCopy(),
					}
				},
				trainer.TrainingRuntimeKind,
			),
			ginkgo.Entry("ClusterTrainingRuntime",
				func() client.Object {
					return &trainer.ClusterTrainingRuntime{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-runtime",
						},
						Spec: *initialRuntimeSpec.DeepCopy(),
					}
				},
				trainer.ClusterTrainingRuntimeKind,
			),
		)
	})
})
