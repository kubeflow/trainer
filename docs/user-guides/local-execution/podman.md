# Podman Backend

Execute distributed TrainJobs in rootless, daemonless containers with Podman for enhanced security and multi-node training simulation.

## Overview

The Podman Container Backend enables distributed training jobs in isolated containers using Podman, a daemonless container engine. Podman provides Docker-compatible functionality with enhanced security through rootless operation and no background daemon requirement.

Key advantages:
- **Daemonless architecture**: No background daemon required
- **Rootless containers**: Run containers without root privileges for enhanced security
- **Full container isolation**: Separate filesystem, network, and resources
- **Multi-node support**: Distributed training across containers
- **Docker compatibility**: Works with Docker images and similar CLI
- **systemd integration**: Better service management on Linux

## When to Use

**Use Podman Backend for:**
- Security-conscious deployments requiring rootless containers
- Linux servers and development environments
- Environments where Docker daemon is restricted
- Testing distributed training without Kubernetes
- CI/CD pipelines requiring container isolation

**Don't use Podman Backend if:**
- You're on macOS/Windows and prefer Docker Desktop simplicity
- You need fastest iteration (use [Local Process Backend](local-process))
- Podman is not available in your environment

## Prerequisites

### Required Software

1. **Podman 3.0+**

   **Linux (Ubuntu/Debian):**
   ```bash
   sudo apt-get update
   sudo apt-get install -y podman
   ```

   **Linux (Fedora/RHEL):**
   ```bash
   sudo dnf install -y podman
   ```

   **macOS:**
   ```bash
   brew install podman
   podman machine init
   podman machine start
   ```

   See [official installation instructions](https://podman.io/docs/installation) for other platforms.

2. **Python 3.9+**

3. **Kubeflow SDK with Podman support:**
   ```bash
   pip install "kubeflow[podman]"
   ```

### Verification

Verify Podman is running:

```bash
# Check Podman version
podman version

# Test Podman is working
podman ps

# Verify you can pull images
podman pull hello-world
podman run --rm hello-world
```

### Optional: Custom Socket Configuration

**macOS:**
```bash
# Start Podman machine with custom socket
podman machine stop
podman machine start

# Or run service with custom socket
podman system service --time=0 unix:///tmp/podman.sock
```

**Linux (user-specific):**
```bash
# Enable user socket
systemctl --user enable --now podman.socket

# Verify socket exists
ls -la /run/user/$(id -u)/podman/podman.sock
```

## Configuration

### Basic Configuration

```python
from kubeflow.trainer import TrainerClient, ContainerBackendConfig

# Configure Podman backend
backend_config = ContainerBackendConfig(
    container_runtime="podman",
)

# Initialize client
client = TrainerClient(backend_config=backend_config)
```

### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `container_runtime` | str | None | Force "podman", "docker", or None (auto-detect) |
| `pull_policy` | str | "IfNotPresent" | Image pull: "IfNotPresent", "Always", "Never" |
| `auto_remove` | bool | True | Auto-remove containers/networks after completion |
| `container_host` | str | None | Override Podman socket URL |
| `runtime_source` | TrainingRuntimeSource | GitHub | Custom runtime configuration |

### Platform-Specific Configuration

**macOS with custom socket:**
```python
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    container_host="unix:///tmp/podman.sock"
)
```

**Linux rootless (user-specific socket):**
```python
import os

uid = os.getuid()
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    container_host=f"unix:///run/user/{uid}/podman/podman.sock"
)
```

**Always pull latest images:**
```python
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    pull_policy="Always"
)
```

**Keep containers for debugging:**
```python
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    auto_remove=False
)
```

## Basic Usage

### Simple Training Example

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def train_simple_model():
    """Simple training in Podman container."""
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

# Configure Podman backend
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    pull_policy="IfNotPresent",
    auto_remove=True
)

client = TrainerClient(backend_config=backend_config)

# Launch training
job_id = client.train(
    trainer=CustomTrainer(
        func=train_simple_model,
        num_nodes=2,
    )
)

print(f"Training job started: {job_id}")

# Wait for completion
job = client.wait_for_job_status(job_id)
print(f"Job completed with status: {job.status}")
```

## Multi-Node Distributed Training

Podman backend automatically configures networking for distributed training:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def distributed_train():
    """Distributed training across Podman containers."""
    import os
    import torch
    import torch.distributed as dist
    from torch.nn.parallel import DistributedDataParallel as DDP

    # Get distributed configuration
    rank = int(os.environ['RANK'])
    world_size = int(os.environ['WORLD_SIZE'])

    print(f"Initializing process group: rank={rank}, world_size={world_size}")

    # Initialize distributed backend
    dist.init_process_group(
        backend='gloo',  # Use 'gloo' for CPU or 'nccl' for GPU
        rank=rank,
        world_size=world_size
    )

    # Create model
    model = torch.nn.Linear(10, 1)
    ddp_model = DDP(model)

    optimizer = torch.optim.SGD(ddp_model.parameters(), lr=0.01)

    # Training loop
    for epoch in range(5):
        inputs = torch.randn(32, 10)
        targets = torch.randn(32, 1)

        optimizer.zero_grad()
        outputs = ddp_model(inputs)
        loss = torch.nn.functional.mse_loss(outputs, targets)
        loss.backward()
        optimizer.step()

        print(f"[Rank {rank}] Epoch {epoch + 1}/5, Loss: {loss.item():.4f}")

    dist.destroy_process_group()
    print(f"[Rank {rank}] Training complete")

# Configure backend
backend_config = ContainerBackendConfig(container_runtime="podman")
client = TrainerClient(backend_config=backend_config)

# Launch 4-node distributed training
job_id = client.train(
    trainer=CustomTrainer(
        func=distributed_train,
        num_nodes=4,
    )
)

print(f"Distributed training job: {job_id}")

# Stream logs from all nodes
for node_idx in range(4):
    print(f"\n=== Node {node_idx} ===")
    for log in client.get_job_logs(job_id, node_index=node_idx):
        print(log)
```

### Networking Architecture

The Podman backend creates DNS-enabled networks and uses IP addresses for reliability:

1. **Network Creation**: Dedicated Podman network with DNS enabled
2. **Master Node Launch**: Rank-0 container started first, IP address inspected
3. **MASTER_ADDR Configuration**: All containers receive rank-0 IP as MASTER_ADDR
4. **Container Launch**: Remaining containers join the network

```python
def test_networking():
    """Test Podman networking between containers."""
    import os
    import socket
    import subprocess

    rank = int(os.environ['RANK'])
    master_addr = os.environ['MASTER_ADDR']

    print(f"Rank {rank}: Hostname = {socket.gethostname()}")
    print(f"Rank {rank}: Master IP = {master_addr}")

    # Test connectivity to master (uses IP, not hostname)
    if rank != 0:
        result = subprocess.run(
            ['ping', '-c', '1', master_addr],
            capture_output=True
        )
        print(f"Rank {rank}: Ping to master IP = {'Success' if result.returncode == 0 else 'Failed'}")

# Test with multiple nodes
backend_config = ContainerBackendConfig(container_runtime="podman")
client = TrainerClient(backend_config=backend_config)

job_id = client.train(
    trainer=CustomTrainer(
        func=test_networking,
        num_nodes=4,
    )
)
```

The backend uses IP addresses instead of hostnames for `MASTER_ADDR` to ensure reliable cross-container communication.

## Complete Training Example

Here's a complete example with distributed data parallelism:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def train_cnn_distributed():
    """Train CNN with distributed data parallelism in Podman."""
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
        print(f"Starting distributed training with {world_size} workers")

    # Initialize distributed
    dist.init_process_group(backend='gloo')

    # Define CNN
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

    # Create and wrap model
    model = SimpleCNN()
    ddp_model = DDP(model)

    # Prepare dataset
    transform = transforms.Compose([
        transforms.ToTensor(),
        transforms.Normalize((0.5,), (0.5,))
    ])

    # Rank 0 downloads dataset
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
        print("Starting training...")

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
backend_config = ContainerBackendConfig(container_runtime="podman")
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

### Using Podman CLI

```bash
# List containers for a job
podman ps -a --filter "label=kubeflow.org/job-name=<job-name>"

# Inspect container
podman inspect <job-name>-node-0

# View logs
podman logs <job-name>-node-0

# Follow logs
podman logs -f <job-name>-node-0

# Execute commands
podman exec -it <job-name>-node-0 /bin/bash

# With custom socket (macOS)
podman --url unix:///tmp/podman.sock logs <job-name>-node-0
```

### Using TrainerClient

```python
# List jobs
jobs = client.list_jobs()
for job in jobs:
    print(f"Job: {job.name}, Status: {job.status}")

# Get logs from specific node
logs = client.get_job_logs(job_id, node_index=0)
for log in logs:
    print(log)

# Follow logs in real-time
for log in client.get_job_logs(job_id, follow=True):
    print(log)

# Wait for completion
job = client.wait_for_job_status(job_id)
print(f"Status: {job.status}")

# Delete job (removes containers, networks, metadata)
client.delete_job(job_id)
```

## Rootless Containers

One of Podman's key advantages is rootless container support:

### Enable Rootless Mode

**Linux:**
```bash
# Configure subuid and subgid
sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 $USER

# Enable user namespaces
sudo sysctl -w user.max_user_namespaces=15000
echo "user.max_user_namespaces=15000" | sudo tee -a /etc/sysctl.conf

# Start user podman socket
systemctl --user start podman.socket
systemctl --user enable podman.socket
```

### Rootless Training Example

```python
import os

# Configure for rootless
uid = os.getuid()
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    container_host=f"unix:///run/user/{uid}/podman/podman.sock"
)

client = TrainerClient(backend_config=backend_config)

# Training runs without root privileges
job_id = client.train(
    trainer=CustomTrainer(
        func=train_simple_model,
        num_nodes=2,
    )
)

print(f"Rootless training job: {job_id}")
```

## GPU Support

Podman supports GPU training with NVIDIA Container Toolkit:

### Prerequisites

1. **NVIDIA Drivers**:
```bash
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
```

3. **Configure Podman**:
```bash
# Generate CDI specification
sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml
```

4. **Verify**:
```bash
podman run --rm --device nvidia.com/gpu=all nvidia/cuda:11.8.0-base-ubuntu22.04 nvidia-smi
```

### GPU Training Example

```python
def train_with_gpu():
    """Training with GPU in Podman container."""
    import torch

    # Check GPU
    if torch.cuda.is_available():
        device = torch.device("cuda")
        print(f"Using GPU: {torch.cuda.get_device_name(0)}")
    else:
        device = torch.device("cpu")
        print("GPU not available, using CPU")

    # Model on GPU
    model = torch.nn.Linear(1000, 10).to(device)
    optimizer = torch.optim.Adam(model.parameters())

    for epoch in range(10):
        inputs = torch.randn(64, 1000).to(device)
        targets = torch.randint(0, 10, (64,)).to(device)

        outputs = model(inputs)
        loss = torch.nn.functional.cross_entropy(outputs, targets)

        optimizer.zero_grad()
        loss.backward()
        optimizer.step()

        print(f"Epoch {epoch + 1}, Loss: {loss.item():.4f}")

# Request GPU
job_id = client.train(
    trainer=CustomTrainer(
        func=train_with_gpu,
        resources_per_node={"gpu": "1"},
    )
)
```

## Examples and Resources

### Complete Examples

- **[MNIST Classification](https://github.com/kubeflow/trainer/tree/master/examples/pytorch/mnist)**: Complete notebook with Podman backend
- **Distributed CNN Training**: See [complete example](#complete-training-example) above

### API Documentation

- [TrainerClient API Reference](../../api-reference/python-sdk/index): Complete SDK documentation
- [ContainerBackendConfig API](../../api-reference/python-sdk/backends): Configuration reference

### Related Guides

- [Local Execution Overview](index): All local execution backends
- [Docker Backend](docker): Docker container alternative
- [Local Process Backend](local-process): Fastest local development

## Troubleshooting

### Podman Service Not Running (macOS)

**Error:** `ConnectionRefusedError: [Errno 61] Connection refused`

**Solution:**
```bash
# Check machine status
podman machine list

# Start machine
podman machine start

# Or restart
podman machine stop
podman machine start --now
```

### Socket Not Found (Linux)

**Error:** `FileNotFoundError: [Errno 2] No such file or directory`

**Solution:**
```bash
# Start user socket
systemctl --user start podman.socket
systemctl --user enable podman.socket

# Verify socket exists
ls -la /run/user/$(id -u)/podman/podman.sock
```

### Permission Denied (Rootless)

**Error:** `Error: container_linux.go:380: starting container process caused`

**Solution:**
```bash
# Enable user namespaces
sudo sysctl -w user.max_user_namespaces=15000
echo "user.max_user_namespaces=15000" | sudo tee -a /etc/sysctl.conf

# Configure subuid/subgid
sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 $USER

# Restart Podman
podman system migrate
```

### DNS Resolution Issues

**Error:** Containers cannot resolve each other

**Solution:**
```bash
# Verify DNS is enabled in network
podman network inspect <job-name>-net | grep dns_enabled
# Should show: "dns_enabled": true
```

The backend uses IP addresses for `MASTER_ADDR` instead of hostnames to avoid DNS issues.

### Containers Not Removed

**Problem:** Containers remain after job completion

**Solution:**
```python
# Ensure auto_remove enabled
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    auto_remove=True
)

# Delete job explicitly
client.delete_job(job_id)

# Manual cleanup
# podman rm -f $(podman ps -aq --filter "label=kubeflow.org/job-name=<job-name>")
```

### Network Conflicts

**Error:** Network already exists

**Solution:**
```bash
# Remove conflicting network
podman network rm <job-name>-net

# Clean up all Kubeflow networks
podman network ls --filter "label=kubeflow.org/job-name" -q | xargs podman network rm
```

### Image Pull Fails

**Error:** Cannot pull images

**Solution:**
```python
# Force pull
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    pull_policy="Always",
)

# Or pre-pull manually
# podman pull <runtime-image>
```

## Podman vs Docker

### Advantages of Podman

- **Rootless**: Run containers without root privileges
- **Daemonless**: No background process consuming resources
- **Security**: Better isolation and no daemon vulnerability surface
- **systemd integration**: Native service management
- **Fork/exec model**: More process-oriented architecture

### When to Use Each

| Use Case | Recommendation |
|----------|----------------|
| Security-first Linux environments | Podman |
| macOS/Windows development | Docker Desktop |
| CI/CD without root | Podman |
| General local development | Either (Docker more common) |
| Rootless requirements | Podman |
| systemd integration | Podman |

## Next Steps

- **Test multi-node training**: Experiment with different node counts
- **Enable rootless mode**: Set up rootless containers for security
- **Try GPU training**: Configure NVIDIA Container Toolkit
- **Deploy to Kubernetes**: Scale to production when ready
