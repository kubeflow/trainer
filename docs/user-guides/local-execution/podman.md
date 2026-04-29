# Podman Backend

## Overview

The Container Backend with Podman enables you to run distributed TrainJobs in isolated containers using Podman, a daemonless container engine. Podman offers several advantages over Docker:

- **Daemonless Architecture**: No background daemon required, reducing attack surface
- **Rootless Containers**: Run containers without root privileges for enhanced security
- **Full Container Isolation**: Each training process runs in its own container with isolated filesystem, network, and resources
- **Multi-Node Support**: Run distributed training across multiple containers with automatic DNS-enabled networking
- **Docker Compatibility**: Compatible with Docker images and Docker CLI syntax
- **systemd Integration**: Better integration with systemd for service management

The Podman backend uses the same adapter pattern as Docker, providing a unified interface for container operations.

## Prerequisites

### Required Software & Initial Setup

- **Podman 3.0+**: Install Podman for your platform by following the
[podman installation instructions](https://podman.io/docs/installation)
- **Kubeflow SDK**: Install with Podman support:
  ```bash
  pip install "kubeflow[podman]"
  ```

### Verify Installation

```bash
podman version
podman ps
```

#### Custom Socket Location (Optional)

By default, Podman uses different socket locations than Docker. You can specify a custom socket:

```bash
# Start Podman with custom socket (macOS/Linux)
podman system service --time=0 unix:///tmp/podman.sock

# Or use systemd (Linux)
systemctl --user enable --now podman.socket
```

## Basic Example

Here's a simple example using the Podman Container Backend:

```python
from kubeflow.trainer import CustomTrainer, TrainerClient, ContainerBackendConfig

def train_model():
    """Simple training function."""
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

backend_config = ContainerBackendConfig(
    container_runtime="podman",
    pull_policy="IfNotPresent",
    auto_remove=True
)

client = TrainerClient(backend_config=backend_config)

trainer = CustomTrainer(
    func=train_model,
    num_nodes=2
)

job_name = client.train(trainer=trainer)
print(f"TrainJob started: {job_name}")

job = client.wait_for_job_status(job_name)
print(f"Job completed with status: {job.status}")
```

## Configuration Options

### ContainerBackendConfig for Podman

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `container_runtime` | `str \| None` | `None` | Force specific runtime: `"podman"`, `"docker"`, or `None` (auto-detect). Use `"podman"` to ensure Podman is used. |
| `pull_policy` | `str` | `"IfNotPresent"` | Image pull policy: `"IfNotPresent"` (pull if missing), `"Always"` (always pull), `"Never"` (use cached only). |
| `auto_remove` | `bool` | `True` | Automatically remove containers and networks after job completion or deletion. Set to `False` for debugging. |
| `container_host` | `str \| None` | `None` | Override Podman socket URL (e.g., `"unix:///tmp/podman.sock"`, `"unix:///run/user/1000/podman/podman.sock"`). |
| `runtime_source` | `TrainingRuntimeSource` | GitHub sources | Configuration for training runtime sources. See "Custom Runtime Sources" section below. |

### Configuration Examples

#### Basic Podman Configuration

```python
backend_config = ContainerBackendConfig(
    container_runtime="podman",
)
```

#### Custom Socket Location

```python
# macOS with Podman machine
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    container_host="unix:///tmp/podman.sock"
)

# Linux rootless (user-specific socket)
import os
uid = os.getuid()
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    container_host=f"unix:///run/user/{uid}/podman/podman.sock"
)
```

#### Always Pull Latest Image

```python
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    pull_policy="Always"
)
```

#### Keep Containers for Debugging

```python
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    auto_remove=False
)
```

## Multi-Node Distributed Training

The Podman backend automatically sets up networking and environment variables for distributed training:

```python
from kubeflow.trainer import CustomTrainer, TrainerClient, ContainerBackendConfig

def distributed_train():
    """PyTorch distributed training example."""
    import os
    import torch
    import torch.distributed as dist

    rank = int(os.environ['RANK'])
    world_size = int(os.environ['WORLD_SIZE'])

    print(f"Initializing process group: rank={rank}, world_size={world_size}")

    dist.init_process_group(
        backend='gloo',
        rank=rank,
        world_size=world_size
    )

    model = torch.nn.Linear(10, 1)
    ddp_model = torch.nn.parallel.DistributedDataParallel(model)

    for epoch in range(5):
        print(f"[Rank {rank}] Training epoch {epoch + 1}")

    dist.destroy_process_group()
    print(f"[Rank {rank}] Training complete")

backend_config = ContainerBackendConfig(
    container_runtime="podman",
)

client = TrainerClient(backend_config=backend_config)

trainer = CustomTrainer(
    func=distributed_train,
    num_nodes=4
)

job_name = client.train(trainer=trainer)
```

### Podman-Specific Networking

Podman creates networks with **DNS enabled by default**, allowing containers to resolve each other by hostname. The backend implementation uses **IP addresses** for the `MASTER_ADDR` environment variable to ensure reliable communication:

#### IP Address Resolution

The Podman backend automatically retrieves the IP address of the rank-0 container using `podman inspect`:

```bash
podman inspect --format '{{.NetworkSettings.Networks.<network-name>.IPAddress}}' <container-name>
```

This IP address is then set as `MASTER_ADDR` for all nodes in the job, ensuring that:
 - Communication works even if DNS resolution has timing issues
 - The master address is available immediately when containers start
 - Distributed training frameworks (PyTorch, TensorFlow) can connect reliably

#### DNS Resolution

While DNS is enabled and containers can resolve each other by hostname, the backend uses IP addresses for reliability:

```python
def test_networking():
    import os
    import socket

    rank = int(os.environ['RANK'])
    master_addr = os.environ['MASTER_ADDR']

    print(f"Rank {rank}: My hostname is {socket.gethostname()}")
    print(f"Rank {rank}: Master address (IP): {master_addr}")
    if rank == 0:
        container_name = os.environ.get('HOSTNAME')
        print(f"Rank {rank}: My IP address: {socket.gethostbyname(container_name)}")

    if rank != 0:
        import subprocess
        result = subprocess.run(['ping', '-c', '1', master_addr], capture_output=True)
        print(f"Ping to master IP: {result.returncode == 0}")
```

#### Network Architecture

For a job with `num_nodes=3`, the Podman backend:

1. Creates a dedicated network: `<job-name>-net` with DNS enabled
2. Launches rank-0 container and waits for it to be running
3. Inspects rank-0 container to get its IP address
4. Sets `MASTER_ADDR` to this IP for all containers
5. Launches remaining containers (rank 1, 2, ...) connected to the same network

This approach combines the benefits of DNS (hostname resolution) with the reliability of IP addresses for critical communication paths.

## Job Management

For common job management operations (listing jobs, viewing logs, deleting jobs), see the [Job Management section](overview.md#job-management) in the overview.

### Inspecting Containers with Podman CLI

When `auto_remove=False`, you can inspect containers:

```bash
podman ps -a --filter "label=kubeflow.org/job-name=<job-name>"
podman inspect <job-name>-node-0
podman logs <job-name>-node-0
podman exec -it <job-name>-node-0 /bin/bash
podman --url unix:///tmp/podman.sock logs <job-name>-node-0
```

## Working with Runtimes

For information about using runtimes and custom runtime sources, see the [Working with Runtimes section](overview.md#working-with-runtimes) in the overview.

## Troubleshooting

### Podman Service Not Running (macOS)

**Error**: `ConnectionRefusedError: [Errno 61] Connection refused`

**Solution**:
```bash
podman machine list
podman machine start
podman machine stop
podman machine start --now
```

### Socket Not Found (Linux)

**Error**: `FileNotFoundError: [Errno 2] No such file or directory: '/run/user/1000/podman/podman.sock'`

**Solution**:
```bash
systemctl --user start podman.socket
systemctl --user enable podman.socket
ls -la /run/user/$(id -u)/podman/podman.sock
```

### Permission Denied (Rootless)

**Error**: `Error: container_linux.go:380: starting container process caused...`

**Solution**:
```bash
sudo sysctl -w user.max_user_namespaces=15000
echo "user.max_user_namespaces=15000" | sudo tee -a /etc/sysctl.conf
sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 $USER
```

### DNS Resolution Issues

**Error**: Containers cannot resolve each other's hostnames

**Solution**:
```bash
podman network inspect <job-name>-net | grep dns_enabled
```

### Containers Not Removed

**Problem**: Containers remain after job completion

**Solution**:
```python
backend_config = ContainerBackendConfig(
    container_runtime="podman",
    auto_remove=True
)
client.delete_job(job_name)
```

Or:
```bash
podman rm -f $(podman ps -aq --filter "label=kubeflow.org/job-name=<job-name>")
```

## Next Steps

- Try the [MNIST example notebook](https://github.com/kubeflow/trainer/blob/master/examples/local/local-container-mnist.ipynb) for a complete end-to-end example
- Learn about the [Container Backend with Docker](docker) for Docker-specific features
- Learn about the [Local Process Backend](local-process) for non-containerized local execution
