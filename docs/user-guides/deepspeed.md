# DeepSpeed Guide

This guide describes how to use TrainJob to train or fine-tune AI models with [DeepSpeed](https://www.deepspeed.ai/).

## Prerequisites

Before exploring this guide, make sure to follow [the Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.

## DeepSpeed Overview

DeepSpeed is a deep learning optimization library that makes distributed training and inference easy,
efficient, and effective. DeepSpeed includes features such as:

- **ZeRO (Zero Redundancy Optimizer)**: Reduces memory consumption by partitioning optimizer states,
 gradients, and parameters across data-parallel processes.
- **3D Parallelism**: Combines data parallelism, model parallelism, and pipeline parallelism for
 training extremely large models.
- **Mixed Precision Training**: Supports FP16 and BF16 training to accelerate training and reduce
 memory usage.
- **Gradient Compression**: Reduces communication overhead through gradient compression techniques.

ZeRO has three stages:

- **ZeRO Stage 0**: Disabled (equivalent to standard data parallel training).
- **ZeRO Stage 1**: Partitions optimizer states across processes.
- **ZeRO Stage 2**: Partitions optimizer states and gradients across processes.
- **ZeRO Stage 3**: Partitions optimizer states, gradients, and model parameters across processes.

## Get DeepSpeed Runtime Packages

Kubeflow Trainer includes a DeepSpeed runtime called [`deepspeed-distributed`](https://github.com/kubeflow/trainer/blob/master/manifests/base/runtimes/deepspeed_distributed.yaml),
which comes with the several pre-installed Python packages.

Run the following command to get a list of the available packages:

```py
from kubeflow.trainer import TrainerClient

TrainerClient().get_runtime_packages(
    runtime=TrainerClient().get_runtime("deepspeed-distributed")
)
```

You should see the installed packages, for example:

```sh
Python: 3.10.12 (main, May 27 2025, 17:12:29) [GCC 11.4.0]

Package            Version
------------------ -----------
...
datasets           4.0.0
deepspeed          0.17.4
dill               0.3.8
...
```

## DeepSpeed Distributed Environment

Kubeflow Trainer uses the MPI-based runtime and [`mpirun` launcher](https://www.deepspeed.ai/getting-started/#mpi-and-azureml-compatibility)
to run DeepSpeed scripts on every training node. It automatically creates the OpenMPI hostfile
to ensure DeepSpeed can discover all MPI nodes, starts the OpenSSH server on the worker nodes,
and configures the distributed DeepSpeed environment:

- `dist.get_world_size()` - Total number of processes (e.g., GPUs) across all DeepSpeed nodes.
- `dist.get_rank()` - Rank of the current process across all DeepSpeed nodes.
- `os.environ["LOCAL_RANK"]` - Rank of the current process within a single DeepSpeed training node.

You can use these values to, for example, download the dataset only on the node with `local_rank=0`,
or export your fine-tuned LLM only on the node with `rank=0` (e.g., the master node).

You can access the distributed environment as follows:

```py
from kubeflow.trainer import TrainerClient, CustomTrainer

def get_deepspeed_dist():
    import os
    import torch.distributed as dist
    import deepspeed

    device = "cuda"
    dist_backend = "nccl"
    deepspeed.init_distributed(dist_backend=dist_backend)

    print("DeepSpeed Distributed Environment")
    print(f"Using device: {device}")
    print(f"WORLD_SIZE: {dist.get_world_size()}")
    print(f"RANK: {dist.get_rank()}")
    print(f"LOCAL_RANK: {os.environ['LOCAL_RANK']}")


# Create the TrainJob.
job_id = TrainerClient().train(
    runtime="deepspeed-distributed",
    trainer=CustomTrainer(func=get_deepspeed_dist),
)

# Wait for TrainJob to complete.
TrainerClient().wait_for_job_status(job_id)

# Since we launch DeepSpeed with `mpirun`, all logs should be consumed from the node-0.
print("\n".join(TrainerClient().get_job_logs(name=job_id)))
```

You should see the distributed environment across the two training nodes as follows:

```shell
[2025-08-15 17:46:46,463] [INFO] [comm.py:891:mpi_discovery] Discovered MPI settings of world_rank=0, local_rank=0, world_size=4, master_addr=..., master_port=...
[2025-08-15 17:46:46,463] [INFO] [comm.py:891:mpi_discovery] Discovered MPI settings of world_rank=1, local_rank=1, world_size=4, master_addr=..., master_port=...
[2025-08-15 17:46:46,463] [INFO] [comm.py:852:init_distributed] Initializing TorchBackend in DeepSpeed with backend nccl
[2025-08-15 17:46:46,463] [INFO] [comm.py:891:mpi_discovery] Discovered MPI settings of world_rank=2, local_rank=0, world_size=4, master_addr=..., master_port=...
[2025-08-15 17:46:46,463] [INFO] [comm.py:891:mpi_discovery] Discovered MPI settings of world_rank=3, local_rank=1, world_size=4, master_addr=..., master_port=...
DeepSpeed Distributed Environment
Using device: cuda
WORLD_SIZE: 4
RANK: 1
LOCAL_RANK: 1
DeepSpeed Distributed Environment
Using device: cuda
WORLD_SIZE: 4
RANK: 3
LOCAL_RANK: 1
DeepSpeed Distributed Environment
Using device: cuda
WORLD_SIZE: 4
RANK: 0
LOCAL_RANK: 0
DeepSpeed Distributed Environment
Using device: cuda
WORLD_SIZE: 4
RANK: 2
LOCAL_RANK: 0
...
```

## Create TrainJob with DeepSpeed Training

### Configure DeepSpeed Training Function

You can leverage the `CustomTrainer()` to wrap your DeepSpeed code inside a function and create a
TrainJob. This function should handle the end-to-end model training or fine-tuning of a
pre-trained model with DeepSpeed optimization.

:::{note}
All necessary imports must be included inside the function body so that the TrainJob can recognize
them on every training node.
:::

Your training function might look like this:

```py
def fine_tune_t5_deepspeed():
    import os
    import torch.distributed as dist
    from torch.utils.data import DataLoader
    from torch.utils.data.distributed import DistributedSampler
    from transformers import T5Tokenizer, T5ForConditionalGeneration
    import deepspeed
    import boto3

    # Initialize DeepSpeed distributed training
    deepspeed.init_distributed(dist_backend="nccl")

    # DeepSpeed Configuration.
    ds_config = {
        # Train batch size = micro batch size * gradient steps * GPUs (e.g. 2 x 1 x 4 = 8).
        "train_micro_batch_size_per_gpu": 2,
        "gradient_accumulation_steps": 1,
        "optimizer": {
            "type": "AdamW",
            "params": {
                "lr": 3e-4,
                "betas": [0.9, 0.95],
                "eps": 1e-8,
                "weight_decay": 0.1,
            },
        },
        # "fp16": {"enabled": True}, # If your GPU (e.g. V100) doesn't support bf16, use fp16.
        "bf16": {"enabled": True},  # Enable mixed precision.
        "zero_optimization": {
            "stage": 2,
            "allgather_partitions": True,
            "allgather_bucket_size": 5e8,
            "overlap_comm": True,
            "reduce_scatter": True,
            "reduce_bucket_size": 5e8,
            "contiguous_gradients": True,
        },
    }

    # Configure the pre-trained model and tokenizer.
    model = T5ForConditionalGeneration.from_pretrained("t5-base")
    tokenizer = T5Tokenizer.from_pretrained("t5-base")

    # Configure the dataset and dataloader.
    dataset = wikihow(tokenizer)
    train_loader = DataLoader(
        dataset, batch_size=16, sampler=DistributedSampler(dataset)
    )

    # Initialize DeepSpeed engine.
    model, _, _, _ = deepspeed.initialize(
        model=model,
        config=ds_config,
        model_parameters=model.parameters(),
    )

    # Start DeepSpeed training loop.
    for epoch in range(10):
        for batch_idx, batch in enumerate(train_loader):
            for key in batch.keys():
                batch[key] = batch[key].to(os.environ["LOCAL_RANK"])
            outputs = model(batch)
            loss = outputs.loss

            model.backward(loss)
            model.step()

            if batch_idx % 100 == 0:
                print(f"Epoch: {epoch}, Batch: {batch_idx}, Loss: {loss.item()}")

    if dist.get_rank() == 0:
        # Export your model to the object storage (e.g. S3)
        boto3.upload_file()
```

### Create a TrainJob

After configuring the DeepSpeed training function, use the `train()` API to create TrainJob:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

job_id = TrainerClient().train(
    runtime="deepspeed-distributed",
    trainer=CustomTrainer(
        func=fine_tune_t5_deepspeed,
        # These packages will be installed on every training node.
        packages_to_install=["boto3"],
        num_nodes=2,
        resources_per_node={
            "gpu": 2,
        },
    )
)
```

### Get the TrainJob Results

You can use the `get_job_logs()` API to see your TrainJob logs:

```py
print("\n".join(TrainerClient().get_job_logs(name=job_id)))
```

:::{note}
Since DeepSpeed training is launched via the mpirun command, all logs can be collected from
node-0, which acts as the OpenMPI launcher.
:::

## DeepSpeed Configuration

DeepSpeed uses a JSON configuration file to specify training parameters, optimization settings,
and memory management options. Learn more about it in
[the DeepSpeed documentation](https://www.deepspeed.ai/docs/config-json/).

Key configuration sections include:

### Basic Training Configuration

```json
{
  "train_batch_size": 128,
  "train_micro_batch_size_per_gpu": 16,
  "gradient_accumulation_steps": 8,
  "steps_per_print": 100
}
```

### ZeRO Configuration

```json
{
  "zero_optimization": {
    "stage": 2,
    "allgather_partitions": true,
    "allgather_bucket_size": 5e8,
    "overlap_comm": true,
    "reduce_scatter": true,
    "reduce_bucket_size": 5e8,
    "contiguous_gradients": true,
    "cpu_offload": false
  }
}
```

### Mixed Precision Training

```json
{
  "fp16": {
    "enabled": true,
    "loss_scale": 0,
    "loss_scale_window": 1000,
    "hysteresis": 2,
    "min_loss_scale": 1
  }
}
```

## Next Steps

- Check out [the DeepSpeed T5 example](https://github.com/kubeflow/trainer/blob/master/examples/deepspeed/text-summarization/T5-Fine-Tuning.ipynb).
- Learn more about `TrainerClient()` APIs [in the Kubeflow SDK](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py).
- Explore [DeepSpeed documentation](https://www.deepspeed.ai/) for advanced configuration options.
