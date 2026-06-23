# KEP-2903: Elastic PyTorch Training in Trainer V2

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [API Details](#api-details)
  - [Example Manifest](#example-manifest)
  - [Implementation](#implementation)
- [Status Management](#status-management)
- [Test Plan](#test-plan)

## Summary

This KEP proposes elastic distributed training support in Kubeflow Trainer V2. It introduces an `ElasticPolicy` field on `TorchMLPolicySource` and `Trainer`, allowing PyTorch jobs to
scale worker count dynamically at runtime. Scaling is delegated to Kubernetes HPA, which drives the `parallelism` mutability introduced in upstream JobSet KEP-463.

## Motivation

`TorchMLPolicySource` supports static multi-node PyTorch deployments but has no way to express elastic bounds (`minNodes`/`maxNodes`) or the metrics that should trigger a scaling
event. Without this, users running jobs on spot instances must tolerate full job failure on preemption, and metric-driven scale-out requires hand-authored HPA manifests outside the `TrainJob` API.

### Goals

- Introduce `ElasticPolicy` with `minNodes`, `maxNodes`, and optional `metrics`.
- Attach `ElasticPolicy` to `TorchMLPolicySource` (runtime default) and `Trainer` (per-job override).
- Auto-inject `--nnodes=<min>:<max>` and `--rdzv-backend=c10d` into the `torchrun` command.
- Generate an `HorizontalPodAutoscaler` targeting the worker `ReplicatedJob` when `metrics` are specified.
- Enforce elastic constraints (Indexed completion mode, `parallelism == completions`) via the validating webhook.

### Non-Goals

- A custom autoscaler inside the Training Operator. Scaling is fully delegated to Kubernetes HPA or Kueue.
- Elastic scaling for frameworks without native dynamic rendezvous (e.g. MPI).

## Proposal

### User Stories

#### Story 1: Spot Instance Resilience

As an MLOps engineer running PyTorch jobs on spot/preemptible GPU nodes, I want the job to
continue on surviving nodes if one is preempted, rather than fail entirely. I configure
`minNodes: 2` so the job keeps running as long as at least two nodes are available, and
scales back up when new spot capacity appears.

```yaml
apiVersion: kubeflow.org/v2alpha1
kind: TrainJob
metadata:
  name: resnet-spot
spec:
  trainer:
    image: pytorch/pytorch:2.3.0-cuda12.1-cudnn8-runtime
    command: ["torchrun", "train.py"]
    elasticPolicy:
      minNodes: 2
      maxNodes: 8
```

#### Story 2: Metric-Driven Scale-Out

As a data scientist, I want my job to start small and scale up automatically when GPU
utilization is high, without writing separate HPA manifests.

```yaml
apiVersion: kubeflow.org/v2alpha1
kind: TrainJob
metadata:
  name: llm-finetune
spec:
  trainer:
    image: pytorch/pytorch:2.3.0-cuda12.1-cudnn8-runtime
    command: ["torchrun", "finetune.py"]
    elasticPolicy:
      minNodes: 1
      maxNodes: 8
      metrics:
        - type: Resource
          resource:
            name: nvidia.com/gpu
            target:
              type: Utilization
              averageUtilization: 85
```

### Risks and Mitigations

**Rendezvous timeouts under rapid scaling**

If nodes are added faster than their pods become `Ready`, the `c10d` rendezvous barrier can
time out, causing workers to exit. This is inherent to `torchrun`'s rendezvous protocol and
is the user's responsibility to tune via `TORCH_ELASTIC_MAX_RESTARTS` and
`--rdzv-timeout`. We will document recommended values and flag this constraint clearly in
the `ElasticPolicy` API comment.

**Conflicts between `RuntimePatches` and elastic scaling**

`RuntimePatches` that mutate `parallelism` or `completions` on the worker `ReplicatedJob`
would race with HPA updates. The validating webhook will reject any `TrainJob` that sets an
`ElasticPolicy` and also includes `RuntimePatches` targeting those fields.

## Design Details

### API Details

```go
import autoscalingv2 "k8s.io/api/autoscaling/v2"

// ElasticPolicy configures elastic scaling bounds and triggers for a PyTorch training job.
// When set, the controller injects the appropriate torchrun rendezvous arguments and,
// if Metrics are provided, creates an HPA targeting the worker ReplicatedJob.
type ElasticPolicy struct {
    // MinNodes is the minimum number of worker nodes. Must be >= 1.
    // +kubebuilder:validation:Minimum=1
    MinNodes *int32 `json:"minNodes"`

    // MaxNodes is the maximum number of worker nodes. Must be >= MinNodes.
    // Cross-field validation is enforced by the validating webhook.
    MaxNodes *int32 `json:"maxNodes"`

    // Metrics defines the scaling triggers for the generated HPA.
    // If empty, no HPA is created; the job still runs with elastic torchrun args,
    // allowing external actors (e.g. Kueue) to drive parallelism changes.
    // +optional
    Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty"`
}

// TorchMLPolicySource represents PyTorch distributed training configuration.
type TorchMLPolicySource struct {
    // ElasticPolicy sets the default elastic bounds for this runtime.
    // Individual TrainJobs may override this via Trainer.ElasticPolicy.
    // +optional
    ElasticPolicy *ElasticPolicy `json:"elasticPolicy,omitempty"`
}

// Trainer configures the training process for a TrainJob.
type Trainer struct {
    // ... existing fields ...

    // ElasticPolicy overrides the ElasticPolicy defined in the TrainingRuntime.
    // +optional
    ElasticPolicy *ElasticPolicy `json:"elasticPolicy,omitempty"`
}
```

**Cross-field validation note:** `MaxNodes >= MinNodes` cannot be expressed as a
kubebuilder marker and is enforced in the validating webhook alongside the
`completionMode: Indexed` and `parallelism == completions` checks.

**Rendezvous backend:** `--rdzv-backend=c10d` is hardcoded for elastic jobs. `c10d` is the
only rendezvous backend that supports dynamic membership without an external store; `etcd`
requires additional infrastructure and is not considered here. Users who need a different
backend can set `torchrun` args directly and omit `ElasticPolicy`.

### Example Manifest

Full `TrainJob` using a `TrainingRuntime` that sets defaults and a job-level override:

```yaml
apiVersion: kubeflow.org/v2alpha1
kind: TrainingRuntime
metadata:
  name: torch-elastic-base
spec:
  mlPolicy:
    torch:
      elasticPolicy:
        minNodes: 1
        maxNodes: 16
---
apiVersion: kubeflow.org/v2alpha1
kind: TrainJob
metadata:
  name: my-elastic-job
spec:
  runtimeRef:
    name: torch-elastic-base
  trainer:
    image: pytorch/pytorch:2.3.0-cuda12.1-cudnn8-runtime
    command: ["torchrun", "train.py", "--dataset", "s3://my-bucket/data"]
    elasticPolicy:          # narrows the runtime default for this job
      minNodes: 2
      maxNodes: 8
      metrics:
        - type: Resource
          resource:
            name: nvidia.com/gpu
            target:
              type: Utilization
              averageUtilization: 80
```

### Implementation

During the **Build Phase** of reconciliation, the `TrainJobController` takes the following
steps when `ElasticPolicy` is present.

**Command injection**

The controller appends to the `torchrun` argument list:

```shell
--nnodes=<MinNodes>:<MaxNodes>
--rdzv-backend=c10d
--rdzv-id=<TrainJob UID>
--rdzv-endpoint=<headless Service>:<port>
```

These are injected after any user-specified args; a webhook prevents users from setting
`--nnodes` manually when `ElasticPolicy` is active to avoid conflicts.

**HPA generation**

If `ElasticPolicy.Metrics` is non-empty, `ComponentBuilder` creates an `HorizontalPodAutoscaler`
with:

- `scaleTargetRef` pointing to the worker `ReplicatedJob` within the `JobSet`
- `minReplicas` / `maxReplicas` sourced from `MinNodes` / `MaxNodes`
- `metrics` copied verbatim from `ElasticPolicy.Metrics`

The HPA is owned by the `TrainJob` and garbage-collected when the job completes or is deleted.

**Webhook validation**

The validating webhook rejects a `TrainJob` if:

1. `ElasticPolicy` is set and the resolved `JobSet` does not use `completionMode: Indexed`.
2. `ElasticPolicy` is set and resolved `parallelism != completions` on the worker `ReplicatedJob`.
3. `ElasticPolicy` is set and `RuntimePatches` mutates `parallelism` or `completions` on
   the worker `ReplicatedJob`.
4. `MaxNodes < MinNodes`.

## Status Management

`TrainJob` status relies on the underlying `JobSet` terminal condition and is unchanged.
Scaling activity is observable through `.status.jobsStatus`, which reflects the `JobSet`'s
live `active`/`succeeded`/`failed` counts as the HPA patches `parallelism`.

During a scale-up from 2 → 4 nodes, users will observe:

```yaml
.status.jobsStatus[worker].active: 2   # initial
.status.jobsStatus[worker].active: 4   # after HPA patch and pod readiness
```

During a scale-down (e.g. spot preemption):

```yaml
.status.jobsStatus[worker].active: 4
.status.jobsStatus[worker].active: 3   # one node lost
```

The `TrainJob` does not transition to `Failed` while `active >= minNodes`. Failure is only
triggered if `active` drops below `minNodes` and the underlying `JobSet` marks itself failed.

A Kubernetes `Event` is emitted on each HPA-driven `parallelism` patch to make scaling
transitions auditable without polling `.status`.

## Test Plan

- **Unit tests:** `ElasticPolicy` → `torchrun` arg translation; HPA generation logic;
  webhook validation rules (all four rejection cases above).
- **Integration tests:** `TrainJob` with `ElasticPolicy` creates a valid `JobSet` and HPA;
  patching `parallelism` on the `JobSet` is reflected in `.status.jobsStatus`.
- **E2E tests:** Simulate node removal (delete a worker pod) and verify the job continues
  at `active == minNodes`; simulate metric threshold breach and verify HPA scales
  `parallelism` up.