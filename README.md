# Kubeflow Trainer

[![Join Slack](https://img.shields.io/badge/Join_Slack-blue?logo=slack)](https://www.kubeflow.org/docs/about/community/#kubeflow-slack-channels)
[![Coverage Status](https://coveralls.io/repos/github/kubeflow/trainer/badge.svg?branch=master)](https://coveralls.io/github/kubeflow/trainer?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubeflow/trainer)](https://goreportcard.com/report/github.com/kubeflow/trainer)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/10435/badge)](https://www.bestpractices.dev/projects/10435)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/kubeflow/trainer)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkubeflow%2Ftrainer.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkubeflow%2Ftrainer?ref=badge_shield)

<h1 align="center">
    <img src="./docs/images/trainer-logo.svg" alt="logo" width="200">
  <br>
</h1>

Latest News 🔥

- [2025/11] Kubeflow Trainer v2.1 is officially released with support of
  [Distributed Data Cache](https://www.kubeflow.org/docs/components/trainer/user-guides/data-cache/),
  topology aware scheduling with Kueue and Volcano, and LLM post-training enhancements. Check out
  [the GitHub release notes](https://github.com/kubeflow/trainer/releases/tag/v2.1.0).
- [2025/09] Kubeflow SDK v0.1 is officially released with support for CustomTrainer,
  BuiltinTrainer, and local PyTorch execution. Check out
  [the GitHub release notes](https://github.com/kubeflow/sdk/releases/tag/0.1.0).
- [2025/07] PyTorch on Kubernetes: Kubeflow Trainer Joins the PyTorch Ecosystem. Find the
  announcement in [the PyTorch blog post](https://pytorch.org/blog/pytorch-on-kubernetes-kubeflow-trainer-joins-the-pytorch-ecosystem/).

<details>
<summary>More</summary>

- [2025/07] Kubeflow Trainer v2.0 has been officially released. Check out
  [the blog post announcement](https://blog.kubeflow.org/trainer/intro/) and [the
  release notes](https://github.com/kubeflow/trainer/releases/tag/v2.0.0).
- [2025/04] From High Performance Computing To AI Workloads on Kubernetes: MPI Runtime in
  Kubeflow TrainJob. See the [KubeCon + CloudNativeCon London talk](https://youtu.be/Fnb1a5Kaxgo)

</details>

## Overview

Kubeflow Trainer is a Kubernetes-native distributed AI platform for scalable large language model
(LLM) fine-tuning and training of AI models across a wide range of frameworks, including
PyTorch, MLX, HuggingFace, DeepSpeed, JAX, XGBoost, and more.

Kubeflow Trainer brings MPI to Kubernetes, orchestrating multi-node, multi-GPU distributed
jobs efficiently across high-performance computing (HPC) clusters. This enables high-throughput
communication between processes, making it ideal for large-scale AI training that requires
ultra-fast synchronization between GPUs nodes.

Kubeflow Trainer seamlessly integrates with the Cloud Native AI ecosystem, including
[Kueue](https://kueue.sigs.k8s.io/docs/tasks/run/trainjobs/) for topology-aware scheduling and
multi-cluster job dispatching, as well as [JobSet](https://github.com/kubernetes-sigs/jobset) and
[LeaderWorkerSet](https://github.com/kubernetes-sigs/lws) for AI workload orchestration.

Kubeflow Trainer provides a distributed data cache designed to stream large-scale data with zero-copy
transfer directly to GPU nodes. This ensures memory-efficient training jobs while maximizing
GPU utilization.

With [the Kubeflow Python SDK](https://github.com/kubeflow/sdk), AI practitioners can effortlessly
develop and fine-tune LLMs while leveraging the Kubeflow Trainer APIs: TrainJob and Runtimes.

<h1 align="center">
    <img src="./docs/images/trainer-tech-stack.drawio.svg" alt="logo" width="500">
  <br>
</h1>

## Kubeflow Trainer Introduction

The following KubeCon + CloudNativeCon talks provides an overview of Kubeflow Trainer capabilities:

[![Kubeflow Trainer](https://img.youtube.com/vi/Lgy4ir1AhYw/0.jpg)](https://www.youtube.com/watch?v=Lgy4ir1AhYw)
[![Fine Tuning with Trainer](https://img.youtube.com/vi/O7cNlaz3Hqs/0.jpg)](https://youtu.be/O7cNlaz3Hqs)


## Getting Started

Please check [the official Kubeflow Trainer documentation](https://www.kubeflow.org/docs/components/trainer/getting-started)
to install and get started with Kubeflow Trainer.

## Community

The following links provide information on how to get involved in the community:

- Join our [`#kubeflow-trainer` Slack channel](https://www.kubeflow.org/docs/about/community/#kubeflow-slack).
- Attend [the bi-weekly AutoML and Training Working Group](https://bit.ly/2PWVCkV) community meeting.
- Check out [who is using Kubeflow Trainer](ADOPTERS.md).

## Contributing

Please refer to the [CONTRIBUTING guide](CONTRIBUTING.md).

## Changelog

Please refer to the [CHANGELOG](CHANGELOG.md).

## Kubeflow Training Operator V1

Kubeflow Trainer project is currently in <strong>alpha</strong> status, and APIs may change.
If you are using Kubeflow Training Operator V1, please refer [to this migration document](https://www.kubeflow.org/docs/components/trainer/operator-guides/migration/).

Kubeflow Community will maintain the Training Operator V1 source code at
[the `release-1.9` branch](https://github.com/kubeflow/trainer/tree/release-1.9).

You can find the documentation for Kubeflow Training Operator V1 in [these guides](https://www.kubeflow.org/docs/components/trainer/legacy-v1).

## Acknowledgement

This project was originally started as a distributed training operator for TensorFlow and later we
merged efforts from other Kubeflow Training Operators to provide a unified and simplified experience
for both users and developers. We are very grateful to all who filed issues or helped resolve them,
asked and answered questions, and were part of inspiring discussions.
We'd also like to thank everyone who's contributed to and maintained the original operators.

- PyTorch Operator: [list of contributors](https://github.com/kubeflow/pytorch-operator/graphs/contributors)
  and [maintainers](https://github.com/kubeflow/pytorch-operator/blob/master/OWNERS).
- MPI Operator: [list of contributors](https://github.com/kubeflow/mpi-operator/graphs/contributors)
  and [maintainers](https://github.com/kubeflow/mpi-operator/blob/master/OWNERS).
- XGBoost Operator: [list of contributors](https://github.com/kubeflow/xgboost-operator/graphs/contributors)
  and [maintainers](https://github.com/kubeflow/xgboost-operator/blob/master/OWNERS).
- Common library: [list of contributors](https://github.com/kubeflow/common/graphs/contributors) and
  [maintainers](https://github.com/kubeflow/common/blob/master/OWNERS).
