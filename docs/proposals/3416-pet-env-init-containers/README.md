# KEP-3416: Inject Torch Distributed `PET_*` Envs into Trainer Init Containers

## Summary

Torch `EnforceMLPolicy` injects `PET_*` envs only into trainer main container today. This KEP proposes to inject same `PET_*` envs into trainer init containers as well.

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

### Goals

- Inject `PET_*` envs to trainer main container and all trainer init containers.
- Keep one deterministic env source for both container types.
- Keep scheduler behavior unchanged.

### Non-Goals

- Change CRD or API schema.
- Add new user-facing field.
- Change scheduling semantics.
- Change JobSet network defaults or DNS behavior.

## Proposal

Apply `PET_*` env set to all containers in trainer `PodSet` (`AncestorTrainer`) with same values.

## Design Details

### Runtime helper

Add helper for init-container lookup by podset ancestor and container name, or generalize existing lookup helper to support both main and init containers.

### Torch plugin changes

In `EnforceMLPolicy`, after `PET_*` values are computed:

- Keep existing injection to trainer main container.
- Add injection to every trainer init container.
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

Backward compatible. Jobs without init containers are unchanged.

## Test Plan

- [x] I/we understand the owners of involved components may require updates to existing tests before implementation is merged.

### Unit Tests

- Add torch plugin unit test with trainer `PodSet` containing init containers.
- Verify `PET_*` env injection for main and init containers.
- Add or extend JobSet Build test to verify init container sync in final JobSet spec.

## Implementation History

- **TBD**: Issue opened [#3416](https://github.com/kubeflow/trainer/issues/3416)
- **2026-04-07**: Initial KEP drafted.

## Alternatives

### Alternative 1: Inject only into selected init containers

- **Pros:** Smaller runtime mutation scope.
- **Cons:** Need selection API/annotation and user education; less predictable behavior.

### Alternative 2: Run preflight in main container startup path

- **Pros:** Works without injecting `PET_*` into init containers.
- **Cons:** Startup probes and entrypoint checks have different failure behavior from init-container gating, and they still depend on DNS/network settings for `PET_MASTER_ADDR` resolution.


## Open Questions

Should this KEP include an opt-out switch to inject `PET_*` only into the main container?

- Option A: keep this KEP simple and inject into all init containers by default.
- Option B: add annotation-based control (for example, `trainer.kubeflow.org/pet-init-env-injection: "false"`).

If annotation-based control is added, rollout and compatibility policy must be explicit:

- Keep current behavior as default first to avoid surprise changes for existing runtimes.
- Define conflict handling when users already set envs with the same names.
- Document migration steps if default behavior changes in a future release.
