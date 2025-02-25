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

package torch

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
)

type Torch struct{}

var _ framework.EnforceMLPolicyPlugin = (*Torch)(nil)
var _ framework.CustomValidationPlugin = (*Torch)(nil)

const Name = "Torch"

func New(context.Context, client.Client, client.FieldIndexer) (framework.Plugin, error) {
	return &Torch{}, nil
}

func (t *Torch) Name() string {
	return Name
}

// TODO: Need to implement validations for Torch policy.
func (t *Torch) Validate(oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}

// TODO (andreyvelich): Add support for PyTorch elastic when JobSet supports Elastic Jobs.
func (t *Torch) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.Torch == nil {
		return nil
	}

	// TrainJob contains the actual information for the Trainer.
	numNodes := info.RuntimePolicy.MLPolicy.NumNodes
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
		numNodes = trainJob.Spec.Trainer.NumNodes
	}
	info.Trainer.NumNodes = numNodes

	numProcPerNode := ptr.Deref(info.RuntimePolicy.MLPolicy.Torch.NumProcPerNode, intstr.FromString("auto"))
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumProcPerNode != nil {
		numProcPerNode = ptr.Deref(trainJob.Spec.Trainer.NumProcPerNode, intstr.FromString("auto"))
	}

	// Update envs for Info object.
	// Add PyTorch distributed "PET_" values for torchrun
	// TODO (andreyvelich): Add validation to check that TrainJob doesn't have "PET_" envs.
	// TODO (andreyvelich): We should validate that envs from different plugins don't conflict with each other.
	// Ref: https://github.com/kubeflow/trainer/pull/2308#discussion_r1823229940

	infoEnvs := []corev1.EnvVar{
		{
			Name:  constants.TorchEnvNumNodes,
			Value: fmt.Sprintf("%d", ptr.Deref(numNodes, 1)),
		},
		{
			Name:  constants.TorchEnvNumProcPerNode,
			Value: numProcPerNode.String(),
		},
		{
			Name: constants.TorchEnvNodeRank,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: constants.JobCompletionIndexFieldPath,
				},
			},
		},
		{
			Name:  constants.TorchEnvMasterAddr,
			Value: fmt.Sprintf("%s-%s-0-0.%s", trainJob.Name, constants.JobTrainerNode, trainJob.Name),
		},
		{
			Name:  constants.TorchEnvMasterPort,
			Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
		},
	}

	// Set for all Info envs.
	envNames := sets.New[string]()
	for _, env := range infoEnvs {
		envNames.Insert(env.Name)
	}
	// Info envs take precedence over TrainJob envs.
	if trainJob.Spec.Trainer != nil {
		for _, env := range trainJob.Spec.Trainer.Env {
			if !envNames.Has(env.Name) {
				info.Trainer.Env = append(info.Trainer.Env, corev1.EnvVar{Name: env.Name, Value: env.Value})
			}
		}
	}
	// Insert Torch distributed envs into the list end.
	info.Trainer.Env = append(info.Trainer.Env, infoEnvs...)

	// Add container port for the headless service.
	info.Trainer.ContainerPort = &corev1.ContainerPort{
		ContainerPort: constants.ContainerTrainerPort,
	}

	// Update total Pod requests for the PodGroupPolicy plugin.
	for rName := range info.TotalRequests {
		// For other Jobs like the Initializer, replica is always equal to 1.
		// TODO (andreyvelich): Add support for total requests from the TrainJob's ResourcesPerNode.
		if rName == constants.JobTrainerNode {
			info.TotalRequests[rName] = runtime.TotalResourceRequest{
				Replicas:    ptr.Deref(numNodes, constants.DefaultJobReplicas),
				PodRequests: info.TotalRequests[rName].PodRequests,
			}
		}
	}

	return nil
}
