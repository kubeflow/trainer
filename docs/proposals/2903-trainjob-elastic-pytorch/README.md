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

This KEP adds elastic execution for PyTorch TrainJobs. A TrainingRuntime can declare the
minimum and maximum worker count accepted by torchrun. A TrainJob then uses its existing
spec.trainer.numNodes as the desired worker count within that range. Updating that field
causes Trainer to update the worker Indexed Job in the generated JobSet.

The first implementation does not choose a desired size from utilization metrics. It provides
the safe execution and reconciliation contract that a user or a future cluster-aware scaler
needs. This keeps scaling decisions separate from the work needed to make a running PyTorch
job safely accept a new membership.

## Motivation

Today Trainer copies spec.trainer.numNodes into both the JobSet worker count and PET_NNODES.
A later update of the TrainJob can therefore not safely resize an existing PyTorch job: JobSet
needs mutable worker counts, while an existing process needs a stable elastic torchrun
--nnodes range.

Upstream JobSet addresses part of this problem with ElasticJobSet. The alpha feature gate
allows an external controller to mutate parallelism and completions together on an Indexed
child Job. It does not make replicatedJobs[].replicas mutable. Trainer must use the supported
pod-level mechanism and preserve the relationship required by Indexed Jobs.

### Goals

- Add an ElasticPolicy to the PyTorch runtime policy with required minNodes and maxNodes.
- Configure the PyTorch launcher once with the immutable min:max PET_NNODES range.
- Let an update to spec.trainer.numNodes resize the worker Indexed Job by updating both
  parallelism and completions.
- Validate the elastic range and the desired size before reconciling the JobSet.
- Record the observed worker count so users can distinguish a requested resize from one still
  being applied.

### Non-Goals

- Selecting a desired size from GPU, CPU, progress, or queue metrics.
- Integrating a specific autoscaler such as KEDA, HPA, or Kueue.
- Partial preemption or quota accounting for elastic JobSets. These require Kueue support and
  a separate design.
- Elastic execution for runtimes other than PyTorch.
- Job-level scaling through replicatedJobs[].replicas, which is not supported by JobSet
  ElasticJobSet.

## Proposal

### User Stories

#### Elastic execution with a controlled desired size

An MLOps engineer configures a PyTorch runtime that can form a worker group from two to eight
nodes. The TrainJob starts with two workers. A cluster-aware controller, or the user, later
updates spec.trainer.numNodes to four. Trainer reconciles the worker Job's parallelism and
completions to four; torchrun retains its original 2:8 membership range and reforms the worker
group.

The workload must checkpoint at application-safe points. PyTorch Elastic restarts the worker
group when membership changes; it does not preserve in-memory training state across that
restart.

### Risks and Mitigations

**Application progress and checkpointing**

An elastic membership change restarts the worker group. A cooldown alone cannot establish that
the workload has written a usable checkpoint. The initial feature accepts only an explicit
desired-size update and makes no progress guarantee. A later autoscaling KEP may define a
workload progress or checkpoint-readiness signal, a per-job stabilization policy, and its
interaction with scale-down.

**JobSet support is gated**

Trainer currently depends on JobSet v0.11.1, which does not contain ElasticJobSet. The
implementation must first upgrade to JobSet v0.12 or later. ElasticJobSet is alpha and disabled
by default, so clusters enabling this feature must also enable the JobSet controller's
ElasticJobSet feature gate. Feature gates are not discoverable through the JobSet API;
deployment documentation and a Trainer feature gate must make this prerequisite explicit. The
feature remains disabled in Trainer unless an administrator has enabled the matching JobSet
gate.

**Ownership of the generated JobSet**

Trainer applies generated objects using server-side apply. A third party must not patch the
generated JobSet directly: the next TrainJob reconciliation can restore Trainer's desired
state. The supported integration point is spec.trainer.numNodes; the actor that changes it
uses its own field manager. Trainer remains the sole owner of the generated JobSet fields.

**Rendezvous configuration**

Elastic torchrun needs a stable rendezvous endpoint and application-specific restart and
timeout settings. The feature is supported only for a Trainer-compatible PyTorch launcher that
consumes the PET_* environment variables described below. Trainer does not rewrite an arbitrary
user command.

## Design Details

### API Details

~~~go
// ElasticPolicy configures the worker range for a PyTorch elastic launcher.
// The bounds are immutable for a running TrainJob.
// +kubebuilder:validation:XValidation:rule="self.minNodes <= self.maxNodes",message="minNodes must be less than or equal to maxNodes"
type ElasticPolicy struct {
	// MinNodes is the smallest worker group that may run.
	// +kubebuilder:validation:Minimum=1
	MinNodes int32 `json:"minNodes"`

	// MaxNodes is the largest worker group that may run.
	// +kubebuilder:validation:Minimum=1
	MaxNodes int32 `json:"maxNodes"`
}

// TorchMLPolicySource represents PyTorch distributed training configuration.
type TorchMLPolicySource struct {
	// ElasticPolicy configures elastic execution for this runtime.
	// +optional
	ElasticPolicy *ElasticPolicy `json:"elasticPolicy,omitempty"`
}
~~~

ElasticPolicy belongs to TorchMLPolicySource, rather than the generic Trainer API, because its
rendezvous and restart semantics are PyTorch-specific. A job selects its current desired count
with the existing spec.trainer.numNodes field. If the runtime has an elastic policy, the default
desired count is minNodes and every requested count must be in [minNodes, maxNodes].

minNodes and maxNodes are required non-pointer fields. The minimum markers reject zero and the
CEL rule rejects an inverted range at admission time.

The API version in the first implementation is trainer.kubeflow.org/v1alpha1.

### Example Manifest

~~~yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: torch-elastic-base
spec:
  mlPolicy:
    torch:
      elasticPolicy:
        minNodes: 2
        maxNodes: 8
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: resnet-elastic
spec:
  runtimeRef:
    name: torch-elastic-base
  trainer:
    numNodes: 2
    image: pytorch/pytorch:2.3.0-cuda12.1-cudnn8-runtime
    command: ["bash", "-c"]
    args:
      - >-
        torchrun
        --nnodes="${PET_NNODES}"
        --nproc-per-node="${PET_NPROC_PER_NODE}"
        --rdzv-backend=c10d
        --rdzv-id=resnet-elastic
        --rdzv-endpoint="${PET_MASTER_ADDR}:${PET_MASTER_PORT}"
        train.py --dataset s3://my-bucket/data
~~~

To request four workers, update the TrainJob with a field manager owned by the scaling actor:

~~~yaml
spec:
  trainer:
    numNodes: 4
~~~

The launcher options precede train.py, as required by torchrun. This KEP deliberately does not
append them to command or args, because after the training entrypoint they would be interpreted
as application arguments.

### Implementation

#### Runtime build

When a resolved Torch runtime has an ElasticPolicy, the Torch plugin:

1. Sets PET_NNODES once to <minNodes>:<maxNodes> instead of the current desired count.
2. Continues to set PET_NPROC_PER_NODE, PET_MASTER_ADDR, and PET_MASTER_PORT using the existing
   Trainer contract.
3. Uses spec.trainer.numNodes, defaulting to minNodes, as the initial worker count.

The initial implementation supports launchers that consume this contract. A later API can add a
structured launcher specification if Trainer needs to construct torchrun commands itself.

#### Reconciliation

On a TrainJob update, the controller resolves the policy and validates that the requested
numNodes is within the declared range. It generates the desired JobSet with the worker Indexed
Job's parallelism and completions set to that value. Server-side apply then updates those two
fields together.

The generated worker Job must use completionMode: Indexed, and its parallelism and completions
must always be equal. Trainer does not update replicatedJobs[].replicas.

The feature is available only when the Trainer and JobSet ElasticJobSet feature gates are both
enabled. The JobSet API does not expose its feature-gate state, so Trainer cannot reliably
discover the capability at admission time.

#### External scaling

The controller does not poll Kubernetes metrics in this phase. metrics.k8s.io commonly provides
CPU and memory only; GPU utilization depends on a device-specific exporter and may be ambiguous
for time-sliced or MIG GPUs. A GPU threshold API would need to define the metric source,
aggregation semantics, authorization, stabilization behavior, and checkpoint readiness.

HPA and KEDA cannot directly manage the nested worker fields because the JobSet does not expose
a suitable scale subresource. They may participate in a future design by producing the desired
TrainJob size through a defined integration, rather than by owning the JobSet. Kueue remains a
future integration for quota-aware growth and partial preemption.

### Validation

The validating webhook rejects a TrainJob when:

1. minNodes exceeds maxNodes.
2. An elastic TrainJob requests numNodes outside the resolved range.
3. The resolved worker Job is not Indexed, or parallelism differs from completions.
4. A running TrainJob changes its elastic range.
5. RuntimePatches try to set worker parallelism or completions while elastic policy is enabled.

## Status Management

spec.trainer.numNodes is the persisted desired size. It survives suspension, so a resumed
TrainJob uses the same requested worker count without relying on transient pod status.

The KEP adds an elastic status section, for example:

~~~yaml
status:
  elastic:
    minNodes: 2
    maxNodes: 8
    desiredNodes: 4
    observedNodes: 4
~~~

The controller snapshots the resolved range in status when it first creates the JobSet. It uses
that snapshot for the lifetime of the TrainJob, so later edits to a shared TrainingRuntime apply
only to new jobs. observedNodes is updated after the JobSet reports the matching worker count. A
pending resize therefore appears as desiredNodes: 4, observedNodes: 2. Existing JobSet-derived
job status remains the source for active, succeeded, and failed pod counts. Trainer does not
promise that an eviction above minNodes cannot fail the job; PyTorch restart policy and JobSet
failure policy determine that outcome.

## Test Plan

- **Unit tests:** policy defaulting and validation; PET_NNODES=min:max; desired-size changes
  update both parallelism and completions; updates outside the range fail validation; status
  records desired and observed counts.
- **Integration tests:** with JobSet ElasticJobSet enabled, create an Indexed worker Job and
  update spec.trainer.numNodes up and down. Verify the child Job receives matching parallelism
  and completions, and the controller does not overwrite the requested size.
- **E2E tests:** run a checkpointing PyTorch workload, resize through the TrainJob API, and
  verify that workers reform a rendezvous group, resume from a checkpoint, and produce the
  expected training result. Include suspend and resume after a resize.

Metric-driven scaling, node preemption, Kueue quota reconciliation, and MIG/time-slicing tests
belong to the follow-up design that introduces a concrete metric and preemption contract.
