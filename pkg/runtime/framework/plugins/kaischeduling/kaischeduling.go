package kai

import (
	"context"
	"errors"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper"
	"k8s.io/apimachinery/pkg/api/meta"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
)

type KAIScheduling struct {
	client     client.Client
	restMapper meta.RESTMapper
	scheme     *apiruntime.Scheme
}

// Implementing interfaces required for GangScheduling
var _ framework.EnforcePodGroupPolicyPlugin = (*KAIScheduling)(nil)
var _ framework.WatchExtensionPlugin = (*KAIScheduling)(nil)
var _ framework.ComponentBuilderPlugin = (*KAIScheduling)(nil)

var (
	ErrorCanNotSetupTrainingRuntimeRuntimeClassIndexer        = errors.New("setting index on runtimeClass for TrainingRuntime")
	ErrorCanNotSetupClusterTrainingRuntimeRuntimeClassIndexer = errors.New("setting index on runtimeClass for ClusterTrainingRuntime")
)

const Name = "KAIScheduling"

func New(ctx context.Context, client client.Client) (framework.Plugin, error) {
	return &KAIScheduling{
		client:     client,
		restMapper: client.RESTMapper(),
		scheme:     client.Scheme(),
	}, nil
}

func (k *KAIScheduling) Name() string {
	return Name
}

func (k *KAIScheduling) EnforcePodGroupPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || trainJob == nil {
		return nil
	}

	if info.Scheduler.PodLabels == nil {
		info.Scheduler.PodLabels = make(map[string]string, 1)
	}
	info.Scheduler.PodLabels["kai-scheduler/podgrouper"] = trainJob.Name
	return nil
}

func (k *KAIScheduling) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]any, error) {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || info.RuntimePolicy.PodGroupPolicy.Kaischeduling == nil || trainJob == nil {
		return nil, nil
	}
	_ = podgrouper.NewPodgrouper(k.client, false, true)
	// if err := k.client.Get(ctx, client.ObjectKeyFromObject(trainJob), oldPodGroup.); err != nil {

	// }
	return []any{}, nil
}

func (k *KAIScheduling) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{}
}
