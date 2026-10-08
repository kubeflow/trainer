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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

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
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := newMetricsScheme(t)
			objects := make([]client.Object, 0, len(test.objects))
			for _, object := range test.objects {
				objects = append(objects, object.(client.Object))
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			reg := prometheus.NewRegistry()
			if err := reg.Register(newRuntimeInfoCollector(cli)); err != nil {
				t.Fatalf("register runtime collector: %v", err)
			}
			assertMetric(t, reg, test.want, "kubeflow_trainer_runtime_info")
		})
	}
}

func TestTrainJobAdd(t *testing.T) {
	tests := map[string]struct {
		initialList bool
		wantCreated float64
	}{
		"initial cache list": {initialList: true, wantCreated: 0},
		"new TrainJob":       {initialList: false, wantCreated: 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resetMetrics(t)
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
			job := trainJob("job-a", false)
			if test.initialList {
				m.onAdd(job, true)
			} else {
				informer.Add(job)
			}
			if got := testutil.ToFloat64(trainJobCreated.WithLabelValues(job.Namespace, runtimeReference(job))); got != test.wantCreated {
				t.Fatalf("created counter = %v, want %v", got, test.wantCreated)
			}
			if !test.initialList {
				reg := prometheus.NewRegistry()
				if err := registerTrainerCollectors(reg, m); err != nil {
					t.Fatalf("register Trainer metrics: %v", err)
				}
				assertHistogramObservation(t, reg, "kubeflow_trainer_trainjob_requested_training_nodes_per_job", map[string]string{
					"runtime_ref": runtimeReference(job),
				}, 1, 2)
			}
		})
	}
}

func TestTrainJobUpdate(t *testing.T) {
	tests := map[string]struct {
		terminalUpdate bool
		repeatUpdate   bool
		wantFinished   float64
	}{
		"nonterminal update":       {wantFinished: 0},
		"terminal transition":      {terminalUpdate: true, wantFinished: 1},
		"repeated terminal update": {terminalUpdate: true, repeatUpdate: true, wantFinished: 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resetMetrics(t)
			scheme := newMetricsScheme(t)
			m := newTrainerMetrics(fake.NewClientBuilder().WithScheme(scheme).Build())
			job := trainJob("job-a", false)
			m.onAdd(job, false)
			updated := job.DeepCopy()
			if test.terminalUpdate {
				updated.Status.Conditions = []metav1.Condition{{
					Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue, Reason: "Completed",
					LastTransitionTime: metav1.Now(),
				}}
			}
			m.onUpdate(job, updated)
			if test.repeatUpdate {
				m.onUpdate(updated, updated.DeepCopy())
			}
			runtimeRef := runtimeReference(job)
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
		"runtime_ref":       runtimeReference(job),
	}, 1, 2)
}

func resetMetrics(t *testing.T) {
	t.Helper()
	trainJobCreated.Reset()
	trainJobFinished.Reset()
	trainJobLifetime.Reset()
	trainJobRequestedAccelerators.Reset()
	trainJobRequestedTrainingNodes.Reset()
}

func assertMetric(t *testing.T, reg *prometheus.Registry, want, name string) {
	t.Helper()
	if want == "" {
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
