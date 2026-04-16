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
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/utils/ptr"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

const (
	metricsNamespace = "kubeflow"
	metricsSubsystem = "trainer"
)

// durationBuckets matches Kueue's bucket policy: covers tiny CI jobs through long-running fine-tunes.
var durationBuckets = []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600, 10800, 21600}

var (
	// BuildInfo surfaces build metadata as a Gauge always set to 1.
	BuildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "build_info",
		Help:      "Build metadata for the Kubeflow Trainer controller manager (always 1).",
	}, []string{"git_version", "git_commit", "build_date", "go_version", "compiler", "platform"})

	// TrainJobsCreatedTotal is a counter for the total number of TrainJobs created.
	TrainJobsCreatedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "trainjobs_created_total",
		Help:      "Total number of TrainJobs created.",
	}, []string{"namespace", "runtime"})

	// TrainJobsCompletedTotal is a counter for TrainJobs that completed successfully.
	TrainJobsCompletedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "trainjobs_completed_total",
		Help:      "Total number of TrainJobs that completed successfully.",
	}, []string{"namespace", "runtime"})

	// TrainJobsFailedTotal is a counter for TrainJobs that failed.
	// The reason label maps to the TrainJob Failed condition reason.
	TrainJobsFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "trainjobs_failed_total",
		Help:      "Total number of TrainJobs that failed.",
	}, []string{"namespace", "runtime", "reason"})

	// TrainJobsSuspendedTotal is a counter for TrainJob suspension events.
	TrainJobsSuspendedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "trainjobs_suspended_total",
		Help:      "Total number of TrainJob suspension events.",
	}, []string{"namespace", "runtime"})

	// TrainJobsDeletedTotal is a counter for TrainJobs that have been deleted.
	TrainJobsDeletedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "trainjobs_deleted_total",
		Help:      "Total number of TrainJobs deleted.",
	}, []string{"namespace", "runtime"})

	// TrainJobsActive is a gauge tracking TrainJobs present in the cluster.
	// It resets on controller restart; incremented on Create events, decremented on Delete events.
	TrainJobsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "trainjobs_active",
		Help:      "Number of TrainJobs currently present in the cluster (resets on controller restart).",
	}, []string{"namespace", "runtime"})

	// TrainJobDurationSeconds is a histogram of TrainJob end-to-end duration from creation to terminal condition.
	TrainJobDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "trainjob_duration_seconds",
		Help:      "End-to-end duration of TrainJobs from creation to terminal condition.",
		Buckets:   durationBuckets,
	}, []string{"namespace", "runtime", "result"})

	// ReconcileDurationSeconds is a histogram of reconcile iteration latencies.
	ReconcileDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "reconcile_duration_seconds",
		Help:      "Latency of reconcile iterations per controller.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"controller", "result"})

	// PluginExecutionDurationSeconds is a histogram of plugin execution latencies.
	PluginExecutionDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "plugin_execution_duration_seconds",
		Help:      "Latency of plugin execution per plugin and phase.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"plugin", "phase"})

	// PluginExecutionErrorsTotal is a counter for plugin execution errors.
	PluginExecutionErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "plugin_execution_errors_total",
		Help:      "Total number of errors encountered during plugin execution.",
	}, []string{"plugin", "phase"})

	// RuntimesRegistered is a gauge for the number of registered training runtimes by kind.
	RuntimesRegistered = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "runtimes_registered",
		Help:      "Number of registered training runtimes by kind.",
	}, []string{"kind"})

	// WebhookValidationTotal is a counter for TrainJob webhook validation calls.
	WebhookValidationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "webhook_validation_total",
		Help:      "Total number of webhook validation calls by resource, operation, and result.",
	}, []string{"resource", "operation", "result"})
)

var registerOnce sync.Once

// Register registers all Kubeflow Trainer metrics with the controller-runtime registry.
// It is idempotent and safe to call multiple times; only the first call takes effect.
func Register() {
	registerOnce.Do(func() {
		ctrlmetrics.Registry.MustRegister(
			BuildInfo,
			TrainJobsCreatedTotal,
			TrainJobsCompletedTotal,
			TrainJobsFailedTotal,
			TrainJobsSuspendedTotal,
			TrainJobsDeletedTotal,
			TrainJobsActive,
			TrainJobDurationSeconds,
			ReconcileDurationSeconds,
			PluginExecutionDurationSeconds,
			PluginExecutionErrorsTotal,
			RuntimesRegistered,
			WebhookValidationTotal,
		)
	})
}

// RuntimeKind extracts the runtime kind label from a TrainJob's RuntimeRef.
// Returns "Unknown" if the Kind field is nil.
func RuntimeKind(trainJob *trainer.TrainJob) string {
	return ptr.Deref(trainJob.Spec.RuntimeRef.Kind, "Unknown")
}

// RecordTrainJobCreated increments the created counter and active gauge.
func RecordTrainJobCreated(namespace, runtimeKind string) {
	TrainJobsCreatedTotal.WithLabelValues(namespace, runtimeKind).Inc()
	TrainJobsActive.WithLabelValues(namespace, runtimeKind).Inc()
}

// RecordTrainJobDeleted increments the deleted counter and decrements the active gauge.
func RecordTrainJobDeleted(namespace, runtimeKind string) {
	TrainJobsDeletedTotal.WithLabelValues(namespace, runtimeKind).Inc()
	TrainJobsActive.WithLabelValues(namespace, runtimeKind).Dec()
}

// RecordTrainJobCompleted increments the completed counter and observes the duration histogram.
func RecordTrainJobCompleted(namespace, runtimeKind string, dur time.Duration) {
	TrainJobsCompletedTotal.WithLabelValues(namespace, runtimeKind).Inc()
	TrainJobDurationSeconds.WithLabelValues(namespace, runtimeKind, "Complete").Observe(dur.Seconds())
}

// RecordTrainJobFailed increments the failed counter and observes the duration histogram.
func RecordTrainJobFailed(namespace, runtimeKind, reason string, dur time.Duration) {
	TrainJobsFailedTotal.WithLabelValues(namespace, runtimeKind, reason).Inc()
	TrainJobDurationSeconds.WithLabelValues(namespace, runtimeKind, "Failed").Observe(dur.Seconds())
}

// RecordTrainJobSuspended increments the suspended counter.
func RecordTrainJobSuspended(namespace, runtimeKind string) {
	TrainJobsSuspendedTotal.WithLabelValues(namespace, runtimeKind).Inc()
}

// ObserveReconcile records the duration of a reconcile iteration.
func ObserveReconcile(controller, result string, dur time.Duration) {
	ReconcileDurationSeconds.WithLabelValues(controller, result).Observe(dur.Seconds())
}

// ObservePlugin records the latency of a plugin execution phase and increments the
// error counter if err is non-nil.
func ObservePlugin(pluginName, phase string, dur time.Duration, err error) {
	PluginExecutionDurationSeconds.WithLabelValues(pluginName, phase).Observe(dur.Seconds())
	if err != nil {
		PluginExecutionErrorsTotal.WithLabelValues(pluginName, phase).Inc()
	}
}
