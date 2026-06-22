# Overview

An overview of the Training Operator

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../operator-guides/migration.md).
:::

## What is the Training Operator

The Training Operator is a Kubernetes-native project for fine-tuning and scalable
distributed training of machine learning (ML) models created with different ML frameworks such as
PyTorch, TensorFlow, XGBoost, JAX, and others.

You can integrate other ML libraries such as [HuggingFace](https://huggingface.co),
[DeepSpeed](https://github.com/microsoft/DeepSpeed), or [Megatron-LM](https://github.com/NVIDIA/Megatron-LM)
with the Training Operator to orchestrate their ML training on Kubernetes.

The Training Operator allows you to use Kubernetes workloads to effectively train your large models
via Kubernetes Custom Resources APIs or using the Training Operator Python SDK.

The Training Operator implements a centralized Kubernetes controller to orchestrate distributed training jobs.

You can run high-performance computing (HPC) tasks with the Training Operator and MPIJob since it
supports running Message Passing Interface (MPI) on Kubernetes which is heavily used for HPC.
The Training Operator implements the V1 API version of MPI Operator. For the MPI Operator V2 version,
please follow [this guide](user-guides/mpi.md) to install MPI Operator V2.

```{image} images/training-operator-overview.drawio.svg
:alt: Training Operator Overview
:class: bg-white p-3
```

The Training Operator is responsible for scheduling the appropriate Kubernetes workloads to implement
various distributed training strategies for different ML frameworks.

## Why use the Training Operator

The Training Operator addresses the Model Training and Model Fine-Tuning steps in the AI/ML
lifecycle as shown in diagram below:

```{image} images/ml-lifecycle-training-operator.drawio.svg
:alt: AI/ML Lifecycle Training Operator
:class: bg-white p-3
```

- **The Training Operator simplifies the ability to run distributed training and fine-tuning.**

You can easily scale their model training from single machine to large-scale distributed
Kubernetes cluster using APIs and interfaces provided by Training Operator.

- **The Training Operator is extensible and portable.**

You can deploy the Training Operator on any cloud where you have Kubernetes cluster and you can
integrate their own ML frameworks written in any programming languages with Training Operator.

- **The Training Operator is integrated with the Kubernetes ecosystem.**

You can leverage Kubernetes advanced scheduling techniques such as Kueue, Volcano, and YuniKorn
with the Training Operator to optimize cost savings for your ML training resources.

## Custom Resources for ML Frameworks

To perform distributed training the Training Operator implements the following
[Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
for each ML framework:

| ML Framework | Custom Resource |
| --- | --- |
| PyTorch | [PyTorchJob](user-guides/pytorch.md) |
| TensorFlow | [TFJob](user-guides/tensorflow.md) |
| XGBoost | [XGBoostJob](user-guides/xgboost.md) |
| MPI | [MPIJob](user-guides/mpi.md) |
| PaddlePaddle | [PaddleJob](user-guides/paddlepaddle.md) |
| JAX | [JAXJob](user-guides/jax.md) |

## Next steps

Follow [the installation guide](installation.md) to deploy the Training Operator.

Run examples from [getting started guide](getting-started.md).
