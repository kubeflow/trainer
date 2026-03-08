# Local Execution

Develop and test training code locally without a Kubernetes cluster before scaling to production.

## Overview

Kubeflow Trainer's local execution mode enables running TrainJobs on your local machine without requiring Kubernetes deployment. This is ideal for:

- **Development and testing** of training scripts
- **Quick prototyping and experimentation**
- **Learning and educational purposes**
- **Environments where Kubernetes is not available**

The same training code works across all backends - only the backend configuration changes, allowing seamless progression from local development to production deployment.

## Available Backends

Kubeflow Trainer supports three local execution backends:

### 1. Local Process Backend

Executes TrainJobs using native Python processes and virtual environments.

**Best for:**
- Fastest iteration during development
- Single-node training scenarios
- Quick prototyping without container overhead
- Simple debugging with standard Python tools

**Limitations:**
- No multi-node support
- No container isolation
- Virtual environment dependencies only

**Learn more:** [Local Process Backend](local-process)

### 2. Docker Container Backend

Supports distributed TrainJobs in isolated Docker containers with multi-node capabilities.

**Best for:**
- General purpose local development
- macOS/Windows development environments
- Reproducible containerized environments
- Multi-node distributed training simulation

**Limitations:**
- Requires Docker Desktop/Engine
- Slower startup than local process
- Docker group membership or root access on Linux

**Learn more:** [Docker Backend](docker)

### 3. Podman Container Backend

Provides daemonless container execution with enhanced security features.

**Best for:**
- Security-conscious deployments
- Linux servers with systemd integration
- Rootless containerization requirements
- Multi-node distributed training simulation

**Limitations:**
- Requires Podman installation
- Slower startup than local process
- Socket configuration may be needed (macOS)

**Learn more:** [Podman Backend](podman)

## Backend Comparison

| Feature | Local Process | Docker | Podman |
|---------|---------------|--------|--------|
| **Setup Complexity** | None (built-in) | Docker Desktop/Engine | Podman installation |
| **Isolation** | Virtual environments | Full container isolation | Full container isolation |
| **Multi-node Support** | No | Yes | Yes |
| **Root Required** | No | Docker group or root | Rootless supported |
| **Startup Time** | Fast (seconds) | Medium (~10-30 seconds) | Medium (~10-30 seconds) |
| **Platform Support** | All platforms | All platforms | Best on Linux |
| **GPU Support** | Yes (local) | Yes (NVIDIA Container Toolkit) | Yes (NVIDIA Container Toolkit) |
| **Debugging** | Easy (standard Python) | Medium (container logs) | Medium (container logs) |
| **Resource Overhead** | Low | Medium | Medium |

## Quick Start Examples

### Local Process Backend

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, LocalProcessBackendConfig

def train_model():
    """Simple training function."""
    import torch
    print("Training with PyTorch on local process")
    # Your training code here

# Configure local process backend
backend_config = LocalProcessBackendConfig()
client = TrainerClient(backend_config=backend_config)

# Launch training
job_id = client.train(
    trainer=CustomTrainer(func=train_model)
)

print(f"Training job started: {job_id}")
```

### Docker Container Backend

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def train_model():
    """Training function with container isolation."""
    import torch
    print("Training in Docker container")
    # Your training code here

# Configure Docker backend
backend_config = ContainerBackendConfig(
    container_runtime="docker",
)
client = TrainerClient(backend_config=backend_config)

# Launch training
job_id = client.train(
    trainer=CustomTrainer(
        func=train_model,
        num_nodes=2,  # Multi-node support
    )
)

print(f"Training job started: {job_id}")
```

### Podman Container Backend

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, ContainerBackendConfig

def train_model():
    """Training function with rootless container."""
    import torch
    print("Training in Podman container")
    # Your training code here

# Configure Podman backend
backend_config = ContainerBackendConfig(
    container_runtime="podman",
)
client = TrainerClient(backend_config=backend_config)

# Launch training
job_id = client.train(
    trainer=CustomTrainer(
        func=train_model,
        num_nodes=4,  # Multi-node support
    )
)

print(f"Training job started: {job_id}")
```

## Common Job Management Operations

All backends support unified job management through the `TrainerClient` interface:

### List Jobs

```python
# List all jobs
jobs = client.list_jobs()
for job in jobs:
    print(f"Job: {job.name}, Status: {job.status}")
```

### View Logs

```python
# View logs from specific node
for log in client.get_job_logs(job_id, node_index=0):
    print(log)

# Follow logs in real-time
for log in client.get_job_logs(job_id, follow=True):
    print(log)

# View logs from all nodes
for node_idx in range(num_nodes):
    print(f"\n=== Node {node_idx} ===")
    for log in client.get_job_logs(job_id, node_index=node_idx):
        print(log)
```

### Wait for Completion

```python
from kubeflow.trainer.constants import constants

# Wait for job to complete
job = client.wait_for_job_status(
    job_id,
    status={constants.TRAINJOB_COMPLETE},
    timeout=3600  # Wait up to 1 hour
)

print(f"Job completed with status: {job.status}")
```

### Delete Jobs

```python
# Delete job (removes containers, networks, and metadata)
client.delete_job(job_id)
print(f"Job {job_id} deleted")

# Clean up all jobs
for job in client.list_jobs():
    client.delete_job(job.name)
```

## Choosing the Right Backend

Use this decision tree to choose the appropriate backend:

```{mermaid}
graph TD
    A[What are you testing?] --> B{Need multi-node?}
    B -->|No| C{Need container isolation?}
    B -->|Yes| D{What OS?}

    C -->|No| E[Local Process Backend<br/>Fastest iteration]
    C -->|Yes| F{Docker installed?}

    F -->|Yes| G[Docker Backend<br/>Wide compatibility]
    F -->|No| H[Podman Backend<br/>Rootless security]

    D -->|Linux| I{Security requirements?}
    D -->|macOS/Windows| J[Docker Backend<br/>Best support]

    I -->|High| K[Podman Backend<br/>Rootless containers]
    I -->|Standard| L{Docker installed?}

    L -->|Yes| G
    L -->|No| K
```

**Quick recommendations:**

| Scenario | Recommended Backend |
|----------|---------------------|
| Quick prototype, single GPU | Local Process |
| Testing distributed training logic | Docker or Podman |
| macOS/Windows development | Docker |
| Linux with security requirements | Podman (rootless) |
| Debugging Python training code | Local Process |
| Simulating production environment | Docker or Podman |
| CI/CD pipeline testing | Docker or Podman |
| No container runtime available | Local Process |

## Development Workflow

Recommended workflow for developing training code:

### 1. Start with Local Process

Begin development with the fastest iteration:

```python
backend_config = LocalProcessBackendConfig()
client = TrainerClient(backend_config=backend_config)

# Quick iterations
job_id = client.train(trainer=CustomTrainer(func=train_model))
```

### 2. Test with Containers

Verify containerized behavior:

```python
backend_config = ContainerBackendConfig(container_runtime="docker")
client = TrainerClient(backend_config=backend_config)

# Test container compatibility
job_id = client.train(trainer=CustomTrainer(func=train_model))
```

### 3. Test Multi-node

Validate distributed training:

```python
backend_config = ContainerBackendConfig(container_runtime="docker")
client = TrainerClient(backend_config=backend_config)

# Test distributed behavior
job_id = client.train(
    trainer=CustomTrainer(
        func=train_model,
        num_nodes=4,  # Simulate 4-node cluster
    )
)
```

### 4. Deploy to Kubernetes

Move to production environment:

```python
# Switch to Kubernetes backend (default)
client = TrainerClient()  # Uses KubernetesBackend

# Same training code, production scale
job_id = client.train(
    trainer=CustomTrainer(
        func=train_model,
        num_nodes=16,  # Real multi-node cluster
        resources_per_node={"gpu": 8},
    )
)
```

## Complete Development Example

Here's a complete example progressing through all stages:

```python
from kubeflow.trainer import (
    TrainerClient,
    CustomTrainer,
    LocalProcessBackendConfig,
    ContainerBackendConfig,
)

def train_distributed_model():
    """Distributed training function."""
    import os
    import torch
    import torch.distributed as dist

    # Get distributed info
    rank = int(os.environ.get("RANK", 0))
    world_size = int(os.environ.get("WORLD_SIZE", 1))

    print(f"Rank {rank}/{world_size} starting training")

    # Initialize distributed (if multi-node)
    if world_size > 1:
        dist.init_process_group(backend="gloo")

    # Training code
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

        print(f"[Rank {rank}] Epoch {epoch}, Loss: {loss.item():.4f}")

    if world_size > 1:
        dist.destroy_process_group()

    print(f"[Rank {rank}] Training complete!")


# Stage 1: Local Process (single node)
print("=== Stage 1: Local Process ===")
client = TrainerClient(backend_config=LocalProcessBackendConfig())
job_id = client.train(trainer=CustomTrainer(func=train_distributed_model))
client.wait_for_job_status(job_id)
print("Local process training completed\n")

# Stage 2: Docker (single node)
print("=== Stage 2: Docker Container ===")
client = TrainerClient(
    backend_config=ContainerBackendConfig(container_runtime="docker")
)
job_id = client.train(trainer=CustomTrainer(func=train_distributed_model))
client.wait_for_job_status(job_id)
print("Docker training completed\n")

# Stage 3: Docker (multi-node)
print("=== Stage 3: Docker Multi-node ===")
job_id = client.train(
    trainer=CustomTrainer(
        func=train_distributed_model,
        num_nodes=4,
    )
)
client.wait_for_job_status(job_id)
print("Multi-node training completed\n")

# Stage 4: Kubernetes (production)
print("=== Stage 4: Kubernetes Production ===")
client = TrainerClient()  # Default KubernetesBackend
job_id = client.train(
    trainer=CustomTrainer(
        func=train_distributed_model,
        num_nodes=8,
        resources_per_node={"gpu": 4},
    )
)
print(f"Production job started: {job_id}")
```

## Backend Configuration Reference

### LocalProcessBackendConfig

```python
from kubeflow.trainer import LocalProcessBackendConfig

config = LocalProcessBackendConfig(
    # No additional configuration needed
)
```

### ContainerBackendConfig

```python
from kubeflow.trainer import ContainerBackendConfig

config = ContainerBackendConfig(
    container_runtime="docker",  # "docker", "podman", or None (auto-detect)
    pull_policy="IfNotPresent",  # "IfNotPresent", "Always", "Never"
    auto_remove=True,            # Auto-remove containers after completion
    container_host=None,         # Override container socket URL
)
```

See individual backend guides for detailed configuration options.

## Examples and Resources

### Complete Examples

- **[MNIST Classification](https://github.com/kubeflow/trainer/tree/master/examples/pytorch/mnist)**: Complete example with all backends
- **Distributed Training**: Multi-node training examples (coming soon)

### API Documentation

- [TrainerClient API Reference](../api-reference/python-sdk/index): Complete SDK documentation
- [Backend Configuration](../api-reference/python-sdk/backends): Backend configuration reference

### Backend-Specific Guides

- [Local Process Backend](local-process): Fast development with Python processes
- [Docker Backend](docker): Container-based development with Docker
- [Podman Backend](podman): Rootless container development with Podman

## Troubleshooting

### Common Issues Across Backends

**Import errors:**

Ensure all imports are inside the training function:

```python
def train():
    # Correct: All imports inside function
    import torch
    import numpy as np

    # Training code...
```

**Environment variable issues:**

Container backends set different environment variables than Kubernetes. Check for:

```python
def train():
    import os

    # These are available in all backends
    rank = int(os.environ.get("RANK", 0))
    world_size = int(os.environ.get("WORLD_SIZE", 1))
    local_rank = int(os.environ.get("LOCAL_RANK", 0))
```

**Job hangs or doesn't start:**

Check job status and logs:

```python
# Get job status
job = client.get_job(name=job_id)
print(f"Status: {job.status}")

# View logs
for log in client.get_job_logs(job_id):
    print(log)
```

**Backend auto-detection fails:**

Explicitly specify the backend:

```python
# Don't rely on auto-detection
config = ContainerBackendConfig(
    container_runtime="docker",  # Explicitly set
)
```

### Backend-Specific Troubleshooting

See individual backend guides for specific troubleshooting:

- [Local Process Troubleshooting](local-process#troubleshooting)
- [Docker Troubleshooting](docker#troubleshooting)
- [Podman Troubleshooting](podman#troubleshooting)

## Next Steps

- **Choose your backend**: Review the [backend comparison](#backend-comparison) and select the appropriate backend
- **Read backend guide**: Follow the detailed guide for your chosen backend
- **Start developing**: Begin with simple examples and iterate
- **Scale to production**: Deploy to Kubernetes when ready

Ready to get started? Choose your backend:

- [Local Process Backend](local-process) - Fastest local development
- [Docker Backend](docker) - Container-based development
- [Podman Backend](podman) - Secure rootless containers

```{toctree}
:hidden:
:maxdepth: 2

local-process
docker
podman
```
