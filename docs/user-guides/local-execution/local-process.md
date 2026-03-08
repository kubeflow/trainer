# Local Process Backend

Execute TrainJobs using native Python processes and virtual environments for the fastest local development iteration.

## Overview

The Local Process Backend executes TrainJobs directly as Python subprocesses using virtual environments. This provides the fastest development experience without container overhead, making it ideal for rapid prototyping and debugging.

Key features:
- **Zero setup required**: No additional software installation needed
- **Fast startup**: Training starts in seconds, not minutes
- **Easy debugging**: Use standard Python debugging tools
- **Low overhead**: Minimal resource consumption
- **Native performance**: Direct access to system resources

## When to Use

**Use Local Process Backend for:**
- Quick prototyping and experimentation
- Debugging training code with Python debuggers
- Testing training logic before containerization
- Single-node training scenarios
- Development on resource-constrained machines

**Don't use Local Process Backend for:**
- Multi-node distributed training (not supported)
- Testing container-specific behavior
- Simulating production environments
- Scenarios requiring strict isolation

## Prerequisites

The Local Process Backend requires minimal setup:

- **Python 3.9+**: Any supported Python version
- **Kubeflow SDK**: `pip install kubeflow-trainer`

That's it! No Docker, Podman, or other container runtimes needed.

## Basic Configuration

The Local Process Backend requires minimal configuration:

```python
from kubeflow.trainer import TrainerClient, LocalProcessBackendConfig

# Create backend configuration
backend_config = LocalProcessBackendConfig()

# Initialize client
client = TrainerClient(backend_config=backend_config)
```

There are no additional configuration parameters - the backend works out of the box.

## Basic Usage

### Simple Training Example

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, LocalProcessBackendConfig

def train_simple_model():
    """Simple training function."""
    import torch
    import torch.nn as nn
    import torch.optim as optim

    print("Starting training...")

    # Create simple model
    model = nn.Linear(10, 1)
    optimizer = optim.SGD(model.parameters(), lr=0.01)
    criterion = nn.MSELoss()

    # Training loop
    for epoch in range(10):
        # Generate random data
        inputs = torch.randn(32, 10)
        targets = torch.randn(32, 1)

        # Forward pass
        optimizer.zero_grad()
        outputs = model(inputs)
        loss = criterion(outputs, targets)

        # Backward pass
        loss.backward()
        optimizer.step()

        print(f"Epoch {epoch + 1}/10, Loss: {loss.item():.4f}")

    print("Training completed!")

# Configure local process backend
backend_config = LocalProcessBackendConfig()
client = TrainerClient(backend_config=backend_config)

# Launch training
job_id = client.train(
    trainer=CustomTrainer(func=train_simple_model)
)

print(f"Training job: {job_id}")

# Wait for completion
job = client.wait_for_job_status(job_id)
print(f"Job status: {job.status}")
```

### Complete Training Example

Here's a more complete example training a CNN on Fashion-MNIST:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer, LocalProcessBackendConfig

def train_fashion_mnist():
    """Train CNN on Fashion-MNIST dataset."""
    import torch
    import torch.nn as nn
    import torch.nn.functional as F
    import torch.optim as optim
    from torchvision import datasets, transforms
    from torch.utils.data import DataLoader

    print("Loading Fashion-MNIST dataset...")

    # Data preparation
    transform = transforms.Compose([
        transforms.ToTensor(),
        transforms.Normalize((0.5,), (0.5,))
    ])

    train_dataset = datasets.FashionMNIST(
        root="./data",
        train=True,
        download=True,
        transform=transform
    )

    train_loader = DataLoader(
        train_dataset,
        batch_size=64,
        shuffle=True,
        num_workers=2
    )

    # Define model
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

    # Training setup
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    print(f"Using device: {device}")

    model = FashionCNN().to(device)
    optimizer = optim.Adam(model.parameters(), lr=0.001)
    criterion = nn.CrossEntropyLoss()

    # Training loop
    print("Starting training...")
    num_epochs = 5

    for epoch in range(num_epochs):
        model.train()
        epoch_loss = 0.0
        correct = 0
        total = 0

        for batch_idx, (data, target) in enumerate(train_loader):
            data, target = data.to(device), target.to(device)

            optimizer.zero_grad()
            output = model(data)
            loss = criterion(output, target)
            loss.backward()
            optimizer.step()

            epoch_loss += loss.item()
            _, predicted = torch.max(output.data, 1)
            total += target.size(0)
            correct += (predicted == target).sum().item()

            if batch_idx % 100 == 0:
                print(f"Epoch {epoch + 1}, Batch {batch_idx}, "
                      f"Loss: {loss.item():.4f}")

        accuracy = 100.0 * correct / total
        avg_loss = epoch_loss / len(train_loader)
        print(f"Epoch {epoch + 1}/{num_epochs} completed. "
              f"Loss: {avg_loss:.4f}, Accuracy: {accuracy:.2f}%")

    # Save model
    torch.save(model.state_dict(), "fashion_mnist_model.pth")
    print("Training completed! Model saved to fashion_mnist_model.pth")

# Configure and launch
backend_config = LocalProcessBackendConfig()
client = TrainerClient(backend_config=backend_config)

job_id = client.train(
    trainer=CustomTrainer(func=train_fashion_mnist)
)

print(f"Training job started: {job_id}")

# Stream logs
for log in client.get_job_logs(job_id, follow=True):
    print(log)

# Wait for completion
job = client.wait_for_job_status(job_id)
print(f"Training completed with status: {job.status}")
```

## Virtual Environment Management

The Local Process Backend automatically creates and manages virtual environments for each training job.

### Automatic Virtual Environment Creation

Each job gets its own isolated virtual environment:

```python
def train_with_dependencies():
    """Training with specific package versions."""
    import torch
    import numpy as np
    from sklearn.metrics import accuracy_score

    print(f"PyTorch version: {torch.__version__}")
    print(f"NumPy version: {np.__version__}")

    # Training code...

# Virtual environment is created automatically with these packages
job_id = client.train(
    trainer=CustomTrainer(
        func=train_with_dependencies,
        # These packages are installed in the virtual environment
        packages_to_install=["torch>=2.0.0", "scikit-learn"],
    )
)
```

### Package Installation

Specify packages to install in the virtual environment:

```python
from kubeflow.trainer import CustomTrainer

trainer = CustomTrainer(
    func=train_model,
    packages_to_install=[
        "torch==2.7.1",
        "torchvision",
        "torchaudio",
        "transformers>=4.30.0",
        "datasets",
        "accelerate",
    ]
)

job_id = client.train(trainer=trainer)
```

The backend will:
1. Create a fresh virtual environment
2. Install specified packages using pip
3. Execute your training function in that environment

## Job Management

### List Jobs

```python
# List all jobs
jobs = client.list_jobs()
for job in jobs:
    print(f"Job: {job.name}")
    print(f"  Status: {job.status}")
    print(f"  Created: {job.creation_timestamp}")
```

### View Logs

```python
# Get all logs
logs = client.get_job_logs(job_id)
for log in logs:
    print(log)

# Follow logs in real-time
for log in client.get_job_logs(job_id, follow=True):
    print(log)
```

### Check Status

```python
# Get job details
job = client.get_job(name=job_id)
print(f"Status: {job.status}")
print(f"Steps:")
for step in job.steps:
    print(f"  {step.name}: {step.status}")
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

print(f"Final status: {job.status}")
```

### Delete Jobs

```python
# Delete specific job
client.delete_job(job_id)

# Clean up all jobs
for job in client.list_jobs():
    client.delete_job(job.name)
```

## GPU Support

The Local Process Backend supports GPU training with CUDA:

```python
def train_with_gpu():
    """Training with GPU acceleration."""
    import torch

    # Check GPU availability
    if torch.cuda.is_available():
        device = torch.device("cuda")
        print(f"Using GPU: {torch.cuda.get_device_name(0)}")
        print(f"GPU Memory: {torch.cuda.get_device_properties(0).total_memory / 1024**3:.2f} GB")
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

backend_config = LocalProcessBackendConfig()
client = TrainerClient(backend_config=backend_config)

job_id = client.train(
    trainer=CustomTrainer(
        func=train_with_gpu,
        packages_to_install=["torch"],
    )
)
```

The backend will use GPUs if available on your system.

## Debugging

### Using Python Debugger

Since jobs run as local processes, you can use standard Python debugging tools:

```python
def train_with_debugging():
    """Training with debugging support."""
    import torch
    import pdb

    print("Starting training...")

    model = torch.nn.Linear(10, 1)
    optimizer = torch.optim.SGD(model.parameters(), lr=0.01)

    for epoch in range(5):
        inputs = torch.randn(32, 10)
        targets = torch.randn(32, 1)

        optimizer.zero_grad()
        outputs = model(inputs)
        loss = torch.nn.functional.mse_loss(outputs, targets)

        # Set breakpoint for debugging
        if epoch == 2:
            pdb.set_trace()

        loss.backward()
        optimizer.step()

        print(f"Epoch {epoch + 1}, Loss: {loss.item():.4f}")

# Run with debugging
job_id = client.train(
    trainer=CustomTrainer(func=train_with_debugging)
)
```

### Using Logging

Add detailed logging for troubleshooting:

```python
def train_with_logging():
    """Training with detailed logging."""
    import torch
    import logging

    # Configure logging
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(levelname)s - %(message)s'
    )
    logger = logging.getLogger(__name__)

    logger.info("Starting training...")

    model = torch.nn.Linear(10, 1)
    optimizer = torch.optim.SGD(model.parameters(), lr=0.01)

    for epoch in range(5):
        logger.debug(f"Starting epoch {epoch + 1}")

        inputs = torch.randn(32, 10)
        targets = torch.randn(32, 1)

        optimizer.zero_grad()
        outputs = model(inputs)
        loss = torch.nn.functional.mse_loss(outputs, targets)

        logger.info(f"Epoch {epoch + 1}, Loss: {loss.item():.4f}")

        loss.backward()
        optimizer.step()

        logger.debug(f"Completed epoch {epoch + 1}")

    logger.info("Training completed!")

job_id = client.train(
    trainer=CustomTrainer(func=train_with_logging)
)
```

## Performance Considerations

### Pros

- **Fastest startup**: No container build or pull time
- **Direct hardware access**: No virtualization overhead
- **Simple debugging**: Use standard Python tools
- **Low resource usage**: No container runtime overhead

### Cons

- **No isolation**: Shares system resources with other processes
- **No multi-node**: Cannot simulate distributed training
- **Environment differences**: May behave differently than containerized environments
- **Package conflicts**: Virtual environment isolation only

### Optimization Tips

**Use for rapid iteration:**

```python
# Quick experiments with fast iteration
for lr in [0.001, 0.01, 0.1]:
    def train_with_lr():
        import torch
        model = torch.nn.Linear(10, 1)
        optimizer = torch.optim.SGD(model.parameters(), lr=lr)
        # Training loop...

    job_id = client.train(trainer=CustomTrainer(func=train_with_lr))
    client.wait_for_job_status(job_id)
    print(f"Completed training with lr={lr}")
```

**Test before containerization:**

```python
# 1. Develop with local process
backend_config = LocalProcessBackendConfig()
client = TrainerClient(backend_config=backend_config)
job_id = client.train(trainer=CustomTrainer(func=train))
client.wait_for_job_status(job_id)

# 2. Test with container
from kubeflow.trainer import ContainerBackendConfig
backend_config = ContainerBackendConfig(container_runtime="docker")
client = TrainerClient(backend_config=backend_config)
job_id = client.train(trainer=CustomTrainer(func=train))
```

## Limitations

Current limitations of the Local Process Backend:

- **Single-node only**: Multi-node distributed training not supported
- **No true isolation**: Uses virtual environments, not containers
- **Limited environment simulation**: May differ from production containers
- **No network simulation**: Cannot test multi-node networking
- **Shared resources**: Competes with other system processes

For multi-node training or container-based workflows, use:
- [Docker Backend](docker): Container-based multi-node support
- [Podman Backend](podman): Rootless container multi-node support

## Examples and Resources

### Complete Examples

- **[MNIST Training](https://github.com/kubeflow/trainer/tree/master/examples/pytorch/mnist)**: Complete example with local process backend
- **Fashion-MNIST CNN**: See the [complete example](#complete-training-example) above

### API Documentation

- [TrainerClient API Reference](../../api-reference/python-sdk/index): Complete SDK documentation
- [LocalProcessBackendConfig API](../../api-reference/python-sdk/backends): Backend configuration reference

### Related Guides

- [Local Execution Overview](index): Overview of all local execution backends
- [Docker Backend](docker): Container-based development
- [Podman Backend](podman): Rootless container development

## Troubleshooting

### Common Issues

**Package installation fails:**

Check that pip can access packages:

```python
# Test package availability
trainer = CustomTrainer(
    func=train,
    packages_to_install=["torch==2.7.1"],  # Specific version
)
```

**Virtual environment creation fails:**

Ensure Python venv module is available:

```bash
# On Ubuntu/Debian
sudo apt-get install python3-venv

# On macOS (usually included)
python3 -m venv --help
```

**GPU not detected:**

Verify CUDA installation:

```bash
# Check CUDA
nvidia-smi

# Verify PyTorch CUDA support
python -c "import torch; print(torch.cuda.is_available())"
```

**Job output not visible:**

Use `follow=True` to stream logs:

```python
for log in client.get_job_logs(job_id, follow=True):
    print(log)
```

**Import errors:**

Ensure all imports are inside the training function:

```python
# Correct
def train():
    import torch  # Inside function
    # Training code...

# Incorrect
import torch  # Outside function
def train():
    # Training code...
```

**Job hangs or doesn't complete:**

Check job status and logs:

```python
job = client.get_job(name=job_id)
print(f"Status: {job.status}")

logs = client.get_job_logs(job_id)
print("\n".join(logs))
```

## Next Steps

- **Try multi-node training**: Move to [Docker Backend](docker) or [Podman Backend](podman)
- **Test containerization**: Verify your code works in containers before production
- **Deploy to Kubernetes**: Scale to production with full Kubeflow Trainer
- **Explore examples**: Browse [example notebooks](https://github.com/kubeflow/trainer/tree/master/examples)
