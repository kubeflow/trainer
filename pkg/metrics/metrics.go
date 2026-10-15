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
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricNamespace = "kubeflow_trainer"
	unknownValue    = "unknown"

	trainingRuntimeKind        = "TrainingRuntime"
	clusterTrainingRuntimeKind = "ClusterTrainingRuntime"
)

var (
	trainJobCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_created_total",
		Help:      "Number of TrainJobs observed after the initial cache listing.",
	}, []string{"namespace", "runtime_ref"})
	trainJobComplete = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_complete_total",
		Help:      "Number of TrainJobs that completed successfully.",
	}, []string{"runtime_ref"})
	trainJobFailed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_failed_total",
		Help:      "Number of TrainJobs that failed.",
	}, []string{"reason", "runtime_ref"})
	trainJobLifetime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_lifetime_seconds",
		Help:      "Time from TrainJob creation to its terminal outcome.",
		Buckets:   []float64{600, 3600, 21600, 43200, 86400, 345600, 604800, 2419200},
	}, []string{"result", "runtime_ref"})
	trainJobRequestedTrainingNodes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_requested_training_nodes_per_job",
		Help:      "Training nodes requested by each TrainJob.",
		Buckets:   []float64{1, 2, 4, 8, 16, 32, 64, 128},
	}, []string{"runtime_ref"})
)

var runtimeInfoDesc = prometheus.NewDesc(
	prometheus.BuildFQName(metricNamespace, "", "runtime_info"),
	"Current TrainingRuntime and ClusterTrainingRuntime resources.",
	[]string{"namespace", "name", "api_group", "kind", "ml_policy", "pod_group_policy"}, nil,
)
