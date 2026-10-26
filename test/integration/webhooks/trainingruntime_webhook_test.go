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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
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

const trainingRuntimeName = "test-trainingruntime"

var _ = ginkgo.Describe("TrainingRuntime Webhook", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

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
				GenerateName: "trainingruntime-webhook-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainingRuntime{}, client.InNamespace(ns.Name))).To(gomega.Succeed())
	})

	ginkgo.When("Creating TrainingRuntime", func() {
		ginkgo.DescribeTable("Validate TrainingRuntime on creation", func(runtime func() *trainer.TrainingRuntime) {
			gomega.Expect(k8sClient.Create(ctx, runtime())).Should(gomega.Succeed())
		},
			ginkgo.Entry("Should succeed to create TrainingRuntime",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, trainingRuntimeName)
					return baseRuntime.
						RuntimeSpec(
							testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
								Obj()).
						Obj()
				}),
		)
	})
})

var _ = ginkgo.Describe("TrainingRuntime marker validations and defaulting", ginkgo.Ordered, func() {
	var ns *corev1.Namespace

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
				GenerateName: "training-runtime-marker-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})
	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.TrainingRuntime{}, client.InNamespace(ns.Name))).Should(gomega.Succeed())
		gomega.Expect(k8sClient.DeleteAllOf(ctx, &trainer.ClusterTrainingRuntime{})).Should(gomega.Succeed())
	})

	ginkgo.When("Creating TrainingRuntime", func() {
		ginkgo.DescribeTable("Validate TrainingRuntime on creation", func(trainingRuntime func() *trainer.TrainingRuntime, errorMatcher gomega.OmegaMatcher) {
			gomega.Expect(k8sClient.Create(ctx, trainingRuntime())).Should(errorMatcher)
		},
			ginkgo.Entry("Should succeed to create trainingRuntime",
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						Obj()
				},
				gomega.Succeed()),
			ginkgo.Entry("Should fail to create trainingRuntime with both MPI and Torch runtimes",
				func() *trainer.TrainingRuntime {
					runtime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj()
					runtime.Spec.MLPolicy = &trainer.MLPolicy{
						MLPolicySource: trainer.MLPolicySource{
							Torch: &trainer.TorchMLPolicySource{},
							MPI:   &trainer.MPIMLPolicySource{},
						},
					}
					return runtime
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create trainingRuntime with both JAX and Torch runtimes",
				func() *trainer.TrainingRuntime {
					runtime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj()
					runtime.Spec.MLPolicy = &trainer.MLPolicy{
						MLPolicySource: trainer.MLPolicySource{
							Torch: &trainer.TorchMLPolicySource{},
							JAX:   &trainer.JAXMLPolicySource{},
						},
					}
					return runtime
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create trainingRuntime with both Torch and Flux runtimes",
				func() *trainer.TrainingRuntime {
					runtime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj()
					runtime.Spec.MLPolicy = &trainer.MLPolicy{
						MLPolicySource: trainer.MLPolicySource{
							Torch: &trainer.TorchMLPolicySource{},
							Flux:  &trainer.FluxMLPolicySource{},
						},
					}
					return runtime
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create trainingRuntime with both MPI and Flux runtimes",
				func() *trainer.TrainingRuntime {
					runtime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj()
					runtime.Spec.MLPolicy = &trainer.MLPolicy{
						MLPolicySource: trainer.MLPolicySource{
							MPI:  &trainer.MPIMLPolicySource{},
							Flux: &trainer.FluxMLPolicySource{},
						},
					}
					return runtime
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create trainingRuntime with both JAX and Flux runtimes",
				func() *trainer.TrainingRuntime {
					runtime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj()
					runtime.Spec.MLPolicy = &trainer.MLPolicy{
						MLPolicySource: trainer.MLPolicySource{
							Flux: &trainer.FluxMLPolicySource{},
							JAX:  &trainer.JAXMLPolicySource{},
						},
					}
					return runtime
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create trainingRuntime with both JAX and MPI runtimes",
				func() *trainer.TrainingRuntime {
					runtime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj()
					runtime.Spec.MLPolicy = &trainer.MLPolicy{
						MLPolicySource: trainer.MLPolicySource{
							MPI: &trainer.MPIMLPolicySource{},
							JAX: &trainer.JAXMLPolicySource{},
						},
					}
					return runtime
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create trainingRuntime with all four runtimes configured",
				func() *trainer.TrainingRuntime {
					runtime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj()
					runtime.Spec.MLPolicy = &trainer.MLPolicy{
						MLPolicySource: trainer.MLPolicySource{
							Torch: &trainer.TorchMLPolicySource{},
							MPI:   &trainer.MPIMLPolicySource{},
							JAX:   &trainer.JAXMLPolicySource{},
							Flux:  &trainer.FluxMLPolicySource{},
						},
					}
					return runtime
				},
				testingutil.BeInvalidError()),
			ginkgo.Entry("Should fail to create trainingRuntime with non-supported mpi.mpiImplementation",
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(nil, "invalid", nil, nil).
										Obj(),
									).
									Obj(),
							).
							Obj()).
						Obj()
				},
				testingutil.BeInvalidError(),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with mpi.numProcPerNode < 1",
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](0), trainer.MPIImplementationOpenMPI, nil, nil).
										Obj(),
									).
									Obj(),
							).
							Obj()).
						Obj()
				},
				testingutil.BeInvalidError(),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with a negative coscheduling timeout",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							PodGroupPolicyCoschedulingSchedulingTimeout(-1).
							Obj()).
						Obj()
				},
				testingutil.BeInvalidError(),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with a zero coscheduling timeout",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							PodGroupPolicyCoschedulingSchedulingTimeout(0).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with a positive coscheduling timeout",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							PodGroupPolicyCoschedulingSchedulingTimeout(60).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with only volcano podGroupPolicy",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							PodGroupPolicyVolcano(&trainer.VolcanoPodGroupPolicySource{}).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with both coscheduling and volcano podGroupPolicy",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							PodGroupPolicyCoschedulingSchedulingTimeout(60).
							PodGroupPolicyVolcano(&trainer.VolcanoPodGroupPolicySource{}).
							Obj()).
						Obj()
				},
				testingutil.BeInvalidError(),
			),
			ginkgo.Entry("Rejects creating a TrainingRuntime with a container resources.claim that does not exist in the pod resourceClaims",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							ContainerResourceClaims(constants.Node, constants.Node, corev1.ResourceClaim{Name: "gpu"}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("references resourceClaim \"gpu\" which is not defined in the Pod's resourceClaims")),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with TrainJob-level gang scheduling",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
								},
								SchedulingConstraints: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingConstraints{
									Topology: []schedulingv1alpha3.TopologyConstraint{{Key: "topology.kubernetes.io/rack"}},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with empty scheduling",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{}).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with both scheduling and podGroupPolicy",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{}).
							PodGroupPolicyCoschedulingSchedulingTimeout(60).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("scheduling and podGroupPolicy are mutually exclusive")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with scheduling and JobSet scheduling",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					runtime := baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{}).
							Obj()).
						Obj()
					runtime.Spec.Template.Spec.Scheduling = &jobsetv1alpha2.JobSetScheduling{}
					return runtime
				},
				gomega.MatchError(gomega.ContainSubstring("JobSet scheduling must not be set, it is owned by the TrainJob controller")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with scheduling and Job scheduling",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					runtime := baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{}).
							Obj()).
						Obj()
					runtime.Spec.Template.Spec.ReplicatedJobs[2].Template.Spec.Scheduling = &batchv1.JobSchedulingConfiguration{}
					return runtime
				},
				gomega.MatchError(gomega.ContainSubstring("Job scheduling must not be set, it is owned by the TrainJob controller")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with scheduling and Pod schedulingGroup",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					runtime := baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{}).
							Obj()).
						Obj()
					runtime.Spec.Template.Spec.ReplicatedJobs[2].Template.Spec.Template.Spec.SchedulingGroup = &corev1.PodSchedulingGroup{
						PodGroupName: ptr.To("pod-group"),
					}
					return runtime
				},
				gomega.MatchError(gomega.ContainSubstring("Pod schedulingGroup must not be set, it is owned by the TrainJob controller")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with defaulted TrainJob-level gang scheduling and dependsOn",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{}).
							DependsOn(constants.Node, jobsetv1alpha2.DependsOn{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("TrainJob-level gang scheduling cannot be used with dependsOn or InOrder startupPolicy")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with TrainJob-level gang scheduling and InOrder startupPolicy",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					runtime := baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
								},
							}).
							Obj()).
						Obj()
					runtime.Spec.Template.Spec.StartupPolicy = &jobsetv1alpha2.StartupPolicy{
						StartupPolicyOrder: jobsetv1alpha2.InOrder,
					}
					return runtime
				},
				gomega.MatchError(gomega.ContainSubstring("TrainJob-level gang scheduling cannot be used with dependsOn or InOrder startupPolicy")),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with TrainJob-level basic scheduling and dependsOn",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
								},
							}).
							DependsOn(constants.Node, jobsetv1alpha2.DependsOn{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							}).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with per-ReplicatedJob scheduling and dependsOn",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{
									{
										TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer},
										SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
											Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
										},
									},
									{
										TargetReplicatedJobs: []string{constants.Node},
										DisruptionMode: &schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode{
											All: &schedulingv1alpha3.WorkloadCompositePodGroupAllDisruptionMode{},
										},
									},
								},
							}).
							DependsOn(constants.Node, jobsetv1alpha2.DependsOn{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							}).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with both TrainJob-level and per-ReplicatedJob scheduling",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
								},
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
									TargetReplicatedJobs: []string{constants.Node},
								}},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("schedulingPolicy, schedulingConstraints, and disruptionMode cannot be set together with replicatedJobs")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with a ReplicatedJob targeted by more than one entry",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{
									{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.Node}},
									{TargetReplicatedJobs: []string{constants.Node}},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("a ReplicatedJob must not be targeted by more than one replicatedJobs entry")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with empty targetReplicatedJobs",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{}},
							}).
							Obj()).
						Obj()
				},
				testingutil.BeInvalidError(),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with both basic and gang scheduling policy",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
									Gang:  &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("exactly one of basic or gang must be set")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with TrainJob-level gang.minGroupCount",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{MinGroupCount: ptr.To[int32](2)},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("gang.minGroupCount must not be set until CompositePodGroup is supported")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with per-ReplicatedJob gang.minGroupCount",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
									TargetReplicatedJobs: []string{constants.Node},
									SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
										Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{MinGroupCount: ptr.To[int32](1)},
									},
								}},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("gang.minGroupCount must not be set until CompositePodGroup is supported")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with both single and all disruptionMode",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								DisruptionMode: &schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode{
									Single: &schedulingv1alpha3.WorkloadCompositePodGroupSingleDisruptionMode{},
									All:    &schedulingv1alpha3.WorkloadCompositePodGroupAllDisruptionMode{},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("exactly one of single or all must be set")),
			),
			ginkgo.Entry("Should succeed to create trainingRuntime with Job-level scheduling",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							PodResourceClaims(constants.Node, corev1.PodResourceClaim{Name: "tpu-slice", ResourceClaimTemplateName: ptr.To("tpu-slice-template")}).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{
									{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
									{
										TargetReplicatedJobs: []string{constants.Node},
										Job: &trainer.JobScheduling{
											SchedulingPolicy: &schedulingv1alpha3.WorkloadPodGroupSchedulingPolicy{
												Gang: &schedulingv1alpha3.WorkloadPodGroupGangSchedulingPolicy{},
											},
											ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
												Name:                      "tpu-slice",
												ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
											}},
										},
									},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.Succeed(),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with Job-level resourceClaims without a matching Pod resourceClaim",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{
									{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
									{
										TargetReplicatedJobs: []string{constants.Node},
										Job: &trainer.JobScheduling{
											ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
												Name:                      "tpu-slice",
												ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
											}},
										},
									},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("must have a Pod resourceClaim with the same name")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with Job-level scheduling targeting more than one ReplicatedJob",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
									TargetReplicatedJobs: []string{constants.ModelInitializer, constants.Node},
									Job:                  &trainer.JobScheduling{},
								}},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("when job is set, targetReplicatedJobs must contain exactly one name")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with Job-level and per-ReplicatedJob scheduling on the same entry",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
									TargetReplicatedJobs: []string{constants.Node},
									SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
										Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
									},
									Job: &trainer.JobScheduling{},
								}},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("when job is set, targetReplicatedJobs must contain exactly one name")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with Job-level gang.minCount",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
									TargetReplicatedJobs: []string{constants.Node},
									Job: &trainer.JobScheduling{
										SchedulingPolicy: &schedulingv1alpha3.WorkloadPodGroupSchedulingPolicy{
											Gang: &schedulingv1alpha3.WorkloadPodGroupGangSchedulingPolicy{MinCount: ptr.To[int32](4)},
										},
									},
								}},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("gang.minCount is computed from numNodes and must not be set")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with Job-level resourceClaims setting both claim name and template name",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
									TargetReplicatedJobs: []string{constants.Node},
									Job: &trainer.JobScheduling{
										ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
											Name:                      "tpu-slice",
											ResourceClaimName:         ptr.To("tpu-slice"),
											ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
										}},
									},
								}},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("exactly one of resourceClaimName or resourceClaimTemplateName must be set")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with duplicate Job-level resourceClaims names",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
									TargetReplicatedJobs: []string{constants.Node},
									Job: &trainer.JobScheduling{
										ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{
											{Name: "tpu-slice", ResourceClaimName: ptr.To("tpu-slice")},
											{Name: "tpu-slice", ResourceClaimName: ptr.To("tpu-slice")},
										},
									},
								}},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("Duplicate value")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with targetReplicatedJobs referencing a ReplicatedJob which does not exist",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{
									{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
									{TargetReplicatedJobs: []string{constants.Node, "invalid"}},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("must reference a replicatedJob in spec.template.spec.replicatedJobs")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with per-ReplicatedJob scheduling leaving a ReplicatedJob untargeted",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								ReplicatedJobs: []trainer.ReplicatedJobScheduling{
									{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("must be targeted by a spec.scheduling.replicatedJobs entry")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with TrainJob-level scheduling and different priorityClassNames",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					runtime := baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{}).
							Obj()).
						Obj()
					runtime.Spec.Template.Spec.ReplicatedJobs[2].Template.Spec.Template.Spec.PriorityClassName = "high-priority"
					return runtime
				},
				gomega.MatchError(gomega.ContainSubstring("of the other ReplicatedJobs sharing the TrainJob-level PodGroup")),
			),
			ginkgo.Entry("Should fail to create trainingRuntime with basic scheduling and the all disruptionMode",
				func() *trainer.TrainingRuntime {
					baseRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime")
					return baseRuntime.
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(baseRuntime.Spec).
							Scheduling(&trainer.TrainingRuntimeScheduling{
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
								},
								DisruptionMode: &schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode{
									All: &schedulingv1alpha3.WorkloadCompositePodGroupAllDisruptionMode{},
								},
							}).
							Obj()).
						Obj()
				},
				gomega.MatchError(gomega.ContainSubstring("the disruptionMode `all` is not supported with the Basic scheduling policy")),
			),
		)
		ginkgo.DescribeTable("Defaulting TrainingRuntime on creation", func(trainingRuntime func() *trainer.TrainingRuntime, wantTrainingRuntime func() *trainer.TrainingRuntime) {
			created := trainingRuntime()
			gomega.Expect(k8sClient.Create(ctx, created)).Should(gomega.Succeed())
			gomega.Expect(created).Should(gomega.BeComparableTo(wantTrainingRuntime(), util.IgnoreObjectMetadata))
		},

			ginkgo.Entry("Should succeed to default mpi.mpiImplementation=OpenMPI",
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, ptr.To("/usr/dir"), ptr.To(false)).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				},
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, ptr.To("/usr/dir"), ptr.To(false)).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default mpi.sshAuthMountPath=/root/.ssh",
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, nil, ptr.To(false)).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				},
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, ptr.To("/root/.ssh"), ptr.To(false)).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default mpi.runLauncherAsNode=false",
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, ptr.To("/usr/dir"), nil).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				},
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, ptr.To("/usr/dir"), ptr.To(false)).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				}),
			ginkgo.Entry("Should succeed to default mpi.numProcPerNode=1",
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(nil, trainer.MPIImplementationOpenMPI, ptr.To("/usr/dir"), ptr.To(false)).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				},
				func() *trainer.TrainingRuntime {
					return testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").
						RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(
							testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Obj().Spec).
							WithMLPolicy(
								testingutil.MakeMLPolicyWrapper().
									WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
										MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, ptr.To("/usr/dir"), ptr.To(false)).
										Obj(),
									).
									Obj(),
							).
							JobSetSpec(
								testingutil.MakeJobSetWrapper(ns.Name, "jobset").
									Replicas(1, constants.Node, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
									Obj().
									Spec,
							).
							Obj(),
						).
						Obj()
				}),
		)
	})
})
