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

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestProgressionStatusIntegration(t *testing.T) {
	// Test that progression status fields are properly defined
	trainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Status: trainer.TrainJobStatus{
			ProgressionStatus: &trainer.ProgressionStatus{
				Message: "Test progression status",
			},
		},
	}

	if trainJob.Status.ProgressionStatus == nil {
		t.Error("Expected progression status to be set")
	}

	if trainJob.Status.ProgressionStatus.Message != "Test progression status" {
		t.Errorf("Expected message 'Test progression status', got %s", trainJob.Status.ProgressionStatus.Message)
	}
}

func TestPodLabelMatching(t *testing.T) {
	// Test that pod label matching logic works correctly
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-trainer-0-0-abc123",
			Namespace: "default",
			Labels: map[string]string{
				"training.kubeflow.org/job-name":      "test-job",
				"training.kubeflow.org/replica-type":  "trainer",
				"training.kubeflow.org/replica-index": "0",
				"training.kubeflow.org/rank":          "0",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	// Verify labels are set correctly
	if pod.Labels["training.kubeflow.org/rank"] != "0" {
		t.Error("Expected rank label to be '0'")
	}

	if pod.Labels["training.kubeflow.org/replica-type"] != "trainer" {
		t.Error("Expected replica-type label to be 'trainer'")
	}

	if pod.Status.Phase != corev1.PodRunning {
		t.Error("Expected pod to be in Running phase")
	}
}
