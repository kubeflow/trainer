/*
Copyright 2026 The Kubeflow Authors.

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

package volcano

import (
	"context"
	"testing"

	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime/indexer"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestPodGroupRuntimeClassHandlerScopesNamespacedTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		jobAName           string
		jobBName           string
		runtimeClass       string
		wantNamespacedName client.ObjectKey
	}{
		"does not enqueue a TrainJob from another namespace": {
			jobAName:           "job-a",
			jobBName:           "job-b",
			runtimeClass:       "class-a",
			wantNamespacedName: client.ObjectKey{Namespace: "ns-a", Name: "job-a"},
		},
		"distinguishes same-named TrainJobs across namespaces": {
			jobAName:           "job",
			jobBName:           "job",
			runtimeClass:       "class-b",
			wantNamespacedName: client.ObjectKey{Namespace: "ns-b", Name: "job"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			runtimeA := utiltesting.MakeTrainingRuntimeWrapper("ns-a", "runtime").Obj()
			runtimeA.Spec.Template.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.RuntimeClassName = new("class-a")
			runtimeB := utiltesting.MakeTrainingRuntimeWrapper("ns-b", "runtime").Obj()
			runtimeB.Spec.Template.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.RuntimeClassName = new("class-b")
			jobA := utiltesting.MakeTrainJobWrapper("ns-a", tc.jobAName).
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "runtime").
				Suspend(true).
				Obj()
			jobB := utiltesting.MakeTrainJobWrapper("ns-b", tc.jobBName).
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "runtime").
				Suspend(true).
				Obj()

			builder := utiltesting.NewClientBuilder().WithObjects(runtimeA, runtimeB, jobA, jobB)
			builder.WithIndex(&trainer.TrainingRuntime{}, indexer.TrainingRuntimeContainerRuntimeClassKey, indexer.IndexTrainingRuntimeContainerRuntimeClass)
			builder.WithIndex(&trainer.ClusterTrainingRuntime{}, indexer.ClusterTrainingRuntimeContainerRuntimeClassKey, indexer.IndexClusterTrainingRuntimeContainerRuntimeClass)
			builder.WithIndex(&trainer.TrainJob{}, indexer.TrainJobRuntimeRefKey, indexer.IndexTrainJobTrainingRuntime)
			builder.WithIndex(&trainer.TrainJob{}, indexer.TrainJobClusterRuntimeRefKey, indexer.IndexTrainJobClusterTrainingRuntime)
			handler := &PodGroupRuntimeClassHandler{client: builder.Build()}
			queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			t.Cleanup(queue.ShutDown)

			if err := handler.queueSuspendedTrainJobs(context.Background(), &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: tc.runtimeClass}}, queue); err != nil {
				t.Fatalf("queueSuspendedTrainJobs() error = %v", err)
			}
			if got := queue.Len(); got != 1 {
				t.Fatalf("queued requests = %d, want 1", got)
			}
			request, _ := queue.Get()
			if request.NamespacedName != tc.wantNamespacedName {
				t.Errorf("queued request = %v, want %v", request.NamespacedName, tc.wantNamespacedName)
			}
		})
	}
}
