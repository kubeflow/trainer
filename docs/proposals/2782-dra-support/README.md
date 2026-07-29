# KEP-2782: Dynamic Resource Allocation (DRA) Support for Kubeflow Trainer

Authors:

- Sridhar Pillai (Red Hat)

## Summary

[Dynamic Resource Allocation (DRA)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)  
graduated to GA in Kubernetes 1.34, providing a modern alternative to extended resources for  
GPUs and accelerators. This KEP adds a top-level `resourceClaimsPerNode` field on `Trainer`,  
next to `resourcesPerNode`, so data scientists can request DRA devices with the same UX they  
already use for CPU/memory/GPU counts. The controller applies these claims to the trainer  
node PodSpec and wires container-level `resources.claims` on the `node` container. It also  
updates GPU auto-detection so `numProcPerNode` continues to work with DRA.

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
4. **Kubeflow Trainer has no first-class DRA UX today.** Users set GPU counts via top-level
  `resourcesPerNode`, but DRA claims would otherwise require deep `runtimePatches` nesting.
   That splits resource allocation across two APIs and is awkward for a common operation.



### Goals

1. Add `ResourceClaimsPerNode` to `Trainer` so users can request DRA claims at the same
  top-level API as `resourcesPerNode`, with optional `containers` targeting (defaults to
   `node` only).
2. Have the controller apply those claims to the trainer node PodSpec and automatically
  wire container-level `resources.claims` on the target containers.
3. Optionally expose `ResourceClaims` on `PodSpecPatch` for advanced multi-replicatedJob
  overrides via `runtimePatches`.
4. Update GPU auto-detection in ML policy plugins (torch, MPI, XGBoost, Flux) so
  `numProcPerNode` continues to work when DRA is used without extended resources.
5. Add SDK support for listing `ResourceClaimTemplates` in a namespace so users can
  discover available templates before submitting a TrainJob.



### Non-Goals

1. **PodGroup-level ResourceClaims.** Multi-node topology-aware allocation requires Trainer's
  WAS KEP ([#3219](https://github.com/kubeflow/trainer/pull/3219)) to land first.
   Upstream [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) is alpha
   in Kubernetes 1.36. Deferred to Phase 2.
2. **ComputeDomain integration.** IMEX channel support for NVL72/GB200 multi-node training is
  under active prototyping at
   [wg-device-management](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)
   and is not ready for Trainer integration.
3. **Replacing existing** `resources.requests/limits` **GPU scheduling.** Extended resources
  (`nvidia.com/gpu`) remain valid. DRA is an additional scheduling path.



## User Story

A data scientist wants to fine-tune on H100s using DRA. Instead of nesting claims under
`runtimePatches`, they set DRA next to the existing trainer fields:

**Simple case — GPUs on the** `node` **container only (default):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: llama-finetune-h100
  namespace: ml-team
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    image: my-registry/llama-trainer:v2
    numNodes: 2
    resourceClaimsPerNode:
      - name: gpu
        resourceClaimTemplateName: h100-80gb-template
```

When no `containers` field is specified, the controller wires the claim to the `node`
container only. This is the 90% use case.

**Advanced case — targeting specific containers:**

```yaml
    resourceClaimsPerNode:
      - name: gpu
        resourceClaimTemplateName: h100-80gb-template
        containers:
          - node
          - gpu-preprocessor
```

When `containers` is specified, the controller wires the claim **only** to the listed
containers. The claim is not auto-wired to containers not in the list.

The controller:

1. Sets pod-level `resourceClaims` on the trainer `node` replicatedJob.
2. Wires container-level `resources.claims` — on the `node` container by default, or on
  the containers listed in `containers` if specified.
3. Resolves GPU count from the `ResourceClaimTemplate` for `numProcPerNode` auto-detection.

This keeps DRA at the same accessibility level as `resourcesPerNode`.

## Design Details



### API changes



#### Primary: `Trainer.ResourceClaimsPerNode`

Add a top-level field next to `ResourcesPerNode` in `pkg/apis/trainer/v1alpha1/trainjob_types.go`:

```go
type Trainer struct {
	// ... existing fields (image, command, args, env, numNodes) ...

	// resourcesPerNode defines the compute resources for each training node.
	// +optional
	ResourcesPerNode *corev1.ResourceRequirements `json:"resourcesPerNode,omitempty"`

	// resourceClaimsPerNode defines the DRA ResourceClaims for each training node.
	// The controller applies these claims to the trainer node PodSpec and wires
	// container-level resources.claims on the specified containers (defaults to the
	// node container only when Containers is empty).
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=32
	// +optional
	ResourceClaimsPerNode []TrainerResourceClaim `json:"resourceClaimsPerNode,omitempty"`

	// numProcPerNode is the number of processes/workers/slots on every training node.
	// ...
	NumProcPerNode *int32 `json:"numProcPerNode,omitempty"`
}
```

`TrainerResourceClaim` wraps upstream `corev1.PodResourceClaim` and adds container targeting:

```go
type TrainerResourceClaim struct {
	// Name uniquely identifies this resource claim inside the pod (DNS_LABEL).
	Name string `json:"name"`

	// ResourceClaimName is the name of a ResourceClaim object in the same namespace.
	// Exactly one of ResourceClaimName and ResourceClaimTemplateName must be set.
	ResourceClaimName *string `json:"resourceClaimName,omitempty"`

	// ResourceClaimTemplateName is the name of a ResourceClaimTemplate in the same namespace.
	// A new ResourceClaim is created from the template, bound to this pod, and deleted with it.
	// Exactly one of ResourceClaimName and ResourceClaimTemplateName must be set.
	ResourceClaimTemplateName *string `json:"resourceClaimTemplateName,omitempty"`

	// Containers is the list of container names that should receive this claim
	// in their resources.claims. When empty, the claim is wired to the "node"
	// container only (the default training container). When specified, the claim
	// is wired only to the listed containers.
	// +optional
	Containers []string `json:"containers,omitempty"`
}
```



#### How the controller applies it

When `trainer.resourceClaimsPerNode` is set, during `newRuntimeInfo()` / trainer resource
application (same place `resourcesPerNode` is applied today):

1. **Pod-level:** set/merge `PodSpec.ResourceClaims` on the trainer `node` replicatedJob
  from `ResourceClaimsPerNode` (the `Name`, `ResourceClaimName`, and
   `ResourceClaimTemplateName` fields map directly to `corev1.PodResourceClaim`).
2. **Container-level:** for each claim, wire `resources.claims` on the target containers:
  - If `containers` is **empty**: wire to the `node` container only (default).
  - If `containers` is **specified**: wire only to the listed containers. Containers not in
  the list do not get the claim.
3. **Precedence:** if both `resourceClaimsPerNode` and a `runtimePatches` claim override
  exist, `resourceClaimsPerNode` wins for the trainer node (same pattern as
   `resourcesPerNode` overriding runtime container resources).

This avoids the bifurcated UX of setting GPUs via `resourcesPerNode` while setting DRA
via deep `runtimePatches`.

#### Advanced: `PodSpecPatch.ResourceClaims`

Also add `ResourceClaims` to `PodSpecPatch` for advanced cases (non-node replicatedJobs,
platform managers via RuntimePatches):

```go
type PodSpecPatch struct {
	// ... existing fields ...

	// resourceClaims defines which ResourceClaims must be allocated and reserved
	// before the Pod is allowed to start.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=32
	// +optional
	ResourceClaims []corev1.PodResourceClaim `json:"resourceClaims,omitempty"`
}
```

`MaxItems=32` is a pragmatic guard; upstream `PodSpec` has no explicit limit but real-world
DRA usage rarely exceeds a handful of claims per pod.

### Application flow

On first reconciliation, the controller snapshots the runtime config into a ConfigMap per
[KEP-2599](https://github.com/kubeflow/trainer/pull/3428). All subsequent reconciliations
read from this snapshot.

Step-by-step:

1. Admin may optionally define default DRA claims in a `ClusterTrainingRuntime` / `TrainingRuntime`.
2. User sets `trainer.resourceClaimsPerNode` on the `TrainJob` (common path).
3. Controller applies claims to the trainer node PodSpec and wires `node` container
  `resources.claims`.
4. Optional `runtimePatches` can still patch claims for advanced cases.
5. The merged JobSet flows to pods; the DRA scheduler allocates devices from the claim
  template.



### SDK changes

The Kubeflow Training SDK must be updated so users can discover which `ResourceClaimTemplates`
are available before submitting a TrainJob. This mirrors how the SDK already lists
`TrainingRuntime` / `ClusterTrainingRuntime` objects.

**New SDK method:**

```python
def list_resource_claim_templates(self, namespace: str) -> list[ResourceClaimTemplate]:
    """List ResourceClaimTemplates available in the given namespace."""
```

This issues a standard `list` API call against `resource.k8s.io/v1beta1` (or `v1` on  
k8s 1.34+) in the user's namespace and returns the available templates. Users can then  
reference the template name in `resourceClaimsPerNode.resourceClaimTemplateName`.

### GPU count is admin-controlled

With extended resources, users set GPU count directly via `resourcesPerNode`
(e.g., `nvidia.com/gpu: 4`). With DRA, GPU count is defined inside the
`ResourceClaimTemplate` (`spec.devices.requests[].count`).

**This KEP does not allow users to override the device count at the TrainJob level.**

Admins create `ResourceClaimTemplates` with predefined device counts (e.g., templates for
2, 4, or 8 GPUs). Users select which template to use but cannot change the count. This is
intentional:

- Prevents fragmented GPU utilization (e.g., user requesting 5 of 8 GPUs leaves 3 stranded)
- Keeps admins in control of hardware allocation policies
- Aligns with how DRA is designed — the template is the unit of allocation

If user-level count overrides are needed, they can be revisited based on user feedback.

### ClusterTrainingRuntime and DRA

With `resourceClaimsPerNode` on the TrainJob, admins do **not** need to pre-populate
`ClusterTrainingRuntimes` with DRA claims. Users add claims directly via the TrainJob API.

Admins **can** still set default DRA claims in a `ClusterTrainingRuntime` if they want a
"batteries-included" experience, but they must ensure the referenced
`ResourceClaimTemplate` exists in every namespace where TrainJobs run (since templates are
namespace-scoped). This is the admin's responsibility, not Trainer's.

The recommended path for Phase 1: users add claims via `resourceClaimsPerNode`, admins
create `ResourceClaimTemplates` in the workload namespaces.

### DRA-aware GPU detection in ML policy plugins

Today, `GetNumGPUPerNode()` derives GPU count by scanning `resources.Requests` and
`resources.Limits` for resource names containing "gpu". When a pod uses DRA claims
instead of extended resources, this function returns 0, causing the torch plugin to
fall back to CPU-based `numProcPerNode` (wrong for GPU training).

**Approach:** The DRA GPU count is resolved in the core runtime layer during PodSet
construction, where the controller's `client.Client` and `context.Context` are already
available. After claims are applied to the merged `JobSetTemplateSpec`, the core runtime
inspects each merged PodSpec's `resourceClaims`. For each `PodResourceClaim` referencing a
`ResourceClaimTemplate`, it looks up the template object and sums the device request counts
for GPU-class requests.

The resolved count is propagated to ML policy plugins (torch, torchtune, MPI, XGBoost,
Flux) via the existing `PodSet` struct. Each plugin uses the DRA count as a fallback
when `GetNumGPUPerNode()` returns 0.

This design:

- Avoids changing the `EnforceMLPolicyPlugin` interface (which has no `context.Context`)
- Avoids adding `client.Client` to plugin structs that do not currently store one
- Keeps the `GetNumGPUPerNode()` signature backward compatible with all existing callers

Extended resources still take priority. DRA count is only used as a fallback when no
`nvidia.com/gpu` (or similar) extended resource is found. If the
`ResourceClaimTemplate` cannot be resolved (not found, different namespace), the DRA
GPU count defaults to 0 and the user can always set `numProcPerNode` explicitly.

**RBAC:** The controller's `ServiceAccount` needs `get` permission on
`resourceclaimtemplates` in the `resource.k8s.io` API group. This is added to the
controller's `ClusterRole` manifest.

### Validation

- Reject `trainer.resourcesPerNode.claims` if set. That field is ignored by Trainer today;
users should use `resourceClaimsPerNode` instead.
- Kubernetes still rejects invalid pods at admission time: malformed claim references,
missing DRA drivers, and cross-namespace template references surface as standard Pod
scheduling or admission errors.
- Trainer does not pre-validate `ResourceClaimTemplate` existence (eventual consistency).



### Edge cases and error handling


| Scenario                                                  | Behavior                                                                                                                                                        |
| --------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Cluster has no DRA driver (or k8s < 1.34)**             | Pods with `resourceClaims` stay `Pending` indefinitely. Standard Kubernetes behavior; users must ensure a DRA driver is installed.                              |
| **Referenced** `ResourceClaimTemplate` **does not exist** | DRA scheduler plugin cannot create a `ResourceClaim`. Pods stay `Pending` with `FailedScheduling` event. Trainer does not pre-validate template existence.      |
| `**resourcesPerNode.claims` is set**                      | Webhook rejects the TrainJob and points users to `resourceClaimsPerNode`.                                                                                       |
| `**ResourceClaimTemplate` is in a different namespace**   | Kubernetes rejects cross-namespace references. Template must be in the same namespace as the TrainJob.                                                          |
| **DRA claims present but template not resolved by core**  | DRA GPU count remains 0; torch falls back to CPU-based `numProcPerNode`. User can set `numProcPerNode` explicitly.                                              |
| **Both extended resources AND DRA claims present**        | Extended resources take priority for `numProcPerNode`. Users should avoid mixing both to prevent double GPU allocation.                                         |
| **Different GPU counts**                                  | Device count lives in the `ResourceClaimTemplate`. Different counts require different templates. Admins create templates for each count; users cannot override. |
| **User wants claim on sidecar, not node**                 | User sets `containers: [sidecar-name]` on the claim. The controller wires the claim only to that container; the `node` container does not get it.               |
| `containers` **lists a name that does not exist**         | Kubernetes rejects the pod at admission (container `resources.claims` references a non-existent container). Trainer does not pre-validate container names.      |


After adding the fields, run `make generate` to regenerate deep copy methods, OpenAPI schema,
and CRD manifests.

### Files modified


| File                                                     | Change                                                                       |
| -------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `pkg/apis/trainer/v1alpha1/trainjob_types.go`            | Add `ResourceClaimsPerNode` to `Trainer`; `ResourceClaims` to `PodSpecPatch` |
| `pkg/apis/trainer/v1alpha1/zz_generated.deepcopy.go`     | Regenerated via `make generate`                                              |
| `pkg/apis/trainer/v1alpha1/zz_generated.openapi.go`      | Regenerated via `make generate`                                              |
| `manifests/base/crds/`                                   | Regenerated CRD YAMLs with new fields                                        |
| `pkg/runtime/core/trainingruntime.go`                    | Apply `resourceClaimsPerNode` to node PodSpec + container claims             |
| `pkg/runtime/runtime.go`                                 | Propagate DRA GPU count via `PodSet`                                         |
| `pkg/webhooks/trainjob_webhook.go`                       | Reject `resourcesPerNode.claims`; point to `resourceClaimsPerNode`           |
| `pkg/runtime/framework/plugins/torch/torch.go`           | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/torch/torchtune.go`       | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/mpi/mpi.go`               | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/xgboost/xgboost.go`       | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/flux/flux.go`             | Use DRA GPU count as fallback when GPU count is 0                            |
| `manifests/base/rbac/`                                   | Add `resourceclaimtemplates` get permission to ClusterRole                   |
| `pkg/runtime/core/trainingruntime_test.go`               | Test top-level claims application, container targeting, and merge behavior   |
| `sdk/python/kubeflow/trainer/api_client.py` (or similar) | Add `list_resource_claim_templates(namespace)` method                        |




### Test plan

- [x] I/we understand the owners of the involved components may require updates to

existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Unit tests

`**pkg/runtime/core/trainingruntime_test.go`:**

- `resourceClaimsPerNode` set with no `containers`: node PodSpec gets claims and `node` container gets `resources.claims`
- `resourceClaimsPerNode` set with `containers: [node, sidecar]`: both containers get `resources.claims`
- `resourceClaimsPerNode` set with `containers: [sidecar]`: only sidecar gets `resources.claims`, not `node`
- Runtime template has default claims, user sets `resourceClaimsPerNode`: user claims win on trainer node
- User also patches via `runtimePatches`: `resourceClaimsPerNode` still wins for trainer node
- Empty `resourceClaimsPerNode`: runtime defaults preserved
- DRA GPU count resolution from referenced `ResourceClaimTemplate`

`**pkg/runtime/framework/plugins/torch/torch_test.go`:**

- DRA GPU count > 0, no extended resources: `numProcPerNode` derived from DRA count
- DRA GPU count > 0 with `numProcPerNode` explicitly set: explicit value wins
- DRA GPU count is 0, no extended resources: falls back to CPU count

`**pkg/webhooks/`:**

- `resourcesPerNode.claims` set: admission rejects with pointer to `resourceClaimsPerNode`



#### Integration tests

`**test/integration/controller/**` (Ginkgo):

- Create `TrainJob` with `resourceClaimsPerNode`: verify node PodSpec claims and container
`resources.claims` are set
- Create `TrainJob` with both runtime defaults and `resourceClaimsPerNode`: verify user wins
- Create `TrainJob` with `resourcesPerNode.claims`: verify webhook rejection



#### E2E tests

Deferred until a DRA-capable test cluster is available in CI. The
[dra-example-driver](https://github.com/kubernetes-sigs/dra-example-driver) can be used
for E2E testing without real GPUs, following the approach used by
[Kueue](https://github.com/kubernetes-sigs/kueue) for its DRA E2E tests.

## Other considered alternatives



### RuntimePatches / `PodSpecPatch` only

Expose DRA only via deep `runtimePatches`. **Rejected as the primary UX:** GPU type/count
changes are common; nesting under PodSpecPatches creates a bifurcated API next to
`resourcesPerNode`. Kept as an advanced escape hatch only.

### Surface claims via `Trainer.ResourcesPerNode.Claims`

`corev1.ResourceRequirements` already has a `Claims` field. **Rejected:** The builder
historically reads `Limits` / `Requests` only. Semantically mixing quantitative resources
and DRA claim refs in one field is confusing. Prefer an explicit `resourceClaimsPerNode`
and webhook-reject `resourcesPerNode.claims`.

### Add claims at the JobSet level

**Rejected:** Upstream JobSet does not support `ResourceClaimTemplates` at the JobSet level.
Pod-level claims are the GA path. Different ReplicatedJobs may need different GPU types.

### Add a new top-level field on `TrainJobSpec` (not under `Trainer`)

**Rejected:** Claims are per training node, same scope as `resourcesPerNode`. Putting them
under `Trainer` keeps the resource API together.

## Future Work (Phase 2)

1. **PodGroup-level ResourceClaims via Workload API.** Depends on upstream
  [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) (alpha in k8s 1.36)
   and the Trainer WAS KEP ([#3219](https://github.com/kubeflow/trainer/pull/3219)). Enables
   shared device allocation across all pods in a training job.
2. **Controller-managed template provisioning.** `ResourceClaimTemplates` are namespaced;
  cluster-scoped runtimes cannot reference them directly across namespaces. Explore
   controller copy/sync of templates into the workload namespace (or a standard external
   operator) so admins/users are not asked to create templates in every namespace.
3. **User-level GPU count overrides.** If user feedback demands it, allow overriding
  the device count from the `ResourceClaimTemplate` at the TrainJob level. Currently
   excluded to prevent fragmented GPU utilization.
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

