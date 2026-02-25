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

package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobsetplgconsts "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jobset/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestTrainingRuntimeNewObjects(t *testing.T) {
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	// TODO (andreyvelich): Add more test cases.
	cases := map[string]struct {
		trainingRuntime *trainer.TrainingRuntime
		trainJob        *trainer.TrainJob
		ObjCmpOpts      []cmp.Option
		wantObjs        []runtime.Object
		wantError       error
	}{
		// Test cases for the PlainML MLPolicy.
		"succeeded to build PodGroup and JobSet with NumNodes from the TrainJob and container from the Runtime.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").
				Label("conflictLabel", "overridden").
				Annotation("conflictAnnotation", "overridden").
				RuntimeSpec(
					testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
						WithMLPolicy(
							testingutil.MakeMLPolicyWrapper().
								WithNumNodes(100).
								Obj(),
						).
						PodGroupPolicyCoschedulingSchedulingTimeout(120).
						Container(constants.DatasetInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Container(constants.ModelInitializer, constants.ModelInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Obj(),
				).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Suspend(true).
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				SpecLabel("conflictLabel", "override").
				SpecAnnotation("conflictAnnotation", "override").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						NumNodes(30).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Suspend(true).
					Label("conflictLabel", "override").
					Annotation("conflictAnnotation", "override").
					PodLabel(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
					NumNodes(30).
					Container(constants.DatasetInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.ModelInitializer, constants.ModelInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
				testingutil.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					MinMember(32). // 32 replicas = 30 Trainer nodes + 2 Initializer.
					MinResources(corev1.ResourceList{
						// Trainer node has 30 CPUs + 2 CPUs from 2 initializer containers.
						corev1.ResourceCPU: resource.MustParse("32"),
					}).
					SchedulingTimeout(120).
					Obj(),
			},
		},
		"succeeded to build JobSet with NumNodes from the Runtime and container from the TrainJob.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						}...,
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Env(
							[]corev1.EnvVar{
								{
									Name:  "TRAIN_JOB",
									Value: "override",
								},
								{
									Name:  "TRAIN_JOB_CUSTOM",
									Value: "test:trainjob",
								},
							}...,
						).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(100).
					Container(constants.Node, constants.Node, "test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "override",
							},
							{
								Name:  "TRAIN_JOB_CUSTOM",
								Value: "test:trainjob",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						}...,
					).
					Obj(),
			},
		},
		"succeeded to build JobSet with TrainJob's PodTemplateOverrides.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					InitContainer(constants.DatasetInitializer, "override-init-container", "test:runtime",
						[]corev1.EnvVar{
							{
								Name:  "INIT_ENV",
								Value: "original_init",
							},
							{
								Name:  "DATASET_PATH",
								Value: "runtime",
							},
						}...,
					).
					InitContainer(constants.Node, "override-init-container", "test:runtime",
						[]corev1.EnvVar{
							{
								Name:  "INIT_ENV",
								Value: "original_init",
							},
						}...,
					).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
						}...,
					).
					Container(constants.Node, "override-container", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Env(constants.Node, "override-container",
						[]corev1.EnvVar{
							{
								Name:  "CONTAINER_ENV",
								Value: "original_container",
							},
						}...,
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						Manager:    ptr.To("manager-1"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.DatasetInitializer}},
						Spec: &trainer.PodTemplateSpecOverride{
							InitContainers: []trainer.ContainerOverride{
								{
									Name: "override-init-container",
									Env: []corev1.EnvVar{
										{
											Name:  "INIT_ENV",
											Value: "override_init",
										},
										{
											Name:  "NEW_VALUE",
											Value: "from_overrides",
										},
									},
								},
							},
							Containers: []trainer.ContainerOverride{
								{
									Name: constants.DatasetInitializer,
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "initializer_secret",
											MountPath: "initializer_secret_mount_path",
										},
										{
											Name:      "initializer_claim",
											MountPath: "initializer_claim_mount_path",
										},
									},
								},
							},
							Affinity: &corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{
											{
												MatchExpressions: []corev1.NodeSelectorRequirement{
													{
														Key:      "topology.kubernetes.io/zone",
														Operator: corev1.NodeSelectorOpIn,
														Values:   []string{"antarctica-east1", "antarctica-west1"},
													},
												},
											},
										},
									},
								},
							},
							NodeSelector: map[string]string{
								"node.kubernetes.io/instance-type": "p5.48xlarge",
							},
							SchedulingGates: []corev1.PodSchedulingGate{
								{
									Name: "kueue.x-k8s.io/admission",
								},
							},
							Tolerations: []corev1.Toleration{
								{
									Key:      "example.com/gpu",
									Operator: corev1.TolerationOpExists,
									Effect:   corev1.TaintEffectNoSchedule,
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "initializer_secret",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "initializer_secret_name",
										},
									},
								},
								{
									Name: "initializer_claim",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "initializer_claim_name",
										},
									},
								},
							},
						},
					},
					{
						Manager:    ptr.To("manager-2"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("override-sa"),
							InitContainers: []trainer.ContainerOverride{
								{
									Name: "override-init-container",
									Env: []corev1.EnvVar{
										{
											Name:  "INIT_ENV",
											Value: "override_init",
										},
									},
								},
							},
							Containers: []trainer.ContainerOverride{
								{
									Name: constants.Node,
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "node_secret",
											MountPath: "node_secret_mount_path",
										},
										{
											Name:      "node_claim",
											MountPath: "node_claim_mount_path",
										},
									},
								},
								{
									Name: "override-container",
									Env: []corev1.EnvVar{
										{
											Name:  "CONTAINER_ENV",
											Value: "override_container",
										},
									},
								},
							},
							Affinity: &corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{
											{
												MatchExpressions: []corev1.NodeSelectorRequirement{
													{
														Key:      "topology.kubernetes.io/zone",
														Operator: corev1.NodeSelectorOpIn,
														Values:   []string{"antarctica-east1", "antarctica-west1"},
													},
												},
											},
										},
									},
								},
							},
							ImagePullSecrets: []corev1.LocalObjectReference{
								{Name: "registry-credential"},
							},
							NodeSelector: map[string]string{
								"node.kubernetes.io/instance-type": "p5.48xlarge",
							},
							SchedulingGates: []corev1.PodSchedulingGate{
								{
									Name: "kueue.x-k8s.io/admission",
								},
							},
							Tolerations: []corev1.Toleration{
								{
									Key:      "example.com/gpu",
									Operator: corev1.TolerationOpExists,
									Effect:   corev1.TaintEffectNoSchedule,
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "node_secret",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "node_secret_name",
										},
									},
								},
								{
									Name: "node_claim",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "node_claim_name",
										},
									},
								},
							},
						},
					},
				}).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					ServiceAccountName(constants.Node, "override-sa").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(100).
					InitContainer(constants.DatasetInitializer, "override-init-container", "test:runtime",
						[]corev1.EnvVar{
							{
								Name:  "INIT_ENV",
								Value: "override_init",
							},
							{
								Name:  "DATASET_PATH",
								Value: "runtime",
							},
							{
								Name:  "NEW_VALUE",
								Value: "from_overrides",
							},
						}...).
					InitContainer(constants.Node, "override-init-container", "test:runtime",
						[]corev1.EnvVar{
							{
								Name:  "INIT_ENV",
								Value: "override_init",
							},
						}...).
					Container(constants.Node, constants.Node, "test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
						}...,
					).
					Container(constants.Node, "override-container", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Env(constants.Node, "override-container",
						[]corev1.EnvVar{
							{
								Name:  "CONTAINER_ENV",
								Value: "override_container",
							},
						}...,
					).
					Affinity(constants.DatasetInitializer,
						corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "topology.kubernetes.io/zone",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"antarctica-east1", "antarctica-west1"},
												},
											},
										},
									},
								},
							},
						},
					).
					Affinity(constants.Node,
						corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "topology.kubernetes.io/zone",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"antarctica-east1", "antarctica-west1"},
												},
											},
										},
									},
								},
							},
						},
					).
					ImagePullSecrets(constants.Node,
						corev1.LocalObjectReference{Name: "registry-credential"},
					).
					NodeSelector(constants.DatasetInitializer,
						map[string]string{
							"node.kubernetes.io/instance-type": "p5.48xlarge",
						}).
					NodeSelector(constants.Node,
						map[string]string{
							"node.kubernetes.io/instance-type": "p5.48xlarge",
						}).
					SchedulingGates(constants.DatasetInitializer, corev1.PodSchedulingGate{
						Name: "kueue.x-k8s.io/admission",
					}).
					SchedulingGates(constants.Node, corev1.PodSchedulingGate{
						Name: "kueue.x-k8s.io/admission",
					}).
					Tolerations(constants.DatasetInitializer,
						corev1.Toleration{
							Key:      "example.com/gpu",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						}).
					Tolerations(constants.Node,
						corev1.Toleration{
							Key:      "example.com/gpu",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						}).
					Volumes(constants.DatasetInitializer,
						corev1.Volume{
							Name: "initializer_claim",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "initializer_claim_name",
								},
							},
						},
						corev1.Volume{
							Name: "initializer_secret",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "initializer_secret_name",
								},
							},
						},
					).
					VolumeMounts(constants.DatasetInitializer, constants.DatasetInitializer,
						corev1.VolumeMount{
							Name:      "initializer_secret",
							MountPath: "initializer_secret_mount_path",
						},
						corev1.VolumeMount{
							Name:      "initializer_claim",
							MountPath: "initializer_claim_mount_path",
						},
					).
					Volumes(constants.Node,
						corev1.Volume{
							Name: "node_claim",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "node_claim_name",
								},
							},
						},
						corev1.Volume{
							Name: "node_secret",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "node_secret_name",
								},
							},
						},
					).
					VolumeMounts(constants.Node, constants.Node,
						corev1.VolumeMount{
							Name:      "node_secret",
							MountPath: "node_secret_mount_path",
						},
						corev1.VolumeMount{
							Name:      "node_claim",
							MountPath: "node_claim_mount_path",
						},
					).
					Obj(),
			},
		},
		"succeeded to build JobSet with nil affinity overrides from the TrainJob's PodTemplateOverrides.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					InitContainer(constants.DatasetInitializer, "override-init-container", "test:runtime").
					InitContainer(constants.Node, "override-init-container", "test:runtime").
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.Node, "override-container", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						Manager:    ptr.To("manager-1"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.DatasetInitializer}},
						Spec: &trainer.PodTemplateSpecOverride{
							Affinity: nil,
						},
					},
					{
						Manager:    ptr.To("manager-2"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							Affinity: nil,
						},
					},
				}).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(100).
					InitContainer(constants.DatasetInitializer, "override-init-container", "test:runtime").
					InitContainer(constants.Node, "override-init-container", "test:runtime").
					Container(constants.Node, constants.Node, "test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Container(constants.Node, "override-container", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			},
		},
		"succeeded to build JobSet with securityContext overrides from the TrainJob's PodTemplateOverrides.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.Node, "sidecar", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup:      ptr.To(int64(2000)),
								RunAsUser:    ptr.To(int64(1000)),
								RunAsGroup:   ptr.To(int64(3000)),
								RunAsNonRoot: ptr.To(true),
							},
							Containers: []trainer.ContainerOverride{
								{
									Name: "sidecar",
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
										ReadOnlyRootFilesystem:   ptr.To(true),
									},
								},
							},
						},
					},
				}).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(100).
					Container(constants.Node, constants.Node, "test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Container(constants.Node, "sidecar", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					PodSecurityContext(constants.Node, corev1.PodSecurityContext{
						FSGroup:      ptr.To(int64(2000)),
						RunAsUser:    ptr.To(int64(1000)),
						RunAsGroup:   ptr.To(int64(3000)),
						RunAsNonRoot: ptr.To(true),
					}).
					ContainerSecurityContext(constants.Node, "sidecar", corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						ReadOnlyRootFilesystem:   ptr.To(true),
					}).
					Obj(),
			},
		},
		"succeeded to build JobSet with labels and annotations overrides from the TrainJob's PodTemplateOverrides.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					InitContainer(constants.DatasetInitializer, "override-init-container", "test:runtime").
					InitContainer(constants.Node, "override-init-container", "test:runtime").
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.Node, "override-container", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						Manager:    ptr.To("manager-1"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.DatasetInitializer}},
						Metadata: &metav1.ObjectMeta{
							Labels:      map[string]string{"k1": "v1"},
							Annotations: map[string]string{"a1": "v1"},
						},
					},
					{
						Manager:    ptr.To("manager-2"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Metadata: &metav1.ObjectMeta{
							Labels:      map[string]string{"k2": "v2"},
							Annotations: map[string]string{"a2": "v2"},
						},
					},
				}).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(100).
					InitContainer(constants.DatasetInitializer, "override-init-container", "test:runtime").
					InitContainer(constants.Node, "override-init-container", "test:runtime").
					Container(constants.Node, constants.Node, "test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Container(constants.Node, "override-container", "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					PodLabelForJobs("k1", "v1", constants.DatasetInitializer).
					PodAnnotationForJobs("a1", "v1", constants.DatasetInitializer).
					PodLabelForJobs("k2", "v2", constants.Node).
					PodAnnotationForJobs("a2", "v2", constants.Node).
					Obj(),
			},
		},
		"succeeded to build JobSet with TrainJob's PodTemplateOverrides containing duplicate TargetJobs": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						Manager:    ptr.To("manager-1"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							NodeSelector: map[string]string{
								"node.kubernetes.io/instance-type": "p5.48xlarge",
							},
						},
					},
					{
						Manager:    ptr.To("manager-1"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							ServiceAccountName: ptr.To("test-sa"),
						},
					},
				}).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					NodeSelector(constants.Node,
						map[string]string{
							"node.kubernetes.io/instance-type": "p5.48xlarge",
						}).
					ServiceAccountName(constants.Node, "test-sa").
					Obj(),
			},
		},
		"succeeded to build JobSet with TrainJob's PodTemplateOverrides targeting the same job with different values": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				PodTemplateOverrides([]trainer.PodTemplateOverride{
					{
						Manager:    ptr.To("manager-1"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							NodeSelector: map[string]string{
								"node.kubernetes.io/instance-type": "p5.48xlarge",
							},
						},
					},
					{
						Manager:    ptr.To("manager-1"),
						TargetJobs: []trainer.PodTemplateOverrideTargetJob{{Name: constants.Node}},
						Spec: &trainer.PodTemplateSpecOverride{
							NodeSelector: map[string]string{
								"node.kubernetes.io/instance-type": "p5en.48xlarge",
							},
						},
					},
				}).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					NodeSelector(constants.Node,
						map[string]string{
							"node.kubernetes.io/instance-type": "p5en.48xlarge",
						}).
					Obj(),
			},
		},
		"succeeded to build JobSet with dataset and model initializer from the TrainJob.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					Container(constants.DatasetInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.ModelInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Initializer(
					testingutil.MakeTrainJobInitializerWrapper().
						DatasetInitializer(
							testingutil.MakeTrainJobDatasetInitializerWrapper().
								StorageUri("hf://trainjob-dataset").
								Env(
									[]corev1.EnvVar{
										{
											Name:  "TRAIN_JOB",
											Value: "test:trainjob:dataset",
										},
									}...,
								).
								SecretRef(corev1.LocalObjectReference{Name: "trainjob-secret-dataset"}).
								Obj(),
						).
						ModelInitializer(
							testingutil.MakeTrainJobModelInitializerWrapper().
								StorageUri("hf://trainjob-model").
								Env(
									[]corev1.EnvVar{
										{
											Name:  "TRAIN_JOB",
											Value: "test:trainjob:model",
										},
									}...,
								).
								SecretRef(corev1.LocalObjectReference{Name: "trainjob-secret-model"}).
								Obj(),
						).
						Obj(),
				).
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(100).
					Container(constants.DatasetInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.ModelInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Env(constants.DatasetInitializer, constants.DatasetInitializer,
						[]corev1.EnvVar{
							{
								Name:  jobsetplgconsts.InitializerEnvStorageUri,
								Value: "hf://trainjob-dataset",
							},
							{
								Name:  "TRAIN_JOB",
								Value: "test:trainjob:dataset",
							},
						}...,
					).
					EnvFrom(constants.DatasetInitializer, constants.DatasetInitializer,
						[]corev1.EnvFromSource{
							{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "trainjob-secret-dataset",
									},
								},
							},
						}...,
					).
					Env(constants.ModelInitializer, constants.ModelInitializer,
						[]corev1.EnvVar{
							{
								Name:  jobsetplgconsts.InitializerEnvStorageUri,
								Value: "hf://trainjob-model",
							},
							{
								Name:  "TRAIN_JOB",
								Value: "test:trainjob:model",
							},
						}...,
					).
					EnvFrom(constants.ModelInitializer, constants.ModelInitializer,
						[]corev1.EnvFromSource{
							{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "trainjob-secret-model",
									},
								},
							},
						}...,
					).
					Obj(),
			},
		},
		// Test cases for the Torch MLPolicy.
		"succeeded to build JobSet with Torch values from the TrainJob": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
								TorchPolicy(ptr.To(intstr.FromString("auto"))).
								Obj(),
							).
							Obj(),
					).
					JobSetSpec(
						testingutil.MakeJobSetWrapper("", "").
							DependsOn(constants.Node,
								[]jobsetv1alpha2.DependsOn{
									{
										Name:   constants.DatasetInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
									{
										Name:   constants.ModelInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
								}...,
							).
							Obj().
							Spec,
					).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						NumNodes(30).
						NumProcPerNode(intstr.FromInt32(3)).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(30).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "30",
							},
							{
								Name:  constants.TorchEnvNumProcPerNode,
								Value: "3",
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
								Value: fmt.Sprintf("test-job-%s-0-0.test-job", constants.Node),
							},
							{
								Name:  constants.TorchEnvMasterPort,
								Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
							},
						}...,
					).
					DependsOn(constants.Node,
						[]jobsetv1alpha2.DependsOn{
							{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
							{
								Name:   constants.ModelInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
						}...,
					).
					Obj(),
			},
		},
		"succeeded to build JobSet with Torch values from the Runtime and envs.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
								TorchPolicy(ptr.To(intstr.FromString("auto"))).
								Obj(),
							).
							Obj(),
					).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						}...,
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Env(
							[]corev1.EnvVar{
								{
									Name:  "TRAIN_JOB",
									Value: "override",
								},
								{
									Name:  "TRAIN_JOB_CUSTOM",
									Value: "test:trainjob",
								},
							}...,
						).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(100).
					Container(constants.Node, constants.Node, "test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "override",
							},
							{
								Name:  "TRAIN_JOB_CUSTOM",
								Value: "test:trainjob",
							},
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "100",
							},
							{
								Name:  constants.TorchEnvNumProcPerNode,
								Value: "1",
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
								Value: fmt.Sprintf("test-job-%s-0-0.test-job", constants.Node),
							},
							{
								Name:  constants.TorchEnvMasterPort,
								Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						}...,
					).
					Obj(),
			},
		},
		"succeeded to build JobSet with TorchTune values from the TrainJob": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.3-70b").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.3-70b").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
								TorchPolicy(ptr.To(intstr.FromString("auto"))).
								Obj(),
							).
							Obj(),
					).
					JobSetSpec(
						testingutil.MakeJobSetWrapper("", "").
							DependsOn(constants.Node,
								[]jobsetv1alpha2.DependsOn{
									{
										Name:   constants.DatasetInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
									{
										Name:   constants.ModelInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
								}...,
							).
							Obj().
							Spec,
					).
					Container(
						constants.Node,
						constants.Node,
						"test:runtime",
						[]string{
							"tune",
							"run",
							constants.TorchTuneFullFinetuneDistributed,
							"--config",
							"llama3_3/70B_full_multinode",
							"output_dir=/workspace/model/llama3_3/70B",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{"runtime"},
						resRequests,
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "torchtune-llama3.3-70b").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container(
							"test:trainjob",
							[]string{"tune", "run"},
							[]string{
								"dtype=fp16",
								"batch_size=32",
								"epochs=10",
								"loss=torchtune.modules.loss.CEWithChunkedOutputLoss",
							},
							resRequests).
						NumNodes(30).
						NumProcPerNode(intstr.FromInt32(3)).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(30).
					Container(
						constants.Node,
						constants.Node,
						"test:trainjob",
						[]string{
							"tune",
							"run",
							fmt.Sprintf("%s=%s", constants.TorchTuneArgRdzvEndpoint, "test-job-node-0-0.test-job:29500"),
							constants.TorchTuneFullFinetuneDistributed,
							"--config",
							"llama3_3/70B_full_multinode",
							"output_dir=/workspace/model/llama3_3/70B",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{
							"dtype=fp16",
							"batch_size=32",
							"epochs=10",
							"loss=torchtune.modules.loss.CEWithChunkedOutputLoss",
						},
						resRequests,
					).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "30",
							},
							{
								Name:  constants.TorchEnvNumProcPerNode,
								Value: "3",
							},
							{
								Name: constants.TorchEnvNodeRank,
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: constants.JobCompletionIndexFieldPath,
									},
								},
							},
						}...,
					).
					DependsOn(constants.Node,
						[]jobsetv1alpha2.DependsOn{
							{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
							{
								Name:   constants.ModelInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
						}...,
					).
					Obj(),
			},
		},
		"succeeded to build Jobset with TorchTune values from the TrainJob (LoRA)": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.2-1b").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.2-1b").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(1).
							WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
								TorchPolicy(ptr.To(intstr.FromString("auto"))).
								Obj(),
							).
							Obj(),
					).
					JobSetSpec(
						testingutil.MakeJobSetWrapper("", "").
							DependsOn(constants.Node,
								[]jobsetv1alpha2.DependsOn{
									{
										Name:   constants.DatasetInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
									{
										Name:   constants.ModelInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
								}...,
							).
							Obj().
							Spec,
					).
					Container(
						constants.Node,
						constants.Node,
						"test:runtime",
						[]string{
							"tune",
							"run",
							"--rdzv_endpoint=localhost:29500",
							constants.TorchTuneFullFinetuneDistributed,
							"--config",
							"llama3_2/1B_full",
							"dataset=torchtune.datasets.instruct_dataset",
							"dataset.source=parquet",
							"dataset.data_dir=/workspace/dataset/data",
							"output_dir=/workspace/output",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{"runtime"},
						resRequests,
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "torchtune-llama3.2-1b").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container(
							"test:trainjob",
							[]string{"tune", "run"},
							[]string{
								"dataset.data_dir=/workspace/dataset/data",
								"model.apply_lora_to_mlp=True",
								"model.lora_attn_modules=[q_proj,k_proj,v_proj,output_proj]",
								"dataset=torchtune.datasets.instruct_dataset",
								"dataset.source=parquet",
							},
							corev1.ResourceList{"example.com/gpu": resource.MustParse("2")},
						).
						NumNodes(1).
						NumProcPerNode(intstr.FromString("auto")).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(1).
					Container(
						constants.Node,
						constants.Node,
						"test:trainjob",
						[]string{
							"tune",
							"run",
							fmt.Sprintf("%s=%s", constants.TorchTuneArgRdzvEndpoint, "test-job-node-0-0.test-job:29500"),
							constants.TorchTuneLoRAFinetuneDistributed,
							"--config",
							"llama3_2/1B_lora",
							"output_dir=/workspace/output",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{
							"dataset.data_dir=/workspace/dataset/data",
							"model.apply_lora_to_mlp=True",
							"model.lora_attn_modules=[q_proj,k_proj,v_proj,output_proj]",
							"dataset=torchtune.datasets.instruct_dataset",
							"dataset.source=parquet",
						},
						corev1.ResourceList{"example.com/gpu": resource.MustParse("2")},
					).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "1",
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
						}...,
					).
					DependsOn(constants.Node,
						[]jobsetv1alpha2.DependsOn{
							{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
							{
								Name:   constants.ModelInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
						}...,
					).
					Obj(),
			},
		},
		"succeeded to build Jobset with TorchTune values from the TrainJob (QLoRA)": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.2-1b").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.2-1b").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(1).
							WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
								TorchPolicy(ptr.To(intstr.FromString("auto"))).
								Obj(),
							).
							Obj(),
					).
					JobSetSpec(
						testingutil.MakeJobSetWrapper("", "").
							DependsOn(constants.Node,
								[]jobsetv1alpha2.DependsOn{
									{
										Name:   constants.DatasetInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
									{
										Name:   constants.ModelInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
								}...,
							).
							Obj().
							Spec,
					).
					Container(
						constants.Node,
						constants.Node,
						"test:runtime",
						[]string{
							"tune",
							"run",
							"--rdzv_endpoint=localhost:29500",
							constants.TorchTuneFullFinetuneDistributed,
							"--config",
							"llama3_2/1B_full",
							"dataset=torchtune.datasets.instruct_dataset",
							"dataset.source=parquet",
							"dataset.data_dir=/workspace/dataset/data",
							"output_dir=/workspace/output",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{"runtime"},
						resRequests,
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "torchtune-llama3.2-1b").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container(
							"test:trainjob",
							[]string{"tune", "run"},
							[]string{
								"dataset.data_dir=/workspace/dataset/data",
								"model.apply_lora_to_mlp=True",
								"model.lora_attn_modules=[q_proj,k_proj,v_proj,output_proj]",
								"model.quantize_base=True",
								"dataset=torchtune.datasets.instruct_dataset",
								"dataset.source=parquet",
							},
							corev1.ResourceList{"example.com/gpu": resource.MustParse("1")},
						).
						NumNodes(1).
						NumProcPerNode(intstr.FromString("auto")).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(1).
					Container(
						constants.Node,
						constants.Node,
						"test:trainjob",
						[]string{
							"tune",
							"run",
							constants.TorchTuneLoRAFinetuneSingleDevice,
							"--config",
							"llama3_2/1B_qlora_single_device",
							"output_dir=/workspace/output",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{
							"dataset.data_dir=/workspace/dataset/data",
							"model.apply_lora_to_mlp=True",
							"model.lora_attn_modules=[q_proj,k_proj,v_proj,output_proj]",
							"model.quantize_base=True",
							"dataset=torchtune.datasets.instruct_dataset",
							"dataset.source=parquet",
						},
						corev1.ResourceList{"example.com/gpu": resource.MustParse("1")},
					).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "1",
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
						}...,
					).
					DependsOn(constants.Node,
						[]jobsetv1alpha2.DependsOn{
							{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
							{
								Name:   constants.ModelInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
						}...,
					).
					Obj(),
			},
		},
		"succeeded to build Jobset with TorchTune values from the TrainJob (DoRA)": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.2-1b").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "torchtune-llama3.2-1b").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(1).
							WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
								TorchPolicy(ptr.To(intstr.FromString("auto"))).
								Obj(),
							).
							Obj(),
					).
					JobSetSpec(
						testingutil.MakeJobSetWrapper("", "").
							DependsOn(constants.Node,
								[]jobsetv1alpha2.DependsOn{
									{
										Name:   constants.DatasetInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
									{
										Name:   constants.ModelInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
								}...,
							).
							Obj().
							Spec,
					).
					Container(
						constants.Node,
						constants.Node,
						"test:runtime",
						[]string{
							"tune",
							"run",
							"--rdzv_endpoint=localhost:29500",
							constants.TorchTuneFullFinetuneDistributed,
							"--config",
							"llama3_2/1B_full",
							"dataset=torchtune.datasets.instruct_dataset",
							"dataset.source=parquet",
							"dataset.data_dir=/workspace/dataset/data",
							"output_dir=/workspace/output",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{"runtime"},
						resRequests,
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "torchtune-llama3.2-1b").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container(
							"test:trainjob",
							[]string{"tune", "run"},
							[]string{
								"dataset.data_dir=/workspace/dataset/data",
								"model.apply_lora_to_mlp=True",
								"model.lora_attn_modules=[q_proj,k_proj,v_proj,output_proj]",
								"model.quantize_base=True",
								"model.use_dora=True",
								"dataset=torchtune.datasets.instruct_dataset",
								"dataset.source=parquet",
							},
							corev1.ResourceList{"example.com/gpu": resource.MustParse("2")},
						).
						NumNodes(1).
						NumProcPerNode(intstr.FromString("auto")).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer).
					NumNodes(1).
					Container(
						constants.Node,
						constants.Node,
						"test:trainjob",
						[]string{
							"tune",
							"run",
							fmt.Sprintf("%s=%s", constants.TorchTuneArgRdzvEndpoint, "test-job-node-0-0.test-job:29500"),
							constants.TorchTuneLoRAFinetuneDistributed,
							"--config",
							"llama3_2/1B_lora",
							"output_dir=/workspace/output",
							"tokenizer.path=/workspace/model/original/tokenizer.model",
							"checkpointer.checkpoint_dir=/workspace/model",
						},
						[]string{
							"dataset.data_dir=/workspace/dataset/data",
							"model.apply_lora_to_mlp=True",
							"model.lora_attn_modules=[q_proj,k_proj,v_proj,output_proj]",
							"model.quantize_base=True",
							"model.use_dora=True",
							"dataset=torchtune.datasets.instruct_dataset",
							"dataset.source=parquet",
						},
						corev1.ResourceList{"example.com/gpu": resource.MustParse("2")},
					).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					Env(constants.Node, constants.Node,
						[]corev1.EnvVar{
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "1",
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
						}...,
					).
					DependsOn(constants.Node,
						[]jobsetv1alpha2.DependsOn{
							{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
							{
								Name:   constants.ModelInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
						}...,
					).
					Obj(),
			},
		},
		"succeeded to build JobSet with OpenMPI values from the TrainJob": {
			ObjCmpOpts: cmp.Options{
				cmp.Comparer(testingutil.MPISecretDataComparer),
			},
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").
				RuntimeSpec(
					testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
						WithMLPolicy(
							testingutil.MakeMLPolicyWrapper().
								WithNumNodes(1).
								WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
									MPIPolicy(ptr.To[int32](1), trainer.MPIImplementationOpenMPI, ptr.To("/root/.ssh"), ptr.To(false)).
									Obj(),
								).
								Obj(),
						).
						LauncherReplica().
						Replicas(1, constants.Launcher).
						Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Obj(),
				).
				Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						NumProcPerNode(intstr.FromInt32(8)).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeConfigMapWrapper(fmt.Sprintf("test-job%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					WithData(map[string]string{
						constants.MPIHostfileName: `test-job-node-0-0.test-job slots=8
test-job-node-0-1.test-job slots=8
`,
					}).
					Obj(),
				testingutil.MakeSecretWrapper(fmt.Sprintf("test-job%s", constants.MPISSHAuthSecretSuffix), metav1.NamespaceDefault).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					WithImmutable(true).
					WithData(map[string][]byte{
						corev1.SSHAuthPrivateKey:  []byte("EXIST"),
						constants.MPISSHPublicKey: []byte("EXIST"),
					}).
					WithType(corev1.SecretTypeSSHAuth).
					Obj(),
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					LauncherReplica().
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
					NumNodes(2).
					Volumes(constants.Launcher,
						corev1.Volume{
							Name: constants.MPISSHAuthVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("test-job%s", constants.MPISSHAuthSecretSuffix),
									Items: []corev1.KeyToPath{
										{
											Key:  corev1.SSHAuthPrivateKey,
											Path: constants.MPISSHPrivateKeyFile,
										},
										{
											Key:  constants.MPISSHPublicKey,
											Path: constants.MPISSHPublicKeyFile,
										},
										{
											Key:  constants.MPISSHPublicKey,
											Path: constants.MPISSHAuthorizedKeys,
										},
									},
								},
							},
						},
						corev1.Volume{
							Name: constants.MPIHostfileVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: fmt.Sprintf("test-job%s", constants.MPIHostfileConfigMapSuffix),
									},
									Items: []corev1.KeyToPath{{
										Key:  constants.MPIHostfileName,
										Path: constants.MPIHostfileName,
										Mode: ptr.To[int32](0444),
									}},
								},
							},
						},
					).
					Volumes(constants.Node,
						corev1.Volume{
							Name: constants.MPISSHAuthVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("test-job%s", constants.MPISSHAuthSecretSuffix),
									Items: []corev1.KeyToPath{
										{
											Key:  corev1.SSHAuthPrivateKey,
											Path: constants.MPISSHPrivateKeyFile,
										},
										{
											Key:  constants.MPISSHPublicKey,
											Path: constants.MPISSHPublicKeyFile,
										},
										{
											Key:  constants.MPISSHPublicKey,
											Path: constants.MPISSHAuthorizedKeys,
										},
									},
								},
							},
						},
					).
					VolumeMounts(constants.Launcher, constants.Node,
						corev1.VolumeMount{Name: constants.MPISSHAuthVolumeName, MountPath: "/root/.ssh"},
						corev1.VolumeMount{Name: constants.MPIHostfileVolumeName, MountPath: constants.MPIHostfileDir},
					).
					VolumeMounts(constants.Node, constants.Node,
						corev1.VolumeMount{Name: constants.MPISSHAuthVolumeName, MountPath: "/root/.ssh"},
					).
					Env(constants.Launcher, constants.Node,
						corev1.EnvVar{
							Name:  constants.OpenMPIEnvHostFileLocation,
							Value: fmt.Sprintf("%s/%s", constants.MPIHostfileDir, constants.MPIHostfileName),
						},
						corev1.EnvVar{
							Name:  constants.OpenMPIEnvKeepFQDNHostNames,
							Value: "true",
						},
						corev1.EnvVar{
							Name:  constants.OpenMPIEnvDefaultSlots,
							Value: "8",
						},
						corev1.EnvVar{
							Name:  constants.OpenMPIEnvKeyRSHArgs,
							Value: constants.OpenMPIEnvDefaultValueRSHArgs,
						},
					).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			},
		},
		// Failed test cases.
		"missing trainingRuntime resource": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job-3").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime-3").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				Obj(),
			wantError: errorNotFoundSpecifiedTrainingRuntime,
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortSlices(func(a, b runtime.Object) bool {
			return a.GetObjectKind().GroupVersionKind().String() < b.GetObjectKind().GroupVersionKind().String()
		}),
		cmpopts.SortSlices(func(a, b corev1.EnvVar) bool {
			return a.Name < b.Name
		}),
		cmpopts.SortSlices(func(a, b corev1.Volume) bool {
			return a.Name < b.Name
		}),
		cmpopts.SortSlices(func(a, b corev1.VolumeMount) bool {
			return a.Name < b.Name
		}),
		cmpopts.SortSlices(func(a, b corev1.Toleration) bool {
			return a.Key < b.Key
		}),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			clientBuilder := testingutil.NewClientBuilder()
			if tc.trainingRuntime != nil {
				clientBuilder.WithObjects(tc.trainingRuntime)
			}
			c := clientBuilder.Build()

			trainingRuntime, err := NewTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}

			objs, err := trainingRuntime.NewObjects(ctx, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}

			resultObjs, err := testingutil.ToObject(c.Scheme(), objs...)
			if err != nil {
				t.Errorf("Pipeline built unrecognizable objects: %v", err)
			}

			if diff := cmp.Diff(tc.wantObjs, resultObjs, append(cmpOpts, tc.ObjCmpOpts...)...); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}
