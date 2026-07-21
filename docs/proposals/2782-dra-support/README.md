# KEP-2782: Dynamic Resource Allocation (DRA) Support for Kubeflow Trainer

Authors:

- Sridhar Pillai (Red Hat)

## Summary

[Dynamic Resource Allocation (DRA)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
graduated to GA in Kubernetes 1.34, providing a modern alternative to extended resources for
GPUs and accelerators. This KEP adds a `resourceClaims` field to `PodSpecPatch` so that data scientists can override
or add pod-level DRA claims via `runtimePatches` in their `TrainJob` specs. Admins already
configure claims in `ClusterTrainingRuntime` templates via the full `PodSpec`; this KEP only
closes the gap for user overrides through `runtimePatches`.

## Motivation

Kubernetes DRA replaces the rigid extended-resource model (`nvidia.com/gpu: 1`) with a flexible,
structured API for device allocation:

1. **DRA is the future of GPU scheduling.** Major cloud providers and hardware vendors ship DRA
  drivers for their GPUs. Extended resources will remain supported but are increasingly a
   compatibility path.
2. **DRA enables user-defined sharing policies.** MIG partitioning and GPU timeslicing move
  from admin-only device plugin config into `DeviceClass` and `ResourceClaimTemplate`,
   letting platform teams offer multiple GPU profiles from the same cluster.
3. **Training workloads are the primary consumer.** Distributed training jobs are the largest
  GPU consumers in Kubernetes. Trainer must provide first-class DRA support.
4. **Kubeflow Trainer has no DRA support today.** The `PodSpecPatch` type does not include
  `resourceClaims`. This is the only gap; the merge pipeline already handles the field.

### Goals

1. Add `ResourceClaims` to `PodSpecPatch` so users can override or add pod-level DRA claims
  via `runtimePatches` in `TrainJob`.
2. Rely on the existing strategic merge patch pipeline to merge claims by name with no
  controller changes.

### Non-Goals

1. **PodGroup-level ResourceClaims.** Multi-node topology-aware allocation requires Trainer's
  WAS KEP ([#3219](https://github.com/kubeflow/trainer/pull/3219)) to land first.
   Upstream [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) is alpha
   in Kubernetes 1.36. Deferred to Phase 2.
2. **ComputeDomain integration.** IMEX channel support for NVL72/GB200 multi-node training is
  under active prototyping at
   [wg-device-management](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)
   and is not ready for Trainer integration.
3. **Replacing existing `resources.requests/limits` GPU scheduling.** Extended resources
  (`nvidia.com/gpu`) remain valid. DRA is an additional scheduling path.

## User Story

A `ClusterTrainingRuntime` named `torch-distributed-a100` already defines a pod-level
`resourceClaims` entry named `gpu` pointing at an A100 `ResourceClaimTemplate`, with
container-level `resources.claims` pre-wired in the template. A data scientist wants H100
instead and overrides the claim via `runtimePatches`.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: llama-finetune-h100
  namespace: ml-team
spec:
  runtimeRef:
    name: torch-distributed-a100
  trainer:
    image: my-registry/llama-trainer:v2
    numNodes: 2
  runtimePatches:
    - manager: user
      trainingRuntimeSpec:
        template:
          spec:
            replicatedJobs:
              - name: node
                template:
                  spec:
                    template:
                      spec:
                        resourceClaims:
                          - name: gpu
                            resourceClaimTemplateName: h100-80gb-template
```

The `runtimePatches` path mirrors the `RuntimePatch` -> `TrainingRuntimeSpecPatch` ->
`JobSetTemplatePatch` -> `JobSetSpecPatch` -> `ReplicatedJobPatch` -> `JobTemplatePatch` ->
`JobSpecPatch` -> `PodTemplatePatch` -> `PodSpecPatch` struct hierarchy defined in
`trainjob_types.go`. The strategic merge patch replaces the `gpu` claim (matched by name)
with the user's H100 template, while preserving all other runtime configuration.

## Design Details

### API changes

Add a `ResourceClaims` field to `PodSpecPatch` in `pkg/apis/trainer/v1alpha1/trainjob_types.go`:

```go
type PodSpecPatch struct {
	// ... existing fields (serviceAccountName, volumes, initContainers,
	// containers, imagePullSecrets, securityContext, nodeSelector,
	// affinity, tolerations, schedulingGates) ...

	// resourceClaims defines which ResourceClaims must be allocated and reserved
	// before the Pod is allowed to start. These claims are merged with any claims
	// defined in the TrainingRuntime template via strategic merge patch.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=32
	// +optional
	ResourceClaims []corev1.PodResourceClaim `json:"resourceClaims,omitempty"`
}
```

The upstream `corev1.PodResourceClaim` struct has three fields:

```go
type PodResourceClaim struct {
	// Name uniquely identifies this resource claim inside the pod (DNS_LABEL).
	Name string `json:"name"`

	// ResourceClaimName is the name of a ResourceClaim object in the same namespace.
	// Exactly one of ResourceClaimName and ResourceClaimTemplateName must be set.
	ResourceClaimName *string `json:"resourceClaimName,omitempty"`

	// ResourceClaimTemplateName is the name of a ResourceClaimTemplate in the same namespace.
	// A new ResourceClaim is created from the template, bound to this pod, and deleted with it.
	// Exactly one of ResourceClaimName and ResourceClaimTemplateName must be set.
	ResourceClaimTemplateName *string `json:"resourceClaimTemplateName,omitempty"`
}
```

The field mirrors existing list fields in `PodSpecPatch`: `+listType=map` with
`+listMapKey=name` for strategic merge patch support, and `MaxItems=32` to bound the list.
Uses `corev1.PodResourceClaim` directly, consistent with how `corev1.Volume` and
`corev1.Toleration` are used in `PodSpecPatch`.

**Container-level claim references** cannot be set through `runtimePatches` because
`ContainerPatch` does not expose a `Resources` field. Admins must pre-configure
container-level `resources.claims` in the runtime template. Adding `Resources` to
`ContainerPatch` is planned as a fast follow-up so users can add container-level
references via `runtimePatches` without waiting for Phase 2.

### Strategic merge patch flow

The existing merge pipeline in `pkg/runtime/core/trainingruntime.go` handles `ResourceClaims`
with no controller changes. `mergeRuntimePatches()` JSON-marshals the runtime snapshot and
user patch, applies `strategicpatch.StrategicMergePatch` on `batchv1.JobTemplateSpec`, and
unmarshals the result. Because upstream `corev1.PodSpec.ResourceClaims` uses
`+listType=map` and `+listMapKey=name`, claims are merged or replaced by name automatically.

On first reconciliation, the controller snapshots the runtime config into a ConfigMap per
[KEP-2599](https://github.com/kubeflow/trainer/pull/3428). All subsequent reconciliations
read from this snapshot, so DRA claims in the runtime are frozen at snapshot time.

Step-by-step:

1. Admin defines `ClusterTrainingRuntime` with `resourceClaims` and container-level
  `resources.claims` in the full `PodSpec` template (already supported today).
2. User creates `TrainJob` with `runtimePatches` containing `PodSpecPatch.ResourceClaims`.
3. Controller calls `mergeRuntimePatches()`, which merges claims by name. A user patch
  replacing `gpu` with an H100 template wins over the runtime's A100 template.
4. The merged `JobTemplateSpec` flows into the JobSet apply configuration and then to pods,
  where the DRA scheduler plugin allocates devices from the resolved claim template.

**Torch plugin note:** The torch plugin's `GetNumGPUPerNode()` does not recognize DRA claims.
Users must set `numProcPerNode` explicitly when using DRA without extended resources. A
follow-up enhancement to inspect `resourceClaims` for GPU count is recommended.

### Validation

No Trainer-specific validation is required for Phase 1. Kubernetes rejects invalid pods at
admission time: malformed claim references, container `resources.claims` pointing to unknown
pod-level claim names, and missing DRA drivers all surface as standard Pod scheduling or
admission errors. Trainer does not pre-validate `ResourceClaimTemplate` existence
(eventual consistency).

### Edge cases and error handling


| Scenario                                                | Behavior                                                                                                                                                   |
| ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Cluster has no DRA driver (or k8s < 1.34)**           | Pods with `resourceClaims` stay `Pending` indefinitely. Standard Kubernetes behavior; users must ensure a DRA driver is installed.                         |
| **Referenced `ResourceClaimTemplate` does not exist**   | DRA scheduler plugin cannot create a `ResourceClaim`. Pods stay `Pending` with `FailedScheduling` event. Trainer does not pre-validate template existence. |
| **Invalid claim in `runtimePatches`**                   | Pod admission rejects the resulting Pod spec.                                                                                                              |
| `**ResourceClaimTemplate` is in a different namespace** | Kubernetes rejects cross-namespace references. `ResourceClaimTemplateName` must reference a template in the same namespace as the pod.                     |


After adding the field, run `make generate` to regenerate deep copy methods, OpenAPI schema,
and CRD manifests.

### Files modified


| File                                                 | Change                                       |
| ---------------------------------------------------- | -------------------------------------------- |
| `pkg/apis/trainer/v1alpha1/trainjob_types.go`        | Add `ResourceClaims` field to `PodSpecPatch` |
| `pkg/apis/trainer/v1alpha1/zz_generated.deepcopy.go` | Regenerated via `make generate`              |
| `pkg/apis/trainer/v1alpha1/zz_generated.openapi.go`  | Regenerated via `make generate`              |
| `manifests/base/crds/`                               | Regenerated CRD YAMLs with new field         |
| `pkg/runtime/core/trainingruntime_test.go`           | Test patch merging with `resourceClaims`     |


### Test plan

- [x] I/we understand the owners of the involved components may require updates to

existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Unit tests

`**pkg/runtime/core/trainingruntime_test.go**` (add cases to `TestTrainingRuntimeNewObjects`):

- Runtime template has resourceClaims, no user patch: claims preserved in JobSet
- User patches resourceClaims via runtimePatches: claims merged by name
- User adds new claims alongside runtime defaults: both present
- Empty resourceClaims list in patch: does not clear existing claims
- Patch targets non-existent replicatedJob name: patch skipped, no error

#### Integration tests

`**test/integration/controller/**` (Ginkgo):

- Create `ClusterTrainingRuntime` with DRA claims, then `TrainJob`: verify resulting
JobSet pods contain the correct `resourceClaims` in `PodSpec`
- Create `TrainJob` with `runtimePatches` overriding claims: verify merge behavior

#### E2E tests

Deferred until a DRA-capable test cluster is available in CI. The
[dra-example-driver](https://github.com/kubernetes-sigs/dra-example-driver) can be used
for E2E testing without real GPUs, following the approach used by
[Kueue](https://github.com/kubernetes-sigs/kueue) for its DRA E2E tests.

## Other considered alternatives

### Surface claims via `Trainer.ResourcesPerNode`

`corev1.ResourceRequirements` includes a `Claims` field in k8s 1.28+. **Rejected:** The
builder only reads `Limits` and `Requests`. Semantically incorrect since DRA claims are
declarative device requests, not quantitative resource requirements.

### Add claims at the JobSet level

**Rejected:** Upstream JobSet does not support `ResourceClaimTemplates` at the JobSet level.
Pod-level claims are the only GA path in k8s 1.34. Different ReplicatedJobs may need
different GPU types, which is incompatible with JobSet-level sharing.

### Add a new top-level TrainJob field

Add a `ResourceClaims` field directly on `TrainJobSpec`. **Rejected:** Breaks the
`RuntimePatches` pattern and would require special-case merge logic. The `PodSpecPatch`
approach is consistent with how `volumes`, `tolerations`, and `nodeSelector` are exposed.

## Future Work (Phase 2)

1. **PodGroup-level ResourceClaims via Workload API.** Depends on upstream
  [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) (alpha in k8s 1.36)
   and the Trainer WAS KEP ([#3219](https://github.com/kubeflow/trainer/pull/3219)). Enables
   shared device allocation across all pods in a training job.
2. **Torch plugin DRA-aware GPU detection.** Update `GetNumGPUPerNode()` to inspect
  `resourceClaims` and derive GPU count from `ResourceClaimTemplate` device request counts,
   removing the requirement for users to set `numProcPerNode` explicitly with DRA.
3. `**Resources` on `ContainerPatch`.** Allow users to add container-level `resources.claims`
  references via `runtimePatches` without requiring admin pre-wiring in the runtime template.
4. **ComputeDomain integration for topology-aware scheduling.** Multi-node device allocation
  for NVL72/GB200 systems via  
   [wg-device-management](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)  
   PodGroup-level claims with ComputeDomain support.

## References

- [Kubernetes DRA documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
- [DRA GA in Kubernetes 1.34](https://kubernetes.io/blog/2025/09/01/kubernetes-v1-34-dra-updates)
- [KEP-5729: DRA ResourceClaim for Workloads](https://github.com/kubernetes/enhancements/issues/5729)
- [KEP-2599: Runtime Snapshot](https://github.com/kubeflow/trainer/pull/3428)
- [GitHub Issue #2782: DRA Support for Trainer](https://github.com/kubeflow/trainer/issues/2782)
- [WAS KEP PR #3219](https://github.com/kubeflow/trainer/pull/3219)
- [wg-device-management topology prototyping](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)
- [Slack thread: DRA discussion (Aug 2025)](https://cloud-native.slack.com/archives/C0742LDFZ4K/p1754410574841529)
- [Slack thread: DRA scope (May 2026)](https://cloud-native.slack.com/archives/C0742LDFZ4K/p1779107242466099)

