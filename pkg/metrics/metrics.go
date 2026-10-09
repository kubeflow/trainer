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
	controllerMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
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
	trainJobFinished = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_finished_total",
		Help:      "Number of TrainJobs that reached a terminal outcome.",
	}, []string{"result", "reason", "runtime_ref"})
	trainJobLifetime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_lifetime_seconds",
		Help:      "Time from TrainJob creation to its terminal outcome.",
	}, []string{"result", "runtime_ref"})
	trainJobRequestedAccelerators = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_requested_accelerators",
		Help:      "Accelerators requested per TrainJob.",
	}, []string{"accelerator_class", "runtime_ref"})
	trainJobRequestedTrainingNodes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricNamespace,
		Name:      "trainjob_requested_training_nodes_per_job",
		Help:      "Training nodes requested by each TrainJob.",
	}, []string{"runtime_ref"})
)

func init() {
	controllerMetrics.Registry.MustRegister(
		trainJobCreated,
		trainJobFinished,
		trainJobLifetime,
		trainJobRequestedAccelerators,
		trainJobRequestedTrainingNodes,
	)
}

var runtimeInfoDesc = prometheus.NewDesc(
	prometheus.BuildFQName(metricNamespace, "", "runtime_info"),
	"Current TrainingRuntime and ClusterTrainingRuntime resources.",
	[]string{"namespace", "name", "api_group", "kind", "ml_policy", "pod_group_policy"}, nil,
)
