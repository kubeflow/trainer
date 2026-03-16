# Installation

How to install the Training Operator

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../operator-guides/migration.md).
:::

This guide describes how to install the Training Operator on your Kubernetes cluster.
The Training Operator is a lightweight Kubernetes controller that orchestrates the
appropriate Kubernetes workloads to perform distributed ML training and fine-tuning.

## Prerequisites

These are the minimal requirements to install the Training Operator:

- Kubernetes >= 1.28
- `kubectl` >= 1.28
- Python >= 3.7

## Installing the Training Operator

You need to install the Training Operator control plane and Python SDK to create training jobs.

### Installing the Control Plane

You can skip these steps if you have already
[installed Kubeflow platform](https://www.kubeflow.org/docs/started/installing-kubeflow/)
using manifests or package distributions. The Kubeflow platform includes the Training Operator.

You can install the Training Operator as a standalone component.

Run the following command to install the stable release of the Training Operator control plane: `v1.8.1`

```shell
kubectl apply --server-side -k "github.com/kubeflow/training-operator.git/manifests/overlays/standalone?ref=v1.8.1"
```

Run the following command to install the latest changes of Training Operator control plane:

```shell
kubectl apply --server-side -k "github.com/kubeflow/training-operator.git/manifests/overlays/standalone?ref=master"
```

After installing it, you can verify that Training Operator controller is running as follows:

```shell
$ kubectl get pods -n kubeflow

NAME                                             READY   STATUS    RESTARTS   AGE
training-operator-658c68d697-46zmn               1/1     Running   0          90s
```

Run this command to check installed Kubernetes CRDs for each supported ML framework:

```shell
$ kubectl get crd

mpijobs.kubeflow.org                                     2023-06-09T00:31:07Z
mxjobs.kubeflow.org                                      2023-06-09T00:31:05Z
paddlejobs.kubeflow.org                                  2023-06-09T00:31:09Z
pytorchjobs.kubeflow.org                                 2023-06-09T00:31:06Z
tfjobs.kubeflow.org                                      2023-06-09T00:31:04Z
xgboostjobs.kubeflow.org                                 2023-06-09T00:31:04Z
```

### Installing the Python SDK

The Training Operator [implements a Python SDK](https://pypi.org/project/kubeflow-training/)
to simplify creation of distributed training and fine-tuning jobs.

Run the following command to install the latest stable release of the Training SDK:

```shell
pip install -U kubeflow-training
```

Run the following command to install the latest changes of Training SDK:

```shell
pip install git+https://github.com/kubeflow/training-operator.git@master#subdirectory=sdk/python
```

Otherwise, you can also install the Training SDK using the specific GitHub commit, for example:

```shell
pip install git+https://github.com/kubeflow/training-operator.git@7345e33b333ba5084127efe027774dd7bed8f6e6#subdirectory=sdk/python
```

#### Install the Python SDK with Fine-Tuning Capabilities

If you want to use the `train` API for LLM fine-tuning with the Training Operator, install the Python SDK
with the additional packages from HuggingFace:

```shell
pip install -U "kubeflow-training[huggingface]"
```

## Next steps

Run your first Training Operator Job by following the [Getting Started guide](getting-started.md).
