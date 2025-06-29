package volcano

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	runtimeindexer "github.com/kubeflow/trainer/pkg/runtime/indexer"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	volcanov1beta1ac "volcano.sh/apis/pkg/client/applyconfiguration/scheduling/v1beta1"
)

type Volcano struct {
	client     client.Client
	restMapper meta.RESTMapper
	scheme     *apiruntime.Scheme
	logger     logr.Logger
}

var _ framework.EnforcePodGroupPolicyPlugin = (*Volcano)(nil)
var _ framework.ComponentBuilderPlugin = (*Volcano)(nil)
var _ framework.WatchExtensionPlugin = (*Volcano)(nil)

var (
	ErrorCanNotSetupTrainingRuntimeRuntimeClassIndexer        = errors.New("setting index on runtimeClass for TrainingRuntime")
	ErrorCanNotSetupClusterTrainingRuntimeRuntimeClassIndexer = errors.New("setting index on runtimeClass for ClusterTrainingRuntime")
)

const Name = "Volcano"

// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups,verbs=create;get;list;watch;update;patch;delete

func New(ctx context.Context, client client.Client, indexer client.FieldIndexer) (framework.Plugin, error) {
	if err := indexer.IndexField(ctx, &trainer.TrainingRuntime{}, TrainingRuntimeContainerRuntimeClassKey,
		IndexTrainingRuntimeContainerRuntimeClass); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrorCanNotSetupTrainingRuntimeRuntimeClassIndexer, err)
	}
	if err := indexer.IndexField(ctx, &trainer.ClusterTrainingRuntime{}, ClusterTrainingRuntimeContainerRuntimeClassKey,
		IndexClusterTrainingRuntimeContainerRuntimeClass); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrorCanNotSetupClusterTrainingRuntimeRuntimeClassIndexer, err)
	}
	return &Volcano{
		client:     client,
		restMapper: client.RESTMapper(),
		scheme:     client.Scheme(),
	}, nil
}

func (v *Volcano) Name() string {
	return Name
}

func (v *Volcano) EnforcePodGroupPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || trainJob == nil {
		return nil
	}
	if info.Scheduler.PodLabels == nil {
		info.Scheduler.PodLabels = map[string]string{}
	}
	info.Scheduler.PodLabels["volcano.sh/podgroup"] = trainJob.Name // TODO: use a constant
	return nil
}

func (v *Volcano) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]any, error) {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || info.RuntimePolicy.PodGroupPolicy.Volcano == nil {
		return nil, nil
	}

	// Do not update the PodGroup if it already exists and the TrainJob is not suspended
	oldPodGroup := &volcanov1beta1.PodGroup{}
	if err := v.client.Get(ctx, client.ObjectKeyFromObject(trainJob), oldPodGroup); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		oldPodGroup = nil
	}
	if oldPodGroup != nil && !ptr.Deref(trainJob.Spec.Suspend, false) {
		return nil, nil
	}

	// Aggregate pod resource requests
	var totalMembers int32
	totalResources := make(corev1.ResourceList)
	for _, ps := range info.TemplateSpec.PodSets {
		count := *ps.Count
		totalMembers += count
		for resName, quantity := range ps.SinglePodRequests {
			quantity.Mul(int64(count))
			current := totalResources[resName]
			current.Add(quantity)
			totalResources[resName] = current
		}
	}

	volcanoSpec := info.RuntimePolicy.PodGroupPolicy.Volcano
	pg := volcanov1beta1ac.PodGroup(trainJob.Name, trainJob.Namespace).
		WithSpec(volcanov1beta1ac.PodGroupSpec().
			WithMinMember(totalMembers).
			WithMinResources(totalResources))

	if *volcanoSpec.Queue != "" {
		pg.Spec.WithQueue(*volcanoSpec.Queue)
	}
	if *volcanoSpec.PriorityClassName != "" {
		pg.Spec.WithPriorityClassName(*volcanoSpec.PriorityClassName)
	}

	pg.WithOwnerReferences(metav1ac.OwnerReference().
		WithAPIVersion(trainer.GroupVersion.String()).
		WithKind(trainer.TrainJobKind).
		WithName(trainJob.Name).
		WithUID(trainJob.UID).
		WithController(true).
		WithBlockOwnerDeletion(true),
	)

	return []any{pg}, nil
}

type PodGroupRuntimeClassHandler struct {
	client client.Client
}

var _ handler.TypedEventHandler[*nodev1.RuntimeClass, reconcile.Request] = (*PodGroupRuntimeClassHandler)(nil)

func (h *PodGroupRuntimeClassHandler) Create(ctx context.Context, e event.TypedCreateEvent[*nodev1.RuntimeClass], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	containerRuntimeClass := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("runtimeClass", klog.KObj(containerRuntimeClass))
	if err := h.queueSuspendedTrainJobs(ctx, containerRuntimeClass, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupRuntimeClassHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*nodev1.RuntimeClass], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	newContainerRuntimeClass := e.ObjectNew
	log := ctrl.LoggerFrom(ctx).WithValues("runtimeClass", klog.KObj(newContainerRuntimeClass))
	if err := h.queueSuspendedTrainJobs(ctx, newContainerRuntimeClass, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupRuntimeClassHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*nodev1.RuntimeClass], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	containerRuntimeClass := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("runtimeClass", klog.KObj(containerRuntimeClass))
	if err := h.queueSuspendedTrainJobs(ctx, containerRuntimeClass, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupRuntimeClassHandler) Generic(context.Context, event.TypedGenericEvent[*nodev1.RuntimeClass], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *PodGroupRuntimeClassHandler) queueSuspendedTrainJobs(ctx context.Context, runtimeClass *nodev1.RuntimeClass, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
	var trainingRuntimes trainer.TrainingRuntimeList
	if err := h.client.List(ctx, &trainingRuntimes, client.MatchingFields{TrainingRuntimeContainerRuntimeClassKey: runtimeClass.Name}); err != nil {
		return err
	}
	var clusterTrainingRuntimes trainer.ClusterTrainingRuntimeList
	if err := h.client.List(ctx, &clusterTrainingRuntimes, client.MatchingFields{ClusterTrainingRuntimeContainerRuntimeClassKey: runtimeClass.Name}); err != nil {
		return err
	}

	var trainJobs []trainer.TrainJob
	for _, trainingRuntime := range trainingRuntimes.Items {
		var trainJobsWithTrainingRuntime trainer.TrainJobList
		err := h.client.List(ctx, &trainJobsWithTrainingRuntime, client.MatchingFields{runtimeindexer.TrainJobRuntimeRefKey: trainingRuntime.Name})
		if err != nil {
			return err
		}
		trainJobs = append(trainJobs, trainJobsWithTrainingRuntime.Items...)
	}
	for _, clusterTrainingRuntime := range clusterTrainingRuntimes.Items {
		var trainJobsWithClTrainingRuntime trainer.TrainJobList
		err := h.client.List(ctx, &trainJobsWithClTrainingRuntime, client.MatchingFields{runtimeindexer.TrainJobClusterRuntimeRefKey: clusterTrainingRuntime.Name})
		if err != nil {
			return err
		}
		trainJobs = append(trainJobs, trainJobsWithClTrainingRuntime.Items...)
	}
	trainJobs = slices.CompactFunc(trainJobs, func(a, b trainer.TrainJob) bool {
		return a.Name == b.Name
	})
	for _, trainJob := range trainJobs {
		if ptr.Deref(trainJob.Spec.Suspend, false) {
			q.Add(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&trainJob)})
		}
	}
	return nil
}

type PodGroupLimitRangeHandler struct {
	client client.Client
}

var _ handler.TypedEventHandler[*corev1.LimitRange, reconcile.Request] = (*PodGroupLimitRangeHandler)(nil)

func (h *PodGroupLimitRangeHandler) Create(ctx context.Context, e event.TypedCreateEvent[*corev1.LimitRange], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	limitRange := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("limitRange", klog.KObj(limitRange))
	if err := h.queueSuspendedTrainJob(ctx, limitRange.Namespace, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupLimitRangeHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*corev1.LimitRange], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	newLimitRange := e.ObjectNew
	log := ctrl.LoggerFrom(ctx).WithValues("limitRange", klog.KObj(newLimitRange))
	if err := h.queueSuspendedTrainJob(ctx, newLimitRange.Namespace, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupLimitRangeHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*corev1.LimitRange], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	limitRange := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("limitRange", klog.KObj(limitRange))
	if err := h.queueSuspendedTrainJob(ctx, limitRange.Namespace, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupLimitRangeHandler) Generic(context.Context, event.TypedGenericEvent[*corev1.LimitRange], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *PodGroupLimitRangeHandler) queueSuspendedTrainJob(ctx context.Context, ns string, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
	var trainJobs trainer.TrainJobList
	if err := h.client.List(ctx, &trainJobs, client.InNamespace(ns)); err != nil {
		return err
	}
	for _, trainJob := range trainJobs.Items {
		if ptr.Deref(trainJob.Spec.Suspend, false) {
			q.Add(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&trainJob)})
		}
	}
	return nil
}

func (v *Volcano) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Watches(
				&volcanov1beta1.PodGroup{},
				handler.EnqueueRequestForOwner(
					v.scheme, v.restMapper, &trainer.TrainJob{}, handler.OnlyControllerOwner(),
				),
			)
		},
	}
}
