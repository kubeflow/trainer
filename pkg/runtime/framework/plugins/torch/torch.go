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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
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

func (t *Torch) Validate(runtimeInfo *runtime.Info, _, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	var allErrs field.ErrorList
	if runtimeInfo == nil || runtimeInfo.RuntimePolicy.MLPolicySource == nil || runtimeInfo.RuntimePolicy.MLPolicySource.Torch == nil || newObj.Spec.Trainer == nil || newObj.Spec.Trainer.NumProcPerNode == nil {
		return nil, allErrs
	}

	specPath := field.NewPath("spec")

	if newObj.Spec.Trainer != nil {
		numProcPerNodePath := specPath.Child("trainer").Child("numProcPerNode")
		numProcPerNode := *newObj.Spec.Trainer.NumProcPerNode
		if numProcPerNode.Type == intstr.String {
			allowed := sets.New("auto", "cpu", "gpu")
			if !allowed.Has(numProcPerNode.StrVal) {
				allErrs = append(allErrs, field.Invalid(numProcPerNodePath, numProcPerNode, fmt.Sprintf("must have an int value or %v", sets.List(allowed))))
			}
		}

		// Check reserved envs for torchrun.
		// TODO(Electronic-Waste): Add validation for torchtune args.
		if !slices.Equal(newObj.Spec.Trainer.Command, constants.TorchTuneEntrypoint) {
			torchEnvs := sets.New[string]()
			for _, env := range newObj.Spec.Trainer.Env {
				if constants.TorchRunReservedEnvNames.Has(env.Name) {
					torchEnvs.Insert(env.Name)
				}
			}

			if torchEnvs.Len() > 0 {
				trainerEnvsPath := specPath.Child("trainer").Child("env")
				allErrs = append(allErrs, field.Invalid(trainerEnvsPath, newObj.Spec.Trainer.Env, fmt.Sprintf("must not have reserved envs, invalid envs configured: %v", sets.List(torchEnvs))))
			}
		}
	}

	return nil, allErrs
}

// TODO (andreyvelich): Add support for PyTorch elastic when JobSet supports Elastic Jobs.
func (t *Torch) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.MLPolicySource == nil || info.RuntimePolicy.MLPolicySource.Torch == nil {
		return nil
	}

	// TrainJob contains the actual information for the Trainer.
	trainerPS := info.FindPodSetByAncestor(constants.AncestorTrainer)
	if trainerPS != nil && trainerPS.Count != nil && trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
		*trainerPS.Count = *trainJob.Spec.Trainer.NumNodes
	}

	numProcPerNode := ptr.Deref(info.RuntimePolicy.MLPolicySource.Torch.NumProcPerNode, intstr.FromString("auto"))
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
	var trainerContainer *runtime.Container
	if trainJob.Spec.Trainer != nil {
		if trainerContainer = info.FindContainerByPodSetAncestorContainerName(constants.AncestorTrainer, constants.Node); trainerContainer != nil {
			apply.UpsertEnvVars(&trainerContainer.Env, apply.EnvVars(trainJob.Spec.Trainer.Env...)...)
		}
	}
	if trainerContainer != nil {
		if !slices.Equal(trainJob.Spec.Trainer.Command, constants.TorchTuneEntrypoint) {
			// Add PyTorch distributed "PET_" values for torchrun.
			// TODO (andreyvelich): We should validate that envs from different plugins don't conflict with each other.
			// Ref: https://github.com/kubeflow/trainer/pull/2308#discussion_r1823229940
			apply.UpsertEnvVar(&trainerContainer.Env,
				*corev1ac.EnvVar().
					WithName(constants.TorchEnvNumNodes).
					WithValue(fmt.Sprintf("%d", ptr.Deref(ptr.Deref(trainerPS, runtime.PodSet{}).Count, 1))),
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
					WithValue(fmt.Sprintf("%s-%s-0-0.%s", trainJob.Name, constants.Node, trainJob.Name)),
				*corev1ac.EnvVar().
					WithName(constants.TorchEnvMasterPort).
					WithValue(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
			)
		} else {
			// Mutate command line args for torchtune.
			// Ref: https://github.com/kubeflow/trainer/tree/master/docs/proposals/2401-llm-trainer-v2#complement-torch-plugin
			oldArgs, newArgs := trainerContainer.Args, []string{}

			// 1. Add PyTorch distributed command line args for torchtune.
			// TODO(Electronic-Waste): Add more args for torchtune if required.
			numNodes := ptr.Deref(ptr.Deref(trainerPS, runtime.PodSet{}).Count, 1)
			newArgs = append(newArgs,
				fmt.Sprintf("%s %d",
					constants.TorchTuneArgNumNodes,
					numNodes,
				),
				fmt.Sprintf("%s %s",
					constants.TorchTuneArgNumProcPerNode,
					numProcPerNode.String(),
				),
				fmt.Sprintf("%s %s",
					constants.TorchTuneArgRdzvId,
					trainJob.Name,
				),
				fmt.Sprintf("%s %s-%s-0-0.%s:%d",
					constants.TorchTuneArgRdzvEndpoint,
					trainJob.Name, constants.Node, trainJob.Name, constants.ContainerTrainerPort,
				),
			)

			// 2. Get the recipe and config from old args and append them to new args.
			recipe := getRecipeFromArgs(numNodes, numProcPerNode, oldArgs)
			config := getConfigFileFromArgs(numNodes, recipe, oldArgs)
			newArgs = append(newArgs, recipe, fmt.Sprintf("--config %s", config))

			// 3. Reserve old arguments to override corresponding items in the config file.
			newArgs = append(newArgs, slices.DeleteFunc(oldArgs, func(arg string) bool {
				return strings.HasPrefix(arg, "model")
			})...)

			trainerContainer.Args = newArgs
		}
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

// getRecipeFromArgs extracts the recipe from the distributed parameters and command line arguments.
// TODO(Electronic-Waste): Add support for more recipes.
func getRecipeFromArgs(numNodes int32, numProcPerNode intstr.IntOrString, _ []string) string {
	recipe := constants.TorchTuneDefaultRecipe
	if numNodes == 1 && numProcPerNode.Type == intstr.Int && numProcPerNode.IntVal == 1 {
		recipe = constants.TorchTuneFullFinetuneSingleDevice
	}
	return recipe
}

// getConfigFromArgs extracts the config from distributed parameters, recipe and command line arguments.
func getConfigFileFromArgs(numNodes int32, recipe string, args []string) string {
	// Extract model from command line args.
	model := constants.MODEL_LLAMA3_2_1B
	for _, arg := range args {
		if strings.HasPrefix(arg, "model") {
			model = strings.Split(arg, "=")[1]
			break
		}
	}

	// Determine the config file name based on the recipe and number of nodes.
	var suffix string
	switch recipe {
	case constants.TorchTuneFullFinetuneDistributed:
		if numNodes == 1 {
			suffix = constants.TorchTuneFullFinetuneMultiDevicesConfigSuffix
		} else {
			suffix = constants.TorchTuneFullFinetuneMultiNodesConfigSuffix
		}
	case constants.TorchTuneFullFinetuneSingleDevice:
		suffix = constants.TorchTuneFullFinetuneSingleDeviceConfigSuffix
	}

	return fmt.Sprintf("%s%s.yaml", model, suffix)
}
