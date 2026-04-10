# KEP-3416: Inject Torch Distributed `PET_*` Envs into Trainer Init Containers

## Summary

Torch `EnforceMLPolicy` injects `PET_*` envs only into trainer main container today. This KEP proposes to inject same `PET_*` envs into trainer init containers as well.

## Motivation

Init containers cannot read distributed topology envs (`PET_NNODES`, `PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`, `PET_MASTER_PORT`). This blocks preflight distributed checks before expensive training startup.

Current code facts:

- `pkg/runtime/framework/plugins/torch/torch.go`: `EnforceMLPolicy` updates only main trainer container path.
- `pkg/runtime/runtime.go`: `PodSet` already stores both `InitContainers` and `Containers`.
- `pkg/runtime/framework/plugins/jobset/jobset.go`: Build sync writes `ps.Containers` back, but not `ps.InitContainers`.

### Goals

- Inject `PET_*` envs to trainer main container and all trainer init containers.
- Keep one deterministic env source for both container types.
- Keep scheduler behavior unchanged.

### Non-Goals

- Change CRD or API schema.
- Add new user-facing field.
- Change scheduling semantics.

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

### Validation and Safety

Reserved-env validation currently checks only `spec.trainer.env`. This KEP does not expand API validation scope.

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


## Open Questions

Default behavior injects into all trainer init containers. If opt-out is needed later, it can be added in a follow-up KEP (for example, annotation-based control).
