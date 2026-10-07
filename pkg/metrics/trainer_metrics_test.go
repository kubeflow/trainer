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

package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestRuntimeInfoCollector(t *testing.T) {
	scheme := newMetricsScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(
			&trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"},
				Spec:       trainer.TrainingRuntimeSpec{MLPolicy: &trainer.MLPolicy{MLPolicySource: trainer.MLPolicySource{Torch: &trainer.TorchMLPolicySource{}}}},
			},
			&trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "mpi"},
				Spec:       trainer.TrainingRuntimeSpec{PodGroupPolicy: &trainer.PodGroupPolicy{PodGroupPolicySource: trainer.PodGroupPolicySource{Volcano: &trainer.VolcanoPodGroupPolicySource{}}}},
			},
		).Build()

	reg := prometheus.NewRegistry()
	collector := newRuntimeInfoCollector(cli)
	if err := reg.Register(collector); err != nil {
		t.Fatalf("register runtime collector: %v", err)
	}

	want := `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{kind="ClusterTrainingRuntime",ml_policy="none",name="mpi",namespace="",pod_group_policy="volcano"} 1
kubeflow_trainer_runtime_info{kind="TrainingRuntime",ml_policy="torch",name="torch",namespace="team-a",pod_group_policy="none"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "kubeflow_trainer_runtime_info"); err != nil {
		t.Fatalf("unexpected runtime metrics: %v", err)
	}
}

func TestTrainJobObserverRecordsLifecycleAndReplicatedJobState(t *testing.T) {
	scheme := newMetricsScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(&trainer.ClusterTrainingRuntime{
			ObjectMeta: metav1.ObjectMeta{Name: "torch"},
			Spec:       trainer.TrainingRuntimeSpec{MLPolicy: &trainer.MLPolicy{MLPolicySource: trainer.MLPolicySource{Torch: &trainer.TorchMLPolicySource{}}}},
		}).Build()
	m := newTrainerMetrics(cli)
	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer metrics: %v", err)
	}

	created := time.Now().Add(-2 * time.Minute)
	finished := metav1.NewTime(created.Add(90 * time.Second))
	job := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job-a",
			Namespace:         "team-a",
			UID:               types.UID("job-a-uid"),
			CreationTimestamp: metav1.NewTime(created),
			Labels:            map[string]string{kueueQueueLabel: "queue-a"},
		},
		Spec: trainer.TrainJobSpec{RuntimeRef: trainer.RuntimeRef{Name: "torch"}},
		Status: trainer.TrainJobStatus{
			Conditions: []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, Reason: "Completed", LastTransitionTime: finished}},
			JobsStatus: []trainer.JobStatus{{Name: "trainer", Ready: int32Ptr(1), Failed: int32Ptr(0)}},
		},
	}
	m.onAdd(job)

	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_info Current TrainJob metadata for local Prometheus joins.
# TYPE kubeflow_trainer_trainjob_info gauge
kubeflow_trainer_trainjob_info{kueue_queue_label_present="yes",ml_policy="torch",namespace="team-a",pod_group_policy="none",runtime_ref="trainer.kubeflow.org/ClusterTrainingRuntime//torch",trainjob="job-a"} 1
`, "kubeflow_trainer_trainjob_info")
	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_status Current normalized TrainJob lifecycle state.
# TYPE kubeflow_trainer_trainjob_status gauge
kubeflow_trainer_trainjob_status{namespace="team-a",status="succeeded",trainjob="job-a"} 1
`, "kubeflow_trainer_trainjob_status")
	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_replicated_jobs_ready Ready replicated jobs in the current TrainJob status.
# TYPE kubeflow_trainer_trainjob_replicated_jobs_ready gauge
kubeflow_trainer_trainjob_replicated_jobs_ready{namespace="team-a",role="trainer",trainjob="job-a"} 1
`, "kubeflow_trainer_trainjob_replicated_jobs_ready")
	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_created_total TrainJobs observed by the controller metrics observer.
# TYPE kubeflow_trainer_trainjob_created_total counter
kubeflow_trainer_trainjob_created_total{kueue_queue_label_present="yes",ml_policy="torch",pod_group_policy="none"} 1
`, "kubeflow_trainer_trainjob_created_total")
	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_finished_total TrainJobs observed reaching a terminal outcome.
# TYPE kubeflow_trainer_trainjob_finished_total counter
kubeflow_trainer_trainjob_finished_total{reason="none",result="succeeded"} 1
`, "kubeflow_trainer_trainjob_finished_total")
}

func TestTrainJobObserverRecordsResumeAfterSuspension(t *testing.T) {
	scheme := newMetricsScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	m := newTrainerMetrics(cli)
	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer metrics: %v", err)
	}
	job := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{Name: "job-a", Namespace: "team-a", UID: types.UID("job-a-uid")},
		Spec:       trainer.TrainJobSpec{RuntimeRef: trainer.RuntimeRef{Name: "torch"}, Suspend: boolPtr(true)},
	}
	m.onAdd(job)

	resumed := job.DeepCopy()
	resumed.Spec.Suspend = boolPtr(false)
	m.onUpdate(job, resumed)

	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_suspend_events_total Observed TrainJob suspension and resumption transitions.
# TYPE kubeflow_trainer_trainjob_suspend_events_total counter
kubeflow_trainer_trainjob_suspend_events_total{event="resume"} 1
	`, "kubeflow_trainer_trainjob_suspend_events_total")
}

func TestTrainJobObserverUpdatesAndDeletesCurrentState(t *testing.T) {
	scheme := newMetricsScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	m := newTrainerMetrics(cli)
	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer metrics: %v", err)
	}
	job := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{Name: "job-a", Namespace: "team-a", UID: types.UID("job-a-uid")},
		Spec:       trainer.TrainJobSpec{RuntimeRef: trainer.RuntimeRef{Name: "torch"}},
	}
	m.onAdd(job)

	completed := job.DeepCopy()
	completed.Status.Conditions = []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}}
	completed.Status.JobsStatus = []trainer.JobStatus{{Name: "worker", Ready: int32Ptr(2), Failed: int32Ptr(1)}}
	m.onUpdate(job, completed)

	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_status Current normalized TrainJob lifecycle state.
# TYPE kubeflow_trainer_trainjob_status gauge
kubeflow_trainer_trainjob_status{namespace="team-a",status="succeeded",trainjob="job-a"} 1
`, "kubeflow_trainer_trainjob_status")
	assertMetric(t, reg, `# HELP kubeflow_trainer_trainjob_replicated_jobs_failed Failed replicated jobs in the current TrainJob status.
# TYPE kubeflow_trainer_trainjob_replicated_jobs_failed gauge
kubeflow_trainer_trainjob_replicated_jobs_failed{namespace="team-a",role="other",trainjob="job-a"} 1
`, "kubeflow_trainer_trainjob_replicated_jobs_failed")

	m.onUpdate(completed, completed.DeepCopy())
	if got := testutil.ToFloat64(m.finished.WithLabelValues("succeeded", "none")); got != 1 {
		t.Fatalf("terminal counter was incremented on a resync: got %v, want 1", got)
	}
	m.onDelete(completed)
	if got := testutil.ToFloat64(m.trainJobStatus.WithLabelValues("team-a", "job-a", "succeeded")); got != 0 {
		t.Fatalf("current status gauge was not removed: got %v, want 0", got)
	}
}

func TestNormalizedStatusUsesTerminalStateBeforeSuspension(t *testing.T) {
	tests := map[string]struct {
		conditions []metav1.Condition
		suspend    *bool
		want       string
	}{
		"nonterminal": {want: "nonterminal"},
		"suspended":   {suspend: boolPtr(true), want: "suspended"},
		"succeeded": {
			conditions: []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}},
			suspend:    boolPtr(true),
			want:       "succeeded",
		},
		"failed": {
			conditions: []metav1.Condition{{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue}},
			want:       "failed",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			job := &trainer.TrainJob{Spec: trainer.TrainJobSpec{Suspend: test.suspend}, Status: trainer.TrainJobStatus{Conditions: test.conditions}}
			if got := normalizedStatus(job); got != test.want {
				t.Fatalf("normalizedStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

func assertMetric(t *testing.T, reg *prometheus.Registry, want, name string) {
	t.Helper()
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), name); err != nil {
		t.Fatalf("unexpected %s metrics: %v", name, err)
	}
}

func newMetricsScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("add Trainer scheme: %v", err)
	}
	return scheme
}

func int32Ptr(value int32) *int32 { return &value }

func boolPtr(value bool) *bool { return &value }
