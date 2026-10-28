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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	for _, kind := range []string{trainer.TrainingRuntimeKind, trainer.ClusterTrainingRuntimeKind} {
		ginkgo.Context(kind+" lifecycle validation", func() {
			ginkgo.DescribeTable("Validates lifecycle fields", func(mutate func(*trainer.TrainingRuntimeSpec), errorMatcher gomega.OmegaMatcher) {
				spec := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "runtime").Spec
				mutate(&spec)
				var runtime client.Object
				if kind == trainer.TrainingRuntimeKind {
					runtime = &trainer.TrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "runtime", Namespace: ns.Name}, Spec: spec}
				} else {
					runtime = &trainer.ClusterTrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "runtime"}, Spec: spec}
				}
				gomega.Expect(k8sClient.Create(ctx, runtime)).Should(errorMatcher)
			},
				ginkgo.Entry("accepts positive defaults and zero TTL", func(spec *trainer.TrainingRuntimeSpec) {
					spec.RunPolicy = &trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](1), TTLSecondsAfterFinished: ptr.To[int32](0)}
				}, gomega.Succeed()),
				ginkgo.Entry("rejects a negative runtime TTL", func(spec *trainer.TrainingRuntimeSpec) {
					spec.RunPolicy = &trainer.RunPolicy{TTLSecondsAfterFinished: ptr.To[int32](-1)}
				}, testingutil.BeInvalidError()),
				ginkgo.Entry("rejects a zero runtime deadline", func(spec *trainer.TrainingRuntimeSpec) {
					spec.RunPolicy = &trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](0)}
				}, testingutil.BeInvalidError()),
				ginkgo.Entry("rejects a negative runtime deadline", func(spec *trainer.TrainingRuntimeSpec) {
					spec.RunPolicy = &trainer.RunPolicy{ActiveDeadlineSeconds: ptr.To[int64](-1)}
				}, testingutil.BeInvalidError()),
				ginkgo.Entry("rejects JobSet TTL zero", func(spec *trainer.TrainingRuntimeSpec) {
					spec.Template.Spec.TTLSecondsAfterFinished = ptr.To[int32](0)
				}, gomega.MatchError(gomega.ContainSubstring("template.spec.ttlSecondsAfterFinished must be unset"))),
				ginkgo.Entry("rejects a positive JobSet TTL", func(spec *trainer.TrainingRuntimeSpec) {
					spec.Template.Spec.TTLSecondsAfterFinished = ptr.To[int32](60)
				}, gomega.MatchError(gomega.ContainSubstring("template.spec.ttlSecondsAfterFinished must be unset"))),
				ginkgo.Entry("rejects Job TTL zero", func(spec *trainer.TrainingRuntimeSpec) {
					spec.Template.Spec.ReplicatedJobs[0].Template.Spec.TTLSecondsAfterFinished = ptr.To[int32](0)
				}, gomega.MatchError(gomega.ContainSubstring("Job ttlSecondsAfterFinished must be unset"))),
				ginkgo.Entry("rejects a positive Job TTL", func(spec *trainer.TrainingRuntimeSpec) {
					spec.Template.Spec.ReplicatedJobs[0].Template.Spec.TTLSecondsAfterFinished = ptr.To[int32](60)
				}, gomega.MatchError(gomega.ContainSubstring("Job ttlSecondsAfterFinished must be unset"))),
				ginkgo.Entry("rejects a Job deadline", func(spec *trainer.TrainingRuntimeSpec) {
					spec.Template.Spec.ReplicatedJobs[0].Template.Spec.ActiveDeadlineSeconds = ptr.To[int64](60)
				}, gomega.MatchError(gomega.ContainSubstring("Job activeDeadlineSeconds must be unset"))),
				ginkgo.Entry("rejects a Pod deadline", func(spec *trainer.TrainingRuntimeSpec) {
					spec.Template.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.ActiveDeadlineSeconds = ptr.To[int64](60)
				}, gomega.MatchError(gomega.ContainSubstring("Pod activeDeadlineSeconds must be unset"))),
			)

			ginkgo.DescribeTable("Validates absent template paths", func(template map[string]interface{}, errorMatcher gomega.OmegaMatcher) {
				spec := map[string]interface{}{"runPolicy": map[string]interface{}{"ttlSecondsAfterFinished": int64(0)}}
				if template != nil {
					spec["template"] = template
				}
				runtime := &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": trainer.GroupVersion.String(),
					"kind":       kind,
					"metadata":   map[string]interface{}{"name": "runtime"},
					"spec":       spec,
				}}
				if kind == trainer.TrainingRuntimeKind {
					runtime.SetNamespace(ns.Name)
				}
				createErr := k8sClient.Create(ctx, runtime)
				gomega.Expect(createErr).Should(errorMatcher)
				if createErr != nil {
					return
				}
				gomega.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(runtime), runtime)).Should(gomega.Succeed())
				ttl, found, err := unstructured.NestedInt64(runtime.Object, "spec", "runPolicy", "ttlSecondsAfterFinished")
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(found).Should(gomega.BeTrue())
				gomega.Expect(ttl).Should(gomega.BeZero())
			},
				ginkgo.Entry("without template", nil, gomega.Succeed()),
				ginkgo.Entry("without template spec", map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"test": "lifecycle"}}}, gomega.Succeed()),
				ginkgo.Entry("without replicatedJobs", map[string]interface{}{"spec": map[string]interface{}{"suspend": true}}, gomega.Succeed()),
				ginkgo.Entry("rejects a missing required Job template", map[string]interface{}{"spec": map[string]interface{}{"replicatedJobs": []interface{}{map[string]interface{}{"name": "node"}}}}, gomega.MatchError(gomega.ContainSubstring("spec.template.spec.replicatedJobs[0].template: Required value"))),
				ginkgo.Entry("without Job spec", map[string]interface{}{"spec": map[string]interface{}{"replicatedJobs": []interface{}{map[string]interface{}{"name": "node", "template": map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"test": "lifecycle"}}}}}}}, gomega.Succeed()),
				ginkgo.Entry("rejects a missing required Pod template", map[string]interface{}{"spec": map[string]interface{}{"replicatedJobs": []interface{}{map[string]interface{}{"name": "node", "template": map[string]interface{}{"spec": map[string]interface{}{"parallelism": int64(1)}}}}}}, gomega.MatchError(gomega.ContainSubstring("spec.template.spec.replicatedJobs[0].template.spec.template: Required value"))),
				ginkgo.Entry("without Pod spec", map[string]interface{}{"spec": map[string]interface{}{"replicatedJobs": []interface{}{map[string]interface{}{"name": "node", "template": map[string]interface{}{"spec": map[string]interface{}{"template": map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"test": "lifecycle"}}}}}}}}}, gomega.Succeed()),
			)

			ginkgo.It("allows updating and clearing runtime defaults", func() {
				runtime := &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": trainer.GroupVersion.String(),
					"kind":       kind,
					"metadata":   map[string]interface{}{"name": "runtime"},
					"spec": map[string]interface{}{
						"runPolicy": map[string]interface{}{"activeDeadlineSeconds": int64(60), "ttlSecondsAfterFinished": int64(60)},
						"mlPolicy":  map[string]interface{}{"numNodes": int64(1)},
					},
				}}
				if kind == trainer.TrainingRuntimeKind {
					runtime.SetNamespace(ns.Name)
				}
				gomega.Expect(k8sClient.Create(ctx, runtime)).Should(gomega.Succeed())
				gomega.Expect(unstructured.SetNestedField(runtime.Object, int64(120), "spec", "runPolicy", "activeDeadlineSeconds")).Should(gomega.Succeed())
				gomega.Expect(unstructured.SetNestedField(runtime.Object, int64(0), "spec", "runPolicy", "ttlSecondsAfterFinished")).Should(gomega.Succeed())
				gomega.Expect(k8sClient.Update(ctx, runtime)).Should(gomega.Succeed())
				unstructured.RemoveNestedField(runtime.Object, "spec", "runPolicy")
				gomega.Expect(k8sClient.Update(ctx, runtime)).Should(gomega.Succeed())
			})
		})
	}

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
