# PyTorch Guide

This guide describes how to run [PyTorch](https://pytorch.org/) on Kubernetes with TrainJob to
train or fine-tune AI models.

## Prerequisites

Before exploring this guide, make sure to follow [the Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.

## PyTorch Distributed Overview

PyTorch has the builtin [`torch.distributed` package](https://docs.pytorch.org/docs/stable/distributed.html)
to perform distributed training, including both data and model parallelism. You can use the Kubeflow
Python SDK to create your TrainJobs with
[PyTorch Distributed Data Parallel (DDP)](https://docs.pytorch.org/tutorials/intermediate/ddp_tutorial.html),
[Fully Sharded Data Parallel (FSDP)](https://docs.pytorch.org/docs/stable/fsdp.html),
[FSDP2](https://docs.pytorch.org/tutorials/intermediate/FSDP_tutorial.html),
or any other parallelism algorithm supported by PyTorch.

In DDP training, the dataset is sharded across multiple GPUs, with each GPU holding one partition
of the dataset and a full copy of the model. The gradients are calculated locally on each GPU and
then synchronized globally to update the model parameters.

In FSDP training, in addition to DDP, the model gets chopped into slices and assigned to the
different GPUs. The model is split into shards, each hosted on a different GPU. Gradients and
parameter updates are computed locally and synchronized globally. FSDP is particularly useful for
training very large models that cannot fit into the memory of a single GPU.

## Get PyTorch Runtime Packages

Kubeflow Trainer includes a PyTorch runtime called [`torch-distributed`](https://github.com/kubeflow/trainer/blob/master/manifests/base/runtimes/torch_distributed.yaml),
which comes with several pre-installed Python packages.

Run the following command to get a list of the available packages:

```py
from kubeflow.trainer import TrainerClient

TrainerClient().get_runtime_packages(
    runtime=TrainerClient().get_runtime("torch-distributed")
)
```

You should see the installed packages, for example:

```sh
Python: 3.11.13 | packaged by conda-forge | (main, Jun  4 2025, 14:48:23) [GCC 13.3.0]
Package                   Version
------------------------- ------------
torch                     2.7.1+cu128
torchaudio                2.7.1+cu128
torchelastic              0.2.2
torchvision               0.22.1+cu128
...
```

## PyTorch Distributed Environment

Kubeflow Trainer uses the [`torchrun` utility](https://docs.pytorch.org/docs/stable/elastic/run.html)
to run PyTorch script on every training node. It automatically configures the appropriate distributed
environment for PyTorch nodes:

- `dist.get_world_size()` - Total number of processes (e.g., GPUs) across all PyTorch nodes.
- `dist.get_rank()` - Rank of the current process across all PyTorch node.
- `os.environ["LOCAL_RANK"]` - Rank of the current process within a single PyTorch training node.

You can use these values to, for example, download the dataset only on the node with `local_rank=0`,
or export your fine-tuned LLM only on the node with `rank=0` (e.g., the master node).

You can access the distributed environment as follows:

```py
from kubeflow.trainer import TrainerClient, CustomTrainer

def get_torch_dist():
    import os
    import torch
    import torch.distributed as dist

    device, backend = ("cuda", "nccl") if torch.cuda.is_available() else ("cpu", "gloo")
    dist.init_process_group(backend=backend)

    print("PyTorch Distributed Environment")
    print(f"Using device: {device}")
    print(f"WORLD_SIZE: {dist.get_world_size()}")
    print(f"RANK: {dist.get_rank()}")
    print(f"LOCAL_RANK: {os.environ['LOCAL_RANK']}")

# Create the TrainJob.
job_id = TrainerClient().train(
    runtime="torch-distributed",
    trainer=CustomTrainer(
        func=get_torch_dist,
        num_nodes=3,
        resources_per_node={
            "cpu": 2,
        },
    ),
)

# Wait for TrainJob to complete.
TrainerClient().wait_for_job_status(job_id)

print("Distributed PyTorch env on node-0")
print("\n".join(TrainerClient().get_job_logs(name=job_id, step="node-0")))

print("Distributed PyTorch env on node-1")
print("\n".join(TrainerClient().get_job_logs(name=job_id, step="node-1")))
```

You should see the distributed environment across the two training nodes as follows:

```shell
Distributed PyTorch env on node-0
PyTorch Distributed Environment
Using device: cpu
WORLD_SIZE: 6
RANK: 0
LOCAL_RANK: 0
PyTorch Distributed Environment
Using device: cpu
WORLD_SIZE: 6
RANK: 1
LOCAL_RANK: 1

Distributed PyTorch env on Node-1
PyTorch Distributed Environment
Using device: cpu
WORLD_SIZE: 6
RANK: 2
LOCAL_RANK: 0
PyTorch Distributed Environment
Using device: cpu
WORLD_SIZE: 6
RANK: 3
LOCAL_RANK: 1
```

## Create TrainJob with PyTorch Training

### Configure PyTorch Training Function

You can leverage the `CustomTrainer()` to wrap your PyTorch code inside a function and create a
TrainJob. This function should handle the end-to-end model training or fine-tuning of a
pre-trained model.

:::{note}
All necessary imports must be included inside the function body so that the TrainJob can recognize
them on every training node.
:::

Your training function might look like this:

```py
def fine_tune_qwen():
    import torch
    import torch.distributed as dist
    from torch.utils.data import DataLoader, DistributedSampler
    from transformers import AutoTokenizer, AutoModelForCausalLM
    import boto3

    # Setup distributed Torch.
    device, backend = ("cuda", "nccl") if torch.cuda.is_available() else ("cpu", "gloo")
    dist.init_process_group(backend=backend)

    # Configure the dataset and dataloader.
    dataset = ...
    train_loader = DataLoader(
        dataset, batch_size=128, sampler=DistributedSampler(dataset)
    )
    # Configure the pre-trained model and tokenizer.
    tokenizer = AutoTokenizer.from_pretrained("Qwen/Qwen3-32B")
    model = AutoModelForCausalLM.from_pretrained("Qwen/Qwen3-32B")

    # Configure the PyTorch training loop.
    for epoch in range(10):
        for batch_idx, batch in enumerate(train_loader):
            output = model(...)
            model.backward(output.loss)
            model.step()
            ...
    if dist.get_rank() == 0:
        # Export your model to the object storage (e.g. S3)
        boto3.upload_file()
```

### Create a TrainJob

After configuring the PyTorch training function, use the `train()` API to create TrainJob:

```python
job_id = TrainerClient().train(
    runtime=TrainerClient().get_runtime("torch-distributed"),
    trainer=CustomTrainer(
        func=fine_tune_qwen,
        num_nodes=2,
        resources_per_node={
            "gpu": 4
        },
        # These packages will be installed on every training node.
        packages_to_install=["transformers>=4.53.0", "boto3"],
    )
)
```

### Get the TrainJob Results

You can use the `get_job_logs()` API to see your TrainJob logs:

```py
print("\n".join(TrainerClient().get_job_logs(name=job_id)))
```

## FSDP Configuration

Kubeflow Trainer supports [Fully Sharded Data Parallel (FSDP)](https://docs.pytorch.org/docs/fsdp.html)
through the Torch plugin, which uses the [`torchrun` utility](https://docs.pytorch.org/docs/elastic/run.html)
to launch distributed PyTorch processes. FSDP splits the model across GPUs, enabling training
of models that do not fit on a single GPU.

### TrainingRuntime Configuration

The following example shows a `TrainingRuntime` with the Torch policy configured to inject
the `PET_*` environment variables required by `torchrun`. The actual FSDP configuration is
provided through the `TrainJob`'s `spec.trainer` fields.

```yaml
# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Example: FSDP configuration through the Torch plugin.
#
# This TrainingRuntime configures the Torch plugin to inject PET_* environment variables.
# FSDP settings are supplied via the TrainJob's spec.trainer.command and spec.trainer.args.
#
# Apply: kubectl apply -f fsdp-trainingruntime.yaml
# Check: kubectl get trainingruntime fsdp-runtime
# Logs:  kubectl logs -l jobset.sigs.k8s.io/jobset-name=fsdp-example
# Clean: kubectl delete trainingruntime fsdp-runtime
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: fsdp-runtime
spec:
  mlPolicy:
    torch:
      envInjection:
        targets:
          - jobName: node
            containerNames:
              - node
```

### TrainJob with FSDP

The following `TrainJob` demonstrates how to configure FSDP through the Torch plugin.
The `torchrun` launcher is used, and FSDP-related arguments are passed through `spec.trainer.args`.

```yaml
# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Example: FSDP training with the Torch plugin.
#
# This TrainJob uses the `torchdistributed` runtime and configures FSDP via the
# `torchrun` command arguments. The `PET_*` environment variables (e.g.
# `PET_NNODES`, `PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`,
# `PET_MASTER_PORT`) are automatically injected by the Torch plugin into the
# trainer container.
#
# Apply: kubectl apply -f fsdp-trainjob.yaml
# Check: kubectl get trainjob fsdp-example
# Logs:  kubectl logs -l jobset.sigs.k8s.io/jobset-name=fsdp-example
# Clean: kubectl delete trainjob fsdp-example
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: fsdp-example
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    kind: TrainingRuntime
    name: fsdp-runtime
  trainer:
    numNodes: 1
    numProcPerNode: 1
    resourcesPerNode:
      gpu: 4
    command:
      - torchrun
    args:
      - --fsdp
      - --fsdp-sharding=auto
      - --fsdp-backward-prefetch=allow
      - --fsdp-limit-all-gathers=""
    env:
      # The Torch plugin automatically injects PET_* environment variables.
      # No manual configuration is required for basic FSDP operation.
    image: docker.io/pytorch/pytorch:2.3.0-cuda12.1-runtime
    commandOverwrite: false
```

#### How FSDP configuration is supplied

- **Launcher**: The `torchrun` utility is used as the entrypoint (configured via `spec.trainer.command`).
- **FSDP strategy**: Set via `--fsdp-sharding=<strategy>` (e.g., `auto`, `FULL_SHARD`, `SHARD_WIDTH`).
- **Backward prefetch**: Set via `--fsdp-backward-prefetch=<policy>` (e.g., `allow`, `require`).
- **Limit all-gathers**: Set via `--fsdp-limit-all-gathers=<policy>` (e.g., `", "`).

These arguments are passed directly to the `torchrun` process. The `PET_NNODES`,
`PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`, and `PET_MASTER_PORT`
environment variables are automatically injected by the Torch plugin's `EnforceMLPolicy`
function, enabling `torchrun` to discover the distributed environment.

#### Training script interface

The user's training script receives the FSDP configuration through the `torchrun`
arguments. The script should:

1. Import `torch.distributed` and initialize the process group:
   ```python
   import torch
   import torch.distributed as dist
   from torch.nn import functional as F

   dist.init_process_group(backend="nccl")
   ```

2. Use `torch.nn.FullyShardedDataParallel` (or `torch.nn.parallel.DistributedDataParallel`)
   for the model:
   ```python
   model = torch.nn.Linear(10, 10).to("cuda")
   model = torch.nn.parallel.FullyShardedDataParallel(model)
   ```

3. The `--fsdp` flag enables FSDP mode. The other `--fsdp-*` arguments control
   the sharding strategy, backward prefetch behavior, and all-gather limits.
   These are PyTorch FSDP-specific flags; the training script does not need to
   explicitly pass `--fsdp` or `SHARDED_DATA_PARALLEL` beyond what `torchrun`
   provides.

**Important**: The FSDP configuration (`--fsdp`, `--fsdp-sharding`, etc.) is handled
by the `torchrun` launcher and the PyTorch runtime. The user's training script must
be written to use FSDP (e.g., `FullyShardedDataParallel`), but does not need to
parse these arguments directly.

## FSDP vs. DeepSpeed Comparison

| Aspect | FSDP (via Torch Plugin) | DeepSpeed (via Torch Plugin) |
|---|---|---|
| **Launcher** | `torchrun` utility | `torchrun` with `--deepspeed` arg |
| **Configuration mechanism** | FSDP flags passed as `torchrun` args: `--fsdp`, `--fsdp-sharding`, `--fsdp-backward-prefetch`, `--fsdp-limit-all-gathers` | DeepSpeed JSON config file mounted as ConfigMap/volume; passed via `--deepspeed <path>` arg |
| **Distributed settings supply** | PET_* env vars automatically injected by Torch plugin (`PET_NNODES`, `PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`, `PET_MASTER_PORT`) | Same PET_* env vars automatically injected; DeepSpeed config supplied separately |
| **Hardware/topology considerations** | Designed for GPU clusters; sharding strategy depends on GPU count and model size; works with standard Kubernetes GPU nodes | Designed for GPU clusters; ZeRO stages optimize memory across nodes; requires NCCL/Gloo backend |
| **Prerequisites** | PyTorch runtime (`torch-distributed`), `torchrun` available, GPU nodes with appropriate drivers | DeepSpeed runtime or ConfigMap with `deepspeed` package, `torchrun` available, GPU nodes with appropriate drivers |
| **Limitations** | FSDP state dict format differs from DDP; model wrapping requires `FullyShardedDataParallel`; backward prefetch and limit-all-gathers have tradeoffs | DeepSpeed config JSON must be valid; `--deepspeed` arg requires mounted config file; some advanced ZeRO features may require specific runtime setup |

**Important**: The FSDP configuration (`--fsdp`, `--fsdp-sharding`, etc.) is handled
by the `torchrun` launcher and the PyTorch runtime. The user's training script must
be written to use FSDP (e.g., `FullyShardedDataParallel`), but does not need to
parse these arguments directly.

## Next Steps

- Check out [the PyTorch MNIST example](https://github.com/kubeflow/trainer/blob/master/examples/pytorch/image-classification/mnist.ipynb).
- Follow [the PyTorch fine-tuning example](https://github.com/kubeflow/trainer/blob/master/examples/pytorch/question-answering/fine-tune-distilbert.ipynb)
 using the pre-trained DistilBERT model.
- Learn more about `TrainerClient()` APIs [in the Kubeflow SDK](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py).
- Learn more about [FSDP documentation](https://docs.pytorch.org/docs/fsdp.html).
