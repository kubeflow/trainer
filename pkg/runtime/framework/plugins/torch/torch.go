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
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
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

func (t *Torch) Validate(_ context.Context, runtimeInfo *runtime.Info, _, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
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

		// Check reserved envs.
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

		// Check supported pretrained models for torchtune.
		// TODO(Electronic-Waste): Add more validation for torchtune when we support more arguments.
		if slices.Equal(newObj.Spec.Trainer.Command, constants.TorchTuneEntrypoint) {
			_, torchTuneErrs := validateTorchTune(runtimeInfo, newObj)
			allErrs = append(allErrs, torchTuneErrs...)
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

	// Determine numProcPerNode based on the resourcesPerNode.
	resourcesPerNode := ptr.Deref(extractResourcePerNodeFromRuntime(info), corev1.ResourceRequirements{})
	if jobTrainer := trainJob.Spec.Trainer; jobTrainer != nil && jobTrainer.ResourcesPerNode != nil {
		resourcesPerNode = ptr.Deref(jobTrainer.ResourcesPerNode, corev1.ResourceRequirements{})
	}
	numProcPerNode = getNumProcPerNode(numProcPerNode, resourcesPerNode)

	// Update envs for Info object.
	var trainerContainer *runtime.Container
	if trainJob.Spec.Trainer != nil {
		if trainerContainer = info.FindContainerByPodSetAncestorContainerName(constants.AncestorTrainer, constants.Node); trainerContainer != nil {
			apply.UpsertEnvVars(&trainerContainer.Env, apply.EnvVars(trainJob.Spec.Trainer.Env...)...)
		}
	}
	if trainerContainer != nil {
		// Add PyTorch distributed "PET_" values for torchrun and torchtune.
		// TODO (andreyvelich): We should validate that envs from different plugins don't conflict with each other.
		// Ref: https://github.com/kubeflow/trainer/pull/2308#discussion_r1823229940
		apply.UpsertEnvVars(&trainerContainer.Env,
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
		)

		if !slices.Equal(trainJob.Spec.Trainer.Command, constants.TorchTuneEntrypoint) {
			// Add PET_MASTER_ADDR and PET_MASTER_PORT envs for torchrun.
			apply.UpsertEnvVars(&trainerContainer.Env,
				*corev1ac.EnvVar().
					WithName(constants.TorchEnvMasterAddr).
					WithValue(fmt.Sprintf("%s-%s-0-0.%s", trainJob.Name, constants.Node, trainJob.Name)),
				*corev1ac.EnvVar().
					WithName(constants.TorchEnvMasterPort).
					WithValue(fmt.Sprintf("%d", constants.ContainerTrainerPort)),
			)
		} else {
			// Mutate trainer command for torchtune.
			// Ref: https://github.com/kubeflow/trainer/tree/master/docs/proposals/2401-llm-trainer-v2#complement-torch-plugin
			// 1. Add rendezvous backend arg for torchtune.
			// Rendezvous backend is only enabled for multi-nodes or multi-devices training.
			var newCommand []string
			gpuQ := getNumGPUPerNode(&resourcesPerNode)
			numNodes := ptr.Deref(ptr.Deref(trainerPS, runtime.PodSet{}).Count, 1)
			if numNodes > 1 || numProcPerNode.Type == intstr.Int && numProcPerNode.IntVal > 1 || numProcPerNode.Type == intstr.String && gpuQ > 1 {
				newCommand = append(newCommand,
					fmt.Sprintf("%s=%s-%s-0-0.%s:%d",
						constants.TorchTuneArgRdzvEndpoint,
						trainJob.Name, constants.Node, trainJob.Name, constants.ContainerTrainerPort,
					),
				)
			}

			// 2. Get the recipe and config from old args and append them to newCommand.
			recipe, config := getRecipeAndConfig(numNodes, numProcPerNode, gpuQ, trainJob)
			newCommand = append(newCommand, recipe, constants.TorchTuneArgConfig, config)

			// 3. Extract output directory, tokenizer path and model mount path from (Cluster)TrainingRuntime.
			newCommand = append(newCommand, extractOverridesFromRuntime(info)...)

			trainJob.Spec.Trainer.Command = append(trainJob.Spec.Trainer.Command, newCommand...)
		}
		// Add container port for the headless service.
		apply.UpsertPort(&trainerContainer.Ports, *corev1ac.ContainerPort().WithContainerPort(constants.ContainerTrainerPort))
	}
	info.SyncPodSetsToTemplateSpec()
	return nil
}

func getNumProcPerNode(nppNode intstr.IntOrString, resourcesPerNode corev1.ResourceRequirements) intstr.IntOrString {
	var (
		shouldUseCPU           func(resources corev1.ResourceList) bool
		fallbackNumProcPerNode intstr.IntOrString
	)
	switch nppNode.String() {
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
		fallbackNumProcPerNode = nppNode
	}

	requestNppNode, requestUseGPU := calculateNumProcPerNode(fallbackNumProcPerNode, resourcesPerNode.Requests, shouldUseCPU)
	limitNppNode, limitUseGPU := calculateNumProcPerNode(fallbackNumProcPerNode, resourcesPerNode.Limits, shouldUseCPU)
	// In these scenarios, we should use the NumProcPerNode calculated from Limits:
	// 1. GPU resources are not specified in Requests but specified in Limits.
	// 2. GPU resources are not specified in both Requests and Limits, but CPU resources are specified in Limits.
	if !requestUseGPU && limitUseGPU || !requestUseGPU && !limitUseGPU && requestNppNode.Type == intstr.Int && limitNppNode.Type == intstr.Int && requestNppNode.IntVal < limitNppNode.IntVal {
		return limitNppNode
	}
	return requestNppNode
}

// calculateNumProcPerNode calculates the number of processes per node based on the provided resources.
// It returns the calculated number of processes per node and a boolean indicating whether GPU resources were used.
func calculateNumProcPerNode(
	fallbackNumProcPerNode intstr.IntOrString, resources corev1.ResourceList, shouldUseCPU func(resources corev1.ResourceList) bool,
) (intstr.IntOrString, bool) {
	var defaultCPU int32 = 1
	if resources != nil {
		// If CPU resource is specified and shouldUseCPU returns true, use the CPU resource value.
		// Otherwise, return the fallback value and indicate whether GPU resources are present.
		if shouldUseCPU(resources) {
			cpuQ := resources[corev1.ResourceCPU]
			return intstr.FromInt32(max(defaultCPU, int32(cpuQ.Value()))), false
		}
		return fallbackNumProcPerNode, numGPU(resources) > 0
	}
	// If resources is nil, return default CPU value.
	return intstr.FromInt32(defaultCPU), false
}

// getNumGPUPerNode returns the GPU count if found.
func getNumGPUPerNode(res *corev1.ResourceRequirements) int {
	if res != nil {
		gpuQ := numGPU(res.Requests)
		if limitGpuQ := numGPU(res.Limits); gpuQ == 0 && limitGpuQ > 0 {
			gpuQ = limitGpuQ
		}
		return gpuQ
	}
	return 0
}

func numGPU(resourcePerNode corev1.ResourceList) int {
	for resName, resQ := range resourcePerNode {
		if strings.Contains(strings.ToLower(resName.String()), "gpu") {
			return int(resQ.Value())
		}
	}
	return 0
}

// extractResourcePerNodeFromRuntime extracts the resource per node from the Trainer Node.
func extractResourcePerNodeFromRuntime(info *runtime.Info) *corev1.ResourceRequirements {
	if jobSetSpec, ok := runtime.TemplateSpecApply[jobsetv1alpha2ac.JobSetSpecApplyConfiguration](info); ok {
		for _, rJob := range jobSetSpec.ReplicatedJobs {
			if rJob.Name != nil && *rJob.Name == constants.Node || rJob.Template.Labels[constants.LabelTrainJobAncestor] == constants.AncestorTrainer {
				for _, container := range rJob.Template.Spec.Template.Spec.Containers {
					if container.Name != nil && *container.Name == constants.Node && container.Resources != nil {
						res := &corev1.ResourceRequirements{
							Limits:   corev1.ResourceList{},
							Requests: corev1.ResourceList{},
						}
						if container.Resources.Limits != nil {
							res.Limits = *container.Resources.Limits
						}
						if container.Resources.Requests != nil {
							res.Requests = *container.Resources.Requests
						}
						return res
					}
				}
			}
		}
	}
	return nil
}
