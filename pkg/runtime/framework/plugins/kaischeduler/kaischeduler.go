/*
Copyright 2025 The Kubeflow Authors.

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

package kaischeduler

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

const (
	// QueueLabelKey is the label key used to specify the scheduling queue for pods.
	// KAI's pod-grouper will use this label to determine the scheduling queue.
	QueueLabelKey = "kai.scheduler/queue"
)

type KAIScheduler struct{}

var _ framework.EnforcePodGroupPolicyPlugin = (*KAIScheduler)(nil)

const Name = "KAIScheduler"

func New(_ context.Context, _ client.Client, _ client.FieldIndexer) (framework.Plugin, error) {
	return &KAIScheduler{}, nil
}

func (k *KAIScheduler) Name() string {
	return Name
}

func (k *KAIScheduler) EnforcePodGroupPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || trainJob == nil || info.RuntimePolicy.PodGroupPolicy.KAIScheduler == nil {
		return nil
	}

	if info.Scheduler.PodLabels == nil {
		info.Scheduler.PodLabels = map[string]string{}
	}

	// Empty queue is treated as unset so KAI's pod-grouper falls back to its default queue.
	if kaiPolicy := info.RuntimePolicy.PodGroupPolicy.KAIScheduler; kaiPolicy.Queue != nil && *kaiPolicy.Queue != "" {
		info.Scheduler.PodLabels[QueueLabelKey] = *kaiPolicy.Queue
	}

	return nil
}
