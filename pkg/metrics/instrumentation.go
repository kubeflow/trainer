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
// term. The handler is registered before waiting for initial synchronization;
// its bootstrap gate prevents initial-list objects from being counted.
func (m *trainerMetrics) Start(ctx context.Context) error {
	if m.cache == nil {
		return fmt.Errorf("metrics cache is required")
	}
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
	result := "succeeded"
	if condition.Type == trainer.TrainJobFailed {
		result = "failed"
	}
	reason := terminalReason(condition, result)
	trainJobFinished.WithLabelValues(result, reason, runtimeRef).Inc()
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
		if *trainJob.Spec.Trainer.NumNodes > 0 {
			trainJobRequestedTrainingNodes.WithLabelValues(runtimeRef).Observe(float64(*trainJob.Spec.Trainer.NumNodes))
		}
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

// NeedLeaderElection keeps the current runtime inventory available on every
// controller replica. It is a scrape-time gauge, not an event counter.
func (c *runtimeInfoCollector) NeedLeaderElection() bool {
	return false
}

func (c *runtimeInfoCollector) Start(ctx context.Context) error {
	if c.cache == nil {
		return fmt.Errorf("metrics cache is required")
	}
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
	<-ctx.Done()
	return nil
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
	next := copyRuntimeSnapshot(c.snapshot)
	next[key] = info
	c.snapshot = next
	c.mu.Unlock()
}

func (c *runtimeInfoCollector) delete(obj any, kind string) {
	_, key, ok := runtimeInfoFromObject(obj, kind)
	if !ok {
		return
	}
	c.mu.Lock()
	next := copyRuntimeSnapshot(c.snapshot)
	delete(next, key)
	c.snapshot = next
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
	if spec.MLPolicy == nil {
		return 0, false
	}
	if spec.MLPolicy.NumNodes == nil {
		return 1, true
	}
	if *spec.MLPolicy.NumNodes <= 0 {
		return 0, false
	}
	return *spec.MLPolicy.NumNodes, true
}

func copyRuntimeSnapshot(snapshot map[string]runtimeInfo) map[string]runtimeInfo {
	next := make(map[string]runtimeInfo, len(snapshot)+1)
	for key, value := range snapshot {
		next[key] = value
	}
	return next
}

func setupTrainerMetrics(mgr ctrl.Manager) error {
	m := newTrainerMetrics(mgr.GetCache())
	if err := controllerMetrics.Registry.Register(m.runtimeInfo); err != nil {
		return fmt.Errorf("register runtime metrics: %w", err)
	}
	if err := mgr.Add(m.runtimeInfo); err != nil {
		return fmt.Errorf("add runtime metrics observer: %w", err)
	}
	if err := mgr.Add(m); err != nil {
		return fmt.Errorf("add TrainJob metrics observer: %w", err)
	}
	return nil
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

func terminalReason(condition metav1.Condition, result string) string {
	if result == "succeeded" {
		return "none"
	}
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
