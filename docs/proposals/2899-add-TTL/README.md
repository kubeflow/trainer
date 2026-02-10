# KEP-2899: Add TTLSecondsAfterFinished and ActiveDeadlineSeconds to Trainer APIs

## Summary

Add lifecycle management fields to the Trainer APIs with a clear separation of concerns:

- **`ActiveDeadlineSeconds`** on `TrainJobSpec`: Allows data scientists to set maximum runtime for individual TrainJobs via the Kubeflow SDK.
- **`TTLSecondsAfterFinished`** on `TrainingRuntimeSpec`: Allows platform admins to configure automatic cleanup policies as defaults for all TrainJobs using a runtime.
- **`ActiveDeadlineSeconds`** on `TrainingRuntimeSpec` (optional): Allows platform admins to set a default deadline that individual TrainJobs can override.

This brings TrainJob lifecycle management in line with Kubernetes Jobs and JobSets while respecting the separation between platform administration and data science workflows.

## Motivation

Currently, `TrainJob` resources persist in the cluster indefinitely after completion unless manually deleted. This leads to:

- **Etcd Bloat:** Accumulation of stale metadata in the cluster state.
- **Resource Contention:** Runaway training jobs can consume GPU/CPU resources indefinitely if they hang or enter an infinite loop.
- **Operational Overhead:** Platform admins have no centralized way to enforce cleanup policies.

### Goals

- Add `ActiveDeadlineSeconds` to `TrainJobSpec` for data scientists to control individual job timeouts
- Add `TTLSecondsAfterFinished` to `TrainingRuntimeSpec` for platform admins to set cleanup defaults
- Optionally add `ActiveDeadlineSeconds` to `TrainingRuntimeSpec` as a default deadline
- Expose `ActiveDeadlineSeconds` in the Kubeflow Python SDK for data scientists
- Follow Kubernetes Job/JobSet patterns and existing Trainer API conventions

### Non-Goals

- Expose `TTLSecondsAfterFinished` in the SDK (this is platform admin controlled)
- Automatically migrate existing TrainJobs to use new defaults
- Provide per-namespace TTL overrides

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
    // This value overrides any default set in the referenced TrainingRuntime.
    // +optional
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:XValidation:rule="self == oldSelf",message="field is immutable"
    ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
}
```

#### TrainingRuntimeSpec Changes

Add both fields to `TrainingRuntimeSpec` in `pkg/apis/trainer/v1alpha1/trainingruntime_types.go`:

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

    // ActiveDeadlineSeconds specifies the default maximum runtime for TrainJobs
    // using this runtime. Individual TrainJobs can override this value by setting
    // their own ActiveDeadlineSeconds.
    // +optional
    // +kubebuilder:validation:Minimum=1
    ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
}
```

#### New Condition Reason

Add new condition reason in `pkg/apis/trainer/v1alpha1/trainjob_types.go`:

```go
const (
    // TrainJobDeadlineExceededReason is used when ActiveDeadlineSeconds is exceeded
    TrainJobDeadlineExceededReason string = "DeadlineExceeded"
)
```

<<<<<<< HEAD
=======

>>>>>>> 230e8a19 (docs: Clarify ActiveDeadlineSeconds behavior with suspension, add TTLSecondsAfterFinished validation, and remove proposed status fields, SDK changes, and metrics.)
### Value Resolution

The controller resolves effective values using the following precedence:

| Field | TrainJob Value | Runtime Value | Effective Value |
|-------|---------------|---------------|-----------------|
| `ActiveDeadlineSeconds` | Set | Set | **TrainJob value** (override) |
| `ActiveDeadlineSeconds` | Set | Unset | TrainJob value |
| `ActiveDeadlineSeconds` | Unset | Set | Runtime value (default) |
| `ActiveDeadlineSeconds` | Unset | Unset | No deadline enforced |
| `TTLSecondsAfterFinished` | N/A | Set | Runtime value |
| `TTLSecondsAfterFinished` | N/A | Unset | No TTL cleanup |

### User Examples

**TrainingRuntime with Defaults (Platform Admin):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-gpu
spec:
  ttlSecondsAfterFinished: 86400      # Auto-delete after 24 hours
  activeDeadlineSeconds: 28800        # Default max runtime: 8 hours
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

**TrainJob Using Defaults (Data Scientist):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: quick-experiment
spec:
  runtimeRef:
    name: torch-distributed-gpu
  trainer:
    image: my-training:latest
    numNodes: 2
# Uses runtime defaults: 8-hour deadline, 24-hour TTL
```

**TrainJob Overriding Deadline (Data Scientist):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: long-finetune
spec:
  activeDeadlineSeconds: 259200       # Override: 72 hours for this job
  runtimeRef:
    name: torch-distributed-gpu
  trainer:
    image: my-training:latest
    numNodes: 8
# Uses job-specific 72-hour deadline, runtime's 24-hour TTL still applies
```

### Implementation Overview

**Controller Changes** (`pkg/controller/trainjob_controller.go`):

1. **Effective Value Resolution:**
    - Fetch the referenced TrainingRuntime/ClusterTrainingRuntime
    - Resolve `ActiveDeadlineSeconds`: TrainJob value takes precedence over Runtime
    - Resolve `TTLSecondsAfterFinished`: Only from Runtime (not on TrainJob)

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

Most validation is handled via kubebuilder CEL markers on the API types:
- `+kubebuilder:validation:Minimum=1` on `ActiveDeadlineSeconds` (both `TrainJobSpec` and `TrainingRuntimeSpec`)
- `+kubebuilder:validation:Minimum=0` on `TTLSecondsAfterFinished` (`TrainingRuntimeSpec`)
- `+kubebuilder:validation:XValidation:rule="self == oldSelf"` on `ActiveDeadlineSeconds` (`TrainJobSpec`) for immutability

The existing `TrainJobValidator` webhook in `pkg/webhooks/trainjob_webhook.go` delegates to `runtime.ValidateObjects()`. The `TrainingRuntimeValidator` and `ClusterTrainingRuntimeValidator` webhooks validate the runtime spec.

The only validation requiring actual webhook code (not expressible via CEL) is:
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
- Deadline from TrainJob only → enforced
- Deadline from Runtime only → enforced as default
- Deadline on both → TrainJob value wins
- Deadline exceeded → job failed with DeadlineExceeded reason
- Deadline not reached → requeue at deadline
- Value resolution with missing runtime → graceful handling
- Clock skew → requeue with delay instead of negative duration

#### Integration Tests

- `test/integration/controller/trainjob_controller_test.go`:
    - End-to-end TTL deletion from Runtime default
    - End-to-end deadline from Runtime default
    - TrainJob deadline overriding Runtime deadline
    - Cascade deletion of owned resources
    - Controller restart (verify timers resume correctly)

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

- **Complexity:** Two-level API (TrainJob + Runtime) adds complexity compared to a single-level approach
- **Potential User Confusion:** Users might not realize their jobs have a default deadline from the Runtime
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
