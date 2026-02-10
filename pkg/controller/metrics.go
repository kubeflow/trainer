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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ttlDeletionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "trainjob_ttl_deletions_total",
			Help: "Total number of TrainJobs deleted due to TTL expiration",
		},
		[]string{"namespace"},
	)
	deadlineExceededTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "trainjob_deadline_exceeded_total",
			Help: "Total number of TrainJobs that exceeded their activeDeadlineSeconds",
		},
		[]string{"namespace"},
	)
)

func init() {
	metrics.Registry.MustRegister(ttlDeletionsTotal, deadlineExceededTotal)
}
