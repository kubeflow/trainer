/*
Copyright The Kubeflow Authors.

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
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	optimizationjob "github.com/kubeflow/trainer/v2/pkg/util/optimizationjob"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

type mockFailingClient struct {
	client.Client
	failPatchDeploy    bool
	failCreateTrainJob bool
}

func (m *mockFailingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if m.failCreateTrainJob {
		if _, ok := obj.(*trainer.TrainJob); ok {
			return fmt.Errorf("mock trainjob creation failed")
		}
	}
	return m.Client.Create(ctx, obj, opts...)
}

func (m *mockFailingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if m.failPatchDeploy {
		if _, ok := obj.(*appsv1.Deployment); ok {
			return fmt.Errorf("mock deployment creation failed")
		}
	}
	return m.Client.Patch(ctx, obj, patch, opts...)
}

type mockSearchAlgorithmClient struct {
	mockedAssignments [][]trainer.ParameterAssignment
	err               error

	lastAddr string
	lastReq  *katibapi.GetSuggestionsRequest
	calls    int
}

func (m *mockSearchAlgorithmClient) GetSuggestions(
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
			APIVersion: trainer.GroupVersion.String(),
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
			SearchAlgorithm: &trainer.SearchAlgorithm{
				Random: &trainer.RandomAlgorithm{},
			},
			Objectives: []trainer.Objective{
				{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
			},
			TrainJobTemplate: trainer.TrainJobTemplateSpec{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{},
				},
			},
		},
	}
}

func TestBuildSuggestionRequest_SearchSpaceMapping(t *testing.T) {
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

			req, err := optimizationjob.BuildSuggestionRequest(optJob, nil, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

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

func TestBuildSuggestionRequest_UnsupportedAlgorithm(t *testing.T) {
	optJob := getBaseOptJob()
	optJob.Spec.SearchAlgorithm = &trainer.SearchAlgorithm{
		Grid: &trainer.GridAlgorithm{},
	}

	_, err := optimizationjob.BuildSuggestionRequest(optJob, nil, 1)
	if err == nil {
		t.Fatal("expected error for unsupported algorithm 'grid', got nil")
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
			},
			Spec: trainer.TrainJobSpec{
				Trainer: &trainer.Trainer{
					Env: []corev1.EnvVar{
						{
							Name:  constants.EnvVarPrefix + "lr",
							Value: "0.03",
						},
					},
				},
			},
			Status: trainer.TrainJobStatus{
				Conditions: []metav1.Condition{
					{
						Type:   trainer.TrainJobComplete,
						Status: metav1.ConditionTrue,
					},
				},
				TrainerStatus: &trainer.TrainerStatus{
					Metrics: []trainer.Metric{
						{
							Name:  "accuracy",
							Value: "0.70",
						},
						{
							Name:  "accuracy",
							Value: "0.85", // final per-epoch value should be picked
						},
					},
				},
			},
		},
	}

	req, err := optimizationjob.BuildSuggestionRequest(
		optJob,
		trainJobs,
		1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

func TestReconcile_OptimizationJobReconciler(t *testing.T) {
	serviceName := optimizationjob.GetAlgorithmServiceName(getBaseOptJob())

	optDeploy := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: metav1.NamespaceDefault},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 1},
	}
	optSvc := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: metav1.NamespaceDefault},
	}

	cases := map[string]struct {
		getInitObjects        func() []client.Object
		searchAlgorithmClient SearchAlgorithmClient
		wantSuggestionCalls   int
		failPatchDeploy       bool
		failCreateTrainJob    bool
		wantRequeue           bool
		wantErr               bool
		getWantOptJob         func() *trainer.OptimizationJob
		wantTrainJobs         int
		wantDeployDeleted     bool
		wantSvcDeleted        bool
	}{
		"fail when search algorithm is unsupported": {
			getInitObjects: func() []client.Object {
				job := getBaseOptJob()
				job.Spec.SearchAlgorithm = &trainer.SearchAlgorithm{
					Grid: &trainer.GridAlgorithm{},
				}
				return []client.Object{job}
			},
			wantRequeue: false,
			wantErr:     false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Spec.SearchAlgorithm = &trainer.SearchAlgorithm{
					Grid: &trainer.GridAlgorithm{},
				}
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobFailed,
							Status:  metav1.ConditionTrue,
							Reason:  "UnsupportedSearchAlgorithm",
							Message: "Only 'random' search algorithm is supported in phase 1",
						},
					},
				}
				return job
			},
		},
		"deploy algorithm service and wait when not ready": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob()} },
			wantRequeue:    false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				return job
			},
		},
		"scale up new trials based on mocked suggestions": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob(), optDeploy, optSvc} },
			searchAlgorithmClient: &mockSearchAlgorithmClient{
				mockedAssignments: [][]trainer.ParameterAssignment{
					{{Name: "lr", Value: "0.03"}},
				},
			},
			wantRequeue: false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
					},
				}
				return job
			},
			wantTrainJobs:       1,
			wantSuggestionCalls: 1,
		},
		"mark optimizationjob complete when all trials finish": {
			getInitObjects: func() []client.Object {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
					},
				}
				return []client.Object{
					job,
					optDeploy,
					optSvc,
					&trainer.TrainJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "tj-1",
							Namespace: metav1.NamespaceDefault,
							Labels:    map[string]string{constants.OptimizationJobNameLabel: "test-optjob"},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: trainer.GroupVersion.String(),
									Kind:       "OptimizationJob",
									Name:       "test-optjob",
									UID:        "test-uid-123",
									Controller: ptr.To(true),
								},
							},
						},
						Spec: trainer.TrainJobSpec{
							Trainer: &trainer.Trainer{
								Env: []corev1.EnvVar{
									{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
								},
							},
						},
						Status: trainer.TrainJobStatus{
							Conditions:    []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}},
							TrainerStatus: &trainer.TrainerStatus{Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.80"}}},
						},
					},
					&trainer.TrainJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "tj-2",
							Namespace: metav1.NamespaceDefault,
							Labels:    map[string]string{constants.OptimizationJobNameLabel: "test-optjob"},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: trainer.GroupVersion.String(),
									Kind:       "OptimizationJob",
									Name:       "test-optjob",
									UID:        "test-uid-123",
									Controller: ptr.To(true),
								},
							},
						},
						Spec: trainer.TrainJobSpec{
							Trainer: &trainer.Trainer{
								Env: []corev1.EnvVar{
									{Name: constants.EnvVarPrefix + "lr", Value: "0.05"},
								},
							},
						},
						Status: trainer.TrainJobStatus{
							Conditions:    []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}},
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
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
						{
							Type:    constants.OptimizationJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  "OptimizationJobCompleted",
							Message: "All trials have completed successfully",
						},
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
		"a trial failed marks optimizationjob failed": {
			getInitObjects: func() []client.Object {
				job := getBaseOptJob()
				job.Spec.NumTrials = 2
				job.Spec.ParallelTrials = 2
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
					},
				}

				failedTrial1 := &trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tj-failed-1",
						Namespace: metav1.NamespaceDefault,
						Labels: map[string]string{
							constants.OptimizationJobNameLabel: "test-optjob",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: trainer.GroupVersion.String(),
								Kind:       "OptimizationJob",
								Name:       "test-optjob",
								UID:        "test-uid-123",
								Controller: ptr.To(true),
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{
								Type:   trainer.TrainJobFailed,
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
							constants.OptimizationJobNameLabel: "test-optjob",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: trainer.GroupVersion.String(),
								Kind:       "OptimizationJob",
								Name:       "test-optjob",
								UID:        "test-uid-123",
								Controller: ptr.To(true),
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{
								Type:   trainer.TrainJobFailed,
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
							Message: "Search algorithm service is running",
						},
						{
							Type:    constants.OptimizationJobFailed,
							Status:  metav1.ConditionTrue,
							Reason:  "TrialFailed",
							Message: "2 trial(s) failed",
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
				minJob.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
					},
				}

				return []client.Object{
					minJob,
					optDeploy,
					optSvc,
					&trainer.TrainJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "tj-1",
							Namespace: metav1.NamespaceDefault,
							Labels:    map[string]string{constants.OptimizationJobNameLabel: "test-optjob"},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: trainer.GroupVersion.String(),
									Kind:       "OptimizationJob",
									Name:       "test-optjob",
									UID:        "test-uid-123",
									Controller: ptr.To(true),
								},
							},
						},
						Spec: trainer.TrainJobSpec{
							Trainer: &trainer.Trainer{
								Env: []corev1.EnvVar{
									{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
								},
							},
						},
						Status: trainer.TrainJobStatus{
							Conditions:    []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}},
							TrainerStatus: &trainer.TrainerStatus{Metrics: []trainer.Metric{{Name: "loss", Value: "0.50"}}},
						},
					},
					&trainer.TrainJob{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "tj-2",
							Namespace: metav1.NamespaceDefault,
							Labels:    map[string]string{constants.OptimizationJobNameLabel: "test-optjob"},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: trainer.GroupVersion.String(),
									Kind:       "OptimizationJob",
									Name:       "test-optjob",
									UID:        "test-uid-123",
									Controller: ptr.To(true),
								},
							},
						},
						Spec: trainer.TrainJobSpec{
							Trainer: &trainer.Trainer{
								Env: []corev1.EnvVar{
									{Name: constants.EnvVarPrefix + "lr", Value: "0.05"},
								},
							},
						},
						Status: trainer.TrainJobStatus{
							Conditions:    []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}},
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
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
						{
							Type:    constants.OptimizationJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  "OptimizationJobCompleted",
							Message: "All trials have completed successfully",
						},
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
							Labels:    map[string]string{constants.OptimizationJobNameLabel: "test-optjob"},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: trainer.GroupVersion.String(),
									Kind:       "OptimizationJob",
									Name:       "test-optjob",
									UID:        "test-uid-123",
									Controller: ptr.To(true),
								},
							},
						},
						Status: trainer.TrainJobStatus{},
					},
				}
			},
			searchAlgorithmClient: &mockSearchAlgorithmClient{
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
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
					},
				}
				return job
			},
			wantTrainJobs:       1,
			wantSuggestionCalls: 0,
		},
		"fail reconciliation on suggestion service error": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob(), optDeploy, optSvc} },
			searchAlgorithmClient: &mockSearchAlgorithmClient{
				err: fmt.Errorf("connection refused"),
			},
			wantRequeue: false,
			wantErr:     true,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
					},
				}
				return job
			},
			wantTrainJobs:       0,
			wantSuggestionCalls: 1,
		},
		"fail on algorithm service deployment creation": {
			getInitObjects:  func() []client.Object { return []client.Object{getBaseOptJob()} },
			failPatchDeploy: true,
			wantRequeue:     false,
			wantErr:         true,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				return job
			},
		},
		"fail on trainjob creation": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob(), optDeploy, optSvc} },
			searchAlgorithmClient: &mockSearchAlgorithmClient{
				mockedAssignments: [][]trainer.ParameterAssignment{{{Name: "lr", Value: "0.03"}}},
			},
			failCreateTrainJob:  true,
			wantRequeue:         false,
			wantErr:             true,
			wantSuggestionCalls: 1,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  "AlgorithmServiceCreated",
							Message: "Search algorithm service is running",
						},
					},
				}
				return job
			},
		},
		"optimizationjob status is nil": {
			getInitObjects: func() []client.Object { return []client.Object{getBaseOptJob()} },
			wantRequeue:    false,
			wantErr:        false,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				return job
			},
		},
		"cleanup algorithm service when optimizationjob is completed": {
			getInitObjects: func() []client.Object {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  "OptimizationJobCompleted",
							Message: "All trials have completed successfully",
						},
					},
				}
				return []client.Object{job, optDeploy, optSvc}
			},
			wantRequeue:       false,
			wantErr:           false,
			wantDeployDeleted: true,
			wantSvcDeleted:    true,
			getWantOptJob: func() *trainer.OptimizationJob {
				job := getBaseOptJob()
				job.Status = &trainer.OptimizationJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    constants.OptimizationJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  "OptimizationJobCompleted",
							Message: "All trials have completed successfully",
						},
					},
				}
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

			builder := utiltesting.NewClientBuilder().
				WithScheme(testScheme).
				WithStatusSubresource(
					&trainer.OptimizationJob{},
					&trainer.TrainJob{},
					&appsv1.Deployment{},
				).
				WithObjects(initObjs...)

			if err := SetupIndexes(ctx, utiltesting.AsIndex(builder)); err != nil {
				t.Fatalf("Failed to setup indexes: %v", err)
			}
			baseCli := builder.Build()

			mockCli := &mockFailingClient{
				Client:             baseCli,
				failPatchDeploy:    tc.failPatchDeploy,
				failCreateTrainJob: tc.failCreateTrainJob,
			}

			algorithmClient := tc.searchAlgorithmClient
			if algorithmClient == nil {
				algorithmClient = &DefaultSearchAlgorithmClient{}
			}

			r := &OptimizationJobReconciler{
				Client:                mockCli,
				Scheme:                testScheme,
				Recorder:              record.NewFakeRecorder(100),
				SearchAlgorithmClient: algorithmClient,
			}

			runtimeKey := types.NamespacedName{
				Name:      "test-optjob",
				Namespace: metav1.NamespaceDefault,
			}
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: runtimeKey})
			if (err != nil) != tc.wantErr {
				t.Fatalf("Reconcile() error = %v, wantErr %v", err, tc.wantErr)
			}

			// Suggestion client assertions
			if tc.searchAlgorithmClient != nil {
				mockClient, ok := tc.searchAlgorithmClient.(*mockSearchAlgorithmClient)
				if !ok {
					t.Fatalf("expected *mockSearchAlgorithmClient, got %T", tc.searchAlgorithmClient)
				}

				if mockClient.calls != tc.wantSuggestionCalls {
					t.Fatalf(
						"expected SearchAlgorithmClient to be called %d times, got %d",
						tc.wantSuggestionCalls,
						mockClient.calls,
					)
				}

				if tc.wantSuggestionCalls > 0 {
					if mockClient.lastReq == nil {
						t.Fatal("expected SearchAlgorithmClient request to be captured")
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

			if err != nil {
				// Expected error case.
				return
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

			if tc.wantDeployDeleted {
				var deploy appsv1.Deployment
				getErr := baseCli.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: metav1.NamespaceDefault}, &deploy)
				if !apierrors.IsNotFound(getErr) {
					t.Errorf("expected Deployment to be deleted, got err: %v", getErr)
				}
			}

			if tc.wantSvcDeleted {
				var svc corev1.Service
				getErr := baseCli.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: metav1.NamespaceDefault}, &svc)
				if !apierrors.IsNotFound(getErr) {
					t.Errorf("expected Service to be deleted, got err: %v", getErr)
				}
			}
		})
	}
}

func TestGenerateTrialName(t *testing.T) {
	params1 := []trainer.ParameterAssignment{
		{Name: "lr", Value: "0.01"},
		{Name: "batch_size", Value: "32"},
	}
	// Different order of parameters should produce identical trial name
	params2 := []trainer.ParameterAssignment{
		{Name: "batch_size", Value: "32"},
		{Name: "lr", Value: "0.01"},
	}

	name1 := optimizationjob.GenerateTrialName("my-optjob", params1)
	name2 := optimizationjob.GenerateTrialName("my-optjob", params2)

	if name1 != name2 {
		t.Errorf("GenerateTrialName should be deterministic regardless of parameter order: got %q and %q", name1, name2)
	}

	params3 := []trainer.ParameterAssignment{
		{Name: "lr", Value: "0.02"},
		{Name: "batch_size", Value: "32"},
	}
	name3 := optimizationjob.GenerateTrialName("my-optjob", params3)
	if name1 == name3 {
		t.Errorf("GenerateTrialName should produce different names for different parameters: got %q", name1)
	}

	// Long OptimizationJob name should not exceed 63 characters
	longName := strings.Repeat("a", 60)
	trialName := optimizationjob.GenerateTrialName(longName, params1)
	if len(trialName) > 63 {
		t.Errorf("GenerateTrialName should not exceed 63 characters, got length %d: %q", len(trialName), trialName)
	}
}

func TestGetAlgorithmServiceName(t *testing.T) {
	// Standard name
	job := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{Name: "my-optjob"},
	}
	svcName := optimizationjob.GetAlgorithmServiceName(job)
	if svcName != "my-optjob-search-algorithm" {
		t.Errorf("expected my-optjob-search-algorithm, got %q", svcName)
	}

	// Very long name should be truncated so total length <= 63
	longJob := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("x", 60)},
	}
	longSvcName := optimizationjob.GetAlgorithmServiceName(longJob)
	if len(longSvcName) > 63 {
		t.Errorf("GetAlgorithmServiceName should not exceed 63 characters, got length %d: %q", len(longSvcName), longSvcName)
	}
}

func TestExtractBestResult(t *testing.T) {
	tests := []struct {
		name       string
		optJob     *trainer.OptimizationJob
		trainJobs  []trainer.TrainJob
		wantResult *trainer.Result
	}{
		{
			name: "no objectives returns nil",
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-1"},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.9"}},
						},
					},
				},
			},
			wantResult: nil,
		},
		{
			name: "maximize chooses highest metric value",
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-low"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.001"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.75"}},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-high"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
								{Name: constants.EnvVarPrefix + "epochs", Value: "10"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.95"}},
						},
					},
				},
			},
			wantResult: &trainer.Result{
				TrainJobName: "tj-high",
				Parameters: []trainer.ParameterAssignment{
					{Name: "epochs", Value: "10"},
					{Name: "lr", Value: "0.01"},
				},
			},
		},
		{
			name: "minimize chooses lowest metric value",
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "loss", Direction: trainer.ObjectiveDirectionMinimize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-high-loss"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.1"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "loss", Value: "1.23"}},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-low-loss"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "loss", Value: "0.12"}},
						},
					},
				},
			},
			wantResult: &trainer.Result{
				TrainJobName: "tj-low-loss",
				Parameters: []trainer.ParameterAssignment{
					{Name: "lr", Value: "0.01"},
				},
			},
		},
		{
			name: "metric value NaN is skipped",
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-nan"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.1"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "NaN"}},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-valid"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.85"}},
						},
					},
				},
			},
			wantResult: &trainer.Result{
				TrainJobName: "tj-valid",
				Parameters: []trainer.ParameterAssignment{
					{Name: "lr", Value: "0.01"},
				},
			},
		},
		{
			name: "only completed trials are considered (running and failed trials are skipped)",
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-running"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.99"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						// In-flight trial without Complete condition
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.99"}},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-failed"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.88"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.88"}},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-completed"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.05"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "0.75"}},
						},
					},
				},
			},
			wantResult: &trainer.Result{
				TrainJobName: "tj-completed",
				Parameters: []trainer.ParameterAssignment{
					{Name: "lr", Value: "0.05"},
				},
			},
		},
		{
			name: "per-epoch history picks final metric value",
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-1"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{
								{Name: "accuracy", Value: "0.99"}, // initial spike
								{Name: "accuracy", Value: "0.60"}, // final overfitted value
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-2"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{
								{Name: constants.EnvVarPrefix + "lr", Value: "0.02"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{
								{Name: "accuracy", Value: "0.50"},
								{Name: "accuracy", Value: "0.80"}, // final converged value
							},
						},
					},
				},
			},
			wantResult: &trainer.Result{
				TrainJobName: "tj-2",
				Parameters: []trainer.ParameterAssignment{
					{Name: "lr", Value: "0.02"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optimizationjob.ExtractBestResult(tt.optJob, tt.trainJobs)
			if diff := cmp.Diff(tt.wantResult, got); diff != "" {
				t.Errorf("ExtractBestResult() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConstructTrainJob(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = trainer.AddToScheme(scheme)

	r := &OptimizationJobReconciler{
		Scheme: scheme,
	}

	optJob := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob",
			Namespace: "default",
		},
		Spec: trainer.OptimizationJobSpec{
			TrainJobTemplate: trainer.TrainJobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"custom-label": "custom-value",
					},
					Annotations: map[string]string{
						"custom-annotation": "custom-annotation-value",
					},
				},
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{},
				},
			},
		},
	}

	params := []trainer.ParameterAssignment{
		{Name: "lr", Value: "0.01"},
	}

	tj, err := r.constructTrainJob(optJob, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLabels := map[string]string{
		"custom-label":                     "custom-value",
		constants.OptimizationJobNameLabel: "test-optjob",
	}
	if diff := cmp.Diff(expectedLabels, tj.Labels); diff != "" {
		t.Errorf("Labels mismatch (-want +got):\n%s", diff)
	}

	expectedAnnotations := map[string]string{
		"custom-annotation": "custom-annotation-value",
	}
	if diff := cmp.Diff(expectedAnnotations, tj.Annotations); diff != "" {
		t.Errorf("Annotations mismatch (-want +got):\n%s", diff)
	}

	// Test when TrainJobTemplate has nil Labels and Annotations
	optJobNoLabels := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob-nolabels",
			Namespace: "default",
		},
		Spec: trainer.OptimizationJobSpec{
			TrainJobTemplate: trainer.TrainJobTemplateSpec{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{},
				},
			},
		},
	}
	tjNoLabels, err := r.constructTrainJob(optJobNoLabels, params)
	if err != nil {
		t.Fatalf("unexpected error with nil labels: %v", err)
	}
	expectedOnlyOptLabel := map[string]string{
		constants.OptimizationJobNameLabel: "test-optjob-nolabels",
	}
	if diff := cmp.Diff(expectedOnlyOptLabel, tjNoLabels.Labels); diff != "" {
		t.Errorf("Labels mismatch for nil labels (-want +got):\n%s", diff)
	}
}

func TestConstructAlgorithmDeployment(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = trainer.AddToScheme(scheme)

	r := &OptimizationJobReconciler{
		Scheme: scheme,
	}

	optJob := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob",
			Namespace: "default",
			UID:       types.UID("12345"),
		},
	}

	deploy, err := r.constructAlgorithmDeployment(optJob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedName := "test-optjob-search-algorithm"
	if deploy.Name != expectedName {
		t.Errorf("expected deployment name %q, got %q", expectedName, deploy.Name)
	}
	if deploy.Namespace != "default" {
		t.Errorf("expected namespace default, got %q", deploy.Namespace)
	}

	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(deploy.Spec.Template.Spec.Containers))
	}
	c := deploy.Spec.Template.Spec.Containers[0]
	if c.Image != constants.DefaultSearchAlgorithmImage {
		t.Errorf("expected image %q, got %q", constants.DefaultSearchAlgorithmImage, c.Image)
	}
	if c.ReadinessProbe == nil || c.ReadinessProbe.GRPC == nil {
		t.Fatalf("expected GRPC readiness probe, got nil")
	}
	if c.ReadinessProbe.GRPC.Port != constants.SearchAlgorithmServicePort {
		t.Errorf("expected GRPC port %d, got %d", constants.SearchAlgorithmServicePort, c.ReadinessProbe.GRPC.Port)
	}
	if c.ReadinessProbe.GRPC.Service == nil || *c.ReadinessProbe.GRPC.Service != constants.SearchAlgorithmServiceName {
		t.Errorf("expected GRPC service %q, got %v", constants.SearchAlgorithmServiceName, c.ReadinessProbe.GRPC.Service)
	}

	controllerRef := metav1.GetControllerOf(deploy)
	if controllerRef == nil || controllerRef.Name != "test-optjob" {
		t.Errorf("expected controller reference to test-optjob, got %v", controllerRef)
	}
}

func TestConstructAlgorithmService(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = trainer.AddToScheme(scheme)

	r := &OptimizationJobReconciler{
		Scheme: scheme,
	}

	optJob := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob",
			Namespace: "default",
			UID:       types.UID("12345"),
		},
	}

	svc, err := r.constructAlgorithmService(optJob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedName := "test-optjob-search-algorithm"
	if svc.Name != expectedName {
		t.Errorf("expected service name %q, got %q", expectedName, svc.Name)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != constants.SearchAlgorithmServicePort {
		t.Errorf("expected service port %d, got %v", constants.SearchAlgorithmServicePort, svc.Spec.Ports)
	}

	controllerRef := metav1.GetControllerOf(svc)
	if controllerRef == nil || controllerRef.Name != "test-optjob" {
		t.Errorf("expected controller reference to test-optjob, got %v", controllerRef)
	}
}
