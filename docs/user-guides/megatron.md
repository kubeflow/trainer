# Megatron Guide

This guide describes how to use TrainJob to train AI models with
[Megatron-Core](https://github.com/NVIDIA/Megatron-LM) and Tensor Parallelism.

## Prerequisites

Before exploring this guide, make sure to follow [the Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.

Your cluster needs at least **2 NVIDIA GPUs** (across one or more nodes). Megatron-Core requires
CUDA and uses the NCCL backend for distributed communication.

## Megatron-Core Overview

[Megatron-Core](https://docs.nvidia.com/megatron-core/developer-guide/latest/user-guide/index.html)
is NVIDIA's library for training large transformer models efficiently across multiple GPUs.
It provides production-grade implementations of parallelism strategies:

- **Tensor Parallelism (TP)**: Splits individual layer weight matrices across GPUs. Each GPU holds
 a slice of every layer, so you can train models whose layers are too large for a single GPU's
 memory. This is the parallelism strategy covered in this guide.
- **Pipeline Parallelism (PP)**: Assigns different layers to different GPUs and overlaps computation
 with micro-batch pipelining.
- **Data Parallelism (DP)**: Replicates the full model on each GPU and splits the data across them.
 Megatron-Core includes `DistributedDataParallel` for gradient synchronization.

Since Megatron-Core uses `torchrun` as its distributed launcher, it works natively with the
existing `torch-distributed` ClusterTrainingRuntime. No dedicated Megatron runtime is needed.

If you want to reuse an existing Hugging Face transformer,
[Megatron-Bridge](https://github.com/NVIDIA-NeMo/Megatron-Bridge) is a PyTorch-native training
library that provides bidirectional conversion between Hugging Face and Megatron-Core formats
with parallelism-aware checkpoints (TP, PP, and more). This guide uses Megatron-Core directly
with a small GPT model built from scratch.

## Megatron Distributed Environment

Kubeflow Trainer uses the `torch-distributed` runtime to launch Megatron-Core training with
`torchrun` on every node. The standard PyTorch distributed environment variables are available
in your training function:

- `os.environ["WORLD_SIZE"]` - Total number of processes across all nodes.
- `os.environ["RANK"]` - Global rank of the current process.
- `os.environ["LOCAL_RANK"]` - Rank of the current process within its node.

Megatron-Core uses these to initialize its own parallel groups. Tensor Parallelism degree is
controlled by the `TP_SIZE` environment variable that you pass via `CustomTrainer`:

```py
parallel_state.initialize_model_parallel(
    tensor_model_parallel_size=int(os.environ["TP_SIZE"]),
    pipeline_model_parallel_size=1,
)
```

For a `TP_SIZE` of 2, Megatron-Core creates a tensor-parallel group spanning 2 processes,
and each layer's weight matrices are split across those 2 GPUs.

Megatron-Core compiles a small C++ dataset helper the first time it is used, so the training
container needs a C/C++ toolchain (`make` and `g++`). The default `pytorch/pytorch:*-runtime`
image does not ship these tools. The example below installs them from inside the training
function with `apt-get`; for production you can bake `make` and `g++` into a custom training
runtime image to skip the install on every run.

Megatron-Core also initializes several NCCL communicators (one per parallel group: TP, DP,
and helper groups) during training. Each communicator reserves shared memory in `/dev/shm`,
which is capped at 64 MB by default in Kubernetes Pods, and that is not enough for multi-group
workloads. See [Increasing /dev/shm for NCCL](#increasing-devshm-for-nccl) below for the fix.

## Create TrainJob with Megatron-Core Training

### Configure the Training Function

You can use the `CustomTrainer()` to wrap your Megatron-Core training code inside a function
and create a TrainJob.

:::{note}
All necessary imports must be included inside the function body so that the TrainJob can recognize
them on every training node.
:::

The following example trains a small GPT model with Tensor Parallelism, based on the official
[run_simple_mcore_train_loop.py](https://github.com/NVIDIA/Megatron-LM/blob/main/examples/run_simple_mcore_train_loop.py):

```py
def train_megatron_gpt_tp():
    import os
    import torch
    from torch.optim import Adam
    from torch.utils.data import DataLoader
    from functools import partial

    from megatron.core import parallel_state
    from megatron.core.pipeline_parallel.schedules import get_forward_backward_func
    from megatron.core.tensor_parallel.random import model_parallel_cuda_manual_seed
    from megatron.core.transformer.transformer_config import TransformerConfig
    from megatron.core.models.gpt.gpt_model import GPTModel
    from megatron.core.models.gpt.gpt_layer_specs import get_gpt_layer_local_spec
    from megatron.core.datasets.utils import compile_helpers
    from megatron.core.datasets.blended_megatron_dataset_builder import (
        BlendedMegatronDatasetBuilder,
    )
    from megatron.core.datasets.gpt_dataset import GPTDatasetConfig, MockGPTDataset
    from megatron.core.distributed import DistributedDataParallel
    from megatron.core.distributed import DistributedDataParallelConfig
    from megatron.core.distributed.finalize_model_grads import finalize_model_grads
    from megatron.core.tokenizers import MegatronTokenizer

    _SEQUENCE_LENGTH = 64

    # Step 1: Initialize torch.distributed and Megatron parallel groups.
    # tensor_model_parallel_size=2 splits each layer's weight matrices across 2 GPUs.
    parallel_state.destroy_model_parallel()

    rank = int(os.environ["RANK"])
    world_size = int(os.environ["WORLD_SIZE"])
    local_rank = int(os.environ["LOCAL_RANK"])

    torch.cuda.set_device(local_rank)
    torch.distributed.init_process_group(
        backend="nccl", rank=rank, world_size=world_size
    )

    tensor_model_parallel_size = int(os.environ["TP_SIZE"])
    parallel_state.initialize_model_parallel(
        tensor_model_parallel_size, pipeline_model_parallel_size=1
    )
    model_parallel_cuda_manual_seed(123)

    # Step 2: Build a small GPT model.
    transformer_config = TransformerConfig(
        num_layers=2,
        hidden_size=12,
        num_attention_heads=4,
        use_cpu_initialization=True,
        pipeline_dtype=torch.float32,
    )

    gpt_model = GPTModel(
        config=transformer_config,
        transformer_layer_spec=get_gpt_layer_local_spec(),
        vocab_size=100,
        max_sequence_length=_SEQUENCE_LENGTH,
    )

    # Move model from CPU to GPU (use_cpu_initialization=True builds weights on CPU).
    gpt_model.to("cuda")

    # Wrap with DistributedDataParallel for gradient synchronization.
    ddp_config = DistributedDataParallelConfig(
        grad_reduce_in_fp32=False,
        overlap_grad_reduce=False,
        use_distributed_optimizer=False,
    )
    gpt_model = DistributedDataParallel(
        config=transformer_config,
        ddp_config=ddp_config,
        module=gpt_model,
    )

    optim = Adam(gpt_model.parameters())

    # Step 3: Prepare a mock dataset (no real data download needed).
    # Install build tools for Megatron's C++ dataset helpers. The default
    # pytorch/pytorch:*-runtime image does not include them; bake them into
    # a custom runtime image if you want to skip this on every run.
    import subprocess as _sp
    _sp.run(["apt-get", "update", "-qq"], capture_output=True)
    _sp.run(["apt-get", "install", "-y", "-qq", "make", "g++"], capture_output=True)

    import urllib.request as _urlreq
    _datasets_dir = os.path.dirname(compile_helpers.__code__.co_filename)
    _base_url = (
        "https://raw.githubusercontent.com/NVIDIA/Megatron-LM/main/megatron/core/datasets/"
    )
    for _f in ["Makefile", "helpers.cpp"]:
        _urlreq.urlretrieve(_base_url + _f, os.path.join(_datasets_dir, _f))

    if torch.distributed.get_rank() == 0:
        compile_helpers()
    torch.distributed.barrier()

    config = GPTDatasetConfig(
        random_seed=0,
        sequence_length=_SEQUENCE_LENGTH,
        reset_position_ids=False,
        reset_attention_mask=False,
        eod_mask_loss=False,
        tokenizer=MegatronTokenizer.from_pretrained(
            metadata_path={"library": "null-text"},
            vocab_size=_SEQUENCE_LENGTH,
        ),
        mid_level_dataset_surplus=0.005,
    )

    datasets = BlendedMegatronDatasetBuilder(
        MockGPTDataset, [1000, None, None], lambda: True, config
    ).build()

    train_dataloader = DataLoader(datasets[0], batch_size=8, shuffle=True)
    train_iterator = iter(train_dataloader)

    # Step 4: Define forward step function.
    def forward_step_func(data_iterator, model):
        def loss_func(loss_mask, output_tensor):
            losses = output_tensor.float()
            loss_mask = loss_mask.view(-1).float()
            loss = torch.sum(losses.view(-1) * loss_mask) / loss_mask.sum()
            return loss, {"lm loss": loss}

        data = next(data_iterator)
        tokens = data["tokens"].to("cuda")
        attention_mask = data["attention_mask"].to("cuda")
        position_ids = data["position_ids"].to("cuda")
        labels = data["labels"].to("cuda")
        loss_mask = data["loss_mask"].to("cuda")

        output_tensor = model(tokens, position_ids, attention_mask, labels=labels)
        return output_tensor, partial(loss_func, loss_mask)

    # Step 5: Training loop.
    forward_backward_func = get_forward_backward_func()

    for iteration in range(5):
        optim.zero_grad()

        losses_reduced = forward_backward_func(
            forward_step_func=forward_step_func,
            data_iterator=train_iterator,
            model=gpt_model,
            num_microbatches=1,
            seq_length=_SEQUENCE_LENGTH,
            micro_batch_size=8,
            decoder_seq_length=_SEQUENCE_LENGTH,
            forward_only=False,
        )

        finalize_model_grads([gpt_model])
        optim.step()

        print(f"Iteration {iteration}: Losses reduced: {losses_reduced}")

    torch.distributed.destroy_process_group()
```

:::{note}
Installing `make` and `g++` from inside the training function is a workaround because the
default `pytorch/pytorch:*-runtime` image used by `torch-distributed` does not ship a C/C++
toolchain. A cluster administrator can move this into a custom training runtime image so
that every job starts faster and does not depend on the Debian package mirrors.
:::

### Create a TrainJob

After configuring the training function, use the `train()` API to create a TrainJob.
This runs Megatron-Core TP across 2 nodes, each with 1 GPU:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

tensor_model_parallel_size = 2

job_name = TrainerClient().train(
    runtime="torch-distributed",
    trainer=CustomTrainer(
        func=train_megatron_gpt_tp,
        num_nodes=2,
        resources_per_node={
            "memory": "16Gi",
            "gpu": 1,
        },
        packages_to_install=["megatron-core", "pybind11"],
        env={"TP_SIZE": str(tensor_model_parallel_size)},
    ),
)
```

:::{note}
The TP_SIZE environment variable must match the total number of GPUs across all nodes
(`num_nodes * gpu`). If you set `num_nodes=2` and `gpu=1`, then `TP_SIZE` should be 2.
:::

### Get the TrainJob Results

You can use the `get_job_logs()` API to get your TrainJob logs:

```py
print("\n".join(TrainerClient().get_job_logs(name=job_name)))
```

## Increasing `/dev/shm` for NCCL

NCCL allocates shared memory in `/dev/shm` for its proxy service (roughly 33 MB per communicator).
Kubernetes pods get 64 MB of `/dev/shm` by default, which is not enough for Megatron-Core because
it creates multiple NCCL communicators (one per parallel group: TP, DP, etc.). When `/dev/shm`
fills up, NCCL operations fail silently or crash.

The fix is to mount `/dev/shm` as a memory-backed `emptyDir` volume. You have two ways to
apply it: a cluster administrator can patch the `torch-distributed` ClusterTrainingRuntime so
every job picks it up, or an individual user can attach the same mount to a single TrainJob
via [RuntimePatches](../../operator-guides/runtime-patches).

**Option 1: Cluster administrator — patch the runtime once for every job.**

```yaml
apiVersion: trainer.kubeflow.org/v2alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed
spec:
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  volumes:
                    - name: dshm
                      emptyDir:
                        medium: Memory
                  containers:
                    - name: node
                      volumeMounts:
                        - name: dshm
                          mountPath: /dev/shm
```

**Option 2: TrainJob author — attach the mount to a single job with `runtimePatches`.**

```yaml
apiVersion: trainer.kubeflow.org/v2alpha1
kind: TrainJob
metadata:
  name: megatron-tp
spec:
  runtimeRef:
    name: torch-distributed
  runtimePatches:
    - manager: trainer.kubeflow.org/kubeflow-sdk
      trainingRuntimeSpec:
        template:
          spec:
            replicatedJobs:
              - name: node
                template:
                  spec:
                    template:
                      spec:
                        volumes:
                          - name: dshm
                            emptyDir:
                              medium: Memory
                        containers:
                          - name: node
                            volumeMounts:
                              - name: dshm
                                mountPath: /dev/shm
```

This is a [well-known NCCL + Kubernetes issue](https://github.com/NVIDIA/nccl/issues/525).
Without the mount, the default 64 MB limit causes failures during multi-node or multi-GPU
training with frameworks that create several NCCL communicators.

## Multi-Node vs Multi-GPU Configuration

When configuring your TrainJob, the relationship between `num_nodes`, `gpu` per node, and
`TP_SIZE` matters:

| Configuration | `num_nodes` | `gpu` | `TP_SIZE` | Pods | How it works |
|---|---|---|---|---|---|
| Multi-GPU single node | 1 | 2 | 2 | 1 | 2 workers in 1 pod, each gets a CUDA device |
| Multi-node | 2 | 1 | 2 | 2 | 1 worker per pod, TP across nodes via NCCL |

Both configurations give you `WORLD_SIZE=2` and `TP_SIZE=2`. Multi-GPU on a single node is
faster (GPU-to-GPU NVLink/PCIe instead of network), but multi-node lets you scale beyond the
GPUs available on a single machine.

:::{note}
If your cluster uses [GPU time-slicing](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/gpu-sharing.html),
Kubernetes may advertise more GPUs than `torch.cuda.device_count()` reports inside the pod.
In that case, use the multi-node configuration (`num_nodes=2, gpu=1`) instead of multi-GPU
on a single node (`num_nodes=1, gpu=2`), because `torchrun --nproc_per_node=auto` uses the
CUDA device count, not the Kubernetes GPU resource count.
:::

### Co-Scheduling Multi-Node Pods on the Same Physical Node

Tensor Parallelism is communication-intensive: every forward and backward step does an
`all-reduce` across the TP group. If you are forced into a multi-node configuration but the
target nodes have more than one physical GPU, you can pin both TrainJob Pods onto the same
node so NCCL can use NVLink or PCIe peer-to-peer instead of the Pod network.

The following `runtimePatches` entry attaches a `podAffinity` rule that co-schedules all
Pods of the TrainJob onto the same node. Replace ` ` with the
`metadata.name` of the TrainJob (JobSet propagates it as the `jobset.sigs.k8s.io/jobset-name`
label on every Pod):

```yaml
runtimePatches:
  - manager: trainer.kubeflow.org/kubeflow-sdk
    trainingRuntimeSpec:
      template:
        spec:
          replicatedJobs:
            - name: node
              template:
                spec:
                  template:
                    spec:
                      affinity:
                        podAffinity:
                          requiredDuringSchedulingIgnoredDuringExecution:
                            - labelSelector:
                                matchExpressions:
                                  - key: jobset.sigs.k8s.io/jobset-name
                                    operator: In
                                    values:
                                      - <your-trainjob-name>
                              topologyKey: kubernetes.io/hostname
```

## Next Steps

- Run the [Megatron-Core GPT Tensor Parallelism notebook](https://github.com/kubeflow/trainer/blob/master/examples/megatron/tensor-parallelism/megatron-core-gpt-tp.ipynb).
- Learn more about `TrainerClient()` APIs [in the Kubeflow SDK](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py).
- Read the [Megatron-Core documentation](https://docs.nvidia.com/megatron-core/developer-guide/latest/user-guide/index.html) for advanced configuration.
- Explore [Tensor Parallelism concepts](https://docs.nvidia.com/nemo/megatron-bridge/0.2.0/parallelisms.html) in NVIDIA's documentation.
