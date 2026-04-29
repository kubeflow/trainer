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
