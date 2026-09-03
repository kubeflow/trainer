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

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	fwkcore "github.com/kubeflow/trainer/v2/pkg/runtime/framework/core"
	fwkplugins "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins"
	idxer "github.com/kubeflow/trainer/v2/pkg/runtime/indexer"
	trainingruntimeutil "github.com/kubeflow/trainer/v2/pkg/util/trainingruntime"
)

var (
	errorNotFoundSpecifiedTrainingRuntime = errors.New("TrainingRuntime specified in TrainJob is not found")
)

type TrainingRuntime struct {
	framework *fwkcore.Framework
	client    client.Client
}

var TrainingRuntimeGroupKind = schema.GroupKind{
	Group: trainer.GroupVersion.Group,
	Kind:  trainer.TrainingRuntimeKind,
}.String()

var _ runtime.Runtime = (*TrainingRuntime)(nil)

var trainingRuntimeFactory *TrainingRuntime

func NewTrainingRuntime(ctx context.Context, c client.Client, indexer client.FieldIndexer, cfg *configapi.Configuration) (runtime.Runtime, error) {
	if err := indexer.IndexField(ctx, &trainer.TrainJob{}, idxer.TrainJobRuntimeRefKey, idxer.IndexTrainJobTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on TrainingRuntime for TrainJob: %w", err)
	}
	if err := indexer.IndexField(ctx, &trainer.TrainJob{}, idxer.TrainJobClusterRuntimeRefKey, idxer.IndexTrainJobClusterTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on ClusterTrainingRuntime for TrainJob: %w", err)
	}
	fwk, err := fwkcore.New(ctx, c, fwkplugins.NewRegistry(), indexer, cfg)
	if err != nil {
		return nil, err
	}
	trainingRuntimeFactory = &TrainingRuntime{
		framework: fwk,
		client:    c,
	}
	return trainingRuntimeFactory, nil
}

func (r *TrainingRuntime) NewObjects(ctx context.Context, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	var trainingRuntime trainer.TrainingRuntime
	// Try to get runtime from snapshot first
	if err := getRuntimeSnapshot(ctx, r.client, trainJob, &trainingRuntime); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("unable to get runtime snapshot: %w", err)
		}

		// Snapshot doesn't exist, load runtime from API server
		err := r.client.Get(ctx, client.ObjectKey{Namespace: trainJob.Namespace, Name: trainJob.Spec.RuntimeRef.Name}, &trainingRuntime)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errorNotFoundSpecifiedTrainingRuntime, err)
		}

		// Create snapshot for future reconciliations
		if err := createRuntimeSnapshot(ctx, r.client, trainJob, &trainingRuntime); err != nil {
			return nil, fmt.Errorf("creating runtime snapshot: %w", err)
		}
	}

	info, err := r.RuntimeInfo(ctx, trainJob, trainingRuntime.Spec.Template, trainingRuntime.Spec.MLPolicy, trainingRuntime.Spec.PodGroupPolicy)
	if err != nil {
		return nil, err
	}
	return r.framework.RunComponentBuilderPlugins(ctx, info, trainJob)
}

// RuntimeInfo builds the Info object for a TrainJob and consolidates it through the
// Build Phase extension points, in this order:
//  1. EnforceMLPolicy, then EnforcePodGroupPolicy, for the parameters declared in the
//     runtime `.spec.mlPolicy` and `.spec.podGroupPolicy`.
//  2. EnforcePodSpec, for the PodSet concerns enabled outside of MLPolicy and PodGroupPolicy APIs.
//  3. PreBuildSync, which consolidates the Info object with the concrete runtime template.
func (r *TrainingRuntime) RuntimeInfo(
	ctx context.Context, trainJob *trainer.TrainJob, runtimeTemplateSpec any, mlPolicy *trainer.MLPolicy, podGroupPolicy *trainer.PodGroupPolicy,
) (*runtime.Info, error) {

	jobSetTemplateSpec, ok := runtimeTemplateSpec.(trainer.JobSetTemplateSpec)
	if !ok {
		return nil, fmt.Errorf("unsupported runtimeTemplateSpec")
	}
	info, err := r.newRuntimeInfo(ctx, trainJob, jobSetTemplateSpec, mlPolicy, podGroupPolicy)
	if err != nil {
		return nil, err
	}
	if err = r.framework.RunEnforceMLPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}
	if err = r.framework.RunEnforcePodGroupPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}
	if err = r.framework.RunEnforcePodSpecPlugins(info, trainJob); err != nil {
		return nil, err
	}
	if err = r.framework.RunPreComponentBuilderPlugins(info, trainJob); err != nil {
		return nil, err
	}

	return info, nil
}

func (r *TrainingRuntime) newRuntimeInfo(
	ctx context.Context, trainJob *trainer.TrainJob, jobSetTemplateSpec trainer.JobSetTemplateSpec, mlPolicy *trainer.MLPolicy, podGroupPolicy *trainer.PodGroupPolicy,
) (*runtime.Info, error) {
	propagationLabels := maps.Clone(jobSetTemplateSpec.Labels)
	propagationAnnotations := maps.Clone(jobSetTemplateSpec.Annotations)
	for _, patch := range trainJob.Spec.RuntimePatches {
		if patch.TrainingRuntimeSpec != nil && patch.TrainingRuntimeSpec.Template != nil && patch.TrainingRuntimeSpec.Template.Metadata != nil {
			if propagationLabels == nil && len(patch.TrainingRuntimeSpec.Template.Metadata.Labels) > 0 {
				propagationLabels = make(map[string]string)
			}
			for k, v := range patch.TrainingRuntimeSpec.Template.Metadata.Labels {
				propagationLabels[k] = v
			}
			if propagationAnnotations == nil && len(patch.TrainingRuntimeSpec.Template.Metadata.Annotations) > 0 {
				propagationAnnotations = make(map[string]string)
			}
			for k, v := range patch.TrainingRuntimeSpec.Template.Metadata.Annotations {
				propagationAnnotations[k] = v
			}
		}
	}
	err := r.mergeRuntimePatches(trainJob, &jobSetTemplateSpec)
	if err != nil {
		return nil, err
	}

	jobSetSpecApply, err := apply.FromTypedObjWithFields[jobsetv1alpha2ac.JobSetSpecApplyConfiguration](&jobsetv1alpha2.JobSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: jobsetv1alpha2.GroupVersion.String(),
			Kind:       "JobSet",
		},
		Spec: jobSetTemplateSpec.Spec,
	}, "spec")
	if err != nil {
		return nil, err
	}

	opts := []runtime.InfoOption{
		runtime.WithLabels(propagationLabels),
		runtime.WithAnnotations(propagationAnnotations),
		runtime.WithMLPolicySource(mlPolicy),
		runtime.WithPodGroupPolicy(podGroupPolicy),
		runtime.WithTemplateSpecObjApply(jobSetSpecApply),
	}

	draCounts := make([]int, len(jobSetSpecApply.ReplicatedJobs))

	for i, rJob := range jobSetSpecApply.ReplicatedJobs {
		// TODO: Support multiple replicas ('.template.spec.replicatedJobs[*].replicas') for replicated Jobs.
		// REF: https://github.com/kubeflow/trainer/issues/2318
		count := ptr.Deref(rJob.Template.Spec.Parallelism, 1)
		var ancestor *string
		if metadata := rJob.Template.ObjectMetaApplyConfiguration; metadata != nil && metadata.Labels != nil {
			if labelAncestor, ok := metadata.Labels[constants.LabelTrainJobAncestor]; ok {
				if labelAncestor == constants.AncestorTrainer && mlPolicy != nil {
					count = ptr.Deref(mlPolicy.NumNodes, 1)
				}
				ancestor = &labelAncestor
			}
		}
		isTrainerAncestor := ancestor != nil && *ancestor == constants.AncestorTrainer && mlPolicy != nil
		isMPILauncherAsNode := mlPolicy != nil && mlPolicy.MPI != nil &&
			ptr.Deref(mlPolicy.MPI.RunLauncherAsNode, false) && ptr.Deref(rJob.Name, "") == constants.Node
		if isTrainerAncestor || isMPILauncherAsNode {
			if applyPodSpec := jobSetSpecApply.ReplicatedJobs[i].Template.Spec.Template.Spec; applyPodSpec != nil {
				if jobTrainer := trainJob.Spec.Trainer; jobTrainer != nil {
					if jobTrainer.ResourcesPerNode != nil {
						for k := range applyPodSpec.Containers {
							if ptr.Deref(applyPodSpec.Containers[k].Name, "") != constants.Node {
								continue
							}
							var baseRes corev1.ResourceRequirements
							if r := applyPodSpec.Containers[k].Resources; r != nil {
								if r.Limits != nil {
									baseRes.Limits = *r.Limits
								}
								if r.Requests != nil {
									baseRes.Requests = *r.Requests
								}
							}
							mergedRes, mergeErr := trainingruntimeutil.MergeResourceRequirements(
								baseRes, *trainJob.Spec.Trainer.ResourcesPerNode)
							if mergeErr != nil {
								return nil, mergeErr
							}
							applyRes := &corev1ac.ResourceRequirementsApplyConfiguration{}
							if mergedRes.Limits != nil {
								limits := maps.Clone(mergedRes.Limits)
								applyRes.Limits = &limits
							}
							if mergedRes.Requests != nil {
								requests := maps.Clone(mergedRes.Requests)
								applyRes.Requests = &requests
							}
							// Preserve existing DRA claims — resourcesPerNode only overrides requests/limits.
							if old := applyPodSpec.Containers[k].Resources; old != nil {
								applyRes.Claims = old.Claims
							}
							applyPodSpec.Containers[k].Resources = applyRes
							jobSetTemplateSpec.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Resources = mergedRes
							break
						}
					}
					if len(jobTrainer.ResourceClaimsPerNode) > 0 {
						applyTrainerResourceClaimsPerNode(applyPodSpec, jobTrainer.ResourceClaimsPerNode)
					}
				}
				draCounts[i] = r.resolveDRAGPUCount(ctx, trainJob.Namespace, applyPodSpec)
			}
		}
		opts = append(opts, runtime.WithPodSet(
			*rJob.Name,
			ancestor,
			count,
			*jobSetTemplateSpec.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.DeepCopy(),
			rJob.Template.Spec.Template.Spec),
		)
	}

	info := runtime.NewInfo(opts...)
	for i, count := range draCounts {
		if count > 0 && i < len(info.TemplateSpec.PodSets) {
			info.TemplateSpec.PodSets[i].DRAGPUCount = count
		}
	}
	return info, nil
}

func (r *TrainingRuntime) mergeRuntimePatches(trainJob *trainer.TrainJob, jobSetTemplateSpec *trainer.JobSetTemplateSpec) error {
	// Capture the original ReplicatedJobs ordering since SMP may reorder the list.
	order := make(map[string]int, len(jobSetTemplateSpec.Spec.ReplicatedJobs))
	for i, rJob := range jobSetTemplateSpec.Spec.ReplicatedJobs {
		order[rJob.Name] = i
	}

	for _, runtimePatch := range trainJob.Spec.RuntimePatches {
		if runtimePatch.TrainingRuntimeSpec == nil ||
			runtimePatch.TrainingRuntimeSpec.Template == nil ||
			runtimePatch.TrainingRuntimeSpec.Template.Spec == nil {
			continue
		}
		source, err := json.Marshal(jobSetTemplateSpec.Spec)
		if err != nil {
			return err
		}
		patch, err := json.Marshal(runtimePatch.TrainingRuntimeSpec.Template.Spec)
		if err != nil {
			return err
		}
		merged, err := strategicpatch.StrategicMergePatch(source, patch, jobsetv1alpha2.JobSetSpec{})
		if err != nil {
			return err
		}
		mergedSpec := jobsetv1alpha2.JobSetSpec{}
		if err := json.Unmarshal(merged, &mergedSpec); err != nil {
			return err
		}
		jobSetTemplateSpec.Spec = mergedSpec
	}

	// Restore the ReplicatedJobs order defined in the runtime template.
	sort.SliceStable(jobSetTemplateSpec.Spec.ReplicatedJobs, func(i, j int) bool {
		return order[jobSetTemplateSpec.Spec.ReplicatedJobs[i].Name] <
			order[jobSetTemplateSpec.Spec.ReplicatedJobs[j].Name]
	})

	return nil
}

func (r *TrainingRuntime) TrainJobStatus(ctx context.Context, trainJob *trainer.TrainJob) (*trainer.TrainJobStatus, error) {
	return r.framework.RunTrainJobStatusPlugin(ctx, trainJob)
}

func (r *TrainingRuntime) EventHandlerRegistrars() []runtime.ReconcilerBuilder {
	var builders []runtime.ReconcilerBuilder
	for _, ex := range r.framework.WatchExtensionPlugins() {
		builders = append(builders, ex.ReconcilerBuilders()...)
	}
	return builders
}

func (r *TrainingRuntime) ValidateObjects(ctx context.Context, old, new *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	// Validate against the snapshot the TrainJob was built from, falling back to the live
	// runtime when no snapshot exists yet, as NewObjects does.
	trainingRuntime := &trainer.TrainingRuntime{}
	if err := getRuntimeSnapshot(ctx, r.client, new, trainingRuntime); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, field.ErrorList{
				field.InternalError(field.NewPath("spec", "runtimeRef"), fmt.Errorf("unable to get runtime snapshot: %w", err)),
			}
		}
		trainingRuntime = &trainer.TrainingRuntime{}
		if err := r.client.Get(ctx, client.ObjectKey{
			Namespace: new.Namespace,
			Name:      new.Spec.RuntimeRef.Name,
		}, trainingRuntime); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, field.ErrorList{
					field.InternalError(field.NewPath("spec", "runtimeRef"), fmt.Errorf("unable to get trainingRuntime: %w", err)),
				}
			}
			return nil, field.ErrorList{
				field.Invalid(field.NewPath("spec", "runtimeRef"), new.Spec.RuntimeRef,
					fmt.Sprintf("%v: specified trainingRuntime must be created before the TrainJob is created", err)),
			}
		}
	}
	var warnings admission.Warnings
	if trainingruntimeutil.IsSupportDeprecated(trainingRuntime.Labels) {
		warnings = append(warnings, fmt.Sprintf(
			"Referenced TrainingRuntime \"%s\" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: %s",
			trainingRuntime.Name,
			constants.RuntimeDeprecationPolicyURL,
		))
	}
	info, _ := r.newRuntimeInfo(ctx, new, trainingRuntime.Spec.Template, trainingRuntime.Spec.MLPolicy, trainingRuntime.Spec.PodGroupPolicy) // ignoring the error here as the runtime configured should be valid
	fwWarnings, errs := r.framework.RunCustomValidationPlugins(ctx, info, old, new)
	if len(fwWarnings) != 0 {
		warnings = append(warnings, fwWarnings...)
	}
	return warnings, errs
}

// applyTrainerResourceClaimsPerNode upserts the TrainJob's resourceClaimsPerNode into the Pod's
// resourceClaims by name and references every claim from the node container's resources.claims.
func applyTrainerResourceClaimsPerNode(podSpec *corev1ac.PodSpecApplyConfiguration, claims []trainer.TrainerResourceClaim) {
	for _, claim := range claims {
		found := false
		for j := range podSpec.ResourceClaims {
			if podSpec.ResourceClaims[j].Name != nil && *podSpec.ResourceClaims[j].Name == claim.Name {
				podSpec.ResourceClaims[j] = *corev1ac.PodResourceClaim().
					WithName(claim.Name).
					WithResourceClaimTemplateName(claim.ResourceClaimTemplateName)
				found = true
				break
			}
		}
		if !found {
			podSpec.ResourceClaims = append(podSpec.ResourceClaims, *corev1ac.PodResourceClaim().
				WithName(claim.Name).
				WithResourceClaimTemplateName(claim.ResourceClaimTemplateName))
		}

		// Wire the claim into the node container's resources.claims.
		for k := range podSpec.Containers {
			if ptr.Deref(podSpec.Containers[k].Name, "") != constants.Node {
				continue
			}
			if podSpec.Containers[k].Resources == nil {
				podSpec.Containers[k].Resources = &corev1ac.ResourceRequirementsApplyConfiguration{}
			}
			claimRef := corev1ac.ResourceClaim().WithName(claim.Name)
			claimFound := false
			for m := range podSpec.Containers[k].Resources.Claims {
				if podSpec.Containers[k].Resources.Claims[m].Name != nil &&
					*podSpec.Containers[k].Resources.Claims[m].Name == claim.Name {
					podSpec.Containers[k].Resources.Claims[m] = *claimRef
					claimFound = true
					break
				}
			}
			if !claimFound {
				podSpec.Containers[k].Resources.Claims = append(
					podSpec.Containers[k].Resources.Claims, *claimRef)
			}
			break
		}
	}
}

// draLookupTimeout bounds how long a single lookup waits for the ResourceClaimTemplate or
// ResourceClaim to be fetched, so a missing RBAC rule or unreachable API server degrades to
// an unknown GPU count instead of blocking the reconciler.
const draLookupTimeout = 10 * time.Second

// resolveDRAGPUCount reads the first resources.claims entry of the node container, resolves the
// referenced pod-level resourceClaim to its ResourceClaimTemplate or ResourceClaim, and returns
// the GPU count from the device requests. It returns 0 when nothing is referenced or the object
// cannot be read: GPU detection never fails the TrainJob.
func (r *TrainingRuntime) resolveDRAGPUCount(ctx context.Context, namespace string, podSpec *corev1ac.PodSpecApplyConfiguration) int {
	// Find the node container's first claim reference.
	var claimName string
	for _, c := range podSpec.Containers {
		if ptr.Deref(c.Name, "") != constants.Node {
			continue
		}
		if c.Resources == nil || len(c.Resources.Claims) == 0 {
			return 0
		}
		claimName = ptr.Deref(c.Resources.Claims[0].Name, "")
		break
	}
	if claimName == "" {
		return 0
	}
	ctx, cancel := context.WithTimeout(ctx, draLookupTimeout)
	defer cancel()

	// Resolve the pod-level claim to find the template or direct claim name.
	for _, pc := range podSpec.ResourceClaims {
		if ptr.Deref(pc.Name, "") != claimName {
			continue
		}
		if pc.ResourceClaimTemplateName != nil && *pc.ResourceClaimTemplateName != "" {
			var rct resourcev1.ResourceClaimTemplate
			if err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: *pc.ResourceClaimTemplateName}, &rct); err != nil {
				log := ctrl.LoggerFrom(ctx)
				log.V(0).Info("Failed to resolve ResourceClaimTemplate for DRA GPU detection", "name", *pc.ResourceClaimTemplateName, "error", err)
				return 0
			}
			return runtime.NumGPUFromDeviceRequests(rct.Spec.Spec.Devices.Requests)
		}
		if pc.ResourceClaimName != nil && *pc.ResourceClaimName != "" {
			var rc resourcev1.ResourceClaim
			if err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: *pc.ResourceClaimName}, &rc); err != nil {
				log := ctrl.LoggerFrom(ctx)
				log.V(0).Info("Failed to resolve ResourceClaim for DRA GPU detection", "name", *pc.ResourceClaimName, "error", err)
				return 0
			}
			return runtime.NumGPUFromDeviceRequests(rc.Spec.Devices.Requests)
		}
		return 0
	}
	return 0
}
