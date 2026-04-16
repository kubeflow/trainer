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

package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestRuntimeKind(t *testing.T) {
	tests := []struct {
		name     string
		trainJob *trainer.TrainJob
		want     string
	}{
		{
			name: "kind set to ClusterTrainingRuntime",
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					RuntimeRef: trainer.RuntimeRef{Kind: ptr.To("ClusterTrainingRuntime")},
				},
			},
			want: "ClusterTrainingRuntime",
		},
		{
			name: "kind set to TrainingRuntime",
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					RuntimeRef: trainer.RuntimeRef{Kind: ptr.To("TrainingRuntime")},
				},
			},
			want: "TrainingRuntime",
		},
		{
			name: "kind is nil",
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					RuntimeRef: trainer.RuntimeRef{},
				},
			},
			want: "Unknown",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RuntimeKind(tc.trainJob)
			if got != tc.want {
				t.Errorf("RuntimeKind() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecordTrainJobCreated(t *testing.T) {
	RecordTrainJobCreated("test-ns", "ClusterTrainingRuntime")

	if got := testutil.ToFloat64(TrainJobsCreatedTotal.WithLabelValues("test-ns", "ClusterTrainingRuntime")); got != 1 {
		t.Errorf("TrainJobsCreatedTotal = %v, want 1", got)
	}
	if got := testutil.ToFloat64(TrainJobsActive.WithLabelValues("test-ns", "ClusterTrainingRuntime")); got != 1 {
		t.Errorf("TrainJobsActive = %v, want 1", got)
	}
}

func TestRecordTrainJobDeleted(t *testing.T) {
	// Use a unique namespace so the active gauge is isolated from other tests.
	// One Create followed by one Delete should net to zero.
	RecordTrainJobCreated("test-ns-deleted", "ClusterTrainingRuntime")
	RecordTrainJobDeleted("test-ns-deleted", "ClusterTrainingRuntime")

	if got := testutil.ToFloat64(TrainJobsDeletedTotal.WithLabelValues("test-ns-deleted", "ClusterTrainingRuntime")); got != 1 {
		t.Errorf("TrainJobsDeletedTotal = %v, want 1", got)
	}
	if got := testutil.ToFloat64(TrainJobsActive.WithLabelValues("test-ns-deleted", "ClusterTrainingRuntime")); got != 0 {
		t.Errorf("TrainJobsActive = %v, want 0", got)
	}
}

func TestRecordTrainJobCompleted(t *testing.T) {
	RecordTrainJobCompleted("test-ns", "ClusterTrainingRuntime", 30*time.Second)

	if got := testutil.ToFloat64(TrainJobsCompletedTotal.WithLabelValues("test-ns", "ClusterTrainingRuntime")); got != 1 {
		t.Errorf("TrainJobsCompletedTotal = %v, want 1", got)
	}
	// Verify histogram received at least one observation.
	if count := testutil.CollectAndCount(TrainJobDurationSeconds, "kubeflow_trainer_trainjob_duration_seconds"); count == 0 {
		t.Errorf("TrainJobDurationSeconds has no series, want at least one observation")
	}
}

func TestRecordTrainJobFailed(t *testing.T) {
	RecordTrainJobFailed("test-ns", "ClusterTrainingRuntime", "DeadlineExceeded", 60*time.Second)

	if got := testutil.ToFloat64(TrainJobsFailedTotal.WithLabelValues("test-ns", "ClusterTrainingRuntime", "DeadlineExceeded")); got != 1 {
		t.Errorf("TrainJobsFailedTotal = %v, want 1", got)
	}
	// Verify histogram received at least one observation.
	if count := testutil.CollectAndCount(TrainJobDurationSeconds, "kubeflow_trainer_trainjob_duration_seconds"); count == 0 {
		t.Errorf("TrainJobDurationSeconds has no series, want at least one observation")
	}
}

func TestRecordTrainJobSuspended(t *testing.T) {
	RecordTrainJobSuspended("test-ns", "TrainingRuntime")

	if got := testutil.ToFloat64(TrainJobsSuspendedTotal.WithLabelValues("test-ns", "TrainingRuntime")); got != 1 {
		t.Errorf("TrainJobsSuspendedTotal = %v, want 1", got)
	}
}

func TestObserveReconcile(t *testing.T) {
	ObserveReconcile("trainjob_controller", "success", 5*time.Millisecond)
	ObserveReconcile("trainjob_controller", "error", 2*time.Millisecond)

	// Verify histogram received observations for both results.
	if count := testutil.CollectAndCount(ReconcileDurationSeconds, "kubeflow_trainer_reconcile_duration_seconds"); count == 0 {
		t.Errorf("ReconcileDurationSeconds has no series, want at least one observation")
	}
}

func TestObservePlugin(t *testing.T) {
	ObservePlugin("jobset", "build", 1*time.Millisecond, nil)
	ObservePlugin("torch", "enforce_ml_policy", 2*time.Millisecond, errors.New("plugin error"))

	// Verify the histogram received observations.
	if count := testutil.CollectAndCount(PluginExecutionDurationSeconds, "kubeflow_trainer_plugin_execution_duration_seconds"); count == 0 {
		t.Errorf("PluginExecutionDurationSeconds has no series, want at least one observation")
	}
	if got := testutil.ToFloat64(PluginExecutionErrorsTotal.WithLabelValues("torch", "enforce_ml_policy")); got != 1 {
		t.Errorf("PluginExecutionErrorsTotal[torch/enforce_ml_policy] = %v, want 1", got)
	}
	// No error for jobset/build
	if got := testutil.ToFloat64(PluginExecutionErrorsTotal.WithLabelValues("jobset", "build")); got != 0 {
		t.Errorf("PluginExecutionErrorsTotal[jobset/build] = %v, want 0", got)
	}
}
