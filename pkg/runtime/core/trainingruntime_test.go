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
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	jobsetplugin "github.com/kubeflow/trainer/pkg/runtime/framework/plugins/jobset"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestTrainingRuntimeNewObjects(t *testing.T) {
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	// TODO (andreyvelich): Add more test cases.
	cases := map[string]struct {
		trainingRuntime *trainer.TrainingRuntime
		trainJob        *trainer.TrainJob
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
						ContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						WithMLPolicy(
							testingutil.MakeMLPolicyWrapper().
								WithNumNodes(100).
								Obj(),
						).
						ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						PodGroupPolicyCoschedulingSchedulingTimeout(120).
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
					ContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					NumNodes(30).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Suspend(true).
					Label("conflictLabel", "override").
					Annotation("conflictAnnotation", "override").
					PodLabel(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Obj(),
				testingutil.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					MinMember(31). // 31 replicas = 30 Trainer nodes + 1 Initializer.
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
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerTrainerEnv(
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						},
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						ContainerEnv(
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
					NumNodes(100).
					ContainerTrainer("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					ContainerTrainerEnv(
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
						},
					).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Obj(),
			},
		},
		"succeeded to build JobSet with dataset and model initializer from the TrainJob.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					ContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				DatasetConfig(
					testingutil.MakeTrainJobDatasetConfigWrapper().
						StorageUri("hf://trainjob-dataset").
						ContainerEnv(
							[]corev1.EnvVar{
								{
									Name:  "TRAIN_JOB",
									Value: "test:trainjob:dataset",
								},
							},
						).
						SecretRef(corev1.LocalObjectReference{Name: "trainjob-secret-dataset"}).
						Obj(),
				).
				ModelConfig(
					testingutil.MakeTrainJobModelConfigWrapper().
						StorageUri("hf://trainjob-model").
						ContainerEnv(
							[]corev1.EnvVar{
								{
									Name:  "TRAIN_JOB",
									Value: "test:trainjob:model",
								},
							},
						).
						SecretRef(corev1.LocalObjectReference{Name: "trainjob-secret-model"}).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					NumNodes(100).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerDatasetInitializerEnv(
						[]corev1.EnvVar{
							{
								Name:  jobsetplugin.InitializerEnvStorageUri,
								Value: "hf://trainjob-dataset",
							},
							{
								Name:  "TRAIN_JOB",
								Value: "test:trainjob:dataset",
							},
						},
					).
					ContainerDatasetInitializerEnvFrom(
						[]corev1.EnvFromSource{
							{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "trainjob-secret-dataset",
									},
								},
							},
						},
					).
					ContainerModelInitializerEnv(
						[]corev1.EnvVar{
							{
								Name:  jobsetplugin.InitializerEnvStorageUri,
								Value: "hf://trainjob-model",
							},
							{
								Name:  "TRAIN_JOB",
								Value: "test:trainjob:model",
							},
						},
					).
					ContainerModelInitializerEnvFrom(
						[]corev1.EnvFromSource{
							{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "trainjob-secret-model",
									},
								},
							},
						},
					).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
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
							TorchPolicy("auto", nil).
							Obj(),
					).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
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
					NumNodes(30).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					ContainerTrainerEnv(
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
								Value: fmt.Sprintf("test-job-%s-0-0.test-job", constants.JobTrainerNode),
							},
							{
								Name:  constants.TorchEnvMasterPort,
								Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
							},
						},
					).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Obj(),
			},
		},
		"succeeded to build JobSet with Torch values from the Runtime and envs.": {
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							TorchPolicy("auto", nil).
							Obj(),
					).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerTrainerEnv(
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						},
					).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						ContainerEnv(
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
					NumNodes(100).
					ContainerTrainer("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					ContainerTrainerEnv(
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
								Value: fmt.Sprintf("test-job-%s-0-0.test-job", constants.JobTrainerNode),
							},
							{
								Name:  constants.TorchEnvMasterPort,
								Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						},
					).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
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
		cmpopts.EquateEmpty(),
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
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

			if diff := cmp.Diff(tc.wantObjs, resultObjs, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}
