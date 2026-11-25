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

package controller

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

// This test reproduces the bug described in trainjob_controller.go:112
// TODO (astefanutti): the error should be surfaced in the TrainJob status to indicate
//
//	the creation of the runtime resources failed and the TrainJob is backed off until
//	the next retry attempt.
//
// Expected behavior: TrainJob status should have a condition indicating runtime creation failure
// Actual behavior: Error is only logged as an Event, which disappears after ~1 hour
//
// This is a FAILING TEST that demonstrates the bug exists.
var _ = ginkgo.Describe("TrainJob runtime resource creation error handling [Bug Reproduction]", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

	resRequests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	}

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
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "trainjob-runtime-error-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainJob{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.ClusterTrainingRuntime{})).Should(gomega.Succeed())
	})

	ginkgo.It("Should surface runtime resource creation errors in TrainJob status", func() {
		// Create a ClusterTrainingRuntime WITHOUT MLPolicy
		// This will cause validation/build to fail
		invalidRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("invalid-runtime").
			RuntimeSpec(trainer.TrainingRuntimeSpec{
				// No MLPolicy - this is required and will cause failure
				Template: trainer.JobSetTemplateSpec{
					Spec: jobsetv1alpha2.JobSetSpec{
						ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
							{
								Name: "worker",
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												// Invalid: no containers specified
												Containers: []corev1.Container{},
											},
										},
									},
								},
							},
						},
					},
				},
			}).
			Obj()

		gomega.Expect(k8sClient.Create(ctx, invalidRuntime)).Should(gomega.Succeed())

		// Create a TrainJob that references this invalid runtime
		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "test-runtime-error").
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "invalid-runtime").
			Suspend(false).
			Trainer(
				testingutil.MakeTrainJobTrainerWrapper().
					Container("test:latest", []string{"python", "train.py"}, []string{}, resRequests).
					Obj(),
			).
			Obj()

		trainJobKey := client.ObjectKeyFromObject(trainJob)
		gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

		// Wait for reconciliation to surface the error
		gomega.Eventually(func(g gomega.Gomega) {
			gotTrainJob := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, trainJobKey, gotTrainJob)).Should(gomega.Succeed())

			// With the fix applied, the TrainJob status should contain a condition
			// indicating runtime resource creation failure
			g.Expect(gotTrainJob.Status.Conditions).Should(
				gomega.ContainElement(
					gomega.And(
						gomega.HaveField("Type", gomega.Equal(trainer.TrainJobRuntimeResourcesReady)),
						gomega.HaveField("Status", gomega.Equal(metav1.ConditionFalse)),
						gomega.HaveField("Reason", gomega.Equal(trainer.TrainJobRuntimeResourcesCreationFailedReason)),
					),
				),
				"TrainJob status should have a RuntimeResourcesReady=False condition when runtime resource creation fails.",
			)

		}, util.Timeout, util.Interval).Should(gomega.Succeed())

		// Additional verification: Check that an Event was created
		// Poll with Eventually to handle race condition - Event might not be ready immediately
		gomega.Eventually(func(g gomega.Gomega) {
			eventList := &corev1.EventList{}
			g.Expect(k8sClient.List(ctx, eventList, client.InNamespace(ns.Name))).Should(gomega.Succeed())

			// Find the error event
			foundEvent := false
			for _, event := range eventList.Items {
				if event.InvolvedObject.Name == trainJob.Name &&
					event.Reason == "TrainJobResourcesCreationFailed" {
					foundEvent = true
					ginkgo.GinkgoWriter.Printf("Found event: %s - %s\n", event.Reason, event.Message)
				}
			}

			g.Expect(foundEvent).Should(gomega.BeTrue(),
				"Error should be in Events as well as Status.Conditions")
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
	})

	ginkgo.It("Should allow users to check runtime resource creation status without parsing Events", func() {
		// Create a valid runtime for comparison
		validRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("valid-runtime").
			RuntimeSpec(trainer.TrainingRuntimeSpec{
				MLPolicy: &trainer.MLPolicy{
					NumNodes: ptr.To[int32](1),
					MLPolicySource: trainer.MLPolicySource{
						Torch: &trainer.TorchMLPolicySource{},
					},
				},
				Template: trainer.JobSetTemplateSpec{
					Spec: jobsetv1alpha2.JobSetSpec{
						ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
							{
								Name: constants.Node,
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name:  constants.Node,
														Image: "test:runtime",
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
			}).
			Obj()

		gomega.Expect(k8sClient.Create(ctx, validRuntime)).Should(gomega.Succeed())

		trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "test-valid-runtime").
			RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "valid-runtime").
			Suspend(false).
			Trainer(
				testingutil.MakeTrainJobTrainerWrapper().
					Container("test:latest", []string{"python", "train.py"}, []string{}, resRequests).
					Obj(),
			).
			Obj()

		trainJobKey := client.ObjectKeyFromObject(trainJob)
		gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

		// DESIRED behavior: Status should have RuntimeResourcesReady=True when successful
		gomega.Eventually(func(g gomega.Gomega) {
			gotTrainJob := &trainer.TrainJob{}
			g.Expect(k8sClient.Get(ctx, trainJobKey, gotTrainJob)).Should(gomega.Succeed())

			// After fix, we expect this condition to exist for successful cases too
			g.Expect(gotTrainJob.Status.Conditions).Should(
				gomega.ContainElement(
					gomega.And(
						gomega.HaveField("Type", gomega.Equal("RuntimeResourcesReady")),
						gomega.HaveField("Status", gomega.Equal(metav1.ConditionTrue)),
					),
				),
				"TrainJob status should have RuntimeResourcesReady=True condition when runtime resources are created successfully",
			)

		}, util.Timeout, util.Interval).Should(gomega.Succeed())
	})
})
