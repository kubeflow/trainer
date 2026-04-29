# LLM Fine-Tuning with the Training Operator

Why the Training Operator needs the fine-tuning API

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This page explains how the [Training Operator fine-tuning API](../user-guides/fine-tuning.md) fits into the Kubeflow ecosystem.

In the rapidly evolving landscape of machine learning (ML) and artificial intelligence (AI), the ability to fine-tune pre-trained models represents a significant leap towards achieving custom solutions with less effort and time. Fine-tuning allows practitioners to adapt large language models (LLMs) like BERT or GPT to their specific needs by training these models on custom datasets. This process maintains the model's architecture and learned parameters while making it more relevant to particular applications. Whether you're working in natural language processing (NLP), image classification, or another ML domain, fine-tuning can drastically improve performance and applicability of pre-existing models to new datasets and problems.

## Why does the Training Operator's Fine-Tuning API Matter ?

The introduction of the Fine-Tuning API in the Training Operator is a game-changer for ML practitioners operating within the Kubernetes ecosystem. Historically, the Training Operator has streamlined the orchestration of ML workloads on Kubernetes, making distributed training more accessible. However, fine-tuning tasks often require extensive manual intervention, including the configuration of training environments and the distribution of data across nodes. The Fine-Tuning API aims to simplify this process, offering an easy-to-use Python interface that abstracts away the complexity involved in setting up and executing fine-tuning tasks on distributed systems.

## The Rationale Behind Kubeflow's Fine-Tuning API

Implementing the Fine-Tuning API within the Training Operator is a logical step in enhancing the platform's capabilities. By providing this API, Training Operator not only simplifies the user experience for ML practitioners but also leverages its existing infrastructure for distributed training. This approach aligns with Kubeflow's mission to democratize distributed ML training, making it more accessible and less cumbersome for users. The API facilitates a seamless transition from model development to deployment, supporting the fine-tuning of LLMs on custom datasets without the need for extensive manual setup or specialized knowledge of Kubernetes internals.

## Roles and Interests

Different user personas can benefit from this feature:

MLOps Engineers: Can leverage this API to automate and streamline the setup and execution of fine-tuning tasks, reducing operational overhead.

Data Scientists: Can focus more on model experimentation and less on the logistical aspects of distributed training, speeding up the iteration cycle.

Business Owners: Can expect quicker turnaround times for tailored ML solutions, enabling faster response to market needs or operational challenges.

Platform Engineers: Can utilize this API to better operationalize the ML toolkit, ensuring scalability and efficiency in managing ML workflows.

## Next Steps

- Understand [the architecture behind train API](../reference/fine-tuning.md).
