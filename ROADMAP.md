# Kubeflow Trainer ROADMAP

## 2025

- Kubeflow Trainer v2 general availability: https://github.com/kubeflow/trainer/issues/2170
- Local execution for Kubeflow Python SDK: https://github.com/kubeflow/sdk/issues/22
- Distributed in-memory data cache powered by Apache Arrow and Apache DataFusion: https://github.com/kubeflow/trainer/issues/2655
- `BuiltinTrainers` for LLMs Fine-Tuning
  - TorchTune support: https://github.com/kubeflow/trainer/issues/2401
  - Explore other libraries for fine-tuning like [Unsloth](https://github.com/unslothai/unsloth), [LLaMA-Factory](https://github.com/hiyouga/LLaMA-Factory), [HuggingFace TRL](https://github.com/huggingface/trl): https://github.com/kubeflow/trainer/issues/2752
  - Design extensible architecture for `BuiltinTrainers`
- Training Runtime support
  - PyTorch: https://github.com/kubeflow/trainer/issues/2211
  - DeepSpeed: https://github.com/kubeflow/trainer/issues/2517
  - MLX: https://github.com/kubeflow/trainer/issues/2047
  - JAX: https://github.com/kubeflow/trainer/issues/2442
- Elastic PyTorch training jobs: https://github.com/kubernetes-sigs/jobset/issues/463
- Gang-scheduling capability for TrainJob
  - Coscheduling: https://github.com/kubeflow/trainer/pull/2248
  - Kueue: https://github.com/kubernetes-sigs/kueue/issues/5719
  - Volcano: https://github.com/kubeflow/trainer/issues/2671
  - KAI Scheduler: https://github.com/kubeflow/trainer/issues/2628
- Multi-cluster TrainJob dispatching with [Multi-Kueue](https://kueue.sigs.k8s.io/docs/concepts/multikueue/).
- Topology aware scheduling with [Kueue](https://kueue.sigs.k8s.io/docs/concepts/topology_aware_scheduling/).
- Implement registration mechanism in the Pipeline Framework to extend plugins and supported ML
  frameworks in the Kubeflow Trainer: https://github.com/kubeflow/trainer/issues/2750
- Enhanced MPI orchestration with SSH-based node communication: https://github.com/kubeflow/trainer/issues/2751
- GPU testing infrastructure: https://github.com/kubeflow/trainer/issues/2432
- Automation checkpointing for GPU-accelerated TrainJobs: https://github.com/kubeflow/trainer/issues/2245
- Automation of Kubeflow Trainer releases: https://github.com/kubeflow/trainer/issues/2155
- Kubeflow Trainer UI and TrainJob History Server: https://github.com/kubeflow/trainer/issues/2648

## 2023/2024

- Training Operator V2
- Enhance JobSet APIs for distributed training and fine-tuning
- Kubeflow Training SDK improvements
- Support for distributed JAX
- Support for LLM Training runtimes
- Python APIs for LLMs fine-tuning
- Consolidate MPI Operator V2 into Training Operator

## 2022

- Release training-operator v1.4 to be included in Kubeflow v1.5 release.
- Migrate v2 MPI operator to unified operator.
- Migrate PaddlePaddle operator to unified operator.
- Support elastic training for additional frameworks besides PyTorch.
- Support different gang scheduling definitions.
- Improve test coverage.

## 2020 and 2021

### Maintenance and reliability

We will continue developing capabilities for better reliability, scaling, and maintenance of production distributed training experiences provided by operators.

- Enhance maintainability of operator common module. Related issue: [#54](https://github.com/kubeflow/common/issues/54).
- Migrate operators to use [kubeflow/common](https://github.com/kubeflow/common) APIs. Related issue: [#64](https://github.com/kubeflow/common/issues/64).
- Graduate MPI Operator, MXNet Operator and XGBoost Operator to v1. Related issue: [#65](https://github.com/kubeflow/common/issues/65).

### Features

To take advantages of other capabilities of job scheduler components, operators will expose more APIs for advanced scheduling. More features will be added to simplify usage like dynamic volume supports and git ops experiences. In order to make it easily used in the Kubeflow ecosystem, we can add more launcher KFP components for adoption.

- Support dynamic volume provisioning for distributed training jobs. Related issue: [#19](https://github.com/kubeflow/common/issues/19).
- MLOps - Allow user to submit jobs using Git repo without building container images. Related issue: [#66](https://github.com/kubeflow/common/issues/66).
- Add Job priority and Queue in SchedulingPolicy for advanced scheduling in common operator. Related issue: [#46](https://github.com/kubeflow/common/issues/46).
- Add pipeline launcher components for different training jobs. Related issue: [pipeline#3445](https://github.com/kubeflow/pipelines/issues/3445).

### Monitoring

- Provides a standardized logging interface. Related issue: [#60](https://github.com/kubeflow/common/issues/60).
- Expose generic prometheus metrics in common operators. Related issue: [#22](https://github.com/kubeflow/common/issues/22).
- Centralized Job Dashboard for training jobs (Add metadata graph, model artifacts later). Related issue: [#67](https://github.com/kubeflow/common/issues/67).

### Performance

Continue to optimize reconciler performance and reduce latency to take actions on CR events.

- Performance optimization for 500 concurrent jobs and large scale completed jobs. Related issues: [#68](https://github.com/kubeflow/common/issues/68), [tf-operator#965](https://github.com/kubeflow/tf-operator/issues/965), and [tf-operator#1079](https://github.com/kubeflow/tf-operator/issues/1079).

### Quarterly Goals

#### Q1 & Q2

- Better log support
  - Support log levels [#1132](https://github.com/kubeflow/training-operator/issues/1132)
  - Log errors in events
- Validating webhook [#1016](https://github.com/kubeflow/training-operator/issues/1016)

#### Q3 & Q4

- Better Volcano support
  - Support queue [#916](https://github.com/kubeflow/training-operator/issues/916)
