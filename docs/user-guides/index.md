# User Guides

*Documentation for AI practitioners and ML engineers using Kubeflow Trainer for distributed training.*

This section contains guides for running distributed training workloads with various ML frameworks using Kubeflow Trainer.

----

## Distributed Training Frameworks

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} PyTorch
:link: pytorch
:link-type: doc

Distributed PyTorch training with FSDP, DDP, and more
::::

::::{grid-item-card} PyTorch on AMD ROCm
:link: pytorch-rocm
:link-type: doc

PyTorch distributed training on AMD ROCm GPUs
::::

::::{grid-item-card} JAX
:link: jax
:link-type: doc

Distributed JAX training with jax.distributed
::::

::::{grid-item-card} JAX on TPU
:link: jax-tpu
:link-type: doc

JAX distributed training on Google Cloud TPUs
::::

::::{grid-item-card} DeepSpeed
:link: deepspeed
:link-type: doc

Large-scale training with DeepSpeed ZeRO optimization
::::

::::{grid-item-card} XGBoost
:link: xgboost
:link-type: doc

Distributed XGBoost training on Kubernetes
::::

::::{grid-item-card} Megatron
:link: megatron
:link-type: doc

Megatron-Core with Tensor Parallelism for large transformers
::::

::::{grid-item-card} MLX
:link: mlx
:link-type: doc

Training on Apple Silicon with MLX framework
::::

::::{grid-item-card} Flux
:link: flux
:link-type: doc

HPC workloads with Flux Framework integration
::::

:::::

## Data and Fine-Tuning

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Distributed Data Cache
:link: data-cache
:link-type: doc

High-performance distributed data caching for training
::::

::::{grid-item-card} Builtin Trainers
:link: builtin-trainer/index
:link-type: doc

Pre-built training workflows (TorchTune and more)
::::

:::::

## Job Lifecycle

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Configure TrainJob Lifecycle
:link: trainjob-lifecycle
:link-type: doc

Active deadlines, suspend/resume for TrainJobs
::::

:::::

## Local Development

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Local Execution Overview
:link: local-execution/index
:link-type: doc

Run TrainJobs locally before deploying to Kubernetes
::::

::::{grid-item-card} Docker Backend
:link: local-execution/docker
:link-type: doc

Execute training jobs in Docker containers locally
::::

::::{grid-item-card} Podman Backend
:link: local-execution/podman
:link-type: doc

Execute training jobs with Podman (Docker alternative)
::::

::::{grid-item-card} Process Backend
:link: local-execution/local-process
:link-type: doc

Run training jobs as local processes for quick iteration
::::

:::::

----

```{toctree}
:hidden:
:maxdepth: 2

pytorch
pytorch-rocm
jax
jax-tpu
deepspeed
mlx
xgboost
megatron
flux
data-cache
builtin-trainer/index
local-execution/index
trainjob-lifecycle
```
