# PyTorch

Train PyTorch models at scale with Kubeflow Trainer using distributed data parallel (DDP) and fully sharded data parallel (FSDP) strategies.

## Overview

Kubeflow Trainer provides seamless distributed PyTorch training on Kubernetes with minimal code changes. The `torch-distributed` runtime handles cluster orchestration, networking configuration, and environment setup automatically, allowing you to focus on your model and training logic.

Key features:
- **Automatic distributed setup**: Environment variables and networking configured automatically
- **Multiple parallelism strategies**: Support for DDP, FSDP, and custom approaches
- **GPU and CPU support**: Train on any hardware configuration
- **Batteries included**: Pre-configured with PyTorch 2.7.1, torchvision, and torchaudio

## Prerequisites

Before following this guide, ensure you have:

- Completed the [Getting Started](../getting-started/index) guide
- Access to a Kubernetes cluster with Kubeflow Trainer installed
- Basic understanding of PyTorch and distributed training concepts

## The torch-distributed Runtime

The `torch-distributed` runtime is a ClusterTrainingRuntime that provides:

- **PyTorch 2.7.1** with torchvision and torchaudio
- **Automatic environment configuration** for distributed training
- **Multi-node networking** setup via PyTorch's c10d backend
- **Flexible device support** (CPU, GPU, or mixed configurations)

### Automatic Environment Variables

Kubeflow Trainer automatically injects these environment variables into each training process:

| Variable | Description | Example |
|----------|-------------|---------|
| `WORLD_SIZE` | Total number of processes across all nodes | `4` |
| `RANK` | Global rank of the current process (0-indexed) | `0`, `1`, `2`, `3` |
| `LOCAL_RANK` | Local rank within the current node (0-indexed) | `0`, `1` |
| `MASTER_ADDR` | Address of the rank 0 node | `trainjob-mnist-0.trainjob-mnist` |
| `MASTER_PORT` | Port for distributed communication | `29500` |

You don't need to set these variables manually. They're available to your training code automatically.

## Training Function Pattern

To run distributed PyTorch training with Kubeflow Trainer, your training function must follow this pattern:

### 1. Import Inside Function Body

All imports must be placed inside the function body, not at the module level:

```python
def train_pytorch():
    """Train model with PyTorch DDP."""
    # All imports go here
    import torch
    import torch.distributed as dist
    from torch.nn.parallel import DistributedDataParallel as DDP

    # Rest of your training code...
```

:::{note}
This requirement allows the SDK to serialize and transfer your training function to the cluster without dependency conflicts.
:::

### 2. Initialize Distributed PyTorch

Call `dist.init_process_group()` once at the start of your training function:

```python
def train_pytorch():
    import torch
    import torch.distributed as dist

    # Initialize distributed backend
    # Use NCCL for GPU, Gloo for CPU
    backend = "nccl" if torch.cuda.is_available() else "gloo"
    dist.init_process_group(backend=backend)

    # Your training code...

    # Clean up at the end
    dist.destroy_process_group()
```

### 3. Use Rank Checks for Single-Process Operations

Operations like downloading datasets or printing logs should typically run on rank 0 only:

```python
def train_pytorch():
    import torch.distributed as dist
    from torchvision import datasets

    # Only rank 0 downloads the dataset
    if dist.get_rank() == 0:
        datasets.FashionMNIST(root="./data", train=True, download=True)

    # Wait for rank 0 to finish downloading
    dist.barrier()

    # All ranks can now load the dataset
    dataset = datasets.FashionMNIST(root="./data", train=True, download=False)
```

## Distributed Data Parallel (DDP)

DDP is the recommended approach for most distributed training workloads. It replicates your model across all processes and synchronizes gradients during backpropagation.

### Complete DDP Example

Here's a complete example training a CNN on Fashion-MNIST with DDP:

```python
def train_fashion_mnist_ddp():
    """Train Fashion-MNIST CNN with PyTorch DDP."""
    import torch
    import torch.nn as nn
    import torch.nn.functional as F
    import torch.optim as optim
    import torch.distributed as dist
    from torch.nn.parallel import DistributedDataParallel as DDP
    from torchvision import datasets, transforms
    from torch.utils.data import DataLoader
    from torch.utils.data.distributed import DistributedSampler

    # Configure device and distributed backend
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    backend = "nccl" if torch.cuda.is_available() else "gloo"
    dist.init_process_group(backend=backend)

    # Define CNN model
    class FashionCNN(nn.Module):
        def __init__(self):
            super(FashionCNN, self).__init__()
            self.conv1 = nn.Conv2d(1, 32, 3, 1)
            self.conv2 = nn.Conv2d(32, 64, 3, 1)
            self.dropout1 = nn.Dropout(0.25)
            self.dropout2 = nn.Dropout(0.5)
            self.fc1 = nn.Linear(9216, 128)
            self.fc2 = nn.Linear(128, 10)

        def forward(self, x):
            x = F.relu(self.conv1(x))
            x = F.relu(self.conv2(x))
            x = F.max_pool2d(x, 2)
            x = self.dropout1(x)
            x = torch.flatten(x, 1)
            x = F.relu(self.fc1(x))
            x = self.dropout2(x)
            return self.fc2(x)

    # Create model and wrap with DDP
    model = FashionCNN().to(device)
    model = DDP(model)

    # Prepare dataset with DistributedSampler
    transform = transforms.Compose([
        transforms.ToTensor(),
        transforms.Normalize((0.5,), (0.5,))
    ])

    # Only rank 0 downloads the dataset
    if dist.get_rank() == 0:
        datasets.FashionMNIST(
            root="./data",
            train=True,
            download=True,
            transform=transform
        )
    dist.barrier()

    # All ranks load the dataset
    train_dataset = datasets.FashionMNIST(
        root="./data",
        train=True,
        download=False,
        transform=transform
    )

    # DistributedSampler ensures each process gets a unique subset
    sampler = DistributedSampler(
        train_dataset,
        num_replicas=dist.get_world_size(),
        rank=dist.get_rank(),
        shuffle=True
    )

    train_loader = DataLoader(
        train_dataset,
        batch_size=64,
        sampler=sampler,
        num_workers=2,
        pin_memory=True
    )

    # Training setup
    optimizer = optim.Adam(model.parameters(), lr=0.001)
    criterion = nn.CrossEntropyLoss()

    # Training loop
    model.train()
    for epoch in range(5):
        # Set epoch for proper shuffling
        sampler.set_epoch(epoch)

        epoch_loss = 0.0
        for batch_idx, (data, target) in enumerate(train_loader):
            data, target = data.to(device), target.to(device)

            optimizer.zero_grad()
            output = model(data)
            loss = criterion(output, target)
            loss.backward()
            optimizer.step()

            epoch_loss += loss.item()

            if batch_idx % 100 == 0 and dist.get_rank() == 0:
                print(f"Epoch {epoch}, Batch {batch_idx}, Loss: {loss.item():.4f}")

        # Print epoch summary on rank 0
        if dist.get_rank() == 0:
            avg_loss = epoch_loss / len(train_loader)
            print(f"Epoch {epoch} completed. Average Loss: {avg_loss:.4f}")

    # Save model checkpoint (rank 0 only)
    if dist.get_rank() == 0:
        torch.save(model.state_dict(), "fashion_mnist_model.pth")
        print("Training completed. Model saved.")

    dist.destroy_process_group()
```

### Launching DDP Training

Use the TrainerClient SDK to launch your DDP training job:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()

job_id = client.train(
    trainer=CustomTrainer(
        func=train_fashion_mnist_ddp,
        num_nodes=4,
        resources_per_node={
            "cpu": 4,
            "memory": "16Gi",
            "gpu": 1,  # One GPU per node
        },
    )
)

print(f"Training job created: {job_id}")
```

This will launch 4 processes (one per node), each with 1 GPU. DDP will synchronize gradients across all 4 GPUs.

## Fully Sharded Data Parallel (FSDP)

FSDP is ideal for training large models that don't fit in a single GPU's memory. It shards model parameters, gradients, and optimizer states across all GPUs.

### When to Use FSDP

Use FSDP when:
- Your model is too large to fit on a single GPU
- You want to train larger models with limited GPU memory
- You need better memory efficiency than DDP

### FSDP Example

```python
def train_large_model_fsdp():
    """Train large model with PyTorch FSDP."""
    import torch
    import torch.nn as nn
    import torch.distributed as dist
    from torch.distributed.fsdp import FullyShardedDataParallel as FSDP
    from torch.distributed.fsdp.wrap import size_based_auto_wrap_policy

    # Initialize distributed training
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    dist.init_process_group(backend="nccl")

    # Create your large model
    model = YourLargeModel().to(device)

    # Wrap with FSDP
    # Auto-wrap policy shards layers above a certain parameter count
    auto_wrap_policy = size_based_auto_wrap_policy(
        min_num_params=1e8  # Shard layers with 100M+ parameters
    )

    model = FSDP(
        model,
        auto_wrap_policy=auto_wrap_policy,
        device_id=torch.cuda.current_device(),
    )

    # Training loop (similar to DDP)
    optimizer = torch.optim.Adam(model.parameters(), lr=0.001)

    for epoch in range(num_epochs):
        for batch in train_loader:
            optimizer.zero_grad()
            loss = model(batch)
            loss.backward()
            optimizer.step()

    dist.destroy_process_group()
```

:::{tip}
FSDP requires more careful configuration than DDP. See the [PyTorch FSDP documentation](https://pytorch.org/docs/stable/fsdp.html) for advanced options like mixed precision, activation checkpointing, and custom wrapping policies.
:::

## SDK Integration

The TrainerClient provides a Pythonic interface for managing distributed training:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()

# List available runtimes
for runtime in client.list_runtimes():
    print(f"Runtime: {runtime.name}")

# Launch training job
job_id = client.train(
    trainer=CustomTrainer(
        func=train_fashion_mnist_ddp,
        num_nodes=8,
        resources_per_node={
            "cpu": 8,
            "memory": "32Gi",
            "gpu": 2,
        },
    )
)

# Monitor job status
job = client.get_job(name=job_id)
print(f"Job Status: {job.status}")

for step in job.steps:
    print(f"  Step: {step.name}")
    print(f"    Status: {step.status}")
    print(f"    Devices: {step.device} x {step.device_count}")

# Stream training logs
for log_line in client.get_job_logs(job_id, follow=True):
    print(log_line)
```

## Examples and Resources

### Complete Examples

- **[PyTorch MNIST Classification](https://github.com/kubeflow/trainer/tree/master/examples/pytorch/image-classification)**: Complete example with Fashion-MNIST dataset
- **DistilBERT Fine-tuning**: Fine-tune transformer models with distributed training (coming soon)

### API Documentation

- [TrainerClient API Reference](../api-reference/python-sdk/index): Complete SDK documentation
- [TrainJob CRD Reference](../api-reference/crd-types/trainjob): TrainJob specification details

### Related Guides

- [Getting Started](../getting-started/index): Initial setup and first training job
- [Local Execution](local-execution/index): Test training locally before deploying to Kubernetes
- [DeepSpeed](deepspeed): Large-scale training with DeepSpeed ZeRO optimization

## Troubleshooting

### Common Issues

**Import errors with `CustomTrainer`:**

Ensure all imports are inside your training function body, not at the module level.

**NCCL timeout errors:**

Increase timeout or check network connectivity between nodes:

```python
import os
os.environ["NCCL_TIMEOUT"] = "1800"  # 30 minutes
```

**Out of memory (OOM) errors:**

- Reduce batch size
- Use gradient accumulation
- Switch from DDP to FSDP
- Enable mixed precision training with `torch.cuda.amp`

**Uneven workload distribution:**

Ensure you're using `DistributedSampler` and calling `sampler.set_epoch(epoch)` in your training loop.
