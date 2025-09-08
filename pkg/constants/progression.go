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

package constants

import "os"

const (
	// ProgressionStatusFileName is the name of the file where training progression status is written
	ProgressionStatusFileName = "training_progression.json"

	// ProgressionStatusFilePath is the default path where progression status file is expected
	ProgressionStatusFilePath = "/tmp/training_progression.json"

	// ProgressionStatusFilePathEnv is the environment variable to override the status file path
	ProgressionStatusFilePathEnv = "TRAINJOB_PROGRESSION_FILE_PATH"

	// ProgressionProbeIntervalSeconds is the default interval for probing progression status
	ProgressionProbeIntervalSeconds = 30

	// ProgressionStatusMaxAge is the maximum age in seconds for progression status to be considered valid
	ProgressionStatusMaxAge = 300 // 5 minutes
)

// ProgressionFileFormat defines the JSON structure for the progression status file
// This structure should be used by training scripts to write progression information
type ProgressionFileFormat struct {
	// CurrentStep is the current training step/iteration
	CurrentStep *int64 `json:"current_step,omitempty"`

	// TotalSteps is the total number of training steps/iterations
	TotalSteps *int64 `json:"total_steps,omitempty"`

	// CurrentEpoch is the current training epoch
	CurrentEpoch *int64 `json:"current_epoch,omitempty"`

	// TotalEpochs is the total number of training epochs
	TotalEpochs *int64 `json:"total_epochs,omitempty"`

	// Message provides additional information about the training progression
	Message string `json:"message,omitempty"`

	// TrainingMetrics contains structured training metrics (loss, learning_rate, etc.)
	TrainingMetrics map[string]interface{} `json:"training_metrics,omitempty"`

	// Metrics contains additional training metrics (loss, accuracy, etc.)
	Metrics map[string]interface{} `json:"metrics,omitempty"`

	// Timestamp is the Unix timestamp when this status was written
	Timestamp int64 `json:"timestamp"`

	// StartTime is the Unix timestamp when training started (for ETA calculation)
	StartTime *int64 `json:"start_time,omitempty"`
}

// GetProgressionFilePath returns the progression file path, checking environment variable first
func GetProgressionFilePath() string {
	if envPath := os.Getenv(ProgressionStatusFilePathEnv); envPath != "" {
		return envPath
	}
	return ProgressionStatusFilePath
}
