# KEP-2899: Add TTLSecondsAfterFinished and ActiveDeadlineSeconds to Trainer APIs

## Summary

Add lifecycle management fields to the Trainer APIs with a clear separation of concerns:

- **`ActiveDeadlineSeconds`** on `TrainJobSpec`: Allows data scientists to set maximum runtime for individual TrainJobs via the Kubeflow SDK.
- **`TTLSecondsAfterFinished`** on `TrainingRuntimeSpec`: Allows platform admins to configure automatic cleanup policies as defaults for all TrainJobs using a runtime.

This brings TrainJob lifecycle management in line with Kubernetes Jobs and JobSets while respecting the separation between platform administration and data science workflows.

## Motivation

Currently, `TrainJob` resources persist in the cluster indefinitely after completion unless manually deleted. This leads to:

- **Etcd Bloat:** Accumulation of stale metadata in the cluster state.
- **Resource Contention:** Runaway training jobs can consume GPU/CPU resources indefinitely if they hang or enter an infinite loop.
- **Operational Overhead:** Platform admins have no centralized way to enforce cleanup policies.

### Goals

- Add `ActiveDeadlineSeconds` to `TrainJobSpec` for data scientists to control individual job timeouts
- Add `TTLSecondsAfterFinished` to `TrainingRuntimeSpec` for platform admins to set cleanup defaults
- Expose `ActiveDeadlineSeconds` in the Kubeflow Python SDK for data scientists
- Follow Kubernetes Job/JobSet patterns and existing Trainer API conventions

### Non-Goals

- Expose `TTLSecondsAfterFinished` in the SDK (this is platform admin controlled)
- Automatically migrate existing TrainJobs to use new defaults
- Provide per-namespace TTL overrides

## Proposal

### User Stories

#### Story 1

As a **Data Scientist**, I want to set a maximum runtime on my TrainJob so that a training job that hangs or diverges is automatically terminated after a specified duration, freeing up expensive GPU resources for other experiments.

#### Story 2

As a **Platform Admin**, I want to configure an automatic cleanup policy on my ClusterTrainingRuntime so that completed or failed TrainJobs are automatically deleted after a defined period, preventing etcd bloat and reducing operational overhead across all teams using the runtime.

#### Story 3

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
}
```

#### TrainingRuntimeSpec Changes

Add `TTLSecondsAfterFinished` to `TrainingRuntimeSpec` in `pkg/apis/trainer/v1alpha1/trainingruntime_types.go`:

```go
type TrainingRuntimeSpec struct {
    // ... existing fields (mlPolicy, podGroupPolicy, template) ...

    // TTLSecondsAfterFinished limits the lifetime of a TrainJob that has finished
    // execution (either Complete or Failed). If this field is set, TrainJobs using
    // this runtime will be deleted after ttlSecondsAfterFinished expires post-completion.
    // If this field is unset, TrainJobs will not be automatically deleted.
    // If set to zero, TrainJobs become eligible for immediate deletion after finishing.
    // This is a platform-level policy that individual TrainJobs cannot override.
    // +optional
    // +kubebuilder:validation:Minimum=0
    TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}
```

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

`ActiveDeadlineSeconds` is set directly on `TrainJobSpec` by the data scientist. `TTLSecondsAfterFinished` is set on `TrainingRuntimeSpec` by the platform admin and applies to all TrainJobs using that runtime.

### User Examples

**TrainingRuntime with TTL (Platform Admin):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-gpu
spec:
  ttlSecondsAfterFinished: 86400      # Auto-delete after 24 hours
  mlPolicy:
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      image: ghcr.io/kubeflow/trainer/torch-trainer
```

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
# 8-hour deadline set on TrainJob, runtime's 24-hour TTL applies after completion
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

The `active_deadline_seconds` parameter in the SDK maps to `ActiveDeadlineSeconds` on the created `TrainJob`.

### Implementation Overview

**Controller Changes** (`pkg/controller/trainjob_controller.go`):

1. **Value Resolution:**
    - Read `ActiveDeadlineSeconds` directly from `TrainJobSpec`
    - Fetch the referenced TrainingRuntime/ClusterTrainingRuntime for `TTLSecondsAfterFinished`

2. **TTL Reconciliation:**
    - Check if job is finished and Runtime has TTL set
    - Calculate `deleteTime = finishTime + TTL`
    - If expired, delete TrainJob (cascades to owned resources)
    - Otherwise, requeue at `deleteTime`

3. **Deadline Enforcement:**
    - Check if job is running and effective deadline is set
    - Calculate `deadline = startTime + effectiveActiveDeadlineSeconds` (where `startTime` is reset on each resume from suspension)
    - If exceeded, mark TrainJob as Failed (`Reason: DeadlineExceeded`); the runtime framework handles cleanup of the underlying JobSet
    - Otherwise, requeue at `deadline`

4. **Clock Skew Handling:**
    - If calculated requeue time is in the past (due to clock skew), requeue with a small delay (e.g., 1 second)

### Clock Skew Handling

Kubernetes clusters may experience clock skew between nodes. When calculating requeue times:

- If the calculated `RequeueAfter` duration is negative or zero (due to clock skew or processing delays), the controller requeues with a 1-second delay
- This prevents tight reconciliation loops while ensuring timely processing
- Example: If `deleteTime` is 10:00:00 but the controller's clock reads 10:00:02, instead of an invalid negative requeue, we wait 1 second and retry

```go
requeueAfter := deleteTime.Sub(time.Now())
if requeueAfter <= 0 {
    // Clock skew detected, use minimum delay
    requeueAfter = 1 * time.Second
}
return ctrl.Result{RequeueAfter: requeueAfter}, nil
```


### Controller Restart Behavior

The controller is stateless and stores no timers in memory. On restart:

1. Controller-runtime triggers initial sync, reconciling all TrainJobs
2. For each TrainJob, deadlines and TTL are recalculated from:
   - The last resume time (or `metadata.creationTimestamp` if never suspended) for deadline calculation
   - `LastTransitionTime` of the `Complete` or `Failed` condition for TTL calculation
   - The referenced TrainingRuntime (protected from deletion via the `ResourceInUse` finalizer)
3. If deadline/TTL already expired during downtime, action is taken immediately
4. Otherwise, appropriate requeue times are set

This design ensures no TrainJobs are "forgotten" after a controller restart.

**Validation:**

**Field-level CEL markers** on the API types:

- `Minimum=1` on `ActiveDeadlineSeconds` (`TrainJobSpec`)
- `Minimum=0` on `TTLSecondsAfterFinished` (`TrainingRuntimeSpec`)
- `XValidation: self == oldSelf` on `ActiveDeadlineSeconds` (`TrainJobSpec`) - immutable after creation

**Cross-field CEL markers** on `TrainingRuntimeSpec` to prevent conflicting lifecycle fields in the JobSet/Job template:

- `!has(self.template.spec.ttlSecondsAfterFinished)` - JobSet-level TTL would delete the JobSet independently, leaving orphaned TrainJobs
- `self.template.spec.replicatedJobs.all(rj, !has(rj.template.spec.activeDeadlineSeconds))` - Job-level deadline would terminate pods independently from TrainJob deadline tracking
- `self.template.spec.replicatedJobs.all(rj, !has(rj.template.spec.ttlSecondsAfterFinished))` - Job-level TTL conflicts with TrainJob-level lifecycle management

**Webhook validation** (not expressible via CEL):
- Warn if `TTLSecondsAfterFinished < 60s` (warnings require webhook response)

### Interaction with Suspend

Matching Kubernetes Job behavior (K8s 1.35+ with `MutableSchedulingDirectivesForSuspendedJobs`), the `ActiveDeadlineSeconds` timer is **stopped and reset** when a TrainJob is suspended. When the TrainJob is resumed, the timer **restarts from zero**, giving the job the full `ActiveDeadlineSeconds` duration again.

- If a TrainJob is created in a suspended state, the timer does not start until the TrainJob is first unsuspended
- When a running TrainJob is suspended, the controller clears the internal start time reference. On resume, the start time is reset to the current time, and the full `ActiveDeadlineSeconds` window applies from that point
- TTL (`TTLSecondsAfterFinished`) is not affected by suspension — it only begins counting after the TrainJob reaches a terminal state (`Complete` or `Failed`)

### Test Plan

[x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Unit Tests

- `pkg/controller/`: High coverage expected for new logic in `trainjob_controller.go`

**Test Cases:**
- TTL from runtime → job deleted after expiration
- TTL not set on runtime → no deletion
- TTL = 0 → immediate deletion after completion
- Deadline from TrainJob → enforced
- No deadline set → no enforcement
- Deadline exceeded → job failed with DeadlineExceeded reason
- Deadline not reached → requeue at deadline
- Clock skew → requeue with delay instead of negative duration

#### Integration Tests

- `test/integration/controller/trainjob_controller_test.go`:
    - End-to-end TTL deletion from Runtime default
    - End-to-end deadline enforcement from TrainJob
    - Cascade deletion of owned resources
    - Controller restart (verify timers resume correctly)
    - Suspended TrainJob → deadline timer does not start until first unsuspend
    - Running TrainJob suspended and resumed → deadline timer resets (full duration available again)
    - Suspended TrainJob reaching terminal state → TTL countdown begins from completion, not from suspend time

#### E2E Tests

- `test/e2e/trainjob_ttl_test.go`:
    - Real training workload with Runtime TTL: Verify resource disappears after expiration
    - Real training workload with deadline: Verify job fails at timeout with DeadlineExceeded reason
    - Verify no orphaned resources remain

## Implementation History

- **2025-10-20**: Issue opened [#2899](https://github.com/kubeflow/trainer/issues/2899)
- **2026-01-04**: Initial KEP drafted
- **2026-01-22**: KEP updated with layered API design (TrainJob + TrainingRuntime)
- **TBD**: Alpha implementation

## Drawbacks

- **Loss of Job History:** TTL deletion removes TrainJob metadata permanently

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

### Alternative 3: Add `ActiveDeadlineSeconds` to TrainingRuntimeSpec as Default

Add `ActiveDeadlineSeconds` to `TrainingRuntimeSpec` as a default that individual TrainJobs can override.

**Pros:**
- Platform admins can enforce default deadlines for all jobs
- Data scientists can still override per job

**Cons:**
- Adds complexity to value resolution logic
- Potential user confusion (users may not realize a default deadline exists)

**Decision:** Deferred to a future iteration. If user feedback shows demand for runtime-level default deadlines, this extension can be added without breaking changes.
