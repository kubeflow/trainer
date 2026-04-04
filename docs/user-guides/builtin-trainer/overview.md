# Overview

What is BuiltinTrainer

## What is BuiltinTrainer

The Kubeflow SDK `train()` API supports two types of trainers: `BuiltinTrainer()` and `CustomTrainer()`.

These options allow you to specify how you want to configure the TrainJob:

1. **CustomTrainer**: Use this when you need full control over the training process. It requires you to define a self-contained Python function that includes the entire model training process.
2. **BuiltinTrainer**: Designed for configuration-driven TrainJobs using a predefined training script, often tailored for tasks like LLMs fine-tuning. The training script contains entire post-training logic for LLMs fine-tuning, and it allows you to adjust the configurations for dataset, LoRA parameters, learning rates, etc. The `BuiltinTrainer` is ideal for fast iteration without modifying the training loop.

Currently, Kubeflow SDK supports these configs for `BuiltinTrainer`:

1. **TorchTuneConfig**: Configuration to fine-tune LLMs with TorchTune.

## Next Steps

- Learn how to use [TorchTune BuiltinTrainer](torchtune.md)
