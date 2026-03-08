# Runtime Guide

This guide gives an overview for TrainingRuntime and ClusterTrainingRuntime.

Runtimes are template configurations or blueprints that are managed by platform administrators and
used by TrainJob to launch the desired training job.

## What is ClusterTrainingRuntime

The ClusterTrainingRuntime is a cluster-scoped API that allows platform administrators to manage
templates for TrainJobs. ClusterTrainingRuntime can be deployed across the entire Kubernetes cluster
and reused by AI practitioners in their TrainJobs. It simplifies the process of running training
jobs by providing standardized blueprints and ready-to-use environments.

### Example of ClusterTrainingRuntime

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime
```

In the runtime specification, platform administrators define a reusable template with the
appropriate configuration for distributed training. A TrainJob references this runtime
via the `RuntimeRef` API, which links to its `APIGroup`, `Kind` and `Name`. This allows the TrainJob
to adopt the runtime's configuration, enabling consistent and modular training setups.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: example-train-job
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    name: torch-distributed
    kind: ClusterTrainingRuntime
```

## What is TrainingRuntime

The TrainingRuntime is a namespace-scoped API that allows platform administrators to manage
templates for TrainJobs per namespace. It is ideal for teams or projects that need their own
customized training setups for each Kubernetes namespace, offering flexibility for
decentralized control.

### Example of TrainingRuntime

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: pytorch-team-runtime
  namespace: team-a
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime
```

:::{note}
When referencing TrainingRuntime, the Kubernetes namespace must be the same as the TrainJob's namespace
:::

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: example-train-job
  namespace: team-a # Only accessible to the namespace for which it is defined
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    name: pytorch-team-runtime
    kind: TrainingRuntime
```

## Framework Label Requirement

Every deployed runtime must have the label `trainer.kubeflow.org/framework` to ensure that
the Kubeflow SDK can recognize it, for example:

```yaml
trainer.kubeflow.org/framework: deepspeed
```

The Kubeflow SDK uses this label to determine the appropriate configuration for the supported
BuiltinTrainers.

Check [the Builtin Trainer guide](../user-guides/builtin-trainer/index) to understand what is CustomTrainer and BuiltinTrainer.

## Supported Runtimes

Kubeflow Trainer community maintains
[several ClusterTrainingRuntimes](https://github.com/kubeflow/trainer/tree/master/manifests/base/runtimes)
to help AI practitioners quickly experiment with Kubeflow Trainer, and to enable
platform administrators to extend these runtimes to fit their specific requirements.

The following runtimes are supported for CustomTrainer:

| Runtime Name | ML Framework |
| --------------------- | ------------------------------------------------------------- |
| torch-distributed | [PyTorch](https://docs.pytorch.org/docs/stable/index.html) |
| deepspeed-distributed | [DeepSpeed](https://www.deepspeed.ai/) |
| mlx-distributed | [MLX](https://ml-explore.github.io/mlx/build/html/index.html) |

The following runtimes are supported for TorchTune BuiltinTrainer:

| Runtime Name | Pre-trained LLM |
| --------------------- | --------------- |
| torchtune-llama3.2-1b | Llama 3.2 (1B) |
| torchtune-llama3.2-3b | Llama 3.2 (3B) |

### Runtime Deprecation Policy

As ML frameworks evolve over time, the Kubeflow community may decide to deprecate
certain supported ClusterTrainingRuntimes. To avoid breaking existing users, we follow a deprecation
policy before removing any runtime.

A supported ClusterTrainingRuntime may be marked as deprecated and is eligible for removal starting
from **two minor releases** after its deprecation.

These measures are taken to inform users about runtime deprecation:

- Add the following label to the deprecated runtime:

  ```yaml
  trainer.kubeflow.org/support: "deprecated"
  ```

- Document the deprecation as **a breaking change** in Kubeflow Trainer release notes.
- Show a warning from the Kubeflow Trainer validation webhook when:
  - A deprecated runtime is deployed on a Kubernetes cluster.
  - A TrainJob is created which references a deprecated runtime.
- Display a warning in the Kubeflow SDK when a deprecated runtime is listed or referenced.

## Next Steps

- Learn how to configure [job scheduling in Kubeflow Trainer](job-scheduling/index).
- Explore how to set up [MLPolicy in runtime](ml-policy).
- See how to define [Job Template in runtimes](job-template).
