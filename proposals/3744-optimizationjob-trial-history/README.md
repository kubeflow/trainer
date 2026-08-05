# KEP-3744: Store Trial History Natively in OptimizationJob Status

Authors:

- Sridhar Pillai (@Sridhar1030)

Related:

- Parent KEP: [KEP-3562: OptimizationJob CRD](../2605-optimization-job-crd/README.md) (in the `2605-optimization-job-crd` directory, tracked by [#3562](https://github.com/kubeflow/trainer/issues/3562))
- Tracking issue: [kubeflow/trainer#3744](https://github.com/kubeflow/trainer/issues/3744)

## Summary

In the Phase 1 design of [KEP-3562](../2605-optimization-job-crd/README.md), the `OptimizationJob` controller reconstructs trial history by listing child `TrainJob` objects and reading their parameter annotations and terminal metrics. This makes completed `TrainJob` objects load-bearing for the optimization itself. If a trial `TrainJob` is deleted — manually, by a cluster cleanup policy, or by the TTL-based lifecycle direction the Trainer project is moving toward — the experiment history is silently lost. The suggestion service, which rebuilds its state from that history on every `GetSuggestions` call, can then re-propose already-explored points or skew the sampler.

This KEP proposes making the `OptimizationJob` self-contained by recording a strongly-typed trial history in `OptimizationJob.status.trials`. The controller writes one record per trial when it creates the trial `TrainJob` (the parameters are known at that moment, because the controller generated them) and patches that record exactly once when the trial reaches a terminal state. Suggestion snapshots are then assembled entirely from `status.trials`. The optimization state lives on the `OptimizationJob` itself, nothing is ever read back from a child that might have been deleted, and the controller remains stateless — all state stays in the Kubernetes API server, preserving the stateless-provider architecture of KEP-3562.

## Motivation

The problem surfaced during API review of the `OptimizationJob` CRD ([#3565, discussion](https://github.com/kubeflow/trainer/pull/3565#discussion_r3582000633)): reconstructing trial history from `TrainJob` objects makes the `OptimizationJob` less self-contained and implicitly assumes those jobs are never garbage collected. `TrainJob` objects are execution resources with an independent lifecycle, and the `OptimizationJob` should own the optimization state itself. Storing the list of all trials (hyperparameters and scores) in status was raised as the more robust design, and deferred to a future iteration tracked by [#3744](https://github.com/kubeflow/trainer/issues/3744).

The deletion pressure on finished trials is real and growing:

- Finished trial `TrainJob` objects can be deleted manually or by cluster-level cleanup tooling today, and the underlying JobSet API already supports `ttlSecondsAfterFinished`.
- [KEP-2899](../2899-resource-timeouts/README.md) exists because finished jobs accumulate and bloat etcd. Its initial release ships `ActiveDeadlineSeconds`, and it explicitly lists `TTLSecondsAfterFinished` on the TrainJob API as a future plan. When that lands, automatic deletion of finished trials becomes a first-class, supported behavior.
- A 100-trial `OptimizationJob` produces on the order of 100 finished `TrainJob` objects (plus their JobSets) — exactly the accumulation that cleanup policies target.

Today, deleting any completed trial `TrainJob` corrupts a running optimization. The controller rebuilds an incomplete history, and the stateless suggestion provider silently degrades.

Storing history natively in status also improves observability and unblocks SDK work:

- Users can see every trial's parameters and objective values with `kubectl get optimizationjob -o yaml` instead of joining across (possibly deleted) child resources.
- The SDK `OptimizerClient` integration ([#3794](https://github.com/kubeflow/trainer/issues/3794)) can implement `get_job(...).trials` and `get_best_results(...)` from a single object read.
- Hyperparameters become strongly typed in the API (reusing `ParameterAssignment`), rather than a JSON blob in a `TrainJob` annotation.

### Goals

- Record each trial (parameters, objective metric values, state, timestamps) in `OptimizationJob.status.trials`.
- Make `status.trials` the canonical history used to build `GetSuggestions` snapshots, removing the dependency on retained `TrainJob` objects entirely.
- Preserve complete history when trial `TrainJob` objects are deleted at any point in their lifecycle, whether manually or via TTL/GC policies.
- Keep the controller stateless and the status write path idempotent, bounded, and low-churn.

### Non-Goals

- **Intermediate (per-epoch) metric time series in status.** Rich metric history remains the domain of external trackers (e.g. MLflow, TensorBoard), consistent with the non-goals of [KEP-2779](../2779-trainjob-progress/README.md). Status records terminal objective values only.
- **Trial suspension/resume and storage checkpointing.** Tracked separately in the KEP-3562 Phase 2 roadmap.
- **Changing the Phase 1 parameter-injection mechanism.** `KUBEFLOW_TRAINER_OPT_<NAME>` environment variables and `TrainJob` annotations remain as designed; annotations are simply no longer the durable record.
- **Changing the gRPC contract.** The snapshot format passed to the suggestion service is unchanged; only its source changes. The contract refactor is a separate Phase 2 item ([#3796](https://github.com/kubeflow/trainer/issues/3796)).
- **Trial failure policy.** How many failures an `OptimizationJob` tolerates before failing is a separate design discussion from KEP-3562; this KEP only ensures failures are durably recorded with a reason.

## Proposal

Add an optional `trials` list to `OptimizationJobStatus`, owned exclusively by the controller. Each trial's record is created together with its `TrainJob` and finalized exactly once at terminal state. Because the record is written *before* the child ever runs, no information is ever recoverable only from the child: deleting a trial `TrainJob` at any point — running or finished — can no longer lose history. No finalizers on child resources are required (see [Alternatives](#alternatives) for the rejected finalizer design).

### User Stories

**Story 1: Platform operator enabling cleanup of finished trials**

- **As a Platform Operator**, I want finished trial `TrainJob` objects to be deletable — manually today, and automatically once TTL cleanup lands per the KEP-2899 future plan — without corrupting running optimizations.
- **Motivation:** A single `OptimizationJob` can leave up to 100 finished `TrainJob` objects (and their JobSets) in etcd. With native history, they can be cleaned up as soon as they finish, while `status.result` and `status.trials` remain intact.

**Story 2: Data scientist auditing an experiment**

- **As a Data Scientist**, I want to inspect a finished `OptimizationJob` and see every trial's hyperparameters, objective value, and outcome — including *why* a trial failed — in one place.
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

Only the new field and types are shown; `conditions` and `result` on `OptimizationJobStatus` are unchanged from KEP-3562. `status.result` is retained as a convenience projection of `trials` (the best terminal trial) — no breaking change to the existing status API.

```go
type OptimizationJobStatus struct {
	// ... existing fields (conditions, result) ...

	// trials is the history of all trials launched by this OptimizationJob,
	// owned by the controller and used as the canonical suggestion snapshot.
	// Each record is created with its trial TrainJob and updated exactly once
	// when the trial reaches a terminal state.
	// The cap must stay >= the maximum allowed spec.numTrials plus headroom
	// for failed trials, which do not count toward numTrials; raise them together.
	// +listType=map
	// +listMapKey=trainJobName
	// +kubebuilder:validation:MaxItems=1000
	// +optional
	Trials []TrialResult `json:"trials,omitempty"`
}

// +kubebuilder:validation:Enum=Running;Succeeded;Failed
type TrialState string

const (
	TrialStateRunning   TrialState = "Running"
	TrialStateSucceeded TrialState = "Succeeded"
	TrialStateFailed    TrialState = "Failed"
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
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	TrainJobName string `json:"trainJobName"`

	// parameters are the hyperparameter assignments evaluated by this trial.
	// +listType=map
	// +listMapKey=name
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

- `ParameterAssignment` is the existing type from the KEP-3562 API — the same one used by `status.result`.
- `Metric` is the existing `{name, value string}` type introduced by KEP-2779 for `TrainJob.status.trainerStatus.metrics`. Reusing it keeps values consistent end to end: they are copied verbatim at terminal state.
- `Metrics` is a list so that multi-objective optimization ([#3799](https://github.com/kubeflow/trainer/issues/3799)) needs no structural change; Phase 1 populates only the single configured objective metric.
- The `Running`/`Succeeded`/`Failed` states mirror the trial lifecycle; `Reason` carries the failure taxonomy that the KEP-3562 review identified as important (distinguishing bad-hyperparameter failures from infrastructure failures and from misconfiguration). Notably, a trial that *succeeded* as a workload but reported no objective metric is recorded as `Failed` with `Reason: MetricsUnavailable` — suggestion providers must exclude it from sampler input either way, and the reason makes the misconfiguration visible and countable rather than silently discarded.

### Controller semantics

1. **On generating a suggestion:** the controller appends a `TrialResult` with `State: Running`, the assigned `Parameters`, and `CreationTime`, keyed by the (deterministically generated) trial `TrainJob` name — and then creates the trial `TrainJob`. The append happens first; both operations are idempotent, so a crash between them converges on the next reconcile (record exists, child missing and non-terminal: re-create the child with the same name and parameters).
2. **On observing a terminal child:** the controller patches the record once — `State`, `Metrics` (from `status.trainerStatus.metrics`), `Reason` if failed, and `CompletionTime` (from the terminal condition's `lastTransitionTime`) — and updates `status.result` if this trial improves the objective. A record that is already terminal is never modified again.
3. **On observing a deleted or deleting child whose record is still `Running`:** the record is patched to `Failed` with `Reason: TrainJobDeleted`. No history is lost — the parameters were recorded at creation.
4. **Suggestion snapshots:** the history passed to `GetSuggestions` is assembled entirely from `status.trials` (terminal records as completed trials, `Running` records as in-flight trials), replacing the Phase 1 list-and-parse of child annotations. The gRPC payload shape is unchanged. This also bounds the snapshot the algorithm service receives, which is relevant to the scalability concern about very large (~1500-trial) histories raised in the [KEP-3562 review](https://github.com/kubeflow/trainer/pull/3565#discussion_r3579906248).
5. **History bound:** before launching a trial, the controller checks that the `trials` list has room under its cap. If it does not (pathological failure churn), the controller stops launching trials and sets a `Failed` condition with reason `TrialHistoryExhausted` — an explicit, observable outcome rather than a rejected status write wedging the reconcile loop.
6. **Status writes:** the controller uses Server-Side Apply with a dedicated field manager for `status.trials`. Writes are bounded at two per trial (creation and terminal patch). On conflict, the write is retried on the next reconcile; because records are keyed by `trainJobName` and terminal patches are idempotent, retries converge.

### Deletion and lifecycle interactions

- **Deleting a trial `TrainJob`** (any state): history is already in status; the child deletes cleanly. No finalizers are involved, so nothing can wedge in `Terminating`.
- **Deleting the `OptimizationJob`:** child `TrainJob` objects are garbage collected through the existing owner references, exactly as in Phase 1. Status dies with the object, as expected.
- **TTL/GC cleanup of finished trials:** fully supported. A finished trial's record is complete before the child becomes deletable.

### Compatibility and phasing

The field is optional and additive on `v1alpha1` — no version bump, no migration. Rollout in two steps aligned with the KEP-3562 phases:

1. The controller populates `status.trials` while keeping the Phase 1 annotation-based snapshot as the source (dual-write, annotation-read).
2. The snapshot source flips to `status.trials`, and the interaction with deletion and TTL/GC cleanup of trial `TrainJob` objects is documented as supported.

### Risks and Mitigations

**Status object growth.** With the realistic record size (~0.5 KB: name, a handful of parameters, one or two metrics, timestamps), the cap of 1000 records yields ~500 KB — under etcd's ~1.5 MB request limit. The schema-permitted worst case is larger (up to 100 maximum-length parameters per trial is expressible), which is why the controller enforces the bound *before* appending (controller semantics, point 5) instead of relying on the schema cap: the failure mode is an explicit `TrialHistoryExhausted` condition, never a rejected status write.

**Status update conflict pressure.** Writes are bounded at two per trial, records are keyed by `trainJobName` (`listType=map`, SSA-friendly), and all patches are idempotent. With `parallelTrials <= 100`, several children can go terminal within one reconcile; the SSA field manager and next-reconcile retry (controller semantics, point 6) make this safe without a transaction.

**API-convention concern (status should be reconstructible from observation).** `status.trials` aggregates observed child outcomes, matching upstream precedent for durable observed state in status: `Job.status.completedIndexes` and `CronJob.status.lastScheduleTime` are both durable, non-reconstructible observed state. The irrecoverability of child-derived state after garbage collection is precisely the motivation for persisting it.

**Records for running trials add churn relative to terminal-only writes.** Accepted deliberately: one extra write per trial is the price of closing every deletion race without child finalizers (see Alternatives), and it makes the suggestion snapshot fully self-contained.

### Test Plan

#### Unit tests

- Append idempotency: re-reconciling the same suggestion produces exactly one record; double-observing the same terminal child patches it exactly once.
- Best-result projection correctness for both `Maximize` and `Minimize` objectives.
- Terminal patch taxonomy: `TrainJobFailed`, `TrainJobDeleted`, and `MetricsUnavailable` reasons each produced under the corresponding conditions.
- Snapshot assembly from `status.trials` equals the Phase 1 annotation-derived snapshot for identical cluster state.
- History bound: launch refusal and `TrialHistoryExhausted` condition when the cap is reached.

#### Integration tests (envtest, Ginkgo — `test/integration/`)

- Record-then-create ordering: controller crash between record append and child creation converges (child created with same name and parameters on restart).
- Deleting a *running* trial `TrainJob` yields a `Failed`/`TrainJobDeleted` record and does not stall the optimization.
- Deleting a *completed* trial `TrainJob` changes neither `status.trials` nor subsequent suggestions.
- Deleting the `OptimizationJob` cascades cleanly: no children left in `Terminating`.
- Schema validation: `MaxItems`, state enum, CEL rules (`Succeeded` requires metrics; terminal requires `completionTime`), and `listMapKey` uniqueness.
- Status-update conflict: concurrent terminal observations converge across reconciles.

#### E2E tests

- An `OptimizationJob` whose finished trial `TrainJob` objects are deleted during the run completes correctly, with `status.trials` complete and `status.result` correct.

## Open Questions

1. Should the snapshot-source flip (phasing step 2) sit behind a feature gate, or is the dual-write step sufficient protection?
2. Is `MaxItems=1000` the right headroom for failed-trial records, given that trial failure policy (how many failures an `OptimizationJob` tolerates) is still an open design in the parent KEP?

## Implementation History

- **2026-07-16:** Problem identified in the KEP-3562 API review ([#3565, discussion](https://github.com/kubeflow/trainer/pull/3565#discussion_r3582000633)); tracking issue [#3744](https://github.com/kubeflow/trainer/issues/3744) opened.
- **2026-07-20:** Parent KEP-3562 merged with annotation-based history reconstruction for Phase 1; native status history deferred to this KEP.
- **2026-08-05:** KEP-3744 created.

## Drawbacks

- Duplicates trial data that (temporarily) also exists on child `TrainJob` objects.
- Two status writes per trial instead of zero; bounded and low, but not free.
- Status becomes semantically durable rather than purely reconstructible, which some reviewers may prefer to avoid despite the upstream precedent cited above.

## Alternatives

### Status quo: reconstruct history from TrainJob annotations only

The Phase 1 design, already implemented in the controller ([#3828](https://github.com/kubeflow/trainer/pull/3828)). Simple, but it makes retained `TrainJob` objects load-bearing and is fundamentally incompatible with deletion of trials — the motivating bug of this KEP.

### Terminal-only records guarded by a child finalizer

An earlier draft of this KEP wrote records only at terminal state and protected the persistence window with a `trainer.kubeflow.org/trial-history` finalizer on each trial `TrainJob`. Rejected after closer analysis:

- A *running* trial deleted by a user never reaches the terminal condition the controller waits for, so its finalizer is never removed and the object wedges in `Terminating`.
- Deleting the `OptimizationJob` cascades deletes to all children while the owner is itself terminating; without careful extra machinery the finalizers block namespace deletion.
- Finalizers add RBAC surface, an operational failure mode when the controller is down, and well-known uninstall pain.

Writing the record at creation makes the entire class of problems unnecessary: there is no persistence race to guard, because nothing must ever be read back from a deleted child.

### External store (per-job ConfigMap or database)

A ConfigMap per `OptimizationJob` avoids status-size concerns but splits the source of truth across two objects, needs its own lifecycle/GC handling, and reintroduces the side-state that KEP-3562's stateless design set out to eliminate. A database contradicts that design outright (see KEP-3562's "Stateful Sidecars with Persistent Storage" alternative).

### Store history in the suggestion service

Would make the provider stateful again — the exact architecture (Katib DB / stateful sidecars) that KEP-3562 replaces. Rejected.
