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
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/util/trainjob"
)

func GetAlgorithmServiceName(optJob *trainer.OptimizationJob) string {
	const suffix = "-search-algorithm"
	maxPrefix := 63 - len(suffix)
	prefix := optJob.Name
	if len(prefix) > maxPrefix {
		prefix = strings.TrimRight(prefix[:maxPrefix], "-")
	}
	return fmt.Sprintf("%s%s", prefix, suffix)
}

func getAlgorithmName(optJob *trainer.OptimizationJob) string {
	if optJob.Spec.SearchAlgorithm != nil {
		if optJob.Spec.SearchAlgorithm.Random != nil {
			return "random"
		}
	}
	return ""
}

// GenerateTrialName gives each trial a random suffix, including trials with
// identical parameter assignments.
func GenerateTrialName(optJobName string) string {
	return generateTrialName(optJobName, utilrand.String(8))
}

func generateTrialName(optJobName, randomSuffix string) string {
	const separator = "-trial-"
	maxPrefix := 63 - len(separator) - len(randomSuffix)
	prefix := optJobName
	if len(prefix) > maxPrefix {
		prefix = strings.TrimRight(prefix[:maxPrefix], "-")
	}
	return prefix + separator + randomSuffix
}

var ErrObjectiveMetricMissing = errors.New("objective metric is missing")

// GetFinalObjectiveMetric returns the last reported objective metric and its finite numeric value.
func GetFinalObjectiveMetric(tj *trainer.TrainJob, metricName string) (*trainer.Metric, float64, error) {
	if tj.Status.TrainerStatus == nil {
		return nil, 0, fmt.Errorf("%w: %q", ErrObjectiveMetricMissing, metricName)
	}

	for i := len(tj.Status.TrainerStatus.Metrics) - 1; i >= 0; i-- {
		metric := &tj.Status.TrainerStatus.Metrics[i]
		if metric.Name != metricName {
			continue
		}
		value, err := strconv.ParseFloat(metric.Value, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, 0, fmt.Errorf("objective metric %q has invalid value %q", metricName, metric.Value)
		}
		return metric, value, nil
	}

	return nil, 0, fmt.Errorf("%w: %q", ErrObjectiveMetricMissing, metricName)
}

type candidateTrial struct {
	job          *trainer.TrainJob
	metricFloats []float64
	metricValues []trainer.ObjectiveMetricValue
}

func ExtractOptimalTrials(optJob *trainer.OptimizationJob, trainJobs []trainer.TrainJob) []trainer.OptimalTrial {
	if optJob == nil || len(optJob.Spec.Objectives) == 0 || len(trainJobs) == 0 {
		return nil
	}

	var candidates []candidateTrial
	for i := range trainJobs {
		tj := &trainJobs[i]
		valid := true
		floats := make([]float64, len(optJob.Spec.Objectives))
		vals := make([]trainer.ObjectiveMetricValue, len(optJob.Spec.Objectives))

		for j, obj := range optJob.Spec.Objectives {
			if obj.Metric == "" {
				valid = false
				break
			}
			metric, floatVal, err := GetFinalObjectiveMetric(tj, obj.Metric)
			if err != nil {
				valid = false
				break
			}
			floats[j] = floatVal
			vals[j] = trainer.ObjectiveMetricValue{
				Metric: obj.Metric,
				Value:  metric.Value,
			}
		}

		if valid {
			candidates = append(candidates, candidateTrial{
				job:          tj,
				metricFloats: floats,
				metricValues: vals,
			})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Single-objective optimization: return the single best trial.
	if len(optJob.Spec.Objectives) == 1 {
		bestIdx := 0
		dir := optJob.Spec.Objectives[0].Direction
		for i := 1; i < len(candidates); i++ {
			if dir == trainer.ObjectiveDirectionMaximize {
				if candidates[i].metricFloats[0] > candidates[bestIdx].metricFloats[0] {
					bestIdx = i
				}
			} else {
				if candidates[i].metricFloats[0] < candidates[bestIdx].metricFloats[0] {
					bestIdx = i
				}
			}
		}
		return []trainer.OptimalTrial{buildOptimalTrial(candidates[bestIdx])}
	}

	// Multi-objective optimization: Pareto front (non-dominated trials).
	var optimalTrials []trainer.OptimalTrial
	for i := range candidates {
		dominated := false
		for j := range candidates {
			if i == j {
				continue
			}
			if dominates(candidates[j], candidates[i], optJob.Spec.Objectives) {
				dominated = true
				break
			}
		}
		if !dominated {
			optimalTrials = append(optimalTrials, buildOptimalTrial(candidates[i]))
		}
	}

	// Sort deterministically by TrainJobName
	sort.Slice(optimalTrials, func(i, j int) bool {
		return optimalTrials[i].TrainJobName < optimalTrials[j].TrainJobName
	})

	return optimalTrials
}

func dominates(a, b candidateTrial, objectives []trainer.Objective) bool {
	strictlyBetter := false
	for k, obj := range objectives {
		valA := a.metricFloats[k]
		valB := b.metricFloats[k]

		if obj.Direction == trainer.ObjectiveDirectionMaximize {
			if valA < valB {
				return false
			}
			if valA > valB {
				strictlyBetter = true
			}
		} else { // Minimize
			if valA > valB {
				return false
			}
			if valA < valB {
				strictlyBetter = true
			}
		}
	}
	return strictlyBetter
}

func buildOptimalTrial(c candidateTrial) trainer.OptimalTrial {
	res := trainer.OptimalTrial{
		TrainJobName: c.job.Name,
		Metrics:      c.metricValues,
	}

	// Sort parameters alphabetically to avoid non-deterministic status updates
	paramMap := make(map[string]string)
	var paramNames []string

	if c.job.Spec.Trainer != nil {
		for _, env := range c.job.Spec.Trainer.Env {
			if strings.HasPrefix(env.Name, constants.EnvVarPrefix) {
				paramName := strings.TrimPrefix(env.Name, constants.EnvVarPrefix)
				paramNames = append(paramNames, paramName)
				paramMap[paramName] = env.Value
			}
		}
	}

	sort.Strings(paramNames)
	for _, name := range paramNames {
		res.Parameters = append(res.Parameters, trainer.ParameterAssignment{
			Name:  name,
			Value: paramMap[name],
		})
	}

	return res
}

func BuildSuggestionRequest(optJob *trainer.OptimizationJob, trainJobs []trainer.TrainJob, trialsToSpawn int32) (*katibapi.GetSuggestionsRequest, error) {
	algorithmName := getAlgorithmName(optJob)
	if algorithmName == "" {
		return nil, fmt.Errorf("unsupported or missing search algorithm: only 'random' is supported")
	}

	var targetMetric string
	var objectiveType katibapi.ObjectiveType
	var additionalMetrics []string
	if len(optJob.Spec.Objectives) > 0 && optJob.Spec.Objectives[0].Metric != "" {
		targetMetric = optJob.Spec.Objectives[0].Metric
		if optJob.Spec.Objectives[0].Direction == trainer.ObjectiveDirectionMaximize {
			objectiveType = katibapi.ObjectiveType_MAXIMIZE
		} else {
			objectiveType = katibapi.ObjectiveType_MINIMIZE
		}
		for _, obj := range optJob.Spec.Objectives[1:] {
			if obj.Metric != "" {
				additionalMetrics = append(additionalMetrics, obj.Metric)
			}
		}
	}

	var grpcParams []*katibapi.ParameterSpec
	for _, p := range optJob.Spec.Parameters {
		paramType := katibapi.ParameterType_DOUBLE
		var feasibleSpace *katibapi.FeasibleSpace

		if p.SearchSpace.Uniform.Min != "" && p.SearchSpace.Uniform.Max != "" {
			if p.SearchSpace.Uniform.Type == trainer.ParameterTypeInt {
				paramType = katibapi.ParameterType_INT
			}
			feasibleSpace = &katibapi.FeasibleSpace{
				Min:          string(p.SearchSpace.Uniform.Min),
				Max:          string(p.SearchSpace.Uniform.Max),
				Distribution: katibapi.Distribution_UNIFORM,
			}
		} else if p.SearchSpace.LogUniform.Min != "" && p.SearchSpace.LogUniform.Max != "" {
			if p.SearchSpace.LogUniform.Type == trainer.ParameterTypeInt {
				paramType = katibapi.ParameterType_INT
			}
			feasibleSpace = &katibapi.FeasibleSpace{
				Min:          string(p.SearchSpace.LogUniform.Min),
				Max:          string(p.SearchSpace.LogUniform.Max),
				Distribution: katibapi.Distribution_LOG_UNIFORM,
			}
		} else if len(p.SearchSpace.Categorical.Choices) > 0 {
			paramType = katibapi.ParameterType_CATEGORICAL
			feasibleSpace = &katibapi.FeasibleSpace{
				List: p.SearchSpace.Categorical.Choices,
			}
		}

		grpcParams = append(grpcParams, &katibapi.ParameterSpec{
			Name:          p.Name,
			ParameterType: paramType,
			FeasibleSpace: feasibleSpace,
		})
	}

	req := &katibapi.GetSuggestionsRequest{
		Experiment: &katibapi.Experiment{
			Name: optJob.Name,
			Spec: &katibapi.ExperimentSpec{
				Algorithm: &katibapi.AlgorithmSpec{
					AlgorithmName: algorithmName,
				},
				Objective: &katibapi.ObjectiveSpec{
					Type:                  objectiveType,
					ObjectiveMetricName:   targetMetric,
					AdditionalMetricNames: additionalMetrics,
				},
				ParameterSpecs: &katibapi.ExperimentSpec_ParameterSpecs{
					Parameters: grpcParams,
				},
			},
		},
		CurrentRequestNumber: trialsToSpawn,
		TotalRequestNumber:   int32(len(trainJobs)),
	}
	if optJob.Spec.SearchAlgorithm != nil &&
		optJob.Spec.SearchAlgorithm.Random != nil &&
		optJob.Spec.SearchAlgorithm.Random.Seed != nil {
		req.Experiment.Spec.Algorithm.AlgorithmSettings = []*katibapi.AlgorithmSetting{
			{
				Name:  "random_state",
				Value: strconv.FormatInt(*optJob.Spec.SearchAlgorithm.Random.Seed, 10),
			},
		}
	}

	for _, tj := range trainJobs {
		trial := &katibapi.Trial{
			Name: tj.Name,
			Spec: &katibapi.TrialSpec{
				Objective: &katibapi.ObjectiveSpec{
					Type:                  objectiveType,
					ObjectiveMetricName:   targetMetric,
					AdditionalMetricNames: additionalMetrics,
				},
				ParameterAssignments: &katibapi.TrialSpec_ParameterAssignments{
					Assignments: []*katibapi.ParameterAssignment{},
				},
			},
		}

		// Reconstruct ParameterAssignments from TrainJob EnvVars
		if tj.Spec.Trainer != nil {
			for _, env := range tj.Spec.Trainer.Env {
				if strings.HasPrefix(env.Name, constants.EnvVarPrefix) {
					paramName := strings.TrimPrefix(env.Name, constants.EnvVarPrefix)
					trial.Spec.ParameterAssignments.Assignments = append(
						trial.Spec.ParameterAssignments.Assignments,
						&katibapi.ParameterAssignment{
							Name:  paramName,
							Value: env.Value,
						},
					)
				}
			}
		}

		// Reconstruct Trial metrics or pass in-flight state
		if trainjob.IsTrainJobFinished(&tj) {
			var obsMetrics []*katibapi.Metric
			for _, obj := range optJob.Spec.Objectives {
				metric, _, err := GetFinalObjectiveMetric(&tj, obj.Metric)
				if err != nil {
					return nil, fmt.Errorf("completed trial %q: %w", tj.Name, err)
				}
				obsMetrics = append(obsMetrics, &katibapi.Metric{
					Name:  metric.Name,
					Value: metric.Value,
				})
			}
			trial.Status = &katibapi.TrialStatus{
				Condition: katibapi.TrialStatus_SUCCEEDED,
				Observation: &katibapi.Observation{
					Metrics: obsMetrics,
				},
			}
		} else {
			// Inform Optuna that this trial is running to avoid duplicate parameters
			trial.Status = &katibapi.TrialStatus{
				Condition: katibapi.TrialStatus_RUNNING,
			}
		}
		req.Trials = append(req.Trials, trial)
	}

	return req, nil
}
