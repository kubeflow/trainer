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

package optimizationjob

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
)

func TestGetAlgorithmServiceName(t *testing.T) {
	cases := map[string]struct {
		jobName string
		want    string
	}{
		"standard short name": {
			jobName: "my-optjob",
			want:    "my-optjob-search-algorithm",
		},
		"name at max limit without truncation": {
			jobName: strings.Repeat("a", 46),
			want:    strings.Repeat("a", 46) + "-search-algorithm",
		},
		"long name gets truncated to 63 chars": {
			jobName: strings.Repeat("a", 60),
			want:    strings.Repeat("a", 46) + "-search-algorithm",
		},
		"long name with hyphen at truncation boundary gets trimmed": {
			// 45 'a's + '-' + 14 'b's = 60 chars.
			// Truncating to 46 gives 45 'a's + '-'. TrimRight removes the hyphen.
			jobName: strings.Repeat("a", 45) + "-" + strings.Repeat("b", 14),
			want:    strings.Repeat("a", 45) + "-search-algorithm",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			job := &trainer.OptimizationJob{
				ObjectMeta: metav1.ObjectMeta{Name: tc.jobName},
			}
			got := GetAlgorithmServiceName(job)
			if got != tc.want {
				t.Errorf("GetAlgorithmServiceName() = %q, want %q", got, tc.want)
			}
			if len(got) > 63 {
				t.Errorf("GetAlgorithmServiceName() length %d exceeds 63 characters: %q", len(got), got)
			}
		})
	}
}

func TestGetAlgorithmName(t *testing.T) {
	cases := map[string]struct {
		optJob *trainer.OptimizationJob
		want   string
	}{
		"nil SearchAlgorithm returns empty": {
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{},
			},
			want: "",
		},
		"nil Random in SearchAlgorithm returns empty": {
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{},
				},
			},
			want: "",
		},
		"random search algorithm returns random": {
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					SearchAlgorithm: &trainer.SearchAlgorithm{
						Random: &trainer.RandomAlgorithm{},
					},
				},
			},
			want: "random",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getAlgorithmName(tc.optJob)
			if got != tc.want {
				t.Errorf("getAlgorithmName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGenerateTrialName(t *testing.T) {
	t.Run("random suffixes distinguish trials", func(t *testing.T) {
		name1 := generateTrialName("my-optjob", "abc12345")
		name2 := generateTrialName("my-optjob", "def67890")
		if name1 == name2 {
			t.Fatalf("trial names must differ for different suffixes: %q", name1)
		}
		if name1 != "my-optjob-trial-abc12345" {
			t.Errorf("unexpected trial name %q", name1)
		}
	})

	t.Run("generated suffix has expected format", func(t *testing.T) {
		name := GenerateTrialName("my-optjob")
		if !strings.HasPrefix(name, "my-optjob-trial-") || len(name) != len("my-optjob-trial-")+8 {
			t.Errorf("unexpected generated trial name %q", name)
		}
	})

	t.Run("length limit of 63 characters is enforced", func(t *testing.T) {
		longName := strings.Repeat("x", 60)
		trialName := generateTrialName(longName, "abc12345")
		if len(trialName) > 63 {
			t.Errorf("GenerateTrialName length %d exceeds 63 characters: %q", len(trialName), trialName)
		}
		if !strings.Contains(trialName, "-trial-") {
			t.Errorf("GenerateTrialName does not contain '-trial-': %q", trialName)
		}
	})
}

func TestGetFinalObjectiveMetric(t *testing.T) {
	cases := map[string]struct {
		metrics         []trainer.Metric
		noTrainerStatus bool
		wantValue       float64
		wantMissing     bool
		wantErr         bool
	}{
		"no trainer status":     {noTrainerStatus: true, wantMissing: true, wantErr: true},
		"no metrics":            {wantMissing: true, wantErr: true},
		"unrelated metric":      {metrics: []trainer.Metric{{Name: "loss", Value: "0.1"}}, wantMissing: true, wantErr: true},
		"malformed value":       {metrics: []trainer.Metric{{Name: "accuracy", Value: "n/a"}}, wantErr: true},
		"NaN value":             {metrics: []trainer.Metric{{Name: "accuracy", Value: "NaN"}}, wantErr: true},
		"infinite value":        {metrics: []trainer.Metric{{Name: "accuracy", Value: "+Inf"}}, wantErr: true},
		"last matching value":   {metrics: []trainer.Metric{{Name: "accuracy", Value: "0.5"}, {Name: "loss", Value: "0.2"}, {Name: "accuracy", Value: "0.9"}}, wantValue: 0.9},
		"last matching invalid": {metrics: []trainer.Metric{{Name: "accuracy", Value: "0.5"}, {Name: "accuracy", Value: "NaN"}}, wantErr: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			job := &trainer.TrainJob{}
			if !tc.noTrainerStatus {
				job.Status.TrainerStatus = &trainer.TrainerStatus{Metrics: tc.metrics}
			}
			metric, value, err := GetFinalObjectiveMetric(job, "accuracy")
			if (err != nil) != tc.wantErr {
				t.Fatalf("GetFinalObjectiveMetric() error = %v, want error %v", err, tc.wantErr)
			}
			if errors.Is(err, ErrObjectiveMetricMissing) != tc.wantMissing {
				t.Errorf("GetFinalObjectiveMetric() missing error = %v, want %v", err, tc.wantMissing)
			}
			if !tc.wantErr && (metric == nil || metric.Value != "0.9" || value != tc.wantValue) {
				t.Errorf("GetFinalObjectiveMetric() = (%v, %v), want final metric value %v", metric, value, tc.wantValue)
			}
		})
	}
}

func TestExtractOptimalTrials(t *testing.T) {
	cases := map[string]struct {
		optJob    *trainer.OptimizationJob
		trainJobs []trainer.TrainJob
		want      []trainer.OptimalTrial
	}{
		"no objectives returns nil": {
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
			want: nil,
		},
		"maximize direction selects highest metric": {
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
			want: []trainer.OptimalTrial{
				{
					TrainJobName: "tj-high",
					Parameters: []trainer.ParameterAssignment{
						{Name: "epochs", Value: "10"},
						{Name: "lr", Value: "0.01"},
					},
					Metrics: []trainer.ObjectiveMetricValue{
						{Metric: "accuracy", Value: "0.95"},
					},
				},
			},
		},
		"minimize direction selects lowest metric": {
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
								{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
							},
						},
					},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "loss", Value: "1.25"}},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-low-loss"},
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
							Metrics: []trainer.Metric{{Name: "loss", Value: "0.25"}},
						},
					},
				},
			},
			want: []trainer.OptimalTrial{
				{
					TrainJobName: "tj-low-loss",
					Parameters: []trainer.ParameterAssignment{
						{Name: "lr", Value: "0.001"},
					},
					Metrics: []trainer.ObjectiveMetricValue{
						{Metric: "loss", Value: "0.25"},
					},
				},
			},
		},
		"multiple epochs logs picks final occurrence of metric": {
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-multi-epoch"},
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
								{Name: "accuracy", Value: "0.50"},
								{Name: "accuracy", Value: "0.75"},
								{Name: "accuracy", Value: "0.88"},
							},
						},
					},
				},
			},
			want: []trainer.OptimalTrial{
				{
					TrainJobName: "tj-multi-epoch",
					Parameters: []trainer.ParameterAssignment{
						{Name: "lr", Value: "0.01"},
					},
					Metrics: []trainer.ObjectiveMetricValue{
						{Metric: "accuracy", Value: "0.88"},
					},
				},
			},
		},
		"non-finite or unparseable metric values are ignored": {
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-invalid"},
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
					ObjectMeta: metav1.ObjectMeta{Name: "tj-infinite"},
					Status: trainer.TrainJobStatus{
						Conditions: []metav1.Condition{
							{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
						},
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{{Name: "accuracy", Value: "+Inf"}},
						},
					},
				},
			},
			want: nil,
		},
		"multi-objective returns pareto front (non-dominated trials)": {
			optJob: &trainer.OptimizationJob{
				Spec: trainer.OptimizationJobSpec{
					Objectives: []trainer.Objective{
						{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
						{Metric: "latency", Direction: trainer.ObjectiveDirectionMinimize},
					},
				},
			},
			trainJobs: []trainer.TrainJob{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-1"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{{Name: constants.EnvVarPrefix + "lr", Value: "0.01"}},
						},
					},
					Status: trainer.TrainJobStatus{
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{
								{Name: "accuracy", Value: "0.90"},
								{Name: "latency", Value: "100"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-2"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{{Name: constants.EnvVarPrefix + "lr", Value: "0.05"}},
						},
					},
					Status: trainer.TrainJobStatus{
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{
								{Name: "accuracy", Value: "0.95"},
								{Name: "latency", Value: "150"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-3"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{{Name: constants.EnvVarPrefix + "lr", Value: "0.02"}},
						},
					},
					Status: trainer.TrainJobStatus{
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{
								// Dominated by tj-1 (lower accuracy and higher latency)
								{Name: "accuracy", Value: "0.85"},
								{Name: "latency", Value: "120"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "tj-4-incomplete"},
					Spec: trainer.TrainJobSpec{
						Trainer: &trainer.Trainer{
							Env: []corev1.EnvVar{{Name: constants.EnvVarPrefix + "lr", Value: "0.03"}},
						},
					},
					Status: trainer.TrainJobStatus{
						TrainerStatus: &trainer.TrainerStatus{
							Metrics: []trainer.Metric{
								// Missing latency objective
								{Name: "accuracy", Value: "0.99"},
							},
						},
					},
				},
			},
			want: []trainer.OptimalTrial{
				{
					TrainJobName: "tj-1",
					Parameters: []trainer.ParameterAssignment{
						{Name: "lr", Value: "0.01"},
					},
					Metrics: []trainer.ObjectiveMetricValue{
						{Metric: "accuracy", Value: "0.90"},
						{Metric: "latency", Value: "100"},
					},
				},
				{
					TrainJobName: "tj-2",
					Parameters: []trainer.ParameterAssignment{
						{Name: "lr", Value: "0.05"},
					},
					Metrics: []trainer.ObjectiveMetricValue{
						{Metric: "accuracy", Value: "0.95"},
						{Metric: "latency", Value: "150"},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ExtractOptimalTrials(tc.optJob, tc.trainJobs)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ExtractOptimalTrials() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildSuggestionRequest(t *testing.T) {
	t.Run("rejects completed trial without a valid objective metric", func(t *testing.T) {
		optJob := &trainer.OptimizationJob{Spec: trainer.OptimizationJobSpec{
			SearchAlgorithm: &trainer.SearchAlgorithm{Random: &trainer.RandomAlgorithm{}},
			Objectives:      []trainer.Objective{{Metric: "accuracy"}},
		}}
		cases := map[string][]trainer.Metric{
			"no metrics": nil,
			"missing":    {{Name: "loss", Value: "0.1"}},
			"malformed":  {{Name: "accuracy", Value: "n/a"}},
			"NaN":        {{Name: "accuracy", Value: "NaN"}},
			"infinite":   {{Name: "accuracy", Value: "+Inf"}},
		}
		for name, metrics := range cases {
			t.Run(name, func(t *testing.T) {
				trial := trainer.TrainJob{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-trial"},
					Status: trainer.TrainJobStatus{
						Conditions:    []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}},
						TrainerStatus: &trainer.TrainerStatus{Metrics: metrics},
					},
				}
				if req, err := BuildSuggestionRequest(optJob, []trainer.TrainJob{trial}, 1); err == nil || req != nil || !strings.Contains(err.Error(), "completed-trial") {
					t.Errorf("BuildSuggestionRequest() = (%v, %v), want trial-specific error", req, err)
				}
			})
		}
	})

	t.Run("passes random seed to algorithm service", func(t *testing.T) {
		seed := int64(42)
		optJob := &trainer.OptimizationJob{
			ObjectMeta: metav1.ObjectMeta{Name: "seeded-optjob"},
			Spec: trainer.OptimizationJobSpec{
				SearchAlgorithm: &trainer.SearchAlgorithm{
					Random: &trainer.RandomAlgorithm{Seed: &seed},
				},
			},
		}

		req, err := BuildSuggestionRequest(optJob, nil, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		settings := req.Experiment.Spec.Algorithm.AlgorithmSettings
		if len(settings) != 1 {
			t.Fatalf("expected one algorithm setting, got %d", len(settings))
		}
		if settings[0].Name != "random_state" || settings[0].Value != "42" {
			t.Fatalf("unexpected random seed setting: %+v", settings[0])
		}
	})

	t.Run("unsupported or missing algorithm returns error", func(t *testing.T) {
		optJob := &trainer.OptimizationJob{
			ObjectMeta: metav1.ObjectMeta{Name: "optjob-no-algo"},
			Spec:       trainer.OptimizationJobSpec{},
		}
		_, err := BuildSuggestionRequest(optJob, nil, 1)
		if err == nil {
			t.Errorf("expected error for missing search algorithm, got nil")
		}
	})

	t.Run("search space mapping across all parameter types", func(t *testing.T) {
		optJob := &trainer.OptimizationJob{
			ObjectMeta: metav1.ObjectMeta{Name: "optjob-params"},
			Spec: trainer.OptimizationJobSpec{
				SearchAlgorithm: &trainer.SearchAlgorithm{
					Random: &trainer.RandomAlgorithm{},
				},
				Objectives: []trainer.Objective{
					{Metric: "val_loss", Direction: trainer.ObjectiveDirectionMinimize},
				},
				Parameters: []trainer.Parameter{
					{
						Name: "lr",
						SearchSpace: &trainer.SearchSpace{
							Uniform: trainer.UniformSpace{
								Min:  "0.001",
								Max:  "0.1",
								Type: trainer.ParameterTypeFloat,
							},
						},
					},
					{
						Name: "num_layers",
						SearchSpace: &trainer.SearchSpace{
							Uniform: trainer.UniformSpace{
								Min:  "1",
								Max:  "10",
								Type: trainer.ParameterTypeInt,
							},
						},
					},
					{
						Name: "weight_decay",
						SearchSpace: &trainer.SearchSpace{
							LogUniform: trainer.LogUniformSpace{
								Min:  "0.00001",
								Max:  "0.01",
								Type: trainer.ParameterTypeFloat,
							},
						},
					},
					{
						Name: "log_int_param",
						SearchSpace: &trainer.SearchSpace{
							LogUniform: trainer.LogUniformSpace{
								Min:  "1",
								Max:  "100",
								Type: trainer.ParameterTypeInt,
							},
						},
					},
					{
						Name: "optimizer",
						SearchSpace: &trainer.SearchSpace{
							Categorical: trainer.CategoricalSpace{
								Choices: []string{"adam", "sgd", "adamw"},
							},
						},
					},
				},
			},
		}

		req, err := BuildSuggestionRequest(optJob, nil, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.CurrentRequestNumber != 2 {
			t.Errorf("expected CurrentRequestNumber=2, got %d", req.CurrentRequestNumber)
		}
		if req.TotalRequestNumber != 0 {
			t.Errorf("expected TotalRequestNumber=0, got %d", req.TotalRequestNumber)
		}
		if req.Experiment.Spec.Algorithm.AlgorithmName != "random" {
			t.Errorf("expected algorithm 'random', got %q", req.Experiment.Spec.Algorithm.AlgorithmName)
		}
		if req.Experiment.Spec.Objective.Type != katibapi.ObjectiveType_MINIMIZE {
			t.Errorf("expected MINIMIZE objective, got %v", req.Experiment.Spec.Objective.Type)
		}
		if req.Experiment.Spec.Objective.ObjectiveMetricName != "val_loss" {
			t.Errorf("expected 'val_loss' metric, got %q", req.Experiment.Spec.Objective.ObjectiveMetricName)
		}

		params := req.Experiment.Spec.ParameterSpecs.Parameters
		if len(params) != 5 {
			t.Fatalf("expected 5 parameters, got %d", len(params))
		}

		// Verify Uniform Float
		if params[0].Name != "lr" || params[0].ParameterType != katibapi.ParameterType_DOUBLE ||
			params[0].FeasibleSpace.Distribution != katibapi.Distribution_UNIFORM {
			t.Errorf("unexpected parameter 0 (lr): %+v", params[0])
		}
		// Verify Uniform Int
		if params[1].Name != "num_layers" || params[1].ParameterType != katibapi.ParameterType_INT ||
			params[1].FeasibleSpace.Distribution != katibapi.Distribution_UNIFORM {
			t.Errorf("unexpected parameter 1 (num_layers): %+v", params[1])
		}
		// Verify LogUniform Float
		if params[2].Name != "weight_decay" || params[2].ParameterType != katibapi.ParameterType_DOUBLE ||
			params[2].FeasibleSpace.Distribution != katibapi.Distribution_LOG_UNIFORM {
			t.Errorf("unexpected parameter 2 (weight_decay): %+v", params[2])
		}
		// Verify LogUniform Int
		if params[3].Name != "log_int_param" || params[3].ParameterType != katibapi.ParameterType_INT ||
			params[3].FeasibleSpace.Distribution != katibapi.Distribution_LOG_UNIFORM {
			t.Errorf("unexpected parameter 3 (log_int_param): %+v", params[3])
		}
		// Verify Categorical
		if params[4].Name != "optimizer" || params[4].ParameterType != katibapi.ParameterType_CATEGORICAL {
			t.Errorf("unexpected parameter 4 (optimizer): %+v", params[4])
		}
		if diff := cmp.Diff([]string{"adam", "sgd", "adamw"}, params[4].FeasibleSpace.List); diff != "" {
			t.Errorf("categorical choices mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("trial history reconstruction for completed and in-flight trials", func(t *testing.T) {
		optJob := &trainer.OptimizationJob{
			ObjectMeta: metav1.ObjectMeta{Name: "optjob-history"},
			Spec: trainer.OptimizationJobSpec{
				SearchAlgorithm: &trainer.SearchAlgorithm{
					Random: &trainer.RandomAlgorithm{},
				},
				Objectives: []trainer.Objective{
					{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
				},
			},
		}

		trainJobs := []trainer.TrainJob{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "trial-completed"},
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						Env: []corev1.EnvVar{
							{Name: constants.EnvVarPrefix + "lr", Value: "0.01"},
							{Name: "OTHER_ENV", Value: "ignore_me"},
						},
					},
				},
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
					},
					TrainerStatus: &trainer.TrainerStatus{
						Metrics: []trainer.Metric{
							{Name: "other_metric", Value: "10"},
							{Name: "accuracy", Value: "0.92"},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "trial-running"},
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						Env: []corev1.EnvVar{
							{Name: constants.EnvVarPrefix + "lr", Value: "0.05"},
						},
					},
				},
				Status: trainer.TrainJobStatus{
					// No Complete condition -> in-flight
				},
			},
		}

		req, err := BuildSuggestionRequest(optJob, trainJobs, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.TotalRequestNumber != 2 {
			t.Errorf("expected TotalRequestNumber=2, got %d", req.TotalRequestNumber)
		}
		if len(req.Trials) != 2 {
			t.Fatalf("expected 2 trials, got %d", len(req.Trials))
		}

		// Trial 1: Completed
		t1 := req.Trials[0]
		if t1.Name != "trial-completed" {
			t.Errorf("expected trial name 'trial-completed', got %q", t1.Name)
		}
		if t1.Spec.GetObjective().GetObjectiveMetricName() != "accuracy" {
			t.Errorf("expected completed trial objective metric accuracy, got %q", t1.Spec.GetObjective().GetObjectiveMetricName())
		}
		if len(t1.Spec.ParameterAssignments.Assignments) != 1 ||
			t1.Spec.ParameterAssignments.Assignments[0].Name != "lr" ||
			t1.Spec.ParameterAssignments.Assignments[0].Value != "0.01" {
			t.Errorf("unexpected parameter assignments for trial 1: %+v", t1.Spec.ParameterAssignments.Assignments)
		}
		if t1.Status.Condition != katibapi.TrialStatus_SUCCEEDED {
			t.Errorf("expected SUCCEEDED status for trial 1, got %v", t1.Status.Condition)
		}
		if len(t1.Status.Observation.Metrics) != 1 ||
			t1.Status.Observation.Metrics[0].Name != "accuracy" ||
			t1.Status.Observation.Metrics[0].Value != "0.92" {
			t.Errorf("unexpected observation metric for trial 1: %+v", t1.Status.Observation.Metrics)
		}

		// Trial 2: Running
		t2 := req.Trials[1]
		if t2.Spec.GetObjective().GetObjectiveMetricName() != "accuracy" {
			t.Errorf("expected running trial objective metric accuracy, got %q", t2.Spec.GetObjective().GetObjectiveMetricName())
		}
		if t2.Name != "trial-running" {
			t.Errorf("expected trial name 'trial-running', got %q", t2.Name)
		}
		if t2.Status.Condition != katibapi.TrialStatus_RUNNING {
			t.Errorf("expected RUNNING status for trial 2, got %v", t2.Status.Condition)
		}
		if len(t2.Spec.ParameterAssignments.Assignments) != 1 ||
			t2.Spec.ParameterAssignments.Assignments[0].Name != "lr" ||
			t2.Spec.ParameterAssignments.Assignments[0].Value != "0.05" {
			t.Errorf("unexpected parameter assignments for trial 2: %+v", t2.Spec.ParameterAssignments.Assignments)
		}
	})

	t.Run("multi-objective suggestion request populates additional metric names and observations", func(t *testing.T) {
		optJob := &trainer.OptimizationJob{
			ObjectMeta: metav1.ObjectMeta{Name: "optjob-multi"},
			Spec: trainer.OptimizationJobSpec{
				SearchAlgorithm: &trainer.SearchAlgorithm{
					Random: &trainer.RandomAlgorithm{},
				},
				Objectives: []trainer.Objective{
					{Metric: "accuracy", Direction: trainer.ObjectiveDirectionMaximize},
					{Metric: "latency", Direction: trainer.ObjectiveDirectionMinimize},
				},
			},
		}

		trainJobs := []trainer.TrainJob{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "trial-1"},
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue},
					},
					TrainerStatus: &trainer.TrainerStatus{
						Metrics: []trainer.Metric{
							{Name: "accuracy", Value: "0.95"},
							{Name: "latency", Value: "120"},
						},
					},
				},
			},
		}

		req, err := BuildSuggestionRequest(optJob, trainJobs, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.Experiment.Spec.Objective.ObjectiveMetricName != "accuracy" {
			t.Errorf("expected primary metric 'accuracy', got %q", req.Experiment.Spec.Objective.ObjectiveMetricName)
		}
		if diff := cmp.Diff([]string{"latency"}, req.Experiment.Spec.Objective.AdditionalMetricNames); diff != "" {
			t.Errorf("additional metrics mismatch (-want +got):\n%s", diff)
		}

		if len(req.Trials) != 1 {
			t.Fatalf("expected 1 trial, got %d", len(req.Trials))
		}
		t1 := req.Trials[0]
		if t1.Spec.Objective.ObjectiveMetricName != "accuracy" {
			t.Errorf("expected trial primary metric 'accuracy', got %q", t1.Spec.Objective.ObjectiveMetricName)
		}
		if diff := cmp.Diff([]string{"latency"}, t1.Spec.Objective.AdditionalMetricNames); diff != "" {
			t.Errorf("trial additional metrics mismatch (-want +got):\n%s", diff)
		}
		if len(t1.Status.Observation.Metrics) != 2 {
			t.Fatalf("expected 2 observed metrics, got %d", len(t1.Status.Observation.Metrics))
		}
		if t1.Status.Observation.Metrics[0].Name != "accuracy" || t1.Status.Observation.Metrics[0].Value != "0.95" {
			t.Errorf("unexpected metric 0: %+v", t1.Status.Observation.Metrics[0])
		}
		if t1.Status.Observation.Metrics[1].Name != "latency" || t1.Status.Observation.Metrics[1].Value != "120" {
			t.Errorf("unexpected metric 1: %+v", t1.Status.Observation.Metrics[1])
		}
	})
}

