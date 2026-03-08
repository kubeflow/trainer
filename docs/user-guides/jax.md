# JAX Guide

This guide describes how to use TrainJob to train or fine-tune AI models with
[JAX](https://jax.readthedocs.io/).

---

## Prerequisites

Before exploring this guide, make sure to follow
[the Getting Started guide](index)
to understand the basics of Kubeflow Trainer.

---

## JAX Overview

JAX supports distributed and parallel computation through its
[`jax.distributed`](https://jax.readthedocs.io/en/latest/jax.distributed.html)
module and Single Program, Multiple Data (SPMD) primitives such as `pmap`, `pjit`, and `shard_map`.
These APIs allow you to scale JAX workloads across multiple devices
and multiple nodes.

Kubeflow Trainer integrates with JAX by:
- Typically launching one worker Pod per JAX process (runtime dependent).
- Injecting the required JAX distributed environment variables.
- Providing consistent process indexing for distributed execution.

With Kubeflow Trainer, you can run:
- Multi-process CPU training
- Multi-GPU training using CUDA-enabled JAX
- Data-parallel and model-parallel JAX workloads

:::{note}
The JAX runtime currently supports CPU and GPU workloads only.

TPU workloads are not supported because installing both `jax[cuda]`
and `jax[tpu]` in the same image leads to backend and plugin conflicts.
A separate TPU-specific runtime is required.
:::

---

## JAX Runtime in Kubeflow Trainer

Kubeflow Trainer provides a built-in JAX distributed runtime named
`jax-distributed`.

This runtime:
- Uses the official NVIDIA JAX container image
- Requires no manual configuration of networking

Internally, the runtime maps:
- One Kubernetes Pod → one JAX process
- Multiple devices per Pod → local JAX devices

---

## Get JAX Runtime Packages

Kubeflow Trainer includes a JAX runtime that uses the official
NVIDIA JAX container image. This runtime provides native CPU and GPU
support and comes with a curated set of pre-installed packages.

Run the following command to inspect the runtime packages:

```python
from kubeflow.trainer import TrainerClient

TrainerClient().get_runtime_packages(
   runtime="jax-distributed"
)

```
You should see the installed packages, for example:

```sh
Python: 3.10.12 (main, Feb 25 2026, 20:34:29) [GCC 11.4.0]

Package            Version
------------------ -----------
...
Flax                 0.11.2
jax                  0.7.2
optax                0.2.4
...
```
## JAX Distributed Environment

Your training script must explicitly initialize the JAX distributed runtime before performing any JAX computation.


```python
from kubeflow.trainer import TrainerClient, CustomTrainer


def get_jax_dist():
    import os
    import jax
    import jax.distributed as dist

    # Initialize distributed JAX.
    dist.initialize(
        coordinator_address=os.environ["JAX_COORDINATOR_ADDRESS"],
        num_processes=int(os.environ["JAX_NUM_PROCESSES"]),
        process_id=int(os.environ["JAX_PROCESS_ID"]),
    )

    print("JAX Distributed Environment")
    print(f"Local devices: {jax.local_devices()}")
    print(f"Global device count: {jax.device_count()}")

    import jax.numpy as jnp

    x = jnp.ones((4,))
    y = jax.pmap(lambda v: v * jax.process_index())(x)

    print("PMAP result:", y)


client = TrainerClient()

# Create TrainJob
job_id = client.train(
    runtime=client.get_runtime("jax-distributed"),
    trainer=CustomTrainer(func=get_jax_dist),
)

# Wait until completion
client.wait_for_job_status(job_id)

# Logs are aggregated from node-0
print("\n".join(client.get_job_logs(name=job_id)))
```

### Environment Variables Injected by the JAX Runtime

The jax-distributed runtime injects environment variables such as

| Variable | Description |
|--------|-------------|
| `JAX_NUM_PROCESSES` | Total number of JAX processes |
| `JAX_PROCESS_ID` | Global process index (0-based) |
| `JAX_COORDINATOR_ADDRESS` | Address of the coordinator (process 0) |

---

### Parallelism with JAX Primitives

Once initialized, you can use JAX SPMD primitives normally:

- `pmap` — data-parallel execution
- `pjit` — explicit global sharding
- `shard_map` — low-level SPMD control

Kubeflow Trainer does not alter JAX semantics, it only provides the distributed execution environment.

:::{warning}
All processes must call **jax.distributed.initialize()** exactly once
and before any JAX computation. Failure to do so may result in deadlocks.
:::

---

### Scaling Semantics

In the JAX runtime:

- `num_nodes` controls the number of worker Pods
- Typically one primary JAX process per worker Pod, depending on runtime implementation.
- All Pods run identical code

Results:

- 2 Pods
- 2 JAX processes
- Enabling a single global SPMD execution across processes.

---

## Creating a TrainJob Example with Python SDK

Kubeflow Trainer provides a Python SDK that allows you to
programmatically create and submit TrainJobs without writing YAML.

```python
def train_mnist_jax():
    import os
    import jax
    import jax.numpy as jnp
    import jax.distributed as dist
    import optax
    from flax import linen as nn
    from flax.training import train_state
    import tensorflow_datasets as tfds

    # Initialize distributed JAX
    dist.initialize(
        num_processes=int(os.environ["JAX_NUM_PROCESSES"]),
        process_id=int(os.environ["JAX_PROCESS_ID"]),
        coordinator_address=os.environ["JAX_COORDINATOR_ADDRESS"],
    )

    process_index = jax.process_index()
    local_device_count = jax.local_device_count()

    print("Process:", process_index)
    print("Global devices:", jax.device_count())
    print("Local devices:", jax.local_devices())

    # Model definition
    class CNN(nn.Module):
        @nn.compact
        def __call__(self, x):
            x = nn.Conv(features=32, kernel_size=(3, 3))(x)
            x = nn.relu(x)
            x = nn.avg_pool(x, (2, 2), (2, 2))
            x = x.reshape((x.shape[0], -1))
            x = nn.Dense(128)(x)
            x = nn.relu(x)
            x = nn.Dense(10)(x)
            return x

    # Dataset
    ds = tfds.load("mnist", split="train", as_supervised=True)

    def preprocess(image, label):
        image = jnp.array(image, dtype=jnp.float32) / 255.0
        image = jnp.expand_dims(image, -1)
        return image, label

    ds = ds.map(preprocess).batch(128).prefetch(1)
    ds = tfds.as_numpy(ds)

    # Training setup
    model = CNN()
    rng = jax.random.PRNGKey(0)

    params = model.init(rng, jnp.ones([1, 28, 28, 1]))["params"]

    tx = optax.adam(1e-3)

    state = train_state.TrainState.create(
        apply_fn=model.apply,
        params=params,
        tx=tx,
    )

    # replicate state across local devices
    state = jax.device_put_replicated(state, jax.local_devices())

    # Training step
    def loss_fn(params, batch):
        images, labels = batch
        logits = model.apply({"params": params}, images)
        onehot = jax.nn.one_hot(labels, 10)
        loss = optax.softmax_cross_entropy(logits, onehot).mean()
        return loss

    grad_fn = jax.value_and_grad(loss_fn)

    @jax.pmap
    def train_step(state, batch):
        loss, grads = grad_fn(state.params, batch)
        state = state.apply_gradients(grads=grads)
        return state, loss

    # Training loop
    for epoch in range(5):
        for images, labels in ds:
            # shard batch per device
            images = images.reshape(
                (local_device_count, -1, 28, 28, 1)
            )
            labels = labels.reshape(
                (local_device_count, -1)
            )

            state, loss = train_step(state, (images, labels))

        if process_index == 0:
            print(f"Epoch {epoch}, Loss: {loss.mean()}")
```

### Create a TrainJob

- After defining the training function, create a TrainJob using the SDK.

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

job_id = TrainerClient().train(
    runtime=TrainerClient().get_runtime("jax-distributed"),
    trainer=CustomTrainer(
        func=train_mnist_jax,
        packages_to_install=[
            "tensorflow-datasets",
            "flax",
            "optax",
        ],
        num_nodes=2,
        resources_per_node={
            "gpu": 1,
        },
    ),
)
```


### Get the TrainJob Results
```python
print("\n".join(TrainerClient().get_job_logs(name=job_id)))
```
---


## Next Steps

- Check out [the MNIST JAX example](https://github.com/kubeflow/trainer/blob/master/examples/jax/image-classification/mnist.ipynb).
- Learn more about `TrainerClient()` APIs [in the Kubeflow SDK](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py).
