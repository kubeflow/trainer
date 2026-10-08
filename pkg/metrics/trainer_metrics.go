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

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
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
		Help:      "Accelerators requested per TrainJob training node.",
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

// trainerMetrics observes TrainJob events and records event-based metrics. It
// intentionally does not retain TrainJob state: counters and histograms are
// derived from add/update events and current object data.
type trainerMetrics struct {
	client      client.Client
	runtimeInfo *runtimeInfoCollector
}

func newTrainerMetrics(cli client.Client) *trainerMetrics {
	return &trainerMetrics{
		client:      cli,
		runtimeInfo: newRuntimeInfoCollector(cli),
	}
}

func (m *trainerMetrics) installObserver(c cache.Cache) error {
	informer, err := c.GetInformer(context.Background(), &trainer.TrainJob{}, cache.BlockUntilSynced(false))
	if err != nil {
		return fmt.Errorf("get TrainJob informer: %w", err)
	}
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerDetailedFuncs{
		AddFunc:    m.onAdd,
		UpdateFunc: m.onUpdate,
		DeleteFunc: m.onDelete,
	})
	if err != nil {
		return fmt.Errorf("register TrainJob metrics observer: %w", err)
	}
	return nil
}

func (m *trainerMetrics) onAdd(obj any, isInInitialList bool) {
	trainJob, ok := obj.(*trainer.TrainJob)
	if !ok || isInInitialList {
		return
	}
	runtimeRef := runtimeReference(trainJob)
	trainJobCreated.WithLabelValues(trainJob.Namespace, runtimeRef).Inc()
	m.observeRequestedWorkload(trainJob, runtimeRef)
	if isTerminal(trainJob) {
		m.observeTerminal(trainJob, runtimeRef)
	}
}

func (m *trainerMetrics) onUpdate(oldObj, newObj any) {
	oldJob, oldOK := oldObj.(*trainer.TrainJob)
	newJob, newOK := newObj.(*trainer.TrainJob)
	if !oldOK || !newOK || isTerminal(oldJob) || !isTerminal(newJob) {
		return
	}
	m.observeTerminal(newJob, runtimeReference(newJob))
}

func (m *trainerMetrics) onDelete(obj any) {
	// Deletions do not affect the event-based metrics currently exposed.
	_, _ = obj.(*trainer.TrainJob)
}

func (m *trainerMetrics) observeTerminal(trainJob *trainer.TrainJob, runtimeRef string) {
	condition := terminalCondition(trainJob)
	result := "succeeded"
	if condition.Type == trainer.TrainJobFailed {
		result = "failed"
	}
	reason := condition.Reason
	if reason == "" {
		reason = unknownValue
	}
	trainJobFinished.WithLabelValues(result, reason, runtimeRef).Inc()
	if !trainJob.CreationTimestamp.IsZero() && !condition.LastTransitionTime.IsZero() {
		trainJobLifetime.WithLabelValues(result, runtimeRef).Observe(
			condition.LastTransitionTime.Sub(trainJob.CreationTimestamp.Time).Seconds(),
		)
	}
}

func (m *trainerMetrics) observeRequestedWorkload(trainJob *trainer.TrainJob, runtimeRef string) {
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
		trainJobRequestedTrainingNodes.WithLabelValues(runtimeRef).Observe(float64(*trainJob.Spec.Trainer.NumNodes))
	} else if nodes := m.requestedNodes(trainJob); nodes >= 0 {
		trainJobRequestedTrainingNodes.WithLabelValues(runtimeRef).Observe(float64(nodes))
	}

	resources := m.resourcesPerNode(trainJob)
	observedAccelerator := false
	for resourceName, quantity := range acceleratorResources(resources) {
		trainJobRequestedAccelerators.WithLabelValues(string(resourceName), runtimeRef).Observe(float64(quantity.Value()))
		observedAccelerator = true
	}
	if !observedAccelerator {
		trainJobRequestedAccelerators.WithLabelValues("none", runtimeRef).Observe(0)
	}
}

func (m *trainerMetrics) runtimeSpec(trainJob *trainer.TrainJob) (*trainer.TrainingRuntimeSpec, error) {
	if ptr.Deref(trainJob.Spec.RuntimeRef.Kind, clusterTrainingRuntimeKind) == trainingRuntimeKind {
		runtime := &trainer.TrainingRuntime{}
		if err := m.client.Get(context.Background(), client.ObjectKey{Namespace: trainJob.Namespace, Name: trainJob.Spec.RuntimeRef.Name}, runtime); err != nil {
			return nil, err
		}
		return &runtime.Spec, nil
	}
	runtime := &trainer.ClusterTrainingRuntime{}
	if err := m.client.Get(context.Background(), client.ObjectKey{Name: trainJob.Spec.RuntimeRef.Name}, runtime); err != nil {
		return nil, err
	}
	return &runtime.Spec, nil
}

func (m *trainerMetrics) requestedNodes(trainJob *trainer.TrainJob) int32 {
	if spec, err := m.runtimeSpec(trainJob); err == nil && spec.MLPolicy != nil && spec.MLPolicy.NumNodes != nil {
		return *spec.MLPolicy.NumNodes
	}
	// MLPolicy.NumNodes defaults to one node in the Trainer API.
	return 1
}

func (m *trainerMetrics) resourcesPerNode(trainJob *trainer.TrainJob) *corev1.ResourceRequirements {
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.ResourcesPerNode != nil {
		return trainJob.Spec.Trainer.ResourcesPerNode
	}
	spec, err := m.runtimeSpec(trainJob)
	if err != nil {
		return nil
	}
	for _, replicatedJob := range spec.Template.Spec.ReplicatedJobs {
		if replicatedJob.Name != "node" && replicatedJob.Template.Labels["trainer.kubeflow.org/trainjob-ancestor-step"] != "trainer" {
			continue
		}
		for _, container := range replicatedJob.Template.Spec.Template.Spec.Containers {
			if container.Name == "node" {
				resources := container.Resources
				return &resources
			}
		}
	}
	return nil
}

func acceleratorResources(resources *corev1.ResourceRequirements) corev1.ResourceList {
	result := corev1.ResourceList{}
	if resources == nil {
		return result
	}
	for name, quantity := range resources.Requests {
		if strings.Contains(strings.ToLower(name.String()), "gpu") {
			result[name] = quantity
		}
	}
	if len(result) == 0 {
		for name, quantity := range resources.Limits {
			if strings.Contains(strings.ToLower(name.String()), "gpu") {
				result[name] = quantity
			}
		}
	}
	return result
}

// runtimeInfoCollector derives runtime inventory from the cache-backed client
// at scrape time and therefore does not retain a second runtime inventory.
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
			[]string{"namespace", "name", "api_group", "kind", "ml_policy", "pod_group_policy"}, nil,
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
				runtime.Namespace, runtime.Name, trainer.GroupVersion.Group,
				trainingRuntimeKind, mlPolicyName(runtime.Spec.MLPolicy), podGroupPolicyName(runtime.Spec.PodGroupPolicy))
		}
	}

	var clusterRuntimes trainer.ClusterTrainingRuntimeList
	if err := c.client.List(context.Background(), &clusterRuntimes); err == nil {
		for i := range clusterRuntimes.Items {
			runtime := &clusterRuntimes.Items[i]
			ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1,
				"", runtime.Name, trainer.GroupVersion.Group,
				clusterTrainingRuntimeKind, mlPolicyName(runtime.Spec.MLPolicy), podGroupPolicyName(runtime.Spec.PodGroupPolicy))
		}
	}
}

func setupTrainerMetrics(mgr ctrl.Manager) error {
	m := newTrainerMetrics(mgr.GetClient())
	return m.installObserver(mgr.GetCache())
}

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

func runtimeReference(trainJob *trainer.TrainJob) string {
	group := ptr.Deref(trainJob.Spec.RuntimeRef.APIGroup, trainer.GroupVersion.Group)
	kind := ptr.Deref(trainJob.Spec.RuntimeRef.Kind, clusterTrainingRuntimeKind)
	namespace := ""
	if kind == trainingRuntimeKind {
		namespace = trainJob.Namespace
	}
	return fmt.Sprintf("%s/%s/%s/%s", group, kind, namespace, trainJob.Spec.RuntimeRef.Name)
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
		return unknownValue
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
		return unknownValue
	}
}
