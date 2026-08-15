package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

type mockFailingClient struct {
	client.Client
	failCreateDeploy   bool
	failCreateTrainJob bool
}

func (m *mockFailingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if m.failCreateDeploy {
		if _, ok := obj.(*appsv1.Deployment); ok {
			return fmt.Errorf("mock deployment creation failed")
		}
	}
	if m.failCreateTrainJob {
		if _, ok := obj.(*trainer.TrainJob); ok {
			return fmt.Errorf("mock trainjob creation failed")
		}
	}
	return m.Client.Create(ctx, obj, opts...)
}

type mockSuggestionClient struct {
	mockedAssignments [][]trainer.ParameterAssignment
	err               error
}

func (m *mockSuggestionClient) GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error) {
	return m.mockedAssignments, m.err
}

func getBaseOptJob() *trainer.OptimizationJob {
	return &trainer.OptimizationJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "trainer.kubeflow.org/v1alpha1",
			Kind:       "OptimizationJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: trainer.OptimizationJobSpec{
			NumTrials:      2,
			ParallelTrials: 1,
			Objectives: []trainer.Objective{
				{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
			},
		},
	}
}

func TestReconcile_OptimizationJobReconciler(t *testing.T) {
	optDeploy := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-optjob-optuna", Namespace: metav1.NamespaceDefault},
	}
	optSvc := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-optjob-optuna", Namespace: metav1.NamespaceDefault},
	}

	cases := map[string]struct {
		getInitObjects     func() []client.Object
		suggestionClient   SuggestionProvider
		failCreateDeploy   bool
		failCreateTrainJob bool
		wantRequeue        bool
		wantErr            bool
		getWantOptJob      func() *trainer.OptimizationJob
		wantTrainJobs      int
	}{
		"create optuna deployment and requeue": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob()} },
			wantRequeue:    true,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status.Conditions = nil
				return job
			},
		},
		"scale up new trials based on mocked suggestions": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob(), optDeploy, optSvc} },
			suggestionClient: &mockSuggestionClient{
				mockedAssignments: [][]trainer.ParameterAssignment{
					{{Name: "lr", Value: "0.03"}},
				},
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status.Conditions = []metav1.Condition{
					{Type: "Created", Status: metav1.ConditionTrue, Reason: "AlgorithmServiceCreated", Message: "Optuna suggestion service is running"},
				}
				return job
			},
			wantTrainJobs: 1,
		},
		"mark optimizationjob complete when all trials finish": {
			getInitObjects: func() []client.Object {
				return []client.Object{
					getBaseOptJob(),
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
				}
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status.Conditions = []metav1.Condition{
					{Type: "Created", Status: metav1.ConditionTrue, Reason: "AlgorithmServiceCreated", Message: "Optuna suggestion service is running"},
					{Type: OptimizationJobComplete, Status: metav1.ConditionTrue, Reason: "OptimizationJobCompleted", Message: "All trials have completed successfully"},
				}
				job.Status.Result = trainer.Result{
					TrainJobName: "tj-2",
					Parameters:   []trainer.ParameterAssignment{{Name: "lr", Value: "0.05"}},
				}
				return job
			},
			wantTrainJobs: 2,
		},
		"update optimizationjob status to running when trials are active": {
			getInitObjects: func() []client.Object {
				return []client.Object{
					getBaseOptJob(),
					optDeploy,
					optSvc,
					&trainer.TrainJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "tj-active",
							Namespace: metav1.NamespaceDefault,
							Labels:    map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
						},
						Status: trainer.TrainJobStatus{},
					},
				}
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status.Conditions = []metav1.Condition{
					{Type: "Created", Status: metav1.ConditionTrue, Reason: "AlgorithmServiceCreated", Message: "Optuna suggestion service is running"},
					{Type: "Running", Status: metav1.ConditionTrue, Reason: "TrialsRunning", Message: "1 trial(s) actively running"},
				}
				return job
			},
			wantTrainJobs: 1,
		},
		"minimize objective selects the lowest metric": {
			getInitObjects: func() []client.Object {
				minJob := getBaseOptJob()
				minJob.Spec.Objectives = []trainer.Objective{
					{Metric: "loss", Direction: trainer.ObjectiveDirectionMinimize},
				}

				return []client.Object{
					minJob,
					optDeploy,
					optSvc,
					&trainer.TrainJob{
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
					&trainer.TrainJob{
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
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Spec.Objectives = []trainer.Objective{
					{Metric: "loss", Direction: trainer.ObjectiveDirectionMinimize},
				}
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    "Created",
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Optuna suggestion service is running",
						},
						{Type: OptimizationJobComplete, Status: metav1.ConditionTrue, Reason: "OptimizationJobCompleted", Message: "All trials have completed successfully"},
					},
				}
				job.Status.Result = trainer.Result{
					TrainJobName: "tj-2",
					Parameters:   []trainer.ParameterAssignment{{Name: "lr", Value: "0.05"}},
				}
				return job
			},
			wantTrainJobs: 2,
		},
		"respect parallel trials budget": {
			getInitObjects: func() []client.Object {
				job := getBaseOptJob()
				job.Spec.NumTrials = 5
				job.Spec.ParallelTrials = 1
				return []client.Object{
					job,
					optDeploy,
					optSvc,
					&trainer.TrainJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "tj-running",
							Namespace: metav1.NamespaceDefault,
							Labels:    map[string]string{"trainer.kubeflow.org/optimizationjob-name": "test-optjob"},
						},
						Status: trainer.TrainJobStatus{},
					},
				}
			},
			suggestionClient: &mockSuggestionClient{
				mockedAssignments: [][]trainer.ParameterAssignment{{{Name: "lr", Value: "0.99"}}},
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Spec.NumTrials = 5
				job.Spec.ParallelTrials = 1
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    "Created",
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Optuna suggestion service is running",
						},
						{
							Type:    "Running",
							Status:  metav1.ConditionTrue,
							Reason:  "TrialsRunning",
							Message: "1 trial(s) actively running",
						},
					},
				}
				return job
			},
			wantTrainJobs: 1,
		},
		"fail optimization job on grpc error": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob(), optDeploy, optSvc} },
			suggestionClient: &mockSuggestionClient{
				err: fmt.Errorf("connection refused"),
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    "Created",
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Optuna suggestion service is running",
						},
						{
							Type:    "Running",
							Status:  metav1.ConditionTrue,
							Reason:  "TrialsRunning",
							Message: "1 trial(s) actively running",
						},
					},
				}
				return job
			},
			wantTrainJobs: 0,
		},
		"fail on algorithm service deployment creation": {
			getInitObjects:   func() []client.Object { return []client.Object{getBaseOptJob()} },
			failCreateDeploy: true,
			wantRequeue:      false,
			wantErr:          true, // we expect an error here
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Created",
							Status: metav1.ConditionFalse,
							Reason: "AlgorithmServiceDeploymentCreationFailed",
						},
					},
				}
				return job
			},
		},
		"fail on trainjob creation": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob(), optDeploy, optSvc} },
			suggestionClient: &mockSuggestionClient{
				mockedAssignments: [][]trainer.ParameterAssignment{{{Name: "lr", Value: "0.03"}}},
			},
			failCreateTrainJob: true,
			wantRequeue:        false,
			wantErr:            true,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Created",
							Status: metav1.ConditionFalse,
							Reason: "TrainJobsCreationFailed",
						},
					},
				}
				return job
			},
		},
		"optimizationjob status is nil": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob()} },
			wantRequeue:    true,
			wantErr:        false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				return job
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			testScheme := runtime.NewScheme()
			_ = corev1.AddToScheme(testScheme)
			_ = appsv1.AddToScheme(testScheme)
			_ = trainer.AddToScheme(testScheme)

			initObjs := tc.getInitObjects()

			baseCli := utiltesting.NewClientBuilder().
				WithScheme(testScheme).
				WithStatusSubresource(
					&trainer.OptimizationJob{},
					&trainer.TrainJob{},
				).
				WithObjects(initObjs...).
				Build()

			mockCli := &mockFailingClient{
				Client:             baseCli,
				failCreateDeploy:   tc.failCreateDeploy,
				failCreateTrainJob: tc.failCreateTrainJob,
			}

			r := &OptimizationJobReconciler{
				Client:           mockCli,
				Scheme:           testScheme,
				SuggestionClient: tc.suggestionClient,
			}

			runtimeKey := types.NamespacedName{
				Name:      "test-optjob",
				Namespace: metav1.NamespaceDefault,
			}
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: runtimeKey})
			if (err != nil) != tc.wantErr {
				t.Fatalf("Reconcile() error = %v, wantErr %v", err, tc.wantErr)
			}

			if res.Requeue != tc.wantRequeue {
				t.Errorf("Reconcile() returned Requeue=%v, want %v", res.Requeue, tc.wantRequeue)
			}

			var gotJob trainer.OptimizationJob
			if err := baseCli.Get(ctx, runtimeKey, &gotJob); err != nil {
				t.Fatalf("Get() returned error: %v", err)
			}

			wantOptJob := tc.getWantOptJob()
			if diff := cmp.Diff(wantOptJob, &gotJob,
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID"),
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime", "Message"),
			); len(diff) != 0 {
				t.Errorf("Unexpected OptimizationJob status (-want, +got): \n%s", diff)
			}

			if tc.wantTrainJobs > 0 {
				var trainJobs trainer.TrainJobList
				if err := baseCli.List(ctx, &trainJobs, client.InNamespace(metav1.NamespaceDefault)); err != nil {
					t.Fatalf("Failed to list TrainJobs: %v", err)
				}
				if len(trainJobs.Items) != tc.wantTrainJobs {
					t.Errorf("Expected %d TrainJobs, got %d", tc.wantTrainJobs, len(trainJobs.Items))
				}
			}
		})
	}
}
