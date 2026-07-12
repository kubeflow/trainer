# ML Policy

This guide describes how to configure
[the `MLPolicy` API](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#MLPolicy)
in the Kubeflow Trainer Runtimes.

Before exploring this guide, make sure to follow [the Runtime guide](runtime)
to understand the basics of Kubeflow Trainer Runtimes.

## MLPolicy Overview

The `MLPolicy` API defines the ML-specific configuration for the training jobs - for example,
the number of training nodes (e.g., Pods) to launch, or PyTorch settings.

## Types of MLPolicy

The `MLPolicy` API supports multiple types, known as `MLPolicySources`. Each type defines how
the training job is launched and orchestrated. You can specify one of the supported sources in the
`MLPolicy` API.

### PlainML

The empty policy configures training jobs without a specialized distributed framework. It simply uses the `numNodes` value to set the job's parallelism (number of pods) and applies any training environment variables to each pod. For empty policy, the PlainML plugin is activated in the extension framework.

TrainJobs using this policy are launched as standard Kubernetes Jobs. The number of pods (parallelism) and completions is set based on the `numNodes` field in the `Trainer` spec, and environment variables from the TrainJob spec are added to the training containers.

```yaml
mlPolicy:
  numNodes: 10
```

### Torch

[The `Torch` policy](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#TorchMLPolicySource)
configures distributed training for PyTorch.

TrainJobs using this policy are launched via [the `torchrun` CLI](https://docs.pytorch.org/docs/stable/elastic/run.html).
You can customize `torchrun` options such as `numProcPerNode` to define number of
processes (e.g. GPUs) to launch per training node.

```yaml
mlPolicy:
  numNodes: 3
  torch:
    numProcPerNode: gpu
```

#### Inject PET_* envs into init/sidecar containers

By default, the Torch plugin injects `PET_*` topology env vars into the main trainer container only.
If you want to run distributed preflight checks in init containers (or other auxiliary containers), you can opt-in by configuring `torch.envInjection.targets` in the Runtime `mlPolicy`.

The `jobName` must match the replicated job name in the runtime template (for example, `node`), and each entry in `containerNames` must exist in that job pod spec (regular or init container).

```yaml
mlPolicy:
  numNodes: 2
  torch:
    envInjection:
      targets:
        - jobName: node
          containerNames:
            - preflight-check
```

Note: `envInjection` targets always get all `PET_*` env vars (`PET_NNODES`, `PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`, and `PET_MASTER_PORT`), independent of the trainer type.

The main trainer container is the exception under torchtune: it gets the rendezvous endpoint via a CLI argument rather than the `PET_MASTER_ADDR` and `PET_MASTER_PORT` env vars.

### MPI

[The `MPI` policy](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#MPIMLPolicySource)
configures distributed training using Message Passing Interface (MPI).

TrainJobs using this policy are launched via the `mpirun` CLI, the standard entrypoint for
MPI-based applications. This makes it compatible with frameworks like DeepSpeed which
[uses OpenMPI for distributed training](https://www.deepspeed.ai/getting-started/#launching-deepspeed-training).

You can customize the `MPI` options such as `numProcPerNode` to define the number of slots per
training node in the MPI hostfile.

```yaml
mlPolicy:
  numNodes: 2
  mpi:
    numProcPerNode: 4
    mpiImplementation: OpenMPI
    sshAuthMountPath: /home/mpiuser/.ssh
    runLauncherAsNode: true
```
