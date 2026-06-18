# PyTorch on AMD ROCm Guide

This guide describes how to use TrainJob to train or fine-tune AI models with
[PyTorch](https://pytorch.org/) on AMD ROCm GPUs on Kubernetes.

---

## Prerequisites

Before exploring this guide, make sure to follow:

- [The Getting Started guide](../getting-started/index.md)
- [AMD GPU Operator documentation](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/) to set up a Kubernetes cluster with AMD GPU nodes and the `amd.com/gpu` device plugin.

---

## PyTorch on AMD ROCm Overview

PyTorch on AMD ROCm requires a different runtime environment than the default NVIDIA CUDA runtime. Specifically:

- **Image**: You must use an AMD ROCm-compatible PyTorch image (e.g., `rocm/pytorch:rocm7.1.1_ubuntu24.04_py3.12_pytorch_release_2.10.0`).
- **Resources**: You must request `amd.com/gpu` resources instead of `nvidia.com/gpu`.
- **Backend**: PyTorch `torch.distributed` with the `nccl` backend works on ROCm via [RCCL](https://github.com/ROCm/rccl), AMD's drop-in NCCL replacement. No code changes are required.

:::{note}
The built-in `torch-distributed` runtime is optimized for NVIDIA CUDA GPUs. For AMD ROCm workloads, you can override the runtime configuration using the Python SDK.
:::

---

## PyTorch Distributed Environment on ROCm

Your training script initializes `torch.distributed` the same way as with NVIDIA GPUs.
ROCm exposes AMD GPUs through the CUDA API surface, so `torch.cuda` calls work without modification.

```python
from kubeflow.trainer import TrainerClient, CustomTrainer
from kubeflow.trainer.options import kubernetes as k8s_options

def train_pytorch_rocm():
    import os
    import torch
    import torch.distributed as dist

    # Initialize distributed training — works identically on ROCm via RCCL.
    dist.init_process_group(backend="nccl")
    local_rank = int(os.environ["LOCAL_RANK"])

    # ROCm exposes AMD GPUs through the CUDA API surface.
    torch.cuda.set_device(local_rank)

    print(f"ROCm training on rank {dist.get_rank()} of {dist.get_world_size()}")
    print(f"Available AMD GPUs: {torch.cuda.device_count()}")
    print(f"Current device: {torch.cuda.current_device()}")

    dist.destroy_process_group()

client = TrainerClient()

# Define AMD GPU tolerations.
# Replace with your cluster's AMD GPU node configuration.
job_patch = k8s_options.RuntimePatch(
    training_runtime_spec=k8s_options.TrainingRuntimeSpecPatch(
        template=k8s_options.JobSetTemplatePatch(
            spec=k8s_options.JobSetSpecPatch(
                replicated_jobs=[
                    k8s_options.ReplicatedJobPatch(
                        name="node",
                        template=k8s_options.JobTemplatePatch(
                            spec=k8s_options.JobSpecPatch(
                                template=k8s_options.PodTemplatePatch(
                                    spec=k8s_options.PodSpecPatch(
                                        tolerations=[
                                            {
                                                "key": "amd.com/gpu",
                                                "operator": "Exists",
                                                "effect": "NoSchedule",
                                            },
                                        ],
                                    )
                                )
                            )
                        )
                    )
                ]
            )
        )
    )
)

# Create TrainJob
job_id = client.train(
    runtime="torch-distributed",
    trainer=CustomTrainer(
        func=train_pytorch_rocm,
        image="rocm/pytorch:rocm7.1.1_ubuntu24.04_py3.12_pytorch_release_2.10.0",
        num_nodes=2,
        resources_per_node={
            "amd.com/gpu": 1,
        },
    ),
    options=[job_patch],
)

# Wait until completion
client.wait_for_job_status(job_id)

# Logs are aggregated from node-0
print("\n".join(client.get_job_logs(name=job_id)))
```

---

## End-to-end Training Example

The following example demonstrates how to train a simple CNN on the MNIST dataset using PyTorch DDP on AMD ROCm GPUs.

```python
from kubeflow.trainer import TrainerClient, CustomTrainer
from kubeflow.trainer.options import kubernetes as k8s_options

def train_mnist_rocm():
    import os
    import torch
    import torch.distributed as dist
    import torch.nn as nn
    import torch.optim as optim
    from torch.nn.parallel import DistributedDataParallel as DDP
    from torch.utils.data import DataLoader
    from torch.utils.data.distributed import DistributedSampler
    import torchvision
    import torchvision.transforms as transforms

    # Initialize distributed training.
    dist.init_process_group(backend="nccl")
    local_rank = int(os.environ["LOCAL_RANK"])
    torch.cuda.set_device(local_rank)
    device = torch.device("cuda", local_rank)

    rank = dist.get_rank()
    world_size = dist.get_world_size()

    print(f"Process: {rank} of {world_size}")
    print(f"AMD GPU: {torch.cuda.get_device_name(local_rank)}")

    # Model definition.
    class CNN(nn.Module):
        def __init__(self):
            super().__init__()
            self.conv = nn.Sequential(
                nn.Conv2d(1, 32, 3, padding=1),
                nn.ReLU(),
                nn.MaxPool2d(2),
            )
            self.fc = nn.Sequential(
                nn.Linear(32 * 14 * 14, 128),
                nn.ReLU(),
                nn.Linear(128, 10),
            )

        def forward(self, x):
            return self.fc(self.conv(x).flatten(1))

    model = CNN().to(device)

    # Wrap model with DDP.
    # DDP synchronizes gradients across all ranks after each backward pass.
    model = DDP(model, device_ids=[local_rank])

    # Dataset — each worker gets a non-overlapping shard via DistributedSampler.
    transform = transforms.Compose([
        transforms.ToTensor(),
        transforms.Normalize((0.1307,), (0.3081,)),
    ])

    dataset = torchvision.datasets.MNIST(
        root="/tmp/data", train=True, download=True, transform=transform
    )

    sampler = DistributedSampler(dataset, num_replicas=world_size, rank=rank)
    loader = DataLoader(dataset, batch_size=128, sampler=sampler)

    # Training setup.
    optimizer = optim.Adam(model.parameters(), lr=1e-3)
    criterion = nn.CrossEntropyLoss()

    # Training loop.
    for epoch in range(5):
        sampler.set_epoch(epoch)
        total_loss = 0.0

        for images, labels in loader:
            images, labels = images.to(device), labels.to(device)
            optimizer.zero_grad()
            loss = criterion(model(images), labels)
            loss.backward()
            optimizer.step()
            total_loss += loss.item()

        if rank == 0:
            print(f"Epoch {epoch}, Loss: {total_loss / len(loader):.4f}")

    dist.destroy_process_group()

client = TrainerClient()

# Define AMD GPU tolerations.
job_patch = k8s_options.RuntimePatch(
    training_runtime_spec=k8s_options.TrainingRuntimeSpecPatch(
        template=k8s_options.JobSetTemplatePatch(
            spec=k8s_options.JobSetSpecPatch(
                replicated_jobs=[
                    k8s_options.ReplicatedJobPatch(
                        name="node",
                        template=k8s_options.JobTemplatePatch(
                            spec=k8s_options.JobSpecPatch(
                                template=k8s_options.PodTemplatePatch(
                                    spec=k8s_options.PodSpecPatch(
                                        tolerations=[
                                            {
                                                "key": "amd.com/gpu",
                                                "operator": "Exists",
                                                "effect": "NoSchedule",
                                            },
                                        ],
                                    )
                                )
                            )
                        )
                    )
                ]
            )
        )
    )
)

job_id = client.train(
    runtime="torch-distributed",
    trainer=CustomTrainer(
        func=train_mnist_rocm,
        image="rocm/pytorch:rocm7.1.1_ubuntu24.04_py3.12_pytorch_release_2.10.0",
        num_nodes=2,
        resources_per_node={
            "amd.com/gpu": 1,
        },
    ),
    options=[job_patch],
)

client.wait_for_job_status(job_id)
print("\n".join(client.get_job_logs(name=job_id)))
```

---

## ROCm Specific Configurations

### Resources

AMD ROCm GPUs are exposed in Kubernetes via the AMD GPU device plugin. Specify `amd.com/gpu`
in `resources_per_node` to request AMD GPU devices:

| Resource | Description |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `amd.com/gpu` | AMD GPU device managed by the AMD GPU Operator. Set the value to match the number of GPUs per node on your hardware (e.g., `8` for MI300X nodes). |

### Node Selector (optional)

In mixed clusters (AMD and NVIDIA GPU nodes), add a `node_selector` to ensure pods schedule exclusively on AMD GPU nodes. The AMD GPU Operator labels GPU nodes by default:

| Label | Value |
| --------------------- | ------ |
| `amd.com/gpu.present` | `true` |

### Tolerations

Depending on your cluster configuration, AMD GPU nodes may have a `NoSchedule` taint.
Add the following toleration if your nodes are tainted:

| Toleration Key | Toleration Operator | Toleration Effect |
| -------------- | ------------------- | ----------------- |
| `amd.com/gpu` | `Exists` | `NoSchedule` |

### Environment Variables

Common ROCm environment variables you may need to configure:

| Variable | Description |
| --------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `HIP_VISIBLE_DEVICES` | Controls which AMD GPUs are visible to the process. Equivalent to `CUDA_VISIBLE_DEVICES` on NVIDIA. |
| `NCCL_SOCKET_IFNAME` | Specifies the network interface RCCL uses for inter-node communication (e.g., `eth0`). |
| `HSA_FORCE_FINE_GRAIN_PCIE` | Set to `1` to enable fine-grain PCIe memory, which can improve performance in some multi-GPU configurations. |

---

## Reusable ClusterTrainingRuntime

Instead of overriding the runtime configuration via the Python SDK for every TrainJob,
cluster administrators can deploy a reusable `ClusterTrainingRuntime` for AMD ROCm GPUs.
This simplifies the user experience by moving infrastructure-specific details out of the training script.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-rocm-distributed
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 2
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  tolerations:
                    - key: "amd.com/gpu"
                      operator: "Exists"
                      effect: "NoSchedule"
                  containers:
                    - name: node
                      image: rocm/pytorch:rocm7.1.1_ubuntu24.04_py3.12_pytorch_release_2.10.0
                      resources:
                        limits:
                          amd.com/gpu: 1
```

Users can then submit TrainJobs referencing this runtime without specifying the image,
resource limits, or tolerations:

```python
job_id = client.train(
    runtime="torch-rocm-distributed",
    trainer=CustomTrainer(
        func=train_mnist_rocm,
    ),
)
```

---

## Next Steps

- Learn more about [PyTorch ROCm support](https://pytorch.org/docs/stable/notes/hip_rocm.html).
- Explore [AMD ROCm documentation](https://rocm.docs.amd.com/).
- Learn about the [AMD GPU Operator for Kubernetes](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/).
