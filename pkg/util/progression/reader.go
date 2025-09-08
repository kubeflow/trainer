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

package progression

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
)

type Reader struct {
	clientset kubernetes.Interface
	config    *rest.Config
}

func NewReader(clientset kubernetes.Interface, config *rest.Config) *Reader {
	return &Reader{
		clientset: clientset,
		config:    config,
	}
}

func (r *Reader) ReadProgressionStatus(ctx context.Context, namespace, podName string) (*trainer.ProgressionStatus, error) {
	cmd := []string{"cat", constants.GetProgressionFilePath()}

	req := r.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: cmd,
		Stdout:  true,
		Stderr:  true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr []byte
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdoutWriter{data: &stdout},
		Stderr: &stderrWriter{data: &stderr},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	if len(stderr) > 0 {
		return nil, fmt.Errorf("command failed: %s", string(stderr))
	}

	var fileFormat constants.ProgressionFileFormat
	if err := json.Unmarshal(stdout, &fileFormat); err != nil {
		return nil, fmt.Errorf("failed to parse progression file: %w", err)
	}

	if time.Now().Unix()-fileFormat.Timestamp > constants.ProgressionStatusMaxAge {
		return nil, fmt.Errorf("progression status is too old")
	}

	return r.convertToProgressionStatus(&fileFormat), nil
}

func (r *Reader) convertToProgressionStatus(fileFormat *constants.ProgressionFileFormat) *trainer.ProgressionStatus {
	status := &trainer.ProgressionStatus{
		CurrentStep:    fileFormat.CurrentStep,
		TotalSteps:     fileFormat.TotalSteps,
		CurrentEpoch:   fileFormat.CurrentEpoch,
		TotalEpochs:    fileFormat.TotalEpochs,
		Message:        fileFormat.Message,
		LastUpdateTime: &metav1.Time{Time: time.Unix(fileFormat.Timestamp, 0)},
		Metrics:        make(map[string]string),
	}

	if fileFormat.CurrentStep != nil && fileFormat.TotalSteps != nil && *fileFormat.TotalSteps > 0 {
		percentage := float64(*fileFormat.CurrentStep) / float64(*fileFormat.TotalSteps) * 100
		percentageStr := fmt.Sprintf("%.2f", percentage)
		status.PercentageComplete = &percentageStr
	}

	if fileFormat.StartTime != nil && fileFormat.CurrentStep != nil && fileFormat.TotalSteps != nil {
		elapsed := time.Now().Unix() - *fileFormat.StartTime
		if *fileFormat.CurrentStep > 0 && elapsed > 0 {
			avgTimePerStep := float64(elapsed) / float64(*fileFormat.CurrentStep)
			remainingSteps := *fileFormat.TotalSteps - *fileFormat.CurrentStep
			eta := int64(avgTimePerStep * float64(remainingSteps))
			status.EstimatedTimeRemaining = &eta
		}
	}

	// Parse structured training metrics from both training_metrics and metrics fields
	trainingMetrics := &trainer.TrainingMetrics{}
	hasStructuredMetrics := false

	// First, check the dedicated training_metrics field
	if fileFormat.TrainingMetrics != nil {
		for key, value := range fileFormat.TrainingMetrics {
			valueStr := fmt.Sprintf("%v", value)

			switch key {
			case "loss":
				trainingMetrics.Loss = &valueStr
				hasStructuredMetrics = true
			case "learning_rate":
				trainingMetrics.LearningRate = &valueStr
				hasStructuredMetrics = true
			case "checkpoints_stored":
				if intVal, ok := value.(float64); ok {
					checkpoints := int64(intVal)
					trainingMetrics.CheckpointsStored = &checkpoints
					hasStructuredMetrics = true
				}
			case "latest_checkpoint_path":
				trainingMetrics.LatestCheckpointPath = &valueStr
				hasStructuredMetrics = true
			case "accuracy":
				trainingMetrics.Accuracy = &valueStr
				hasStructuredMetrics = true
			}
		}
	}

	// Also parse structured metrics from the generic metrics field (for backward compatibility)
	if fileFormat.Metrics != nil {
		for key, value := range fileFormat.Metrics {
			valueStr := fmt.Sprintf("%v", value)

			switch key {
			case "loss":
				if trainingMetrics.Loss == nil { // Don't override if already set from training_metrics
					trainingMetrics.Loss = &valueStr
					hasStructuredMetrics = true
				}
			case "learning_rate":
				if trainingMetrics.LearningRate == nil {
					trainingMetrics.LearningRate = &valueStr
					hasStructuredMetrics = true
				}
			case "checkpoints_stored":
				if trainingMetrics.CheckpointsStored == nil {
					if intVal, ok := value.(float64); ok {
						checkpoints := int64(intVal)
						trainingMetrics.CheckpointsStored = &checkpoints
						hasStructuredMetrics = true
					}
				}
			case "latest_checkpoint_path":
				if trainingMetrics.LatestCheckpointPath == nil {
					trainingMetrics.LatestCheckpointPath = &valueStr
					hasStructuredMetrics = true
				}
			case "accuracy":
				if trainingMetrics.Accuracy == nil {
					trainingMetrics.Accuracy = &valueStr
					hasStructuredMetrics = true
				}
			default:
				// Add to generic metrics map
				status.Metrics[key] = valueStr
			}
		}
	}

	if hasStructuredMetrics {
		status.TrainingMetrics = trainingMetrics
	}

	return status
}

type stdoutWriter struct {
	data *[]byte
}

func (w *stdoutWriter) Write(p []byte) (int, error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

type stderrWriter struct {
	data *[]byte
}

func (w *stderrWriter) Write(p []byte) (int, error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}
