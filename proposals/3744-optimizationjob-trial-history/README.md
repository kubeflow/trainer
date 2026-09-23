# KEP-3744: Store Trial History Natively in OptimizationJob Status

Authors:

- Sridhar Pillai (@Sridhar1030)

Related:

- Parent KEP: [KEP-3562: OptimizationJob CRD](../2605-optimization-job-crd/README.md) (in the `2605-optimization-job-crd` directory, tracked by [#3562](https://github.com/kubeflow/trainer/issues/3562))
- Tracking issue: [kubeflow/trainer#3744](https://github.com/kubeflow/trainer/issues/3744)

## Summary

In the Phase 1 design of [KEP-3562](../2605-optimization-job-crd/README.md), the `OptimizationJob` controller reconstructs trial history by listing child `TrainJob` objects and reading back their injected parameters and terminal metrics. This makes completed `TrainJob` objects load-bearing for the optimization itself. If a trial `TrainJob` is deleted (manually, by a cluster cleanup policy, or by the TTL-based lifecycle direction the Trainer project is moving toward), the experiment history is silently lost. The suggestion service, which rebuilds its state from that history on every `GetSuggestions` call, can then re-propose already-explored points or skew the sampler.

This KEP proposes making the `OptimizationJob` self-contained by recording a strongly-typed trial history in `OptimizationJob.status.trials`. The controller writes one record per trial when it creates the trial `TrainJob` (the parameters are known at that moment, because the controller generated them) and patches that record exactly once when the trial reaches a terminal state. Suggestion snapshots are then assembled entirely from `status.trials`. The optimization state lives on the `OptimizationJob` itself, nothing is ever read back from a child that might have been deleted, and the controller remains stateless: all state stays in the Kubernetes API server, preserving the stateless-provider architecture of KEP-3562.

## Motivation

The problem surfaced during API review of the `OptimizationJob` CRD ([#3565, discussion](https://github.com/kubeflow/trainer/pull/3565#discussion_r3582000633)): reconstructing trial history from `TrainJob` objects makes the `OptimizationJob` less self-contained and implicitly assumes those jobs are never garbage collected. `TrainJob` objects are execution resources with an independent lifecycle, and the `OptimizationJob` should own the optimization state itself. Storing the list of all trials (hyperparameters and scores) in status was raised as the more robust design, and deferred to a future iteration tracked by [#3744](https://github.com/kubeflow/trainer/issues/3744).

The deletion pressure on finished trials is real and growing:

- Finished trial `TrainJob` objects can be deleted manually or by cluster-level cleanup tooling today, and the underlying JobSet API already supports `ttlSecondsAfterFinished`.
- [KEP-2899](../2899-resource-timeouts/README.md) exists because finished jobs accumulate and bloat etcd. Its initial release ships `ActiveDeadlineSeconds`, and it explicitly lists `TTLSecondsAfterFinished` on the TrainJob API as a future plan. When that lands, automatic deletion of finished trials becomes a first-class, supported behavior.
- A 100-trial `OptimizationJob` produces on the order of 100 finished `TrainJob` objects (plus their JobSets), which is exactly the accumulation that cleanup policies target.

Today, deleting any completed trial `TrainJob` corrupts a running optimization. The controller rebuilds an incomplete history, and the stateless suggestion provider silently degrades.

Storing history natively in status also improves observability and unblocks SDK work:

- Users can see every trial's parameters and objective values with `kubectl get optimizationjob -o yaml` instead of joining across (possibly deleted) child resources.
- The SDK `OptimizerClient` integration ([#3794](https://github.com/kubeflow/trainer/issues/3794)) can implement `get_job(...).trials` and `get_best_results(...)` from a single object read.
- Hyperparameters become strongly typed in the API (reusing `ParameterAssignment`), rather than reconstructed from the child's injected environment variables.

### Goals

- Record each trial (parameters, objective metric values, state, timestamps) in `OptimizationJob.status.trials`.
- Make `status.trials` the canonical history used to build `GetSuggestions` snapshots, removing the dependency on retained `TrainJob` objects entirely.
- Preserve complete history when trial `TrainJob` objects are deleted at any point in their lifecycle, whether manually or via TTL/GC policies.
- Keep the controller stateless and the status write path idempotent, bounded, and low-churn.

### Non-Goals

- **Intermediate (per-epoch) metric time series in status.** Rich metric history remains the domain of external trackers (e.g. MLflow, TensorBoard), consistent with the non-goals of [KEP-2779](../2779-trainjob-progress/README.md). Status records terminal objective values only.
- **Trial suspension/resume and storage checkpointing.** Tracked separately in the KEP-3562 Phase 2 roadmap.
- **Changing the Phase 1 parameter-injection mechanism.** `KUBEFLOW_TRAINER_OPT_<NAME>` environment variable injection remains as designed; the child `TrainJob` simply stops being the durable record of the assignments. (KEP-3562 §8.2 also describes a parameter annotation, which the current implementation does not write; this KEP makes that annotation unnecessary rather than required.)
- **Changing the gRPC contract.** The snapshot format passed to the suggestion service is unchanged; only its source changes. The contract refactor is a separate Phase 2 item ([#3796](https://github.com/kubeflow/trainer/issues/3796)).
- **Trial failure policy.** How many failures an `OptimizationJob` tolerates before failing is a separate design discussion from KEP-3562; this KEP only ensures failures are durably recorded with a reason.

## Proposal

Add an optional `trials` list to `OptimizationJobStatus`, owned exclusively by the controller. Each trial's record is created together with its `TrainJob` and finalized exactly once at terminal state. Because the record is written *before* the child ever runs, no information is ever recoverable only from the child: deleting a trial `TrainJob` at any point, running or finished, can no longer lose history. No finalizers on child resources are required (see [Alternatives](#alternatives) for the rejected finalizer design).

### User Stories

**Story 1: Platform operator enabling cleanup of finished trials**

- **As a Platform Operator**, I want finished trial `TrainJob` objects to be deletable without corrupting running optimizations, whether deleted manually today or automatically once TTL cleanup lands per the KEP-2899 future plan.
- **Motivation:** A single `OptimizationJob` can leave up to 100 finished `TrainJob` objects (and their JobSets) in etcd. With native history, they can be cleaned up as soon as they finish, while `status.result` and `status.trials` remain intact.

**Story 2: Data scientist auditing an experiment**

- **As a Data Scientist**, I want to inspect a finished `OptimizationJob` and see every trial's hyperparameters, objective value, and outcome (including *why* a trial failed) in one place.
- **Motivation:** Avoid querying (possibly deleted) `TrainJob` objects or standing up an external experiment tracker just to answer what a given trial evaluated and what it scored.

```yaml
status:
  conditions:
    - type: "Complete"
      status: "True"
      reason: "MaxTrialsReached"
  result:
    trainJobName: "random-tuning-mvp-trial-ab12c"
    parameters:
      - name: "learning_rate"
        value: "0.0021"
      - name: "batch_size"
        value: "32"
  trials:
    - trainJobName: "random-tuning-mvp-trial-ab12c"
      state: "Succeeded"
      creationTime: "2026-08-05T10:12:00Z"
      completionTime: "2026-08-05T10:41:00Z"
      parameters:
        - name: "learning_rate"
          value: "0.0021"
        - name: "batch_size"
          value: "32"
      metrics:
        - name: "val_loss"
          value: "0.182"
    - trainJobName: "random-tuning-mvp-trial-cd34e"
      state: "Failed"
      reason: "MetricsUnavailable"
      creationTime: "2026-08-05T10:12:00Z"
      completionTime: "2026-08-05T10:39:00Z"
      parameters:
        - name: "learning_rate"
          value: "0.0899"
        - name: "batch_size"
          value: "16"
```

**Story 3: SDK listing trials**

- **As an ML Researcher using the Kubeflow SDK**, I want `OptimizerClient.get_job(name).trials` and `get_best_results(...)` to work from a single `OptimizationJob` read ([#3794](https://github.com/kubeflow/trainer/issues/3794)).
- **Motivation:** No list-and-parse of child resources in the SDK, and no breakage when children have been cleaned up.

## Design Details

### Prerequisites

- **TrainJobStatus feature gate (hard dependency for metrics).** Objective metric values are read from `TrainJob.status.trainerStatus.metrics`, introduced by [KEP-2779](../2779-trainjob-progress/README.md) behind the `TrainJobStatus` feature gate (alpha, default off). The parent KEP-3562 already carries this dependency; with the gate off, trials complete without reported metrics and are recorded as such (see [controller semantics](#controller-semantics)).

### API

Only the new field and types are shown; `conditions` and `result` on `OptimizationJobStatus` are unchanged from KEP-3562. `status.result` is retained as a convenience projection of `trials` (the best terminal trial), so there is no breaking change to the existing status API.

```go
type OptimizationJobStatus struct {
	// ... existing fields (conditions, result) ...

	// trials is the history of all trials launched by this OptimizationJob,
	// owned by the controller and used as the canonical suggestion snapshot.
	// Each record is created with its trial TrainJob and updated exactly once
	// when the trial reaches a terminal state.
	// Interim budget rule (until the trial failure policy lands in #3743):
	// every record counts toward numTrials regardless of its state, so the
	// cap equals the maximum allowed spec.numTrials; raise the two together.
	// +listType=map
	// +listMapKey=trainJobName
	// +kubebuilder:validation:MaxItems=100
	// +optional
	Trials []TrialResult `json:"trials,omitempty"`

	// nextTrialIndex is the monotonically increasing index the controller
	// assigns to the next trial. It is persisted (rather than derived from
	// the record count) so trial naming stays idempotent when a reconcile
	// reads a stale cache.
	// +kubebuilder:validation:Minimum=0
	// +optional
	NextTrialIndex *int32 `json:"nextTrialIndex,omitempty"`
}

// +kubebuilder:validation:Enum=Running;Succeeded;Failed;EarlyStopped
type TrialState string

const (
	TrialStateRunning      TrialState = "Running"
	TrialStateSucceeded    TrialState = "Succeeded"
	TrialStateFailed       TrialState = "Failed"
	// TrialStateEarlyStopped marks a trial pruned by an early-stopping
	// algorithm (#3802). Kept distinct from Failed so samplers can treat
	// pruned trials as evaluated-and-stopped rather than crashed, aligned
	// with the EARLY_STOPPED state in the #3950 gRPC contract.
	TrialStateEarlyStopped TrialState = "EarlyStopped"
)

const (
	// TrialReasonTrainJobFailed indicates the trial TrainJob reached the Failed condition.
	TrialReasonTrainJobFailed string = "TrainJobFailed"
	// TrialReasonTrainJobDeleted indicates the trial TrainJob was deleted before completing.
	TrialReasonTrainJobDeleted string = "TrainJobDeleted"
	// TrialReasonMetricsUnavailable indicates the trial TrainJob completed without
	// reporting the objective metric (e.g. the TrainJobStatus feature gate is
	// disabled, or the training code never reported it).
	TrialReasonMetricsUnavailable string = "MetricsUnavailable"
)

// TrialResult records a single trial launched by the OptimizationJob.
// +kubebuilder:validation:XValidation:rule="self.state != 'Succeeded' || (has(self.metrics) && self.metrics.size() > 0)",message="a Succeeded trial must record at least one metric"
// +kubebuilder:validation:XValidation:rule="self.state == 'Running' || has(self.completionTime)",message="completionTime is required for terminal trials"
type TrialResult struct {
	// trainJobName is the name of the trial TrainJob this record belongs to.
	// Bounded at 63 to match TrainJob name validation (RFC 1035) and the
	// existing status.result.trainJobName limit.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +required
	TrainJobName string `json:"trainJobName"`

	// parameters are the hyperparameter assignments evaluated by this trial.
	// The list is atomic: assignments are written once at record creation and
	// never mutated, and atomic listType roughly halves the managedFields
	// overhead a per-key map incurs for every record.
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +required
	Parameters []ParameterAssignment `json:"parameters"`

	// metrics are the objective metric values observed at terminal state,
	// copied from the trial TrainJob's status.trainerStatus.metrics.
	// The list is atomic, matching TrainJob.status.trainerStatus.metrics.
	// It is bounded above the current single-objective cap to leave headroom
	// for multi-objective support without a schema change.
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=8
	// +optional
	Metrics []Metric `json:"metrics,omitempty"`

	// state is the observed state of the trial.
	// +required
	State TrialState `json:"state"`

	// reason is a machine-readable explanation for a Failed state,
	// e.g. TrainJobFailed, TrainJobDeleted, MetricsUnavailable.
	// +kubebuilder:validation:MaxLength=128
	// +optional
	Reason *string `json:"reason,omitempty"`

	// creationTime is when the trial TrainJob was created.
	// +required
	CreationTime metav1.Time `json:"creationTime"`

	// completionTime is when the trial reached a terminal state, taken from
	// the lastTransitionTime of the trial TrainJob's terminal condition.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}
```

Notes:

- `ParameterAssignment` is the existing type from the KEP-3562 API, the same one used by `status.result`.
- `Metric` is the existing `{name, value string}` type introduced by KEP-2779 for `TrainJob.status.trainerStatus.metrics`. Reusing it keeps values consistent end to end: they are copied verbatim at terminal state.
- `Metrics` is a list so that multi-objective optimization ([#3799](https://github.com/kubeflow/trainer/issues/3799)) needs no structural change; Phase 1 populates only the single configured objective metric.
- The `Running`/`Succeeded`/`Failed`/`EarlyStopped` states mirror the trial lifecycle, matching the trial states in the [#3950](https://github.com/kubeflow/trainer/pull/3950) gRPC contract so a pruned trial is never conflated with a crashed one; `Reason` carries the failure taxonomy that the KEP-3562 review identified as important (distinguishing bad-hyperparameter failures from infrastructure failures and from misconfiguration). Notably, a trial that *succeeded* as a workload but reported no objective metric is recorded as `Failed` with `Reason: MetricsUnavailable`: suggestion providers must exclude it from sampler input either way, and the reason makes the misconfiguration visible and countable rather than silently discarded.

### Controller semantics

1. **On generating a suggestion:** the controller takes the next trial index from `status.nextTrialIndex` (persisted, so a stale-cache reconcile cannot reuse an index and silently overwrite an existing record), appends a `TrialResult` with `State: Running`, the assigned `Parameters`, and `CreationTime`, keyed by the generated trial `TrainJob` name, increments `nextTrialIndex` in the same write, and then creates the trial `TrainJob`. The append happens first, so the parameters are durable before the child exists; a crash between the two converges on the next reconcile by marking the childless record `Failed`/`TrainJobDeleted` (see the reconcile flow for why the controller never re-creates instead).
2. **On observing a terminal child:** the controller patches the record once, setting `State`, `Metrics` (from `status.trainerStatus.metrics`), `Reason` if failed, and `CompletionTime` (from the terminal condition's `lastTransitionTime`), and updates `status.result` if this trial improves the objective. A record that is already terminal is never modified again.
3. **On observing a deleted or deleting child whose record is still `Running`:** the record is patched to `Failed` with `Reason: TrainJobDeleted`. No history is lost, because the parameters were recorded at creation.
4. **Suggestion snapshots:** the history passed to `GetSuggestions` is assembled from `status.trials` (terminal records as completed trials, `Running` records as in-flight trials), replacing the reconstruction from child `TrainJob` objects. Two caveats. First, the Phase 1 pinned Katib Optuna image drops `RUNNING` and `FAILED` trials before the sampler sees them and tracks in-flight suggestions only in process memory, so the in-flight and restart-safety benefits on the *provider* side materialize only with the refactored gRPC contract ([#3796](https://github.com/kubeflow/trainer/issues/3796) / [#3950](https://github.com/kubeflow/trainer/pull/3950)); what this KEP guarantees on its own is that the *controller's* snapshot is complete and deletion-proof. Second, per-trial intermediate metric series for pruning decisions ([#3802](https://github.com/kubeflow/trainer/issues/3802)) stay out of status (see Non-Goals), so pruning reads live child `trainerStatus` in addition to `status.trials`.
5. **History and size bound:** before launching a trial, the controller checks the trial budget (see point 7) and additionally estimates the serialized size of `status.trials` before every append; the schema cap counts records, not bytes, so the byte check is what actually protects the etcd object limit when parameter counts are large. If either bound is hit, the controller stops launching trials and sets a `Failed` condition with reason `TrialHistoryExhausted`: an explicit, observable outcome rather than a rejected status write wedging the reconcile loop.
6. **Status writes:** `status.trials` and `status.nextTrialIndex` are written with Server-Side Apply under a dedicated field manager. Because an apply expresses the manager's full intent, every apply carries the complete `trials` list, and it sets `metadata.resourceVersion` as an optimistic-concurrency precondition so an apply built from a stale cache fails with a conflict instead of silently dropping records. `conditions` and `result` stay on the controller's existing status write path (merge-patch in [#3828](https://github.com/kubeflow/trainer/pull/3828)) to avoid cross-manager conflicts between SSA and merge writers on the same fields. Writes are bounded at two per trial; on conflict, the next reconcile re-reads and retries, and since records are keyed by `trainJobName` and terminal patches are idempotent, retries converge.
7. **Interim trial budget (until [#3743](https://github.com/kubeflow/trainer/issues/3743) defines the real failure policy):** every record counts toward `numTrials` regardless of state. This keeps the record cap equal to the `numTrials` maximum, bounds the object size, and prevents runaway trial creation when every trial is recorded `Failed`/`MetricsUnavailable` (for example, with the `TrainJobStatus` feature gate off). The full failure policy (retries, failure thresholds) supersedes this rule when it lands.

### Deletion and lifecycle interactions

- **Deleting a trial `TrainJob`** (any state): history is already in status; the child deletes cleanly. No finalizers are involved, so nothing can wedge in `Terminating`.
- **Deleting the `OptimizationJob`:** child `TrainJob` objects are garbage collected through the existing owner references, exactly as in Phase 1. Status dies with the object, as expected.
- **TTL/GC cleanup of finished trials:** fully supported. A finished trial's record is complete before the child becomes deletable.

### Implementation plan

#### Reconcile flow

One pass of the `OptimizationJob` reconcile loop under this design:

1. List child `TrainJob` objects (informer cache) and load `status.trials`.
2. For every record in `status.trials` with `State: Running`:
   - Child terminal: stage the terminal patch (state, metrics, reason, completion time).
   - Child missing or has a deletion timestamp: stage a `Failed`/`TrainJobDeleted` patch, unless an in-flight creation expectation exists for that name (see below).

   A `Running` record with no child is ambiguous from the API server alone: a crash between record append and child creation looks identical to a user deleting the child. The controller resolves this with the in-memory expectations pattern used by the upstream Job controller: a staged creation registers an expectation, and a missing child is only tolerated while its expectation is pending. After a controller restart the expectations are empty, so a missing child is uniformly treated as deleted and the record is marked `Failed`/`TrainJobDeleted`. The controller therefore never resurrects a trial the user deliberately removed; the cost is that a crash inside the append-to-create window converts that one suggestion into a failed record instead of retrying it, which is safe and rare.
3. If capacity allows (`parallelTrials` not saturated, `numTrials` not reached, `trials` under its cap): call `GetSuggestions` with the snapshot assembled from `status.trials`, stage new `Running` records for the returned assignments, then create the corresponding `TrainJob` objects only after the status write succeeds.
4. Apply all staged `status.trials` changes in a single SSA patch: field manager `optimizationjob-trial-history`, the complete `trials` list (an apply is the manager's full intent, so a partial list would delete the omitted records), and `metadata.resourceVersion` set so a stale-cache apply gets a conflict instead of silently replacing records. `status.result` and conditions are updated through the controller's existing merge-patch status writer, not this apply.

Step 4 writing before step 3's child creation preserves the record-before-child invariant; every other ordering risk collapses into "re-reconcile and converge" because all patches are keyed by `trainJobName` and idempotent.

#### Snapshot mapping to the gRPC adapter

Phase 1 keeps the Katib `api.v1.beta1` contract via the adapter in the controller (parent KEP §7.2). The only change is the adapter's input source:

| gRPC field (per past trial) | Phase 1 source ([#3828](https://github.com/kubeflow/trainer/pull/3828), from children) | This KEP (status) |
|---|---|---|
| `Trial.name` | child `TrainJob` name | `TrialResult.TrainJobName` |
| `Trial.spec.parameter_assignments` | `KUBEFLOW_TRAINER_OPT_<NAME>` env vars read back from the child spec | `TrialResult.Parameters` |
| `Trial.status.observation.metrics` | child `status.trainerStatus.metrics` | `TrialResult.Metrics` |
| `Trial.status.condition` | child conditions | `TrialResult.State`/`Reason` |

The suggestion service is unchanged in Phase 1. Note the pinned Katib Optuna image only consumes succeeded and early-stopped trials and tracks its own in-flight suggestions in memory, so `Running` records benefit the provider only once the [#3950](https://github.com/kubeflow/trainer/pull/3950) contract lands; until then they serve the controller (dedup, capacity accounting) and observability.

#### Trial naming

The controller derives the trial `TrainJob` name before the record is written: `<prefix>-trial-<n>`, where `n` comes from the persisted `status.nextTrialIndex` (deriving it from the record count is not idempotent: a stale-cache reconcile would reuse the last index, overwrite that record in the map-keyed list, and then hit `AlreadyExists` on a `TrainJob` created with different parameters). `TrainJob` names are capped at 63 characters (RFC 1035), while `OptimizationJob` names are not length-bounded, so the prefix is the truncated `OptimizationJob` name plus a short hash of its UID; the hash keeps trial names collision-free when two long job names truncate identically in the same namespace. Persisting the name in the record before the child exists is what keeps record and child correlated across crashes and lets the expectations check key on a concrete name.

#### Code layout and PR breakdown

| Deliverable | Where | Depends on |
|---|---|---|
| `Trials`, `TrialResult`, `TrialState` API types + CEL + generated artifacts (`make generate`) | `pkg/apis/trainer/v1alpha1` | API PR [#3552](https://github.com/kubeflow/trainer/pull/3552) |
| Record lifecycle + SSA writer (using the existing `pkg/apply` helpers) | OptimizationJob controller | controller PR [#3828](https://github.com/kubeflow/trainer/pull/3828) |
| Snapshot source flip in the gRPC adapter | same controller package | dual-write step complete |
| Integration/E2E tests | `test/integration/`, `test/e2e/` | above |

The controller work lands as a follow-up to [#3828](https://github.com/kubeflow/trainer/pull/3828) rather than rewriting it: Phase 1's child-derived read paths stay untouched during the dual-write step and are removed together at the flip. This keeps each PR small and reviewable and avoids conflicting with the in-flight implementation.

### Compatibility and phasing

This KEP deliberately amends two design decisions of KEP-3562: §8.2, where the annotation-based history reconstruction gives way to `status.trials` as the durable record (parameter injection itself is unchanged), and §8.4, where "reconstructs historical state by reading their labels and annotations" is replaced by reading `status.trials`.

The field is optional and additive on `v1alpha1`: no version bump, no migration. Rollout in two steps aligned with the KEP-3562 phases:

1. The controller populates `status.trials` while keeping the Phase 1 child-derived snapshot as the source (dual-write, child-read).
2. Everything the controller currently derives from the child `TrainJob` list flips to `status.trials` together: the gRPC adapter's history, the `totalTrials`/`activeTrials` accounting, the completion check, the failure check, and the best-result computation. A partial flip is not safe: if only the adapter switches, deleting a finished child still lowers the trial counts (launching an extra trial) and can change `status.result`, since the best result would still be recomputed from surviving children each reconcile. After the flip, the interaction with deletion and TTL/GC cleanup of trial `TrainJob` objects is documented as supported.

### Risks and Mitigations

**Status object growth.** Measured on envtest 1.37 with these types, a stored record costs roughly 1.2 KB with map-typed parameters (about half of it `managedFields` bookkeeping) and roughly 0.8 KB with the atomic `parameters` list this KEP specifies. At the cap of 100 records that is well under 100 KB. The schema-permitted worst case is what matters: 100 records with 100 maximum-length parameters each serializes to ~2.7 MB and is rejected by the API server (the object also carries the spec), and the `MaxItems` caps count records and parameters, not bytes. That is why the controller estimates serialized size *before* every append (controller semantics, point 5) rather than relying on schema caps: the intended failure mode in normal operation is an explicit `TrialHistoryExhausted` condition, not a rejected status write. Should future work raise trial counts well beyond the current 100 (histories of 5,000+ trials have been discussed for Katib-scale workloads), record compaction or offloading becomes necessary and is deferred to the failure-policy and gRPC-contract work ([#3743](https://github.com/kubeflow/trainer/issues/3743), [#3796](https://github.com/kubeflow/trainer/issues/3796)). The CEL rules on `TrialResult` are not in tension with this principle: they reject only writes that violate the record's own invariants (a `Succeeded` record without metrics, a terminal record without a completion time), which can occur only through a controller bug. Rejecting those loudly at the API boundary is deliberate fail-fast; the principle above is about never letting *normal* operation (many failed trials) depend on a write the API server may refuse.

**Status update conflict pressure.** Writes are bounded at two per trial, records are keyed by `trainJobName` (`listType=map`, SSA-friendly), and all patches are idempotent. With `parallelTrials <= 100`, several children can go terminal within one reconcile; the SSA field manager and next-reconcile retry (controller semantics, point 6) make this safe without a transaction.

**API-convention concern (status should be reconstructible from observation).** `status.trials` aggregates observed child outcomes, matching upstream precedent for durable observed state in status: `Job.status.completedIndexes` and `CronJob.status.lastScheduleTime` are both durable, non-reconstructible observed state. The irrecoverability of child-derived state after garbage collection is precisely the motivation for persisting it.

**Records for running trials add churn relative to terminal-only writes.** Accepted deliberately: one extra write per trial is the price of closing every deletion race without child finalizers (see Alternatives), and it makes the suggestion snapshot fully self-contained.

### Test Plan

- [x] I/we understand the owners of the involved components may require updates to existing tests to make this code solid enough prior to committing the changes necessary to implement this enhancement.

#### Unit tests

- Append idempotency: re-reconciling the same suggestion produces exactly one record; double-observing the same terminal child patches it exactly once.
- Best-result projection correctness for both `Maximize` and `Minimize` objectives.
- Terminal patch taxonomy: `TrainJobFailed`, `TrainJobDeleted`, and `MetricsUnavailable` reasons each produced under the corresponding conditions.
- Snapshot assembly from `status.trials` equals the Phase 1 child-derived snapshot for identical cluster state.
- History bound: launch refusal and `TrialHistoryExhausted` condition when the record cap or the estimated byte bound is reached.
- Naming idempotency: a reconcile against stale status does not reuse a trial index (`nextTrialIndex`) or overwrite an existing record.
- `EarlyStopped` mapping: a pruned trial is recorded `EarlyStopped`, not `Failed`, and surfaces as such in the snapshot.

#### Integration tests (envtest, Ginkgo, `test/integration/`)

- Record-then-create ordering: controller crash (restart) between record append and child creation converges by marking the childless record `Failed`/`TrainJobDeleted`; no duplicate or resurrected `TrainJob` is created.
- Never-resurrect rule: a trial `TrainJob` deleted by a user while `Running` stays deleted after controller restart.
- Deleting a *running* trial `TrainJob` yields a `Failed`/`TrainJobDeleted` record and does not stall the optimization.
- Deleting a *completed* trial `TrainJob` changes neither `status.trials` nor subsequent suggestions.
- Deleting the `OptimizationJob` cascades cleanly: no children left in `Terminating`.
- Schema validation: `MaxItems`, state enum, CEL rules (`Succeeded` requires metrics; terminal requires `completionTime`), and `listMapKey` uniqueness.
- Status-update conflict: concurrent terminal observations converge across reconciles.
- SSA full-intent semantics: every apply carries the complete `trials` list, and an apply built from a stale read fails on the `resourceVersion` precondition (conflict) instead of silently dropping records.

#### E2E tests

- An `OptimizationJob` whose finished trial `TrainJob` objects are deleted during the run completes correctly, with `status.trials` complete and `status.result` correct.

## Open Questions

1. Should the snapshot-source flip (phasing step 2) sit behind a feature gate, or is the dual-write step sufficient protection?
2. When the trial failure policy ([#3743](https://github.com/kubeflow/trainer/issues/3743)) replaces the interim every-record-counts rule, and if trial counts eventually grow toward Katib-scale histories (5,000+), does bounded status remain the right store, or does the history need compaction (e.g. dropping parameters from failed records) or offloading?

## Implementation History

- **2026-07-16:** Problem identified in the KEP-3562 API review ([#3565, discussion](https://github.com/kubeflow/trainer/pull/3565#discussion_r3582000633)); tracking issue [#3744](https://github.com/kubeflow/trainer/issues/3744) opened.
- **2026-07-20:** Parent KEP-3562 merged with annotation-based history reconstruction for Phase 1; native status history deferred to this KEP.
- **2026-08-05:** KEP-3744 created.

## Drawbacks

- Duplicates trial data that (temporarily) also exists on child `TrainJob` objects.
- Two status writes per trial instead of zero; bounded and low, but not free.
- Status becomes semantically durable rather than purely reconstructible, which some reviewers may prefer to avoid despite the upstream precedent cited above.

## Alternatives

### Status quo: reconstruct history from child TrainJobs only

The Phase 1 design, already implemented in the controller ([#3828](https://github.com/kubeflow/trainer/pull/3828)). Simple, but it makes retained `TrainJob` objects load-bearing and is fundamentally incompatible with deletion of trials, which is the motivating bug of this KEP.

### Terminal-only records guarded by a child finalizer

An earlier draft of this KEP wrote records only at terminal state and protected the persistence window with a `trainer.kubeflow.org/trial-history` finalizer on each trial `TrainJob`. Rejected after closer analysis:

- A *running* trial deleted by a user never reaches the terminal condition the controller waits for, so its finalizer is never removed and the object wedges in `Terminating`.
- Deleting the `OptimizationJob` cascades deletes to all children while the owner is itself terminating; without careful extra machinery the finalizers block namespace deletion.
- Finalizers add RBAC surface, an operational failure mode when the controller is down, and well-known uninstall pain.

Writing the record at creation makes the entire class of problems unnecessary: there is no persistence race to guard, because nothing must ever be read back from a deleted child.

### External store (per-job ConfigMap or database)

A ConfigMap per `OptimizationJob` avoids status-size concerns but splits the source of truth across two objects, needs its own lifecycle/GC handling, and reintroduces the side-state that KEP-3562's stateless design set out to eliminate. A database contradicts that design outright (see KEP-3562's "Stateful Sidecars with Persistent Storage" alternative).

### Store history in the suggestion service

Would make the provider stateful again, recreating the exact architecture (Katib DB / stateful sidecars) that KEP-3562 replaces. Rejected.
