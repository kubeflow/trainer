# LLM Fine-Tuning with Training Operator

How Training Operator performs fine-tuning on Kubernetes

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This page shows how Training Operator implements the [API to fine-tune LLMs](../user-guides/fine-tuning.md).

## Architecture

In the following diagram you can see how `train` Python API works:

```{image} ../images/fine-tune-llm-api.drawio.svg
:alt: Fine-Tune API for LLMs
:class: bg-white p-3
```

Once user executes `train` API, Training Operator creates PyTorchJob with appropriate resources to fine-tune LLM.

Storage initializer InitContainer is added to the PyTorchJob worker 0 to download pre-trained model and dataset with provided parameters.

PVC with [ReadOnlyMany access mode](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes) it attached to each PyTorchJob worker to distribute model and dataset across Pods. Note: Your Kubernetes cluster must support volumes with`ReadOnlyMany` access mode, otherwise you can use a single PyTorchJob worker.

Every PyTorchJob worker runs LLM Trainer that fine-tunes model using provided parameters.

Training Operator implements`train` API with these pre-created components:

### Model Provider

Model provider downloads pre-trained model. Currently, Training Operator supports [HuggingFace model provider](https://github.com/kubeflow/training-operator/blob/6ce4d57d699a76c3d043917bd0902c931f14080f/sdk/python/kubeflow/storage_initializer/hugging_face.py#L56) that downloads model from HuggingFace Hub.

You can implement your own model provider by using [this abstract base class](https://github.com/kubeflow/training-operator/blob/6ce4d57d699a76c3d043917bd0902c931f14080f/sdk/python/kubeflow/storage_initializer/abstract_model_provider.py#L4)

### Dataset Provider

Dataset provider downloads dataset. Currently, Training Operator supports [AWS S3](https://github.com/kubeflow/training-operator/blob/6ce4d57d699a76c3d043917bd0902c931f14080f/sdk/python/kubeflow/storage_initializer/s3.py#L37) and [HuggingFace](https://github.com/kubeflow/training-operator/blob/6ce4d57d699a76c3d043917bd0902c931f14080f/sdk/python/kubeflow/storage_initializer/hugging_face.py#L92) dataset providers.

You can implement your own dataset provider by using [this abstract base class](https://github.com/kubeflow/training-operator/blob/6ce4d57d699a76c3d043917bd0902c931f14080f/sdk/python/kubeflow/storage_initializer/abstract_dataset_provider.py)

### LLM Trainer

Trainer implements training loop to fine-tune LLM. Currently, Training Operator supports [HuggingFace trainer](https://github.com/kubeflow/training-operator/blob/6ce4d57d699a76c3d043917bd0902c931f14080f/sdk/python/kubeflow/trainer/hf_llm_training.py#L118-L139) to fine-tune LLMs.

You can implement your own trainer for other ML use-cases such as image classification, voice recognition, etc.
