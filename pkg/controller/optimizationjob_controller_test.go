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
	"github.com/kubeflow/trainer/v2/pkg/constants"
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

	lastAddr string
	lastReq  *katibapi.GetSuggestionsRequest
	calls    int
}

func (m *mockSuggestionClient) GetSuggestions(
	ctx context.Context,
	addr string,
	req *katibapi.GetSuggestionsRequest,
) ([][]trainer.ParameterAssignment, error) {
	m.calls++
	m.lastAddr = addr
	m.lastReq = req

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
			UID:       "test-uid-123",
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

func TestBuildSuggestionRequest_SearchSpaceMapping(t *testing.T) {
	r := &OptimizationJobReconciler{}

	tests := []struct {
		name        string
		searchSpace trainer.SearchSpace
		wantType    katibapi.ParameterType
		wantMin     string
		wantMax     string
		wantList    []string
	}{
		{
			name: "uniform float",
			searchSpace: trainer.SearchSpace{
				Uniform: trainer.UniformSpace{
					Min:  "0.01",
					Max:  "0.10",
					Type: trainer.ParameterTypeFloat,
				},
			},
			wantType: katibapi.ParameterType_DOUBLE,
			wantMin:  "0.01",
			wantMax:  "0.10",
		},
		{
			name: "uniform int",
			searchSpace: trainer.SearchSpace{
				Uniform: trainer.UniformSpace{
					Min:  "1",
					Max:  "10",
					Type: trainer.ParameterTypeInt,
				},
			},
			wantType: katibapi.ParameterType_INT,
			wantMin:  "1",
			wantMax:  "10",
		},
		{
			name: "log uniform float",
			searchSpace: trainer.SearchSpace{
				LogUniform: trainer.LogUniformSpace{
					Min:  "0.001",
					Max:  "1.0",
					Type: trainer.ParameterTypeFloat,
				},
			},
			wantType: katibapi.ParameterType_DOUBLE,
			wantMin:  "0.001",
			wantMax:  "1.0",
		},
		{
			name: "log uniform int",
			searchSpace: trainer.SearchSpace{
				LogUniform: trainer.LogUniformSpace{
					Min:  "1",
					Max:  "100",
					Type: trainer.ParameterTypeInt,
				},
			},
			wantType: katibapi.ParameterType_INT,
			wantMin:  "1",
			wantMax:  "100",
		},
		{
			name: "categorical",
			searchSpace: trainer.SearchSpace{
				Categorical: trainer.CategoricalSpace{
					Choices: []string{"adam", "sgd", "rmsprop"},
				},
			},
			wantType: katibapi.ParameterType_CATEGORICAL,
			wantList: []string{"adam", "sgd", "rmsprop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optJob := getBaseOptJob()

			optJob.Spec.Parameters = []trainer.Parameter{
				{
					Name:        "test-param",
					SearchSpace: &tt.searchSpace,
				},
			}

			req := r.buildSuggestionRequest(optJob, nil, 1)

			if req == nil {
				t.Fatal("expected non-nil GetSuggestionsRequest")
			}

			if req.Experiment == nil || req.Experiment.Spec == nil {
				t.Fatal("expected Experiment and Experiment.Spec to be populated")
			}

			if req.Experiment.Spec.ParameterSpecs == nil {
				t.Fatal("expected ParameterSpecs to be populated")
			}

			params := req.Experiment.Spec.ParameterSpecs.Parameters

			if len(params) != 1 {
				t.Fatalf("expected 1 parameter, got %d", len(params))
			}

			param := params[0]

			if param.Name != "test-param" {
				t.Errorf(
					"expected parameter name %q, got %q",
					"test-param",
					param.Name,
				)
			}

			if param.ParameterType != tt.wantType {
				t.Errorf(
					"expected parameter type %v, got %v",
					tt.wantType,
					param.ParameterType,
				)
			}

			if param.FeasibleSpace == nil {
				t.Fatal("expected FeasibleSpace to be populated")
			}

			if tt.wantList != nil {
				if diff := cmp.Diff(tt.wantList, param.FeasibleSpace.List); diff != "" {
					t.Errorf("unexpected categorical choices (-want +got):\n%s", diff)
				}
				return
			}

			if param.FeasibleSpace.Min != tt.wantMin {
				t.Errorf(
					"expected min %q, got %q",
					tt.wantMin,
					param.FeasibleSpace.Min,
				)
			}

			if param.FeasibleSpace.Max != tt.wantMax {
				t.Errorf(
					"expected max %q, got %q",
					tt.wantMax,
					param.FeasibleSpace.Max,
				)
			}
		})
	}
}

func TestReconcile_OptimizationJobReconciler(t *testing.T) {
	optDeploy := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-optjob-optuna", Namespace: metav1.NamespaceDefault},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 1},
	}
	optSvc := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-optjob-optuna", Namespace: metav1.NamespaceDefault},
	}

	cases := map[string]struct {
		getInitObjects      func() []client.Object
		suggestionClient    SuggestionProvider
		wantSuggestionCalls int
		failCreateDeploy    bool
		failCreateTrainJob  bool
		wantRequeue         bool
		wantErr             bool
		getWantOptJob       func() *trainer.OptimizationJob
		wantTrainJobs       int
	}{
		"create optuna deployment and requeue": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob()} },
			wantRequeue:    true,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = nil
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
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{Type: "Created", Status: metav1.ConditionTrue, Reason: "AlgorithmServiceCreated", Message: "Optuna suggestion service is running"},
					},
				}
				return job
			},
			wantTrainJobs:       1,
			wantSuggestionCalls: 1,
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
							OwnerReferences: []metav1.OwnerReference{
								{UID: "test-uid-123"},
							},
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
							OwnerReferences: []metav1.OwnerReference{
								{UID: "test-uid-123"},
							},
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
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{Type: "Created", Status: metav1.ConditionTrue, Reason: "AlgorithmServiceCreated", Message: "Optuna suggestion service is running"},
						{Type: constants.OptimizationJobComplete, Status: metav1.ConditionTrue, Reason: "OptimizationJobCompleted", Message: "All trials have completed successfully"},
					},
					Result: trainer.Result{
						TrainJobName: "tj-2",
						Parameters:   []trainer.ParameterAssignment{{Name: "lr", Value: "0.05"}},
					},
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
							OwnerReferences: []metav1.OwnerReference{
								{UID: "test-uid-123"},
							},
						},
						Status: trainer.TrainJobStatus{},
					},
				}
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{Type: "Created", Status: metav1.ConditionTrue, Reason: "AlgorithmServiceCreated", Message: "Optuna suggestion service is running"},
						{Type: "Running", Status: metav1.ConditionTrue, Reason: "TrialsRunning", Message: "1 trial(s) actively running"},
					},
				}
				return job
			},
			wantTrainJobs: 1,
		},
		"all trials failed marks optimizationjob failed": {
			getInitObjects: func() []client.Object {
				job := getBaseOptJob()
				job.Spec.NumTrials = 2
				job.Spec.ParallelTrials = 2

				failedTrial1 := &trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tj-failed-1",
						Namespace: metav1.NamespaceDefault,
						Labels: map[string]string{
							"trainer.kubeflow.org/optimizationjob-name": "test-optjob",
						},
						OwnerReferences: []metav1.OwnerReference{
							{UID: "test-uid-123"},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{
								Type:   TrainJobFailed,
								Status: metav1.ConditionTrue,
								Reason: "JobFailed",
							},
						},
					},
				}

				failedTrial2 := &trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tj-failed-2",
						Namespace: metav1.NamespaceDefault,
						Labels: map[string]string{
							"trainer.kubeflow.org/optimizationjob-name": "test-optjob",
						},
						OwnerReferences: []metav1.OwnerReference{
							{UID: "test-uid-123"},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{
								Type:   TrainJobFailed,
								Status: metav1.ConditionTrue,
								Reason: "JobFailed",
							},
						},
					},
				}

				return []client.Object{
					job,
					optDeploy,
					optSvc,
					failedTrial1,
					failedTrial2,
				}
			},

			wantRequeue: false,

			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Spec.NumTrials = 2
				job.Spec.ParallelTrials = 2

				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Optuna suggestion service is running",
						},
						{
							Type:    constants.OptimizationJobFailed,
							Status:  metav1.ConditionTrue,
							Reason:  "AllTrialsFailed",
							Message: "2 out of 2 trials failed",
						},
					},
				}

				return job
			},

			wantTrainJobs: 2,
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
							OwnerReferences: []metav1.OwnerReference{
								{UID: "test-uid-123"},
							},
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
							OwnerReferences: []metav1.OwnerReference{
								{UID: "test-uid-123"},
							},
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
						{Type: constants.OptimizationJobComplete, Status: metav1.ConditionTrue, Reason: "OptimizationJobCompleted", Message: "All trials have completed successfully"},
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
							OwnerReferences: []metav1.OwnerReference{
								{UID: "test-uid-123"},
							},
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
			wantTrainJobs:       1,
			wantSuggestionCalls: 0,
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
							Type:    constants.OptimizationJobFailed,
							Status:  metav1.ConditionTrue,
							Reason:  "SuggestionServiceFailed",
							Message: "Optuna gRPC service error: connection refused",
						},
					},
				}
				return job
			},
			wantTrainJobs:       0,
			wantSuggestionCalls: 1,
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
							Reason: "AlgorithmServiceCreationFailed",
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

			if err != nil {
				// Expected error case.
				return
			}

			// Suggestion client assertions
			if tc.suggestionClient != nil {
				mockClient, ok := tc.suggestionClient.(*mockSuggestionClient)
				if !ok {
					t.Fatalf("expected *mockSuggestionClient, got %T", tc.suggestionClient)
				}

				if mockClient.calls != tc.wantSuggestionCalls {
					t.Fatalf(
						"expected SuggestionClient to be called %d times, got %d",
						tc.wantSuggestionCalls,
						mockClient.calls,
					)
				}

				if tc.wantSuggestionCalls > 0 {
					if mockClient.lastReq == nil {
						t.Fatal("expected SuggestionClient request to be captured")
					}

					if mockClient.lastReq.CurrentRequestNumber != 1 {
						t.Errorf(
							"expected CurrentRequestNumber=1, got %d",
							mockClient.lastReq.CurrentRequestNumber,
						)
					}
				}
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

func TestBuildSuggestionRequest_ReconstructsTrialHistory(t *testing.T) {
	optJob := getBaseOptJob()

	optJob.Spec.Parameters = []trainer.Parameter{
		{
			Name: "lr",
			SearchSpace: &trainer.SearchSpace{
				Uniform: trainer.UniformSpace{
					Min:  "0.01",
					Max:  "0.1",
					Type: trainer.ParameterTypeFloat,
				},
			},
		},
	}

	trainJobs := []trainer.TrainJob{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-optjob-trial-0",
				Annotations: map[string]string{
					"trainer.kubeflow.org/opt-param-lr": "0.03",
				},
			},
			Status: trainer.TrainJobStatus{
				TrainerStatus: &trainer.TrainerStatus{
					Metrics: []trainer.Metric{
						{
							Name:  "accuracy",
							Value: "0.85",
						},
					},
				},
			},
		},
	}

	r := &OptimizationJobReconciler{}

	req := r.buildSuggestionRequest(
		optJob,
		trainJobs,
		1,
	)

	if req.TotalRequestNumber != 1 {
		t.Fatalf(
			"expected TotalRequestNumber=1, got %d",
			req.TotalRequestNumber,
		)
	}

	if len(req.Trials) != 1 {
		t.Fatalf(
			"expected 1 trial, got %d",
			len(req.Trials),
		)
	}

	trial := req.Trials[0]

	if trial.Name != "test-optjob-trial-0" {
		t.Fatalf(
			"expected trial name test-optjob-trial-0, got %q",
			trial.Name,
		)
	}

	assignments :=
		trial.GetSpec().
			GetParameterAssignments().
			GetAssignments()

	if len(assignments) != 1 {
		t.Fatalf(
			"expected 1 parameter assignment, got %d",
			len(assignments),
		)
	}

	if assignments[0].Name != "lr" {
		t.Errorf(
			"expected parameter name lr, got %q",
			assignments[0].Name,
		)
	}

	if assignments[0].Value != "0.03" {
		t.Errorf(
			"expected parameter value 0.03, got %q",
			assignments[0].Value,
		)
	}

	if trial.Status == nil {
		t.Fatal("expected trial status")
	}

	observation := trial.Status.Observation

	if observation == nil {
		t.Fatal("expected observation")
	}

	if len(observation.Metrics) != 1 {
		t.Fatalf(
			"expected 1 metric, got %d",
			len(observation.Metrics),
		)
	}

	if observation.Metrics[0].Name != "accuracy" {
		t.Errorf(
			"expected metric accuracy, got %q",
			observation.Metrics[0].Name,
		)
	}

	if observation.Metrics[0].Value != "0.85" {
		t.Errorf(
			"expected metric value 0.85, got %q",
			observation.Metrics[0].Value,
		)
	}
}
