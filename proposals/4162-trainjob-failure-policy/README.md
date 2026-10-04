# KEP-4162: TrainJob Failure Policy

## Summary

This KEP adds a `failurePolicy` API to `TrainJob`, with a runtime-level default under
`TrainingRuntimeSpec.runPolicy`. The policy controls what happens when a Pod of a TrainJob
fails:

- **`maxRestarts`** bounds how many times the TrainJob is restarted before it is marked `Failed`.
- **`rules`** decide, for each failure, whether to restart the TrainJob, restart it without
  consuming the restart budget, or fail it immediately. A rule matches on Pod conditions, for
  example `DisruptionTarget`, which Kubernetes sets on preemption and node drain, or on
  container exit codes.
- **`status.restarts`** and **`status.restartsCountTowardsMax`** report the restart counters on
  the TrainJob.

The Trainer controller does not implement restarts itself. It translates the policy into the
[JobSet failure policy](https://github.com/kubernetes-sigs/jobset/blob/main/keps/262-ConfigurableFailurePolicy/README.md)
and the
[Job Pod failure policy](https://kubernetes.io/docs/concepts/workloads/controllers/job/#pod-failure-policy)
of the JobSet it already creates, and the JobSet and Job controllers do the work.

## Motivation

Long training jobs fail for reasons that have nothing to do with the training code. Nodes are
preempted, drained for upgrades, or lost. On preemptible GPU capacity this is the normal case.
A training platform has to answer two questions when a Pod fails: should the job be retried,
and how many times. Today a TrainJob user cannot answer either question.

[KEP-2170](../2170-kubeflow-trainer-v2/README.md) names `PodFailurePolicy` and the Pod
disruption condition as features that moving to JobSet would let Trainer "introduce easily".
The v2 API never surfaced them.

### How failures are handled today

A TrainJob becomes a JobSet, the JobSet creates one Job per step, and each Job creates Pods.
Failure handling is spread over three layers of that stack:

| Layer | Field | What it controls | Value in a TrainJob today |
| --- | --- | --- | --- |
| Pod | `restartPolicy` | Whether the kubelet restarts a failed container in place. | `OnFailure`. JobSet sets it when the template leaves it empty. |
| Job | `backoffLimit` | How many Pod failures and container restarts the Job tolerates. | 6, the Kubernetes default. |
| Job | `podFailurePolicy` | Rules that treat failures differently by exit code or Pod condition. | Unset. |
| JobSet | `failurePolicy` | What happens when a Job fails: fail the JobSet, or recreate its Jobs up to `maxRestarts` times. | Unset. The JobSet fails as soon as one Job fails. |

None of the runtimes shipped in `manifests/base/runtimes` set any of these fields, so every
TrainJob runs with the values in the last column. That has three consequences:

1. **A bug is retried before it is reported.** A training script that fails deterministically
   is restarted in place, with increasing backoff, until the Job-wide limit of six is reached.
   The user waits for all of those attempts before the TrainJob shows `Failed`.
2. **One incident can use up the whole budget.** All Pods of the trainer step belong to one
   Job and share one `backoffLimit`. With `OnFailure`, every container restart in the Job
   counts towards it. When one worker of an 8-node job disappears, the other seven usually
   crash as well, and those crashes alone can exceed the limit of six.
3. **A preemption is counted like a bug.** Nothing distinguishes a Pod that was evicted from a
   Pod whose training code crashed.

A platform admin can change this by hand-writing the JobSet and Job fields in the runtime
template, and some do
([#3779](https://github.com/kubeflow/trainer/issues/3779)). That requires understanding how
the three layers interact, and it still leaves two gaps:

- A data scientist cannot change the policy for a single TrainJob. `runtimePatches` does not
  allow the JobSet `failurePolicy`, or the Job `backoffLimit` and `podFailurePolicy`.
- `TrainJobStatus` does not report how many times a TrainJob was restarted. Users have to read
  the JobSet.

### What users have asked for

- [#3316](https://github.com/kubeflow/trainer/pull/3316) added Pod recreation after scheduler
  preemption directly in the TrainJob controller. It was closed with a request to propose a KEP.
- [#2185](https://github.com/kubeflow/trainer/issues/2185) and
  [#2072](https://github.com/kubeflow/trainer/issues/2072) asked for a maximum retry count and
  for recovery from failed nodes in v1. Both were deferred to the v2 API.
- [kubeflow/sdk#285](https://github.com/kubeflow/sdk/issues/285) proposes a `max_restarts`
  argument in the SDK, which has no TrainJob API to map to.

[KEP-2899](../2899-resource-timeouts/README.md) is introducing `RunPolicy` on the runtime
([#3824](https://github.com/kubeflow/trainer/pull/3824)) and names `BackoffLimit` as an example
of a lifecycle knob that can be added to it later. This KEP is that addition.

### Goals

- Add `failurePolicy` to `TrainJobSpec` so data scientists can bound restarts and choose which
  failures are retried.
- Add `failurePolicy` to `RunPolicy` on `TrainingRuntimeSpec` so platform admins can set a
  default for every TrainJob that uses a runtime.
- Let a policy treat infrastructure disruptions (preemption, eviction, node drain) differently
  from application failures, and fail immediately on exit codes the user marks as non-retriable.
- Report restart counters in `TrainJobStatus`.
- Expose the policy in the Kubeflow Python SDK.
- Build on the JobSet and Job failure policies. Add no Pod watches and no Pod permissions to
  the Trainer controller.

### Non-Goals

- Checkpointing or resuming training state. A restarted TrainJob starts its containers again.
  Resuming from a checkpoint is the responsibility of the training code. See
  [#2777](https://github.com/kubeflow/trainer/issues/2777) and
  [#2245](https://github.com/kubeflow/trainer/issues/2245).
- Restarting only part of a TrainJob, such as a single Pod or a single step. See
  [Future Plan](#future-plan).
- Detecting TrainJobs that hang without failing.
- Preventing disruptions. PodDisruptionBudgets are covered by
  [#3304](https://github.com/kubeflow/trainer/issues/3304).
- Retrying failed trials of an OptimizationJob
  ([#3743](https://github.com/kubeflow/trainer/issues/3743)).
- Changing the behavior of TrainJobs that have no failure policy.

## Proposal

A TrainJob gets an optional `failurePolicy`. When a Pod fails, the policy picks one of three
actions:

| Action | Effect |
| --- | --- |
| `FailTrainJob` | The TrainJob is marked `Failed` immediately. |
| `RestartTrainJob` | The TrainJob is restarted if fewer than `maxRestarts` counted restarts have happened. Otherwise it is marked `Failed`. This is the default for a failure that matches no rule. |
| `RestartTrainJobAndIgnoreMaxRestarts` | The TrainJob is restarted and the restart does not count towards `maxRestarts`. |

The decision path for one Pod failure:

```mermaid
flowchart TD
    A[A Pod of the TrainJob fails] --> B{Does the failure match<br/>a rule of the policy?}
    B -- "Yes" --> C{Action of the<br/>first matching rule}
    B -- "No" --> R[RestartTrainJob]
    C -- "FailTrainJob" --> F[TrainJob is Failed]
    C -- "RestartTrainJob" --> R
    C -- "RestartTrainJobAndIgnoreMaxRestarts" --> U[Restart<br/>restarts + 1]
    R --> M{restartsCountTowardsMax<br/>less than maxRestarts?}
    M -- "Yes" --> K[Restart<br/>restarts + 1<br/>restartsCountTowardsMax + 1]
    M -- "No" --> F
```

A policy with only `maxRestarts: 3` already covers the common request for a bounded number of
retries. Adding one rule for `DisruptionTarget` makes the TrainJob survive preemption without
spending that budget.

### User Stories

#### Story 1

As a **Data Scientist** running an 8-node fine-tuning job on preemptible GPU nodes, I want my
TrainJob to restart when a node is preempted without using up its retry budget, and to stop
after a small number of restarts when my own code crashes, so that I neither lose the run to a
preemption nor burn GPU hours retrying a bug.

#### Story 2

As a **Data Scientist**, my training script exits with code 42 when it detects an invalid
configuration. I want the TrainJob to fail immediately on that exit code and skip the restarts.

#### Story 3

As a **Platform Admin**, I want to set a default failure policy on a `ClusterTrainingRuntime`
so that every TrainJob using it survives node drains during cluster upgrades, without each
data scientist configuring it. Data scientists can still override the policy on a TrainJob.

#### Story 4

As a **Data Scientist**, I want to set the maximum number of restarts from the Kubeflow Python
SDK and see how many times my TrainJob has restarted, without reading JobSet objects.

### Notes/Constraints/Caveats

- **A restart recreates every Job of the TrainJob.** For distributed training this is the
  correct unit, because a single replaced worker cannot rejoin a running PyTorch DDP or MPI
  job. It also means that initializer steps run again on every restart, so initializers must
  be safe to re-run. [What a restart does](#what-a-restart-does) has the details.
- **Classification is best effort.** The first failed Pod that Kubernetes observes decides
  which rule applies. When a preempted worker takes its peers down with it, a peer's crash can
  be observed before the preempted Pod's `DisruptionTarget` condition. That incident is then
  handled as an ordinary failure.
- **The policy is fixed when the TrainJob is created.** `spec.failurePolicy` is immutable, and
  the runtime default is read from the runtime snapshot, so editing a runtime only affects
  TrainJobs created afterwards.
- **A restart is only as useful as the checkpoint behind it.** Without checkpoints on a
  persistent volume or in object storage, a restarted TrainJob repeats the whole run.

### Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| A disruption is classified as an ordinary failure and consumes a counted restart. | The error is in the safe direction, because it never causes extra restarts. The user guide recommends `maxRestarts` of at least 1 when relying on a disruption rule. |
| A `FailTrainJob` exit-code rule matches a worker that crashed only because a peer was preempted, which turns a preemption into a terminal failure. | The user guide recommends `FailTrainJob` only for exit codes that the training code reserves for non-retriable errors, and warns against generic codes such as 1. |
| `RestartTrainJobAndIgnoreMaxRestarts` restarts a TrainJob forever on a flapping node pool. | `activeDeadlineSeconds` is not reset by restarts, so it bounds the total run time. The webhook returns an admission warning when this action is used and no deadline is in effect. |
| Rules are mapped to JobSet rules by matching the Job failure message (`... matching FailJob rule at index N`), which Kubernetes does not version as an API. | This is the usage documented by JobSet [KEP-262](https://github.com/kubernetes-sigs/jobset/blob/main/keps/262-ConfigurableFailurePolicy/README.md). Integration and E2E tests pin the format on every supported Kubernetes version. |
| Re-running initializers on each restart is slow for large models and datasets. | Documented. Restarting only the failed step is planned once the JobSet `RestartJob` action graduates. |

## Design Details

### API Design

#### TrainJobSpec Changes

Add `FailurePolicy` to `TrainJobSpec` in `pkg/apis/trainer/v1alpha1/trainjob_types.go`:

```go
type TrainJobSpec struct {
    // ... existing fields ...

    // failurePolicy defines how the TrainJob reacts when one of its Pods fails.
    // It replaces the default from the referenced runtime's spec.runPolicy.failurePolicy.
    // If neither is set, failures are handled by the runtime's JobSet template.
    // This is an alpha field and requires enabling the TrainJobFailurePolicy feature gate.
    // +optional
    // +kubebuilder:validation:XValidation:rule="self == oldSelf", message="field is immutable"
    FailurePolicy *FailurePolicy `json:"failurePolicy,omitempty"`
}

// FailurePolicy defines how a TrainJob reacts to Pod failures.
type FailurePolicy struct {
    // maxRestarts is the number of times the TrainJob can be restarted by the
    // RestartTrainJob action before it is marked Failed.
    // Defaults to 0, which fails the TrainJob on the first counted failure.
    // +optional
    // +kubebuilder:validation:Minimum=0
    MaxRestarts *int32 `json:"maxRestarts,omitempty"`

    // rules are evaluated in order against a failed Pod. The first matching rule
    // decides the action. A failure that matches no rule is handled with the
    // RestartTrainJob action.
    // +optional
    // +listType=atomic
    // +kubebuilder:validation:MaxItems=20
    Rules []FailurePolicyRule `json:"rules,omitempty"`
}

// FailurePolicyAction is the action taken when a FailurePolicyRule matches.
// +kubebuilder:validation:Enum=FailTrainJob;RestartTrainJob;RestartTrainJobAndIgnoreMaxRestarts
type FailurePolicyAction string

const (
    // FailTrainJob marks the TrainJob as Failed immediately.
    FailTrainJob FailurePolicyAction = "FailTrainJob"

    // RestartTrainJob restarts the TrainJob if fewer than maxRestarts counted
    // restarts have happened. Otherwise the TrainJob is marked Failed.
    RestartTrainJob FailurePolicyAction = "RestartTrainJob"

    // RestartTrainJobAndIgnoreMaxRestarts restarts the TrainJob without counting
    // the restart towards maxRestarts.
    RestartTrainJobAndIgnoreMaxRestarts FailurePolicyAction = "RestartTrainJobAndIgnoreMaxRestarts"
)

// FailurePolicyRule describes the action taken when a failed Pod meets the requirement.
// Exactly one of onExitCodes and onPodConditions must be set.
// +kubebuilder:validation:XValidation:rule="has(self.onExitCodes) != has(self.onPodConditions)", message="exactly one of onExitCodes and onPodConditions must be set"
type FailurePolicyRule struct {
    // action is taken when the requirement is met.
    // +required
    Action FailurePolicyAction `json:"action,omitempty"`

    // onExitCodes is the requirement on the container exit codes of the failed Pod.
    // +optional
    OnExitCodes *batchv1.PodFailurePolicyOnExitCodesRequirement `json:"onExitCodes,omitempty"`

    // onPodConditions is the requirement on the conditions of the failed Pod.
    // It is met if at least one pattern matches a Pod condition.
    // +optional
    // +listType=atomic
    // +kubebuilder:validation:MaxItems=20
    OnPodConditions []batchv1.PodFailurePolicyOnPodConditionsPattern `json:"onPodConditions,omitempty"`
}
```

Three choices in this API are worth calling out:

- **The requirement types are reused from `batch/v1`.** `onExitCodes` and `onPodConditions`
  have the same fields and the same meaning as in the Job Pod failure policy, so users do not
  learn a second vocabulary and the controller can copy them unchanged.
- **The action names follow JobSet** (`FailJobSet`, `RestartJobSet`,
  `RestartJobSetAndIgnoreMaxRestarts`), with `TrainJob` as the object.
- **`spec.failurePolicy` is immutable in alpha.** The policy is written into the JobSet when
  it is created, and changing it mid-run raises questions about the counters that are better
  answered once there is usage feedback.

#### TrainingRuntimeSpec Changes

Add `FailurePolicy` to the `RunPolicy` struct introduced by KEP-2899:

```go
type RunPolicy struct {
    // ... activeDeadlineSeconds and ttlSecondsAfterFinished from KEP-2899 ...

    // failurePolicy is the default failure policy for TrainJobs that reference this runtime.
    // A TrainJob's own spec.failurePolicy takes precedence.
    // +optional
    FailurePolicy *FailurePolicy `json:"failurePolicy,omitempty"`
}
```

If `RunPolicy` has not merged when this KEP is implemented, the TrainJob field is implemented
first and the runtime default follows once `RunPolicy` exists.

#### TrainJobStatus Changes

```go
type TrainJobStatus struct {
    // ... existing fields ...

    // restarts is the number of times the TrainJob has been restarted by its failure policy.
    // +optional
    Restarts int32 `json:"restarts,omitempty"`

    // restartsCountTowardsMax is the number of restarts that count towards
    // failurePolicy.maxRestarts.
    // +optional
    RestartsCountTowardsMax int32 `json:"restartsCountTowardsMax,omitempty"`
}
```

Both fields mirror the fields of the same name in `JobSetStatus`. A TrainJob that survived one
preemption and one crash, with `maxRestarts: 2`, reports:

```yaml
status:
  restarts: 2
  restartsCountTowardsMax: 1
```

This KEP does not change how the `Failed` condition is derived. Its reason and message are
copied from the JobSet `Failed` condition, as today. With a failure policy the reasons are:

| TrainJob outcome | `Failed` condition reason |
| --- | --- |
| A `FailTrainJob` rule matched. | `FailJobSetFailurePolicyAction` |
| A counted failure occurred after `maxRestarts` restarts. | `ReachedMaxRestarts` |

### Semantics

#### Which failures match which requirement

`onPodConditions` is evaluated against the conditions of the failed Pod. The condition that
matters for training jobs is `DisruptionTarget`, which Kubernetes adds before it removes a Pod
for a reason outside the Pod's control. The
[Pod disruption conditions](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/#pod-disruption-conditions)
documentation lists the reasons:

| `DisruptionTarget` reason | When Kubernetes sets it |
| --- | --- |
| `PreemptionByScheduler` | The scheduler preempts the Pod to place a Pod with a higher priority. |
| `DeletionByTaintManager` | The Pod is deleted because of a `NoExecute` taint it does not tolerate, for example on an unreachable node. |
| `EvictionByEvictionAPI` | The Pod is evicted through the Eviction API, for example by `kubectl drain` or by the cluster autoscaler. |
| `DeletionByPodGC` | The Pod is bound to a node that no longer exists. |
| `TerminationByKubelet` | The kubelet terminates the Pod because of node pressure or a graceful node shutdown. |

`onExitCodes` is evaluated against the exit codes of the containers of the failed Pod, and can
be limited to one container with `containerName`. The container names in the built-in runtimes
are `node`, `launcher`, `dataset-initializer`, and `model-initializer`.

A container that is killed for exceeding its own memory limit exits with code 137 and does not
get a `DisruptionTarget` condition. It is an ordinary failure unless an exit-code rule matches
it.

#### Rule evaluation

Rules are evaluated in order, and the first rule whose requirement is met decides the action.
A failure that meets no requirement is handled with `RestartTrainJob`. This has two useful
consequences:

- A policy without rules is a plain retry limit.
- `maxRestarts: 0` with a single `DisruptionTarget` rule means "never retry my own failures,
  always survive a disruption".

#### What a restart does

A restart is the JobSet `RestartJobSet` action with the default `Recreate` strategy:

- Every Job of the JobSet is deleted and created again, and with it every Pod of every step.
  The new Pods are scheduled from scratch.
- Steps start in the same order as on the first attempt. In runtimes where the trainer step
  depends on the initializers, the initializer Jobs run again before the trainer starts.
- The JobSet, its headless Service, and the objects that Trainer plugins create, such as the
  MPI SSH Secret and hostfile ConfigMap, are kept. Job names and Pod hostnames do not change,
  so the addresses injected into the trainer, for example `PET_MASTER_ADDR`, stay valid.
- Data on persistent volumes is kept. Data in `emptyDir` volumes is lost.
- The Jobs and Pods of the new attempt carry the label and annotation
  `jobset.sigs.k8s.io/restart-attempt` with the attempt number. Training code can read it
  through the downward API to tell a restart from a first start.

| Runtime | What restarts |
| --- | --- |
| PyTorch (`torch-distributed`, TorchTune) | All `node` Pods. `torchrun` rendezvous again on the same addresses. |
| MPI (OpenMPI, DeepSpeed, MLX) | The `launcher` and `node` Jobs. The SSH keys and the hostfile are reused. |
| JAX, XGBoost | All `node` Pods. |
| Any runtime with initializers | The initializer Jobs first, then the trainer. |

### Value Resolution

The policy follows the precedence defined in KEP-2899 for `RunPolicy` fields:

```
effectivePolicy = trainJob.spec.failurePolicy               (if set)
             else runtime.spec.runPolicy.failurePolicy      (if set)
             else none (failures are handled by the runtime's JobSet template, as today)
```

The TrainJob policy replaces the runtime default as a whole. The two are not merged, because a
merged rule list would make the evaluation order hard to predict. The runtime default is read
from the runtime snapshot.

| `TrainJob.spec.failurePolicy` | `runPolicy.failurePolicy` | Effective policy |
| --- | --- | --- |
| Set | Set | The TrainJob policy. |
| Set | Unset | The TrainJob policy. |
| Unset | Set | The runtime default. |
| Unset | Unset | None. The behavior is unchanged. |

### User Examples

**Bound the number of restarts (Data Scientist):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: bounded-restarts
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
  failurePolicy:
    maxRestarts: 3
```

**Preemptible nodes with a non-retriable exit code (Data Scientist):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: preemptible-finetune
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8
  activeDeadlineSeconds: 86400
  failurePolicy:
    maxRestarts: 2
    rules:
      # Preemption, eviction, and node drain do not consume the restart budget.
      - action: RestartTrainJobAndIgnoreMaxRestarts
        onPodConditions:
          - type: DisruptionTarget
      # The training script reserves exit code 42 for invalid configuration.
      - action: FailTrainJob
        onExitCodes:
          containerName: node
          operator: In
          values: [42]
```

**Runtime default (Platform Admin):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed
spec:
  runPolicy:
    failurePolicy:
      maxRestarts: 1
      rules:
        - action: RestartTrainJobAndIgnoreMaxRestarts
          onPodConditions:
            - type: DisruptionTarget
  template:
    # ... JobSet template ...
```

### Implementation Overview

All changes are in the JobSet plugin (`pkg/runtime/framework/plugins/jobset`), the webhooks,
and the runtime `Info` object, which carries the effective policy. The controller gains no new
watches and no new RBAC.

#### Translation to JobSet

When an effective policy exists, the JobSet plugin sets the following fields when it builds
the JobSet:

1. On every replicated Job:
   - `backoffLimit: 0` and Pod `restartPolicy: Never`, so that the first Pod failure fails the
     Job and the decision is made once, at the JobSet level. `restartPolicy: Never` is also
     required by Kubernetes when `podFailurePolicy` is set.
   - `podFailurePolicy` with one `FailJob` rule per TrainJob rule, in the same order, with the
     requirement copied unchanged.
2. On the JobSet, `failurePolicy.maxRestarts` and one rule per TrainJob rule:

   | TrainJob action | JobSet action |
   | --- | --- |
   | `FailTrainJob` | `FailJobSet` |
   | `RestartTrainJob` | `RestartJobSet` |
   | `RestartTrainJobAndIgnoreMaxRestarts` | `RestartJobSetAndIgnoreMaxRestarts` |

   Each JobSet rule matches `onJobFailureReasons: [PodFailurePolicy]` and the message pattern
   `matching FailJob rule at index N$`, where N is the index of the corresponding Pod failure
   policy rule.

The two layers then work together as follows:

```mermaid
flowchart LR
    P[Pod fails] --> J{Job<br/>podFailurePolicy}
    J -- "rule N matches" --> JF1[Job Failed<br/>reason PodFailurePolicy<br/>message ends with<br/>rule at index N]
    J -- "no rule matches" --> JF2[Job Failed<br/>reason BackoffLimitExceeded]
    JF1 --> S{JobSet<br/>failurePolicy}
    JF2 --> S
    S -- "rule N matches" --> A1[Action of rule N]
    S -- "no rule matches" --> A2[RestartJobSet<br/>bounded by maxRestarts]
```

A Pod failure that matches no rule fails its Job with reason `BackoffLimitExceeded`. No JobSet
rule matches that reason, so JobSet applies its default action, `RestartJobSet`, which is
bounded by `maxRestarts`.

Kubernetes rejects a Job whose Pod failure policy names a container that is not in its Pod
template. An exit-code rule with `containerName` is therefore added only to the Jobs that have
that container. Because this can shift rule indexes between Jobs, the generated JobSet rules
are scoped with `targetReplicatedJobs`.

For the second user example above, the generated JobSet contains:

```yaml
spec:
  failurePolicy:
    maxRestarts: 2
    rules:
      - name: TrainJobFailurePolicyRule0
        action: RestartJobSetAndIgnoreMaxRestarts
        onJobFailureReasons: [PodFailurePolicy]
        onJobFailureMessagePatterns: ["matching FailJob rule at index 0$"]
      - name: TrainJobFailurePolicyRule1
        action: FailJobSet
        onJobFailureReasons: [PodFailurePolicy]
        onJobFailureMessagePatterns: ["matching FailJob rule at index 1$"]
        targetReplicatedJobs: [node]
  replicatedJobs:
    - name: node
      template:
        spec:
          backoffLimit: 0
          podFailurePolicy:
            rules:
              - action: FailJob
                onPodConditions:
                  - type: DisruptionTarget
                    status: "True"
              - action: FailJob
                onExitCodes:
                  containerName: node
                  operator: In
                  values: [42]
          template:
            spec:
              restartPolicy: Never
```

#### Worked Scenarios

The three scenarios below use the `preemptible-finetune` TrainJob from the user examples, with
`maxRestarts: 2`.

**A node is preempted.**

1. The scheduler preempts one of the eight `node` Pods. It adds the `DisruptionTarget`
   condition with reason `PreemptionByScheduler` and deletes the Pod.
2. The Pod terminates. The Job controller matches Pod failure policy rule 0 and fails the Job
   with reason `PodFailurePolicy` and a message that ends with
   `has condition DisruptionTarget matching FailJob rule at index 0`. The remaining Pods of
   the Job are terminated.
3. The JobSet controller matches `TrainJobFailurePolicyRule0` and applies
   `RestartJobSetAndIgnoreMaxRestarts`. It recreates the Jobs.
4. The TrainJob reports `restarts: 1` and `restartsCountTowardsMax: 0`. The restart budget is
   untouched.

**The training code crashes with exit code 1.**

1. A `node` container exits with code 1. No Pod failure policy rule matches. With
   `backoffLimit: 0` the Job fails with reason `BackoffLimitExceeded`.
2. No JobSet rule matches, so the JobSet controller applies `RestartJobSet`. The TrainJob
   reports `restarts: 1` and `restartsCountTowardsMax: 1`.
3. The code crashes again on the second and the third attempt. After the third failure the
   budget of two restarts is used up, and the TrainJob is `Failed` with reason
   `ReachedMaxRestarts`.

**The training code exits with the reserved code 42.**

1. The `node` container exits with code 42. The Job controller matches Pod failure policy
   rule 1 and fails the Job with a message that ends with `matching FailJob rule at index 1`.
2. The JobSet controller matches `TrainJobFailurePolicyRule1` and applies `FailJobSet`.
3. The TrainJob is `Failed` with reason `FailJobSetFailurePolicyAction` and `restarts: 0`.

#### Status

The JobSet plugin copies `status.restarts` and `status.restartsCountTowardsMax` from the JobSet
to the TrainJob. The controller emits a `Warning` event on the TrainJob each time
`status.restarts` increases, so that a restart is visible with `kubectl describe trainjob`.

#### Validation

| Check | Where | Result |
| --- | --- | --- |
| `maxRestarts` is negative. | CRD schema | Rejected. |
| A rule sets both or neither of `onExitCodes` and `onPodConditions`. | CRD schema (CEL) | Rejected. |
| `onExitCodes` or `onPodConditions` values break the Job API constraints, for example unordered exit codes. | TrainJob and runtime webhooks | Rejected at admission, so that the error does not surface later when the JobSet is created. |
| `onPodConditions[].status` is omitted. | Defaulting webhook | Set to `"True"`, matching the Job API. |
| `spec.failurePolicy` is changed on an existing TrainJob. | CRD schema (CEL) | Rejected. |
| A runtime sets `runPolicy.failurePolicy` together with `failurePolicy` on the JobSet template or `podFailurePolicy` on a Job template. | Runtime webhook | Rejected. A Trainer policy and a hand-written one cannot be combined. |
| A TrainJob sets `spec.failurePolicy` and the referenced runtime's template sets either of those fields. | TrainJob webhook | Rejected. |
| A rule uses `RestartTrainJobAndIgnoreMaxRestarts` and no `activeDeadlineSeconds` is in effect. | TrainJob webhook | Accepted with an admission warning. |

`backoffLimit` and `restartPolicy` in the runtime template are overridden when a policy is in
effect, because the translation depends on them.

#### Feature Gate

The feature is guarded by the `TrainJobFailurePolicy` feature gate, alpha and disabled by
default.

- With the gate disabled, the webhooks reject a TrainJob or a runtime that sets the new fields.
- A policy that is already stored is still honored after the gate is disabled, so that turning
  the gate off does not change the JobSet of a running TrainJob.
- TrainJobs without an effective policy are not affected by the gate in either state.

### Interaction with Other Features

- **`activeDeadlineSeconds`** ([KEP-2899](../2899-resource-timeouts/README.md)): a restart does
  not reset the deadline. The deadline bounds the total active time across all restarts.
- **Suspend and Kueue**: suspending a TrainJob is not a failure and evaluates no rule. Kueue
  preempts a TrainJob by suspending it, so Kueue preemption does not consume the restart
  budget. The restart counters are kept across suspend and resume.
- **MultiKueue**: the MultiKueue adapter copies `TrainJobSpec` to the worker cluster, so
  `spec.failurePolicy` travels with the TrainJob. A runtime default is resolved from the
  runtime in the worker cluster.
- **`ttlSecondsAfterFinished`** (KEP-2899): applies only after the TrainJob is terminal, so it
  does not interact with restarts.
- **TrainJob progress** ([KEP-2779](../2779-trainjob-progress/README.md)): `trainerStatus` is
  not reset on restart. The restarted trainer overwrites it with its next update.
- **PodDisruptionBudgets** ([#3304](https://github.com/kubeflow/trainer/issues/3304)): the two
  features complement each other. A PodDisruptionBudget reduces voluntary disruptions, and the
  failure policy decides what happens when a disruption still occurs.
- **OptimizationJob** ([KEP-3562](https://github.com/kubeflow/trainer/issues/3562)): trials are
  TrainJobs and inherit the runtime default, so a trial can survive a preemption without being
  reported as failed.

### Kubeflow SDK Changes

Add a `FailurePolicy` option to `kubeflow.trainer.options`, next to `ActiveDeadlineSeconds`,
and expose the restart counter on the SDK `TrainJob` type:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer
from kubeflow.trainer.options import FailurePolicy, FailurePolicyRule

TrainerClient().train(
    trainer=CustomTrainer(func=train_func, num_nodes=8),
    options=[
        FailurePolicy(
            max_restarts=2,
            rules=[
                FailurePolicyRule(
                    action="RestartTrainJobAndIgnoreMaxRestarts",
                    on_pod_conditions=["DisruptionTarget"],
                ),
            ],
        ),
    ],
)
```

The final shape of the SDK option is decided in `kubeflow/sdk`.

### Test Plan

[x] I/we understand the owners of the involved components may require updates to existing
tests to make this code solid enough prior to committing the changes necessary to implement
this enhancement.

#### Unit Tests

- `pkg/runtime/framework/plugins/jobset`:
  - Translation: each action maps to the expected JobSet rule and Pod failure policy rule.
  - An exit-code rule with `containerName` is added only to Jobs that have the container, and
    the JobSet rule indexes and `targetReplicatedJobs` match.
  - No effective policy: the JobSet built from the runtime template is unchanged.
  - Status: restart counters are copied from the JobSet.
- `pkg/runtime/core`: value resolution (TrainJob set, runtime default only, neither).
- `pkg/webhooks`: field validation, immutability, conflict with a template-level failure
  policy, feature gate disabled, admission warning for unbounded restarts.

#### Integration Tests

- A TrainJob with a policy produces a JobSet with the expected `failurePolicy`,
  `podFailurePolicy`, `backoffLimit`, and `restartPolicy`.
- A runtime default is applied, and a TrainJob policy replaces it.
- A runtime edited after the TrainJob is created does not change the applied policy.
- Restart counters on the JobSet status are reflected on the TrainJob status.
- Conflicting runtime and TrainJob configurations are rejected.

#### E2E Tests

- `test/e2e/trainjob_failure_policy_test.go`:
  - A trainer that fails on every attempt with `maxRestarts: 1` ends `Failed` with reason
    `ReachedMaxRestarts` and `status.restarts: 1`.
  - A trainer that exits with a code matched by a `FailTrainJob` rule ends `Failed` with
    `status.restarts: 0`.
  - A trainer Pod evicted through the Eviction API, with a `DisruptionTarget` rule, is
    restarted with `status.restarts: 1` and `status.restartsCountTowardsMax: 0`.

### Graduation Criteria

**Alpha**

- `TrainJobFailurePolicy` feature gate, disabled by default.
- TrainJob API, runtime default, translation, status counters, and validation implemented.
- Unit, integration, and E2E tests in place.

**Beta**

- Feature gate enabled by default.
- SDK support released.
- User guide for fault-tolerant TrainJobs published.
- Decision made on restarting only the failed step, based on the state of the JobSet
  `RestartJob` feature.
- Decision made on shipping a default policy in the built-in runtimes.

**Stable**

- No changes to the API for two releases.

## Future Plan

- **Restart only the failed step.** JobSet v0.12 has the `RestartJob` and
  `RestartJobAndIgnoreMaxRestarts` actions behind an alpha feature gate. Once they are enabled
  by default, a restart can recreate only the trainer step and leave completed initializer
  steps alone.
- **In-place restart.** The JobSet `InPlaceRestart` strategy, also alpha, restarts healthy
  Pods in place and avoids rescheduling them.
- **Default policy in the built-in runtimes**, for example a disruption rule.
- **Stalled TrainJob detection**, building on the progress reported through KEP-2779.

## Open Questions

1. When a TrainJob sets `spec.failurePolicy` and the runtime template already has a hand-written
   JobSet `failurePolicy`, this KEP rejects the TrainJob. The alternative is to let the TrainJob
   policy override the template. Rejecting is the conservative choice for alpha and can be
   relaxed later.
2. The `Failed` condition reasons are the JobSet reasons (`ReachedMaxRestarts`,
   `FailJobSetFailurePolicyAction`). Should the TrainJob map them to its own reason names?
3. Should the built-in runtimes ship a default policy with a `DisruptionTarget` rule once the
   feature is beta?

## Implementation History

- **2026-10-05**: Initial KEP draft.

## Drawbacks

- The feature adds a third place where failure handling can be configured, next to the JobSet
  and Job templates in the runtime. Validation rejects mixing them, but users have to learn
  which one applies.
- The translation depends on the Job failure message format, which is not a versioned API.

## Alternatives

### Alternative 1: Allow `runtimePatches` to patch the JobSet and Job failure policies

Extend `runtimePatches` with the JobSet `failurePolicy` and the Job `backoffLimit` and
`podFailurePolicy`.

**Pros:**
- No new API types.

**Cons:**
- Exposes three interacting Kubernetes layers to data scientists, which is the complexity the
  TrainJob API exists to hide.
- Users can build combinations that do not work, for example a Pod failure policy with
  `restartPolicy: OnFailure`.
- Hard to expose as a simple SDK option.

**Decision:** Rejected.

### Alternative 2: Only `maxRestarts`, without rules

Add a single `maxRestarts` field.

**Pros:**
- Smallest API.

**Cons:**
- Cannot distinguish a preemption from an application failure, which is the main use case.
- Cannot fail immediately on a non-retriable error.

**Decision:** Rejected. `maxRestarts` alone is still valid usage of the proposed API.

### Alternative 3: Handle failures in the TrainJob controller

Watch Pods in the TrainJob controller and delete or recreate them, as proposed in
[#3316](https://github.com/kubeflow/trainer/pull/3316).

**Cons:**
- Duplicates logic that the Job and JobSet controllers already implement.
- Requires Pod watches and Pod delete permissions in the Trainer controller, which works
  against the scalability goals in [#2318](https://github.com/kubeflow/trainer/issues/2318).
- Restart counters would have to be stored in annotations.

**Decision:** Rejected.

### Alternative 4: A fixed set of options without rules

Offer an enumerated field, for example `disruptionPolicy: Restart | Count`, and a list of
non-retriable exit codes.

**Pros:**
- Simpler to read for the two most common cases.

**Cons:**
- Every new case needs a new API field.
- Introduces vocabulary that differs from the Job and JobSet APIs users already know.

**Decision:** Rejected in favor of rules that reuse the `batch/v1` requirement types. The SDK
can still offer shortcuts for the common cases.

### Alternative 5: Retry individual Pods

Keep the Job `backoffLimit` as the retry mechanism and expose it on the TrainJob.

**Cons:**
- A replaced worker cannot rejoin a running PyTorch DDP or MPI job, so the unit of retry has to
  be the whole TrainJob.
- One incident consumes several retries, as described in
  [How failures are handled today](#how-failures-are-handled-today).

**Decision:** Rejected.
