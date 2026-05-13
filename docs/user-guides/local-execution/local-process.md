# Local Process Backend

How to run TrainJobs with native Python processes and virtual environments

## Overview

The Local Process Backend allows you to execute TrainJobs directly on your local machine using native Python processes and virtual environments. This backend is ideal for:

- Quick prototyping and development
- Testing training scripts without container overhead
- Environments where Docker/Podman is not available
- Debugging training code locally

The Local Process Backend creates isolated Python virtual environments for each TrainJob, automatically installs required dependencies, and manages the lifecycle of training processes in background threads with real-time log streaming.

:::{note}
Only single-node training is currently supported.
:::

## Prerequisites

- Python 3.9 or later
- pip (Python package installer)
- **Kubeflow SDK**: Install the base package:
  ```bash
  pip install kubeflow
  ```
- Sufficient disk space for virtual environments
- Required Python packages for your training framework (e.g., PyTorch, TensorFlow)

## Basic Example

Here's a simple example using the Local Process Backend:

```python
from kubeflow.trainer import CustomTrainer, TrainerClient, LocalProcessBackendConfig

def train_model():
    import torch
    import time

    print("Starting training...")

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

        print(f"Epoch {epoch + 1}/5, Loss: {loss.item():.4f}")

    print("Training completed!")

backend_config = LocalProcessBackendConfig(
    cleanup_venv=True
)

client = TrainerClient(backend_config=backend_config)

trainer = CustomTrainer(
    func=train_model,
    num_nodes=1,
)

job_name = client.train(trainer=trainer)
print(f"TrainJob started: {job_name}")

job = client.wait_for_job_status(
    job_name,
)

print(f"Job completed with status: {job.status}")
```

## Configuration Options

### LocalProcessBackendConfig

The `LocalProcessBackendConfig` class provides configuration options for the Local Process Backend:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cleanup_venv` | `bool` | `True` | Whether to automatically remove virtual environments after job completion. Set to `False` to preserve environments for debugging. |

**Example:**

```python
from kubeflow.trainer import LocalProcessBackendConfig

backend_config = LocalProcessBackendConfig(
    cleanup_venv=False
)
```

## Working with Runtimes

The Local Process Backend has a fixed set of built-in runtimes (unlike Container Backends which load runtimes from external sources).

### Supported Runtimes

| Runtime | Framework | Description | Packages |
|---------|-----------|-------------|----------|
| `torch-distributed` | PyTorch | PyTorch training with torchrun | `torch` |

## Job Management

For common job management operations (listing jobs, viewing logs, deleting jobs), see the [Job Management section](overview.md#job-management) in the overview.

### Checking Job Status

The Local Process Backend also supports checking detailed job status:

```python
job = client.get_job(job_name)
print(f"Status: {job.status}")
print(f"Created: {job.created}")
print(f"Completed: {job.completed}")
```

## Advanced Usage

### Custom Training with Dependencies

You can specify additional packages to install in the training environment:

```python
from kubeflow.trainer import CustomTrainer, TrainerClient, LocalProcessBackendConfig

def train_with_dependencies():
    import numpy as np
    import pandas as pd
    from sklearn.ensemble import RandomForestClassifier

    print("Training with scikit-learn...")
    X = np.random.rand(100, 4)
    y = np.random.randint(0, 2, 100)

    clf = RandomForestClassifier(n_estimators=10)
    clf.fit(X, y)

    print(f"Model accuracy: {clf.score(X, y):.2f}")

backend_config = LocalProcessBackendConfig()
client = TrainerClient(backend_config=backend_config)

trainer = CustomTrainer(
    func=train_with_dependencies,
    packages_to_install=["numpy", "pandas", "scikit-learn"],
    pip_index_urls=["https://pypi.org/simple"]
)

job_name = client.train(trainer=trainer)
```

### Environment Variables

Pass custom environment variables to your TrainJob:

```python
trainer = CustomTrainer(
    func=train_model,
    env={
        "CUDA_VISIBLE_DEVICES": "0",
        "OMP_NUM_THREADS": "4",
        "CUSTOM_VAR": "value"
    }
)
```

### Debugging Failed Jobs

When `cleanup_venv=False`, you can inspect the virtual environment after job failure:

```python
backend_config = LocalProcessBackendConfig(cleanup_venv=False)
client = TrainerClient(backend_config=backend_config)

trainer = CustomTrainer(func=train_model)
job_name = client.train(trainer=trainer)

try:
    job = client.wait_for_job_status(
        job_name,
        status={constants.TRAINJOB_COMPLETE, constants.TRAINJOB_FAILED},
        timeout=300
    )

    if job.status == constants.TRAINJOB_FAILED:
        print("Job failed. Logs:")
        for log_line in client.get_job_logs(job_name):
            print(log_line, end='')

        print(f"\nVirtual environment preserved for job: {job_name}")
except Exception as e:
    print(f"Error: {e}")
finally:
    client.delete_job(job_name)
```

## How It Works

Understanding the internal workflow helps with debugging and optimization:

### 1. Job Creation

- A unique job name is generated (e.g., `a1b2c3d4e5f`)
- A temporary directory is created: `/tmp/a1b2c3d4e5f_xyz/`
- A Python virtual environment is set up with isolation

### 2. Environment Setup

```bash
python -m venv --without-pip /tmp/a1b2c3d4e5f_xyz/
source /tmp/a1b2c3d4e5f_xyz/bin/activate
python -m ensurepip --upgrade --default-pip
```

### 3. Package Dependency Resolution

The backend implements intelligent package dependency management:

**Package Sources:**

When you use a runtime (e.g., `torch-distributed`), the backend installs packages from two sources:

1. **Runtime packages**: Built-in packages defined by the runtime itself
   - For Local Process Backend, runtimes are hardcoded in the SDK (see Supported Runtimes)
   - Example: The `torch-distributed` runtime automatically includes `torch`
   - Note: Unlike Container Backends which load runtimes from external sources (GitHub/YAML files), Local Process Backend uses a fixed set of runtimes
2. **Trainer packages**: Additional packages you specify in your `CustomTrainer`
   - Specified via the `packages_to_install` parameter
   - Example: `packages_to_install=["pandas", "scikit-learn"]`

**Dependency Resolution Rules:**

When packages from both sources are combined:

1. **Trainer Override**: If you specify a package in `packages_to_install` that also exists in the runtime, your version takes precedence
2. **Case-Insensitive Matching**: Package names are normalized (PEP 503)
3. **Duplicate Detection**: Prevents duplicate packages in trainer dependencies
4. **Order Preservation**: Runtime packages come first, then trainer packages (except where overridden)

**Example:**

```python
# Runtime packages: ["torch==1.9.0", "numpy"]
# Trainer packages: ["torch==2.0.0", "scipy"]
# Result: ["numpy", "torch==2.0.0", "scipy"]
# torch==2.0.0 from trainer overrides torch==1.9.0 from runtime
```

### 4. Training Code Preparation

The training function source code is extracted and written to a Python file:

```python
# Written to: /tmp/a1b2c3d4e5f_xyz/train_a1b2c3d4e5f.py

def train_model():
    import torch
    print("Starting PyTorch training...")
    # ... your code ...

train_model()  # Auto-generated function call
```

### 5. Execution

For PyTorch framework:

```bash
/tmp/a1b2c3d4e5f_xyz/bin/torchrun train_a1b2c3d4e5f.py
```

For other frameworks:

```bash
/tmp/a1b2c3d4e5f_xyz/bin/python train_a1b2c3d4e5f.py
```

### 6. Cleanup (Optional)

When `cleanup_venv=True`:

```bash
rm -rf /tmp/a1b2c3d4e5f_xyz/
```

## Best Practices

### Perfect For

- **Local Development**: Developing and testing training code
- **Experimentation**: Quick iteration on training algorithms
- **CI/CD Pipelines**: Automated testing of TrainJobs
- **Educational Use**: Learning distributed training concepts
- **Prototyping**: Validating ideas before cluster deployment

### Not Suitable For

- **Production Training**: Large-scale distributed TrainJobs
- **Multi-Node Training**: Training across multiple machines
- **Resource Management**: Fine-grained GPU/memory allocation
- **Long-Running Jobs**: TrainJobs that run for days/weeks
- **High Availability**: Mission-critical training pipelines

### General Best Practices

1. **Use for Development**: The Local Process Backend is best suited for development and testing. For production workloads, consider using the Container Backend or Kubernetes Backend.
2. **Clean Up Resources**: Set `cleanup_venv=True` (default) to avoid filling disk space with virtual environments.
3. **Test Before Containerizing**: Use the Local Process Backend to quickly validate your training code before moving to containerized environments.
4. **Monitor Resource Usage**: Since jobs run directly on your machine, monitor CPU, memory, and disk usage to avoid resource exhaustion.
5. **Specify Dependencies Explicitly**: Use `packages_to_install` to ensure all required packages are installed in the isolated environment.

## Troubleshooting

### Common Issues

| Error | Cause | Solution |
|-------|-------|----------|
| `ValueError: CustomTrainer must be set` | Using BuiltinTrainer | Use CustomTrainer instead |
| `ValueError: Runtime 'name' not found` | Invalid runtime name | Use `list_runtimes()` to see available options |
| `ValueError: No python executable found` | Missing Python | Install Python or ensure it's in PATH |
| `No TrainJob with name 'name'` | Job doesn't exist | Check job name spelling |

### Job Status Flow

```
Created → Running → Complete
    ↓       ↓
  Failed ← Failed
```

### Virtual Environment Creation Fails

**Problem**: Error creating virtual environment.

**Solution**: Ensure you have sufficient disk space and that Python's `venv` module is installed:

```bash
python -m venv --help
```

### Package Installation Errors

**Problem**: Required packages fail to install in the virtual environment.

**Solution**:

- Check your internet connection
- Verify that package names are correct
- Use `packages_to_install` to explicitly specify packages
- Ensure pip is up to date: `pip install --upgrade pip`

### Jobs Not Cleaning Up

**Problem**: Virtual environments remain after job completion.

**Solution**: Verify that `cleanup_venv=True` in your config, or manually delete jobs:

```python
client.delete_job(job_name)
```

### Permission Errors

**Problem**: Permission denied when creating virtual environments.

**Solution**: Ensure you have write permissions to the temp directory. On Unix-like systems, check:

```bash
ls -ld /tmp
```

### Debug Mode

Enable debug logging for detailed execution information:

```python
import logging
logging.basicConfig(level=logging.DEBUG)

backend_config = LocalProcessBackendConfig(cleanup_venv=False)
client = TrainerClient(backend_config=backend_config)
```

## Limitations

- **Single Machine Only**: Only runs on local machine, no distributed training across multiple nodes. The `num_nodes` parameter is ignored.
- **CustomTrainer Only**: Does not support BuiltinTrainer configurations.
- **No GPU Scheduling**: Cannot manage GPU allocation across multiple jobs.
- **Process Isolation**: Jobs are isolated by virtual environment, not containers.
- **Limited Scaling**: Not suitable for large-scale production training.
- **System Dependencies**: Training code must be compatible with your local Python environment and operating system.

## Switching Between Backends

For information about switching between Local Process, Container (Docker/Podman), and Kubernetes backends, see the [Switching Between Backends section](overview.md#switching-between-backends) in the overview.

## Next Steps

- Try the [MNIST example notebook](https://github.com/kubeflow/trainer/blob/master/examples/local/local-training-mnist.ipynb) for a complete end-to-end example
- Learn about the [Container Backend with Docker](docker) for containerized training
- Learn about the [Container Backend with Podman](podman) for rootless containerized training
