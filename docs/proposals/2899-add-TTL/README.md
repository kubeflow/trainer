# Summary

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

1. Add TTL reconciliation logic:
    - Check if job is finished and TTL is set
    - Calculate deleteTime = finishTime + TTL
    - If expired, delete TrainJob (cascades to owned resources)
    - Otherwise, requeue at deleteTime
2. Add deadline enforcement logic:
    - Check if job is running and deadline is set
    - Calculate deadline = creationTime + ActiveDeadlineSeconds
    - If exceeded, mark TrainJob as Failed and delete JobSet
    - Otherwise, requeue at deadline
3. Integration: Add both reconcilers to main Reconcile() loop after status sync

**Key Implementation Notes:**

- Use `condition.LastTransitionTime` of Complete/Failed condition for finish time
- Use `TrainJob.CreationTimestamp` for deadline calculation
- Use `ctrl.Result{RequeueAfter: duration}` for precise timing
- Deletion respects finalizers and cascades via OwnerReference

**Webhook Validation** (`pkg/webhooks/trainjob_webhook.go`):

- Validate TTL ≥ 0
- Validate deadline > 0
- Warn if TTL < 60s (might lose job before review)
- Prevent modification after creation (fields are immutable)

**CRD Generation:**

```bash
make generate   # Generate Go code
make manifests  # Generate CRDs in manifests/base/crds/
```

## Design Details

### Test Plan

**Unit Tests** (`pkg/controller/trainjob/trainjob_controller_test.go`):

- TTL not set → no deletion
- TTL expired → job deleted
- TTL not expired → requeue at correct time
- TTL = 0 → immediate deletion
- Deadline exceeded → job failed, pods terminated
- Deadline not reached → requeue at deadline
- Both fields set → correct interaction

**Integration Tests** (`test/integration/controller/trainjob_controller_test.go`):

- End-to-end TTL deletion workflow
- End-to-end deadline enforcement
- Cascade deletion of owned resources
- Controller restart (timers resume correctly)

**E2E Tests** (`test/e2e/trainjob_ttl_test.go`):

- Real training workload with TTL
- Real training workload with deadline
- Verify no orphaned resources

### Upgrade / Downgrade Strategy

**Upgrade:** No action needed - new fields are optional, existing TrainJobs unaffected

**Downgrade:** Remove TTL/deadline fields from TrainJobs before downgrading controller:

```bash
kubectl get trainjobs -A -o json | \
  jq 'del(.items[].spec.ttlSecondsAfterFinished, .items[].spec.activeDeadlineSeconds)' | \
  kubectl apply -f -
```

## Production Readiness

### Feature Enablement

**Mechanism:** Fields are part of API (optional to use). No feature gate needed.

- Enable: Users add fields to TrainJob spec
- Disable: Users don't set fields (nil)

### Monitoring

**Recommended Metrics:**

- `trainjob_ttl_deletions_total` - Count of TTL-triggered deletions
- `trainjob_deadline_exceeded_total` - Count of deadline terminations
- `trainjob_ttl_deletion_latency_seconds` - Time from expiration to deletion

## Graduation Criteria

### Alpha

- Feature implemented behind optional API fields (no feature gate required)
- Unit tests for TTL and deadline logic with >80% coverage
- Integration tests for controller behavior
- Basic documentation in KEP and code comments
- Manual testing on a development cluster

### Beta

- E2E tests passing in CI
- Metrics implemented and exposed via Prometheus
- Documentation published on kubeflow.org
- At least 2 production users providing feedback
- No critical bugs reported for 1 release cycle
- Stress testing with 100+ concurrent TrainJobs with TTL/deadline

### GA

- Conformance tests added
- Feature stable for 2+ release cycles
- Wide adoption confirmed via community feedback
- Performance benchmarks documented
- Upgrade/downgrade testing automated

---

## Version Skew Strategy

### Controller ↔ CRD Version Skew

- **New controller, old CRD**: Controller gracefully handles missing TTL/deadline fields (nil check)
- **Old controller, new CRD**: Old controller ignores unknown fields; TrainJobs with TTL/deadline behave as if fields are unset

### Controller ↔ JobSet Version Skew

- **Minimum JobSet version**: v0.10.0 (current dependency)
- **Behavior**: TTL/deadline are TrainJob-level features, independent of JobSet TTL
- JobSet's own `ttlSecondsAfterFinished` can coexist but TrainJob controller manages TrainJob resource lifecycle

### Controller ↔ Kubernetes Version Skew

- **Minimum Kubernetes version**: 1.29+ (aligns with client-go v0.34.x)
- Uses stable APIs: `metav1.Condition`, `ctrl.Result{RequeueAfter}`

---

## Production Readiness Review Questionnaire

### Feature Enablement and Rollback

**How can this feature be enabled / disabled in a live cluster?**

- **Enable**: Set `ttlSecondsAfterFinished` and/or `activeDeadlineSeconds` on TrainJob spec
- **Disable**: Omit these fields (nil/unset) - TrainJobs behave as before
- No feature gate required; fields are optional API additions

**Does enabling the feature change any default behavior?**

No. Existing TrainJobs without these fields are unaffected.

**Can the feature be disabled once it has been enabled?**

Yes. Remove the fields from TrainJob specs. Already-scheduled deletions will complete, but new TrainJobs won't have TTL/deadline behavior.

**What happens if we reenable the feature?**

No special handling needed. New TrainJobs with fields set will behave normally.

**Are there any tests for feature enablement/disablement?**

Yes, unit tests cover:
- TrainJob with nil TTL/deadline (no-op)
- TrainJob with TTL set then unset via update (immutable, blocked by webhook)

---

### Rollout, Upgrade and Rollback Planning

**How can a rollout or rollback fail? Can it impact already running workloads?**

- **Rollout**: If webhook validation is misconfigured, TrainJob creation may be blocked
- **Running workloads**: Unaffected; TTL/deadline only apply to newly reconciled states
- **Rollback**: If controller is downgraded, fields are ignored; no automatic cleanup occurs

**What specific metrics should inform rollback?**

- `trainjob_ttl_deletions_total` increasing unexpectedly
- `trainjob_deadline_exceeded_total` showing false positives
- Controller error rate in logs

**Were upgrade and rollback tested?**

To be tested as part of Beta graduation:
- Upgrade: Deploy new controller, verify existing TrainJobs unaffected
- Rollback: Downgrade controller, verify no panics on TrainJobs with TTL/deadline

**Is the rollout accompanied by any deprecations and/or removals?**

No.

---

### Monitoring Requirements

**How can an operator determine if the feature is in use?**

- Check TrainJob specs for `ttlSecondsAfterFinished` or `activeDeadlineSeconds` fields
- Query metrics: `trainjob_ttl_deletions_total > 0` or `trainjob_deadline_exceeded_total > 0`

**How can someone using this feature know that it is working?**

- TrainJobs are automatically deleted after TTL expires (observable via `kubectl get trainjobs`)
- TrainJobs fail with `DeadlineExceeded` reason when deadline is hit
- Metrics increment as expected

**What are the reasonable SLOs (Service Level Objectives)?**

- TTL deletion latency: <30 seconds from expiration to deletion (p99)
- Deadline enforcement accuracy: ±5 seconds of specified deadline

**What are the SLIs (Service Level Indicators)?**

- `trainjob_ttl_deletion_latency_seconds` histogram
- `trainjob_deadline_exceeded_total` counter
- Controller reconciliation error rate

**Are there any missing metrics that would be useful?**

Recommended additions:
- `trainjob_ttl_pending_deletions` gauge (TrainJobs waiting for TTL expiry)
- `trainjob_active_deadline_remaining_seconds` gauge (time until deadline per running TrainJob)

---

### Dependencies

**Does this feature depend on any specific services running in the cluster?**

No new dependencies. Uses existing:
- Kubernetes API server (for CRUD operations)
- controller-runtime (for reconciliation and requeue)

**Dependency Versions:**

| Dependency | Minimum Version | Notes |
|------------|-----------------|-------|
| Kubernetes | 1.29+ | Client-go v0.34.x compatibility |
| JobSet | v0.10.0+ | Current project dependency |
| controller-runtime | v0.22.0+ | For RequeueAfter support |
| Go | 1.24+ | Required by go.mod |

---

### Scalability

**Will enabling / using this feature result in any new API calls?**

- **Additional GET**: None (uses existing reconcile data)
- **Additional DELETE**: 1 per TrainJob when TTL expires
- **Additional PATCH**: 1 per TrainJob when deadline is exceeded (status update)

**Will enabling / using this feature result in increasing size or count of resources?**

No. Feature reduces resource count by deleting finished TrainJobs.

**Will enabling / using this feature result in increasing time taken by any operations?**

- Reconcile loop: +O(1) time per TrainJob (simple time comparison)
- No significant impact expected

**Can enabling / using this feature result in resource exhaustion?**

- **RequeueAfter timers**: Managed by controller-runtime; no memory leak risk
- **Concurrent deletions**: Bounded by controller concurrency settings

**What are the scaling limits?**

Tested/expected to work with:
- 1000+ TrainJobs with TTL in a single namespace
- 5000+ TrainJobs cluster-wide
- Deletion rate: 100 TrainJobs/minute without degradation

---

### Troubleshooting

**How does this feature react if the API server and/or etcd is unavailable?**

- TTL deletions are delayed until API server is available
- RequeueAfter is respected; no data loss
- Deadline enforcement may be delayed (TrainJob continues running)

**What are other known failure modes?**

| Failure Mode | Detection | Mitigation |
|--------------|-----------|------------|
| TTL deletion fails | Events on TrainJob, controller logs | Check RBAC permissions for delete |
| Deadline not enforced | TrainJob runs past deadline | Check controller logs, verify clock sync |
| Premature deletion | TrainJob deleted before expected | Verify TTL value, check for clock skew |

**What steps should be taken if SLOs are not being met?**

1. Check controller pod logs for errors
2. Verify controller has delete permissions: `kubectl auth can-i delete trainjobs`
3. Check for high reconcile queue depth
4. Increase controller replicas or concurrency if needed

---

## Implementation History

| Date | Milestone | Details |
|------|-----------|---------|
| 2025-10-20 | Issue opened | [#2899](https://github.com/kubeflow/trainer/issues/2899) - TTL for TrainJobs |
| 2026-01-04 | KEP drafted | Initial KEP-2899 document created |
| TBD | Alpha implementation | API fields added, controller logic implemented |
| TBD | Beta | E2E tests, metrics, documentation |
| TBD | GA | Stable release |

---

## Drawbacks

### Potential User Confusion

- Users unfamiliar with TTL may be surprised when TrainJobs disappear
- **Mitigation**: Clear documentation, webhook warning for TTL < 60s

### Loss of Job History

- TTL deletion removes TrainJob metadata permanently
- **Mitigation**: 
  - Recommend users export logs/metrics before TTL expiry
  - Consider future integration with TrainJob History Server (#2648)

### Clock Skew Sensitivity

- TTL and deadline rely on accurate cluster time
- Significant clock skew between controller and API server could cause timing issues
- **Mitigation**: Recommend NTP synchronization; use server-side timestamps

### No Pause/Resume for Deadline

- Once deadline is set, it cannot be extended
- Long-running jobs that legitimately need more time will fail
- **Mitigation**: Document clearly; users should set generous deadlines or not use the feature

### Interaction with Suspend

- If a TrainJob is suspended, should the deadline timer pause?
- **Current design**: Timer continues during suspend (matches Kubernetes Job behavior)
- **Alternative considered**: Pause timer during suspend (more complex, less consistent with K8s)

---

## References

- [Issue #2899](https://github.com/kubeflow/trainer/issues/2899)
- [Kubernetes Job TTL](https://kubernetes.io/docs/concepts/workloads/controllers/job/#ttl-mechanism-for-finished-jobs)
- [Kubernetes Job ActiveDeadlineSeconds](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup)
- [JobSet API](https://github.com/kubernetes-sigs/jobset)
- [KEP Template](https://github.com/kubernetes/enhancements/tree/master/keps/NNNN-kep-template)