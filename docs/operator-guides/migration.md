# Migrating to Kubeflow Trainer v2

## Overview

Kubeflow Trainer is a significant update to the Kubeflow Training Operator project.

The key features introduced by Kubeflow Trainer are:

- The new CRDs: TrainJob, TrainingRuntime, and ClusterTrainingRuntime APIs. These APIs enable the
  creation of templates for distributed model training and LLM fine-tuning. It abstracts the
  Kubernetes complexities, providing more intuitive experience for data scientists and ML engineers.

- The Kubeflow Python SDK: to further enhance ML user experience and to provide seamless integration
  with Kubeflow Trainer APIs.

- Custom dataset and model initializer: to streamline assets initialization across distributed
  training nodes and to reduce GPU cost by offloading I/O tasks to CPU workloads.

- Enhanced MPI support with OpenMPI, featuring MPI-Operator v2 features with SSH-based
  optimization to boost MPI performance.

## Migration Paths

Kubeflow Trainer v2 introduces new APIs that replace the older, framework-specific CRDs such as
`PyTorchJob`, `TFJob`, and `MPIJob`. These new APIs - `TrainJob`, `ClusterTrainingRuntime`,
and `TrainingRuntime` — offer a more flexible and unified interface for defining training
jobs across frameworks.

Please see [the runtime guide](runtime) to understand the concepts
of `TrainJob` and `ClusterTrainingRuntime`.

### Migrate PyTorchJob to TrainJob

The following example demonstrates how to migrate from `PyTorchJob` to `TrainJob`, utilizing the
default Torch runtime:

#### Old: PyTorchJob (v1)

```yaml
apiVersion: kubeflow.org/v1
kind: PyTorchJob
metadata:
  name: pytorch-simple
spec:
  pytorchReplicaSpecs:
    Master:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - name: pytorch
              image: docker.io/kubeflowkatib/pytorch-mnist:v1beta1-45c5727
              command:
                - "python3"
                - "/opt/pytorch-mnist/mnist.py"
                - "--epochs=1"
    Worker:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - name: pytorch
              image: docker.io/kubeflowkatib/pytorch-mnist:v1beta1-45c5727
              command:
                - "python3"
                - "/opt/pytorch-mnist/mnist.py"
                - "--epochs=1"
```

#### New: TrainJob (v2)

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-simple
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 2
    image: docker.io/kubeflowkatib/pytorch-mnist:v1beta1-45c5727
    command:
      - "python3"
      - "/opt/pytorch-mnist/mnist.py"
      - "--epochs=1"
```

### Migrate MPIJob to TrainJob

In Kubeflow Training Operator v1, an `MPIJob` defines the launcher and worker workloads directly
through `mpiReplicaSpecs`. MPI-specific settings such as the number of processes per worker are
also configured on the `MPIJob`.

Kubeflow Trainer v2 separates the training workload from the MPI execution environment. A
`TrainJob` references an MPI-enabled `TrainingRuntime` or `ClusterTrainingRuntime`, while the
runtime provides the reusable MPI configuration.

This means that migrating an `MPIJob` is not a direct field-by-field conversion of
`mpiReplicaSpecs`. The launcher/worker topology and MPI configuration move into the reusable
runtime, while workload-specific settings move into the `TrainJob`.

#### Old: MPIJob (v2beta1)

```yaml
apiVersion: kubeflow.org/v2beta1
kind: MPIJob
metadata:
  name: mpi-training
spec:
  slotsPerWorker: 1
  runPolicy:
    cleanPodPolicy: Running
  mpiReplicaSpecs:
    Launcher:
      replicas: 1
      template:
        spec:
          containers:
            - name: launcher
              image: mpi-training:latest
              command: ["mpirun", "-np", "2", "python3", "train.py"]
    Worker:
      replicas: 2
      template:
        spec:
          containers:
            - name: worker
              image: mpi-training:latest
```

#### New: TrainJob (v2)

The MPI runtime is configured separately by the platform administrator. The `TrainJob` references
that runtime and provides the workload-specific configuration:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: mpi-training
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    kind: ClusterTrainingRuntime
    name: <mpi-runtime>
  trainer:
    numNodes: 2
    numProcPerNode: 1
    image: mpi-training:latest
    command:
      - mpirun
      - python3
      - train.py
```

The runtime name in this example, `<mpi-runtime>`, is a placeholder for an MPI-enabled runtime
configured on the cluster. Runtime names are cluster-specific, so use the MPI runtime installed by
your platform administrator.

The `TrainJob` provides:

- `runtimeRef`: references the MPI-enabled runtime.
- `trainer.numNodes`: specifies the number of training nodes.
- `trainer.numProcPerNode`: specifies the number of MPI processes or slots per node and overrides
  the value configured in the runtime. For MPI, this must be an integer.
- `trainer.image`: specifies the container image used by the runtime's trainer container, typically
  the launcher. It does not replace the runtime's worker/node image.
- `trainer.command`: specifies the training command.

To use a custom MPI image for worker nodes, the platform administrator must create a
`TrainingRuntime` or `ClusterTrainingRuntime` whose node containers provide the required SSH
environment.

The MPI runtime provides the distributed-training environment, including the launcher/worker
topology, SSH communication configuration, and hostfile generation. The MPI plugin configures
the OpenMPI environment but does not add `mpirun` to the command, so users should include
`mpirun` in `trainer.command`.

The number of training nodes depends on the runtime's `mlPolicy.mpi.runLauncherAsNode` setting:

- When `runLauncherAsNode` is disabled, the launcher is separate and `trainer.numNodes` equals the
  old `Worker.replicas` value.
- When `runLauncherAsNode` is enabled, the launcher also participates as a training node, and the
  node replica count becomes `max(numNodes - 1, 1)`. Therefore, the old example's two workers and
  one launcher require `trainer.numNodes: 3` with such a runtime.

Always check the runtime before mapping the old launcher and worker counts.


### Kubeflow Trainer Python SDK

Kubeflow Trainer uses Kubeflow Python SDK to allow AI practitioners interact with Kubeflow Trainer
APIs without dealing with YAMLs or `kubectl`.

Check the [Getting Started](../getting-started/index) guide to learn how
to scale PyTorch code with `TrainJob` using Python SDK.

### MPI support

Kubeflow Trainer currently supports OpenMPI only. See the
[MLPolicy guide](ml-policy) for the full MPI policy reference, and the
[Flux guide](../user-guides/flux) for HPC workloads.

### Additional information

- Kubeflow Trainer v2 does not use separate CRDs for each framework. Instead, it implements all
  functionality within a single `TrainJob` CRD.
- AI practitioners should use the Kubeflow Python SDK to convert their model training code into a
  `TrainJob`.
- Platform administrators can leverage the `ClusterTrainingRuntime` and `TrainingRuntime` CRDs
  to configure reusable blueprints that enable AI practitioners to create `TrainJobs`.
- For a detailed overview of Kubeflow Trainer v2, please see
  [the announcement blog post](https://blog.kubeflow.org/trainer/intro/).

## Next Steps

- Learn about [the Kubeflow Trainer runtimes](runtime)
