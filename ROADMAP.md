# Roadmap

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
