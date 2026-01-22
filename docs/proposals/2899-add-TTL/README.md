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
    // creation time that the TrainJob may be active before the system tries to terminate
    // it. Value must be a positive integer. Once reached, all running Pods are terminated
    // and the TrainJob status becomes Failed with reason: DeadlineExceeded.
    // This value overrides any default set in the referenced TrainingRuntime.
    // +optional
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:XValidation:rule="self == oldSelf", message="activeDeadlineSeconds is immutable"
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
    - Calculate `deadline = creationTime + effectiveActiveDeadlineSeconds`
    - If exceeded, mark TrainJob as Failed (`Reason: DeadlineExceeded`) and delete JobSet
    - Otherwise, requeue at `deadline`

4. **Clock Skew Handling:**
    - If calculated requeue time is in the past (due to clock skew), requeue with a small delay (e.g., 1 second)

**Webhook Validation** (`pkg/webhooks/trainjob_webhook.go`):

For TrainJob:
- Validate `ActiveDeadlineSeconds > 0` if set
- Make `ActiveDeadlineSeconds` immutable after creation

For TrainingRuntime:
- Validate `TTLSecondsAfterFinished >= 0` if set
- Validate `ActiveDeadlineSeconds > 0` if set
- Warn if `TTLSecondsAfterFinished < 60s`

### Kubeflow SDK Changes

Update the Python SDK to expose `ActiveDeadlineSeconds` for data scientists in `api/python_api`:

```python
@dataclass
class TrainJobSpec:
    runtime_ref: RuntimeRef
    trainer: Optional[Trainer] = None
    initializer: Optional[Initializer] = None
    labels: Optional[Dict[str, str]] = None
    annotations: Optional[Dict[str, str]] = None
    suspend: Optional[bool] = None
    managed_by: Optional[str] = None
    # NEW: Exposed for data scientists
    active_deadline_seconds: Optional[int] = None


def train(
    self,
    name: str,
    func: Optional[Callable] = None,
    runtime_ref: str = "torch-distributed",
    num_nodes: int = 1,
    resources_per_node: Optional[Dict[str, str]] = None,
    active_deadline_seconds: Optional[int] = None,  # NEW parameter
    # ... other parameters
) -> str:
    """Create a TrainJob."""
    # ...
```

> **Note:** `TTLSecondsAfterFinished` is intentionally NOT exposed in the SDK as it is a platform admin policy.

### Interaction with Suspend

If a TrainJob is suspended, the `ActiveDeadlineSeconds` timer continues to count down. This aligns with the behavior of Kubernetes Jobs. Platform admins should account for potential suspend duration when setting default deadlines.

### Metrics

Add Prometheus metrics for observability:

```go
var (
    deadlineExceededTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "trainjob_deadline_exceeded_total",
            Help: "Total number of TrainJobs that exceeded their deadline",
        },
        []string{"namespace"},
    )
    ttlDeletionsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "trainjob_ttl_deletions_total",
            Help: "Total number of TrainJobs deleted due to TTL expiration",
        },
        []string{"namespace"},
    )
)
```

### Test Plan

[x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Unit Tests

- `pkg/controller/trainjob/`: High coverage expected for new logic

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
