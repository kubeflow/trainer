# KEP-2782: Dynamic Resource Allocation (DRA) Support for Kubeflow Trainer

Authors:

- Sridhar Pillai (Red Hat)

## Summary

[Dynamic Resource Allocation (DRA)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
graduated to GA in Kubernetes 1.34, providing a modern alternative to extended resources for
GPUs and accelerators. This KEP adds a top-level `resourceClaimsPerNode` field on `Trainer`,
next to `resourcesPerNode`, so data scientists can request DRA devices with the same UX they
already use for CPU/memory/GPU counts. The controller applies these claims to the trainer
node PodSpec and wires container-level `resources.claims` on the `node` container. Claims for
other containers (sidecars, init containers) are configured via the `runtimePatches` API. It also
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
  top-level API as `resourcesPerNode`.
2. Have the controller apply those claims to the trainer node PodSpec and automatically
  wire container-level `resources.claims` on the `node` container.
3. Expose `ResourceClaims` on `PodSpecPatch` and `Resources` on `ContainerPatch` so claims can
  be attached to any replicatedJob and any container (sidecars, init containers) via
   `runtimePatches`.
4. Update GPU auto-detection in ML policy plugins (torch, MPI, XGBoost, Flux) so
  `numProcPerNode` continues to work when DRA is used without extended resources.
5. Add SDK support for listing `ResourceClaimTemplates` in a namespace so users can
  discover available templates before submitting a TrainJob.



### Non-Goals

1. **PodGroup-level ResourceClaims.** Multi-node topology-aware allocation requires Trainer's
  WAS KEP ([#3219](https://github.com/kubeflow/trainer/pull/3219)) to land first.
   Upstream [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) is alpha
   in Kubernetes 1.36 and beta in 1.37, so #3219 is the remaining gate. Deferred to Phase 2.
2. **ComputeDomain integration.** IMEX channel support for NVL72/GB200 multi-node training is
  under active prototyping at
   [wg-device-management](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)
   and is not ready for Trainer integration.
3. **Replacing existing** `resources.requests/limits` **GPU scheduling.** Extended resources
  (`nvidia.com/gpu`) remain valid. DRA is an additional scheduling path.
4. **Automated ResourceClaimTemplate creation across namespaces.** Templates are
  namespace-scoped; this KEP does not automate copying or syncing them into workload
   namespaces. Admins create templates in each namespace manually (or via external tooling).
   Controller-managed provisioning is deferred to Phase 2.
5. **Direct** `ResourceClaim` **references in** `resourceClaimsPerNode`**.** The top-level API is
  template-only (`resourceClaimTemplateName`): each training node Pod gets its own claim, which
   is what node-local devices need. A pre-created, shared `ResourceClaim` (`resourceClaimName`)
   is still reachable through the `runtimePatches` escape hatch, which uses the upstream
   `corev1.PodResourceClaim` type, and can be added to `resourceClaimsPerNode` later without a
   breaking change if users ask for it.



## User Stories

### Admin: setting up DRA accelerator options

A platform admin wants to offer H100 and A100 GPU options to data scientists. They create
`ResourceClaimTemplates` in each workload namespace:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: h100-80gb-template
  namespace: ml-team
spec:
  spec:
    devices:
      requests:
        - name: gpu # "gpu" in the request name enables numProcPerNode auto-detection
          exactly:
            deviceClassName: h100
            count: 8
---
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: a100-40gb-template
  namespace: ml-team
spec:
  spec:
    devices:
      requests:
        - name: gpu
          exactly:
            deviceClassName: a100
            count: 4
```

The admin repeats this for each workload namespace. For `numProcPerNode` auto-detection to
work, the device request `name` in the template must contain "gpu" (see
[DRA-aware GPU detection](#dra-aware-gpu-detection-in-ml-policy-plugins)); this requirement is
called out in the admin-facing DRA guide added to `docs/`. Users then select from the available
templates via `resourceClaimsPerNode`. The admin controls which GPU types and counts are
available; users cannot override the device count.

### Data scientist: requesting DRA devices

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

The controller:

1. Sets pod-level `resourceClaims` on the trainer `node` replicatedJob.
2. Wires container-level `resources.claims` on the `node` container.
3. Resolves GPU count from the `ResourceClaimTemplate` for `numProcPerNode` auto-detection.

This keeps DRA at the same accessibility level as `resourcesPerNode`.

`resourceClaimsPerNode` deliberately has no container-targeting option, mirroring
`resourcesPerNode`, which also only applies to the `node` container. Attaching a claim to a
sidecar or an init container is done with the `runtimePatches` API
([escape hatch](#escape-hatch-podspecpatchresourceclaims-and-containerpatchresources)).

**Advanced case: claim on an init container**, e.g. a pre-flight GPU check as described in
[KEP-3416](https://github.com/kubeflow/trainer/blob/master/proposals/3416-pet-env-init-containers/README.md):

```yaml
spec:
  trainer:
    numNodes: 2
    resourceClaimsPerNode:            # pod-level claim + node container
      - name: gpu
        resourceClaimTemplateName: h100-80gb-template
  runtimePatches:                     # escape hatch for other containers
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
                        initContainers:
                          - name: pre-flight-check
                            resources:
                              claims:
                                - name: gpu
```

Both containers then reference the same pod-level claim, so they share the devices allocated
to that Pod.

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
	// container-level resources.claims on the node container. To attach a claim to
	// other containers, use the runtimePatches API.
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

`TrainerResourceClaim` is deliberately narrower than upstream `corev1.PodResourceClaim`:
it exposes `ResourceClaimTemplateName` only.

```go
type TrainerResourceClaim struct {
	// Name uniquely identifies this resource claim inside the Pod (DNS_LABEL) and is what
	// the container's resources.claims entries reference. It is the list map key, so it
	// must be set.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`

	// ResourceClaimTemplateName is the name of a ResourceClaimTemplate in the same namespace.
	// A separate ResourceClaim is created from the template for every training node Pod,
	// bound to that Pod and deleted with it.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	ResourceClaimTemplateName string `json:"resourceClaimTemplateName"`
}
```

Two notes on this shape:

- **Template only.** Each training node Pod owns its own devices, so the per-Pod
  `ResourceClaimTemplate` path is the right one for distributed training: one template yields
  one `ResourceClaim` per rank. A directly referenced `ResourceClaim` (`resourceClaimName`) is a
  single object shared by every Pod that references it, which is not what a PyTorch job wants.
  It is still reachable through the `runtimePatches` escape hatch, which uses the upstream
  `corev1.PodResourceClaim` type unchanged, and can be added here later without a breaking
  change if users ask for it.
- **`Name` is required** because it is the list map key (Kubernetes requires map key fields to
  be set) and it matches upstream, where `corev1.PodResourceClaim.Name` is also required.
  Keeping `name` as the key also lets `resourceClaimName` be added later without changing the
  key, which would be a breaking change.

The simple case is:

```yaml
resourceClaimsPerNode:
  - name: gpu
    resourceClaimTemplateName: h100-80gb-template
```



#### How the controller applies it

When `trainer.resourceClaimsPerNode` is set, during `newRuntimeInfo()` / trainer resource
application (same place `resourcesPerNode` is applied today):

1. **Pod-level:** set/merge `PodSpec.ResourceClaims` on the trainer `node` replicatedJob
  from `ResourceClaimsPerNode` (`Name` and `ResourceClaimTemplateName` map directly onto
   `corev1.PodResourceClaim`).
2. **Container-level:** for each claim, add an entry to the `node` container's
  `resources.claims`. No other container is touched; use `runtimePatches` for those.
3. **Precedence:** if both `resourceClaimsPerNode` and a `runtimePatches` claim override
  exist, `resourceClaimsPerNode` wins for the trainer node (same pattern as
   `resourcesPerNode` overriding runtime container resources).

This avoids the bifurcated UX of setting GPUs via `resourcesPerNode` while setting DRA
via deep `runtimePatches`.

**Example: resulting JobSet PodSpec**

Given this TrainJob input:

```yaml
trainer:
  resourceClaimsPerNode:
    - name: gpu
      resourceClaimTemplateName: h100-80gb-template
```

The controller produces this on the `node` replicatedJob's PodSpec:

```yaml
spec:
  resourceClaims:
    - name: gpu
      resourceClaimTemplateName: h100-80gb-template
  containers:
    - name: node
      resources:
        claims:
          - name: gpu
```

#### Escape hatch: `PodSpecPatch.ResourceClaims` and `ContainerPatch.Resources`

Since `resourceClaimsPerNode` only covers the trainer `node` container, `runtimePatches` must be
able to express the general case: any replicatedJob, any container. That needs two fields, both
reusing the upstream `corev1` types so the Trainer API stays aligned with Kubernetes as DRA
evolves (for example the `request` field on a container claim, which restricts a container to a
subset of the devices in a claim, works with no Trainer change):

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

type ContainerPatch struct {
	// ... existing fields ...

	// resources patches the container's compute resources, including the resources.claims
	// that reference the Pod's resourceClaims.
	// For the node container, trainer.resourcesPerNode takes precedence for requests and
	// limits, and trainer.resourceClaimsPerNode takes precedence for claims.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}
```

Without `ContainerPatch.Resources`, a claim added through `runtimePatches` could only be consumed
by a container whose `resources.claims` was already pre-wired in the runtime template, so users
could not attach DRA devices to their own sidecars or init containers. Note that `Resources` also
enables patching `requests` / `limits` on non-node containers, which is orthogonal to DRA but
useful in its own right.

`MaxItems=32` is a pragmatic guard; upstream `PodSpec` has no explicit limit but real-world
DRA usage rarely exceeds a handful of claims per pod.

**Example: PodSpecPatch via runtimePatches**

A user wants to add a DRA claim to a non-node replicatedJob (e.g., a `preprocessor`):

```yaml
runtimePatches:
  - manager: user
    trainingRuntimeSpec:
      template:
        spec:
          replicatedJobs:
            - name: preprocessor
              template:
                spec:
                  template:
                    spec:
                      resourceClaims:
                        - name: accel
                          resourceClaimTemplateName: t4-template
```

The strategic merge patch adds the claim to the `preprocessor` replicatedJob's PodSpec. The
consuming container references it through `ContainerPatch.Resources`:

```yaml
                    spec:
                      containers:
                        - name: preprocessor
                          resources:
                            claims:
                              - name: accel
```

#### Admin-defined DRA in TrainingRuntime / ClusterTrainingRuntime

Admins can pre-configure DRA claims in a runtime template. The full `PodSpec` is available
in the runtime, so both pod-level and container-level claims are set directly:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-h100
spec:
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
                  containers:
                    - name: node
                      resources:
                        claims:
                          - name: gpu
```

When a user creates a TrainJob referencing this runtime, the resulting JobSet inherits
the DRA claims. If the user also sets `resourceClaimsPerNode`, their claims take
precedence for the trainer node.

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



### Kubeflow SDK changes

The Kubeflow SDK needs four changes so that DRA is usable end to end: discovering templates,
submitting a TrainJob that uses them, and surfacing claims on `Runtime` and on `Step`.

#### 1. Discovering templates: `list_resource_claim_templates()`

```python
def list_resource_claim_templates(self) -> list[ResourceClaimTemplate]:
    """List the ResourceClaimTemplates available in the client namespace."""
```

This issues a standard `list` call against `resource.k8s.io/v1` (GA since Kubernetes 1.34, the
minimum version for this feature) in the client's current namespace, matching `list_runtimes()`.

**Returned object:**

```python
@dataclass
class DeviceRequest:
    name: str                      # spec.spec.devices.requests[].name
    device_class_name: str | None  # spec.spec.devices.requests[].exactly.deviceClassName
    device_count: int | None       # spec.spec.devices.requests[].exactly.count


@dataclass
class ResourceClaimTemplate:
    name: str
    device_requests: list[DeviceRequest]
```

The goal is to expose just enough for a user to pick a template, using the same `device` /
`device_count` vocabulary as `RuntimeTrainer` and `Step`:

```python
>>> for rct in TrainerClient().list_resource_claim_templates():
...     print(rct.name, [(d.device_class_name, d.device_count) for d in rct.device_requests])
h100-x8   [('gpu.nvidia.com', 8)]
h100-rdma [('gpu.nvidia.com', 8), ('rdma.example.com', 1)]
```

`device_requests` is a list because a template can request several device classes, e.g. GPUs
plus a NIC or a NUMA-aligned CPU set.

The `exactly` fields are optional because a request can instead use `firstAvailable`
sub-requests; in that case `device_class_name` and `device_count` are `None` and users fall back
to reading the template directly. Deliberately not exposed for now, to keep the surface small:
`allocationMode` (`All` makes `count` meaningless), device `selectors` and `constraints`,
`config`, and `firstAvailable` sub-requests. The template `namespace` is also omitted since it
is always the client namespace. Any of these can be added later without a breaking change.

#### 2. Submitting: `resource_claims_per_node` on the trainer types

`CustomTrainer`, `CustomTrainerContainer` and the builtin trainer configs gain a field next to
`resources_per_node`, taking template names:

```python
@dataclass
class CustomTrainer:
    ...
    resources_per_node: dict | None = None
    resource_claims_per_node: list[str] | None = None  # ResourceClaimTemplate names
```

```python
client.train(
    runtime=client.get_runtime("torch-distributed"),
    trainer=CustomTrainer(
        func=train_fn,
        num_nodes=2,
        resource_claims_per_node=["h100-x8"],
    ),
)
```

The backend maps each entry to `spec.trainer.resourceClaimsPerNode[]`, setting both
`resourceClaimTemplateName` and `name` to the template name (`name` is required; a dict entry
form lets users pick a different claim name).
Claims for sidecars or init containers keep using the existing `RuntimePatch` option, whose
`PodSpecPatch` / `ContainerPatch` dataclasses gain `resource_claims` and `resources` fields
mirroring the CRD escape hatch.

#### 3. Runtime introspection: `RuntimeTrainer`

When an admin pre-populates claims in a `TrainingRuntime` / `ClusterTrainingRuntime` (see
[Admin-defined DRA in TrainingRuntime](#admin-defined-dra-in-trainingruntime--clustertrainingruntime)),
`get_runtime()` / `list_runtimes()` must show them. `RuntimeTrainer` gains:

```python
@dataclass
class RuntimeTrainer:
    ...
    device: str = UNKNOWN
    device_count: str = UNKNOWN
    resource_claim_templates: list[str] = field(default_factory=list)  # new
```

populated from the `node` container's `resources.claims[].name`, resolved through the Pod's
`resourceClaims[]` to the `resourceClaimTemplateName`. In addition, `get_container_devices()`
(which today reads only `resources.limits`) gains a fallback: when no known extended resource is
present and the container has `resources.claims`, the SDK reads the referenced
`ResourceClaimTemplate` from the client namespace and sets `device` to the `deviceClassName` and
`device_count` to the summed `count`, so `device` / `device_count` keep working for DRA runtimes.
If the template cannot be read (for example a `ClusterTrainingRuntime` whose template is not
present in this namespace), `resource_claim_templates` is still populated and `device` /
`device_count` stay `Unknown`.

#### 4. Job introspection: `Step`

`Step` gains the same `resource_claim_templates: list[str]` field, read from the Pod spec of the
node, and uses the same `get_container_devices()` fallback for `device` / `device_count`. For
Torch jobs the existing override from the `PET_NPROC_PER_NODE` env continues to apply, so
`device_count` is correct for the trainer step whenever the controller resolved the GPU count,
even if the SDK cannot read the template.

The allocated devices of a running Pod (`pod.status.resourceClaimStatuses` and the generated
`ResourceClaim.status.allocation`) are not surfaced in this KEP; they can be added to `Step`
later if users need to see which physical devices a step received.

### GPU count is admin-controlled

With extended resources, users set GPU count directly via `resourcesPerNode`
(e.g., `nvidia.com/gpu: 4`). With DRA, GPU count is defined inside the
`ResourceClaimTemplate` (`spec.devices.requests[].exactly.count`).

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
starts from the **`node` container's `resources.claims`** (not the aggregated pod-level
`resourceClaims`, which may also carry claims consumed only by sidecars or init containers,
e.g. a GPU monitoring sidecar) and resolves only the **first** entry in that list by name
(supporting multiple claims per node container is future work, if users need it for
node-local resources):

- For `ResourceClaimTemplateName`: look up the `ResourceClaimTemplate` and inspect
  `spec.spec.devices.requests[]`.
- For a `ResourceClaimName` set through the `runtimePatches` escape hatch: look up the
  `ResourceClaim` object directly and inspect its device requests.

This mirrors how `GetNumGPUPerNode()` already reads only the `node` container's
`resources.requests`/`limits`, so a claim wired solely to another container never counts
toward `numProcPerNode`.

Within that single claim, the controller checks whether each device request's `name`
(`spec.devices.requests[].name`) contains "gpu" (same heuristic as `GetNumGPUPerNode`
matching resource names containing "gpu") and sums the `count` for matching requests.
If the first claim yields no GPU count (e.g. it is a NIC claim), the count is 0 and the
user sets `numProcPerNode` explicitly.
The request name is used rather than `deviceClassName` because it is free text the admin
fully controls when authoring the template, while DeviceClass names are defined by the
installed DRA drivers.

Auto-detection only counts plain exact requests. A request contributes to the GPU count
only when all of the following hold; otherwise it contributes 0 and the user sets
`numProcPerNode` explicitly:

- it uses `exactly` (prioritized-list `firstAvailable` requests are skipped, since the
  chosen alternative is unknown until allocation);
- `allocationMode` is `ExactCount` (or unset); `All` has no meaningful count before
  allocation;
- `exactly.adminAccess` is not `true` (DRAAdminAccess claims are for cluster
  administration, e.g. monitoring, and must not count as workload GPUs).

**Admin requirement:** For auto-detection to work, the device request `name` in the
`ResourceClaimTemplate` must contain "gpu" (e.g., `gpu`, `h100-gpu`). If no request name
matches, the DRA GPU count defaults to 0 and the user can set `numProcPerNode` explicitly.
This requirement is documented in the admin-facing DRA setup guide (see Files modified),
alongside the `ResourceClaimTemplate` examples.

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

**RBAC:** The controller's `ServiceAccount` needs `get`, `list`, `watch` permission on
`resourceclaimtemplates` and `resourceclaims` in the `resource.k8s.io` API group. This
is added to the controller's `ClusterRole` manifest.

### Validation

- Reject `trainer.resourcesPerNode.claims` if set. That field is ignored by Trainer today;
users should use `resourceClaimsPerNode` instead.
- Emit an admission warning (not a rejection) when the trainer `node` container has DRA claims
but no extended GPU resource and `numProcPerNode` is not set, since the DRA GPU count may resolve
to 0 (see [DRA-aware GPU detection](#dra-aware-gpu-detection-in-ml-policy-plugins)) and Torch
would silently fall back to the CPU count.
- Reject a container `resources.claims` entry (set via `ContainerPatch.Resources`) whose `name`
does not match any pod-level `resourceClaims` entry in the merged PodSpec, so users get a clear
error instead of a Pod that is rejected later by the API server.
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
| **User wants a claim on a sidecar or init container**     | Not expressible via `resourceClaimsPerNode`. The user adds the claim with `runtimePatches` (`PodSpecPatch.ResourceClaims` + `ContainerPatch.Resources.Claims`).  |
| **Container `resources.claims` references an unknown claim** | Kubernetes rejects the Pod at admission. Trainer webhook also rejects a container claim with no matching pod-level `resourceClaims` entry.                    |


After adding the fields, run `make generate` to regenerate deep copy methods, OpenAPI schema,
and CRD manifests.

### Files modified


| File                                                     | Change                                                                       |
| -------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `pkg/apis/trainer/v1alpha1/trainjob_types.go`            | Add `ResourceClaimsPerNode` to `Trainer`; `ResourceClaims` to `PodSpecPatch`; `Resources` to `ContainerPatch` |
| `pkg/apis/trainer/v1alpha1/zz_generated.deepcopy.go`     | Regenerated via `make generate`                                              |
| `pkg/apis/trainer/v1alpha1/zz_generated.openapi.go`      | Regenerated via `make generate`                                              |
| `manifests/base/crds/`                                   | Regenerated CRD YAMLs with new fields                                        |
| `pkg/runtime/core/trainingruntime.go`                    | Apply `resourceClaimsPerNode` to node PodSpec + `node` container claims; preserve existing `resources.claims` when merging `resourcesPerNode` |
| `pkg/runtime/runtime.go`                                 | Propagate DRA GPU count via `PodSet`                                         |
| `pkg/webhooks/trainjob_webhook.go`                       | Reject `resourcesPerNode.claims`; point to `resourceClaimsPerNode`           |
| `pkg/runtime/framework/plugins/torch/torch.go`           | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/torch/torchtune.go`       | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/mpi/mpi.go`               | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/xgboost/xgboost.go`       | Use DRA GPU count as fallback when GPU count is 0                            |
| `pkg/runtime/framework/plugins/flux/flux.go`             | Use DRA GPU count as fallback when GPU count is 0                            |
| `manifests/base/rbac/`                                   | Add `resourceclaimtemplates` and `resourceclaims` get/list/watch permission to ClusterRole |
| `pkg/runtime/core/trainingruntime_test.go`               | Test top-level claims application and merge behavior                         |
| `sdk/python/kubeflow/trainer/api_client.py` (or similar) | Add `list_resource_claim_templates(namespace)` method                        |
| `docs/` (Trainer website)                                | Admin DRA setup guide: template examples, "gpu" request naming requirement for auto-detection |




### Test plan

- [x] I/we understand the owners of the involved components may require updates to

existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Unit tests

`**pkg/runtime/core/trainingruntime_test.go`:**

- `resourceClaimsPerNode` set: node PodSpec gets claims and the `node` container gets `resources.claims`; no other container is modified
- Claim added to a sidecar via `runtimePatches` (`PodSpecPatch.ResourceClaims` + `ContainerPatch.Resources.Claims`): only that container gets `resources.claims`
- `resourceClaimsPerNode` and a `runtimePatches` claim on an init container combined: both containers reference the same pod-level claim
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

### `containerNames` targeting on `resourceClaimsPerNode`

An earlier revision let each `resourceClaimsPerNode` entry list the containers that should
receive the claim. **Rejected:** `resourcesPerNode` has no container targeting either, so it
would make the two neighbouring fields inconsistent; it duplicates what `runtimePatches` already
expresses; and it would drift from the upstream container `ResourceClaim` type, which supports a
`request` field to select a subset of devices that a Trainer-specific list of names cannot
represent. Non-node containers use the `runtimePatches` escape hatch instead.

### Add a new top-level field on `TrainJobSpec` (not under `Trainer`)

**Rejected:** Claims are per training node, same scope as `resourcesPerNode`. Putting them
under `Trainer` keeps the resource API together.

## Future Work (Phase 2)

1. **PodGroup-level ResourceClaims via Workload API.** Depends on upstream
  [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) (alpha in k8s 1.36,
   beta in 1.37) and, primarily, the Trainer WAS KEP
   ([#3219](https://github.com/kubeflow/trainer/pull/3219)). Enables
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
