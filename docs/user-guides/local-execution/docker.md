# Docker Backend

Execute distributed TrainJobs in isolated Docker containers for reproducible local development and multi-node testing.

## Overview

The Docker Backend enables execution of distributed TrainJobs in isolated Docker containers locally. This provides a containerized environment that closely matches production while supporting multi-node distributed training simulation.

Key features:
- **Container isolation**: Each TrainJob runs in isolated Docker containers with separate filesystem, network, and resources
- **Multi-node support**: Distributed training across multiple containers with automatic networking
- **Reproducibility**: Consistent containerized environments matching production
- **Flexible configuration**: Customizable image policies and resource allocation
- **Wide compatibility**: Works on macOS, Windows, and Linux

## When to Use

**Use Docker Backend for:**
- Testing distributed training logic locally
- Simulating multi-node training environments
- Ensuring code works in containers before production
- Reproducible development environments
- General-purpose local development (especially on macOS/Windows)

**Don't use Docker Backend if:**
- You need rootless containers (use [Podman Backend](podman))
- You want fastest iteration (use [Local Process Backend](local-process))
- Docker is not available in your environment

## Prerequisites

### Required Software

1. **Docker Desktop** (macOS/Windows) or **Docker Engine** (Linux)

   **macOS/Windows:**
   - Download from [docker.com](https://www.docker.com/products/docker-desktop)
   - Install and start Docker Desktop

   **Linux:**
   - Follow [official Docker Engine installation](https://docs.docker.com/engine/install/)

2. **Python 3.9+**

3. **Kubeflow SDK with Docker support:**
   ```bash
   pip install "kubeflow[docker]"
   ```

### Verification

Verify Docker is running:

```bash
# Check Docker version
docker version

# Test Docker is working
docker ps

# Verify you can pull images
docker pull hello-world
docker run --rm hello-world
```

Expected output should show Docker client and server versions, and the hello-world container should run successfully.

## Configuration

### Basic Configuration

```python
from kubeflow.trainer import TrainerClient, ContainerBackendConfig

# Configure Docker backend
backend_config = ContainerBackendConfig(
    container_runtime="docker",
)

# Initialize client
client = TrainerClient(backend_config=backend_config)
```

### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `container_runtime` | str | None | Force "docker", "podman", or None (auto-detect) |
| `pull_policy` | str | "IfNotPresent" | Image pull policy: "IfNotPresent", "Always", "Never" |
| `auto_remove` | bool | True | Auto-remove containers/networks after completion |
| `container_host` | str | None | Override Docker daemon connection URL |
| `runtime_source` | TrainingRuntimeSource | GitHub | Custom runtime configuration source |

### Advanced Configuration

```python
from kubeflow.trainer import ContainerBackendConfig

# Always pull latest images
config = ContainerBackendConfig(
    container_runtime="docker",
    pull_policy="Always",
)

# Keep containers for debugging
config = ContainerBackendConfig(
    container_runtime="docker",
    auto_remove=False,
)

# Custom Docker daemon
config = ContainerBackendConfig(
    container_runtime="docker",
    container_host="tcp://192.168.1.100:2375",
)
```

## Basic Usage

### Simple Training Example

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def train_simple_model():
    """Simple training in Docker container."""
    import torch
    import os

    rank = int(os.environ.get('RANK', '0'))
    world_size = int(os.environ.get('WORLD_SIZE', '1'))

    print(f"Training on rank {rank}/{world_size}")

    model = torch.nn.Linear(10, 1)
    optimizer = torch.optim.SGD(model.parameters(), lr=0.01)

    for epoch in range(5):
        loss = torch.nn.functional.mse_loss(
            model(torch.randn(32, 10)),
            torch.randn(32, 1)
        )
        optimizer.zero_grad()
        loss.backward()
        optimizer.step()

        print(f"[Rank {rank}] Epoch {epoch + 1}/5, Loss: {loss.item():.4f}")

    print(f"[Rank {rank}] Training completed!")

# Configure Docker backend
backend_config = ContainerBackendConfig(
    container_runtime="docker",
    pull_policy="IfNotPresent",
    auto_remove=True
)

client = TrainerClient(backend_config=backend_config)

# Launch training
job_id = client.train(
    trainer=CustomTrainer(
        func=train_simple_model,
        num_nodes=2,  # 2 containers
    )
)

print(f"Training job started: {job_id}")

# Wait for completion
job = client.wait_for_job_status(job_id)
print(f"Job completed with status: {job.status}")
```

## Multi-Node Distributed Training

The Docker backend automatically sets up networking for multi-node training:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def distributed_train():
    """Distributed training across multiple Docker containers."""
    import os
    import torch
    import torch.distributed as dist
    from torch.nn.parallel import DistributedDataParallel as DDP

    # Get distributed info
    rank = int(os.environ['RANK'])
    world_size = int(os.environ['WORLD_SIZE'])
    local_rank = int(os.environ.get('LOCAL_RANK', '0'))

    print(f"Initializing process group: rank={rank}, world_size={world_size}")

    # Initialize distributed backend
    dist.init_process_group(
        backend='gloo',  # Use 'gloo' for CPU or 'nccl' for GPU
        rank=rank,
        world_size=world_size
    )

    # Create model
    model = torch.nn.Linear(10, 1)

    # Wrap with DDP
    ddp_model = DDP(model)

    optimizer = torch.optim.SGD(ddp_model.parameters(), lr=0.01)

    # Training loop
    for epoch in range(5):
        # Generate data
        inputs = torch.randn(32, 10)
        targets = torch.randn(32, 1)

        # Forward pass
        optimizer.zero_grad()
        outputs = ddp_model(inputs)
        loss = torch.nn.functional.mse_loss(outputs, targets)

        # Backward pass (gradients automatically synchronized)
        loss.backward()
        optimizer.step()

        print(f"[Rank {rank}] Epoch {epoch + 1}/5, Loss: {loss.item():.4f}")

    # Clean up
    dist.destroy_process_group()
    print(f"[Rank {rank}] Training complete")

# Configure backend
backend_config = ContainerBackendConfig(container_runtime="docker")
client = TrainerClient(backend_config=backend_config)

# Launch 4-node distributed training
job_id = client.train(
    trainer=CustomTrainer(
        func=distributed_train,
        num_nodes=4,  # 4 containers simulating 4 nodes
    )
)

print(f"Distributed training job started: {job_id}")

# Stream logs from all nodes
for node_idx in range(4):
    print(f"\n=== Node {node_idx} Logs ===")
    for log in client.get_job_logs(job_id, node_index=node_idx):
        print(log)
```

### Networking Architecture

The Docker backend creates networking automatically:

1. **Network Creation**: Dedicated Docker network with DNS enabled
2. **Master Node Discovery**: Rank-0 container launched first, IP address captured
3. **Environment Variables**: `MASTER_ADDR` set to rank-0 IP for all containers
4. **Container Communication**: All containers on same network can communicate

```python
def test_networking():
    """Test Docker networking between containers."""
    import os
    import socket
    import subprocess

    rank = int(os.environ['RANK'])
    master_addr = os.environ['MASTER_ADDR']

    print(f"Rank {rank}: Hostname = {socket.gethostname()}")
    print(f"Rank {rank}: Master IP = {master_addr}")

    # Test connectivity to master
    if rank != 0:
        result = subprocess.run(
            ['ping', '-c', '1', master_addr],
            capture_output=True
        )
        print(f"Rank {rank}: Ping to master = {'Success' if result.returncode == 0 else 'Failed'}")

# Test with multiple nodes
job_id = client.train(
    trainer=CustomTrainer(
        func=test_networking,
        num_nodes=4,
    )
)
```

## Complete Training Example

Here's a complete example training a CNN with data parallelism:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def train_cnn_distributed():
    """Train CNN with distributed data parallelism."""
    import os
    import torch
    import torch.nn as nn
    import torch.nn.functional as F
    import torch.distributed as dist
    from torch.nn.parallel import DistributedDataParallel as DDP
    from torchvision import datasets, transforms
    from torch.utils.data import DataLoader
    from torch.utils.data.distributed import DistributedSampler

    # Get distributed configuration
    rank = int(os.environ['RANK'])
    world_size = int(os.environ['WORLD_SIZE'])

    if rank == 0:
        print(f"Starting distributed training with {world_size} nodes")

    # Initialize distributed
    dist.init_process_group(backend='gloo')

    # Define CNN model
    class SimpleCNN(nn.Module):
        def __init__(self):
            super(SimpleCNN, self).__init__()
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
    model = SimpleCNN()
    ddp_model = DDP(model)

    # Prepare dataset
    transform = transforms.Compose([
        transforms.ToTensor(),
        transforms.Normalize((0.5,), (0.5,))
    ])

    # Only rank 0 downloads
    if rank == 0:
        datasets.FashionMNIST(
            root="./data",
            train=True,
            download=True,
            transform=transform
        )
    dist.barrier()

    # All ranks load dataset
    train_dataset = datasets.FashionMNIST(
        root="./data",
        train=True,
        download=False,
        transform=transform
    )

    # Distributed sampler
    sampler = DistributedSampler(
        train_dataset,
        num_replicas=world_size,
        rank=rank,
        shuffle=True
    )

    train_loader = DataLoader(
        train_dataset,
        batch_size=64,
        sampler=sampler,
        num_workers=2
    )

    # Training setup
    optimizer = torch.optim.Adam(ddp_model.parameters(), lr=0.001)
    criterion = nn.CrossEntropyLoss()

    # Training loop
    num_epochs = 3
    if rank == 0:
        print(f"Starting training for {num_epochs} epochs...")

    for epoch in range(num_epochs):
        ddp_model.train()
        sampler.set_epoch(epoch)

        epoch_loss = 0.0
        for batch_idx, (data, target) in enumerate(train_loader):
            optimizer.zero_grad()
            output = ddp_model(data)
            loss = criterion(output, target)
            loss.backward()
            optimizer.step()

            epoch_loss += loss.item()

            if batch_idx % 100 == 0 and rank == 0:
                print(f"Epoch {epoch}, Batch {batch_idx}, Loss: {loss.item():.4f}")

        if rank == 0:
            avg_loss = epoch_loss / len(train_loader)
            print(f"Epoch {epoch} completed. Average Loss: {avg_loss:.4f}")

    # Save model (rank 0 only)
    if rank == 0:
        torch.save(ddp_model.state_dict(), "cnn_model.pth")
        print("Training completed. Model saved.")

    dist.destroy_process_group()

# Configure and launch
backend_config = ContainerBackendConfig(container_runtime="docker")
client = TrainerClient(backend_config=backend_config)

job_id = client.train(
    trainer=CustomTrainer(
        func=train_cnn_distributed,
        num_nodes=4,
    )
)

print(f"Training job: {job_id}")

# Monitor training
for log in client.get_job_logs(job_id, follow=True):
    print(log)
```

## Job Management

### Inspecting Containers

```bash
# List containers for a job
docker ps -a --filter "label=kubeflow.org/job-name=<job-name>"

# Inspect specific container
docker inspect <job-name>-node-0

# View container logs
docker logs <job-name>-node-0

# Follow logs
docker logs -f <job-name>-node-0

# Execute commands in running container
docker exec -it <job-name>-node-0 /bin/bash
```

### Using TrainerClient

```python
# List all jobs
jobs = client.list_jobs()
for job in jobs:
    print(f"Job: {job.name}, Status: {job.status}")

# Get job logs
logs = client.get_job_logs(job_id, node_index=0)
for log in logs:
    print(log)

# Follow logs in real-time
for log in client.get_job_logs(job_id, follow=True):
    print(log)

# Wait for completion
job = client.wait_for_job_status(job_id)
print(f"Status: {job.status}")

# Delete job
client.delete_job(job_id)
```

## GPU Support

The Docker backend supports GPU training with NVIDIA Container Toolkit:

### Prerequisites

1. **NVIDIA Drivers**: Install latest NVIDIA drivers

```bash
# Check NVIDIA drivers
nvidia-smi
```

2. **NVIDIA Container Toolkit**:

```bash
# Install (Ubuntu/Debian)
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | \
    sudo tee /etc/apt/sources.list.d/nvidia-docker.list

sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker
```

3. **Verify GPU support**:

```bash
docker run --rm --gpus all nvidia/cuda:11.8.0-base-ubuntu22.04 nvidia-smi
```

### GPU Training Example

```python
def train_with_gpu():
    """Training with GPU in Docker container."""
    import torch

    # Check GPU
    if torch.cuda.is_available():
        device = torch.device("cuda")
        print(f"Using GPU: {torch.cuda.get_device_name(0)}")
    else:
        device = torch.device("cpu")
        print("GPU not available, using CPU")

    # Create model on GPU
    model = torch.nn.Linear(1000, 10).to(device)
    optimizer = torch.optim.Adam(model.parameters())

    # Training loop
    for epoch in range(10):
        inputs = torch.randn(64, 1000).to(device)
        targets = torch.randint(0, 10, (64,)).to(device)

        outputs = model(inputs)
        loss = torch.nn.functional.cross_entropy(outputs, targets)

        optimizer.zero_grad()
        loss.backward()
        optimizer.step()

        print(f"Epoch {epoch + 1}, Loss: {loss.item():.4f}")

# Request GPU resources
job_id = client.train(
    trainer=CustomTrainer(
        func=train_with_gpu,
        resources_per_node={"gpu": "1"},
    )
)
```

## Examples and Resources

### Complete Examples

- **[MNIST Classification](https://github.com/kubeflow/trainer/tree/master/examples/pytorch/mnist)**: Complete notebook demonstrating Docker backend
- **Distributed CNN Training**: See [complete example](#complete-training-example) above

### API Documentation

- [TrainerClient API Reference](../../api-reference/python-sdk/index): Complete SDK documentation
- [ContainerBackendConfig API](../../api-reference/python-sdk/backends): Backend configuration reference

### Related Guides

- [Local Execution Overview](index): Overview of all backends
- [Podman Backend](podman): Rootless container alternative
- [Local Process Backend](local-process): Fastest local development

## Troubleshooting

### Docker Daemon Not Running

**Error:** `Cannot connect to the Docker daemon`

**Solution:**

```bash
# Start Docker Desktop (macOS/Windows)
# Or start Docker service (Linux)
sudo systemctl start docker

# Verify
docker ps
```

### Permission Denied (Linux)

**Error:** `Got permission denied while trying to connect to the Docker daemon socket`

**Solution:**

```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Apply group changes
newgrp docker

# Verify
docker ps
```

### GPU Not Available

**Error:** Training runs on CPU instead of GPU

**Solution:**

```bash
# Verify NVIDIA drivers
nvidia-smi

# Verify Container Toolkit
docker run --rm --gpus all nvidia/cuda:11.8.0-base-ubuntu22.04 nvidia-smi

# Request GPU in trainer
trainer = CustomTrainer(
    func=train_with_gpu,
    resources_per_node={"gpu": "1"}
)
```

### Containers Not Removed

**Problem:** Containers remain after job completion

**Solution:**

```python
# Ensure auto_remove is enabled
backend_config = ContainerBackendConfig(
    container_runtime="docker",
    auto_remove=True,
)

# Or delete job explicitly
client.delete_job(job_id)

# Manual cleanup
# docker rm -f $(docker ps -aq --filter "label=kubeflow.org/job-name=<job-name>")
```

### Network Conflicts

**Error:** Network already exists or conflicts

**Solution:**

```bash
# Remove conflicting network
docker network rm <job-name>-net

# Clean up all Kubeflow networks
docker network ls --filter "label=kubeflow.org/job-name" -q | xargs docker network rm
```

### Image Pull Fails

**Error:** Cannot pull training runtime images

**Solution:**

```python
# Use Always pull policy
backend_config = ContainerBackendConfig(
    container_runtime="docker",
    pull_policy="Always",
)

# Or pre-pull images
# docker pull <runtime-image>
```

### Out of Memory

**Error:** Container killed due to OOM

**Solution:**

```bash
# Check Docker resource limits
docker info | grep -i memory

# Increase Docker memory (Docker Desktop Settings)
# Or reduce batch size in training code
```

## Next Steps

- **Test multi-node training**: Experiment with different `num_nodes` values
- **Try GPU training**: Set up NVIDIA Container Toolkit for GPU support
- **Explore Podman**: Try [Podman Backend](podman) for rootless containers
- **Deploy to Kubernetes**: Scale to production clusters when ready
