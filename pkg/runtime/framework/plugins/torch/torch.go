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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	autoscalingv2ac "k8s.io/client-go/applyconfigurations/autoscaling/v2"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

type Torch struct{}

var _ framework.EnforceMLPolicyPlugin = (*Torch)(nil)
var _ framework.CustomValidationPlugin = (*Torch)(nil)
var _ framework.ComponentBuilderPlugin = (*Torch)(nil)

const Name = "Torch"

func New(context.Context, client.Client, client.FieldIndexer) (framework.Plugin, error) {
	return &Torch{}, nil
}

func (t *Torch) Name() string {
	return Name
}

func (t *Torch) Build(_ context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	if info == nil || info.RuntimePolicy.MLPolicySource == nil || info.RuntimePolicy.MLPolicySource.Torch == nil {
		return nil, nil
	}

	elasticPolicy := info.RuntimePolicy.MLPolicySource.Torch.ElasticPolicy
	if elasticPolicy == nil || len(elasticPolicy.Metrics) == 0 {
		return nil, nil
	}

	minNodes := ptr.Deref(elasticPolicy.MinNodes, 1)
	maxNodes := ptr.Deref(elasticPolicy.MaxNodes, 1)

	metrics := make([]*autoscalingv2ac.MetricSpecApplyConfiguration, len(elasticPolicy.Metrics))
	for i, metric := range elasticPolicy.Metrics {
		metrics[i] = autoscalingv2ac.MetricSpec()
		if metric.Type != "" {
			metrics[i].WithType(metric.Type)
		}
		if metric.Object != nil {
			objectMetric := autoscalingv2ac.ObjectMetricSource()
			if metric.Object.DescribedObject.Kind != "" {
				objectMetric.WithDescribedObject(autoscalingv2ac.CrossVersionObjectReference().
					WithKind(metric.Object.DescribedObject.Kind).
					WithName(metric.Object.DescribedObject.Name).
					WithAPIVersion(metric.Object.DescribedObject.APIVersion))
			}
			if metric.Object.Metric.Name != "" {
				objectMetric.WithMetric(autoscalingv2ac.MetricIdentifier().
					WithName(metric.Object.Metric.Name).
					WithSelector(metav1ac.LabelSelector().
						WithMatchLabels(metric.Object.Metric.Selector.MatchLabels).
						WithMatchExpressions(toLabelSelectorRequirementApplyConfig(metric.Object.Metric.Selector.MatchExpressions)...)))
			}
			if metric.Object.Target.Type != "" {
				target := autoscalingv2ac.MetricTarget().WithType(metric.Object.Target.Type)
				if metric.Object.Target.Value != nil {
					target.WithValue(*metric.Object.Target.Value)
				}
				if metric.Object.Target.AverageValue != nil {
					target.WithAverageValue(*metric.Object.Target.AverageValue)
				}
				if metric.Object.Target.AverageUtilization != nil {
					target.WithAverageUtilization(*metric.Object.Target.AverageUtilization)
				}
				objectMetric.WithTarget(target)
			}
			metrics[i].WithObject(objectMetric)
		}
		if metric.Pods != nil {
			podsMetric := autoscalingv2ac.PodsMetricSource()
			if metric.Pods.Metric.Name != "" {
				podsMetric.WithMetric(autoscalingv2ac.MetricIdentifier().
					WithName(metric.Pods.Metric.Name).
					WithSelector(metav1ac.LabelSelector().
						WithMatchLabels(metric.Pods.Metric.Selector.MatchLabels).
						WithMatchExpressions(toLabelSelectorRequirementApplyConfig(metric.Pods.Metric.Selector.MatchExpressions)...)))
			}
			if metric.Pods.Target.Type != "" {
				target := autoscalingv2ac.MetricTarget().WithType(metric.Pods.Target.Type)
				if metric.Pods.Target.Value != nil {
					target.WithValue(*metric.Pods.Target.Value)
				}
				if metric.Pods.Target.AverageValue != nil {
					target.WithAverageValue(*metric.Pods.Target.AverageValue)
				}
				if metric.Pods.Target.AverageUtilization != nil {
					target.WithAverageUtilization(*metric.Pods.Target.AverageUtilization)
				}
				podsMetric.WithTarget(target)
			}
			metrics[i].WithPods(podsMetric)
		}
		if metric.Resource != nil {
			resourceMetric := autoscalingv2ac.ResourceMetricSource()
			if metric.Resource.Name != "" {
				resourceMetric.WithName(metric.Resource.Name)
			}
			if metric.Resource.Target.Type != "" {
				target := autoscalingv2ac.MetricTarget().WithType(metric.Resource.Target.Type)
				if metric.Resource.Target.Value != nil {
					target.WithValue(*metric.Resource.Target.Value)
				}
				if metric.Resource.Target.AverageValue != nil {
					target.WithAverageValue(*metric.Resource.Target.AverageValue)
				}
				if metric.Resource.Target.AverageUtilization != nil {
					target.WithAverageUtilization(*metric.Resource.Target.AverageUtilization)
				}
				resourceMetric.WithTarget(target)
			}
			metrics[i].WithResource(resourceMetric)
		}
		if metric.ContainerResource != nil {
			containerResourceMetric := autoscalingv2ac.ContainerResourceMetricSource()
			if metric.ContainerResource.Name != "" {
				containerResourceMetric.WithName(metric.ContainerResource.Name)
			}
			if metric.ContainerResource.Container != "" {
				containerResourceMetric.WithContainer(metric.ContainerResource.Container)
			}
			if metric.ContainerResource.Target.Type != "" {
				target := autoscalingv2ac.MetricTarget().WithType(metric.ContainerResource.Target.Type)
				if metric.ContainerResource.Target.Value != nil {
					target.WithValue(*metric.ContainerResource.Target.Value)
				}
				if metric.ContainerResource.Target.AverageValue != nil {
					target.WithAverageValue(*metric.ContainerResource.Target.AverageValue)
				}
				if metric.ContainerResource.Target.AverageUtilization != nil {
					target.WithAverageUtilization(*metric.ContainerResource.Target.AverageUtilization)
				}
				containerResourceMetric.WithTarget(target)
			}
			metrics[i].WithContainerResource(containerResourceMetric)
		}
		if metric.External != nil {
			externalMetric := autoscalingv2ac.ExternalMetricSource()
			if metric.External.Metric.Name != "" {
				externalMetric.WithMetric(autoscalingv2ac.MetricIdentifier().
					WithName(metric.External.Metric.Name).
					WithSelector(metav1ac.LabelSelector().
						WithMatchLabels(metric.External.Metric.Selector.MatchLabels).
						WithMatchExpressions(toLabelSelectorRequirementApplyConfig(metric.External.Metric.Selector.MatchExpressions)...)))
			}
			if metric.External.Target.Type != "" {
				target := autoscalingv2ac.MetricTarget().WithType(metric.External.Target.Type)
				if metric.External.Target.Value != nil {
					target.WithValue(*metric.External.Target.Value)
				}
				if metric.External.Target.AverageValue != nil {
					target.WithAverageValue(*metric.External.Target.AverageValue)
				}
				if metric.External.Target.AverageUtilization != nil {
					target.WithAverageUtilization(*metric.External.Target.AverageUtilization)
				}
				externalMetric.WithTarget(target)
			}
			metrics[i].WithExternal(externalMetric)
		}
	}

	hpa := autoscalingv2ac.HorizontalPodAutoscaler(trainJob.Name, trainJob.Namespace).
		WithLabels(trainJob.Labels).
		WithAnnotations(trainJob.Annotations).
		WithSpec(autoscalingv2ac.HorizontalPodAutoscalerSpec().
			WithScaleTargetRef(autoscalingv2ac.CrossVersionObjectReference().
				WithKind(constants.JobSetKind).
				WithName(trainJob.Name).
				WithAPIVersion(jobsetv1alpha2.SchemeGroupVersion.String())).
			WithMinReplicas(minNodes).
			WithMaxReplicas(maxNodes).
			WithMetrics(metrics...))
	return []apiruntime.ApplyConfiguration{hpa}, nil
}

func toLabelSelectorRequirementApplyConfig(requirements []metav1.LabelSelectorRequirement) []*metav1ac.LabelSelectorRequirementApplyConfiguration {
	res := make([]*metav1ac.LabelSelectorRequirementApplyConfiguration, len(requirements))
	for i, r := range requirements {
		res[i] = metav1ac.LabelSelectorRequirement().
			WithKey(r.Key).
			WithOperator(r.Operator).
			WithValues(r.Values...)
	}
	return res
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

	numNodes := "1"
	if info.RuntimePolicy.MLPolicySource.Torch.ElasticPolicy != nil {
		elasticPolicy := info.RuntimePolicy.MLPolicySource.Torch.ElasticPolicy
		minNodes := ptr.Deref(elasticPolicy.MinNodes, 1)
		maxNodes := ptr.Deref(elasticPolicy.MaxNodes, 1)
		numNodes = fmt.Sprintf("%d:%d", minNodes, maxNodes)

		if trainerPS != nil && trainerPS.Count != nil {
			*trainerPS.Count = minNodes
		}
	} else {
		if trainerPS != nil && trainerPS.Count != nil && trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
			*trainerPS.Count = *trainJob.Spec.Trainer.NumNodes
		}
		if trainerPS != nil && trainerPS.Count != nil {
			numNodes = fmt.Sprintf("%d", *trainerPS.Count)
		}
	}

	numProcPerNode := ptr.Deref(info.RuntimePolicy.MLPolicySource.Torch.NumProcPerNode, intstr.FromString("auto"))
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumProcPerNode != nil {
		numProcPerNode = ptr.Deref(trainJob.Spec.Trainer.NumProcPerNode, intstr.FromString("auto"))
	}

	// Determine numProcPerNode based on the resourcesPerNode.
	resourcesPerNode := ptr.Deref(runtime.ExtractResourcePerNodeFromRuntime(info), corev1.ResourceRequirements{})
	if jobTrainer := trainJob.Spec.Trainer; jobTrainer != nil && jobTrainer.ResourcesPerNode != nil {
		resourcesPerNode = ptr.Deref(jobTrainer.ResourcesPerNode, corev1.ResourceRequirements{})
	}
	gpuQ := runtime.GetNumGPUPerNode(&resourcesPerNode)
	// If numProcPerNode is "cpu" or no GPU is set in resource, we calculate numProcPerNode based on CPU.
	if numProcPerNode.String() == "cpu" || numProcPerNode.String() == "auto" && gpuQ == 0 {
		numProcPerNode = intstr.FromInt(max(1, getNumCPUPerNode(&resourcesPerNode)))
	}

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
				WithValue(numNodes),
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

	return nil
}

// getNumCPUPerNode calculates the number of CPU processes per node based on the provided resources.
func getNumCPUPerNode(res *corev1.ResourceRequirements) int {
	if res == nil {
		return 0
	}
	limitCpuQ, requestCpuQ := res.Limits.Cpu(), res.Requests.Cpu()
	if requestCpuQ == nil || requestCpuQ.IsZero() {
		if limitCpuQ != nil {
			return int(limitCpuQ.Value())
		}
		return 0
	}
	return int(requestCpuQ.Value())
}
