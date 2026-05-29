# JAX on TPU Guide

This guide describes how to use TrainJob to train or fine-tune AI models with
[JAX](https://jax.readthedocs.io/) on Cloud TPU on Google Kubernetes Engine (GKE).

---

## Prerequisites

Before exploring this guide, make sure to follow:
- [The Getting Started guide](../getting-started/index.md)
- [GKE Cloud TPU documentation](https://cloud.google.com/kubernetes-engine/docs/concepts/tpus) to set up a GKE cluster with TPU nodes. For example, for an autopilot GKE cluster, you can create a [TPU custom ComputeClass](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/tpus#custom-compute-classes) like
```
apiVersion: cloud.google.com/v1
kind: ComputeClass
metadata:
  name: tpu-multihost-v5-8
spec:
  priorities:
  - tpu:
      type: tpu-v5-lite-podslice
      count: 4
      topology: 2x4
  nodePoolAutoCreation:
    enabled: true
```

---

## JAX on TPU Overview

JAX on TPU requires a different runtime environment than GPU. Specifically:
- **Image**: You must use a JAX image compatible with TPUs (e.g., `us-docker.pkg.dev/cloud-tpu-images/jax-ai-image/tpu`).
- **Resources**: You must request `google.com/tpu` resources.
- **Node Selectors**: You must specify GKE-specific node selectors and topology for TPU nodes.
- **Environment Variables**: Some TPU-specific JAX environment variables might be required depending on your JAX version.

:::{note}
The built-in `jax-distributed` runtime is optimized for GPUs. For TPU workloads, you can override the runtime configuration using the Python SDK.
:::

---

## JAX Distributed Environment on TPU

Your training script must explicitly initialize the JAX distributed runtime.

```python
from kubeflow.trainer import TrainerClient, CustomTrainer
from kubeflow.trainer.options import kubernetes as k8s_options

def get_jax_tpu_dist():
    import os
    import jax
    import jax.distributed as dist

    # Initialize distributed JAX.
    dist.initialize(
        coordinator_address=os.environ["JAX_COORDINATOR_ADDRESS"],
        num_processes=int(os.environ["JAX_NUM_PROCESSES"]),
        process_id=int(os.environ["JAX_PROCESS_ID"]),
    )

    print("JAX Distributed Environment on TPU")
    print(f"Local devices: {jax.local_devices()}")
    print(f"Global device count: {jax.device_count()}")
    print(f"Process index: {jax.process_index()}")

    import jax.numpy as jnp

    # Use local_device_count for the leading axis of the local input to pmap
    x = jnp.ones((jax.local_device_count(),))
    
    # Pass process_index as a sharded argument to ensure SPMD consistency across all processes in the distributed job.
    p_idx = jnp.array([jax.process_index()] * jax.local_device_count())
    
    y = jax.pmap(lambda v, p: v * p)(x, p_idx)

    print("PMAP result:", y)

client = TrainerClient()

# Define TPU Node Selectors and Tolerations
# Replace with your GKE TPU configuration
node_selector = {
    "cloud.google.com/compute-class": "tpu-multihost-v5-8",
    "cloud.google.com/gke-tpu-accelerator": "tpu-v5-lite-podslice",
    "cloud.google.com/gke-tpu-topology": "2x4",
}

job_patch = k8s_options.RuntimePatch(
    training_runtime_spec=k8s_options.TrainingRuntimeSpecPatch(
        template=k8s_options.JobSetTemplatePatch(
            spec=k8s_options.JobSetSpecPatch(
                replicated_jobs=[
                    k8s_options.ReplicatedJobPatch(
                        name="node",
                        template=k8s_options.JobTemplatePatch(
                            spec=k8s_options.JobSpecPatch(
                                template=k8s_options.PodTemplatePatch(
                                    spec=k8s_options.PodSpecPatch(
                                        node_selector=node_selector,
                                        tolerations=[
                                            {
                                                "key": "google.com/tpu",
                                                "operator": "Exists",
                                                "effect": "NoSchedule",
                                            },
                                            {
                                                "key": "cloud.google.com/compute-class",
                                                "operator": "Exists",
                                                "effect": "NoSchedule",
                                            },
                                        ],
                                    )
                                )
                            )
                        )
                    )
                ]
            )
        )
    )
)

# Create TrainJob
job_id = client.train(
    runtime="jax-distributed",
    trainer=CustomTrainer(
        func=get_jax_tpu_dist,
        image="us-docker.pkg.dev/cloud-tpu-images/jax-ai-image/tpu:latest",
        num_nodes=2,
        resources_per_node={
            "google.com/tpu": 4,
        },
        env={
            "JAX_PLATFORMS": "tpu,cpu",
            "ENABLE_PJRT_COMPATIBILITY": "true",
        }
    ),
    options=[job_patch],
)

# Wait until completion
client.wait_for_job_status(job_id)

# Logs are aggregated from node-0
print("\n".join(client.get_job_logs(name=job_id)))
```

---

## End-to-end Training Example

The following example demonstrates how to train a simple CNN on the MNIST dataset using JAX on multihost TPUs.

```python
from kubeflow.trainer import TrainerClient, CustomTrainer
from kubeflow.trainer.options import kubernetes as k8s_options

def train_mnist_jax():
    import os
    import jax
    import jax.numpy as jnp
    import jax.distributed as dist
    import optax
    from flax import linen as nn
    from flax.training import train_state
    import tensorflow_datasets as tfds
    import tensorflow as tf

    # Initialize distributed JAX
    dist.initialize(
        coordinator_address=os.environ["JAX_COORDINATOR_ADDRESS"],
        num_processes=int(os.environ["JAX_NUM_PROCESSES"]),
        process_id=int(os.environ["JAX_PROCESS_ID"]),
    )

    # Prevent TF from grabbing TPU
    tf.config.set_visible_devices([], 'TPU')

    process_index = jax.process_index()
    local_device_count = jax.local_device_count()

    print(f"Process: {process_index}")
    print(f"Global devices: {jax.device_count()}")
    print(f"Local devices: {jax.local_devices()}")

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

    # Dataset sharding:
    # In JAX's SPMD model, each process runs the same code but should handle
    # different data to increase throughput. Without sharding, every node
    # would process the same images, wasting compute.
    ds = tfds.load("mnist", split="train", as_supervised=True)
    ds = ds.shard(num_shards=jax.process_count(), index=process_index)

    def preprocess(image, label):
        image = tf.cast(image, tf.float32) / 255.0
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

    def train_step(state, batch):
        loss, grads = grad_fn(state.params, batch)
        # Average gradients across all devices.
        # We must bind "batch" axis in jax.pmap for pmean to work.
        grads = jax.lax.pmean(grads, axis_name="batch")
        state = state.apply_gradients(grads=grads)
        return state, loss

    train_step = jax.pmap(train_step, axis_name="batch")

    # Training loop
    for epoch in range(5):
        for images, labels in ds:
            # Convert to jnp and shard batch per local device
            images = jnp.array(images).reshape(
                (local_device_count, -1, 28, 28, 1)
            )
            labels = jnp.array(labels).reshape(
                (local_device_count, -1)
            )

            state, loss = train_step(state, (images, labels))

        if process_index == 0:
            print(f"Epoch {epoch}, Loss: {loss.mean()}")

client = TrainerClient()

# Define TPU Node Selectors and Tolerations
node_selector = {
    "cloud.google.com/compute-class": "tpu-multihost-v5-8",
    "cloud.google.com/gke-tpu-accelerator": "tpu-v5-lite-podslice",
    "cloud.google.com/gke-tpu-topology": "2x4",
}

job_patch = k8s_options.RuntimePatch(
    training_runtime_spec=k8s_options.TrainingRuntimeSpecPatch(
        template=k8s_options.JobSetTemplatePatch(
            spec=k8s_options.JobSetSpecPatch(
                replicated_jobs=[
                    k8s_options.ReplicatedJobPatch(
                        name="node",
                        template=k8s_options.JobTemplatePatch(
                            spec=k8s_options.JobSpecPatch(
                                template=k8s_options.PodTemplatePatch(
                                    spec=k8s_options.PodSpecPatch(
                                        node_selector=node_selector,
                                        tolerations=[
                                            {
                                                "key": "google.com/tpu",
                                                "operator": "Exists",
                                                "effect": "NoSchedule",
                                            },
                                            {
                                                "key": "cloud.google.com/compute-class",
                                                "operator": "Exists",
                                                "effect": "NoSchedule",
                                            },
                                        ],
                                    )
                                )
                            )
                        )
                    )
                ]
            )
        )
    )
)

job_id = client.train(
    runtime="jax-distributed",
    trainer=CustomTrainer(
        func=train_mnist_jax,
        image="us-docker.pkg.dev/cloud-tpu-images/jax-ai-image/tpu:latest",
        num_nodes=2,
        resources_per_node={
            "google.com/tpu": 4,
        },
        env={
            "JAX_PLATFORMS": "tpu,cpu",
            "ENABLE_PJRT_COMPATIBILITY": "true",
        },
        packages_to_install=[
            "tensorflow-datasets",
            "flax",
            "optax",
            "tensorflow",
        ],
    ),
    options=[job_patch],
)

client.wait_for_job_status(job_id)
print("\n".join(client.get_job_logs(name=job_id)))
```

---

## TPU Specific Configurations

### Node Selectors and Topology

When running on GKE, TPUs are managed via specific node pools and you must match the proper `node_selector` and `tolerations` to your TPU node pool labels. 
If you are using [custom ComputeClasses](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/tpus#custom-compute-classes), add the following `node_selector` and `tolerations` to your TPU node pool labels:

| Label | Example Value |
|-------|---------------|
| `cloud.google.com/compute-class` | `tpu-multihost-v5-8` |
| `cloud.google.com/gke-tpu-accelerator` | `tpu-v5-lite-podslice` |
| `cloud.google.com/gke-tpu-topology` | `2x4` |

| Toleration Key | Toleration Operator | Toleration Effect |
|-------|---------------|---------------|
| `google.com/tpu` | `Exists` | `NoSchedule` |
| `cloud.google.com/compute-class` | `Exists` | `NoSchedule` |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `JAX_PLATFORMS` | Set to `tpu,cpu` to ensure JAX uses the TPU backend. |
| `ENABLE_PJRT_COMPATIBILITY` | Set to `true` for compatibility with newer JAX/LibTPU versions. |

---

## Reusable ClusterTrainingRuntime

Instead of setting the node selectors and tolerations via the Python SDK for every TrainJob, cluster administrators can configure a reusable `ClusterTrainingRuntime` for a specific TPU type and topology. This simplifies the user experience by moving infrastructure-specific details out of the training script.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: jax-tpu-v5-8
  labels:
    trainer.kubeflow.org/framework: jax
spec:
  mlPolicy:
    numNodes: 2
    jax: {}
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  nodeSelector:
                    cloud.google.com/compute-class: "tpu-multihost-v5-8"
                    cloud.google.com/gke-tpu-accelerator: "tpu-v5-lite-podslice"
                    cloud.google.com/gke-tpu-topology: "2x4"
                  tolerations:
                    - key: "google.com/tpu"
                      operator: "Exists"
                      effect: "NoSchedule"
                    - key: "cloud.google.com/compute-class"
                      operator: "Exists"
                      effect: "NoSchedule"
                  containers:
                    - name: node
                      image: us-docker.pkg.dev/cloud-tpu-images/jax-ai-image/tpu:latest
                      resources:
                        limits:
                          google.com/tpu: 4
```

Users can then submit TrainJobs referencing this runtime without specifying the target infrastructure details, such as the `job_patch`, base `image`, or TPU resource limits:

```python
job_id = client.train(
    runtime="jax-tpu-v5-8",
    trainer=CustomTrainer(
        func=train_mnist_jax,
        env={
            "JAX_PLATFORMS": "tpu,cpu",
            "ENABLE_PJRT_COMPATIBILITY": "true",
        },
        packages_to_install=[
            "tensorflow-datasets",
            "flax",
            "optax",
            "tensorflow",
        ],
    ),
)
```

---

## Next Steps

- Learn more about [JAX distributed training](https://jax.readthedocs.io/en/latest/jax.distributed.html).
- Explore [GKE Cloud TPU best practices](https://cloud.google.com/kubernetes-engine/docs/how-to/tpus).
