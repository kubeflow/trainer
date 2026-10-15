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
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	controllerMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	trainjobutil "github.com/kubeflow/trainer/v2/pkg/util/trainjob"
)

var (
	_ manager.Runnable               = (*trainerMetrics)(nil)
	_ manager.LeaderElectionRunnable = (*trainerMetrics)(nil)
	_ manager.Runnable               = (*runtimeInfoCollector)(nil)
	_ manager.LeaderElectionRunnable = (*runtimeInfoCollector)(nil)
)

// trainerMetrics observes TrainJob events and records event-based metrics.
// It is run only by the elected controller replica.
type trainerMetrics struct {
	cache       cache.Cache
	runtimeInfo *runtimeInfoCollector
	log         logr.Logger
	ready       atomic.Bool
}

func newTrainerMetrics(c cache.Cache) *trainerMetrics {
	return &trainerMetrics{
		cache:       c,
		log:         ctrl.Log.WithName("trainer-metrics"),
		runtimeInfo: newRuntimeInfoCollector(c),
	}
}

// NeedLeaderElection makes lifecycle metrics leader-owned. The metrics server
// itself remains available on every replica.
func (m *trainerMetrics) NeedLeaderElection() bool {
	return true
}

// Start registers the TrainJob observer for the lifetime of this leadership
// term. ready becomes true after handler synchronization so initial-list
// objects are not counted as newly created TrainJobs.
func (m *trainerMetrics) Start(ctx context.Context) error {
	if m.cache == nil {
		return fmt.Errorf("metrics cache is required")
	}
	if err := m.runtimeInfo.waitForSync(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("wait for runtime metrics snapshot: %w", err)
	}
	if err := registerLeaderMetrics(m.runtimeInfo); err != nil {
		return err
	}
	defer unregisterLeaderMetrics(m.runtimeInfo)
	m.ready.Store(false)

	informer, err := m.cache.GetInformer(ctx, &trainer.TrainJob{})
	if err != nil {
		return fmt.Errorf("get TrainJob informer: %w", err)
	}
	registration, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    m.onAdd,
		UpdateFunc: m.onUpdate,
	})
	if err != nil {
		return fmt.Errorf("register TrainJob metrics observer: %w", err)
	}
	defer func() {
		m.ready.Store(false)
		if removeErr := informer.RemoveEventHandler(registration); removeErr != nil {
			m.log.Error(removeErr, "failed to remove TrainJob metrics observer")
		}
	}()

	if err := waitForHandlerSync(ctx, registration); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("wait for TrainJob metrics observer: %w", err)
	}
	m.ready.Store(true)
	<-ctx.Done()
	return nil
}

func waitForHandlerSync(ctx context.Context, registrations ...toolscache.ResourceEventHandlerRegistration) error {
	return wait.PollUntilContextCancel(ctx, 10*time.Millisecond, true, func(context.Context) (bool, error) {
		for _, registration := range registrations {
			if !registration.HasSynced() {
				return false, nil
			}
		}
		return true, nil
	})
}

func (m *trainerMetrics) onAdd(obj any) {
	if !m.ready.Load() {
		return
	}
	trainJob, ok := obj.(*trainer.TrainJob)
	if !ok {
		return
	}
	runtimeRef := runtimeRefLabel(trainJob)
	trainJobCreated.WithLabelValues(trainJob.Namespace, runtimeRef).Inc()
	m.observeRequestedWorkload(trainJob, runtimeRef)
}

func (m *trainerMetrics) onUpdate(oldObj, newObj any) {
	if !m.ready.Load() {
		return
	}
	oldJob, oldOK := oldObj.(*trainer.TrainJob)
	newJob, newOK := newObj.(*trainer.TrainJob)
	if !oldOK || !newOK || trainjobutil.IsTrainJobFinished(oldJob) || !trainjobutil.IsTrainJobFinished(newJob) {
		return
	}
	m.observeTerminal(newJob, runtimeRefLabel(newJob))
}

func (m *trainerMetrics) observeTerminal(trainJob *trainer.TrainJob, runtimeRef string) {
	condition := terminalCondition(trainJob)
	if condition.Type == trainer.TrainJobFailed {
		trainJobFailed.WithLabelValues(terminalReason(condition), runtimeRef).Inc()
		m.observeLifetime(trainJob, condition, "failed", runtimeRef)
		return
	}
	trainJobComplete.WithLabelValues(runtimeRef).Inc()
	m.observeLifetime(trainJob, condition, "succeeded", runtimeRef)
}

func (m *trainerMetrics) observeLifetime(trainJob *trainer.TrainJob, condition metav1.Condition, result, runtimeRef string) {
	if trainJob.CreationTimestamp.IsZero() || condition.LastTransitionTime.IsZero() {
		return
	}
	lifetime := condition.LastTransitionTime.Sub(trainJob.CreationTimestamp.Time).Seconds()
	if lifetime < 0 {
		m.log.Error(fmt.Errorf("negative lifetime %v", lifetime), "skipping TrainJob lifetime metric", "namespace", trainJob.Namespace, "name", trainJob.Name)
		return
	}
	trainJobLifetime.WithLabelValues(result, runtimeRef).Observe(lifetime)
}

func (m *trainerMetrics) observeRequestedWorkload(trainJob *trainer.TrainJob, runtimeRef string) {
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
		if *trainJob.Spec.Trainer.NumNodes < 0 {
			m.log.Info("skipping requested training nodes metric because TrainJob numNodes is invalid", "namespace", trainJob.Namespace, "name", trainJob.Name, "runtimeRef", runtimeRef, "numNodes", *trainJob.Spec.Trainer.NumNodes)
			return
		}
		trainJobRequestedTrainingNodes.WithLabelValues(runtimeRef).Observe(float64(*trainJob.Spec.Trainer.NumNodes))
		return
	}

	nodes, ok := m.runtimeInfo.requestedNodes(trainJob)
	if !ok {
		m.log.Info("skipping requested training nodes metric because runtime is not available in the informer snapshot", "namespace", trainJob.Namespace, "name", trainJob.Name, "runtimeRef", runtimeRef)
		return
	}
	trainJobRequestedTrainingNodes.WithLabelValues(runtimeRef).Observe(float64(nodes))
}

// runtimeInfoCollector maintains current runtime inventory from informer
// events. Collect only reads the in-memory snapshot and never calls the API.
type runtimeInfoCollector struct {
	cache cache.Cache

	mu       sync.RWMutex
	snapshot map[string]runtimeInfo
	ready    atomic.Bool
}

type runtimeInfo struct {
	namespace      string
	name           string
	kind           string
	mlPolicy       string
	podGroupPolicy string
	requestedNodes int32
	hasNodeCount   bool
}

func newRuntimeInfoCollector(c cache.Cache) *runtimeInfoCollector {
	return &runtimeInfoCollector{
		cache:    c,
		snapshot: make(map[string]runtimeInfo),
	}
}

// NeedLeaderElection ensures runtime inventory is exposed only by the elected
// controller replica, alongside the event-based TrainJob metrics.
func (c *runtimeInfoCollector) NeedLeaderElection() bool {
	return true
}

func (c *runtimeInfoCollector) Start(ctx context.Context) error {
	if c.cache == nil {
		return fmt.Errorf("metrics cache is required")
	}
	c.ready.Store(false)
	defer func() {
		c.ready.Store(false)
		c.mu.Lock()
		c.snapshot = make(map[string]runtimeInfo)
		c.mu.Unlock()
	}()
	trainingInformer, err := c.cache.GetInformer(ctx, &trainer.TrainingRuntime{})
	if err != nil {
		return fmt.Errorf("get TrainingRuntime informer: %w", err)
	}
	clusterInformer, err := c.cache.GetInformer(ctx, &trainer.ClusterTrainingRuntime{})
	if err != nil {
		return fmt.Errorf("get ClusterTrainingRuntime informer: %w", err)
	}

	trainingRegistration, err := trainingInformer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.upsert(obj, trainingRuntimeKind) },
		UpdateFunc: func(_, obj any) { c.upsert(obj, trainingRuntimeKind) },
		DeleteFunc: func(obj any) { c.delete(obj, trainingRuntimeKind) },
	})
	if err != nil {
		return fmt.Errorf("register TrainingRuntime metrics observer: %w", err)
	}
	defer func() {
		if removeErr := trainingInformer.RemoveEventHandler(trainingRegistration); removeErr != nil {
			ctrl.Log.WithName("trainer-metrics").Error(removeErr, "failed to remove TrainingRuntime metrics observer")
		}
	}()

	clusterRegistration, err := clusterInformer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.upsert(obj, clusterTrainingRuntimeKind) },
		UpdateFunc: func(_, obj any) { c.upsert(obj, clusterTrainingRuntimeKind) },
		DeleteFunc: func(obj any) { c.delete(obj, clusterTrainingRuntimeKind) },
	})
	if err != nil {
		return fmt.Errorf("register ClusterTrainingRuntime metrics observer: %w", err)
	}
	defer func() {
		if removeErr := clusterInformer.RemoveEventHandler(clusterRegistration); removeErr != nil {
			ctrl.Log.WithName("trainer-metrics").Error(removeErr, "failed to remove ClusterTrainingRuntime metrics observer")
		}
	}()

	if err := waitForHandlerSync(ctx, trainingRegistration, clusterRegistration); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("wait for runtime metrics observers: %w", err)
	}
	c.ready.Store(true)
	<-ctx.Done()
	return nil
}

func (c *runtimeInfoCollector) waitForSync(ctx context.Context) error {
	return wait.PollUntilContextCancel(ctx, 10*time.Millisecond, true, func(context.Context) (bool, error) {
		return c.ready.Load(), nil
	})
}

func (c *runtimeInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- runtimeInfoDesc
}

func (c *runtimeInfoCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	values := make([]runtimeInfo, 0, len(c.snapshot))
	for _, info := range c.snapshot {
		values = append(values, info)
	}
	c.mu.RUnlock()

	for _, info := range values {
		ch <- prometheus.MustNewConstMetric(runtimeInfoDesc, prometheus.GaugeValue, 1,
			info.namespace, info.name, trainer.GroupVersion.Group, info.kind, info.mlPolicy, info.podGroupPolicy)
	}
}

func (c *runtimeInfoCollector) requestedNodes(trainJob *trainer.TrainJob) (int32, bool) {
	kind := ptr.Deref(trainJob.Spec.RuntimeRef.Kind, clusterTrainingRuntimeKind)
	namespace := trainJob.Namespace
	if kind == clusterTrainingRuntimeKind {
		namespace = ""
	}
	key := runtimeInfoKey(kind, namespace, trainJob.Spec.RuntimeRef.Name)

	c.mu.RLock()
	info, ok := c.snapshot[key]
	c.mu.RUnlock()
	if !ok || !info.hasNodeCount {
		return 0, false
	}
	return info.requestedNodes, true
}

func (c *runtimeInfoCollector) upsert(obj any, kind string) {
	info, key, ok := runtimeInfoFromObject(obj, kind)
	if !ok {
		return
	}
	c.mu.Lock()
	c.snapshot[key] = info
	c.mu.Unlock()
}

func (c *runtimeInfoCollector) delete(obj any, kind string) {
	_, key, ok := runtimeInfoFromObject(obj, kind)
	if !ok {
		return
	}
	c.mu.Lock()
	delete(c.snapshot, key)
	c.mu.Unlock()
}

func runtimeInfoFromObject(obj any, kind string) (runtimeInfo, string, bool) {
	object, ok := obj.(client.Object)
	if !ok {
		if tombstone, tombstoneOK := obj.(toolscache.DeletedFinalStateUnknown); tombstoneOK {
			object, ok = tombstone.Obj.(client.Object)
		}
	}
	if !ok {
		return runtimeInfo{}, "", false
	}
	info := runtimeInfo{
		namespace:      object.GetNamespace(),
		name:           object.GetName(),
		kind:           kind,
		mlPolicy:       "none",
		podGroupPolicy: "none",
	}
	switch runtime := object.(type) {
	case *trainer.TrainingRuntime:
		info.mlPolicy = mlPolicyName(runtime.Spec.MLPolicy)
		info.podGroupPolicy = podGroupPolicyName(runtime.Spec.PodGroupPolicy)
		info.requestedNodes, info.hasNodeCount = requestedNodesFromSpec(&runtime.Spec)
	case *trainer.ClusterTrainingRuntime:
		info.namespace = ""
		info.mlPolicy = mlPolicyName(runtime.Spec.MLPolicy)
		info.podGroupPolicy = podGroupPolicyName(runtime.Spec.PodGroupPolicy)
		info.requestedNodes, info.hasNodeCount = requestedNodesFromSpec(&runtime.Spec)
	default:
		return runtimeInfo{}, "", false
	}
	return info, runtimeInfoKey(kind, info.namespace, info.name), true
}

func runtimeInfoKey(kind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

func requestedNodesFromSpec(spec *trainer.TrainingRuntimeSpec) (int32, bool) {
	if spec.MLPolicy != nil {
		if spec.MLPolicy.NumNodes == nil {
			return 1, true
		}
		return validRequestedNodes(*spec.MLPolicy.NumNodes)
	}

	for _, replicatedJob := range spec.Template.Spec.ReplicatedJobs {
		if replicatedJob.Template.Labels[constants.LabelTrainJobAncestor] != constants.AncestorTrainer {
			continue
		}
		if replicatedJob.Template.Spec.Parallelism == nil {
			return 1, true
		}
		return validRequestedNodes(*replicatedJob.Template.Spec.Parallelism)
	}

	return 1, true
}

func validRequestedNodes(nodes int32) (int32, bool) {
	if nodes < 0 {
		return 0, false
	}
	return nodes, true
}

func setupTrainerMetrics(mgr ctrl.Manager) error {
	m := newTrainerMetrics(mgr.GetCache())
	if err := mgr.Add(m.runtimeInfo); err != nil {
		return fmt.Errorf("add runtime metrics observer: %w", err)
	}
	if err := mgr.Add(m); err != nil {
		return fmt.Errorf("add TrainJob metrics observer: %w", err)
	}
	return nil
}

func registerLeaderMetrics(runtimeInfo prometheus.Collector) error {
	for _, collector := range leaderMetricCollectors(runtimeInfo) {
		if err := controllerMetrics.Registry.Register(collector); err != nil {
			unregisterLeaderMetrics(runtimeInfo)
			return fmt.Errorf("register leader-owned metrics: %w", err)
		}
	}
	return nil
}

func unregisterLeaderMetrics(runtimeInfo prometheus.Collector) {
	collectors := leaderMetricCollectors(runtimeInfo)
	for _, collector := range collectors {
		controllerMetrics.Registry.Unregister(collector)
	}
}

func leaderMetricCollectors(runtimeInfo prometheus.Collector) []prometheus.Collector {
	collectors := []prometheus.Collector{
		trainJobCreated,
		trainJobComplete,
		trainJobFailed,
		trainJobLifetime,
		trainJobRequestedTrainingNodes,
	}
	if runtimeInfo != nil {
		collectors = append(collectors, runtimeInfo)
	}
	return collectors
}

func runtimeRefLabel(trainJob *trainer.TrainJob) string {
	group := ptr.Deref(trainJob.Spec.RuntimeRef.APIGroup, trainer.GroupVersion.Group)
	kind := ptr.Deref(trainJob.Spec.RuntimeRef.Kind, clusterTrainingRuntimeKind)
	namespace := ""
	if kind == trainingRuntimeKind {
		namespace = trainJob.Namespace
	}
	return fmt.Sprintf("%s/%s/%s/%s", group, kind, namespace, trainJob.Spec.RuntimeRef.Name)
}

func terminalReason(condition metav1.Condition) string {
	switch condition.Reason {
	case trainer.TrainJobRuntimeNotSupportedReason, trainer.TrainJobDeadlineExceededReason, jobsetconsts.FailedJobsReason:
		return condition.Reason
	default:
		return "other"
	}
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
	case policy.NumNodes != nil:
		// MLPolicy with only numNodes set has no runtime-specific source.
		return "none"
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
