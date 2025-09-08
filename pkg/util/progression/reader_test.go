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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
)

func TestConvertToProgressionStatus(t *testing.T) {
	reader := &Reader{}
	now := time.Now().Unix()
	startTime := now - 3600 // 1 hour ago

	testCases := []struct {
		name     string
		input    *constants.ProgressionFileFormat
		expected *trainer.ProgressionStatus
	}{
		{
			name: "basic progression with steps and epochs",
			input: &constants.ProgressionFileFormat{
				CurrentStep:  ptr.To(int64(150)),
				TotalSteps:   ptr.To(int64(1000)),
				CurrentEpoch: ptr.To(int64(2)),
				TotalEpochs:  ptr.To(int64(5)),
				Message:      "Training in progress",
				Timestamp:    now,
				StartTime:    &startTime,
				Metrics: map[string]interface{}{
					"loss":          0.245,
					"learning_rate": 0.0001,
					"accuracy":      0.892,
				},
			},
			expected: &trainer.ProgressionStatus{
				CurrentStep:        ptr.To(int64(150)),
				TotalSteps:         ptr.To(int64(1000)),
				PercentageComplete: ptr.To("15.00"),
				CurrentEpoch:       ptr.To(int64(2)),
				TotalEpochs:        ptr.To(int64(5)),
				Message:            "Training in progress",
				LastUpdateTime:     &metav1.Time{Time: time.Unix(now, 0)},
				TrainingMetrics: &trainer.TrainingMetrics{
					Loss:         ptr.To("0.245"),
					LearningRate: ptr.To("0.0001"),
					Accuracy:     ptr.To("0.892"),
				},
				Metrics: map[string]string{},
			},
		},
		{
			name: "progression with checkpoint metrics",
			input: &constants.ProgressionFileFormat{
				CurrentStep: ptr.To(int64(500)),
				TotalSteps:  ptr.To(int64(2000)),
				Message:     "Checkpoint saved",
				Timestamp:   now,
				Metrics: map[string]interface{}{
					"checkpoints_stored":     float64(3),
					"latest_checkpoint_path": "/workspace/checkpoints/checkpoint-500",
					"custom_metric":          "custom_value",
				},
			},
			expected: &trainer.ProgressionStatus{
				CurrentStep:        ptr.To(int64(500)),
				TotalSteps:         ptr.To(int64(2000)),
				PercentageComplete: ptr.To("25.00"),
				Message:            "Checkpoint saved",
				LastUpdateTime:     &metav1.Time{Time: time.Unix(now, 0)},
				TrainingMetrics: &trainer.TrainingMetrics{
					CheckpointsStored:    ptr.To(int64(3)),
					LatestCheckpointPath: ptr.To("/workspace/checkpoints/checkpoint-500"),
				},
				Metrics: map[string]string{
					"custom_metric": "custom_value",
				},
			},
		},
		{
			name: "progression with ETA calculation",
			input: &constants.ProgressionFileFormat{
				CurrentStep: ptr.To(int64(100)),
				TotalSteps:  ptr.To(int64(1000)),
				Timestamp:   now,
				StartTime:   &startTime,
			},
			expected: &trainer.ProgressionStatus{
				CurrentStep:            ptr.To(int64(100)),
				TotalSteps:             ptr.To(int64(1000)),
				PercentageComplete:     ptr.To("10.00"),
				EstimatedTimeRemaining: ptr.To(int64(32400)), // 9 hours remaining
				LastUpdateTime:         &metav1.Time{Time: time.Unix(now, 0)},
				Metrics:                map[string]string{},
			},
		},
		{
			name: "minimal progression data",
			input: &constants.ProgressionFileFormat{
				Message:   "Starting training",
				Timestamp: now,
			},
			expected: &trainer.ProgressionStatus{
				Message:        "Starting training",
				LastUpdateTime: &metav1.Time{Time: time.Unix(now, 0)},
				Metrics:        map[string]string{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := reader.convertToProgressionStatus(tc.input)

			// Check basic fields
			if result.CurrentStep != nil && tc.expected.CurrentStep != nil {
				if *result.CurrentStep != *tc.expected.CurrentStep {
					t.Errorf("CurrentStep: expected %d, got %d", *tc.expected.CurrentStep, *result.CurrentStep)
				}
			}

			if result.TotalSteps != nil && tc.expected.TotalSteps != nil {
				if *result.TotalSteps != *tc.expected.TotalSteps {
					t.Errorf("TotalSteps: expected %d, got %d", *tc.expected.TotalSteps, *result.TotalSteps)
				}
			}

			if result.PercentageComplete != nil && tc.expected.PercentageComplete != nil {
				if *result.PercentageComplete != *tc.expected.PercentageComplete {
					t.Errorf("PercentageComplete: expected %s, got %s", *tc.expected.PercentageComplete, *result.PercentageComplete)
				}
			}

			if result.Message != tc.expected.Message {
				t.Errorf("Message: expected %s, got %s", tc.expected.Message, result.Message)
			}

			// Check training metrics
			if tc.expected.TrainingMetrics != nil {
				if result.TrainingMetrics == nil {
					t.Error("Expected TrainingMetrics, got nil")
				} else {
					if result.TrainingMetrics.Loss != nil && tc.expected.TrainingMetrics.Loss != nil {
						if *result.TrainingMetrics.Loss != *tc.expected.TrainingMetrics.Loss {
							t.Errorf("Loss: expected %s, got %s", *tc.expected.TrainingMetrics.Loss, *result.TrainingMetrics.Loss)
						}
					}
				}
			}

			// Check custom metrics
			for key, expectedValue := range tc.expected.Metrics {
				if actualValue, exists := result.Metrics[key]; !exists {
					t.Errorf("Missing custom metric: %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("Custom metric %s: expected %s, got %s", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestPercentageCalculation(t *testing.T) {
	reader := &Reader{}
	now := time.Now().Unix()

	testCases := []struct {
		name            string
		currentStep     int64
		totalSteps      int64
		expectedPercent string
	}{
		{"zero progress", 0, 1000, "0.00"},
		{"quarter progress", 250, 1000, "25.00"},
		{"half progress", 500, 1000, "50.00"},
		{"near complete", 999, 1000, "99.90"},
		{"complete", 1000, 1000, "100.00"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := &constants.ProgressionFileFormat{
				CurrentStep: &tc.currentStep,
				TotalSteps:  &tc.totalSteps,
				Timestamp:   now,
			}

			result := reader.convertToProgressionStatus(input)

			if result.PercentageComplete == nil {
				t.Error("Expected PercentageComplete, got nil")
			} else if *result.PercentageComplete != tc.expectedPercent {
				t.Errorf("Expected %s, got %s", tc.expectedPercent, *result.PercentageComplete)
			}
		})
	}
}

func TestETACalculation(t *testing.T) {
	reader := &Reader{}
	now := time.Now().Unix()
	startTime := now - 100 // 100 seconds ago

	input := &constants.ProgressionFileFormat{
		CurrentStep: ptr.To(int64(10)),  // 10 steps completed
		TotalSteps:  ptr.To(int64(100)), // 100 total steps
		Timestamp:   now,
		StartTime:   &startTime,
	}

	result := reader.convertToProgressionStatus(input)

	if result.EstimatedTimeRemaining == nil {
		t.Error("Expected EstimatedTimeRemaining, got nil")
	} else {
		// 10 steps in 100 seconds = 10 seconds per step
		// 90 steps remaining = 900 seconds ETA
		expectedETA := int64(900)
		if *result.EstimatedTimeRemaining != expectedETA {
			t.Errorf("Expected ETA %d, got %d", expectedETA, *result.EstimatedTimeRemaining)
		}
	}
}
