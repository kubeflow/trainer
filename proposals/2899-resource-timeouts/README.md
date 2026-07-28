# KEP-2899: Add Resource Timeouts APIs to the Trainer

## Summary

Add lifecycle management fields to the Trainer APIs:

- **`ActiveDeadlineSeconds`** on `TrainJobSpec` (shipped in v2.2): lets data scientists set the maximum runtime for individual TrainJobs via the Kubeflow SDK.
- **`TTLSecondsAfterFinished`** on `TrainJobSpec`: lets data scientists set how long a finished TrainJob and its child resources are retained before automatic cleanup.
- **`RunPolicy`** on `TrainingRuntimeSpec` (`ClusterTrainingRuntime` and `TrainingRuntime`): lets platform admins set runtime-level defaults for `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished` that TrainJobs using the runtime inherit and can override.

This brings TrainJob lifecycle management in line with Kubernetes Jobs and JobSets.

## Motivation

Currently, `TrainJob` resources persist in the cluster indefinitely after completion unless manually deleted. This leads to:

- **Etcd Bloat:** Accumulation of stale metadata in the cluster state.
- **Resource Contention:** Runaway training jobs can consume GPU/CPU resources indefinitely if they hang or enter an infinite loop.
- **Operational Overhead:** Platform admins have no centralized way to enforce cleanup policies.

### Goals

- Add `ActiveDeadlineSeconds` to `TrainJobSpec` for data scientists to control individual job timeouts
- Add `TTLSecondsAfterFinished` to `TrainJobSpec` for data scientists to control post-finish cleanup of individual jobs
- Add a `RunPolicy` struct to `TrainingRuntimeSpec` so platform admins can set default `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished` for all TrainJobs using a runtime, with per-TrainJob overrides
- Expose `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished` in the Kubeflow Python SDK for data scientists
- Follow Kubernetes Job/JobSet patterns and existing Trainer API conventions

### Non-Goals

- Automatically migrate existing TrainJobs to use new defaults
- Provide per-namespace TTL overrides

## Proposal

### User Stories

#### Story 1

As a **Data Scientist**, I want to set a maximum runtime on my TrainJob so that a training job that hangs or diverges is automatically terminated after a specified duration, freeing up expensive GPU resources for other experiments.

#### Story 2

As a **Data Scientist**, I want to set a `activeDeadlineSeconds` via the Kubeflow Python SDK when submitting a training job from my notebook, so that I don't need to write or understand Kubernetes YAML to protect my experiment from running indefinitely.

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, Initializer, HuggingFaceDatasetInitializer

TrainerClient().train(
    trainer=CustomTrainer(
        func=train_func,
        num_nodes=3,
    ),
    initializer=Initializer(
        model=HuggingFaceDatasetInitializer(storage_uri="hf://qwen3.2-instruct")
    ),
    activeDeadlineSeconds=28800,  # 8 hours max
)
```

#### Story 3

As a **Platform Admin**, I want to set default `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished` on a `ClusterTrainingRuntime` so that every TrainJob using that runtime is bounded and cleaned up automatically, without each data scientist setting these fields. Data scientists can still override either value on an individual TrainJob.

#### Story 4

As a **Data Scientist**, I want finished TrainJobs and their pods, services, and endpoints to be removed automatically after a bounded time, so that stale resources from completed runs do not accumulate or interfere with later jobs (for example, Pod IP reuse resolving to a previous job's headless Service).

## Design Details

### API Design

#### TrainJobSpec Changes

Add `ActiveDeadlineSeconds` to `TrainJobSpec` in `pkg/apis/trainer/v1alpha1/trainjob_types.go`:

```go
type TrainJobSpec struct {
    // ... existing fields ...

    // ActiveDeadlineSeconds specifies the duration in seconds relative to the TrainJob
    // start time (which resets on resume from suspension) that the TrainJob may be active
    // before the system tries to terminate it. Value must be a positive integer.
    // Once reached, all running Pods are terminated and the TrainJob status becomes
    // Failed with reason: DeadlineExceeded.
    // +optional
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:XValidation:rule="self == oldSelf",message="field is immutable"
    ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`

    // TTLSecondsAfterFinished specifies the duration in seconds the TrainJob is retained after it
    // reaches a terminal state (Complete or Failed), after which the TrainJob and its child
    // resources become eligible for automatic deletion. Value must be a non-negative integer;
    // 0 means delete immediately after finishing. Overrides the runtime default in
    // spec.runPolicy.ttlSecondsAfterFinished. Following Kubernetes Job semantics, this field is
    // mutable after creation.
    // +optional
    // +kubebuilder:validation:Minimum=0
    TTLSecondsAfterFinished *int64 `json:"ttlSecondsAfterFinished,omitempty"`
}
```

#### TrainingRuntimeSpec Changes

Add a `RunPolicy` struct to `TrainingRuntimeSpec` (used by both `ClusterTrainingRuntime` and `TrainingRuntime`) in `pkg/apis/trainer/v1alpha1/trainingruntime_types.go`:

```go
type TrainingRuntimeSpec struct {
    // ... existing fields ...

    // RunPolicy defines lifecycle defaults applied to TrainJobs that reference this runtime.
    // Each field is a default that a TrainJob can override on its own spec.
    // +optional
    RunPolicy *RunPolicy `json:"runPolicy,omitempty"`
}

// RunPolicy holds runtime-level lifecycle defaults for TrainJobs using this runtime.
type RunPolicy struct {
    // ActiveDeadlineSeconds is the default maximum active duration in seconds for TrainJobs that
    // reference this runtime. A TrainJob's own spec.activeDeadlineSeconds takes precedence.
    // +optional
    // +kubebuilder:validation:Minimum=1
    ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`

    // TTLSecondsAfterFinished is the default retention duration in seconds after a TrainJob using
    // this runtime reaches a terminal state, after which it is eligible for automatic deletion.
    // A TrainJob's own spec.ttlSecondsAfterFinished takes precedence.
    // +optional
    // +kubebuilder:validation:Minimum=0
    TTLSecondsAfterFinished *int64 `json:"ttlSecondsAfterFinished,omitempty"`
}
```

The name `runPolicy` is used on the runtime (rather than a flat field) so it reads as policy applied to the resulting runs, not to the runtime object's own lifecycle. On `TrainJobSpec` the fields stay flat, matching the already-shipped `activeDeadlineSeconds`.

#### New Condition Reason

Add new condition reason in `pkg/apis/trainer/v1alpha1/trainjob_types.go`:

```go
const (
    // TrainJobDeadlineExceededReason is the reason for the "Failed" condition
    // when ActiveDeadlineSeconds is exceeded.
    TrainJobDeadlineExceededReason string = "DeadlineExceeded"
)
```

When a TrainJob exceeds its `ActiveDeadlineSeconds`, the controller sets a `Failed` condition with `Reason: DeadlineExceeded`, matching the [Kubernetes Job behavior](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup).

### Value Resolution

Both `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished` follow the same precedence: the value on `TrainJobSpec` wins if set; otherwise the controller falls back to the runtime default in `TrainingRuntimeSpec.RunPolicy`; if neither is set, the feature is disabled for that TrainJob.

```
effectiveValue = trainJob.spec.<field>  (if set)
              else runtime.spec.runPolicy.<field>  (if set)
              else unset (no enforcement / no auto-cleanup)
```

Resolution happens per reconcile, so a runtime default cannot silently override a value the user explicitly set on the TrainJob.

### User Examples

**TrainJob with Deadline (Data Scientist):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: quick-experiment
spec:
  activeDeadlineSeconds: 28800        # Max runtime: 8 hours
  runtimeRef:
    name: torch-distributed-gpu
  trainer:
    image: my-training:latest
    numNodes: 2
# 8-hour deadline set on TrainJob
```

**TrainJob with activeDeadlineSeconds via SDK (Data Scientist):**

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, Initializer, HuggingFaceDatasetInitializer

TrainerClient().train(
    trainer=CustomTrainer(
        func=train_func,
        num_nodes=3,
    ),
    initializer=Initializer(
        model=HuggingFaceDatasetInitializer(storage_uri="hf://qwen3.2-instruct")
    ),
    active_deadline_seconds=28800,  # 8 hours max
)
```

The `active_deadline_seconds` parameter in the SDK maps to `ActiveDeadlineSeconds` on the created `TrainJob`. `ttl_seconds_after_finished` maps to `TTLSecondsAfterFinished` the same way.

**Runtime defaults (Platform Admin):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-gpu
spec:
  runPolicy:
    activeDeadlineSeconds: 28800       # Default max runtime: 8 hours
    ttlSecondsAfterFinished: 3600      # Default cleanup: 1 hour after finish
  template:
    # ... JobSet template ...
```

**TrainJob overriding the runtime TTL default (Data Scientist):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: keep-longer-for-debugging
spec:
  ttlSecondsAfterFinished: 86400       # Keep this failed run for 24h, overrides the 1h default
  runtimeRef:
    name: torch-distributed-gpu
  trainer:
    numNodes: 2
```

### Implementation Overview

**Controller Changes** (`pkg/controller/trainjob_controller.go`):

1. **Value Resolution:**
    - Resolve `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished` using the precedence above: `TrainJobSpec` value if set, else `TrainingRuntimeSpec.RunPolicy` default, else unset

2. **Deadline Enforcement:**
    - Check if job is running and effective deadline is set
    - Calculate `deadline = startTime + effectiveActiveDeadlineSeconds` (where `startTime` is reset on each resume from suspension)
    - If exceeded, mark TrainJob as Failed (`Reason: DeadlineExceeded`); the runtime framework handles cleanup of the underlying JobSet
    - Otherwise, requeue at `deadline`

3. **TTL Enforcement:**
    - Only applies once the TrainJob is finished (`Complete` or `Failed` condition is true)
    - Calculate `expiry = finishTime + effectiveTTLSecondsAfterFinished`, where `finishTime` is the transition time of the terminal condition
    - If `expiry` has passed, delete the TrainJob; owner references cascade the delete to the child JobSet, Jobs, Pods, and Services, matching Kubernetes Job `ttlSecondsAfterFinished` semantics
    - Otherwise, requeue at `expiry`
    - TTL deletes the TrainJob itself rather than only the child JobSet. This avoids the JobSet-recreation loop from [#3779](https://github.com/kubeflow/trainer/issues/3779): once the parent TrainJob is gone there is nothing left to reconcile, so no new JobSet is created with a reset restart count

4. **Clock Skew Handling:**
    - If calculated requeue time is in the past (due to clock skew), requeue with a small delay (e.g., 1 second)

### Clock Skew Handling

Kubernetes clusters may experience clock skew between nodes. When calculating requeue times:

- If the calculated `RequeueAfter` duration is negative or zero (due to clock skew or processing delays), the controller requeues with a 1-second delay
- This prevents tight reconciliation loops while ensuring timely processing
- Example: If `deadline` is 10:00:00 but the controller's clock reads 10:00:02, instead of an invalid negative requeue, we wait 1 second and retry

```go
requeueAfter := deadline.Sub(time.Now())
if requeueAfter <= 0 {
    // Clock skew detected, use minimum delay
    requeueAfter = 1 * time.Second
}
return ctrl.Result{RequeueAfter: requeueAfter}, nil
```


### Controller Restart Behavior

The controller is stateless and stores no timers in memory. On restart:

1. Controller-runtime triggers initial sync, reconciling all TrainJobs
2. For each TrainJob, deadlines are recalculated from:
   - The last resume time (or `metadata.creationTimestamp` if never suspended) for deadline calculation
3. If deadline already expired during downtime, action is taken immediately
4. Otherwise, appropriate requeue times are set

This design ensures no TrainJobs are "forgotten" after a controller restart.

**Validation:**

**Field-level CEL markers** on the API types:

- `Minimum=1` on `ActiveDeadlineSeconds` (`TrainJobSpec` and `RunPolicy`)
- `Minimum=0` on `TTLSecondsAfterFinished` (`TrainJobSpec` and `RunPolicy`)
- `XValidation: self == oldSelf` on `ActiveDeadlineSeconds` (`TrainJobSpec`) - immutable after creation
- `TTLSecondsAfterFinished` is mutable after creation, matching Kubernetes Job behavior (users may extend or shorten retention on a finished job)

**Cross-field CEL markers** on `TrainingRuntimeSpec` to prevent conflicting lifecycle fields in the JobSet/Job template:

- `self.template.spec.replicatedJobs.all(rj, !has(rj.template.spec.activeDeadlineSeconds))` - Job-level deadline would terminate pods independently from TrainJob deadline tracking
- `!has(self.template.spec.ttlSecondsAfterFinished)` - JobSet-level TTL would delete the JobSet out from under the TrainJob and trigger the recreation loop in [#3779](https://github.com/kubeflow/trainer/issues/3779); TTL must be expressed via `runPolicy` or the TrainJob so the controller owns cleanup

### Interaction with Suspend

Matching Kubernetes Job behavior (K8s 1.35+ with `MutableSchedulingDirectivesForSuspendedJobs`), the `ActiveDeadlineSeconds` timer is **stopped and reset** when a TrainJob is suspended. When the TrainJob is resumed, the timer **restarts from zero**, giving the job the full `ActiveDeadlineSeconds` duration again.

- If a TrainJob is created in a suspended state, the timer does not start until the TrainJob is first unsuspended
- When a running TrainJob is suspended, the controller clears the internal start time reference. On resume, the start time is reset to the current time, and the full `ActiveDeadlineSeconds` window applies from that point

### Test Plan

[x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Unit Tests

- `pkg/controller/`: High coverage expected for new logic in `trainjob_controller.go`

**Test Cases:**
- Deadline from TrainJob → enforced
- No deadline set → no enforcement
- Deadline exceeded → job failed with DeadlineExceeded reason
- Deadline not reached → requeue at deadline
- Clock skew → requeue with delay instead of negative duration
- Value resolution: TrainJob value set → runtime default ignored
- Value resolution: TrainJob value unset, runtime default set → runtime default used
- Value resolution: neither set → feature disabled
- TTL on unfinished job → no deletion
- TTL expired on finished job → TrainJob deleted (children cascade)
- TTL not reached on finished job → requeue at expiry

#### Integration Tests

- `test/integration/controller/trainjob_controller_test.go`:
    - End-to-end deadline enforcement from TrainJob
    - Suspended TrainJob → deadline timer does not start until first unsuspend
    - Running TrainJob suspended and resumed → deadline timer resets (full duration available again)

#### E2E Tests

- `test/e2e/trainjob_deadline_test.go`:
    - Real training workload with deadline: Verify job fails at timeout with DeadlineExceeded reason
    - Verify no orphaned resources remain
- `test/e2e/trainjob_ttl_test.go`:
    - Real training workload with TTL: verify the TrainJob and its child JobSet, Pods, and Services are deleted after the TTL elapses
    - Verify a failed TrainJob with maxRestarts stays terminally Failed and is not recreated after TTL cleanup (regression for [#3779](https://github.com/kubeflow/trainer/issues/3779))

## Future Plan

The initial `RunPolicy` struct holds `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished`. If future lifecycle knobs (for example `BackoffLimit` or a cleanup policy) are needed, they can be added under `RunPolicy` without breaking changes.

## Implementation History

- **2025-10-20**: Issue opened [#2899](https://github.com/kubeflow/trainer/issues/2899)
- **2026-01-04**: Initial KEP drafted
- **2026-01-22**: KEP updated with layered API design (TrainJob + TrainingRuntime)
- **2026-02-28**: `ActiveDeadlineSeconds` on `TrainJobSpec` shipped in v2.2
- **2026-07-28**: KEP extended with `TTLSecondsAfterFinished` and the `RunPolicy` runtime defaults, prompted by [#3779](https://github.com/kubeflow/trainer/issues/3779)
- **TBD**: TTL alpha implementation


## Alternatives

### Alternative 1: Both Fields on TrainJobSpec Only

Put both `TTLSecondsAfterFinished` and `ActiveDeadlineSeconds` only on `TrainJobSpec`.

**Pros:**
- Simpler API surface
- Users have full control

**Cons:**
- No centralized policy enforcement for platform admins
- Data scientists must set TTL on every job
- Difficult to enforce cluster-wide cleanup policies

### Alternative 2: Both Fields on TrainingRuntimeSpec Only

Put both fields only on `TrainingRuntimeSpec`.

**Pros:**
- Centralized control for platform admins
- Consistent policies across all jobs

**Cons:**
- Data scientists cannot customize deadlines for specific jobs
- Less flexible for varying job requirements

### Alternative 3: Runtime defaults that TrainJobs can override

Add lifecycle fields to `TrainingRuntimeSpec` as defaults that individual TrainJobs can override.

**Pros:**
- Platform admins can enforce default deadlines and cleanup for all jobs
- Data scientists can still override per job

**Cons:**
- Adds complexity to value resolution logic
- Potential user confusion (users may not realize a default exists)

**Decision:** Adopted for `ActiveDeadlineSeconds` and `TTLSecondsAfterFinished`, grouped under a `RunPolicy` struct on `TrainingRuntimeSpec` (see API Design). The runtime defaults are additive, so this does not break the already-shipped flat `TrainJobSpec.ActiveDeadlineSeconds`.

### Alternative 4: Flat `ttlSecondsAfterFinished` on the runtime

Put `ttlSecondsAfterFinished` directly on `TrainingRuntimeSpec` instead of under `RunPolicy`.

**Cons:**
- On a runtime (a template that never runs), a bare `ttlSecondsAfterFinished` reads as "TTL of the runtime object," which is misleading
- No grouping for future lifecycle knobs

**Decision:** Rejected in favor of `RunPolicy` on the runtime. `TrainJobSpec` keeps flat fields, since a TrainJob is the thing that runs and finishes, so no disambiguation is needed there.
