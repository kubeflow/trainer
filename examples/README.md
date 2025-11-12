# Kubeflow Trainer Examples

Welcome to Kubeflow Trainer examples!

This directory contains examples for using Kubeflow Trainer with different interfaces and frameworks.

## Directory Structure

```
examples/
├── yaml/          # YAML examples for kubectl users (Platform Admins)
│   ├── basic/     # Simple getting started examples
│   └── advanced/  # Production-ready configurations
├── pytorch/       # PyTorch SDK examples (AI Practitioners)
├── deepspeed/     # DeepSpeed framework examples
├── mlx/           # MLX framework examples
└── torchtune/     # TorchTune fine-tuning examples
```

## Quick Start

### For Platform Administrators (YAML + kubectl)

If you prefer working with Kubernetes YAML files and `kubectl`, check out the [YAML examples](./yaml/):

```bash
# Simple hello-world example
kubectl apply -f yaml/basic/01-hello-world.yaml

# Multi-node distributed training
kubectl apply -f yaml/basic/02-multi-node.yaml

# Production with PodSpec overrides
kubectl apply -f yaml/advanced/01-podspec-overrides.yaml
```

👉 **[Browse YAML Examples](./yaml/)**

### For AI Practitioners (Python SDK)

If you prefer Python and want to focus on your training code without dealing with YAML, use the Kubeflow Python SDK:

```python
from kubeflow.trainer import TrainJob

# Your training code
def train():
    # Your PyTorch, TensorFlow, or JAX code here
    pass

# Create and submit TrainJob
train_job = TrainJob(
    name="my-training-job",
    num_nodes=2,
    entrypoint=train,
)
train_job.create()
```

👉 **[Browse Python SDK Examples](./pytorch/)**

## Example Categories

### YAML Examples (kubectl)

Perfect for:
- Platform administrators
- CI/CD pipelines
- GitOps workflows
- Kubernetes-native development

**Available examples:**
- ✅ Hello World (no GPU needed)
- ✅ Multi-node distributed training
- ✅ Custom TrainingRuntime
- ✅ PyTorch MNIST training
- ✅ PodSpec overrides
- ✅ Kueue integration
- ✅ Volcano gang scheduling
- ✅ Multi-step pipelines

**[View YAML Examples →](./yaml/)**

### Python SDK Examples

Perfect for:
- AI practitioners and data scientists
- Rapid experimentation
- Notebook-based development
- Framework-specific features

**Available frameworks:**
- PyTorch
- DeepSpeed
- MLX
- TorchTune

**[View Python Examples →](./pytorch/)**

## Documentation

The comprehensive Kubeflow Trainer documentation is available on [kubeflow.org](https://www.kubeflow.org/docs/components/trainer/).

Key resources:
- [Getting Started Guide](https://www.kubeflow.org/docs/components/trainer/getting-started/)
- [Runtime Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Migration Guide (v1 to v2)](https://www.kubeflow.org/docs/components/trainer/operator-guides/migration/)
- [Python SDK Reference](https://www.kubeflow.org/docs/components/trainer/user-guides/builtin-trainer/overview/)

## Contributing

Found a bug or have a feature request? Please [open an issue](https://github.com/kubeflow/training-operator/issues/new)!

Want to contribute an example? Check out our [contributing guidelines](../CONTRIBUTING.md).
