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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	controllerMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

type metricValue struct {
	labels map[string]string
	count  uint64
	value  float64
}

func TestRuntimeInfoCollector(t *testing.T) {
	tests := map[string]struct {
		objects []runtime.Object
		want    string
	}{
		"namespaced runtime": {
			objects: []runtime.Object{&trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"},
				Spec: trainer.TrainingRuntimeSpec{
					MLPolicy: &trainer.MLPolicy{MLPolicySource: trainer.MLPolicySource{Torch: &trainer.TorchMLPolicySource{}}},
				},
			}},
			want: `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="TrainingRuntime",ml_policy="torch",name="torch",namespace="team-a",pod_group_policy="none"} 1
`,
		},
		"cluster runtime": {
			objects: []runtime.Object{&trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "mpi"},
				Spec: trainer.TrainingRuntimeSpec{
					PodGroupPolicy: &trainer.PodGroupPolicy{PodGroupPolicySource: trainer.PodGroupPolicySource{Volcano: &trainer.VolcanoPodGroupPolicySource{}}},
				},
			}},
			want: `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="ClusterTrainingRuntime",ml_policy="none",name="mpi",namespace="",pod_group_policy="volcano"} 1
			`,
		},
		"empty inventory": {},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { resetMetrics(t) })
			scheme := newMetricsScheme(t)
			objects := make([]client.Object, 0, len(test.objects))
			for _, object := range test.objects {
				objects = append(objects, object.(client.Object))
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			m := newTrainerMetrics(cli)
			reg := prometheus.NewRegistry()
			if err := registerTrainerCollectors(reg, m); err != nil {
				t.Fatalf("register Trainer metrics: %v", err)
			}
			assertMetric(t, reg, test.want, "kubeflow_trainer_runtime_info")
		})
	}
}

func TestSetupTrainerMetricsRegistersRuntimeCollector(t *testing.T) {
	previousRegistry := controllerMetrics.Registry
	t.Cleanup(func() { controllerMetrics.Registry = previousRegistry })
	controllerMetrics.Registry = prometheus.NewRegistry()

	scheme := newMetricsScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&trainer.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"},
	}).Build()
	fakeCache := &informertest.FakeInformers{Scheme: scheme}
	mgr := &metricsTestManager{client: cli, cache: fakeCache}
	if err := setupTrainerMetrics(mgr); err != nil {
		t.Fatalf("setup Trainer metrics: %v", err)
	}

	want := `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="TrainingRuntime",ml_policy="none",name="torch",namespace="team-a",pod_group_policy="none"} 1
`
	if err := testutil.GatherAndCompare(controllerMetrics.Registry, strings.NewReader(want), "kubeflow_trainer_runtime_info"); err != nil {
		t.Fatalf("unexpected runtime metrics: %v", err)
	}
}

func TestTrainJobAdd(t *testing.T) {
	tests := map[string]struct {
		initialTrainJobs    []*trainer.TrainJob
		addedTrainJobs      []*trainer.TrainJob
		wantTrainJobCreated []metricValue
		wantTrainingNodes   []metricValue
	}{
		"ignores initial TrainJobs": {
			initialTrainJobs: []*trainer.TrainJob{trainJob("initial", false)},
		},
		"single TrainJob updates created metrics": {
			addedTrainJobs: []*trainer.TrainJob{trainJob("job-a", false)},
			wantTrainJobCreated: []metricValue{{
				labels: map[string]string{"namespace": "team-a", "runtime_ref": runtimeRefLabel(trainJob("job-a", false))},
				value:  1,
			}},
			wantTrainingNodes: []metricValue{{
				labels: map[string]string{"runtime_ref": runtimeRefLabel(trainJob("job-a", false))},
				count:  1,
				value:  2,
			}},
		},
		"multiple TrainJobs with same runtime": {
			addedTrainJobs: []*trainer.TrainJob{trainJob("job-a", false), trainJob("job-b", false)},
			wantTrainJobCreated: []metricValue{{
				labels: map[string]string{"namespace": "team-a", "runtime_ref": runtimeRefLabel(trainJob("job-a", false))},
				value:  2,
			}},
			wantTrainingNodes: []metricValue{{
				labels: map[string]string{"runtime_ref": runtimeRefLabel(trainJob("job-a", false))},
				count:  2,
				value:  4,
			}},
		},
		"multiple TrainJobs with different runtimes": {
			addedTrainJobs: []*trainer.TrainJob{
				trainJob("job-a", false),
				withRuntime(trainJob("job-b", false), "mpi"),
			},
			wantTrainJobCreated: []metricValue{
				{labels: map[string]string{"namespace": "team-a", "runtime_ref": runtimeRefLabel(trainJob("job-a", false))}, value: 1},
				{labels: map[string]string{"namespace": "team-a", "runtime_ref": runtimeRefLabel(withRuntime(trainJob("job-b", false), "mpi"))}, value: 1},
			},
			wantTrainingNodes: []metricValue{
				{labels: map[string]string{"runtime_ref": runtimeRefLabel(trainJob("job-a", false))}, count: 1, value: 2},
				{labels: map[string]string{"runtime_ref": runtimeRefLabel(withRuntime(trainJob("job-b", false), "mpi"))}, count: 1, value: 2},
			},
		},
		"terminal TrainJob add does not finish": {
			addedTrainJobs: []*trainer.TrainJob{trainJob("terminal", true)},
			wantTrainJobCreated: []metricValue{{
				labels: map[string]string{"namespace": "team-a", "runtime_ref": runtimeRefLabel(trainJob("terminal", true))},
				value:  1,
			}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { resetMetrics(t) })
			scheme := newMetricsScheme(t)
			m := newTrainerMetrics(fake.NewClientBuilder().WithScheme(scheme).Build())
			fakeCache := &informertest.FakeInformers{Scheme: scheme}
			if err := m.installObserver(fakeCache); err != nil {
				t.Fatalf("install observer: %v", err)
			}
			informer, err := fakeCache.FakeInformerFor(context.Background(), &trainer.TrainJob{})
			if err != nil {
				t.Fatalf("get fake informer: %v", err)
			}
			for _, job := range test.initialTrainJobs {
				m.onAdd(job, true)
			}
			for _, job := range test.addedTrainJobs {
				informer.Add(job)
			}
			for _, want := range test.wantTrainJobCreated {
				if got := testutil.ToFloat64(trainJobCreated.WithLabelValues(want.labels["namespace"], want.labels["runtime_ref"])); got != want.value {
					t.Fatalf("created counter for %v = %v, want %v", want.labels, got, want.value)
				}
			}
			reg := prometheus.NewRegistry()
			if err := registerTrainerCollectors(reg, m); err != nil {
				t.Fatalf("register Trainer metrics: %v", err)
			}
			for _, want := range test.wantTrainingNodes {
				assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_requested_training_nodes_per_job", want.labels, want.count, want.value)
			}
		})
	}
}

func TestTrainJobUpdate(t *testing.T) {
	tests := map[string]struct {
		terminalUpdate bool
		repeatUpdate   bool
		failed         bool
		wantFinished   float64
		wantLifetime   float64
	}{
		"nonterminal update":             {wantFinished: 0},
		"successful terminal transition": {terminalUpdate: true, wantFinished: 1, wantLifetime: 60},
		"failed terminal transition":     {terminalUpdate: true, failed: true, wantFinished: 1, wantLifetime: 60},
		"repeated terminal update":       {terminalUpdate: true, repeatUpdate: true, wantFinished: 1, wantLifetime: 60},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { resetMetrics(t) })
			scheme := newMetricsScheme(t)
			m := newTrainerMetrics(fake.NewClientBuilder().WithScheme(scheme).Build())
			job := trainJob("job-a", false)
			job.CreationTimestamp = metav1.NewTime(time.Unix(1000, 0))
			m.onAdd(job, false)
			updated := job.DeepCopy()
			if test.terminalUpdate {
				conditionType := trainer.TrainJobComplete
				result := "succeeded"
				if test.failed {
					conditionType = trainer.TrainJobFailed
					result = "failed"
				}
				reason := "Completed"
				if test.failed {
					reason = jobsetconsts.FailedJobsReason
				}
				updated.Status.Conditions = []metav1.Condition{{
					Type: conditionType, Status: metav1.ConditionTrue, Reason: reason,
					LastTransitionTime: metav1.NewTime(time.Unix(1060, 0)),
				}}
				m.onUpdate(job, updated)
				if test.repeatUpdate {
					m.onUpdate(updated, updated.DeepCopy())
				}
				runtimeRef := runtimeRefLabel(job)
				if got := testutil.ToFloat64(trainJobFinished.WithLabelValues(result, terminalReason(updated.Status.Conditions[0], result), runtimeRef)); got != test.wantFinished {
					t.Fatalf("finished counter = %v, want %v", got, test.wantFinished)
				}
				reg := prometheus.NewRegistry()
				if err := registerTrainerCollectors(reg, m); err != nil {
					t.Fatalf("register Trainer metrics: %v", err)
				}
				assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_lifetime_seconds", map[string]string{
					"result": result, "runtime_ref": runtimeRef,
				}, 1, test.wantLifetime)
				return
			}
			m.onUpdate(job, updated)
			runtimeRef := runtimeRefLabel(job)
			if got := testutil.ToFloat64(trainJobFinished.WithLabelValues("succeeded", "Completed", runtimeRef)); got != test.wantFinished {
				t.Fatalf("finished counter = %v, want %v", got, test.wantFinished)
			}
		})
	}
}

func TestTrainJobAddRecordsAcceleratorClass(t *testing.T) {
	resetMetrics(t)
	scheme := newMetricsScheme(t)
	m := newTrainerMetrics(fake.NewClientBuilder().WithScheme(scheme).Build())
	job := trainJob("gpu-job", false)
	job.Spec.Trainer.ResourcesPerNode = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{"nvidia.com/gpu": resource.MustParse("2")},
	}
	m.onAdd(job, false)

	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer metrics: %v", err)
	}
	assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_requested_accelerators", map[string]string{
		"accelerator_class": "nvidia.com/gpu",
		"runtime_ref":       runtimeRefLabel(job),
	}, 1, 4)
}

func resetMetrics(t *testing.T) {
	t.Helper()
	trainJobCreated.Reset()
	trainJobFinished.Reset()
	trainJobLifetime.Reset()
	trainJobRequestedAccelerators.Reset()
	trainJobRequestedTrainingNodes.Reset()
}

type metricsTestManager struct {
	ctrl.Manager
	client client.Client
	cache  cache.Cache
}

func (m *metricsTestManager) GetClient() client.Client { return m.client }

func (m *metricsTestManager) GetCache() cache.Cache { return m.cache }

func registerTrainerCollectors(reg prometheus.Registerer, m *trainerMetrics) error {
	for _, collector := range []prometheus.Collector{
		m.runtimeInfo,
		trainJobCreated,
		trainJobFinished,
		trainJobLifetime,
		trainJobRequestedAccelerators,
		trainJobRequestedTrainingNodes,
	} {
		if err := reg.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

func withRuntime(job *trainer.TrainJob, name string) *trainer.TrainJob {
	job.Spec.RuntimeRef.Name = name
	return job
}

func assertMetric(t *testing.T, reg *prometheus.Registry, want, name string) {
	t.Helper()
	if want == "" {
		families, err := reg.Gather()
		if err != nil {
			t.Fatalf("gather %s metrics: %v", name, err)
		}
		if len(families) != 0 {
			t.Fatalf("expected no metrics, got %d families", len(families))
		}
		return
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), name); err != nil {
		t.Fatalf("unexpected %s metrics: %v", name, err)
	}
}

func assertHistogramObservation(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string, wantCount uint64, wantSum float64) {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather %s metrics: %v", name, err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			gotLabels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				gotLabels[label.GetName()] = label.GetValue()
			}
			if equalLabels(gotLabels, labels) {
				histogram := metric.GetHistogram()
				if histogram.GetSampleCount() != wantCount || histogram.GetSampleSum() != wantSum {
					t.Fatalf("%s observation = count %d, sum %v; want count %d, sum %v", name, histogram.GetSampleCount(), histogram.GetSampleSum(), wantCount, wantSum)
				}
				return
			}
		}
	}
	t.Fatalf("%s labels %v were not found", name, labels)
}

func equalLabels(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}

func newMetricsScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("add Trainer scheme: %v", err)
	}
	return scheme
}

func trainJob(name string, terminal bool) *trainer.TrainJob {
	job := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "team-a", UID: types.UID(name + "-uid"),
			CreationTimestamp: metav1.Now(),
		},
		Spec: trainer.TrainJobSpec{
			RuntimeRef: trainer.RuntimeRef{Name: "torch"},
			Trainer:    &trainer.Trainer{NumNodes: ptrInt32(2)},
		},
	}
	if terminal {
		job.Status.Conditions = []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}}
	}
	return job
}

func ptrInt32(value int32) *int32 { return &value }
