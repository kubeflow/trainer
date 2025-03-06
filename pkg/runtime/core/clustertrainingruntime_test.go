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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestClusterTrainingRuntimeNewObjects(t *testing.T) {

	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	cases := map[string]struct {
		trainJob               *trainer.TrainJob
		clusterTrainingRuntime *trainer.ClusterTrainingRuntime
		wantObjs               []runtime.Object
		wantError              error
	}{
		"succeeded to build PodGroup and JobSet with NumNodes from the Runtime and container from the Trainer.": {
			clusterTrainingRuntime: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Spec).
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
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					NumNodes(100).
					ContainerTrainer("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Suspend(true).
					PodLabel(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Obj(),
				testingutil.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					MinMember(101). // 101 replicas = 100 Trainer nodes + 1 Initializers.
					MinResources(corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("102"), // Trainer node has 100 CPUs + 2 CPUs from 2 initializer containers.
					}).
					SchedulingTimeout(120).
					Obj(),
			},
		},
		"missing trainingRuntime resource": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				Obj(),
			wantError: errorNotFoundSpecifiedClusterTrainingRuntime,
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortSlices(func(a, b runtime.Object) bool {
			return a.GetObjectKind().GroupVersionKind().String() < b.GetObjectKind().GroupVersionKind().String()
		}),
		cmpopts.EquateEmpty(),
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testingutil.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil {
				clientBuilder.WithObjects(tc.clusterTrainingRuntime)
			}
			c := clientBuilder.Build()

			trainingRuntime, err := NewTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			var ok bool
			trainingRuntimeFactory, ok = trainingRuntime.(*TrainingRuntime)
			if !ok {
				t.Fatal("Failed type assertion from Runtime interface to TrainingRuntime")
			}

			clTrainingRuntime, err := NewClusterTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}

			objs, err := clTrainingRuntime.NewObjects(ctx, tc.trainJob)
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
