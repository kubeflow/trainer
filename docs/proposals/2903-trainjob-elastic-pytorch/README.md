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

This KEP proposes elastic distributed training support in Kubeflow Trainer V2. It introduces an `ElasticPolicy` field on `TorchMLPolicySource` and `Trainer`, allowing PyTorch jobs to scale worker count dynamically at runtime. Scaling is driven by a lightweight metric-reconciliation loop inside the `TrainJob` controller (or external orchestrators like Kueue), which directly patches the `parallelism` field introduced in upstream JobSet KEP-463.

## Motivation

`TorchMLPolicySource` supports static multi-node PyTorch deployments but has no way to express elastic bounds (`minNodes`/`maxNodes`) or the metrics that should trigger a scaling event. 
Without this, users running jobs on spot instances must tolerate full job failure on preemption, and metric-driven scale-out requires hand-authored autoscaling manifests outside the `TrainJob` API.

### Goals

- Introduce `ElasticPolicy` with `minNodes`, `maxNodes`, and optional `metrics`.
- Attach `ElasticPolicy` to `TorchMLPolicySource` (runtime default) and `Trainer` (per-job override).
- Auto-inject `--nnodes=<min>:<max>` and `--rdzv-backend=c10d` into the `torchrun` command.
- Implement a lightweight metrics-watcher in the `TrainJob` controller to directly patch `JobSet` parallelism when `metrics` are specified.
- Enforce elastic constraints (Indexed completion mode, `parallelism == completions`) via the validating webhook.

### Non-Goals

- Advanced, predictive autoscaling algorithms. The internal metrics loop performs basic step-scaling; complex queue-based or budget-constrained scaling remains delegated to external actors (e.g., Kueue).
- Elastic scaling for frameworks without native dynamic rendezvous (eg. MPI).

## Proposal

### User Stories

#### Story 1: Spot Instance Resilience

As an MLOps engineer running PyTorch jobs on spot/preemptible GPU nodes, I want the job to
continue on surviving nodes if one is preempted, rather than fail entirely. I configure
`minNodes: 2` so the job keeps running as long as at least two nodes are available, and
scales back up when new spot capacity appears.

```yaml
apiVersion: trainer.kubeflow.org/v2alpha1
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

As a data scientist, I want my job to start small and scale up automatically when GPU utilization is high, 
without writing external autoscaling configurations.

```yaml
apiVersion: trainer.kubeflow.org/v2alpha1
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

`RuntimePatches` that mutate `parallelism` or `completions` on the worker `ReplicatedJob` would race with the controller's internal scaling loop. The validating webhook will reject any `TrainJob` that sets an `ElasticPolicy` and also includes `RuntimePatches` targeting those fields.

## Design Details

### API Details

```go
import autoscalingv2 "k8s.io/api/autoscaling/v2"

// ElasticPolicy configures elastic scaling bounds and triggers for a PyTorch training job.
// When set, the controller injects the appropriate torchrun rendezvous arguments and,
// if Metrics are provided, actively reconciles worker parallelism based on live utilization.
type ElasticPolicy struct {
    // MinNodes is the minimum number of worker nodes. Must be >= 1.
    // +kubebuilder:validation:Minimum=1
    MinNodes *int32 `json:"minNodes"`

    // MaxNodes is the maximum number of worker nodes. Must be >= MinNodes.
    // Cross-field validation is enforced by the validating webhook.
    MaxNodes *int32 `json:"maxNodes"`

    // Metrics defines the scaling triggers for the internal controller reconciliation loop.
    // If empty, internal metric evaluation is skipped; the job still runs with elastic torchrun args,
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
apiVersion: trainer.kubeflow.org/v2alpha1
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
apiVersion: trainer.kubeflow.org/v2alpha1
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
--rdzv-endpoint=<TrainJob-Name>-<ReplicatedJob-Name>-0-0.<TrainJob-Name>.<Namespace>.svc.cluster.local:<Port>
```

These are injected after any user-specified args; a webhook prevents users from setting
`--nnodes` manually when `ElasticPolicy` is active to avoid conflicts.

**Metric Reconciliation Loop**

Because an inline `ReplicatedJob` does not expose a top-level `/scale` subresource for the native `HorizontalPodAutoscaler` to target, the `TrainJobController` natively handles metric evaluation:

1. **Querying:** When `ElasticPolicy.Metrics` is present, the controller initializes a standard Kubernetes metrics client (`k8s.io/metrics/pkg/client/clientset_generated/clientset`) and polls the requested metrics for the active worker Pods.
2. **Evaluation:** It calculates the current utilization percentage against the target defined in the `MetricSpec`.
3. **Direct Patching:** If a scaling threshold is breached, the controller issues a direct in-place `PATCH` to the underlying `jobset.spec.replicatedJobs[worker].template.spec.parallelism`.
4. **Hysteresis (Cooldown):** To prevent rapid oscillation ("flapping") of GPU allocations, the controller enforces a static `scaleCooldownSeconds` window (defaulting to 60 seconds) after any successful patch before evaluating metrics again. 

If `ElasticPolicy.Metrics` is omitted entirely, the controller skips this loop, leaving the `parallelism` field completely open for external controllers (such as Kueue) to patch dynamically based on cluster quota.

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
live `active`/`succeeded`/`failed` counts as the controller patches `parallelism`.

During a scale-up from 2 → 4 nodes, users will observe:

```yaml
.status.jobsStatus[worker].active: 2   # initial
.status.jobsStatus[worker].active: 4   # after controller parallelism patch and pod readiness
```

During a scale-down (e.g. spot preemption):

```yaml
.status.jobsStatus[worker].active: 4
.status.jobsStatus[worker].active: 3   # highest indexed pod removed
```

The `TrainJob` does not transition to `Failed` while `active >= minNodes`. Failure is only
triggered if `active` drops below `minNodes` and the underlying `JobSet` marks itself failed.

A Kubernetes `Event` is emitted on each controller-driven `parallelism` patch to make 
scaling transitions auditable without polling `.status`.

## Test Plan

- **Unit tests:** `ElasticPolicy` → `torchrun` arg translation; metric evaluation and 
hysteresis cooldown logic; webhook validation rules (all four rejection cases above).
- **Integration tests:** `TrainJob` with `ElasticPolicy` creates a valid `JobSet`; 
simulated metric threshold breaches correctly issue `PATCH` requests to `JobSet` 
parallelism; manual patching of `parallelism` on the `JobSet` is reflected in `.status.jobsStatus`.
- **E2E tests:** Simulate node removal (delete a worker pod) and verify the job continues
  at `active == minNodes`; simulate metric threshold breach and verify the controller scales `parallelism` up.