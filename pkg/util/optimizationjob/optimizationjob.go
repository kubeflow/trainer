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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
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

func GenerateTrialName(optJobName string, params []trainer.ParameterAssignment) string {
	// Ensure deterministic order for the hash
	var paramStrings []string
	for _, p := range params {
		paramStrings = append(paramStrings, fmt.Sprintf("%s=%s", p.Name, p.Value))
	}
	sort.Strings(paramStrings)

	hashInput := optJobName + "-" + strings.Join(paramStrings, ",")
	hash := sha256.Sum256([]byte(hashInput))
	shortHash := hex.EncodeToString(hash[:])[:8]

	maxPrefix := 63 - len("-trial-") - len(shortHash)
	prefix := optJobName
	if len(prefix) > maxPrefix {
		prefix = strings.TrimRight(prefix[:maxPrefix], "-")
	}

	return fmt.Sprintf("%s-trial-%s", prefix, shortHash)
}

func ExtractBestResult(optJob *trainer.OptimizationJob, trainJobs []trainer.TrainJob) *trainer.Result {
	if len(optJob.Spec.Objectives) == 0 || optJob.Spec.Objectives[0].Metric == "" {
		return nil
	}

	targetMetric := optJob.Spec.Objectives[0].Metric
	isMaximize := optJob.Spec.Objectives[0].Direction == trainer.ObjectiveDirectionMaximize

	var bestJob *trainer.TrainJob
	var bestVal float64

	for i, tj := range trainJobs {
		// Only consider completed trials (do not consider running or failed trials)
		if !meta.IsStatusConditionTrue(tj.Status.Conditions, trainer.TrainJobComplete) {
			continue
		}
		if tj.Status.TrainerStatus == nil {
			continue
		}

		// Use the final (last reported) occurrence of targetMetric
		var lastMetric *trainer.Metric
		for j := len(tj.Status.TrainerStatus.Metrics) - 1; j >= 0; j-- {
			if tj.Status.TrainerStatus.Metrics[j].Name == targetMetric {
				lastMetric = &tj.Status.TrainerStatus.Metrics[j]
				break
			}
		}
		if lastMetric == nil {
			continue
		}

		val, err := strconv.ParseFloat(lastMetric.Value, 64)
		if err != nil || math.IsNaN(val) {
			continue
		}

		if bestJob == nil {
			bestJob = &trainJobs[i]
			bestVal = val
		} else if isMaximize && val > bestVal {
			bestJob = &trainJobs[i]
			bestVal = val
		} else if !isMaximize && val < bestVal {
			bestJob = &trainJobs[i]
			bestVal = val
		}
	}

	if bestJob == nil {
		return nil
	}

	res := &trainer.Result{
		TrainJobName: bestJob.Name,
	}

	// Sort parameters alphabetically to avoid non-deterministic status updates
	paramMap := make(map[string]string)
	var paramNames []string

	if bestJob.Spec.Trainer != nil {
		for _, env := range bestJob.Spec.Trainer.Env {
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
	if len(optJob.Spec.Objectives) > 0 && optJob.Spec.Objectives[0].Metric != "" {
		targetMetric = optJob.Spec.Objectives[0].Metric
		if optJob.Spec.Objectives[0].Direction == trainer.ObjectiveDirectionMaximize {
			objectiveType = katibapi.ObjectiveType_MAXIMIZE
		} else {
			objectiveType = katibapi.ObjectiveType_MINIMIZE
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
					Type:                objectiveType,
					ObjectiveMetricName: targetMetric,
				},
				ParameterSpecs: &katibapi.ExperimentSpec_ParameterSpecs{
					Parameters: grpcParams,
				},
			},
		},
		CurrentRequestNumber: trialsToSpawn,
		TotalRequestNumber:   int32(len(trainJobs)),
	}

	for _, tj := range trainJobs {
		trial := &katibapi.Trial{
			Name: tj.Name,
			Spec: &katibapi.TrialSpec{
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
			if tj.Status.TrainerStatus != nil && len(tj.Status.TrainerStatus.Metrics) > 0 {
				for j := len(tj.Status.TrainerStatus.Metrics) - 1; j >= 0; j-- {
					m := tj.Status.TrainerStatus.Metrics[j]
					if m.Name == targetMetric {
						trial.Status = &katibapi.TrialStatus{
							Condition: katibapi.TrialStatus_SUCCEEDED,
							Observation: &katibapi.Observation{
								Metrics: []*katibapi.Metric{
									{Name: m.Name, Value: m.Value},
								},
							},
						}
						break
					}
				}
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
