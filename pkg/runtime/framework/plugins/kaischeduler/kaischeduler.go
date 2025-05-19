package kai

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
)

type KAIScheduler struct {
	client     client.Client
	restMapper meta.RESTMapper
	scheme     *apiruntime.Scheme
	logger     logr.Logger
}

// Implementing interfaces required for GangScheduling
var _ framework.EnforcePodGroupPolicyPlugin = (*KAIScheduler)(nil)
var _ framework.WatchExtensionPlugin = (*KAIScheduler)(nil)
var _ framework.ComponentBuilderPlugin = (*KAIScheduler)(nil)

const Name = "KAIScheduler"

func New(ctx context.Context, client client.Client, indexer client.FieldIndexer) (framework.Plugin, error) {
	// No need of indexing for KAI
	return &KAIScheduler{
		client:     client,
		restMapper: client.RESTMapper(),
		scheme:     client.Scheme(),
		logger:     ctrl.LoggerFrom(ctx).WithValues("pluginName", constants.JobSetKind),
	}, nil
}

func (k *KAIScheduler) Name() string {
	return Name
}

func (k *KAIScheduler) EnforcePodGroupPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || trainJob == nil {
		return nil
	}

	if info.Scheduler.PodLabels == nil {
		info.Scheduler.PodLabels = make(map[string]string, 1)
	}
	info.Scheduler.PodLabels[schedulerpluginsv1alpha1.PodGroupLabel] = trainJob.Name
	return nil
}

func (k *KAIScheduler) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]any, error) {
	return []any{}, nil
}

func (k *KAIScheduler) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{}
}
