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

package pdb

import (
	"context"

	policyv1 "k8s.io/api/policy/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	policyv1ac "k8s.io/client-go/applyconfigurations/policy/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

// PodDisruptionBudget builds a PodDisruptionBudget that protects the pods of a
// gang-scheduled TrainJob from voluntary disruptions (node drains, cluster
// autoscaler scale-downs). Distributed training is tightly coupled: every rank
// must be available for the job to make progress, so evicting a single pod
// stalls the whole gang until it is restarted from the last checkpoint.
type PodDisruptionBudget struct {
	client client.Client
}

var _ framework.ComponentBuilderPlugin = (*PodDisruptionBudget)(nil)
var _ framework.WatchExtensionPlugin = (*PodDisruptionBudget)(nil)

const Name = "PodDisruptionBudget"

// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

func New(_ context.Context, client client.Client, _ client.FieldIndexer, _ *configapi.Configuration) (framework.Plugin, error) {
	return &PodDisruptionBudget{
		client: client,
	}, nil
}

func (p *PodDisruptionBudget) Name() string {
	return Name
}

// Build creates a PodDisruptionBudget for gang-scheduled, multi-pod TrainJobs.
// The budget's minAvailable equals the number of training replicas (the trainer
// PodSets), which is the long-lived gang that must stay available for the job to
// make progress. Short-lived initializer pods are deliberately excluded: they
// run to completion and stop being Ready, so counting them would make the PDB
// permanently unsatisfiable (currentHealthy could never reach minAvailable).
func (p *PodDisruptionBudget) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	// Only gang-scheduled TrainJobs declare all-or-nothing semantics, so we
	// scope the PDB to them to avoid blocking node maintenance for ordinary
	// workloads.
	if info == nil || trainJob == nil || info.RuntimePolicy.PodGroupPolicy == nil {
		return nil, nil
	}

	var trainerReplicas int32
	for _, ps := range info.TemplateSpec.PodSets {
		if ptr.Deref(ps.Ancestor, "") == constants.AncestorTrainer && ps.Count != nil {
			trainerReplicas += *ps.Count
		}
	}

	// A PodDisruptionBudget is only meaningful for distributed (multi-pod)
	// TrainJobs. Protecting a single pod would block routine node maintenance
	// without providing gang-availability guarantees.
	if trainerReplicas <= 1 {
		return nil, nil
	}

	pdb := policyv1ac.PodDisruptionBudget(trainJob.Name, trainJob.Namespace).
		WithSpec(policyv1ac.PodDisruptionBudgetSpec().
			WithMinAvailable(intstr.FromInt32(trainerReplicas)).
			WithSelector(metav1ac.LabelSelector().
				WithMatchLabels(map[string]string{
					jobsetv1alpha2.JobSetNameKey: trainJob.Name,
				}))).
		WithOwnerReferences(metav1ac.OwnerReference().
			WithAPIVersion(trainer.GroupVersion.String()).
			WithKind(trainer.TrainJobKind).
			WithName(trainJob.Name).
			WithUID(trainJob.UID).
			WithController(true).
			WithBlockOwnerDeletion(true))

	return []apiruntime.ApplyConfiguration{pdb}, nil
}

func (p *PodDisruptionBudget) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Watches(
				&policyv1.PodDisruptionBudget{},
				handler.EnqueueRequestForOwner(
					p.client.Scheme(), p.client.RESTMapper(), &trainer.TrainJob{}, handler.OnlyControllerOwner(),
				),
			)
		},
	}
}
