# Kubeflow Trainer Examples

This directory contains examples for using Kubeflow Trainer with different interfaces and frameworks.

## Directory Structure

```
examples/
├── pytorch/       # PyTorch SDK examples (AI Practitioners)
├── deepspeed/     # DeepSpeed framework examples
├── mlx/           # MLX framework examples
├── torchtune/     # TorchTune fine-tuning examples
├── megatron/      # Megatron-LM examples
├── flux/          # Flux workload manager examples
├── xgboost/       # XGBoost examples
├── jax/           # JAX framework examples
├── local/         # Local backend examples
└── yaml/          # YAML examples for kubectl users (Platform Admins)
```

## For AI Practitioners (Python SDK)

Use the [Kubeflow Python SDK](https://sdk.kubeflow.org/en/latest/) for a code-first experience. Install with `pip install -U kubeflow`, then submit a TrainJob:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, TrainJobTemplate

def get_torch_dist(learning_rate: str, num_epochs: str):
    import os
    import torch
    import torch.distributed as dist

    dist.init_process_group(backend="gloo")
    print("PyTorch Distributed Environment")
    print(f"WORLD_SIZE: {dist.get_world_size()}")
    print(f"RANK: {dist.get_rank()}")
    print(f"LOCAL_RANK: {os.environ['LOCAL_RANK']}")

    lr = float(learning_rate)
    epochs = int(num_epochs)
    loss = 1.0 - (lr * 2) - (epochs * 0.01)
    if dist.get_rank() == 0:
        print(f"loss={loss}")

template = TrainJobTemplate(
    runtime="torch-distributed",
    trainer=CustomTrainer(
        func=get_torch_dist,
        func_args={"learning_rate": "0.01", "num_epochs": "5"},
        num_nodes=3,
        resources_per_node={"cpu": 2},
    ),
)

job_id = TrainerClient().train(**template)
TrainerClient().wait_for_job_status(job_id)
print("\n".join(TrainerClient().get_job_logs(name=job_id)))
```

**[Browse Python SDK Examples](./pytorch/)**

## For Platform Administrators (YAML + kubectl)

Ready-to-use YAML manifests that can be applied directly with `kubectl`:

```bash
# Multi-node distributed training
kubectl apply -f yaml/01-multi-node.yaml

# Pod customization with the runtimePatches API
kubectl apply -f yaml/02-runtime-patches.yaml
```

**[Browse YAML Examples](./yaml/)**

## Documentation

- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)
- [Getting Started Guide](https://www.kubeflow.org/docs/components/trainer/getting-started/)
- [Runtime Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Kubeflow SDK Documentation](https://sdk.kubeflow.org/en/latest/)

## Contributing

Found a bug or have a feature request? Please [open an issue](https://github.com/kubeflow/trainer/issues/new)!

Want to contribute an example? Check out our [contributing guidelines](../CONTRIBUTING.md).
