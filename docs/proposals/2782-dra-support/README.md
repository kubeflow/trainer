# KEP-2782: Dynamic Resource Allocation (DRA) Support for Kubeflow Trainer

Authors:

- Sridhar Pillai (Red Hat)

## Summary

[Dynamic Resource Allocation (DRA)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
graduated to GA in Kubernetes 1.34, establishing a new standard for device scheduling that
supersedes extended resources for GPUs and accelerators. Users need the ability to configure
`ResourceClaims` on training workloads managed by Kubeflow Trainer.

This KEP proposes adding a `resourceClaims` field to `PodSpecPatch` so that platform admins
can pre-configure DRA claims in `ClusterTrainingRuntime` templates, and data scientists can
reference or override those claims via `runtimePatches` in their `TrainJob` specs. Because the
existing strategic merge patch pipeline already handles `corev1.PodSpec.ResourceClaims` natively,
the implementation requires only an API surface change. The torch plugin's GPU auto-detection
logic requires a documentation update (users must set `numProcPerNode` explicitly when using
DRA without extended resources), and a follow-up enhancement to inspect `resourceClaims` is
recommended but not required for Phase 1.

## Motivation

Kubernetes DRA replaces the rigid extended-resource model (`nvidia.com/gpu: 1`) with a flexible,
structured API for device allocation. Under DRA:

- **DeviceClasses** describe categories of hardware (e.g., `gpu.nvidia.com`).
- **ResourceClaimTemplates** define allocation parameters (GPU model, MIG profiles, sharing policies).
- **ResourceClaims** bind a pod to its allocated devices at scheduling time.

This matters for Kubeflow Trainer because:

1. **DRA is the future of GPU scheduling.** Major cloud providers and hardware vendors ship DRA
   drivers for their GPUs. Extended resources (`nvidia.com/gpu`) will remain supported but are
   increasingly treated as a compatibility path.
2. **DRA enables user-defined sharing policies.** Under extended resources, MIG partitioning and
   GPU timeslicing are admin-only decisions baked into the device plugin configuration. DRA moves
   sharing policy into the `DeviceClass` and `ResourceClaimTemplate`, letting platform teams offer
   multiple GPU profiles (full GPU, MIG 3g.20gb, timesliced) from the same cluster.
3. **Training workloads are the primary consumer.** Distributed training jobs are the largest
   consumers of GPU resources in Kubernetes. Trainer must provide first-class DRA support for
   its users to adopt the modern scheduling model.
4. **Kubeflow Trainer has no DRA support today.** The `PodSpecPatch` type, which acts as a
   curated allowlist of pod spec fields users can override, does not include `resourceClaims`.
   This is the only gap; the merge pipeline already handles the field.

### Goals

1. Enable pod-level DRA `ResourceClaims` via `PodSpecPatch` in `RuntimePatches`, allowing users
   to inject claims into pods created by `TrainingRuntime` templates.
2. Allow platform admins to pre-configure `ResourceClaimTemplates` and claims in
   `ClusterTrainingRuntime` templates (already possible via the full `PodSpec` in templates).
3. Allow data scientists to override or add claims via `runtimePatches` in `TrainJob`.
4. Validate claim references in webhooks to ensure consistency between pod-level and
   container-level claim names.

### Non-Goals

1. **PodGroup-level ResourceClaims.** Multi-node topology-aware allocation (e.g., NVL72,
   ComputeDomains) requires Trainer's WAS KEP
   ([#3219](https://github.com/kubeflow/trainer/pull/3219)) to land. The upstream dependency,
   [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) (`DRAWorkloadResourceClaims`
   feature gate), is alpha in Kubernetes 1.36 with beta targeting 1.37. The primary remaining
   blocker is #3219 on the Trainer side. Phase 1 is independent of both dependencies. This is
   deferred to Phase 2.
2. **ComputeDomain integration.** IMEX channel support for NVL72/GB200 multi-node training is
   under active prototyping at [wg-device-management](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)
   and is not ready for Trainer integration.
3. **Modifying `Trainer.ResourcesPerNode` to support DRA claims.** While `corev1.ResourceRequirements`
   includes a `Claims` field in recent Kubernetes versions, the Trainer controller only reads
   `Limits` and `Requests`. Surfacing claims through `ResourcesPerNode` would require builder
   changes and is semantically incorrect. Claims are not per-node resources.
4. **Replacing existing `resources.requests/limits` GPU scheduling.** Extended resources
   (`nvidia.com/gpu`) remain valid and supported. DRA is an additional scheduling path.

## User Stories

### Story 1: Platform Admin Configures GPU Access via DRA

As a platform engineer, I want to create a `ClusterTrainingRuntime` with DRA ResourceClaimTemplates
pre-configured so that data scientists using this runtime automatically get the correct GPU
allocation without needing to understand DRA internals.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-a100
spec:
  mlPolicy:
    numNodes: 2
    torch:
      numProcPerNode: 4
  template:
    spec:
      replicatedJobs:
        - name: Node
          template:
            spec:
              template:
                spec:
                  resourceClaims:
                    - name: gpu
                      resourceClaimTemplateName: a100-40gb-template
                  containers:
                    - name: trainer
                      image: docker.io/kubeflow/pytorch-mnist
                      resources:
                        claims:
                          - name: gpu
---
apiVersion: resource.k8s.io/v1beta2
kind: ResourceClaimTemplate
metadata:
  name: a100-40gb-template
spec:
  spec:
    devices:
      requests:
        - name: gpu
          deviceClassName: gpu.nvidia.com
          selectors:
            - cel:
                expression: device.attributes["gpu.nvidia.com"].productName == "A100-SXM4-40GB"
          count: 4
```

### Story 2: Data Scientist References GPU Claim in TrainJob

As a data scientist, I want to submit a `TrainJob` that uses DRA claims defined by my platform
team, referencing the claim by name without needing to understand DeviceClasses or
ResourceClaimTemplates.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: llama-finetune
  namespace: ml-team
spec:
  runtimeRef:
    name: torch-distributed-a100
  trainer:
    image: my-registry/llama-trainer:v2
    command:
      - torchrun
    args:
      - --nproc_per_node=4
      - train.py
    numNodes: 4
```

The data scientist gets DRA-managed A100 GPUs without any DRA-specific configuration in their
TrainJob. The `ClusterTrainingRuntime` provides the claims.

### Story 3: User Overrides Default GPU Allocation

As a data scientist, I want to use a different GPU type than what the runtime provides by
default. I override the GPU claim via `runtimePatches` to request H100 GPUs instead of A100s.

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
              - name: Node
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

**UX note:** The deeply nested `runtimePatches` path is verbose. A typo at any level
(e.g., misspelling `replicatedJobs` or using the wrong `name`) causes the patch to be
silently ignored (it does not match any target). Users should validate their patch structure
against the runtime template. Tooling (IDE schema validation, `kubectl --dry-run=server`)
can help catch path errors before submission.

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
	// +kubebuilder:validation:XValidation:rule="self.all(c, has(c.resourceClaimName) && c.resourceClaimName != '' || has(c.resourceClaimTemplateName) && c.resourceClaimTemplateName != '')", message="each claim must set either resourceClaimName or resourceClaimTemplateName"
	// +kubebuilder:validation:XValidation:rule="self.all(c, !(has(c.resourceClaimName) && c.resourceClaimName != '' && has(c.resourceClaimTemplateName) && c.resourceClaimTemplateName != ''))", message="each claim must set only one of resourceClaimName or resourceClaimTemplateName"
	// +optional
	ResourceClaims []corev1.PodResourceClaim `json:"resourceClaims,omitempty"`
}
```

The upstream `corev1.PodResourceClaim` struct (k8s.io/api v0.35.2) has three fields:

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

The new `ResourceClaims` field mirrors the pattern of existing list fields in `PodSpecPatch`:
- `+listType=map` with `+listMapKey=name` for strategic merge patch support (matches
  `volumes`, `initContainers`, `containers`, `imagePullSecrets`, `schedulingGates`)
- `+kubebuilder:validation:MaxItems=32` to bound the list size (32 is generous for
  realistic DRA workloads; most training jobs use 1-4 claims)
- **No immutability rule.** ResourceClaims are scheduling-related, similar to `nodeSelector`,
  `affinity`, `tolerations`, and `schedulingGates` (all mutable in `PodSpecPatch`). Making
  claims mutable allows users to swap GPU types on suspended TrainJobs without deletion.
- Uses `corev1.PodResourceClaim` directly, consistent with how other upstream types
  (`corev1.Volume`, `corev1.Toleration`, `corev1.PodSchedulingGate`) are used in
  `PodSpecPatch`. This risks auto-inheriting upstream field additions, but the tradeoff is
  acceptable: wrapping would diverge from Kubernetes semantics, and v1alpha1 allows breaking
  changes if upstream adds fields that conflict.

#### ContainerPatch and container-level claim references

The existing `ContainerPatch` struct only exposes `Name`, `Env`, `VolumeMounts`, and
`SecurityContext`. It does **not** include a `Resources` field, so container-level claim
references (`resources.claims[].name`) cannot be set through `runtimePatches`.

This is acceptable for Phase 1 because:

1. **Container-level references are set in the runtime template.** Platform admins define
   the full `corev1.PodSpec` in `ClusterTrainingRuntime` templates, where they can set both
   `spec.resourceClaims` (pod-level) and `containers[].resources.claims` (container-level).
   The strategic merge patch preserves container-level references from the template.

2. **Pod-level claims are the critical gap.** The `PodSpecPatch` allowlist missing
   `resourceClaims` is the only blocker. Users cannot inject or override claims at all today.

3. **Adding `Resources` to `ContainerPatch` is a separate concern.** If needed, a future
   enhancement can add `Resources *corev1.ResourceRequirements` to `ContainerPatch`, which
   would also surface `Resources.Claims` for container-level references.

### Strategic merge patch flow

The existing merge pipeline in `pkg/runtime/core/trainingruntime.go` handles `ResourceClaims`
with no code changes. The following diagram shows how claims flow from definition to pod:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        TrainJob (user submits)                      │
│                                                                     │
│  spec.runtimeRef: torch-distributed-a100                           │
│  spec.runtimePatches:                                              │
│    - manager: user                                                  │
│      trainingRuntimeSpec.template.spec.replicatedJobs:             │
│        - name: Node                                                 │
│          template.spec.template.spec:                              │
│            resourceClaims:                                          │
│              - name: gpu                                            │
│                resourceClaimTemplateName: h100-80gb-template       │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│          ClusterTrainingRuntime (read on first reconciliation)      │
│                                                                     │
│  template.spec.replicatedJobs[*].template.spec.template.spec:      │
│    resourceClaims:                                                  │
│      - name: gpu                                                    │
│        resourceClaimTemplateName: a100-40gb-template               │
│    containers:                                                      │
│      - name: trainer                                                │
│        resources.claims: [{name: gpu}]                             │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│        Runtime Snapshot ConfigMap (KEP-2599)                        │
│                                                                     │
│  On first reconciliation, the controller snapshots the runtime     │
│  config into a ConfigMap ({trainjob-name}-runtime-snapshot).        │
│  All subsequent reconciliations read from this snapshot.            │
│  Admin updates to the runtime do NOT affect existing TrainJobs.    │
│  DRA claims in the runtime are frozen at snapshot time.            │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│              mergeRuntimePatches() [runtime/core]                   │
│                                                                     │
│  For each runtimePatch, for each replicatedJob (matched by name):  │
│    1. JSON-marshal source: batchv1.JobTemplateSpec (from snapshot)  │
│    2. JSON-marshal patch:  trainer.JobTemplatePatch (from patches)  │
│    3. strategicpatch.StrategicMergePatch(source, patch,            │
│                                         batchv1.JobTemplateSpec{}) │
│    4. JSON-unmarshal → merged batchv1.JobTemplateSpec               │
│                                                                     │
│  corev1.PodSpec.ResourceClaims has +listType=map +listMapKey=name  │
│  → strategic merge patch merges/replaces by claim name             │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│         JobSet ApplyConfiguration (submitted to k8s API)           │
│                                                                     │
│  spec.replicatedJobs[*].template.spec.template.spec:               │
│    resourceClaims:                                                  │
│      - name: gpu                                                    │
│        resourceClaimTemplateName: h100-80gb-template  ← user wins  │
│    containers:                                                      │
│      - name: trainer                                                │
│        resources.claims: [{name: gpu}]  ← preserved from template  │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Pods (created by JobSet → Job)                   │
│                                                                     │
│  DRA scheduler plugin allocates devices from                       │
│  h100-80gb-template → ResourceClaim bound to pod                   │
└─────────────────────────────────────────────────────────────────────┘
```

Step-by-step:

1. Admin defines `ClusterTrainingRuntime` with `resourceClaims` in the full `corev1.PodSpec`
   template, and sets container-level `resources.claims` references.
2. User creates `TrainJob` with `runtimePatches` containing `PodSpecPatch.ResourceClaims`.
3. On first reconciliation, the controller reads the live `ClusterTrainingRuntime` and stores
   a snapshot in a ConfigMap (`{trainjob-name}-runtime-snapshot`) per
   [KEP-2599](https://github.com/kubeflow/trainer/pull/3428). All subsequent reconciliations
   read from this snapshot. Admin updates to the runtime after this point do not affect the
   TrainJob. DRA claims configured in the runtime are frozen at snapshot time.
4. Controller calls `mergeRuntimePatches()` in `pkg/runtime/core/trainingruntime.go`:
   a. JSON-marshals the existing `batchv1.JobTemplateSpec` (from the snapshot).
   b. JSON-marshals the `*trainer.JobTemplatePatch` (from `runtimePatches`). The
      `PodSpecPatch.ResourceClaims` field serializes to the JSON key `resourceClaims`,
      matching `corev1.PodSpec.ResourceClaims`.
   c. Applies `strategicpatch.StrategicMergePatch` on `batchv1.JobTemplateSpec`.
   d. Unmarshals back to the typed struct.
5. `corev1.PodSpec.ResourceClaims` is a list with `+listMapKey=name`, so strategic merge
   patch merges by claim name automatically.
6. The merged `JobTemplateSpec` flows into the JobSet apply configuration and then to pods.

**Why no controller changes are needed for the merge path:** The `PodSpecPatch` struct is a
curated allowlist, a subset of `corev1.PodSpec` fields that users are permitted to override.
When a field is present in `PodSpecPatch`, it gets marshaled to JSON and included in the
strategic merge patch. Since `corev1.PodSpec` already has `ResourceClaims` as a native field
with proper merge directives, the patch infrastructure handles it transparently. The only
gate is adding the field to the typed `PodSpecPatch` allowlist.

**Note on the torch plugin:** While the merge pipeline requires no changes, the torch plugin's
GPU auto-detection (`GetNumGPUPerNode()`) does not recognize DRA claims. See the Known
Limitations section for details and the required user workaround.

**Key insight from `mergeRuntimePatches()`:** The function marshals the patch as
`*trainer.JobTemplatePatch`, not as `batchv1.JobTemplateSpec`. This works because the JSON
struct tags in the `JobTemplatePatch` → `JobSpecPatch` → `PodTemplatePatch` → `PodSpecPatch`
hierarchy produce the same JSON keys as the corresponding `batchv1.JobTemplateSpec` →
`batchv1.JobSpec` → `corev1.PodTemplateSpec` → `corev1.PodSpec` fields. Adding
`ResourceClaims` to `PodSpecPatch` with the JSON tag `json:"resourceClaims,omitempty"`
aligns with `corev1.PodSpec.ResourceClaims`.

### Validation

Validation follows the existing Trainer pattern: compile-time CEL rules handle structural
constraints on the CRD, while runtime validation (cross-referencing between pod-level claims
and container-level references) goes through the framework plugin pipeline.

#### CEL validation (on the CRD)

Add CEL rules on the `ResourceClaims` field in `PodSpecPatch`. Since `corev1.PodResourceClaim`
is an upstream type (we cannot add kubebuilder markers to it), the CEL rule is applied at the
list level using `self.all()`:

```go
	// resourceClaims defines which ResourceClaims must be allocated and reserved
	// before the Pod is allowed to start.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:XValidation:rule="self.all(c, has(c.resourceClaimName) && c.resourceClaimName != '' || has(c.resourceClaimTemplateName) && c.resourceClaimTemplateName != '')", message="each claim must set either resourceClaimName or resourceClaimTemplateName"
	// +kubebuilder:validation:XValidation:rule="self.all(c, !(has(c.resourceClaimName) && c.resourceClaimName != '' && has(c.resourceClaimTemplateName) && c.resourceClaimTemplateName != ''))", message="each claim must set only one of resourceClaimName or resourceClaimTemplateName"
	// +optional
	ResourceClaims []corev1.PodResourceClaim `json:"resourceClaims,omitempty"`
```

These CEL rules are enforced by the Kubernetes API server at admission time, before any
webhook or controller code runs. They catch malformed claims early with clear error messages.

**CEL design notes:**

- The rules check both `has(field)` AND non-empty (`!= ''`) to avoid false positives. The
  `has()` function returns true for empty strings in pointer/optional fields, so checking
  presence alone would pass invalid configurations where both fields are set to `""`.
- Two separate rules are used instead of a single XOR expression. This produces distinct,
  actionable error messages for each failure mode (missing vs. duplicate). The tradeoff is
  slightly more verbose markers, but significantly better UX.
- These rules duplicate upstream `corev1.PodSpec` validation logic. If upstream adds a third
  option to `PodResourceClaim` in a future Kubernetes release, the XOR rule would reject
  valid claims. This is acceptable for v1alpha1; the rules can be updated without a
  deprecation cycle.
- The kubebuilder markers on `PodSpecPatch` affect only CRD schema validation, not the
  strategic merge patch behavior. The merge uses `batchv1.JobTemplateSpec` as its schema
  source, which carries its own upstream merge directives.

#### Framework plugin validation (webhook pipeline)

The `TrainJobValidator` webhook delegates all validation to `runtime.ValidateObjects()`, which
calls `framework.RunCustomValidationPlugins()`. This is the existing pattern; the webhook
file (`pkg/webhooks/trainjob_webhook.go`) stays thin and delegates to framework plugins.

For DRA, add a validation check in the JobSet plugin's `Validate()` method (or a new
dedicated DRA validation plugin) that runs **after** the runtime template and patches are
merged. This validation has access to the full merged `runtime.Info` and can cross-reference:

1. **Claim name consistency (post-merge):** After `mergeRuntimePatches()` produces the final
   `PodSpec`, verify that every container-level `resources.claims[].name` reference points to
   a valid entry in `podSpec.resourceClaims[].name`. This catches mismatches between what the
   runtime template defines and what the user patches.

2. **Claim name uniqueness:** Verify no duplicate claim names exist in the merged
   `resourceClaims` list. While strategic merge patch merges by name (last writer wins),
   validation should confirm the result is consistent.

This pattern matches how the existing `RunCustomValidationPlugins` works:

```go
func (f *Framework) RunCustomValidationPlugins(
	ctx context.Context, info *runtime.Info, oldObj, newObj *trainer.TrainJob,
) (admission.Warnings, field.ErrorList) {
	var aggregatedWarnings admission.Warnings
	var aggregatedErrors field.ErrorList
	for _, plugin := range f.customValidationPlugins {
		warnings, errs := plugin.Validate(ctx, info, oldObj, newObj)
		// aggregate...
	}
	return aggregatedWarnings, aggregatedErrors
}
```

### Edge cases and error handling

| Scenario | Behavior | Rationale |
|----------|----------|-----------|
| **Cluster has no DRA driver (or k8s < 1.34)** | Pods with `resourceClaims` stay `Pending` indefinitely; DRA scheduler plugin is not present to allocate claims. The TrainJob will eventually fail via `ActiveDeadlineSeconds` if set, or remain stuck. | This is standard Kubernetes behavior. No Trainer-specific handling needed. Users must ensure a DRA driver is installed for their device class. |
| **Referenced `ResourceClaimTemplate` does not exist** | The DRA scheduler plugin cannot create a `ResourceClaim` from the template. Pods stay `Pending` with an event like `FailedScheduling: resourceclaimtemplate "X" not found`. | Kubernetes handles this at scheduling time. Trainer does not need to pre-validate template existence because templates may be created after the TrainJob (eventual consistency). |
| **Container references a claim name not defined at pod level** | Pod admission fails with `Invalid value: "X": must match the name of an entry in spec.resourceClaims`. | Kubernetes API server validates this. The framework plugin validation (above) catches this earlier during TrainJob admission, before the JobSet is created. |
| **Both `resourceClaimName` and `resourceClaimTemplateName` are set** | CEL rule on `PodSpecPatch.ResourceClaims` rejects the TrainJob at admission time with `each claim must set only one of resourceClaimName or resourceClaimTemplateName`. | Caught by compile-time CEL validation on the CRD. |
| **Neither `resourceClaimName` nor `resourceClaimTemplateName` is set** | CEL rule rejects the TrainJob at admission with `each claim must set either resourceClaimName or resourceClaimTemplateName`. | Caught by compile-time CEL validation on the CRD. |
| **User patches a claim name that does not exist in the runtime template** | Strategic merge patch adds the new claim (merge-by-name creates new entries for unknown names). Container-level references must be updated separately in the template. | This is valid. Users can add new claims. If container references are not updated, the container simply does not consume the claim. |
| **`ResourceClaimTemplate` is in a different namespace** | `ResourceClaimTemplateName` must reference a template in the same namespace as the pod. Kubernetes rejects cross-namespace references. | Standard Kubernetes restriction. No Trainer-specific handling needed. |
| **User adds a claim but no container references it** | The GPU is allocated by the DRA scheduler plugin but no container consumes it. Resources are wasted silently. | Valid Kubernetes configuration; no API-level rejection. A future enhancement could add a webhook warning (not rejection) for unreferenced claims. Users should verify container-level `resources.claims` references match pod-level claim names. |
| **Both extended resources AND DRA claims set simultaneously** | Both allocation paths proceed independently. Extended resources are handled by the kubelet device plugin; DRA claims are handled by the DRA scheduler plugin. The pod receives GPUs from both paths. | Both are valid simultaneously. Users should be aware they will receive double GPU allocation if they set both `nvidia.com/gpu: 4` in resources AND a DRA claim requesting 4 GPUs. The runtime template should use one path or the other, not both. |
| **Suspend/resume with template-based DRA claims** | Template-based claims (`resourceClaimTemplateName`) are created per-pod and deleted when the pod is deleted. On suspend, pods are deleted along with their claims. On resume, new pods get fresh claims. | Standard Kubernetes lifecycle. No Trainer-specific handling needed. |
| **Suspend/resume with named DRA claims** | Named claims (`resourceClaimName`) reference pre-existing `ResourceClaim` objects that persist independently of pod lifecycle. On suspend/resume, the same claim is re-bound to the new pod. | Users must ensure the named `ResourceClaim` exists before resume. If the claim was manually deleted during suspension, the resumed pod will fail scheduling. |
| **User changes claim name (not just template) in a patch** | If the user renames a claim (e.g., `gpu` to `accelerator`), existing container-level `resources.claims` references still point to the old name (`gpu`). The container will not consume the renamed claim. | Claim names are stable identifiers that containers reference. Users should override the claim's template (change `resourceClaimTemplateName`), not rename the claim itself. If renaming is needed, the admin must update both the pod-level claim and all container references in the runtime template. |
| **Scheduling failure due to typo in template name** | Pods remain `Pending` with `FailedScheduling` events. The TrainJob status does not surface DRA-specific scheduling failures (it only reflects Job/JobSet status). | This is a UX gap. Users must inspect pod events to diagnose the issue. A follow-up enhancement should surface DRA scheduling failures as TrainJob conditions. |

### Code generation

After adding the field, run:

```bash
make generate
```

This regenerates:
- `pkg/apis/trainer/v1alpha1/zz_generated.deepcopy.go` (deep copy methods for the new field)
- `pkg/apis/trainer/v1alpha1/zz_generated.openapi.go` (OpenAPI schema)
- CRD manifests in `manifests/` (updated CRD YAML with the new field and validation rules)

### Files modified

| File | Change |
|------|--------|
| `pkg/apis/trainer/v1alpha1/trainjob_types.go` | Add `ResourceClaims` field to `PodSpecPatch` with kubebuilder markers and CEL rules |
| `pkg/apis/trainer/v1alpha1/zz_generated.deepcopy.go` | Regenerated via `make generate` |
| `pkg/apis/trainer/v1alpha1/zz_generated.openapi.go` | Regenerated via `make generate` |
| `manifests/base/crds/` | Regenerated CRD YAMLs with new field and CEL validation |
| `pkg/runtime/framework/plugins/jobset/` | Add DRA claim validation in `Validate()` (claim name cross-referencing) |
| `pkg/runtime/core/trainingruntime_test.go` | Test patch merging with `resourceClaims` |
| `pkg/runtime/framework/core/framework_test.go` | Test DRA validation plugin |
| `pkg/webhooks/trainjob_webhook_test.go` | Test validation rules end-to-end |
| `test/integration/controller/` | Integration test for end-to-end patch flow |

**Files NOT modified:**
- `pkg/controller/` (no controller logic changes required)
- `pkg/runtime/core/trainingruntime.go` (strategic merge patch handles the field natively)
- `pkg/webhooks/trainjob_webhook.go` (webhook delegates to framework plugins; no changes needed)
- `pkg/runtime/framework/plugins/torch/torch.go` (Phase 1 does not modify GPU detection;
  see Known Limitations for the `numProcPerNode` requirement with DRA)

### Test plan

- [x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Unit tests

Tests follow the existing dictionary-style pattern (`cases := map[string]struct{...}`) with
`testingutil` wrapper functions and `cmp.Diff` for comparison.

**`pkg/runtime/core/trainingruntime_test.go`**: Add cases to `TestTrainingRuntimeNewObjects`:

```go
cases := map[string]struct {
    trainingRuntime *trainer.TrainingRuntime
    trainJob        *trainer.TrainJob
    wantObjs        []runtime.Object
    wantError       error
}{
    "runtime template has resourceClaims, no user patch → claims preserved in JobSet": {
        // trainingRuntime with resourceClaims in PodSpec
        // trainJob with no runtimePatches touching claims
        // wantObjs: JobSet with original resourceClaims
    },
    "user patches resourceClaims via runtimePatches → claims merged by name": {
        // trainingRuntime with resourceClaim name=gpu (a100 template)
        // trainJob with runtimePatch: resourceClaim name=gpu (h100 template)
        // wantObjs: JobSet with gpu claim pointing to h100 template
    },
    "user adds new claims alongside runtime defaults → both present": {
        // trainingRuntime with resourceClaim name=gpu
        // trainJob with runtimePatch: resourceClaim name=rdma
        // wantObjs: JobSet with both gpu and rdma claims
    },
}
```

The `runtimePatches` in test cases use the full struct hierarchy, matching existing test
patterns (e.g., `TrainingRuntimeSpecPatch` → `JobSetTemplatePatch` → `JobSetSpecPatch` →
`ReplicatedJobPatch` → `JobTemplatePatch` → `JobSpecPatch` → `PodTemplatePatch` →
`PodSpecPatch` with `ResourceClaims`).

**`pkg/webhooks/trainjob_webhook_test.go`**: Add cases to `TestValidateCreate`:

```go
cases := map[string]struct {
    obj                    *trainer.TrainJob
    clusterTrainingRuntime *trainer.ClusterTrainingRuntime
    wantError              field.ErrorList
    wantWarnings           admission.Warnings
}{
    "valid trainjob with resourceClaims in runtimePatches": {
        // TrainJob with runtimePatch containing valid resourceClaims
        // ClusterTrainingRuntime with matching container references
        // wantError: nil
    },
    "container references claim not in podSpec.resourceClaims - rejected": {
        // ClusterTrainingRuntime with container referencing claim name "gpu"
        // but no resourceClaim entry with that name
        // wantError: field.ErrorList with Invalid error
    },
}
```

**Negative test cases for merge failures** (`pkg/runtime/core/trainingruntime_test.go`):

```go
cases := map[string]struct {
    trainingRuntime *trainer.TrainingRuntime
    trainJob        *trainer.TrainJob
    wantObjs        []runtime.Object
    wantError       error
}{
    "malformed runtimePatch with invalid JSON structure - returns merge error": {
        // trainJob with runtimePatch containing structurally invalid patch
        // (e.g., claim with conflicting nested structures)
        // wantError: non-nil (strategic merge patch error)
    },
    "runtimePatch targets non-existent replicatedJob name - no merge, no error": {
        // trainJob with runtimePatch for replicatedJob name "nonexistent"
        // Runtime template only has "Node"
        // wantObjs: JobSet with original claims unchanged (patch skipped)
    },
    "empty resourceClaims list in patch - does not clear existing claims": {
        // trainingRuntime with resourceClaim name=gpu
        // trainJob with runtimePatch containing empty resourceClaims: []
        // wantObjs: JobSet with original gpu claim preserved
        // (empty list in JSON merge patch does not delete; this should be tested)
    },
    "both resourceClaimName and resourceClaimTemplateName set - CEL rejects": {
        // This is a CEL validation test, verifying the XValidation rule
        // wantError: admission rejection
    },
}
```

**`pkg/runtime/framework/core/framework_test.go`**: Add cases to
`TestRunCustomValidationPlugins` for DRA-specific validation once a validation plugin is added.

#### Integration tests

**`test/integration/controller/`**: Add Ginkgo integration tests:

- Create `ClusterTrainingRuntime` with DRA claims → create `TrainJob` → verify resulting
  JobSet pods contain the correct `resourceClaims` in `PodSpec`.
- Create `TrainJob` with `runtimePatches` overriding claims → verify merge behavior
  (claim replaced by name).
- Suspend and resume `TrainJob` with claims → verify claims persist across reconciliation.

#### E2E tests

E2E tests are deferred until a DRA-capable test cluster is available in CI. When
implemented, the [dra-example-driver](https://github.com/kubernetes-sigs/dra-example-driver)
can be used for E2E testing without real GPUs, following the approach used by
[Kueue](https://github.com/kubernetes-sigs/kueue) for its DRA E2E tests. The unit and
integration tests cover the API surface and merge behavior comprehensively.

### Known Limitations

The following are known limitations of the Phase 1 implementation. These are documented here
as accepted tradeoffs, with recommendations for future resolution.

#### Torch plugin GPU auto-detection does not recognize DRA claims

The torch plugin's `GetNumGPUPerNode()` function (in `pkg/runtime/runtime.go`) determines
`nproc_per_node` by searching `resources.Requests` and `resources.Limits` for resource names
containing "gpu". DRA-allocated GPUs do not appear in these fields; they are managed through
`ResourceClaim` objects and exposed to containers via CDI device injection.

**Impact:** When a training job uses DRA claims exclusively (no extended resources like
`nvidia.com/gpu`), `GetNumGPUPerNode()` returns 0. If `numProcPerNode` is not explicitly
set, the torch plugin falls back to CPU-based calculation, resulting in incorrect
`--nproc_per_node` values.

**Required user action:** Users MUST set `numProcPerNode` explicitly in the `Trainer` spec
when using DRA without extended resources:

```yaml
spec:
  trainer:
    numProcPerNode: 4  # Must be set explicitly with DRA
```

**Recommended Phase 1 follow-up:** Add a webhook warning (not rejection) when
`resourceClaims` are present in the merged PodSpec but `numProcPerNode` is not explicitly
set in the `Trainer` spec. This would catch misconfiguration early at admission time rather
than causing silent incorrect behavior at runtime.

**Future enhancement:** A follow-up issue should update `GetNumGPUPerNode()` to also inspect
the merged `PodSpec.ResourceClaims` and cross-reference with the container's
`resources.claims` entries and the referenced `ResourceClaimTemplate`'s device request count.
This would require the torch plugin to receive the full `PodSpec` (or the claim count) as
context. If this enhancement is pursued, it would be the one runtime logic change introduced
by DRA support.

#### Users cannot remove claims provided by a runtime template

Strategic merge patch with typed Go structs cannot express "delete claim X from the list."
If an admin provides 3 claims in a `ClusterTrainingRuntime` and a user only needs 2, they
cannot remove the unwanted claim via `runtimePatches`. They can only override claim sources
(change the `resourceClaimTemplateName`) or add new claims.

**Workaround:** Users can change the unwanted claim's template to point to a minimal/no-op
`ResourceClaimTemplate`, or request admin assistance to create a variant runtime without
the extra claim.

**Future enhancement:** If this becomes a common pain point, a future API version could add
an `excludeResourceClaims` field (list of claim names to remove from the template).

#### ContainerPatch does not expose `Resources`, limiting self-service DRA adoption

The `ContainerPatch` struct only exposes `Name`, `Env`, `VolumeMounts`, and
`SecurityContext`. Users cannot add container-level `resources.claims` references via
`runtimePatches`. If a runtime template lacks container claim references, data scientists
cannot add DRA to that runtime without admin intervention.

**Phase 1 tradeoff:** Platform admins MUST pre-configure both pod-level
`spec.resourceClaims` AND container-level `containers[].resources.claims` references in
`ClusterTrainingRuntime` templates. Both are required; a pod-level claim without a
matching container-level reference will not make the device available to the container.
This is consistent with the existing model where admins own infrastructure configuration.

**Future enhancement:** Adding `Resources *corev1.ResourceRequirements` to `ContainerPatch`
would allow users to self-service container-level claim references. This has no dependency
on WAS or PodGroups and could land as a fast follow-up to Phase 1.

#### ValidateObjects() ignores merge errors (pre-existing issue)

In `pkg/runtime/core/trainingruntime.go` line 264, the `ValidateObjects()` function calls
`newRuntimeInfo()` and discards the error:

```go
info, _ := r.newRuntimeInfo(new, trainingRuntime.Spec.Template, ...)
```

This means a malformed `runtimePatch` (e.g., one that causes a merge failure) will pass
admission validation but fail during reconciliation. This is a pre-existing bug not
introduced by this KEP, but DRA claims with complex structures may be more likely to trigger
it.

**Recommendation:** A follow-up fix should propagate this error to the admission response.
This is tracked independently from the DRA feature.

### RBAC Analysis

The Trainer controller does **not** need new RBAC permissions for DRA support. The controller
does not create, read, update, or delete `ResourceClaim` or `ResourceClaimTemplate` objects
directly. The strategic merge patch puts `resourceClaims` into the `PodSpec` of the JobSet
template. From there:

1. The JobSet controller creates Jobs with the merged PodSpec.
2. The Job controller creates Pods with `resourceClaims`.
3. The Kubernetes DRA scheduler plugin allocates devices and creates `ResourceClaim` objects.

The only RBAC requirements are on the DRA scheduler plugin itself (which is part of
kube-scheduler) and on any DRA driver DaemonSets (which manage device allocation). These
are outside Trainer's control.

### Migration Path

DRA and extended resources coexist. There is no deprecation of extended resources.

**Migration steps for platform teams:**

1. Install a DRA driver for the target hardware (e.g., `nvidia-dra-driver`).
2. Create `DeviceClass` and `ResourceClaimTemplate` objects for each GPU profile.
3. Create new `ClusterTrainingRuntime` templates that use DRA claims instead of (or in
   addition to) extended resources.
4. Optionally, update existing runtimes to replace `nvidia.com/gpu` limits with DRA claims.
   Existing TrainJobs using the old runtime continue to work; only new jobs pick up changes.

**Coexistence model:**

- Runtimes using `resources.limits: {nvidia.com/gpu: N}` continue to work unchanged.
- Runtimes using `resourceClaims` work with DRA drivers.
- A runtime can use both paths simultaneously (e.g., extended resources for one container
  and DRA claims for another), though this is unusual.
- Users migrate by switching their `runtimeRef` to a DRA-enabled runtime, or by patching
  claims via `runtimePatches`.

### Feature Gate

This KEP does **not** propose a feature gate for the `resourceClaims` field in `PodSpecPatch`.

**Rationale:** The field is a passthrough API addition with no controller logic. It adds a
field to the allowlist that the strategic merge patch already handles. If the field is unused,
it has zero runtime impact. The API is v1alpha1, so the field can be removed without a
deprecation cycle if needed (see Rollback).

**Exception:** If the torch plugin is updated in a follow-up to inspect `resourceClaims` for
GPU auto-detection (see Known Limitations), that behavior change SHOULD be gated behind a
feature flag (e.g., `DRAGPUDetection`) since it alters existing runtime logic.

### Rollback Strategy

The `resourceClaims` field is added to a v1alpha1 API. If the feature needs to be rolled
back, the field can be removed from `PodSpecPatch` without a deprecation cycle. Existing
TrainJobs that use the field would fail re-validation on update, but would continue running
(Kubernetes does not re-validate existing objects at rest). A migration note in the release
changelog would suffice.

### Observability and Debugging

When DRA claims cause scheduling failures, the debugging workflow is:

1. Check TrainJob status (shows Job-level failures but not DRA-specific errors).
2. Inspect pod events: `kubectl describe pod <pod-name>` shows DRA scheduling events.
3. Check `ResourceClaim` status: `kubectl get resourceclaim -n <namespace>` shows allocation
   state.
4. Check DRA driver logs for device-level errors.

A follow-up enhancement should surface DRA scheduling failures as TrainJob conditions to
reduce the debugging steps required.

### Scale Considerations

DRA scale characteristics are determined by the upstream Kubernetes DRA implementation and
the specific DRA driver. The Trainer controller adds no additional scaling bottlenecks
because it does not interact with DRA objects at runtime. Upstream DRA has been tested at
scale as part of the Kubernetes 1.34 GA graduation. Per-pod claim creation is bounded by
the same factors that bound pod creation itself.

## Other considered alternatives

### Surface claims via `Trainer.ResourcesPerNode`

`corev1.ResourceRequirements` in Kubernetes 1.28+ includes a `Claims` field. We could expose
DRA claims through `Trainer.ResourcesPerNode.Claims`:

```go
type Trainer struct {
    ResourcesPerNode *corev1.ResourceRequirements
    // ResourceRequirements.Claims would carry DRA claim references
}
```

**Rejected because:**
- The Trainer controller's builder only reads `Limits` and `Requests` from `ResourcesPerNode`.
  Supporting `Claims` would require builder changes.
- Semantically incorrect: `ResourcesPerNode` describes quantitative resource requirements,
  while DRA claims are declarative device requests with structured allocation parameters.
- `Claims` in `ResourceRequirements` are container-level references to pod-level claims. The
  pod-level `ResourceClaims` definition must still live somewhere, and `PodSpecPatch` is the
  natural place.

### Add claims at the JobSet level

Define `ResourceClaimTemplates` at the JobSet level so claims are shared across all
ReplicatedJobs:

**Rejected because:**
- Upstream JobSet does not yet support `ResourceClaimTemplates` at the JobSet level.
- Pod-level claims are the only GA path in Kubernetes 1.34.
- Different ReplicatedJobs in a training job may need different GPU types (e.g., parameter
  servers vs. workers), which is incompatible with JobSet-level sharing.

### Add a new top-level TrainJob field

Add a dedicated `ResourceClaims` field directly on `TrainJobSpec`:

```go
type TrainJobSpec struct {
    // ...
    ResourceClaims []corev1.PodResourceClaim
}
```

**Rejected because:**
- This breaks the established `RuntimePatches` pattern where all pod-level configuration
  flows through the patch hierarchy.
- It would require special-case merge logic in the controller instead of leveraging the
  existing strategic merge patch pipeline.
- The `PodSpecPatch` approach is consistent with how other pod-level fields
  (`volumes`, `tolerations`, `nodeSelector`, etc.) are already exposed.

## Future Work (Phase 2)

Phase 2 depends on upstream Kubernetes and Trainer changes that are currently in development:

1. **PodGroup-level ResourceClaims via Workload API.** The
   [DRAWorkloadResourceClaims feature gate](https://github.com/kubernetes/enhancements/issues/5729)
   is alpha in Kubernetes 1.36 with beta targeting 1.37. The primary Trainer-side blocker is
   the WAS KEP ([#3219](https://github.com/kubeflow/trainer/pull/3219)). Once #3219 lands,
   ResourceClaims can be defined at the PodGroup level, enabling shared device allocation
   across all pods in a training job.

2. **ComputeDomain integration for topology-aware scheduling.** NVIDIA NVL72 and GB200 systems
   require topology-aware multi-node device allocation. The
   [wg-device-management topology prototyping](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)
   is exploring PodGroup-level claims with ComputeDomain support. As John Belamaric noted:
   "PodGroup integration eliminates a scaling issue where only 32 Pods could share a
   ResourceClaim. Now it's shared with the PodGroup and you can have unlimited Pods."

3. **Shared claims across pods.** Currently, each pod gets its own ResourceClaim. Phase 2 will
   enable scenarios where pods in a ReplicatedJob share a single claim (e.g., for IMEX channels
   in multi-node GPU training).

4. **Integration with Kueue DRA support.** Kueue is developing its own DRA integration for
   quota and admission control of DRA-managed resources. Kueue may need to inspect
   `resourceClaims` in TrainJob specs to calculate quota consumption for DRA-managed
   devices. Trainer should align with Kueue's approach once it stabilizes, and ensure that
   `resourceClaims` in `runtimePatches` are visible to Kueue's admission logic.

5. **Torch plugin DRA-aware GPU detection.** Update `GetNumGPUPerNode()` to inspect
   `resourceClaims` and derive GPU count from `ResourceClaimTemplate` device request counts.
   This would remove the requirement for users to set `numProcPerNode` explicitly when using
   DRA.

6. **Surfacing DRA scheduling failures as TrainJob conditions.** Add a controller watch or
   event handler that surfaces pod-level DRA scheduling failures (template not found, driver
   unavailable, device exhausted) as TrainJob-level conditions for better observability.

## Implementation History

- **2025-08-07**: [Issue #2782](https://github.com/kubeflow/trainer/issues/2782) opened by @kannon92
- **2025-08-07**: Initial Slack discussion on DRA levels (pod vs job vs jobset) in `#wg-training`
- **2026-01-29**: @Sridhar1030 assigned to the issue
- **2026-05-18**: Slack thread confirming pod-level DRA is independent of WAS
  ([thread](https://cloud-native.slack.com/archives/C0742LDFZ4K/p1779107242466099))
- **2026-05-19**: John Belamaric confirms PodGroup topology approach, separate from pod-level
- **2026-05-23**: KEP draft (this document)

## References

- [Kubernetes DRA documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
- [DRA GA in Kubernetes 1.34](https://kubernetes.io/blog/2025/09/01/kubernetes-v1-34-dra-updates)
- [KEP-5729: DRA ResourceClaim for Workloads](https://github.com/kubernetes/enhancements/issues/5729)
- [GitHub Issue #2782: DRA Support for Trainer](https://github.com/kubeflow/trainer/issues/2782)
- [WAS KEP PR #3219](https://github.com/kubeflow/trainer/pull/3219)
- [wg-device-management topology prototyping](https://github.com/kubernetes-sigs/wg-device-management/tree/main/topology/gpu)
- [Slack thread: DRA discussion (Aug 2025)](https://cloud-native.slack.com/archives/C0742LDFZ4K/p1754410574841529)
- [Slack thread: DRA scope (May 2026)](https://cloud-native.slack.com/archives/C0742LDFZ4K/p1779107242466099)
