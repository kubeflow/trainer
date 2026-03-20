# Getting Started

Welcome to Kubeflow Trainer! This guide covers initial setup and distributed PyTorch training with Kubeflow Trainer.

## Prerequisites

Before you begin, ensure you have:

- **Kubernetes cluster** (1.31+) with Kubeflow Trainer installed
- **kubectl** configured to access your cluster
- **Python 3.9+** for SDK usage
- **Basic understanding** of Kubernetes concepts (Pods, Jobs, CRDs)

## Installation

### Install the Kubeflow Python SDK

Install the stable SDK release:

```bash
pip install -U kubeflow
```

Or install the development version from source:

```bash
pip install git+https://github.com/kubeflow/sdk.git@main
```

### Install Kubeflow Trainer on Kubernetes

For cluster administrators, see the [Installation Guide](../operator-guides/installation.md) for kubectl and Helm installation options.

## PyTorch Distributed Training Example

Let's create a complete distributed PyTorch training workflow using the Kubeflow SDK.

### Define Your Training Function

Create a training function that will run on each node in your distributed setup:

```python
def train_pytorch():
    """Train Fashion-MNIST with PyTorch DDP."""
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
    # Kubeflow Trainer automatically configures the distributed environment
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    backend = "nccl" if torch.cuda.is_available() else "gloo"
    dist.init_process_group(backend=backend)

    # Define CNN model
    class Net(nn.Module):
        def __init__(self):
            super(Net, self).__init__()
            self.conv1 = nn.Conv2d(1, 32, 3, 1)
            self.conv2 = nn.Conv2d(32, 64, 3, 1)
            self.fc1 = nn.Linear(9216, 128)
            self.fc2 = nn.Linear(128, 10)

        def forward(self, x):
            x = F.relu(self.conv1(x))
            x = F.relu(self.conv2(x))
            x = F.max_pool2d(x, 2)
            x = torch.flatten(x, 1)
            x = F.relu(self.fc1(x))
            return self.fc2(x)

    model = Net().to(device)
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
    dataset = datasets.FashionMNIST(
        root="./data",
        train=True,
        download=False,
        transform=transform
    )

    # Distribute data across nodes
    sampler = DistributedSampler(dataset)
    dataloader = DataLoader(dataset, batch_size=64, sampler=sampler)

    # Training loop
    optimizer = optim.Adam(model.parameters(), lr=0.001)
    criterion = nn.CrossEntropyLoss()

    for epoch in range(3):
        model.train()
        for batch_idx, (data, target) in enumerate(dataloader):
            data, target = data.to(device), target.to(device)
            optimizer.zero_grad()
            output = model(data)
            loss = criterion(output, target)
            loss.backward()
            optimizer.step()

            if batch_idx % 100 == 0:
                print(f"Epoch {epoch}, Batch {batch_idx}, Loss: {loss.item():.4f}")

    dist.destroy_process_group()
```

### List Available Training Runtimes

Check which runtimes are available in your cluster:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()

for runtime in client.list_runtimes():
    print(f"Runtime: {runtime.name}")
```

Output:
```
Runtime: torch-distributed
```

### Create and Submit a Training Job

Launch a 4-node distributed training job with GPU resources:

```python
job_id = client.train(
    trainer=CustomTrainer(
        func=train_pytorch,
        num_nodes=4,
        resources_per_node={
            "cpu": 3,
            "memory": "16Gi",
            "gpu": 1,
        },
    )
)

print(f"Training job created: {job_id}")
```

### Monitor Job Status

Track the status of your training steps:

```python
job = client.get_job(name=job_id)

for step in job.steps:
    print(f"Step: {step.name}")
    print(f"  Status: {step.status}")
    print(f"  Devices: {step.device} x {step.device_count}")
```

### View Training Logs

Stream logs from your training job:

```python
for log_line in client.get_job_logs(job_id, follow=True):
    print(log_line)
```

## What's Next?

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} User Guides
:link: ../user-guides/index
:link-type: doc

**Documentation for AI practitioners of Kubeflow Trainer**

Train models with PyTorch, JAX, DeepSpeed, MLX, and use builtin trainers for LLM fine-tuning. Develop locally before deploying to Kubernetes.
::::

::::{grid-item-card} Operator Guides
:link: ../operator-guides/index
:link-type: doc

**Documentation for cluster operators of Kubeflow Trainer**

Install and configure Kubeflow Trainer, manage runtimes and ML policies, and integrate with job schedulers like Kueue and Volcano.
::::

::::{grid-item-card} Contributor Guides
:link: ../contributor-guides/index
:link-type: doc

**Documentation for Kubeflow Trainer contributors**

Learn the architecture, development workflow, and how to contribute code and build custom plugins for Kubeflow Trainer.
::::

::::{grid-item-card} Legacy Kubeflow Training Operator (v1)
:link: ../legacy-v1/index
:link-type: doc

**Kubeflow Training Operator v1 Documentation**

Archived documentation for the deprecated v1 operator, including user guides, installation, and migration guidance to v2.
::::

:::::
