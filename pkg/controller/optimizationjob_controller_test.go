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

package controller

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

// mockSuggestionClient fakes the network call for unit tests
type mockSuggestionClient struct {
	mockedAssignments [][]trainer.ParameterAssignment
	err               error
}

func (m *mockSuggestionClient) GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error) {
	return m.mockedAssignments, m.err
}

func TestReconcile_OptimizationJobReconciler(t *testing.T) {
	baseOptJob := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: trainer.OptimizationJobSpec{
			NumTrials:      ptr.To(int32(2)),
			ParallelTrials: ptr.To(int32(1)),
			Objectives: []trainer.Objective{
				{Metric: ptr.To("accuracy"), Direction: ptr.To(trainer.ObjectiveDirectionMaximize)},
			},
		},
	}

	optDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-optjob-optuna", Namespace: metav1.NamespaceDefault},
	}
	optSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "test-optjob-optuna", Namespace: metav1.NamespaceDefault},
	}

	cases := map[string]struct {
		initObjects         []client.Object
		suggestionClient    SuggestionProvider
		wantRequeue         bool
		wantOptimizationJob *trainer.OptimizationJob
		wantTrainJobs       int // Check if a TrainJob was actually spawned
	}{
		"create optuna deployment and requeue": {
			initObjects:         []client.Object{baseOptJob.DeepCopy()},
			wantRequeue:         true,
			wantOptimizationJob: baseOptJob.DeepCopy(),
		},
		"scale up new trials based on mocked suggestions": {
			initObjects: []client.Object{baseOptJob.DeepCopy(), optDeploy, optSvc},
			// Provide the fake client to bypass grpc.Dial
			suggestionClient: &mockSuggestionClient{
				mockedAssignments: [][]trainer.ParameterAssignment{
					{{Name: "lr", Value: "0.03"}},
				},
			},
			wantRequeue:         false,
			wantOptimizationJob: baseOptJob.DeepCopy(),
			wantTrainJobs:       1,
		},
		"mark optimizationjob complete when all trials finish": {
			initObjects: []client.Object{
				baseOptJob.DeepCopy(),
				optDeploy,
				optSvc,
				&trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "tj-1",
						Namespace:   metav1.NamespaceDefault,
						Labels:      map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
						Annotations: map[string]string{"trainer.kubeflow.org/opt-param-lr": "0.01"},
					},
					Status: trainer.TrainJobStatus{
						Conditions:    []metav1.Condition{{Type: TrainJobComplete, Status: metav1.ConditionTrue}},
						TrainerStatus: &trainer.TrainerStatus{Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.80"}}},
					},
				},
				&trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "tj-2",
						Namespace:   metav1.NamespaceDefault,
						Labels:      map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
						Annotations: map[string]string{"trainer.kubeflow.org/opt-param-lr": "0.05"},
					},
					Status: trainer.TrainJobStatus{
						Conditions:    []metav1.Condition{{Type: TrainJobComplete, Status: metav1.ConditionTrue}},
						TrainerStatus: &trainer.TrainerStatus{Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.95"}}},
					},
				},
			},
			wantRequeue: false,
			wantOptimizationJob: func() *trainer.OptimizationJob {
				job := baseOptJob.DeepCopy()
				job.Status.Conditions = []metav1.Condition{
					{Type: OptimizationJobComplete, Status: metav1.ConditionTrue, Reason: "OptimizationJobCompleted", Message: "All trials have completed successfully"},
				}
				job.Status.Result = &trainer.Result{
					TrainJobName: "tj-2",
					Parameters:   []trainer.ParameterAssignment{{Name: "lr", Value: "0.05"}},
				}
				return job
			}(),
			wantTrainJobs: 2, // The 2 existing completed trials
		},
		"update optimizationjob status to running when trials are active": {
			initObjects: []client.Object{
				baseOptJob.DeepCopy(),
				optDeploy,
				optSvc,
				// Spawn a TrainJob that doesn't have a Complete/Failed condition yet (it's Active)
				&trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tj-active",
						Namespace: metav1.NamespaceDefault,
						Labels:    map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
					},
					Status: trainer.TrainJobStatus{},
				},
			},
			wantRequeue: false,
			wantOptimizationJob: func() *trainer.OptimizationJob {
				job := baseOptJob.DeepCopy()
				// Expect controller to append the "Running" condition
				job.Status.Conditions = []metav1.Condition{
					{
						Type:    "Running",
						Status:  metav1.ConditionTrue,
						Reason:  "TrialsRunning",
						Message: "1 trial(s) actively running",
					},
				}
				return job
			}(),
			wantTrainJobs: 1,
		},
		"minimize objective selects the lowest metric": {
			initObjects: func() []client.Object {
				minJob := baseOptJob.DeepCopy()
				minJob.Spec.Objectives[0].Metric = ptr.To("loss")
				minJob.Spec.Objectives[0].Direction = ptr.To(trainer.ObjectiveDirectionMinimize)

				return []client.Object{
					minJob,
					optDeploy,
					optSvc,
					&trainer.TrainJob{ // Trial 1: Loss 0.50
						ObjectMeta: metav1.ObjectMeta{
							Name: "tj-1", Namespace: metav1.NamespaceDefault,
							Labels:      map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
							Annotations: map[string]string{"trainer.kubeflow.org/opt-param-lr": "0.01"},
						},
						Status: trainer.TrainJobStatus{
							Conditions:    []metav1.Condition{{Type: TrainJobComplete, Status: metav1.ConditionTrue}},
							TrainerStatus: &trainer.TrainerStatus{Metrics: []trainer.Metric{{Name: "loss", Value: "0.50"}}},
						},
					},
					&trainer.TrainJob{ // Trial 2: Loss 0.10 (The Winner)
						ObjectMeta: metav1.ObjectMeta{
							Name: "tj-2", Namespace: metav1.NamespaceDefault,
							Labels:      map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
							Annotations: map[string]string{"trainer.kubeflow.org/opt-param-lr": "0.05"},
						},
						Status: trainer.TrainJobStatus{
							Conditions:    []metav1.Condition{{Type: TrainJobComplete, Status: metav1.ConditionTrue}},
							TrainerStatus: &trainer.TrainerStatus{Metrics: []trainer.Metric{{Name: "loss", Value: "0.10"}}},
						},
					},
				}
			}(),
			wantRequeue: false,
			wantOptimizationJob: func() *trainer.OptimizationJob {
				job := baseOptJob.DeepCopy()
				job.Spec.Objectives[0].Metric = ptr.To("loss")
				job.Spec.Objectives[0].Direction = ptr.To(trainer.ObjectiveDirectionMinimize)
				job.Status.Conditions = []metav1.Condition{
					{Type: OptimizationJobComplete, Status: metav1.ConditionTrue, Reason: "OptimizationJobCompleted", Message: "All trials have completed successfully"},
				}
				// Should pick tj-2 because 0.10 < 0.50
				job.Status.Result = &trainer.Result{
					TrainJobName: "tj-2",
					Parameters:   []trainer.ParameterAssignment{{Name: "lr", Value: "0.05"}},
				}
				return job
			}(),
			wantTrainJobs: 2,
		},
		"respect parallel trials budget": {
			initObjects: []client.Object{
				func() *trainer.OptimizationJob {
					job := baseOptJob.DeepCopy()
					job.Spec.NumTrials = ptr.To(int32(5))
					job.Spec.ParallelTrials = ptr.To(int32(1))
					return job
				}(),
				optDeploy,
				optSvc,
				// 1 Active Trial already exists
				&trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tj-running",
						Namespace: metav1.NamespaceDefault,
						Labels:    map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
					},
					Status: trainer.TrainJobStatus{},
				},
			},
			// We provide a mock client, but it should NEVER be called because activeTrials (1) == parallelLimit (1)
			suggestionClient: &mockSuggestionClient{
				mockedAssignments: [][]trainer.ParameterAssignment{{{Name: "lr", Value: "0.99"}}},
			},
			wantRequeue: false, // Just idles until the active trial completes
			wantOptimizationJob: func() *trainer.OptimizationJob {
				job := baseOptJob.DeepCopy()
				job.Spec.NumTrials = ptr.To(int32(5))
				job.Spec.ParallelTrials = ptr.To(int32(1))
				job.Status.Conditions = []metav1.Condition{
					{Type: "Running", Status: metav1.ConditionTrue, Reason: "TrialsRunning", Message: "1 trial(s) actively running"},
				}
				return job
			}(),
			wantTrainJobs: 1, // Still only 1 train job; controller did NOT spawn a second one
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			cli := utiltesting.NewClientBuilder().
				WithStatusSubresource(
					&trainer.OptimizationJob{},
					&trainer.TrainJob{},
				).
				WithObjects(tc.initObjects...).
				Build()

			r := &OptimizationJobReconciler{
				Client:           cli,
				Scheme:           cli.Scheme(),
				SuggestionClient: tc.suggestionClient, // Inject the mock
			}

			runtimeKey := client.ObjectKeyFromObject(baseOptJob)
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: runtimeKey})
			if err != nil {
				t.Fatalf("Reconcile() returned error: %v", err)
			}

			if res.Requeue != tc.wantRequeue {
				t.Errorf("Reconcile() returned Requeue=%v, want %v", res.Requeue, tc.wantRequeue)
			}

			var gotJob trainer.OptimizationJob
			if err := cli.Get(ctx, runtimeKey, &gotJob); err != nil {
				t.Fatalf("Get() returned error: %v", err)
			}

			if diff := cmp.Diff(tc.wantOptimizationJob, &gotJob,
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID"),
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime"),
			); len(diff) != 0 {
				t.Errorf("Unexpected OptimizationJob status (-want, +got): \n%s", diff)
			}

			// Verify TrainJob creation
			if tc.wantTrainJobs > 0 {
				var trainJobs trainer.TrainJobList
				if err := cli.List(ctx, &trainJobs, client.InNamespace(metav1.NamespaceDefault)); err != nil {
					t.Fatalf("Failed to list TrainJobs: %v", err)
				}
				if len(trainJobs.Items) != tc.wantTrainJobs {
					t.Errorf("Expected %d TrainJobs, got %d", tc.wantTrainJobs, len(trainJobs.Items))
				}
			}
		})
	}
}
