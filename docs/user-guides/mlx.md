# MLX

Train and fine-tune AI models on Apple Silicon with MLX, Apple's NumPy-like array framework optimized for Metal GPUs.

## Overview

MLX is Apple's machine learning framework designed for Apple Silicon, providing a NumPy-like API with automatic differentiation and GPU acceleration. Kubeflow Trainer enables distributed MLX training on Kubernetes clusters with Apple Silicon nodes, making it easy to scale your MLX workloads.

Key features:
- **Composable function transformations**: Automatic differentiation, vectorization, and computation graph optimization
- **Lazy computation**: Arrays materialize only when needed for memory efficiency
- **Multi-device support**: Seamless operation across CPUs and GPUs
- **Unified memory**: Efficient data sharing between CPU and GPU on Apple Silicon
- **Distributed training**: Data parallelism with gradient averaging across multiple devices

## Prerequisites

Before following this guide, ensure you have:

- Completed the [Getting Started](../getting-started/index) guide
- Access to a Kubernetes cluster with Apple Silicon nodes (M1, M2, M3, or M4)
- Basic understanding of MLX and distributed training concepts
- MLX-compatible hardware for local development (optional)

:::{note}
While MLX is optimized for Apple Silicon GPUs, you can develop on Apple Silicon locally and then deploy to GPU clusters for training, evaluating the results back on your local machine.
:::

## The mlx-distributed Runtime

Kubeflow Trainer provides the `mlx-distributed` ClusterTrainingRuntime that uses MPI backend to coordinate distributed MLX training across multiple nodes.

### Runtime Features

- **Automatic distributed initialization**: MLX distributed setup handled automatically
- **Data parallelism support**: Dataset sharding across devices with gradient averaging
- **CUDA runtime support**: Pre-installed CUDA drivers and `mlx[cuda]` package for GPU clusters
- **Flexible deployment**: Train on GPU clusters, evaluate on Apple Silicon locally

### Automatic Environment Variables

The runtime provides access to distributed configuration:

| Variable/Function | Description | Example |
|-------------------|-------------|---------|
| `mx.distributed.init()` | Initialize distributed environment | Called automatically |
| `mx.distributed.size()` | Total number of processes | `4` |
| `mx.distributed.rank()` | Current process rank (0-indexed) | `0`, `1`, `2`, `3` |

## Training Function Pattern

To run MLX training with Kubeflow Trainer, follow this pattern:

### 1. Import Inside Function Body

All imports must be placed inside the function body:

```python
def train_mlx_model():
    """Train model with MLX distributed."""
    # All imports go here
    import mlx.core as mx
    import mlx.nn as nn
    import mlx.optimizers as optim
    from mlx.utils import tree_flatten

    # Rest of your training code...
```

:::{note}
This requirement allows the SDK to serialize and transfer your training function to the cluster without dependency conflicts.
:::

### 2. Initialize Distributed MLX

MLX handles distributed initialization automatically through the `mlx.distributed` module:

```python
def train_mlx_model():
    import mlx.core as mx
    import mlx.distributed as dist

    # Initialize distributed environment (if not already initialized)
    if not dist.is_available():
        raise RuntimeError("MLX distributed is not available")

    # Get distributed info
    world_size = dist.size()
    rank = dist.rank()

    print(f"Process {rank}/{world_size} initialized")

    # Your training code...
```

### 3. Use Gradient Averaging

Synchronize gradients across processes using `nn.average_gradients()`:

```python
def train_mlx_model():
    import mlx.core as mx
    import mlx.nn as nn
    import mlx.distributed as dist

    # ... model and optimizer setup ...

    # Training step
    def train_step(model, batch):
        def loss_fn(model):
            logits = model(batch["input"])
            return nn.losses.cross_entropy(logits, batch["target"])

        # Compute loss and gradients
        loss, grads = nn.value_and_grad(model, loss_fn)(model)

        # Average gradients across all processes
        grads = nn.average_gradients(grads)

        return loss, grads

    # Training loop
    for batch in train_loader:
        loss, grads = train_step(model, batch)
        optimizer.update(model, grads)
        mx.eval(model.parameters())
```

## Complete MLX Training Example

Here's a complete example fine-tuning a Llama model with MLX:

```python
def finetune_llama_mlx():
    """Fine-tune Llama model with MLX distributed training."""
    import mlx.core as mx
    import mlx.nn as nn
    import mlx.optimizers as optim
    import mlx.distributed as dist
    from mlx_lm import load, generate
    from mlx_lm.utils import load_dataset
    from mlx.utils import tree_flatten
    import numpy as np

    # Get distributed info
    world_size = dist.size()
    rank = dist.rank()

    if rank == 0:
        print(f"Starting distributed training with {world_size} processes")

    # Load model and tokenizer
    model_name = "mlx-community/Llama-3.2-1B-Instruct-4bit"
    if rank == 0:
        print(f"Loading model: {model_name}")

    model, tokenizer = load(model_name)

    # Prepare dataset
    if rank == 0:
        print("Loading dataset...")

    # Load and shard dataset across processes
    dataset = load_dataset("wikitext", "wikitext-2-raw-v1", split="train")

    # Shard dataset - each process gets a subset
    dataset_size = len(dataset)
    shard_size = dataset_size // world_size
    start_idx = rank * shard_size
    end_idx = start_idx + shard_size if rank < world_size - 1 else dataset_size
    local_dataset = dataset[start_idx:end_idx]

    if rank == 0:
        print(f"Dataset sharded: {shard_size} samples per process")

    # Tokenize dataset
    def tokenize_batch(examples):
        return tokenizer(
            examples["text"],
            max_length=512,
            truncation=True,
            padding="max_length",
        )

    tokenized_dataset = [
        tokenize_batch({"text": item["text"]})
        for item in local_dataset
    ]

    # Training configuration
    learning_rate = 1e-5
    num_epochs = 3
    batch_size = 4

    # Setup optimizer (LoRA parameters only)
    trainable_params = [
        (name, param) for name, param in model.named_parameters()
        if "lora" in name
    ]

    if rank == 0:
        print(f"Trainable parameters: {len(trainable_params)}")

    optimizer = optim.Adam(learning_rate=learning_rate)

    # Training function with gradient averaging
    def train_step(model, batch):
        def loss_fn(model):
            input_ids = mx.array(batch["input_ids"])
            labels = mx.array(batch["labels"])

            logits = model(input_ids)
            return nn.losses.cross_entropy(logits, labels, reduction="mean")

        # Compute loss and gradients
        loss_and_grad_fn = nn.value_and_grad(model, loss_fn)
        loss, grads = loss_and_grad_fn(model)

        # Average gradients across all processes
        grads = nn.average_gradients(grads)

        return loss, grads

    # Training loop
    if rank == 0:
        print(f"Starting training for {num_epochs} epochs...")

    global_step = 0

    for epoch in range(num_epochs):
        epoch_loss = 0.0
        num_batches = 0

        # Create batches from local dataset
        num_local_batches = len(tokenized_dataset) // batch_size

        for batch_idx in range(num_local_batches):
            start = batch_idx * batch_size
            end = start + batch_size

            batch = {
                "input_ids": [
                    tokenized_dataset[i]["input_ids"]
                    for i in range(start, end)
                ],
                "labels": [
                    tokenized_dataset[i]["input_ids"]
                    for i in range(start, end)
                ]
            }

            # Forward and backward pass
            loss, grads = train_step(model, batch)

            # Update model
            optimizer.update(model, grads)

            # Ensure arrays are evaluated
            mx.eval(model.parameters())

            # Accumulate loss
            epoch_loss += loss.item()
            num_batches += 1
            global_step += 1

            # Log progress (rank 0 only)
            if batch_idx % 50 == 0 and rank == 0:
                print(f"Epoch {epoch}, Step {global_step}, "
                      f"Batch {batch_idx}/{num_local_batches}, "
                      f"Loss: {loss.item():.4f}")

        # Print epoch summary (rank 0 only)
        if rank == 0:
            avg_loss = epoch_loss / num_batches
            print(f"Epoch {epoch} completed. Average Loss: {avg_loss:.4f}")

    # Save model (rank 0 only)
    if rank == 0:
        print("Saving fine-tuned model...")
        model.save_weights("./mlx_model_weights.npz")
        print("Training completed successfully!")

    # Cleanup
    dist.finalize()
```

## Launching MLX Training with SDK

Use the TrainerClient SDK to launch your MLX training job:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()

job_id = client.train(
    trainer=CustomTrainer(
        func=finetune_llama_mlx,
        num_nodes=4,
        resources_per_node={
            "cpu": 8,
            "memory": "32Gi",
            "gpu": 1,  # Apple Silicon GPU or CUDA GPU
        },
        packages_to_install=[
            "mlx>=0.15.0",
            "mlx-lm",
            "transformers",
            "datasets",
        ],
    ),
    runtime="mlx-distributed"
)

print(f"MLX training job created: {job_id}")

# Monitor training logs
for log_line in client.get_job_logs(job_id, follow=True):
    print(log_line)
```

This launches a distributed MLX training job across 4 nodes, automatically handling distributed initialization and gradient averaging.

## Data Parallelism with MLX

MLX distributed training uses data parallelism where:

1. Each device maintains a complete copy of the model
2. Dataset is sharded across devices
3. Each device computes gradients on its data shard
4. Gradients are averaged using `all_sum()` collective operation
5. Model parameters are updated with averaged gradients

### Manual Dataset Sharding

```python
def shard_dataset(dataset, world_size, rank):
    """Shard dataset across distributed processes."""
    dataset_size = len(dataset)
    shard_size = dataset_size // world_size

    start_idx = rank * shard_size
    end_idx = start_idx + shard_size if rank < world_size - 1 else dataset_size

    return dataset[start_idx:end_idx]

# In your training function
import mlx.distributed as dist

world_size = dist.size()
rank = dist.rank()

local_dataset = shard_dataset(full_dataset, world_size, rank)
```

### Gradient Averaging Pattern

```python
import mlx.nn as nn
import mlx.distributed as dist

def distributed_train_step(model, batch, optimizer):
    """Training step with gradient averaging."""

    def loss_fn(model):
        predictions = model(batch["input"])
        return nn.losses.cross_entropy(predictions, batch["target"])

    # Compute gradients locally
    loss, grads = nn.value_and_grad(model, loss_fn)(model)

    # Average gradients across all processes using all-reduce
    grads = nn.average_gradients(grads)

    # Update model with averaged gradients
    optimizer.update(model, grads)

    return loss
```

## MLX with LoRA Fine-tuning

MLX provides excellent support for parameter-efficient fine-tuning with LoRA (Low-Rank Adaptation):

```python
def finetune_with_lora():
    """Fine-tune model using LoRA with MLX."""
    import mlx.core as mx
    import mlx.nn as nn
    from mlx_lm import load
    from mlx_lm.tuner.lora import apply_lora_layers

    # Load base model
    model, tokenizer = load("mlx-community/Llama-3.2-1B-Instruct")

    # Apply LoRA layers
    lora_config = {
        "rank": 8,
        "alpha": 16,
        "dropout": 0.1,
        "target_modules": ["q_proj", "v_proj"]
    }

    model = apply_lora_layers(model, **lora_config)

    # Freeze base model parameters
    for name, param in model.named_parameters():
        if "lora" not in name:
            param.requires_grad = False

    # Train only LoRA parameters
    # ... training loop ...
```

## Cross-Platform Training

One powerful feature of MLX is the ability to train on GPU clusters and evaluate locally:

```python
# Train on GPU cluster with CUDA runtime
job_id = client.train(
    trainer=CustomTrainer(
        func=finetune_llama_mlx,
        num_nodes=4,
        resources_per_node={"gpu": "1"},
    ),
    runtime="mlx-distributed"  # Uses mlx[cuda] on GPU clusters
)

# Wait for training to complete
client.wait_for_job_status(job_id)

# Download weights to local Apple Silicon machine
# (implement your own download logic)

# Evaluate locally on M3 MacBook
import mlx.core as mx
from mlx_lm import load, generate

model, tokenizer = load("mlx-community/Llama-3.2-1B-Instruct")
model.load_weights("./mlx_model_weights.npz")

response = generate(
    model,
    tokenizer,
    prompt="What is machine learning?",
    max_tokens=100
)
print(response)
```

## SDK Integration

The TrainerClient provides comprehensive job management:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()

# List available runtimes
for runtime in client.list_runtimes():
    if "mlx" in runtime.name:
        print(f"MLX Runtime: {runtime.name}")

# Launch training
job_id = client.train(
    trainer=CustomTrainer(
        func=finetune_llama_mlx,
        num_nodes=2,
        resources_per_node={
            "cpu": 4,
            "memory": "16Gi",
            "gpu": 1,
        },
    ),
    runtime="mlx-distributed"
)

# Monitor job status
job = client.get_job(name=job_id)
print(f"Job Status: {job.status}")

# Stream logs from specific node
for log_line in client.get_job_logs(job_id, node_index=0, follow=True):
    print(log_line)

# Wait for completion
final_job = client.wait_for_job_status(job_id)
print(f"Training completed with status: {final_job.status}")
```

## Examples and Resources

### Complete Examples

- **[MLX Llama Fine-tuning](https://github.com/kubeflow/trainer/tree/master/examples/mlx)**: Complete notebook demonstrating Llama model fine-tuning with MLX
- **MLX Image Classification**: Vision model training with MLX (coming soon)

### API Documentation

- [TrainerClient API Reference](../api-reference/python-sdk/index): Complete SDK documentation
- [MLX Documentation](https://ml-explore.github.io/mlx/): Official MLX framework documentation
- [MLX LM](https://github.com/ml-explore/mlx-examples/tree/main/llms): MLX language model examples

### Related Guides

- [Getting Started](../getting-started/index): Initial setup and first training job
- [PyTorch Distributed](pytorch): PyTorch distributed training patterns
- [Local Execution](local-execution/index): Test MLX training locally

## Troubleshooting

### Common Issues

**MLX not available error:**

Ensure MLX is installed in your training environment:

```python
trainer = CustomTrainer(
    func=finetune_llama_mlx,
    packages_to_install=["mlx>=0.15.0", "mlx-lm"],
    # ... other params
)
```

**Out of memory on Apple Silicon:**

MLX uses unified memory on Apple Silicon. To reduce memory usage:

- Reduce batch size
- Use gradient accumulation
- Enable quantization (4-bit or 8-bit)
- Use LoRA for parameter-efficient fine-tuning

```python
# Load quantized model
from mlx_lm import load

model, tokenizer = load(
    "mlx-community/Llama-3.2-1B-Instruct-4bit",
    quantize=True
)
```

**Gradient averaging fails:**

Ensure all processes call `average_gradients()` at the same time:

```python
# Correct: All processes call average_gradients
grads = nn.average_gradients(grads)

# Incorrect: Only rank 0 calls it
if rank == 0:
    grads = nn.average_gradients(grads)  # Will hang!
```

**Metal GPU not detected:**

Verify Metal support on your system:

```python
import mlx.core as mx

# Check available devices
print(f"Metal GPU available: {mx.metal.is_available()}")

# Set default device
mx.set_default_device(mx.gpu)
```

**Import errors with mlx-lm:**

Ensure all MLX dependencies are imported inside the training function:

```python
def train():
    # Correct: All imports inside function
    import mlx.core as mx
    from mlx_lm import load, generate

    # Training code...

# Incorrect: Imports at module level
# import mlx.core as mx  # Don't do this!
```

**CUDA runtime issues on GPU clusters:**

The `mlx-distributed` runtime includes `mlx[cuda]` for GPU clusters. If you encounter CUDA errors:

```python
# Check CUDA availability
import mlx.core as mx
print(f"CUDA available: {mx.cuda.is_available()}")

# Explicitly use CUDA device
mx.set_default_device(mx.Device(mx.gpu, 0))
```
