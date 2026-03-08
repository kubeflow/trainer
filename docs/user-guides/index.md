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

::::{grid-item-card} JAX
:link: jax
:link-type: doc

Distributed JAX training with jax.distributed
::::

::::{grid-item-card} DeepSpeed
:link: deepspeed
:link-type: doc

Large-scale training with DeepSpeed ZeRO optimization
::::

::::{grid-item-card} MLX
:link: mlx
:link-type: doc

Training on Apple Silicon with MLX framework
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
jax
deepspeed
mlx
data-cache
builtin-trainer/index
local-execution/index
```
