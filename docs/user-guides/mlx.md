# MLX Guide

This guide describes how to use TrainJob to train or fine-tune AI models with [MLX](https://ml-explore.github.io/mlx/build/html/index.html).

## Prerequisites

Before exploring this guide, make sure to follow [the Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.

## MLX Distributed Overview

MLX is a NumPy-like array framework designed for efficient and flexible machine learning, created by
Apple machine learning researchers. The main differences between MLX and NumPy are:

- **Composable function transformations:** MLX has composable function transformations for automatic
 differentiation, automatic vectorization, and computation graph optimization.

- **Lazy computation:** Computations in MLX are lazy. Arrays are only materialized when needed.

- **Multi-device:** Operations can run on any of the supported devices (CPU, GPU, etc.)

In Kubeflow Trainer, MLX distributed training is supported via the MPI backend which enables:

- **Data Parallelism**: The dataset is sharded across multiple devices, with each device processing
 a partition of the data and maintaining a copy of the model.
- **Gradient Averaging**: Gradients are computed locally and then averaged across all processes
 using efficient communication primitives like `all_sum()`.
- **Automatic Process Management**: MLX handles process initialization and communication setup
 through the `mlx.distributed` module.

## MLX with CUDA Backend

Kubeflow Trainer includes an MLX runtime called [`mlx-distributed`](https://github.com/kubeflow/trainer/blob/master/manifests/base/runtimes/mlx_distributed.yaml).
This runtime installs the CUDA driver and `mlx[cuda]` package to enable MLX on distributed GPUs.
Currently, you can't use this runtime for non-GPU TrainJobs.

This setup is especially powerful: you can train or fine-tune models on a GPU cluster and then
seamlessly evaluate them locally on an Apple silicon machine like in
[this MNIST example](https://github.com/kubeflow/trainer/tree/master/examples/mlx/image-classification/mnist.ipynb).

## Get MLX Runtime Packages

MLX runtime comes with several pre-installed Python packages.

Run the following command to get a list of the available packages:

```py
from kubeflow.trainer import TrainerClient

TrainerClient().get_runtime_packages(
    runtime=TrainerClient().get_runtime("mlx-distributed")
)
```

You should see the installed packages, for example:

```sh
Python: 3.10.12 (main, May 27 2025, 17:12:29) [GCC 11.4.0]
Package                Version
---------------------- -----------
...
mlx                    0.28.0
mlx-cuda               0.28.0
mlx-data               0.1.0
mlx-lm                 0.26.3
...
```

## MLX Distributed Environment

Kubeflow Trainer uses the MPI-based runtime and [`mpirun` launcher](https://ml-explore.github.io/mlx/build/html/usage/distributed.html#installing-mpi)
to run MLX scripts on every training node. It automatically creates the OpenMPI hostfile to ensure
MLX can discover all MPI nodes, starts the OpenSSH server on the worker nodes, and configures
the distributed MLX environment:

- `mx.distributed.size()` - Total number of processes across all MLX nodes.
- `mx.distributed.rank()` - Rank of the current process across all MLX nodes.

You can use these values to, for example, load different data shards on each process,
or evaluate your fine-tuned model only on the process with `rank=0` (e.g., the master process).

You can access the distributed environment as follows:

```py
from kubeflow.trainer import TrainerClient, CustomTrainer

def get_mlx_dist():
    import mlx.core as mx

    # Initialize MLX distributed backend.
    dist = mx.distributed.init()

    print("MLX Distributed Environment")
    print(f"WORLD_SIZE: {dist.size()}")
    print(f"RANK: {dist.rank()}")
    print(f"Device: {mx.default_device()}")

# Create the TrainJob on 3 nodes.
job_id = TrainerClient().train(
    runtime="mlx-distributed",
    trainer=CustomTrainer(
        func=get_mlx_dist,
        num_nodes=3,
        resources_per_node={
            "gpu": 1,
        },
    ),
)

# Wait for TrainJob to complete.
TrainerClient().wait_for_job_status(job_id)


# Since we launch MLX with `mpirun`, all logs should be consumed from the node-0.
print("\n".join(TrainerClient().get_job_logs(name=job_id)))
```

You should see the distributed environment as follows:

```shell
MLX Distributed Environment
WORLD_SIZE: 3
RANK: 0
Device: Device(gpu, 0)
```

## Fine-Tune LLM with MLX and TrainJob

The following sections describe how to fine-tune Llama using MLX and Kubeflow Trainer.

### Configure MLX Training Function

You can leverage [the `mlx-lm` library](https://github.com/ml-explore/mlx-lm?tab=readme-ov-file) to
fine-tune and evaluate LLMs. It provides builtin functions for rapid fine-tuning with Low-Rank
Adaptation (LoRA) as well as full model fine-tuning, with support for quantized models.

Wrap your MLX code inside a function and create a TrainJob with the `CustomTrainer()`. Your function
should handle the end-to-end MLX script including distributed fine-tuning and model evaluation.

:::{note}
All necessary imports must be included inside the function body so that the TrainJob can recognize
them on every training node.
:::

Your training function might look like this:

```py
def fine_tune_llama():
    import types
    import os
    import mlx.core as mx
    from mlx_lm.lora import train_model, CONFIG_DEFAULTS
    from mlx_lm.tuner.datasets import load_dataset
    from mlx_lm.utils import load
    from mlx_lm.generate import generate

    # Set parameters for the mlx-lm.
    args = types.SimpleNamespace()
    args.model = "meta-llama/Llama-3.2-3B-Instruct"
    args.data = "mlx-community/WikiSQL"
    args.train = True

    # Set defaults for other required parameters.
    for k, v in CONFIG_DEFAULTS.items():
        if not hasattr(args, k):
            setattr(args, k, v)

    # Load pre-trained model and dataset, set your HF token.
    os.environ["HF_TOKEN"] = "<YOUR_HF_TOKEN>"  # Replace with your Hugging Face token.
    model, tokenizer = load(args.model)
    train_set, valid_set, _ = load_dataset(args, tokenizer)

    # Start the Llama distributed fine-tuning.
    train_model(args, model, train_set, valid_set)

    # Evaluate the fine-tuned adapter.
    dist = mx.distributed.init(strict=True, backend="mpi")
    if dist.rank() == 0:
        finetuned_model, finetuned_tokenizer = load(
            args.model, adapter_path=args.adapter_path
        )
        # Pass prompt to the fine-tuned LLM.
        print(
            generate(
                model=finetuned_model,
                tokenizer=finetuned_tokenizer,
                prompt="What is SQL?",
                max_tokens=1000,
            )
        )
```

### Create a TrainJob

After configuring the MLX training function, use the `train()` API to create a TrainJob:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

job_id = TrainerClient().train(
    runtime="mlx-distributed",
    trainer=CustomTrainer(
        func=fine_tune_llama,
        num_nodes=2,
        resources_per_node={
            "gpu": 1,
        },
    ),
)
```

### Get the TrainJob Results

You can use the `get_job_logs()` API to see your TrainJob logs:

```py
print("\n".join(TrainerClient().get_job_logs(name=job_id)))
```

:::{note}
Since MLX training is launched via the mpirun command, all logs can be collected from
node-0, which acts as the OpenMPI launcher.
:::

You should see the fine-tuning results as follows:

```shell
Trainable parameters: 0.041% (1.311M/3212.750M)
Starting training..., iters: 1000
Node 0 of 2
Node 1 of 2
Calculating loss...: 100%|██████████| 25/25 [00:28<00:00,  1.13s/it]
Calculating loss...: 100%|██████████| 25/25 [00:28<00:00,  1.13s/it]
Iter 1: Val loss 2.935, Val took 28.274s
Iter 10: Train loss 2.687, Learning Rate 1.000e-05, It/sec 0.051, Tokens/sec 16.150, Trained Tokens 3137, Peak mem 8.008 GB
Iter 20: Train loss 2.010, Learning Rate 1.000e-05, It/sec 7.467, Tokens/sec 2425.386, Trained Tokens 6385, Peak mem 8.034 GB
Iter 30: Train loss 1.746, Learning Rate 1.000e-05, It/sec 7.789, Tokens/sec 2425.490, Trained Tokens 9499, Peak mem 8.469 GB
Iter 40: Train loss 1.737, Learning Rate 1.000e-05, It/sec 7.643, Tokens/sec 2533.776, Trained Tokens 12814, Peak mem 8.469 GB
...

SQL (Structured Query Language) is a programming language designed for managing and manipulating data stored in relational database management systems (RDBMS).
```

## Gradient Averaging Patterns

MLX provides efficient gradient averaging utilities:

```py
# Method 1: Using mx.distributed.all_sum directly.
averaged_grad = mx_dist.all_sum(gradient) / mx_dist.size()

# Method 2: Using mlx.nn.average_gradients (recommended).
import mlx.nn as nn
gradients = {"layer1": grad1, "layer2": grad2}
averaged_gradients = nn.average_gradients(gradients)
```

For more information, check the [official MLX guides](https://ml-explore.github.io/mlx/build/html/usage/distributed.html#utilizing-nn-average-gradients).

## Next Steps

- Check out [the distributed MNIST example with MLX](https://github.com/kubeflow/trainer/tree/master/examples/mlx/image-classification/mnist.ipynb).
- Follow [the complete example](https://github.com/kubeflow/trainer/tree/master/examples/mlx/language-modeling/fine-tune-llama.ipynb)
 to fine-tune Llama3.2 with MLX and Kubeflow Trainer.
- Explore [the official MLX documentation](https://ml-explore.github.io/mlx/build/html/index.html).
- Learn more about `TrainerClient()` APIs [in the Kubeflow SDK](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py).
