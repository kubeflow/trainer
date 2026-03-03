# Debug Session: PR #3258 — KEP-2899 ActiveDeadlineSeconds for TrainJob

**Date**: March 3, 2026  
**Branch**: `ttl`  
**PR**: [kubeflow/trainer#3258](https://github.com/kubeflow/trainer/pull/3258)  
**Author**: XploY04  
**KEP**: KEP-2899 (Resource Timeouts / ActiveDeadlineSeconds)

---

## 1. Feature Overview

PR #3258 implements `ActiveDeadlineSeconds` for `TrainJob`, allowing users to set a maximum duration for training jobs. If a TrainJob exceeds this deadline, the controller marks it as `Failed` with reason `DeadlineExceeded`.

### Files Changed (17 files, +349 / -5)

| File | Change |
|------|--------|
| `pkg/apis/trainer/v1alpha1/trainjob_types.go` | Added `ActiveDeadlineSeconds int64` field with `+kubebuilder:validation:Minimum=1` and immutability CEL rule. Added `TrainJobDeadlineExceededReason` constant. |
| `pkg/apis/trainer/v1alpha1/trainingruntime_types.go` | Added CEL validation on `TrainingRuntimeSpec` blocking `activeDeadlineSeconds` in `replicatedJobs` (must use TrainJobSpec instead). |
| `pkg/controller/trainjob_controller.go` | Added `reconcileDeadline()` method, `if runtime != nil` guard around `setTrainJobStatus`, and deadline reconciliation call. |
| `test/e2e/trainjob_deadline_test.go` | New 61-line e2e test: busybox sleep 600 + `ActiveDeadlineSeconds(10)` with torchRuntime. |
| `manifests/base/crds/trainer.kubeflow.org_trainjobs.yaml` | Added `activeDeadlineSeconds` field with `minimum: 1`, `format: int64`, immutability CEL rule. |
| `manifests/base/crds/trainer.kubeflow.org_clustertrainingruntimes.yaml` | Added CEL validation at spec level (line 10491). |
| `manifests/base/crds/trainer.kubeflow.org_trainingruntimes.yaml` | Same CEL validation as ClusterTrainingRuntimes. |
| Various test/unit files | Corresponding test coverage for deadline logic. |

### Key Design Decisions

- **`int64` not `*int64`**: kube-api-linter enforces `int64` for fields with `Minimum=1` (zero value = disabled).
- **CEL immutability**: `activeDeadlineSeconds` cannot be changed after creation.
- **CEL on runtimes**: `activeDeadlineSeconds` is forbidden inside `replicatedJobs` — must be set on `TrainJobSpec` instead.
- **Zero value = no deadline**: When `ActiveDeadlineSeconds == 0`, `reconcileDeadline()` returns `ctrl.Result{}, nil` (no-op).

---

## 2. The Problem: CI E2E Test Failures

### Symptom

E2E tests fail **only** on PR #3258. All other PRs pass. The failure is consistent across all 4 Kubernetes version matrices (1.32.3, 1.33.1, 1.34.0, 1.35.0).

### Which Test Fails

Only **one** test fails: the **DeepSpeed** distributed training test. The other 4 tests pass:

| Test | Duration | Result |
|------|----------|--------|
| TrainJob deadline exceeded | 9.8s | ✅ PASS |
| PyTorch distributed training | 53.6s | ✅ PASS |
| **DeepSpeed distributed training** | **600s** | **❌ FAIL (timeout)** |
| JAX distributed training | 54.7s | ✅ PASS |
| PodTemplateOverrides | 0.009s | ✅ PASS |

### Key Confirmation from Maintainer

> **jaiakash** (via Slack): The DeepSpeed failure is **only happening on PR #3258**. It is not an image pull or infrastructure issue.

---

## 3. CI Log Analysis

### CI Run Details

- **CI ran on commit**: `19c28e6` (before any fix attempts)
- **Runner**: `oracle-vm-16cpu-64gb-x86-64`
- **Workflow**: `.github/workflows/test-e2e.yaml`

### DeepSpeed Failure Details

The test at `test/e2e/e2e_test.go:159` times out (600s) waiting for the TrainJob to reach `Succeeded` status.

**What happens**:
1. DeepSpeed TrainJob is created ✅
2. Launcher pod becomes Active, then Succeeded (Succeeded=1) ✅
3. **Node worker pod**: `Active: &0`, `Succeeded: &0`, `Failed: &0` ❌
   - The node worker **never becomes Active** or **drops from Active to nothing**
   - It neither succeeds nor fails — it just vanishes

**Error diff from CI**:
```
Expected (launcher): Succeeded=&1 ✅
Expected (node):     Active=&1
Got (node):          Active=&0, Succeeded=&0
```

The launcher completes but the node worker never finishes, causing the test to timeout at 600s.

### Tests That Pass

- **Deadline test**: Creates TrainJob with busybox, sleep 600, `ActiveDeadlineSeconds(10)`. Controller correctly marks it as `Failed` with `DeadlineExceeded` after ~10s.
- **PyTorch**: Simple distributed training, completes in ~53s.
- **JAX**: Image classification, completes in ~54s.
- **PodTemplateOverrides**: Quick validation test.

---

## 4. Investigation Timeline

### Phase 1: Initial Hypothesis — Image Pull Timeouts (WRONG)

Agent initially suspected image pull timeouts causing the DeepSpeed failure. This was debunked by jaiakash's Slack feedback confirming the failure is PR-specific.

### Phase 2: Deep Code Analysis

Agent analyzed all code changes in the PR diff against `upstream/master`:

#### Change 1: `reconcileDeadline()` method
```go
func (r *TrainJobReconciler) reconcileDeadline(ctx context.Context, trainJob *trainer.TrainJob) (ctrl.Result, error) {
    if trainJob.Spec.ActiveDeadlineSeconds == 0 {
        return ctrl.Result{}, nil  // No-op for non-deadline TrainJobs
    }
    // ... deadline logic ...
}
```
**Verdict**: Safe. For non-deadline TrainJobs (like DeepSpeed test), this is a complete no-op.

#### Change 2: `if runtime != nil` guard (SUSPICIOUS)
```go
// Before (upstream):
setTrainJobStatus(trainJob, runtime, jobSet)

// After (PR):
if runtime != nil {
    setTrainJobStatus(trainJob, runtime, jobSet)
}
```
**Verdict**: This guard was added to handle cases where the runtime might be nil, but it could potentially skip status updates that were previously always executed.

#### Change 3: Early return block for finished TrainJobs (REMOVED IN FIX)
The original PR code (commit `19c28e6`) had:
```go
// Return early if the TrainJob has already finished
for _, c := range trainJob.Status.Conditions {
    if (c.Type == trainer.TrainJobSuspended && c.Status == metav1.ConditionTrue) ||
        c.Type == trainer.TrainJobComplete || c.Type == trainer.TrainJobFailed {
        return ctrl.Result{}, nil
    }
}
```
**Verdict**: This was the most suspicious change — it could cause the controller to stop reconciling a TrainJob prematurely if a transient condition was set.

#### Change 4: CEL validation on TrainingRuntimeSpec
```go
// +kubebuilder:validation:XValidation:rule="!has(self.template) || !has(self.template.spec) || !has(self.template.spec.replicatedJobs) || self.template.spec.replicatedJobs.all(rj, !has(rj.template.spec.activeDeadlineSeconds))"
```
**Verdict**: Verified safe. No existing ClusterTrainingRuntimes use `activeDeadlineSeconds`. All 7 runtimes apply cleanly with `kubectl apply --server-side --dry-run=server`.

### Phase 3: Local Testing on Kind Cluster

```
Kind cluster: k8s v1.32.0
```

| Test | Result | Notes |
|------|--------|-------|
| Deadline test | ✅ PASS (9.8s) | Controller correctly marks Failed/DeadlineExceeded |
| PyTorch | ✅ PASS (3.6s) | Works fine |
| JAX | ✅ PASS (4.7s) | Works fine |
| PodTemplateOverrides | ✅ PASS (0.05s) | Works fine |
| **DeepSpeed** | **❌ FAIL** | **Image pull failure** (local Docker doesn't have deepspeed image) |

Local testing could not validate the DeepSpeed fix because the image isn't available locally.

### Phase 4: Fix Attempt

#### Commit `1251a0cc` — "Remove early return for finished TrainJobs"

Two changes:
1. **Removed the early return block** that could cause premature reconciliation exit
2. **Removed unused `jobsetv1alpha2` import**

**Rationale**: The early return block was the most likely culprit. If a TrainJob temporarily had a condition set (e.g., during status updates), the early return could prevent subsequent reconciliation loops from progressing the job to completion.

**Honest Assessment**: This fix changed zero local test outcomes (same 4 pass, DeepSpeed still fails on image pull locally). The fix cannot be validated locally — it needs to be pushed to CI.

---

## 5. Current State of the Code

### HEAD: Commit `1251a0cc` (after fix)

**Remaining diffs from `upstream/master` in `trainjob_controller.go`:**

1. **`if runtime != nil` guard** around `setTrainJobStatus` — still present
2. **`reconcileDeadline()` call** with early return on `RequeueAfter` — core feature
3. **`reconcileDeadline()` function** — core feature implementation
4. **`removeFailedCondition()` helper** — used by deadline logic

### What CI Ran (commit `19c28e6`)

CI ran the code **before** the fix, which included the early return block for finished TrainJobs. The fix at `1251a0cc` has **not yet been pushed** to CI.

---

## 6. Verification Commands Run

```bash
# CRD dry-run — PASSED
kubectl apply --server-side -k manifests/base/crds --dry-run=server

# Runtime manifests — ALL 7 APPLIED CLEANLY
kubectl apply --server-side -k manifests/overlays/runtimes --dry-run=server

# No runtimes use activeDeadlineSeconds
grep activeDeadlineSeconds manifests/base/runtimes/*.yaml  # No matches

# Confirm CI ran before fix
git log --oneline 19c28e6..HEAD
# 1251a0cc Remove early return for finished TrainJobs (FIX - not in CI yet)

# Show code CI ran (the early return block)
git show 19c28e6:pkg/controller/trainjob_controller.go | sed -n '105,130p'
```

---

## 7. Open Questions & Theories

### Theory A: Early Return Block (Most Likely)

The early return block (removed in fix) could interfere with DeepSpeed specifically because:
- DeepSpeed has a **launcher + node** two-job pattern
- The launcher completes first (Succeeded=1)
- If the controller sees a condition change during the launcher→node transition and returns early, the node worker may never be properly managed
- PyTorch/JAX don't have this two-job pattern (or handle it differently)

### Theory B: `if runtime != nil` Guard

The `if runtime != nil` guard could skip status updates in edge cases where the runtime lookup fails or returns nil. This would affect all runtimes equally though, so it's less likely to be DeepSpeed-specific.

### Theory C: Timing/Race Condition

The deadline reconciliation adds a new `reconcileDeadline()` call that returns early with `RequeueAfter`. Even though for non-deadline TrainJobs it's a no-op, the additional reconciliation path could introduce timing differences that expose a latent race condition in the DeepSpeed flow.

---

## 8. Next Steps

1. **Push commit `1251a0cc`** to the `ttl` branch and let CI run
2. **If DeepSpeed still fails**: Investigate the `if runtime != nil` guard and timing theories
3. **If DeepSpeed passes**: The early return block was the root cause; document it
4. **Consider adding**: DeepSpeed-specific unit test for the deadline + multi-job interaction

---

## 9. Key File Locations

| Purpose | Path |
|---------|------|
| TrainJob controller | `pkg/controller/trainjob_controller.go` |
| TrainJob API types | `pkg/apis/trainer/v1alpha1/trainjob_types.go` |
| TrainingRuntime types | `pkg/apis/trainer/v1alpha1/trainingruntime_types.go` |
| Deadline e2e test | `test/e2e/trainjob_deadline_test.go` |
| Main e2e tests (DeepSpeed) | `test/e2e/e2e_test.go` |
| TrainJob CRD manifest | `manifests/base/crds/trainer.kubeflow.org_trainjobs.yaml` |
| ClusterTrainingRuntime CRD | `manifests/base/crds/trainer.kubeflow.org_clustertrainingruntimes.yaml` |
| DeepSpeed runtime manifest | `manifests/base/runtimes/deepspeed-distributed.yaml` |
| E2E test workflow | `.github/workflows/test-e2e.yaml` |

---

## 10. Git History (PR Branch)

```
1251a0cc Remove early return for finished TrainJobs  ← FIX (not yet pushed to CI)
19c28e6  <previous commit>                           ← What CI ran (FAILED)
...      <earlier PR commits>
```

## 11. Environment Details

| Component | Version |
|-----------|---------|
| Go | 1.25 |
| controller-runtime | v0.23.1 |
| Kind (local) | v0.31.0 |
| k8s (local) | v1.32.0 |
| k8s (CI matrix) | 1.32.3, 1.33.1, 1.34.0, 1.35.0 |
| CI Runner | oracle-vm-16cpu-64gb-x86-64 |
| Ginkgo | v2 (e2e test framework) |
| Remote origin | XploY04/trainer |
| Remote upstream | kubeflow/trainer |
