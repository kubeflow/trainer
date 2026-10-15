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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	controllerMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
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
		"runtime with only numNodes": {
			objects: []runtime.Object{&trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"},
				Spec: trainer.TrainingRuntimeSpec{
					MLPolicy: &trainer.MLPolicy{NumNodes: ptr.To(int32(4))},
				},
			}},
			want: `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="TrainingRuntime",ml_policy="none",name="torch",namespace="team-a",pod_group_policy="none"} 1
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
			objects := make([]client.Object, 0, len(test.objects))
			for _, object := range test.objects {
				objects = append(objects, object.(client.Object))
			}
			m := newTrainerMetrics(nil)
			for _, object := range objects {
				switch runtimeObject := object.(type) {
				case *trainer.TrainingRuntime:
					m.runtimeInfo.upsert(runtimeObject, trainingRuntimeKind)
				case *trainer.ClusterTrainingRuntime:
					m.runtimeInfo.upsert(runtimeObject, clusterTrainingRuntimeKind)
				}
			}
			reg := prometheus.NewRegistry()
			if err := registerTrainerCollectors(reg, m); err != nil {
				t.Fatalf("register Trainer metrics: %v", err)
			}
			assertMetric(t, reg, test.want, "kubeflow_trainer_runtime_info")
		})
	}
}

func TestSetupTrainerMetricsAddsLeaderOwnedRunnables(t *testing.T) {
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

	if len(mgr.added) != 2 {
		t.Fatalf("added runnables = %d, want 2", len(mgr.added))
	}
	if _, ok := mgr.added[0].(*runtimeInfoCollector); !ok {
		t.Fatalf("first runnable = %T, want runtimeInfoCollector", mgr.added[0])
	}
}

func TestLeaderMetricsRegistration(t *testing.T) {
	previousRegistry := controllerMetrics.Registry
	t.Cleanup(func() { controllerMetrics.Registry = previousRegistry })
	controllerMetrics.Registry = prometheus.NewRegistry()

	collector := newRuntimeInfoCollector(nil)
	collector.upsert(&trainer.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"},
	}, trainingRuntimeKind)
	if err := testutil.GatherAndCompare(controllerMetrics.Registry, strings.NewReader(""), "kubeflow_trainer_runtime_info"); err != nil {
		t.Fatalf("metrics are registered before leadership: %v", err)
	}

	if err := registerLeaderMetrics(collector); err != nil {
		t.Fatalf("register leader metrics: %v", err)
	}
	want := `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="TrainingRuntime",ml_policy="none",name="torch",namespace="team-a",pod_group_policy="none"} 1
`
	if err := testutil.GatherAndCompare(controllerMetrics.Registry, strings.NewReader(want), "kubeflow_trainer_runtime_info"); err != nil {
		t.Fatalf("unexpected leader metrics: %v", err)
	}

	unregisterLeaderMetrics(collector)
	if err := testutil.GatherAndCompare(controllerMetrics.Registry, strings.NewReader(""), "kubeflow_trainer_runtime_info"); err != nil {
		t.Fatalf("metrics remain registered after leadership loss: %v", err)
	}
	if err := registerLeaderMetrics(collector); err != nil {
		t.Fatalf("re-register leader metrics: %v", err)
	}
	unregisterLeaderMetrics(collector)
}

func TestMetricsRunnablesUseExpectedLeadership(t *testing.T) {
	m := newTrainerMetrics(nil)
	if !m.NeedLeaderElection() {
		t.Fatal("TrainJob metrics observer must require leader election")
	}
	collector := newRuntimeInfoCollector(nil)
	if !collector.NeedLeaderElection() {
		t.Fatal("runtime inventory collector must require leader election")
	}
}

func TestRuntimeInfoCollectorSnapshotUpdates(t *testing.T) {
	t.Cleanup(func() { resetMetrics(t) })
	collector := newRuntimeInfoCollector(nil)
	runtime := &trainer.TrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"}}
	collector.upsert(runtime, trainingRuntimeKind)

	reg := prometheus.NewRegistry()
	if err := reg.Register(collector); err != nil {
		t.Fatalf("register runtime collector: %v", err)
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(`# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="TrainingRuntime",ml_policy="none",name="torch",namespace="team-a",pod_group_policy="none"} 1
`), "kubeflow_trainer_runtime_info"); err != nil {
		t.Fatalf("unexpected runtime snapshot: %v", err)
	}

	collector.delete(toolscache.DeletedFinalStateUnknown{Obj: runtime}, trainingRuntimeKind)
	assertMetric(t, reg, "", "kubeflow_trainer_runtime_info")
}

func TestRuntimeInfoCollectorStartTracksInformerLifecycle(t *testing.T) {
	scheme := newMetricsScheme(t)
	fakeCache := &informertest.FakeInformers{Scheme: scheme}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	trainingRuntime := &trainer.TrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"}}
	clusterRuntime := &trainer.ClusterTrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "mpi"}}
	trainingInformer := newReplayableInformer(trainingRuntime)
	clusterInformer := newReplayableInformer(clusterRuntime)
	metricsCache := &replayableMetricsCache{
		Cache:                   fakeCache,
		trainingRuntimeInformer: trainingInformer,
		clusterRuntimeInformer:  clusterInformer,
	}

	started := make(chan error, 1)
	collector := newRuntimeInfoCollector(metricsCache)
	go func() { started <- collector.Start(ctx) }()

	deadline := time.Now().Add(time.Second)
	for !collector.ready.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !collector.ready.Load() {
		t.Fatal("runtime metrics collector did not become ready")
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(collector); err != nil {
		t.Fatalf("register runtime collector: %v", err)
	}
	assertMetric(t, reg, `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="ClusterTrainingRuntime",ml_policy="none",name="mpi",namespace="",pod_group_policy="none"} 1
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="TrainingRuntime",ml_policy="none",name="torch",namespace="team-a",pod_group_policy="none"} 1
`, "kubeflow_trainer_runtime_info")

	updatedRuntime := trainingRuntime.DeepCopy()
	updatedRuntime.Spec.MLPolicy = &trainer.MLPolicy{NumNodes: ptr.To(int32(4))}
	trainingInformer.Update(trainingRuntime, updatedRuntime)
	trainingKind := trainingRuntimeKind
	if nodes, ok := collector.requestedNodes(&trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		Spec:       trainer.TrainJobSpec{RuntimeRef: trainer.RuntimeRef{Name: "torch", Kind: &trainingKind}},
	}); !ok || nodes != 4 {
		t.Fatalf("requested runtime nodes = (%d, %t), want (4, true)", nodes, ok)
	}

	clusterInformer.Delete(clusterRuntime)
	assertMetric(t, reg, `# HELP kubeflow_trainer_runtime_info Current TrainingRuntime and ClusterTrainingRuntime resources.
# TYPE kubeflow_trainer_runtime_info gauge
kubeflow_trainer_runtime_info{api_group="trainer.kubeflow.org",kind="TrainingRuntime",ml_policy="none",name="torch",namespace="team-a",pod_group_policy="none"} 1
`, "kubeflow_trainer_runtime_info")

	cancel()
	select {
	case err := <-started:
		if err != nil {
			t.Fatalf("runtime metrics collector stopped with error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime metrics collector did not stop after context cancellation")
	}
	collector.mu.RLock()
	snapshotSize := len(collector.snapshot)
	collector.mu.RUnlock()
	if snapshotSize != 0 {
		t.Fatalf("runtime snapshot after stop = %d entries, want 0", snapshotSize)
	}
	trainingInformer.Add(&trainer.TrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "after-stop", Namespace: "team-a"}})
	collector.mu.RLock()
	snapshotSize = len(collector.snapshot)
	collector.mu.RUnlock()
	if snapshotSize != 0 {
		t.Fatalf("runtime event was processed after stop: snapshot size = %d, want 0", snapshotSize)
	}
}

func TestRequestedNodesLookupFailureSkipsObservation(t *testing.T) {
	t.Cleanup(func() { resetMetrics(t) })
	m := newTrainerMetrics(nil)
	m.ready.Store(true)
	job := trainJob("job-a", false)
	job.Spec.Trainer = nil
	m.onAdd(job)

	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer metrics: %v", err)
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(""), "kubeflow_trainer_trainjob_requested_training_nodes_per_job"); err != nil {
		t.Fatalf("unexpected requested-node metric: %v", err)
	}
}

func TestRequestedNodesUsesRuntimeSnapshot(t *testing.T) {
	tests := map[string]struct {
		runtime client.Object
		kind    string
	}{
		"namespaced runtime": {
			runtime: &trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"},
				Spec:       trainer.TrainingRuntimeSpec{MLPolicy: &trainer.MLPolicy{NumNodes: ptr.To(int32(4))}},
			},
			kind: trainingRuntimeKind,
		},
		"cluster runtime": {
			runtime: &trainer.ClusterTrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "torch"},
				Spec:       trainer.TrainingRuntimeSpec{MLPolicy: &trainer.MLPolicy{NumNodes: ptr.To(int32(4))}},
			},
			kind: clusterTrainingRuntimeKind,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { resetMetrics(t) })
			collector := newRuntimeInfoCollector(nil)
			collector.upsert(test.runtime, test.kind)
			m := newTrainerMetrics(nil)
			m.runtimeInfo = collector
			m.ready.Store(true)
			job := trainJob("job-a", false)
			if test.kind == trainingRuntimeKind {
				kind := trainingRuntimeKind
				job.Spec.RuntimeRef.Kind = &kind
			}
			job.Spec.Trainer = nil
			m.onAdd(job)

			reg := prometheus.NewRegistry()
			if err := registerTrainerCollectors(reg, m); err != nil {
				t.Fatalf("register Trainer collectors: %v", err)
			}
			assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_requested_training_nodes_per_job", map[string]string{
				"runtime_ref": runtimeRefLabel(job),
			}, 1, 4)
		})
	}
}

func TestRequestedNodesDefaultsToOneFromRuntimeSnapshot(t *testing.T) {
	t.Cleanup(func() { resetMetrics(t) })
	collector := newRuntimeInfoCollector(nil)
	collector.upsert(&trainer.ClusterTrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "torch"},
		Spec:       trainer.TrainingRuntimeSpec{},
	}, clusterTrainingRuntimeKind)
	m := newTrainerMetrics(nil)
	m.runtimeInfo = collector
	m.ready.Store(true)
	job := trainJob("job-a", false)
	job.Spec.Trainer = nil
	m.onAdd(job)

	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer collectors: %v", err)
	}
	assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_requested_training_nodes_per_job", map[string]string{
		"runtime_ref": runtimeRefLabel(job),
	}, 1, 1)
}

func TestRequestedNodesUsesRuntimeTemplateParallelism(t *testing.T) {
	for name, test := range map[string]struct {
		object client.Object
		kind   string
	}{
		"namespaced runtime": {
			object: &trainer.TrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "torch", Namespace: "team-a"}, Spec: templateRuntimeSpec(8)},
			kind:   trainingRuntimeKind,
		},
		"cluster runtime": {
			object: &trainer.ClusterTrainingRuntime{ObjectMeta: metav1.ObjectMeta{Name: "torch"}, Spec: templateRuntimeSpec(8)},
			kind:   clusterTrainingRuntimeKind,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { resetMetrics(t) })
			collector := newRuntimeInfoCollector(nil)
			collector.upsert(test.object, test.kind)
			m := newTrainerMetrics(nil)
			m.runtimeInfo = collector
			m.ready.Store(true)
			job := trainJob("job-a", false)
			job.Spec.Trainer = nil
			if test.kind == trainingRuntimeKind {
				kind := trainingRuntimeKind
				job.Spec.RuntimeRef.Kind = &kind
			}
			m.onAdd(job)

			reg := prometheus.NewRegistry()
			if err := registerTrainerCollectors(reg, m); err != nil {
				t.Fatalf("register Trainer collectors: %v", err)
			}
			assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_requested_training_nodes_per_job", map[string]string{
				"runtime_ref": runtimeRefLabel(job),
			}, 1, 8)
		})
	}
}

func TestRequestedNodesFromRuntimeValidatesValues(t *testing.T) {
	tests := map[string]struct {
		nodes int32
		want  int32
		ok    bool
	}{
		"zero is preserved":     {nodes: 0, want: 0, ok: true},
		"negative is skipped":   {nodes: -1, ok: false},
		"positive is preserved": {nodes: 4, want: 4, ok: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec := trainer.TrainingRuntimeSpec{MLPolicy: &trainer.MLPolicy{NumNodes: ptr.To(test.nodes)}}
			got, ok := requestedNodesFromSpec(&spec)
			if got != test.want || ok != test.ok {
				t.Fatalf("requestedNodesFromSpec(%d) = (%d, %t), want (%d, %t)", test.nodes, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestRequestedNodesIncludesExplicitZero(t *testing.T) {
	t.Cleanup(func() { resetMetrics(t) })
	m := newTrainerMetrics(nil)
	m.ready.Store(true)
	job := trainJob("job-a", false)
	job.Spec.Trainer.NumNodes = ptr.To(int32(0))
	m.onAdd(job)

	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer collectors: %v", err)
	}
	assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_requested_training_nodes_per_job", map[string]string{
		"runtime_ref": runtimeRefLabel(job),
	}, 1, 0)
}

func TestRequestedNodesSkipsNegativeExplicitValue(t *testing.T) {
	t.Cleanup(func() { resetMetrics(t) })
	collector := newRuntimeInfoCollector(nil)
	collector.upsert(&trainer.ClusterTrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "torch"},
		Spec:       trainer.TrainingRuntimeSpec{MLPolicy: &trainer.MLPolicy{NumNodes: ptr.To(int32(4))}},
	}, clusterTrainingRuntimeKind)
	m := newTrainerMetrics(nil)
	m.runtimeInfo = collector
	m.ready.Store(true)
	job := trainJob("job-a", false)
	job.Spec.Trainer.NumNodes = ptr.To(int32(-1))
	m.onAdd(job)

	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, m); err != nil {
		t.Fatalf("register Trainer collectors: %v", err)
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(""), "kubeflow_trainer_trainjob_requested_training_nodes_per_job"); err != nil {
		t.Fatalf("negative TrainJob numNodes produced an observation: %v", err)
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
			m := newTrainerMetrics(nil)
			m.ready.Store(true)
			for _, job := range test.addedTrainJobs {
				m.onAdd(job)
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

func TestTrainJobMetricsStartSkipsInitialObjects(t *testing.T) {
	previousRegistry := controllerMetrics.Registry
	t.Cleanup(func() { controllerMetrics.Registry = previousRegistry })
	controllerMetrics.Registry = prometheus.NewRegistry()
	resetMetrics(t)
	t.Cleanup(func() { resetMetrics(t) })

	scheme := newMetricsScheme(t)
	baseCache := &informertest.FakeInformers{Scheme: scheme}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	informer := newReplayableInformer(trainJob("initial", false))
	fakeCache := &replayableMetricsCache{Cache: baseCache, trainJobInformer: informer}
	if _, err := fakeCache.GetInformer(ctx, &trainer.TrainJob{}); err != nil {
		t.Fatalf("get TrainJob informer: %v", err)
	}
	if _, err := baseCache.FakeInformerFor(ctx, &trainer.TrainingRuntime{}); err != nil {
		t.Fatalf("get TrainingRuntime informer: %v", err)
	}
	if _, err := baseCache.FakeInformerFor(ctx, &trainer.ClusterTrainingRuntime{}); err != nil {
		t.Fatalf("get ClusterTrainingRuntime informer: %v", err)
	}
	if err := func() error {
		m := newTrainerMetrics(fakeCache)
		m.runtimeInfo.ready.Store(true)
		started := make(chan error, 1)
		go func() { started <- m.Start(ctx) }()
		deadline := time.Now().Add(time.Second)
		for !m.ready.Load() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !m.ready.Load() {
			return fmt.Errorf("TrainJob metrics observer did not become ready")
		}

		if got := testutil.ToFloat64(trainJobCreated.WithLabelValues("team-a", runtimeRefLabel(trainJob("initial", false)))); got != 0 {
			return fmt.Errorf("initial TrainJob was counted as created: %v", got)
		}

		created := trainJob("created", false)
		informer.Add(created)
		runtimeRef := runtimeRefLabel(created)
		if got := testutil.ToFloat64(trainJobCreated.WithLabelValues("team-a", runtimeRef)); got != 1 {
			return fmt.Errorf("created counter = %v, want 1", got)
		}

		nonTerminal := created.DeepCopy()
		nonTerminal.Generation = 2
		informer.Update(created, nonTerminal)
		if got := testutil.ToFloat64(trainJobComplete.WithLabelValues(runtimeRef)); got != 0 {
			return fmt.Errorf("non-terminal update changed complete counter to %v", got)
		}
		if got := testutil.ToFloat64(trainJobFailed.WithLabelValues("other", runtimeRef)); got != 0 {
			return fmt.Errorf("non-terminal update changed failed counter to %v", got)
		}

		terminal := nonTerminal.DeepCopy()
		terminal.Status.Conditions = []metav1.Condition{{
			Type:               trainer.TrainJobComplete,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		}}
		informer.Update(nonTerminal, terminal)
		if got := testutil.ToFloat64(trainJobComplete.WithLabelValues(runtimeRef)); got != 1 {
			return fmt.Errorf("complete counter = %v, want 1", got)
		}
		informer.Update(terminal, terminal.DeepCopy())
		if got := testutil.ToFloat64(trainJobComplete.WithLabelValues(runtimeRef)); got != 1 {
			return fmt.Errorf("repeated terminal update changed counter to %v, want 1", got)
		}

		cancel()
		select {
		case err := <-started:
			return err
		case <-time.After(time.Second):
			return fmt.Errorf("metrics observer did not stop after context cancellation")
		}
	}(); err != nil {
		t.Fatal(err)
	}

	createdBeforeStop := testutil.ToFloat64(trainJobCreated.WithLabelValues("team-a", runtimeRefLabel(trainJob("created", false))))
	informer.Add(trainJob("after-stop", false))
	if got := testutil.ToFloat64(trainJobCreated.WithLabelValues("team-a", runtimeRefLabel(trainJob("created", false)))); got != createdBeforeStop {
		t.Fatalf("TrainJob was counted after observer stop: %v, want %v", got, createdBeforeStop)
	}
}

func TestTrainJobMetricsCanRestartLeadership(t *testing.T) {
	previousRegistry := controllerMetrics.Registry
	t.Cleanup(func() { controllerMetrics.Registry = previousRegistry })
	controllerMetrics.Registry = prometheus.NewRegistry()
	resetMetrics(t)

	scheme := newMetricsScheme(t)
	baseCache := &informertest.FakeInformers{Scheme: scheme}
	informer := newReplayableInformer()
	fakeCache := &replayableMetricsCache{Cache: baseCache, trainJobInformer: informer}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	startTerm := func() <-chan error {
		m := newTrainerMetrics(fakeCache)
		m.runtimeInfo.ready.Store(true)
		result := make(chan error, 1)
		go func() { result <- m.Start(ctx) }()
		deadline := time.Now().Add(time.Second)
		for !m.ready.Load() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !m.ready.Load() {
			t.Fatal("TrainJob metrics observer did not become ready")
		}
		return result
	}

	first := startTerm()
	cancel()
	if err := <-first; err != nil {
		t.Fatalf("first leadership term stopped with error: %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	second := startTerm()
	cancel()
	if err := <-second; err != nil {
		t.Fatalf("second leadership term stopped with error: %v", err)
	}
}

func TestTrainJobUpdate(t *testing.T) {
	tests := map[string]struct {
		terminalUpdate bool
		repeatUpdate   bool
		failed         bool
		failureReason  string
		wantComplete   float64
		wantFailed     float64
		wantLifetime   float64
		wantReason     string
	}{
		"nonterminal update":             {},
		"successful terminal transition": {terminalUpdate: true, wantComplete: 1, wantLifetime: 60},
		"failed terminal transition":     {terminalUpdate: true, failed: true, wantFailed: 1, wantLifetime: 60, wantReason: jobsetconsts.FailedJobsReason},
		"unknown failure reason":         {terminalUpdate: true, failed: true, failureReason: "UnexpectedFailure", wantFailed: 1, wantLifetime: 60, wantReason: "other"},
		"repeated terminal update":       {terminalUpdate: true, repeatUpdate: true, wantComplete: 1, wantLifetime: 60},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { resetMetrics(t) })
			m := newTrainerMetrics(nil)
			m.ready.Store(true)
			job := trainJob("job-a", false)
			job.CreationTimestamp = metav1.NewTime(time.Unix(1000, 0))
			m.onAdd(job)
			updated := job.DeepCopy()
			if test.terminalUpdate {
				conditionType := trainer.TrainJobComplete
				if test.failed {
					conditionType = trainer.TrainJobFailed
				}
				reason := "Completed"
				if test.failed {
					reason = jobsetconsts.FailedJobsReason
				}
				if test.failureReason != "" {
					reason = test.failureReason
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
				if got := testutil.ToFloat64(trainJobComplete.WithLabelValues(runtimeRef)); got != test.wantComplete {
					t.Fatalf("complete counter = %v, want %v", got, test.wantComplete)
				}
				expectedReason := test.wantReason
				if expectedReason == "" {
					expectedReason = jobsetconsts.FailedJobsReason
				}
				if got := testutil.ToFloat64(trainJobFailed.WithLabelValues(expectedReason, runtimeRef)); got != test.wantFailed {
					t.Fatalf("failed counter = %v, want %v", got, test.wantFailed)
				}
				reg := prometheus.NewRegistry()
				if err := registerTrainerCollectors(reg, m); err != nil {
					t.Fatalf("register Trainer metrics: %v", err)
				}
				result := "succeeded"
				if test.failed {
					result = "failed"
				}
				assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_lifetime_seconds", map[string]string{
					"result": result, "runtime_ref": runtimeRef,
				}, 1, test.wantLifetime)
				return
			}
			m.onUpdate(job, updated)
			runtimeRef := runtimeRefLabel(job)
			if got := testutil.ToFloat64(trainJobComplete.WithLabelValues(runtimeRef)); got != test.wantComplete {
				t.Fatalf("complete counter = %v, want %v", got, test.wantComplete)
			}
			if got := testutil.ToFloat64(trainJobFailed.WithLabelValues("other", runtimeRef)); got != test.wantFailed {
				t.Fatalf("failed counter = %v, want %v", got, test.wantFailed)
			}
		})
	}
}

func TestTrainJobLifetimeSkipsInvalidTimestamps(t *testing.T) {
	tests := map[string]struct {
		creationTimestamp metav1.Time
		transitionTime    metav1.Time
	}{
		"missing creation timestamp": {
			transitionTime: metav1.NewTime(time.Unix(1060, 0)),
		},
		"missing terminal timestamp": {
			creationTimestamp: metav1.NewTime(time.Unix(1000, 0)),
		},
		"negative lifetime": {
			creationTimestamp: metav1.NewTime(time.Unix(1060, 0)),
			transitionTime:    metav1.NewTime(time.Unix(1000, 0)),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { resetMetrics(t) })
			m := newTrainerMetrics(nil)
			job := trainJob("job-a", false)
			job.CreationTimestamp = test.creationTimestamp
			job.Status.Conditions = []metav1.Condition{{
				Type:               trainer.TrainJobComplete,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: test.transitionTime,
			}}
			m.observeTerminal(job, runtimeRefLabel(job))

			if got := testutil.ToFloat64(trainJobComplete.WithLabelValues(runtimeRefLabel(job))); got != 1 {
				t.Fatalf("complete counter = %v, want 1", got)
			}
			reg := prometheus.NewRegistry()
			if err := registerTrainerCollectors(reg, m); err != nil {
				t.Fatalf("register Trainer metrics: %v", err)
			}
			if err := testutil.GatherAndCompare(reg, strings.NewReader(""), "kubeflow_trainer_trainjob_lifetime_seconds"); err != nil {
				t.Fatalf("unexpected lifetime metric: %v", err)
			}
		})
	}
}

func TestTrainJobFinishedMetricIsRemoved(t *testing.T) {
	t.Cleanup(func() { resetMetrics(t) })
	reg := prometheus.NewRegistry()
	if err := registerTrainerCollectors(reg, newTrainerMetrics(nil)); err != nil {
		t.Fatalf("register Trainer metrics: %v", err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather Trainer metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() == "kubeflow_trainer_trainjob_finished_total" {
			t.Fatal("legacy trainjob_finished_total metric must not be registered")
		}
	}
}

func resetMetrics(t *testing.T) {
	t.Helper()
	trainJobCreated.Reset()
	trainJobComplete.Reset()
	trainJobFailed.Reset()
	trainJobLifetime.Reset()
	trainJobRequestedTrainingNodes.Reset()
}

type metricsTestManager struct {
	ctrl.Manager
	client client.Client
	cache  cache.Cache
	added  []manager.Runnable
}

func (m *metricsTestManager) GetClient() client.Client { return m.client }

func (m *metricsTestManager) GetCache() cache.Cache { return m.cache }

func (m *metricsTestManager) Add(runnable manager.Runnable) error {
	m.added = append(m.added, runnable)
	return nil
}

func registerTrainerCollectors(reg prometheus.Registerer, m *trainerMetrics) error {
	for _, collector := range []prometheus.Collector{
		m.runtimeInfo,
		trainJobCreated,
		trainJobComplete,
		trainJobFailed,
		trainJobLifetime,
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

func templateRuntimeSpec(parallelism int32) trainer.TrainingRuntimeSpec {
	return trainer.TrainingRuntimeSpec{
		Template: trainer.JobSetTemplateSpec{
			Spec: jobsetv1alpha2.JobSetSpec{ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{{
				Template: batchv1.JobTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
						constants.LabelTrainJobAncestor: constants.AncestorTrainer,
					}},
					Spec: batchv1.JobSpec{Parallelism: ptr.To(parallelism)},
				},
			}}},
		},
	}
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

type replayableMetricsCache struct {
	cache.Cache
	trainJobInformer        cache.Informer
	trainingRuntimeInformer cache.Informer
	clusterRuntimeInformer  cache.Informer
}

func (c *replayableMetricsCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	switch obj.(type) {
	case *trainer.TrainJob:
		return c.trainJobInformer, nil
	case *trainer.TrainingRuntime:
		return c.trainingRuntimeInformer, nil
	case *trainer.ClusterTrainingRuntime:
		return c.clusterRuntimeInformer, nil
	}
	return c.Cache.GetInformer(ctx, obj, opts...)
}

type replayableInformer struct {
	*controllertest.FakeInformer
	mu       sync.Mutex
	initial  []metav1.Object
	handlers map[toolscache.ResourceEventHandlerRegistration]*replayableHandler
}

type replayableHandler struct {
	delegate toolscache.ResourceEventHandler
	active   atomic.Bool
}

func newReplayableInformer(initial ...metav1.Object) *replayableInformer {
	return &replayableInformer{
		FakeInformer: controllertest.NewFakeInformer(controllertest.Synced),
		initial:      initial,
		handlers:     make(map[toolscache.ResourceEventHandlerRegistration]*replayableHandler),
	}
}

func (h *replayableHandler) OnAdd(obj any, isInInitialList bool) {
	if h.active.Load() {
		h.delegate.OnAdd(obj, isInInitialList)
	}
}

func (h *replayableHandler) OnUpdate(oldObj, newObj any) {
	if h.active.Load() {
		h.delegate.OnUpdate(oldObj, newObj)
	}
}

func (h *replayableHandler) OnDelete(obj any) {
	if h.active.Load() {
		h.delegate.OnDelete(obj)
	}
}

func (i *replayableInformer) AddEventHandler(handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	wrapped := &replayableHandler{delegate: handler}
	wrapped.active.Store(true)
	registration, err := i.FakeInformer.AddEventHandler(wrapped)
	if err != nil {
		return nil, err
	}
	i.mu.Lock()
	i.handlers[registration] = wrapped
	i.mu.Unlock()
	for _, object := range i.initial {
		wrapped.OnAdd(object, true)
	}
	return registration, nil
}

func (i *replayableInformer) RemoveEventHandler(registration toolscache.ResourceEventHandlerRegistration) error {
	i.mu.Lock()
	if handler, ok := i.handlers[registration]; ok {
		handler.active.Store(false)
		delete(i.handlers, registration)
	}
	i.mu.Unlock()
	return i.FakeInformer.RemoveEventHandler(registration)
}

var _ cache.Informer = (*replayableInformer)(nil)
var _ cache.Cache = (*replayableMetricsCache)(nil)

func trainJob(name string, terminal bool) *trainer.TrainJob {
	job := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "team-a", UID: types.UID(name + "-uid"),
			CreationTimestamp: metav1.Now(),
		},
		Spec: trainer.TrainJobSpec{
			RuntimeRef: trainer.RuntimeRef{Name: "torch"},
			Trainer:    &trainer.Trainer{NumNodes: ptr.To(int32(2))},
		},
	}
	if terminal {
		job.Status.Conditions = []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}}
	}
	return job
}
