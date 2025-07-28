package volcano

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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

	volcanoSpec := info.RuntimePolicy.PodGroupPolicy.Volcano

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
	pg := volcanov1beta1ac.PodGroup(trainJob.Name, trainJob.Namespace).
		WithSpec(volcanov1beta1ac.PodGroupSpec().
			WithMinMember(totalMembers).
			WithMinResources(totalResources))

	// Set Volcano-specific fields only if explicitly configured.
	// Volcano uses default values when unset (nil),
	if volcanoSpec.Queue != nil {
		pg.Spec.WithQueue(*volcanoSpec.Queue)
	}
	if volcanoSpec.PriorityClassName != nil {
		pg.Spec.WithPriorityClassName(*volcanoSpec.PriorityClassName)
	}
	if volcanoSpec.NetworkTopology != nil {
		pg.Spec.WithNetworkTopology(volcanov1beta1ac.NetworkTopologySpec().
			WithMode(volcanoSpec.NetworkTopology.Mode).
			WithHighestTierAllowed(*volcanoSpec.NetworkTopology.HighestTierAllowed))
	}

	pg.WithOwnerReferences(metav1ac.OwnerReference().
		WithAPIVersion(trainer.GroupVersion.String()).
		WithKind(trainer.TrainJobKind).
		WithName(trainJob.Name).
		WithUID(trainJob.UID).
		WithController(true).
		WithBlockOwnerDeletion(true))

	return []any{pg}, nil
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
