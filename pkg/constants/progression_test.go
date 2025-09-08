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

import (
	"os"
	"testing"
)

func TestGetProgressionFilePath(t *testing.T) {
	// Save original env var
	originalEnv := os.Getenv(ProgressionStatusFilePathEnv)
	defer func() {
		if originalEnv != "" {
			os.Setenv(ProgressionStatusFilePathEnv, originalEnv)
		} else {
			os.Unsetenv(ProgressionStatusFilePathEnv)
		}
	}()

	testCases := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "default path when env var not set",
			envValue: "",
			expected: ProgressionStatusFilePath,
		},
		{
			name:     "custom path from env var",
			envValue: "/custom/path/progress.json",
			expected: "/custom/path/progress.json",
		},
		{
			name:     "workspace path from env var",
			envValue: "/workspace/shared/training_status.json",
			expected: "/workspace/shared/training_status.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable
			if tc.envValue != "" {
				os.Setenv(ProgressionStatusFilePathEnv, tc.envValue)
			} else {
				os.Unsetenv(ProgressionStatusFilePathEnv)
			}

			result := GetProgressionFilePath()
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestProgressionFileFormatValidation(t *testing.T) {
	testCases := []struct {
		name   string
		format ProgressionFileFormat
		valid  bool
	}{
		{
			name: "valid minimal format",
			format: ProgressionFileFormat{
				Timestamp: 1703123456,
			},
			valid: true,
		},
		{
			name: "valid complete format",
			format: ProgressionFileFormat{
				CurrentStep:  &[]int64{150}[0],
				TotalSteps:   &[]int64{1000}[0],
				CurrentEpoch: &[]int64{2}[0],
				TotalEpochs:  &[]int64{5}[0],
				Message:      "Training in progress",
				Metrics: map[string]interface{}{
					"loss":          0.245,
					"learning_rate": 0.0001,
				},
				Timestamp: 1703123456,
				StartTime: &[]int64{1703120000}[0],
			},
			valid: true,
		},
		{
			name: "missing timestamp",
			format: ProgressionFileFormat{
				CurrentStep: &[]int64{150}[0],
				TotalSteps:  &[]int64{1000}[0],
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Basic validation - timestamp should be present
			isValid := tc.format.Timestamp > 0
			if isValid != tc.valid {
				t.Errorf("Expected valid=%v, got valid=%v", tc.valid, isValid)
			}
		})
	}
}

func TestProgressionConstants(t *testing.T) {
	// Test that constants have expected values
	expectedValues := map[string]interface{}{
		"ProgressionStatusFileName":       "training_progression.json",
		"ProgressionStatusFilePath":       "/tmp/training_progression.json",
		"ProgressionStatusFilePathEnv":    "TRAINJOB_PROGRESSION_FILE_PATH",
		"ProgressionProbeIntervalSeconds": 30,
		"ProgressionStatusMaxAge":         300,
	}

	actualValues := map[string]interface{}{
		"ProgressionStatusFileName":       ProgressionStatusFileName,
		"ProgressionStatusFilePath":       ProgressionStatusFilePath,
		"ProgressionStatusFilePathEnv":    ProgressionStatusFilePathEnv,
		"ProgressionProbeIntervalSeconds": ProgressionProbeIntervalSeconds,
		"ProgressionStatusMaxAge":         ProgressionStatusMaxAge,
	}

	for key, expected := range expectedValues {
		if actual := actualValues[key]; actual != expected {
			t.Errorf("Constant %s: expected %v, got %v", key, expected, actual)
		}
	}
}
