# Overview

Kubeflow Trainer is a **Kubernetes-native distributed AI platform** for scalable large language model (LLM) fine-tuning and training of AI models across a wide range of frameworks, including PyTorch, MLX, HuggingFace, DeepSpeed, JAX, XGBoost, and more.

## What is Kubeflow Trainer?

Kubeflow Trainer brings **MPI to Kubernetes** for multi-node, multi-GPU distributed jobs across HPC clusters. It integrates seamlessly with the Cloud Native AI ecosystem through tools like:

- **Kueue** for advanced job scheduling
- **JobSet** and **LeaderWorkerSet** for orchestration
- **Volcano** for batch scheduling
- **YuniKorn** for resource optimization

The platform features **distributed data caching** using Apache Arrow and Apache DataFusion for zero-copy tensor streaming directly to GPU nodes, maximizing training performance.

![Kubeflow Trainer Tech Stack](../images/trainer-tech-stack.drawio.svg)

## Who is This For?

Kubeflow Trainer documentation is organized around three key personas:

![User Personas](../images/user-personas.drawio.svg)

### AI Practitioners

ML engineers and data scientists who use the **Kubeflow Python SDK** and **TrainJob APIs** to train and fine-tune models at scale.

**What you'll find:**
- Training guides for PyTorch, JAX, DeepSpeed, MLX
- LLM fine-tuning blueprints with TorchTune
- Local execution backends for development
- Python SDK reference documentation

### Platform Administrators

DevOps engineers and cluster operators who **deploy and manage** Kubeflow Trainer on Kubernetes clusters.

**What you'll find:**
- Installation and configuration guides
- Runtime and policy management
- Integration with schedulers (Kueue, Volcano)
- Extension framework architecture

### Contributors

Open source developers who want to **contribute** to the Kubeflow Trainer project.

**What you'll find:**
- Architecture documentation
- Development workflow
- Contributing guidelines
- Community resources

## Why Use Kubeflow Trainer?

### Simple, Scalable, and Built for LLM Fine-Tuning

Train models with a **single Kubernetes CRD** (TrainJob) across any supported framework. Scale from single-GPU workloads to massive multi-node distributed training with minimal code changes.

### Extensible and Portable

Run anywhere: **public clouds, on-premises, or hybrid environments**. The plugin-based architecture allows custom ML policies, runtimes, and schedulers to be added without modifying the core platform.

### Distributed AI Data Caching

Optimize data loading with **Apache Arrow** and **Apache DataFusion** for high-performance, zero-copy tensor streaming. The distributed cache reduces training time by eliminating data loading bottlenecks.

### LLM Fine-Tuning Blueprints

Pre-built templates for **generative AI fine-tuning** with TorchTune, supporting popular models like Llama and Qwen. Configuration-driven workflows eliminate boilerplate code.

### Optimized GPU Efficiency

Intelligent data streaming and caching maximize **GPU utilization**, reducing training costs and time. Supports efficient model parallelism with PyTorch FSDP and DeepSpeed ZeRO.

### Native Kubernetes Integrations

First-class support for Kubernetes ecosystem tools:
- **Kueue**: Advanced queue management and fair sharing
- **Volcano**: Batch scheduling with gang scheduling
- **Coscheduling**: Resource orchestration for distributed jobs
- **YuniKorn**: Resource scheduling and multi-tenancy

![AI Lifecycle with Kubeflow Trainer](../images/ai-lifecycle-trainer.drawio.svg)

## Learn More

Watch the **KubeCon + CloudNativeCon 2024** introduction to Kubeflow Trainer:

```{raw} html
<div style="position: relative; padding-bottom: 56.25%; height: 0; overflow: hidden; max-width: 100%; margin: 2rem 0;">
  <iframe
    src="https://www.youtube.com/embed/Lgy4ir1AhYw"
    style="position: absolute; top: 0; left: 0; width: 100%; height: 100%;"
    frameborder="0"
    allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
    allowfullscreen>
  </iframe>
</div>
```



## Next Steps

Ready to get started? Run your first Kubeflow TrainJob by following the **Getting Started** guide.

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Getting Started
:link: ../getting-started/index
:link-type: doc

Install Kubeflow Trainer and run your first distributed training job
::::

::::{grid-item-card} User Guides
:link: ../user-guides/index
:link-type: doc

Learn how to train with PyTorch, JAX, DeepSpeed, MLX, and more
::::

::::{grid-item-card} Operator Guides
:link: ../operator-guides/index
:link-type: doc

Deploy and manage Kubeflow Trainer in production
::::

::::{grid-item-card} Examples Repository
:link: https://github.com/kubeflow/trainer/tree/master/examples

Explore complete training examples on GitHub
::::

:::::
