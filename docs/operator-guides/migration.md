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

- Enhanced MPI support: featuring MPI-Operator v2 features with SSH-based optimization to boost
  MPI performance.

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

The following example demonstrates how to migrate the `tensorflow-benchmarks`
workload from the legacy `MPIJob` API to the `TrainJob` API. It is based on the
MPIJob configuration in the [legacy MPI guide](../legacy-v1/user-guides/mpi).

#### Old: MPIJob (v1)

```yaml
apiVersion: kubeflow.org/v1
kind: MPIJob
metadata:
  name: tensorflow-benchmarks
spec:
  slotsPerWorker: 2
  runPolicy:
    cleanPodPolicy: Running
  mpiReplicaSpecs:
    Launcher:
      replicas: 1
      template:
        spec:
          containers:
            - name: tensorflow-benchmarks
              image: mpioperator/tensorflow-benchmarks:latest
              command:
                - mpirun
                - --allow-run-as-root
                - -np
                - "2"
                - -bind-to
                - none
                - -map-by
                - slot
                - python
                - scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py
                - --model=resnet101
                - --batch_size=64
                - --variable_update=horovod
    Worker:
      replicas: 1
      template:
        spec:
          containers:
            - name: tensorflow-benchmarks
              image: mpioperator/tensorflow-benchmarks:latest
```

#### New: TrainJob (v2)

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: tensorflow-benchmarks
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    name: <mpi-runtime>
    kind: ClusterTrainingRuntime
  trainer:
    numNodes: 1
    image: mpioperator/tensorflow-benchmarks:latest
    command:
      - mpirun
      - --allow-run-as-root
      - -np
      - "2"
      - -bind-to
      - none
      - -map-by
      - slot
      - python
      - scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py
      - --model=resnet101
      - --batch_size=64
      - --variable_update=horovod
```

There is not currently a generic MPI runtime in `manifests/base/runtimes`, so
`<mpi-runtime>` is a placeholder. Reference an appropriate MPI-enabled
`TrainingRuntime` or `ClusterTrainingRuntime` for your environment.

MPI configuration is defined in the referenced runtime through its
`mlPolicy.mpi` configuration, rather than in the `TrainJob` specification.

For example:

```yaml
spec:
  mlPolicy:
    numNodes: 1
    mpi:
      numProcPerNode: 2
      mpiImplementation: OpenMPI
      sshAuthMountPath: /home/mpiuser/.ssh
      runLauncherAsNode: true
```

The MPI policy provides the configuration required to launch distributed MPI
workloads. The `numProcPerNode` field defines the number of MPI processes per
training node.

When migrating from `MPIJob` to `TrainJob`, the MPI-specific configuration is
moved from the `MPIJob` specification into the reusable runtime configuration.
This allows platform administrators to define the MPI environment once and
reuse it across multiple `TrainJobs`.

Map `MPIJob.spec.slotsPerWorker` to `TrainJob` runtime
`spec.mlPolicy.mpi.numProcPerNode`. In this example, both values are `2`.

The `mpiImplementation` field specifies the MPI implementation used by the
runtime. For the OpenMPI configuration, set it to `OpenMPI`.

### Kubeflow Trainer Python SDK

Kubeflow Trainer uses Kubeflow Python SDK to allow AI practitioners interact with Kubeflow Trainer
APIs without dealing with YAMLs or `kubectl`.

Check the [Getting Started](../getting-started/index) guide to learn how
to scale PyTorch code with `TrainJob` using Python SDK.

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
