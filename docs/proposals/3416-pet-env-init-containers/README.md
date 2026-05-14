# KEP-3416: Inject Torch Distributed `PET_*` Envs into Trainer Init Containers

## Summary

Torch `EnforceMLPolicy` injects `PET_*` envs only into trainer main container today. This KEP proposes adding an API field in `TorchMLPolicy` to opt-in `PET_*` env injection into trainer init containers.

## Motivation

Init containers cannot read distributed topology envs (`PET_NNODES`, `PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`, `PET_MASTER_PORT`). This blocks preflight distributed checks before expensive training startup.

Also, this proposal only solves env visibility for init containers. It does not change DNS publishing behavior. If preflight scripts need to resolve `PET_MASTER_ADDR` before Pods become Ready, the runtime network settings must allow publishing pod DNS records for not-ready Pods (for example, `publishNotReadyAddresses: true` in JobSet network config).

Current code facts:

- `pkg/runtime/framework/plugins/torch/torch.go`: `EnforceMLPolicy` updates only main trainer container path.
- `pkg/runtime/runtime.go`: `PodSet` already stores both `InitContainers` and `Containers`.
- `pkg/runtime/framework/plugins/jobset/jobset.go`: Build sync writes `ps.Containers` back, but not `ps.InitContainers`.

Which envs are needed by preflight:

- Always needed for distributed connection checks: `PET_MASTER_ADDR`, `PET_MASTER_PORT`, `PET_NODE_RANK`.
- Commonly needed by launch logic: `PET_NNODES`, `PET_NPROC_PER_NODE`.
- `PET_NODE_RANK` can be read from Pod metadata (`batch.kubernetes.io/job-completion-index`), but the other values are runtime-derived and must still be injected by the plugin.

This KEP does not claim that every preflight check needs all `PET_*` envs.
The goal is to make the same runtime-computed `PET_*` values available to init containers when users choose to use them.

### User Stories

- As a platform administrator, I want `PET_*` topology environment variables available in  distributed trainer init containers so preflight checks can validate distributed readiness before expensive training starts.
- As a job submitter, I want preflight to fail fast with clear, machine-readable reasons (GPU, network, DNS, storage, runtime smoke test) so I can fix issues quickly instead of debugging mid-run failures.
- As an operator, I want preflight outcomes to map to deterministic actions ( Warning , Retry , Reschedule , Stop ) to avoid inconsistent behavior across clusters.
- As a runtime engineer, I want early clarify and detect any of unstable problems like ensure cross-pod DNS resolution for MASTER_ADDR to prevent out-of-band communication failures during training.


Moreover: 
- Init-container preflight emits structured results ( json ) and normalized exit codes (example: 0=pass , 10=warning , 20=retryable , 30=fatal ).
- Preflight covers at least: GPU health, driver/CUDA compatibility, NCCL connectivity, Kubernetes API reachability, storage accessibility, minimal torchrun smoke test, and repeated DNS resolution for MASTER_ADDR .

| ID | Real-World Story (Init-Container Context) | Typical Check in Init Container | Recommended Action | Rationale |
|----|------------------------------------------|--------------------------------|--------------------|-----------|
| 1 | GPU missing/unhealthy on a node causes immediate CUDA failures after launch. | `nvidia-smi -L`, DCGM health/diag (or vendor equivalent) | Stop + Reschedule | Node-local hardware issue is unlikely to self-heal in-place. |
| 2 | Driver/CUDA incompatibility causes runtime crashes despite successful pod startup. | Compare host driver (`nvidia-smi`) vs image CUDA compatibility | Stop | Configuration mismatch; rescheduling usually reproduces same failure class. |
| 3 | NCCL path is broken across nodes, leading to all-reduce hang/timeout. | `nccl-tests` (small all-reduce) using PET_* topology | Retry once → Reschedule once → Stop | Transient network issues may recover; persistent failures should fail fast. |
| 4 | API server reachability is intermittent, causing control-plane communication issues. | `curl https://$KUBERNETES_SERVICE_HOST:$PORT/version` | Warning + Retry, then Stop if persistent | Short blips are common; sustained failure is fatal for orchestration. |
| 5 | Required storage path is not writable/readable, causing checkpoint/data IO failures. | Read/write/delete probe on mounted volumes | Stop for required path; Warning + Degrade for optional cache path | Required IO must be hard-gated; optional paths can fall back. |
| 6 | Minimal distributed launch fails although single checks pass. | Tiny `torchrun` smoke test (`--nnodes`, `--nproc_per_node`) | Stop + Reschedule once | End-to-end distributed readiness is the final gate before expensive training. |
| 7 | Cross-pod DNS for `MASTER_ADDR` is unstable; out-of-band runtime communication fails mid-run. | Resolve `MASTER_ADDR` repeatedly (`nslookup` / `getent hosts`) and optional TCP probe | Warning + Degrade if fallback endpoint exists; otherwise Stop | Name resolution instability can silently break runtime coordination later. Ensure `publishNotReadyAddresses=true` when early resolution is required. |





### Goals

- Keep `PET_*` env injection to trainer main container unchanged.
- Add a type-safe API field in `TorchMLPolicy` to opt-in `PET_*` env injection for trainer containers.
- Keep one deterministic env source for both container types.
- Keep scheduler behavior unchanged.

### Non-Goals

- Add user-facing fields to `TrainJob` CRD (deferred to future RuntimePatches discussion).
- Change scheduling semantics.
- Change JobSet network defaults or DNS behavior.

## Proposal

Keep existing behavior by default: inject `PET_*` only into trainer main container.

Add a new `envInjection` field to `TorchMLPolicySource` in the TrainingRuntime CRD.
When configured, it applies the same `PET_*` env set to specified containers in the
target replicated jobs.

Proposed API:

```yaml
# TrainingRuntime.spec.mlPolicy.torch
torch:
  numNodes: 4
  envInjection:
    targets:
    - jobName: "node"
      containerNames: ["nccl-check", "driver-check"]
```

```go
type TorchMLPolicySource struct {
    // envInjection specifies PET_* env injection configuration.
    // Defaults to empty (main container only).
    // +optional
    EnvInjection *EnvInjection `json:"envInjection,omitempty"`
}

// EnvInjection specifies which containers in which jobs receive framework env injection.
// Defined at the MLPolicy level to allow reuse across policy types in the future.
type EnvInjection struct {
    // targets defines which replicated job containers receive PET_* env injection.
    // +listType=map
    // +listMapKey=jobName
    // +optional
    Targets []EnvInjectionTarget `json:"targets,omitempty"`
}

type EnvInjectionTarget struct {
    // jobName is the name of the target replicated job (e.g. "node").
    // Using "jobName" rather than "replicatedJobName" keeps the API
    // future-proof for other CRD types (LWS, Grove, Slurm, etc.).
    // +kubebuilder:validation:MinLength=1
    // +required
    JobName string `json:"jobName"`

    // containerNames lists the container names within the target job
    // that should receive PET_* envs.
    // +listType=set
    // +kubebuilder:validation:MinItems=1
    // +required
    ContainerNames []string `json:"containerNames"`
}
```

The main container `node` is always injected with `PET_*` envs regardless of
`envInjection` settings. It does NOT need to be listed in `containerNames`.

When `envInjection` is omitted (`EnvInjection == nil`) or `targets` is empty,
only the main container receives `PET_*` envs (backward-compatible default).

## Design Details

### Runtime helper

Add a helper to find a container by replicated job name and container name,
supporting both main containers and init containers, or generalize the existing
lookup helper (e.g., `FindContainerByPodSetAncestorContainerName`) to accept
a `searchInitContainers` flag.

### Torch plugin changes

In `EnforceMLPolicy`, after `PET_*` values are computed:

- Keep existing injection to trainer main container (node).
- For each entry in `EnvInjection.Targets`, find the PodSet matching `jobName`, then locate each container in `containerNames` (searching both main containers and init containers) and inject the same `PET_*` envs.
- Keep torchtune command mutation scoped to trainer main container only.

### JobSet plugin changes

In `Build`, mirror existing sync logic for `ps.Containers` to `ps.InitContainers`:

- Sync command, image, env, ports, and volumeMounts where applicable.
- Write updates to `ReplicatedJobs[*].Template.Spec.Template.Spec.InitContainers`.

### Which `PET_*` values come from where

- `PET_NODE_RANK`: comes from Pod metadata field `batch.kubernetes.io/job-completion-index`.
- `PET_MASTER_ADDR`: computed by runtime naming convention for the master Pod DNS name.
- `PET_MASTER_PORT`: set from trainer runtime port config.
- `PET_NNODES`, `PET_NPROC_PER_NODE`: derived from TrainJob/runtime policy values.

### Validation and Safety

Reserved-env validation currently checks only `spec.trainer.env`. This KEP does not expand API validation scope.

At the implementation level, the torch plugin should validate that each container name listed in `containerNames` actually exists in the target replicated job. Missing container names should produce an error to prevent silent misconfiguration.

### Networking prerequisite for preflight

`PET_MASTER_ADDR` is injected as a DNS name (not a direct Pod IP). Because of that, preflight checks that run before readiness may fail to resolve the address when pod DNS records are not published for not-ready Pods.

This KEP does not enforce any network setting. Runtime authors and users should ensure the selected runtime template has suitable JobSet network configuration when they depend on early DNS resolution (for example, `publishNotReadyAddresses: true`).

### Compatibility

Backward compatible by default.

- Existing jobs keep current behavior (main container injection only).
- Jobs without init containers are unchanged.
- Init-container injection is enabled only for users who opt in.

## Test Plan

- [x] I/we understand the owners of involved components may require updates to existing tests before implementation is merged.

### Unit Tests

- Add torch plugin unit test with trainer `PodSet` containing init containers.
- Verify default behavior: `PET_*` env injection only for main container (`node`).
- Verify opt-in behavior: when `envInjection.targets` lists a `jobName` with `containerNames`, `PET_*` envs are injected into the matching containers (both main and init containers) and always into the main container.
- Verify that containers not listed in `containerNames` do not receive `PET_*` envs.

## Implementation History

- **TBD**: Issue opened [#3416](https://github.com/kubeflow/trainer/issues/3416)
- **2026-04-07**: Initial KEP drafted.

## Alternatives

### Alternative 1: Inject into all init containers (blanket approach)

- **Pros:** Simplest to configure (single boolean/flag). No need to list container names.
- **Cons:** Pollutes unrelated init containers with `PET_*` envs. Less predictable for users who have multiple init containers with different purposes.

### Alternative 2: Run preflight in main container startup path (entrypoint)

- **Pros:** Works without injecting `PET_*` into init containers.
- **Cons:** Startup probes and entrypoint checks have different failure behavior from init-container gating, and they still depend on DNS/network settings for `PET_MASTER_ADDR` resolution after Pod's ready (without explicit `publishNotReadyAddresses: true`)

### Alternative 3: Container name list without jobName

- **Pros:** Shorter configuration when all target containers are in the same replicated job.
- **Cons:** Implicit scoping (always the trainer job) limits future extensibility to other CRDs (LWS, Grove, Slurm) and makes the API less declarative.

## Open Questions

Should a future KEP support the `jobName` in `torch.envInjection.targets` when the runtime template uses a different CRD (LWS, Grove, Slurm, etc.)?

Should a future KEP allow omitting `jobName` in a target to auto-match all replicated jobs (a "match container name in any job" mode)?

Should TrainJob allow overriding `envInjection` via RuntimePatches (for example by extending `TrainingRuntimeSpecPatch`)?
