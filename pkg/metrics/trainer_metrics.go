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
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

const (
	metricNamespace = "kubeflow_trainer"

	unknownValue = "unknown"

	trainingRuntimeKind        = "TrainingRuntime"
	clusterTrainingRuntimeKind = "ClusterTrainingRuntime"

	kueueQueueLabel = "kueue.x-k8s.io/queue-name"
)

// trainerMetrics owns Trainer-specific metrics. Dependency-owned metrics, such as
// JobSet and Kueue metrics, are intentionally not duplicated here.
type trainerMetrics struct {
	client client.Client

	runtimeInfo            *runtimeInfoCollector
	trainJobInfo           *prometheus.GaugeVec
	trainJobStatus         *prometheus.GaugeVec
	requestedTrainingNodes *prometheus.GaugeVec
	replicatedJobCount     *prometheus.GaugeVec
	replicatedJobsReady    *prometheus.GaugeVec
	replicatedJobsFailed   *prometheus.GaugeVec

	created          *prometheus.CounterVec
	finished         *prometheus.CounterVec
	lifetime         *prometheus.HistogramVec
	suspendEvents    *prometheus.CounterVec
	suspendedSeconds *prometheus.HistogramVec

	mu   sync.Mutex
	seen map[string]observedTrainJob
}

type observedTrainJob struct {
	infoLabels           []string
	statusLabels         []string
	requestedNodesLabels []string
	requestedNodes       float64
	terminal             bool
	suspended            bool
	suspendedAt          time.Time
}

func newTrainerMetrics(cli client.Client) *trainerMetrics {
	return &trainerMetrics{
		client:      cli,
		runtimeInfo: newRuntimeInfoCollector(cli),
		trainJobInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_info",
			Help:      "Current TrainJob metadata for local Prometheus joins.",
		}, []string{"namespace", "trainjob", "runtime_ref", "ml_policy", "pod_group_policy", "kueue_queue_label_present"}),
		trainJobStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_status",
			Help:      "Current normalized TrainJob lifecycle state.",
		}, []string{"namespace", "trainjob", "status"}),
		requestedTrainingNodes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_requested_training_nodes",
			Help:      "Logical training nodes requested by the current TrainJob.",
		}, []string{"namespace", "trainjob"}),
		replicatedJobCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_replicated_job_count",
			Help:      "Number of replicated jobs in the current TrainJob status.",
		}, []string{"namespace", "trainjob", "role"}),
		replicatedJobsReady: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_replicated_jobs_ready",
			Help:      "Ready replicated jobs in the current TrainJob status.",
		}, []string{"namespace", "trainjob", "role"}),
		replicatedJobsFailed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_replicated_jobs_failed",
			Help:      "Failed replicated jobs in the current TrainJob status.",
		}, []string{"namespace", "trainjob", "role"}),
		created: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_created_total",
			Help:      "TrainJobs observed by the controller metrics observer.",
		}, []string{"ml_policy", "pod_group_policy", "kueue_queue_label_present"}),
		finished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_finished_total",
			Help:      "TrainJobs observed reaching a terminal outcome.",
		}, []string{"result", "reason"}),
		lifetime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_lifetime_seconds",
			Help:      "Time from TrainJob creation to its terminal condition.",
			Buckets:   prometheus.ExponentialBuckets(60, 4, 8),
		}, []string{"result"}),
		suspendEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_suspend_events_total",
			Help:      "Observed TrainJob suspension and resumption transitions.",
		}, []string{"event"}),
		suspendedSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "trainjob_suspended_seconds",
			Help:      "Time a TrainJob remained suspended before resuming.",
			Buckets:   prometheus.ExponentialBuckets(1, 4, 8),
		}, nil),
		seen: make(map[string]observedTrainJob),
	}
}

func (m *trainerMetrics) installObserver(ctx context.Context, c cache.Cache) error {
	informer, err := c.GetInformer(ctx, &trainer.TrainJob{})
	if err != nil {
		return fmt.Errorf("get TrainJob informer: %w", err)
	}
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    m.onAdd,
		UpdateFunc: m.onUpdate,
		DeleteFunc: m.onDelete,
	})
	if err != nil {
		return fmt.Errorf("register TrainJob metrics observer: %w", err)
	}
	return nil
}

func (m *trainerMetrics) onAdd(obj any) {
	trainJob, ok := obj.(*trainer.TrainJob)
	if !ok {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(trainJob.UID)
	if key == "" {
		key = trainJob.Namespace + "/" + trainJob.Name
	}
	if _, exists := m.seen[key]; exists {
		return
	}
	state := m.observeCurrentLocked(trainJob)
	if state.suspended {
		state.suspendedAt = time.Now()
	}
	m.seen[key] = state
	m.created.WithLabelValues(policyLabels(trainJob, m)...).Inc()
	if state.terminal {
		m.observeTerminalLocked(trainJob)
	}
}

func (m *trainerMetrics) onUpdate(oldObj, newObj any) {
	oldJob, oldOK := oldObj.(*trainer.TrainJob)
	newJob, newOK := newObj.(*trainer.TrainJob)
	if !oldOK || !newOK {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(newJob.UID)
	if key == "" {
		key = newJob.Namespace + "/" + newJob.Name
	}
	previous, exists := m.seen[key]
	if !exists {
		previous = m.observeCurrentLocked(oldJob)
	}
	if previous.infoLabels != nil {
		m.trainJobInfo.DeleteLabelValues(previous.infoLabels...)
		m.trainJobStatus.DeleteLabelValues(previous.statusLabels...)
		m.requestedTrainingNodes.DeleteLabelValues(previous.requestedNodesLabels...)
	}
	m.observeCurrentLocked(newJob)
	if !previous.terminal && isTerminal(newJob) {
		m.observeTerminalLocked(newJob)
	}
	previousSuspended := previous.suspended
	currentSuspended := isSuspended(newJob)
	if previousSuspended != currentSuspended {
		if currentSuspended {
			m.suspendEvents.WithLabelValues("suspend").Inc()
		} else {
			m.suspendEvents.WithLabelValues("resume").Inc()
			if !previous.suspendedAt.IsZero() {
				m.suspendedSeconds.WithLabelValues().Observe(time.Since(previous.suspendedAt).Seconds())
			}
		}
	}
	state := m.currentState(newJob)
	if currentSuspended && previous.suspendedAt.IsZero() {
		state.suspendedAt = time.Now()
	}
	m.seen[key] = state
}

func (m *trainerMetrics) onDelete(obj any) {
	trainJob, ok := obj.(*trainer.TrainJob)
	if !ok {
		if tombstone, tombstoneOK := obj.(toolscache.DeletedFinalStateUnknown); tombstoneOK {
			trainJob, _ = tombstone.Obj.(*trainer.TrainJob)
		}
	}
	if trainJob == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(trainJob.UID)
	if key == "" {
		key = trainJob.Namespace + "/" + trainJob.Name
	}
	if state, exists := m.seen[key]; exists {
		m.trainJobInfo.DeleteLabelValues(state.infoLabels...)
		m.trainJobStatus.DeleteLabelValues(state.statusLabels...)
		m.requestedTrainingNodes.DeleteLabelValues(state.requestedNodesLabels...)
		m.deleteReplicatedJobMetrics(trainJob.Namespace, trainJob.Name)
		delete(m.seen, key)
	}
}

func (m *trainerMetrics) observeCurrentLocked(trainJob *trainer.TrainJob) observedTrainJob {
	state := m.currentState(trainJob)
	m.trainJobInfo.WithLabelValues(state.infoLabels...).Set(1)
	m.trainJobStatus.WithLabelValues(state.statusLabels...).Set(1)
	if state.requestedNodesLabels != nil {
		m.requestedTrainingNodes.WithLabelValues(state.requestedNodesLabels...).Set(state.requestedNodes)
	}
	m.deleteReplicatedJobMetrics(trainJob.Namespace, trainJob.Name)
	for _, jobStatus := range trainJob.Status.JobsStatus {
		role := normalizeRole(jobStatus.Name)
		m.replicatedJobCount.WithLabelValues(trainJob.Namespace, trainJob.Name, role).Set(1)
		m.replicatedJobsReady.WithLabelValues(trainJob.Namespace, trainJob.Name, role).Set(float64(ptr.Deref(jobStatus.Ready, 0)))
		m.replicatedJobsFailed.WithLabelValues(trainJob.Namespace, trainJob.Name, role).Set(float64(ptr.Deref(jobStatus.Failed, 0)))
	}
	return state
}

func (m *trainerMetrics) observeTerminalLocked(trainJob *trainer.TrainJob) {
	condition := terminalCondition(trainJob)
	result := "succeeded"
	if condition.Type == trainer.TrainJobFailed {
		result = "failed"
	}
	reason := condition.Reason
	if result == "succeeded" || reason == "" {
		reason = "none"
	}
	if result == "failed" && reason == "" {
		reason = "other"
	}
	m.finished.WithLabelValues(result, reason).Inc()
	if !trainJob.CreationTimestamp.IsZero() && !condition.LastTransitionTime.IsZero() {
		m.lifetime.WithLabelValues(result).Observe(condition.LastTransitionTime.Sub(trainJob.CreationTimestamp.Time).Seconds())
	}
}

func (m *trainerMetrics) currentState(trainJob *trainer.TrainJob) observedTrainJob {
	info := runtimeReference(trainJob)
	mlPolicy, podGroupPolicy := m.runtimePolicies(trainJob)
	queuePresent := "no"
	if trainJob.Labels[kueueQueueLabel] != "" {
		queuePresent = "yes"
	}
	status := normalizedStatus(trainJob)
	state := observedTrainJob{
		infoLabels:   []string{trainJob.Namespace, trainJob.Name, info, mlPolicy, podGroupPolicy, queuePresent},
		statusLabels: []string{trainJob.Namespace, trainJob.Name, status},
		terminal:     isTerminal(trainJob),
		suspended:    isSuspended(trainJob),
	}
	if nodes := m.requestedNodes(trainJob); nodes >= 0 {
		state.requestedNodes = float64(nodes)
		state.requestedNodesLabels = []string{trainJob.Namespace, trainJob.Name}
	}
	return state
}

func (m *trainerMetrics) runtimePolicies(trainJob *trainer.TrainJob) (string, string) {
	var mlPolicy *trainer.MLPolicy
	var podGroupPolicy *trainer.PodGroupPolicy
	if ptr.Deref(trainJob.Spec.RuntimeRef.Kind, clusterTrainingRuntimeKind) == trainingRuntimeKind {
		runtime := &trainer.TrainingRuntime{}
		if err := m.client.Get(context.Background(), client.ObjectKey{Namespace: trainJob.Namespace, Name: trainJob.Spec.RuntimeRef.Name}, runtime); err == nil {
			mlPolicy, podGroupPolicy = runtime.Spec.MLPolicy, runtime.Spec.PodGroupPolicy
		}
	} else {
		runtime := &trainer.ClusterTrainingRuntime{}
		if err := m.client.Get(context.Background(), client.ObjectKey{Name: trainJob.Spec.RuntimeRef.Name}, runtime); err == nil {
			mlPolicy, podGroupPolicy = runtime.Spec.MLPolicy, runtime.Spec.PodGroupPolicy
		}
	}
	return mlPolicyName(mlPolicy), podGroupPolicyName(podGroupPolicy)
}

func (m *trainerMetrics) requestedNodes(trainJob *trainer.TrainJob) int32 {
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
		return *trainJob.Spec.Trainer.NumNodes
	}
	if ptr.Deref(trainJob.Spec.RuntimeRef.Kind, clusterTrainingRuntimeKind) == trainingRuntimeKind {
		runtime := &trainer.TrainingRuntime{}
		if err := m.client.Get(context.Background(), client.ObjectKey{Namespace: trainJob.Namespace, Name: trainJob.Spec.RuntimeRef.Name}, runtime); err == nil && runtime.Spec.MLPolicy != nil && runtime.Spec.MLPolicy.NumNodes != nil {
			return *runtime.Spec.MLPolicy.NumNodes
		}
	} else {
		runtime := &trainer.ClusterTrainingRuntime{}
		if err := m.client.Get(context.Background(), client.ObjectKey{Name: trainJob.Spec.RuntimeRef.Name}, runtime); err == nil && runtime.Spec.MLPolicy != nil && runtime.Spec.MLPolicy.NumNodes != nil {
			return *runtime.Spec.MLPolicy.NumNodes
		}
	}
	return -1
}

func (m *trainerMetrics) deleteReplicatedJobMetrics(namespace, name string) {
	for _, role := range []string{"dataset-initializer", "model-initializer", "trainer", "other"} {
		m.replicatedJobCount.DeleteLabelValues(namespace, name, role)
		m.replicatedJobsReady.DeleteLabelValues(namespace, name, role)
		m.replicatedJobsFailed.DeleteLabelValues(namespace, name, role)
	}
}

// runtimeInfoCollector derives the runtime inventory from the controller-runtime
// cache-backed client at scrape time. It does not maintain a second inventory or
// expose a historical runtime counter.
type runtimeInfoCollector struct {
	client client.Client
	desc   *prometheus.Desc
}

func newRuntimeInfoCollector(cli client.Client) *runtimeInfoCollector {
	return &runtimeInfoCollector{
		client: cli,
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(metricNamespace, "", "runtime_info"),
			"Current TrainingRuntime and ClusterTrainingRuntime resources.",
			[]string{"kind", "namespace", "name", "ml_policy", "pod_group_policy"}, nil,
		),
	}
}

func (c *runtimeInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *runtimeInfoCollector) Collect(ch chan<- prometheus.Metric) {
	var runtimes trainer.TrainingRuntimeList
	if err := c.client.List(context.Background(), &runtimes); err == nil {
		for i := range runtimes.Items {
			runtime := &runtimes.Items[i]
			ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1,
				trainingRuntimeKind, runtime.Namespace, runtime.Name,
				mlPolicyName(runtime.Spec.MLPolicy), podGroupPolicyName(runtime.Spec.PodGroupPolicy))
		}
	}

	var clusterRuntimes trainer.ClusterTrainingRuntimeList
	if err := c.client.List(context.Background(), &clusterRuntimes); err == nil {
		for i := range clusterRuntimes.Items {
			runtime := &clusterRuntimes.Items[i]
			ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1,
				clusterTrainingRuntimeKind, "", runtime.Name,
				mlPolicyName(runtime.Spec.MLPolicy), podGroupPolicyName(runtime.Spec.PodGroupPolicy))
		}
	}
}

func setupTrainerMetrics(mgr ctrl.Manager) error {
	m := newTrainerMetrics(mgr.GetClient())
	if err := registerTrainerCollectors(controllerMetrics.Registry, m); err != nil {
		return err
	}
	return m.installObserver(context.Background(), mgr.GetCache())
}

func registerTrainerCollectors(reg prometheus.Registerer, m *trainerMetrics) error {
	for _, collector := range []prometheus.Collector{
		m.runtimeInfo, m.trainJobInfo, m.trainJobStatus, m.requestedTrainingNodes,
		m.replicatedJobCount, m.replicatedJobsReady, m.replicatedJobsFailed,
		m.created, m.finished, m.lifetime, m.suspendEvents, m.suspendedSeconds,
	} {
		if err := reg.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

func runtimeReference(trainJob *trainer.TrainJob) string {
	group := ptr.Deref(trainJob.Spec.RuntimeRef.APIGroup, trainer.GroupVersion.Group)
	kind := ptr.Deref(trainJob.Spec.RuntimeRef.Kind, clusterTrainingRuntimeKind)
	namespace := ""
	if kind == trainingRuntimeKind {
		namespace = trainJob.Namespace
	}
	return fmt.Sprintf("%s/%s/%s/%s", group, kind, namespace, trainJob.Spec.RuntimeRef.Name)
}

func policyLabels(trainJob *trainer.TrainJob, m *trainerMetrics) []string {
	mlPolicy, podGroupPolicy := unknownValue, unknownValue
	if m != nil {
		mlPolicy, podGroupPolicy = m.runtimePolicies(trainJob)
	}
	queuePresent := "no"
	if trainJob.Labels[kueueQueueLabel] != "" {
		queuePresent = "yes"
	}
	return []string{mlPolicy, podGroupPolicy, queuePresent}
}

func normalizedStatus(trainJob *trainer.TrainJob) string {
	if meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobFailed) {
		return "failed"
	}
	if meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobComplete) {
		return "succeeded"
	}
	if isSuspended(trainJob) {
		return "suspended"
	}
	return "nonterminal"
}

func isTerminal(trainJob *trainer.TrainJob) bool {
	return meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobFailed) ||
		meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobComplete)
}

func terminalCondition(trainJob *trainer.TrainJob) metav1.Condition {
	if condition := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed); condition != nil && condition.Status == metav1.ConditionTrue {
		return *condition
	}
	if condition := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobComplete); condition != nil && condition.Status == metav1.ConditionTrue {
		return *condition
	}
	return metav1.Condition{}
}

func isSuspended(trainJob *trainer.TrainJob) bool {
	return ptr.Deref(trainJob.Spec.Suspend, false) || meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobSuspended)
}

func normalizeRole(name string) string {
	switch name {
	case "dataset-initializer", "model-initializer", "trainer":
		return name
	default:
		return "other"
	}
}

func mlPolicyName(policy *trainer.MLPolicy) string {
	if policy == nil {
		return "none"
	}
	switch {
	case policy.Torch != nil:
		return "torch"
	case policy.MPI != nil:
		return "mpi"
	case policy.Flux != nil:
		return "flux"
	case policy.JAX != nil:
		return "jax"
	case policy.XGBoost != nil:
		return "xgboost"
	default:
		return "none"
	}
}

func podGroupPolicyName(policy *trainer.PodGroupPolicy) string {
	if policy == nil {
		return "none"
	}
	switch {
	case policy.Coscheduling != nil:
		return "coscheduling"
	case policy.Volcano != nil:
		return "volcano"
	default:
		return "none"
	}
}
