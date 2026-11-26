# PyTorch FSDP Guide

This guide shows how to fine-tune T5 for text summarization with PyTorch Fully Sharded
Data Parallel (FSDP) and the Kubeflow Python SDK. It adapts the
[Trainer v1 T5 FSDP notebook](https://github.com/kubeflow/trainer/blob/release-1.9/examples/pytorch/fsdp/fine-tune-t5-with-fsdp.ipynb)
to TrainJob and the existing `torch-distributed` runtime. An equivalent
[notebook](https://github.com/kubeflow/trainer/blob/master/examples/pytorch/fsdp/fine-tune-t5-with-fsdp.ipynb)
is available.

FSDP shards model parameters, gradients, and optimizer state across GPUs. The runtime
launches the processes with `torchrun`; your training function wraps the model with
FSDP. You do not need a custom TrainingRuntime, an FSDP-specific launcher flag, or
`FSDP_*` environment variables to enable it.

## Prerequisites

- Follow [Getting Started](../getting-started/index) to install Kubeflow Trainer and
  configure Kubernetes access from your notebook or Python environment.
- Ask your platform administrator to make the `torch-distributed` runtime available.
  This example uses NVIDIA GPUs, a CUDA-enabled PyTorch image, and the NCCL backend.
- Have four allocatable GPUs: the example requests two training nodes with two GPUs
  each. These are training pods, not necessarily distinct Kubernetes worker machines.
  Ensure that NCCL communication between the pods is supported by your cluster network.
- Allow training pods to download packages from PyPI, the T5 model from Hugging Face,
  and the WikiHow CSV from S3. The full CSV is downloaded before selecting 1,500 rows;
  use a smaller accessible CSV with `text` and `headline` columns for a short trial.
- Check model and dataset terms before using them. This is a training example, not a
  quality benchmark or a production fine-tuning recipe.

Install the client in your notebook environment:

```sh
pip install -U kubeflow
```

## Define the Training Function

The function below uses [T5-base](https://huggingface.co/google-t5/t5-base) and the
[WikiHow dataset](https://github.com/mahnazkoupaee/WikiHow-Dataset), as in the v1 notebook.
All imports are inside the function because the SDK sends it to each training pod.
Each process chooses its GPU using `LOCAL_RANK` and initializes the distributed group.
`DistributedSampler` assigns data to ranks and reshuffles it each epoch. If the number
of rows is not divisible by the world size, the sampler pads with repeated examples
so every rank performs the same number of steps.

The model stays on CPU until FSDP wraps each `T5Block`. `FULL_SHARD` shards parameters,
gradients, and optimizer state; the optimizer must be created after wrapping the model.
T5 shares embedding weights, which remain managed by the outer FSDP wrapper. Labels
use `-100` for padding so padding does not contribute to the loss.

This example deliberately uses the FSDP1 API to match the original notebook. It is
not an FSDP2 example. The runtime's installed PyTorch must support this API; use
`TrainerClient().get_runtime_packages()` as shown in the [PyTorch guide](pytorch)
to inspect your runtime before starting.

```python
def fine_tune_t5(
    model_name="google-t5/t5-base",
    dataset_url="https://public-nlp-datasets.s3.us-west-2.amazonaws.com/wikihowAll.csv",
    num_samples=1500,
    epochs=2,
    batch_size=4,
):
    import functools
    import os

    import torch
    import torch.distributed as dist
    from datasets import load_dataset
    from torch.distributed.fsdp import FullyShardedDataParallel as FSDP
    from torch.distributed.fsdp import ShardingStrategy
    from torch.distributed.fsdp.wrap import transformer_auto_wrap_policy
    from torch.utils.data import DataLoader, DistributedSampler
    from transformers import AutoTokenizer, T5ForConditionalGeneration, set_seed
    from transformers.models.t5.modeling_t5 import T5Block

    if not torch.cuda.is_available():
        raise RuntimeError("This FSDP example requires NVIDIA GPUs and NCCL.")

    local_rank = int(os.environ["LOCAL_RANK"])
    torch.cuda.set_device(local_rank)
    device = torch.device("cuda", local_rank)
    dist.init_process_group(backend="nccl")

    try:
        set_seed(42)
        rank = dist.get_rank()
        tokenizer = AutoTokenizer.from_pretrained(model_name)
        dataset = load_dataset("csv", data_files=dataset_url, split="train")
        dataset = dataset.filter(lambda row: bool(row["text"] and row["headline"]))
        dataset = dataset.select(range(min(num_samples, len(dataset))))
        if len(dataset) < dist.get_world_size():
            raise ValueError("Use at least one training example per GPU.")

        def tokenize(batch):
            inputs = tokenizer(
                ["summarize: " + text for text in batch["text"]],
                max_length=512,
                padding="max_length",
                truncation=True,
            )
            targets = tokenizer(
                text_target=batch["headline"],
                max_length=150,
                padding="max_length",
                truncation=True,
            )
            # Ignore padding tokens when calculating the T5 loss.
            inputs["labels"] = [
                [token if token != tokenizer.pad_token_id else -100 for token in row]
                for row in targets["input_ids"]
            ]
            return inputs

        dataset = dataset.map(
            tokenize, batched=True, remove_columns=dataset.column_names
        )
        dataset.set_format("torch")
        sampler = DistributedSampler(dataset, shuffle=True, seed=42)
        loader = DataLoader(dataset, batch_size=batch_size, sampler=sampler)

        # Keep the model on CPU until FSDP moves each wrapped unit to its GPU.
        model = T5ForConditionalGeneration.from_pretrained(model_name)
        model.config.use_cache = False
        model = FSDP(
            model,
            auto_wrap_policy=functools.partial(
                transformer_auto_wrap_policy, transformer_layer_cls={T5Block}
            ),
            sharding_strategy=ShardingStrategy.FULL_SHARD,
            device_id=device,
        )
        # FSDP changes the parameter variables, so construct the optimizer afterwards.
        optimizer = torch.optim.AdamW(model.parameters(), lr=2e-5)

        for epoch in range(epochs):
            sampler.set_epoch(epoch)
            model.train()
            totals = torch.zeros(2, device=device, dtype=torch.float64)
            for batch in loader:
                batch = {key: value.to(device) for key, value in batch.items()}
                optimizer.zero_grad(set_to_none=True)
                loss = model(**batch).loss
                loss.backward()
                optimizer.step()
                tokens = (batch["labels"] != -100).sum()
                totals[0] += loss.detach().double() * tokens
                totals[1] += tokens
            dist.all_reduce(totals, op=dist.ReduceOp.SUM)
            if rank == 0:
                print(f"Epoch {epoch + 1}: loss={totals[0] / totals[1]:.4f}")
    finally:
        dist.destroy_process_group()
```

## Submit the TrainJob

Use `CustomTrainer` with `TrainerClient`, not the v1 `TrainingClient`/`PyTorchJob` API.
The SDK installs the extra packages in every training pod. PyTorch itself comes from
the existing runtime image; do not replace it with a different CUDA build via pip.
The package pins below apply to this example, not to the Kubeflow SDK.

```python
from kubeflow.trainer import CustomTrainer, TrainerClient

client = TrainerClient()
job_id = client.train(
    runtime=client.get_runtime("torch-distributed"),
    trainer=CustomTrainer(
        func=fine_tune_t5,
        func_args={"num_samples": 1500, "epochs": 2, "batch_size": 4},
        num_nodes=2,
        resources_per_node={"gpu": 2, "cpu": 4, "memory": "32Gi"},
        packages_to_install=[
            "transformers==4.51.3",
            "datasets==3.5.0",
            "sentencepiece==0.2.0",
        ],
    ),
)
```

The Torch plugin starts one process per requested GPU, so this configuration has
`WORLD_SIZE=4`. To try two GPUs in one pod, set `num_nodes=1` and keep `"gpu": 2`.
Do not switch this example to CPU/Gloo: FSDP here requires an accelerator. Reduce
`batch_size` or use `model_name="google-t5/t5-small"` in `func_args` if you run out of
GPU memory. The CPU/memory requests above are a starting point, not a GPU memory guarantee.

## Inspect and Clean Up

Wait for completion, then inspect the rank-zero loss in the first training node's logs.
The call below waits up to one hour. If it times out during downloads or training,
the TrainJob keeps running; inspect its logs and wait again, or delete it explicitly.
Increase `timeout` if your dataset download or training needs more time:

```python
client.wait_for_job_status(name=job_id, timeout=3600)
print("\n".join(client.get_job_logs(name=job_id, step="node-0")))
```

The loss is averaged over non-padding target tokens across ranks. It measures the
training run only; the example has no evaluation or checkpoint export. Model weights
and downloaded data are ephemeral in the training pods. For continued training,
add distributed checkpointing and persistent storage before deleting the job.

If pods stay Pending, check GPU capacity and namespace quotas. For download or import
failures, inspect pod logs and network access. For NCCL initialization failures, check
GPU visibility and inter-pod networking with your administrator.

Delete the TrainJob when you no longer need its pods and logs:

```python
client.delete_job(name=job_id)
```

## Next Steps

- See the [PyTorch guide](pytorch) for distributed environment setup and other SDK examples.
- Read the [PyTorch FSDP documentation](https://docs.pytorch.org/docs/stable/fsdp.html)
  before changing wrapping, precision, or checkpointing.
- Use the [DeepSpeed guide](deepspeed) if you need the separate DeepSpeed runtime.
