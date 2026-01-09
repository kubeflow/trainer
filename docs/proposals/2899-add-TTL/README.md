# KEP-2899: Add TTLSecondsAfterFinished and ActiveDeadlineSeconds to TrainJob CRD

## Summary

Add `TTLSecondsAfterFinished` and `ActiveDeadlineSeconds` fields to TrainJob CRD to enable automatic cleanup of finished jobs and enforce maximum runtime limits respectively, bringing TrainJob lifecycle management in check with Kubernetes Jobs and JobSets.

## Motivation

Currently, `TrainJob` resources persist in the cluster indefinitely after completion unless manually deleted. This leads to:

- **Etcd Bloat:** Accumulation of stale metadata in the cluster state.
- **Resource Contention:** Runaway training jobs can consume GPU/CPU resources indefinitely if they hang or enter an infinite loop.

### Goals

- Add `TTLSecondsAfterFinished` for automatic deletion of finished TrainJobs
- Add `ActiveDeadlineSeconds` to enforce maximum runtime
- Follow Kubernetes Job/JobSet patterns

## Proposal

- **Interaction with Suspend:** If a TrainJob is suspended, the `ActiveDeadlineSeconds` timer continues to count down. This aligns with the behavior of Kubernetes Jobs.
- **Clock Skew:** Both TTL and deadline enforcement rely on the controller's local clock being synchronized with the Kubernetes API server creation timestamps. Significant clock skew could lead to premature or delayed actions.

### Risks and Mitigations

- **User Confusion:** Users might be surprised when their finished TrainJobs disappear.
    - *Mitigation:* We will rely on clear documentation and potentially add a webhook warning if TTL is set to a very short value (< 60s).
- **Loss of Job History:** Automatic deletion removes the resource and its status.
    - *Mitigation:* Users should utilize logging/monitoring solutions or the future TrainJob History Server to persist results beyond the resource lifespan.

## Design Details

### API Design

Add two optional fields to `TrainJobSpec` in `pkg/apis/trainer/v1alpha1/trainjob_types.go`:

```go
type TrainJobSpec struct {
    // ... existing fields ...

    // TTLSecondsAfterFinished limits the lifetime of a TrainJob that has finished
    // execution (either Complete or Failed). If this field is set, once the TrainJob
    // finishes, it will be deleted after ttlSecondsAfterFinished expires. If this
    // field is unset, the TrainJob will not be automatically deleted. If set to zero,
    // the TrainJob becomes eligible for immediate deletion after finishing.
    // +optional
    TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`

    // ActiveDeadlineSeconds specifies the duration in seconds relative to the TrainJob
    // creation time that the TrainJob may be active before the system tries to terminate
    // it. Value must be a positive integer. Once reached, all running Pods are terminated
    // and the TrainJob status becomes Failed with reason: DeadlineExceeded.
    // +optional
    // +kubebuilder:validation:Minimum=1
    ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
}
```

Add new condition reason:

```go
const (
    // TrainJobDeadlineExceededReason is used when ActiveDeadlineSeconds is exceeded
    TrainJobDeadlineExceededReason string = "DeadlineExceeded"
)
```

**User Example:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-training
spec:
  ttlSecondsAfterFinished: 3600    # Delete 1 hour after completion
  activeDeadlineSeconds: 7200      # Max 2 hours runtime
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 2
```

### Implementation Overview

**Controller Changes** (`pkg/controller/trainjob_controller.go`):

1. **TTL Reconciliation:**
    - Check if job is finished and TTL is set.
    - Calculate `deleteTime = finishTime + TTL`.
    - If expired, delete TrainJob (cascades to owned resources).
    - Otherwise, requeue at `deleteTime`.
2. **Deadline Enforcement:**
    - Check if job is running and deadline is set.
    - Calculate `deadline = creationTime + ActiveDeadlineSeconds`.
    - If exceeded, mark TrainJob as Failed (`Reason: DeadlineExceeded`) and delete JobSet.
    - Otherwise, requeue at `deadline`.
3. **Integration:** Add both logic blocks to the main `Reconcile()` loop.

**Webhook Validation** (`pkg/webhooks/trainjob_webhook.go`):

- Validate `TTLSecondsAfterFinished >= 0`.
- Validate `ActiveDeadlineSeconds > 0`.
- Warn if `TTLSecondsAfterFinished < 60s`.
- Make fields immutable after creation.

### Test Plan

[x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Prerequisite testing updates

None identified.

#### Unit Tests

- `pkg/controller/trainjob/`: 2026-01-04 - High coverage expected for new logic

**Test Cases:**
- TTL not set → no deletion
- TTL expired → job deleted
- TTL not expired → requeue at correct time
- TTL = 0 → immediate deletion
- Deadline exceeded → job failed, pods terminated
- Deadline not reached → requeue at deadline
- Both fields set → correct interaction

#### E2E tests

- `test/e2e/trainjob_ttl_test.go`:
    - Real training workload with TTL: Verify resource disappears after expiration.
    - Real training workload with deadline: Verify job fails at timeout with DeadlineExceeded reason.
    - Verify no orphaned resources remain.

#### Integration tests

- `test/integration/controller/trainjob_controller_test.go`:
    - End-to-end TTL deletion workflow.
    - End-to-end deadline enforcement.
    - Cascade deletion of owned resources.
    - Controller restart (verify timers resume correctly).

## Implementation History

- **2025-10-20**: Issue opened [#2899](https://github.com/kubeflow/trainer/issues/2899).
- **2026-01-04**: KEP drafted.
- **TBD**: Alpha implementation.

## Drawbacks

- **Potential User Confusion:** Users unfamiliar with TTL may be surprised when TrainJobs disappear.
- **Loss of Job History:** TTL deletion removes TrainJob metadata permanently. Use logging/exporting to mitigate.

## Alternatives
