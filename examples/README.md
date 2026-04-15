# Kubeflow Trainer Examples

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

## For Platform Administrators (YAML + kubectl)

Ready-to-use YAML examples that can be applied directly with `kubectl`:

```bash
# Multi-node distributed training
kubectl apply -f yaml/basic/01-multi-node.yaml

# Production with PodSpec overrides
kubectl apply -f yaml/advanced/01-podspec-overrides.yaml
```

**[Browse YAML Examples](./yaml/)**

## For AI Practitioners (Python SDK)

Use the Kubeflow Python SDK for a code-first experience:

```python
from kubeflow.trainer import TrainJob

train_job = TrainJob(
    name="my-training-job",
    num_nodes=2,
    entrypoint=train,
)
train_job.create()
```

**[Browse Python SDK Examples](./pytorch/)**

## Documentation

- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)
- [Getting Started Guide](https://www.kubeflow.org/docs/components/trainer/getting-started/)
- [Runtime Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)

## Contributing

Found a bug or have a feature request? Please [open an issue](https://github.com/kubeflow/trainer/issues/new)!

Want to contribute an example? Check out our [contributing guidelines](../CONTRIBUTING.md).
