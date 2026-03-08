# JAX

Train JAX models with distributed parallelism using Kubeflow Trainer's seamless multi-process orchestration.

## Overview

Kubeflow Trainer provides first-class support for distributed JAX training on Kubernetes. The `jax-distributed` runtime handles cluster coordination, networking setup, and environment configuration automatically, enabling you to scale JAX workloads across multiple nodes effortlessly.

Key features:
- **Automatic multi-process setup**: JAX distributed environment configured automatically
- **Official NVIDIA containers**: Pre-built images with JAX, Flax, and Optax
- **Flexible parallelism**: Support for pmap, pjit, and shard_map
- **GPU and CPU support**: Train on any hardware (TPU not supported)

## Prerequisites

Before following this guide, ensure you have:

- Completed the [Getting Started](../getting-started/index) guide
- Access to a Kubernetes cluster with Kubeflow Trainer installed
- Familiarity with JAX and distributed training concepts

## The jax-distributed Runtime

The `jax-distributed` runtime is a ClusterTrainingRuntime that provides:

- **Official NVIDIA JAX containers** with pre-installed JAX, Flax, and Optax
- **Automatic networking configuration** for multi-process communication
- **One Pod = One JAX process** architecture for simplicity
- **GPU and CPU support** (TPU not supported due to backend conflicts)

### Architecture

Unlike PyTorch's process-per-GPU model, JAX uses a **one-process-per-pod** architecture:

- Each Kubernetes Pod runs a single JAX process
- JAX automatically discovers and uses all GPUs within a Pod
- Processes communicate via JAX's distributed runtime

### Automatic Environment Variables

Kubeflow Trainer automatically configures these environment variables for JAX distributed training:

| Variable | Description | Example |
|----------|-------------|---------|
| `JAX_NUM_PROCESSES` | Total number of JAX processes across all nodes | `4` |
| `JAX_PROCESS_ID` | Global process index (zero-based) | `0`, `1`, `2`, `3` |
| `JAX_COORDINATOR_ADDRESS` | Network address of the primary process (process 0) | `trainjob-mnist-0.trainjob-mnist:1234` |

These variables are used internally by `jax.distributed.initialize()` to establish multi-process communication.

## Training Function Pattern

To run distributed JAX training with Kubeflow Trainer, follow this pattern:

### 1. Import Inside Function Body

All imports must be inside your training function:

```python
def train_jax():
    """Train model with JAX distributed."""
    # All imports go here
    import jax
    import jax.numpy as jnp
    from flax import linen as nn

    # Rest of your training code...
```

:::{note}
This requirement allows the TrainerClient SDK to serialize your function and transfer it to the cluster.
:::

### 2. Initialize JAX Distributed

Call `jax.distributed.initialize()` once at the start of your training function:

```python
def train_jax():
    import jax
    import jax.distributed

    # Initialize JAX distributed runtime
    # This MUST be called before any JAX operations
    jax.distributed.initialize()

    # Now you can use JAX distributed operations
    process_id = jax.process_index()
    num_processes = jax.process_count()

    print(f"Process {process_id} of {num_processes} initialized")

    # Your training code...
```

:::{warning}
`jax.distributed.initialize()` must be called **before** any JAX computations. Calling it after JAX operations will raise an error.
:::

### 3. Use Process Index for Single-Process Operations

Similar to PyTorch's rank checks, use `jax.process_index()` for operations that should run on a single process:

```python
def train_jax():
    import jax

    jax.distributed.initialize()

    # Only process 0 downloads the dataset
    if jax.process_index() == 0:
        download_dataset()

    # Wait for process 0 to finish
    jax.experimental.multihost_utils.sync_global_devices("download_complete")

    # All processes can now load the dataset
    dataset = load_dataset()
```

## Parallelism Strategies

JAX provides multiple approaches to distributed parallelism. Choose based on your model architecture and scaling needs.

### pmap: Data Parallel Execution

`pmap` (parallel map) is the simplest approach for data parallelism. It replicates your model across all devices and processes different data batches in parallel.

**When to use:**
- Standard data-parallel training
- Model fits comfortably on a single device
- You want the simplest distributed setup

**Example:**

```python
import jax
import jax.numpy as jnp
from flax import linen as nn

# Define model
class SimpleCNN(nn.Module):
    @nn.compact
    def __call__(self, x):
        x = nn.Conv(features=32, kernel_size=(3, 3))(x)
        x = nn.relu(x)
        x = nn.avg_pool(x, window_shape=(2, 2), strides=(2, 2))
        x = x.reshape((x.shape[0], -1))
        x = nn.Dense(features=10)(x)
        return x

# Replicate parameters across devices
params = replicate_across_devices(initial_params)

# Training step (runs on all devices in parallel)
@jax.pmap
def train_step(params, batch):
    def loss_fn(params):
        logits = model.apply(params, batch['image'])
        return jnp.mean((logits - batch['label']) ** 2)

    loss, grads = jax.value_and_grad(loss_fn)(params)
    # Gradients are automatically averaged across devices
    return loss, grads
```

### pjit: Explicit Global Sharding

`pjit` (partitioned jit) allows fine-grained control over how arrays are sharded across devices. It's ideal for large models that need model parallelism.

**When to use:**
- Large models that don't fit on a single device
- You need model parallelism (sharding weights across devices)
- You want explicit control over sharding strategies

**Example:**

```python
import jax
from jax.experimental import mesh_utils
from jax.sharding import Mesh, PartitionSpec as P

# Create device mesh for sharding
devices = mesh_utils.create_device_mesh((4,))  # 4-way data parallelism
mesh = Mesh(devices, axis_names=('data',))

# Define sharding strategy
with mesh:
    # Shard data batch across devices
    sharded_batch = jax.device_put(batch, P('data'))

    # Replicate model parameters (not sharded)
    replicated_params = jax.device_put(params, P())

    @jax.jit
    def train_step(params, batch):
        # Training logic here
        return updated_params
```

### shard_map: Low-Level SPMD Control

`shard_map` provides the most control for Single Program Multiple Data (SPMD) parallelism.

**When to use:**
- You need full control over computation and communication patterns
- Implementing custom parallelism strategies
- Advanced use cases beyond pmap and pjit

## Complete MNIST Example with Flax

Here's a complete example training a CNN on MNIST with JAX distributed:

```python
def train_mnist_jax():
    """Train MNIST CNN with JAX and Flax using distributed training."""
    import jax
    import jax.numpy as jnp
    import jax.distributed
    import optax
    from flax import linen as nn
    from flax.training import train_state
    import tensorflow_datasets as tfds

    # Initialize JAX distributed (MUST be first)
    jax.distributed.initialize()

    process_id = jax.process_index()
    num_processes = jax.process_count()
    print(f"JAX process {process_id} of {num_processes} started")

    # Define CNN model with Flax
    class MNISTNet(nn.Module):
        @nn.compact
        def __call__(self, x):
            x = nn.Conv(features=32, kernel_size=(3, 3))(x)
            x = nn.relu(x)
            x = nn.avg_pool(x, window_shape=(2, 2), strides=(2, 2))
            x = nn.Conv(features=64, kernel_size=(3, 3))(x)
            x = nn.relu(x)
            x = nn.avg_pool(x, window_shape=(2, 2), strides=(2, 2))
            x = x.reshape((x.shape[0], -1))
            x = nn.Dense(features=128)(x)
            x = nn.relu(x)
            x = nn.Dense(features=10)(x)
            return x

    # Load MNIST dataset
    # Only process 0 downloads
    if process_id == 0:
        print("Downloading MNIST dataset...")
        ds_builder = tfds.builder('mnist')
        ds_builder.download_and_prepare()

    # Synchronize all processes
    jax.experimental.multihost_utils.sync_global_devices("dataset_download")

    # All processes load the dataset
    ds = tfds.load('mnist', split='train', shuffle_files=True)

    # Preprocess dataset
    def preprocess(sample):
        image = sample['image'].astype(jnp.float32) / 255.0
        label = sample['label']
        return {'image': image, 'label': label}

    ds = ds.map(preprocess)
    ds = ds.batch(64)
    ds = ds.prefetch(10)

    # Initialize model and optimizer
    model = MNISTNet()
    rng = jax.random.PRNGKey(0)
    sample_input = jnp.ones([1, 28, 28, 1])
    params = model.init(rng, sample_input)

    optimizer = optax.adam(learning_rate=0.001)

    # Create training state
    class TrainState(train_state.TrainState):
        pass

    state = TrainState.create(
        apply_fn=model.apply,
        params=params,
        tx=optimizer,
    )

    # Replicate state across all local devices
    num_devices = jax.local_device_count()
    state = jax.device_put_replicated(state, jax.local_devices())

    # Define training step with pmap
    @jax.pmap
    def train_step(state, batch):
        def loss_fn(params):
            logits = state.apply_fn(params, batch['image'])
            one_hot = jax.nn.one_hot(batch['label'], 10)
            loss = jnp.mean(optax.softmax_cross_entropy(logits, one_hot))
            return loss

        loss, grads = jax.value_and_grad(loss_fn)(state.params)
        # Average gradients across devices
        grads = jax.lax.pmean(grads, axis_name='batch')
        state = state.apply_gradients(grads=grads)
        return state, loss

    # Training loop
    num_epochs = 5
    for epoch in range(num_epochs):
        epoch_loss = 0.0
        num_batches = 0

        for batch in tfds.as_numpy(ds):
            # Reshape batch for pmap (devices, batch_per_device, ...)
            batch_images = batch['image'].reshape(
                (num_devices, -1, 28, 28, 1)
            )
            batch_labels = batch['label'].reshape((num_devices, -1))

            pmap_batch = {
                'image': batch_images,
                'label': batch_labels,
            }

            state, loss = train_step(state, pmap_batch)

            # Aggregate loss from all devices
            loss_value = jnp.mean(loss)
            epoch_loss += loss_value
            num_batches += 1

            if num_batches % 100 == 0 and process_id == 0:
                print(f"Epoch {epoch}, Batch {num_batches}, Loss: {loss_value:.4f}")

        # Print epoch summary
        if process_id == 0:
            avg_loss = epoch_loss / num_batches
            print(f"Epoch {epoch} completed. Average Loss: {avg_loss:.4f}")

    # Save model (process 0 only)
    if process_id == 0:
        # Extract params from first device replica
        final_params = jax.tree_util.tree_map(lambda x: x[0], state.params)
        print("Training completed!")

    print(f"Process {process_id} finished training")
```

## SDK Integration

Launch JAX training jobs using the TrainerClient:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()

# Launch distributed JAX training
job_id = client.train(
    trainer=CustomTrainer(
        func=train_mnist_jax,
        num_nodes=4,  # 4 JAX processes
        resources_per_node={
            "cpu": 8,
            "memory": "32Gi",
            "gpu": 2,  # Each process will use 2 GPUs
        },
    )
)

print(f"JAX training job created: {job_id}")

# Monitor training progress
for log_line in client.get_job_logs(job_id, follow=True):
    print(log_line)
```

### Multi-Process Configuration

JAX processes are configured via `num_nodes`:

- `num_nodes=1`: Single process, all local GPUs
- `num_nodes=4`: 4 processes, distributed across nodes
- `num_nodes=8`: 8 processes for large-scale training

Each process will automatically discover and use all GPUs allocated to its Pod.

## Examples and Resources

### Complete Examples

- **[JAX MNIST Classification](https://github.com/kubeflow/trainer/blob/master/examples/jax/image-classification/mnist.ipynb)**: Interactive notebook with complete MNIST training example

### API Documentation

- [TrainerClient API Reference](../api-reference/python-sdk/index): Complete SDK documentation
- [TrainJob CRD Reference](../api-reference/crd-types/trainjob): TrainJob specification details

### Related Guides

- [Getting Started](../getting-started/index): Initial setup and first training job
- [PyTorch Guide](pytorch): Distributed PyTorch training with DDP and FSDP
- [Local Execution](local-execution/index): Test JAX training locally before deploying

### External Resources

- [JAX Documentation](https://jax.readthedocs.io/): Official JAX documentation
- [Flax Documentation](https://flax.readthedocs.io/): Neural network library for JAX
- [JAX Distributed Guide](https://jax.readthedocs.io/en/latest/multi_process.html): Multi-process JAX training

## Troubleshooting

### Common Issues

**Import errors with `CustomTrainer`:**

Ensure all imports are inside your training function body, not at the module level.

**JAX distributed initialization fails:**

Verify that `jax.distributed.initialize()` is called before any JAX operations:

```python
def train_jax():
    import jax
    import jax.distributed

    # MUST be first
    jax.distributed.initialize()

    # Now safe to use JAX
    devices = jax.devices()
```

**Process hanging or timeout:**

Check that all processes reach synchronization barriers:

```python
# All processes must call this
jax.experimental.multihost_utils.sync_global_devices("checkpoint")
```

**Out of memory errors:**

- Reduce batch size per device
- Use gradient accumulation
- Enable mixed precision with `jax.default_matmul_precision('tensorfloat32')`
- Shard large model parameters with pjit

**TPU backend not available:**

TPU is not supported with the `jax-distributed` runtime due to backend conflicts. Use GPU or CPU configurations instead.

**Uneven device utilization:**

JAX automatically balances computation across devices within a process. If you see imbalance:

1. Ensure data batches are evenly divisible by device count
2. Use `pmap` with proper batch sharding
3. Check that `jax.local_device_count()` matches your GPU allocation
