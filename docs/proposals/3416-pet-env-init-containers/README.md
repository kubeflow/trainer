# KEP-3416: Inject Torch Distributed `PET_*` Envs into Trainer Init Containers

## Summary

Torch `EnforceMLPolicy` injects `PET_*` envs only into trainer main container today. This KEP proposes an opt-in way to inject the same `PET_*` envs into trainer init containers.

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
- Add opt-in `PET_*` env injection for trainer init containers.
- Keep one deterministic env source for both container types.
- Keep scheduler behavior unchanged.

### Non-Goals

- Change CRD or API schema.
- Add new CRD or API fields.
- Change scheduling semantics.
- Change JobSet network defaults or DNS behavior.

## Proposal

Keep existing behavior by default: inject `PET_*` only into trainer main container.

Add annotation-based opt-in for init containers. When enabled, apply the same `PET_*` env set to trainer init containers in `PodSet` (`AncestorTrainer`).
In this KEP, the annotation controls only Torch plugin env injection behavior.

Proposed annotation:

- `trainer.kubeflow.org/plugin-env-injection-mode: "init-containers"`

## Design Details

### Runtime helper

Add helper for init-container lookup by podset ancestor and container name, or generalize existing lookup helper to support both main and init containers.

### Torch plugin changes

In `EnforceMLPolicy`, after `PET_*` values are computed:

- Keep existing injection to trainer main container.
- Add injection to trainer init containers only when `trainer.kubeflow.org/plugin-env-injection-mode` is set to `init-containers`.
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
- Verify default behavior: `PET_*` env injection only for main container.
- Verify opt-in behavior: `PET_*` env injection for main and init containers.
- Add or extend JobSet Build test to verify init container sync in final JobSet spec.

## Implementation History

- **TBD**: Issue opened [#3416](https://github.com/kubeflow/trainer/issues/3416)
- **2026-04-07**: Initial KEP drafted.

## Alternatives

### Alternative 1: Inject only into selected init containers

- **Pros:** Smaller runtime mutation scope. Also allows mixed sourcing of env values:
  - `PET_NODE_RANK` from Pod `fieldRef` (`batch.kubernetes.io/job-completion-index`).
  - `PET_NNODES` or `PET_NPROC_PER_NODE` from user-provided overrides when values are fixed.
  - `PET_MASTER_ADDR` can be derived from `metadata.annotations['jobset.sigs.k8s.io/jobset-name']` (`$(JOBSET_NAME)-node-0-0.$(JOBSET_NAME)`)
  - `PET_MASTER_PORT` can be fixed to `29500`, so it may not need explicit injection in some setups.
- **Cons:** Need clear precedence rules when values are available from multiple sources (plugin injection, metadata-derived values, and user-provided envs). For example, `PET_NNODES` may duplicate `TrainJob.spec.trainer.numNodes`, which can cause configuration drift.

### Alternative 2: Run preflight in main container startup path(entrypoint)

- **Pros:** Works without injecting `PET_*` into init containers.
- **Cons:** Startup probes and entrypoint checks have different failure behavior from init-container gating, and they still depend on DNS/network settings for `PET_MASTER_ADDR` resolution after Pod's ready (without explicit `publishNotReadyAddresses: true` )



## Open Questions

Should a future KEP change the default from opt-in to opt-out after enough adoption data?

Should this annotation-based control move to a dedicated API field in Torch MLPolicy for better type safety and discoverability?

If a dedicated API field is introduced, should TrainJob override be supported via RuntimePatches (for example by extending `TrainingRuntimeSpecPatch`)?

Should future scope include finer-grained targets such as specific replicated jobs and container names?
