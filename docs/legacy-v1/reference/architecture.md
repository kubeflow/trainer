# Architecture

The Training Operator Architecture

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

## What is the Training Operator Architecture?

The original design was drafted in April 2021 and is [available here for reference](https://docs.google.com/document/d/1x1JPDQfDMIbnoQRftDH1IzGU0qvHGSU4W6Jl4rJLPhI/). The goal was to provide a unified Kubernetes operator that supports multiple machine learning/deep learning frameworks. This was done by having a "Frontend" operator that decomposes the job into different configurable Kubernetes components (e.g., Role, PodTemplate, Fault-Tolerance, etc.), watches all Role Customer Resources, and manages pod performance. The dedicated "Backend" operator was not implemented and instead consolidated to the "Frontend" operator.

The benefits of this approach were:

1. A Single Source of Truth (SSOT) for other Kubeflow components to interact with
2. Simpler Kubeflow releases
3. Unlocked production grade features like manifests and metadata support
4. Shared testing and release infrastructure

The V1 Training Operator architecture diagram can be seen in the diagram below:

```{image} ../images/training-operator-v1-architecture.drawio.svg
:alt: Training Operator V1 Architecture
:class: bg-white p-3
```

The diagram displays PyTorchJob and its configured communication methods but it is worth mentioning that each framework can have its own appraoch(es) to communicating across pods. Additionally, each framework can have its own set of configurable resources.

As a concrete example, PyTorch has several [Communication Backends](https://pytorch.org/docs/stable/distributed.html#torch.distributed.init_process_group) available, see the [source code documentation for the full list](https://pytorch.org/docs/stable/distributed.html#torch.distributed.init_process_group). ).
