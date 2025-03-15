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
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/apply"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	corev1 "k8s.io/api/core/v1"
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

func (t *Torch) Validate(runtimeJobTemplate client.Object, runtimeInfo *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	var allErrs field.ErrorList
	if runtimeInfo == nil || runtimeInfo.RuntimePolicy.MLPolicy == nil || runtimeInfo.RuntimePolicy.MLPolicy.Torch == nil || newObj.Spec.Trainer.NumProcPerNode == nil {
		return nil, allErrs
	}

	specPath := field.NewPath("spec")

	if newObj.Spec.Trainer != nil {
		numProcPerNodePath := specPath.Child("trainer").Child("numProcPerNode")
		numProcPerNode := *newObj.Spec.Trainer.NumProcPerNode
		if numProcPerNode.Type == intstr.String {
			allowed := sets.New("auto", "cpu", "gpu")
			if !allowed.Has(numProcPerNode.StrVal) {
				allErrs = append(allErrs, field.Invalid(numProcPerNodePath, newObj.Spec.Trainer.NumProcPerNode, fmt.Sprintf("must have an int value or %v", allowed.UnsortedList())))
			}
		}

		torchEnvs := sets.New[string]()
		for _, env := range newObj.Spec.Trainer.Env {
			if constants.TorchRunReservedEnvNames.Has(env.Name) {
				torchEnvs.Insert(env.Name)
			}
		}

		if torchEnvs.Len() > 0 {
			trainerEnvsPath := specPath.Child("trainer").Child("env")
			allErrs = append(allErrs, field.Invalid(trainerEnvsPath, newObj.Spec.Trainer.Env, fmt.Sprintf("must not have reserved envs, invalid envs configured: %s", strings.Join(torchEnvs.UnsortedList(), ", "))))
		}
	}

	return nil, allErrs
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
	info.RuntimePolicy.MLPolicy.NumNodes = numNodes

	numProcPerNode := ptr.Deref(info.RuntimePolicy.MLPolicy.Torch.NumProcPerNode, intstr.FromString("auto"))
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumProcPerNode != nil {
		numProcPerNode = ptr.Deref(trainJob.Spec.Trainer.NumProcPerNode, intstr.FromString("auto"))
	}

	if jobTrainer := trainJob.Spec.Trainer; jobTrainer != nil && jobTrainer.ResourcesPerNode != nil {
		var (
			shouldUseCPU           func(resources corev1.ResourceList) bool
			fallbackNumProcPerNode intstr.IntOrString
		)
		switch numProcPerNode.String() {
		case "auto":
			shouldUseCPU = func(resources corev1.ResourceList) bool {
				for resName := range resources {
					if strings.Contains(strings.ToLower(resName.String()), "gpu") {
						return false
					}
				}
				return true
			}
			fallbackNumProcPerNode = intstr.FromString("auto")
		case "cpu":
			shouldUseCPU = func(resources corev1.ResourceList) bool {
				_, ok := resources[corev1.ResourceCPU]
				return ok
			}
			fallbackNumProcPerNode = intstr.FromInt32(1)
		default:
			shouldUseCPU = func(corev1.ResourceList) bool { return false }
			fallbackNumProcPerNode = numProcPerNode
		}
		nppNode, usedCPU := calculateNumProcPerNode(fallbackNumProcPerNode, jobTrainer.ResourcesPerNode.Limits, shouldUseCPU)
		if !usedCPU {
			nppNode, _ = calculateNumProcPerNode(fallbackNumProcPerNode, jobTrainer.ResourcesPerNode.Requests, shouldUseCPU)
		}
		numProcPerNode = nppNode
	}

	// Update envs for Info object.
	// Add PyTorch distributed "PET_" values for torchrun
	// TODO (andreyvelich): We should validate that envs from different plugins don't conflict with each other.
	// Ref: https://github.com/kubeflow/trainer/pull/2308#discussion_r1823229940
	var trainerContainer *runtime.Container
	if trainJob.Spec.Trainer != nil {
		if trainerContainer = info.FindContainerByPodSetContainerName(constants.JobTrainerNode, constants.ContainerTrainer); trainerContainer != nil {
			apply.UpsertEnvVars(&trainerContainer.Env, apply.EnvVars(trainJob.Spec.Trainer.Env...)...)
		}
	}
	if trainerContainer != nil {
		apply.UpsertEnvVar(&trainerContainer.Env,
			*corev1ac.EnvVar().
				WithName(constants.TorchEnvNumNodes).
				WithValue(fmt.Sprintf("%d", ptr.Deref(numNodes, 1))),
			*corev1ac.EnvVar().
				WithName(constants.TorchEnvNumProcPerNode).
				WithValue(numProcPerNode.String()),
			*corev1ac.EnvVar().
				WithName(constants.TorchEnvNodeRank).
				WithValueFrom(corev1ac.EnvVarSource().
					WithFieldRef(corev1ac.ObjectFieldSelector().
						WithFieldPath(constants.JobCompletionIndexFieldPath))),
			*corev1ac.EnvVar().
				WithName(constants.TorchEnvMasterAddr).
				WithValue(fmt.Sprintf("%s-%s-0-0.%s", trainJob.Name, constants.JobTrainerNode, trainJob.Name)),
			*corev1ac.EnvVar().
				WithName(constants.TorchEnvMasterPort).
				WithValue(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
		)
		// Add container port for the headless service.
		apply.UpsertPort(&trainerContainer.Ports, *corev1ac.ContainerPort().WithContainerPort(constants.ContainerTrainerPort))
	}
	info.SyncPodSetsToTemplateSpec()
	return nil
}

// calculateNumProcPerNode calculates the number of processes per node based on the provided resources.
// It returns the calculated number of processes per node and a boolean indicating whether CPU resources were used.
func calculateNumProcPerNode(
	fallbackNumProcPerNode intstr.IntOrString, resources corev1.ResourceList, shouldUseCPU func(resources corev1.ResourceList) bool,
) (intstr.IntOrString, bool) {
	var defaultCPU int32 = 1
	if resources != nil {
		if shouldUseCPU(resources) {
			cpuQ := resources[corev1.ResourceCPU]
			return intstr.FromInt32(max(defaultCPU, int32(cpuQ.Value()))), true
		}
		return fallbackNumProcPerNode, false
	}
	return intstr.FromInt32(defaultCPU), false
}
