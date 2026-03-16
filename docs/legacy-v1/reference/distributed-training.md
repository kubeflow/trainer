# Distributed Training with the Training Operator

How the Training Operator performs distributed training on Kubernetes

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This page shows different distributed strategies that can be used by the Training Operator.

## Distributed Training for PyTorch

This diagram shows how the Training Operator creates PyTorch workers for the [ring all-reduce algorithm](https://tech.preferred.jp/en/blog/technologies-behind-distributed-deep-learning-allreduce/).

```{image} ../images/distributed-pytorchjob.drawio.svg
:alt: Distributed PyTorchJob
:class: bg-white p-3
```

You are responsible for writing the training code using native [PyTorch Distributed APIs](https://pytorch.org/tutorials/beginner/dist_overview.html) and creating a PyTorchJob with the required number of workers and GPUs using the Training Operator Python SDK. Then, the Training Operator creates Kubernetes pods with the appropriate environment variables for the [torchrun](https://pytorch.org/docs/stable/elastic/run.html) CLI to start the distributed PyTorch training job.

At the end of the ring all-reduce algorithm gradients are synchronized in every worker (`g1, g2, g3, g4`) and the model is trained.

You can define various distributed strategies supported by PyTorch in your training code (e.g. [PyTorch FSDP](https://pytorch.org/docs/stable/fsdp.html)), and the Training Operator will set the appropriate environment variables for`torchrun`.

## Distributed Training for TensorFlow

This diagram shows how the Training Operator creates the TensorFlow parameter server (PS) and workers for [PS distributed training](https://www.tensorflow.org/tutorials/distribute/parameter_server_training).

```{image} ../images/distributed-tfjob.drawio.svg
:alt: Distributed TFJob
:class: bg-white p-3
```

You are responsible for writing the training code using native [TensorFlow Distributed APIs](https://www.tensorflow.org/guide/distributed_training) and creating a TFJob with the required number of PSs, workers, and GPUs using the Training Operator Python SDK. Then, the Training Operator creates Kubernetes pods with the appropriate environment variables for [TF_CONFIG](https://www.tensorflow.org/guide/distributed_training#setting_up_the_tf_config_environment_variable) to start the distributed TensorFlow training job.

The Parameter server splits training data for every worker and averages model weights based on gradients produced by every worker.

You can define various [distributed strategies supported by TensorFlow](https://www.tensorflow.org/guide/distributed_training#types_of_strategies) in your training code, and the Training Operator will set the appropriate environment variables for`TF_CONFIG`.
