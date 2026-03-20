# How to manage Jobs in multi-cluster environment

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

## Overview

This documentation details the usage of the `MultiKueue` feature within the Kueue project, specifically for Kubeflow MPI Jobs. The `MultiKueue` capability allows for efficient management and scheduling of multiple queues, optimizing resource allocation and improving the overall efficiency of MPI Jobs. The `spec.runPolicy.managedBy` field is a new feature introduced for MultiKueue support in the Kubeflow Training Operator. This field allows for more robust management of multi-cluster job dispatching by specifying the managing entity.

## Prerequisites

1. Ensure that you have the version up to 1.9 of the Kubeflow Training Operator installed and version 0.11+ for kueue.
2. Make sure Kueue is compiled against the new operator to leverage the `spec.runPolicy.managedBy` field.

## Usage

To use the `spec.runPolicy.managedBy` field in your training jobs, include it in the job specification as shown below:

```yaml
apiVersion: "kubeflow.org/v1"
kind: "TFJob"
metadata:
  name: "example-tfjob"
spec:
  runPolicy:
    managedBy: "kueue.x-k8s.io/multikueue"
  tfReplicaSpecs:
    ...
```

Here is a complete example of a TensorFlow job using the `spec.managedBy` field:

```yaml
apiVersion: "kubeflow.org/v1"
kind: "TFJob"
metadata:
  name: "example-tfjob"
spec:
  runPolicy:
    managedBy: "kueue.x-k8s.io/multikueue"
  tfReplicaSpecs:
    Chief:
      replicas: 1
      template:
        spec:
          containers:
            - name: tensorflow
              image: tensorflow/tensorflow:latest
              args: ["python", "model.py"]
    Worker:
      replicas: 2
      template:
        spec:
          containers:
            - name: tensorflow
              image: tensorflow/tensorflow:latest
              args: ["python", "model.py"]
```

## More Details

For more details on setting up and using MultiKueue with the Kubeflow Training Operator, refer to the following documentation pages:

- [Kueue/Kubeflow](https://kueue.sigs.k8s.io/docs/tasks/run/kubeflow/)
- [Kueue MultiKueue Docs](https://kueue.sigs.k8s.io/docs/concepts/multikueue/)
