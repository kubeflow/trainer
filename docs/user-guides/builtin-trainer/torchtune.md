# TorchTune BuiltinTrainer

This guide describes how to fine-tune LLMs using `BuiltinTrainer` and `TorchTuneConfig`. `TorchTuneConfig` leverages [TorchTune](https://github.com/pytorch/torchtune) to streamline LLMs fine-tuning on Kubernetes. To understand the concept of `BuiltinTrainer`, see the [overview guide](index).

:::{note}
The supported model list can be seen in [this directory](https://github.com/kubeflow/trainer/tree/master/manifests/base/runtimes/torchtune). It's worth noticing that we do not support multi-node fine-tuning with TorchTune. However, **LoRA (PEFT) fine-tuning is supported** starting from Kubeflow Trainer V2.1.0 and SDK v0.2.0.

If you want to learn more about TorchTune BuiltinTrainer, please refer to [KEP-2401](https://github.com/kubeflow/trainer/tree/master/docs/proposals/2401-llm-trainer-v2) in Kubeflow Trainer.
:::

## Prerequisites

Before exploring this guide, make sure to follow [the Getting Started guide](https://www.kubeflow.org/docs/components/trainer/getting-started/) to understand the basics of Kubeflow Trainer.

```Python
# Get torchtune-llama3.2-1b TorchTune runtime.
from kubeflow.trainer import *

# List all TorchTune runtimes
client = TrainerClient()
for r in TrainerClient().list_runtimes():
    if r.name.startswith("torchtune"):
        print(r)
```

Output:

```sh
Runtime(name='torchtune-llama3.2-1b', trainer=Trainer(trainer_type=<TrainerType.BUILTIN_TRAINER: 'BuiltinTrainer'>, framework=<Framework.TORCHTUNE: 'torchtune'>, entrypoint=['tune', 'run'], accelerator='Unknown', accelerator_count='2.0'), pretrained_model=None)
Runtime(name='torchtune-llama3.2-3b', trainer=Trainer(trainer_type=<TrainerType.BUILTIN_TRAINER: 'BuiltinTrainer'>, framework=<Framework.TORCHTUNE: 'torchtune'>, entrypoint=['tune', 'run'], accelerator='Unknown', accelerator_count='2.0'), pretrained_model=None)
```

## Fine-Tune LLM with TorchTune BuiltinTrainer

The guide below shows how to fine-tune Llama-3.2-1B-Instruct with alpaca dataset by BuiltinTrainer.

### Use TorchTune BuiltinTrainer with train() API

You need to provide the following parameters to use TorchTune BuiltinTrainer with `train()` API:

- Reference to existing Runtimes.
- Initializer parameters.
- Trainer configuration.

Kubeflow TrainJob will train the model using the referenced TorchTune runtime `torchtune-llama3.2-1b`.

For example, you can use the `train()` API to fine-tune the Llama-3.2-1B-Instruct model using the alpaca dataset from HuggingFace with the code below (need 2 GPUs):

```python
job_name = client.train(
    runtime="torchtune-llama3.2-1b",
    initializer=Initializer(
        model=HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
            access_token="<YOUR_HF_TOKEN>"  # Replace with your Hugging Face token
        )
    ),
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            dataset_preprocess_config=TorchTuneInstructDataset(
                source=DataFormat.PARQUET,
            ),
        )
    )
)
```

### Watch the TrainJob Logs

We can use the `get_job_logs()` API to get the TrainJob logs.

#### Dataset Initializer

```python
from kubeflow.trainer.constants import constants

print("\n".join(client.get_job_logs(job_name, step=constants.DATASET_INITIALIZER)))
```

Output:

```sh
2025-07-02T08:24:01Z INFO     [__main__.py:16] Starting dataset initialization
2025-07-02T08:24:01Z INFO     [huggingface.py:28] Downloading dataset: tatsu-lab/alpaca
2025-07-02T08:24:01Z INFO     [huggingface.py:29] ----------------------------------------
Fetching 3 files: 100%|██████████| 3/3 [00:01<00:00,  1.82it/s]
2025-07-02T08:24:04Z INFO     [huggingface.py:40] Dataset has been downloaded
```

#### Model Initializer

```python
print("\n".join(client.get_job_logs(job_name, step=constants.MODEL_INITIALIZER)))
```

Output:

```sh
2025-07-02T08:24:23Z INFO     [__main__.py:16] Starting pre-trained model initialization
2025-07-02T08:24:23Z INFO     [huggingface.py:26] Downloading model: meta-llama/Llama-3.2-1B-Instruct
2025-07-02T08:24:23Z INFO     [huggingface.py:27] ----------------------------------------
Fetching 8 files: 100%|██████████| 8/8 [01:02<00:00,  7.87s/it]
2025-07-02T08:25:27Z INFO     [huggingface.py:43] Model has been downloaded
```

#### Training Node

```python
print("\n".join(client.get_job_logs(job_name)))
```

Output:

```sh
INFO:torchtune.utils._logging:Running FullFinetuneRecipeDistributed with resolved config:

<Complete TorchTune config used by `tune run` command>

DEBUG:torchtune.utils._logging:Setting manual seed to local seed 3686749453. Local seed is seed + rank = 3686749453 + 0
Writing logs to /workspace/output/logs/log_1751444966.txt
INFO:torchtune.utils._logging:Distributed training is enabled. Instantiating model and loading checkpoint on Rank 0 ...
INFO:torchtune.utils._logging:Instantiating model and loading checkpoint took 17.31 secs
INFO:torchtune.utils._logging:Memory stats after model init:
	GPU peak memory allocation: 2.33 GiB
	GPU peak memory reserved: 2.34 GiB
	GPU peak memory active: 2.33 GiB
/opt/conda/lib/python3.11/site-packages/torch/distributed/distributed_c10d.py:4631: UserWarning: No device id is provided via `init_process_group` or `barrier `. Using the current device set by the user.
  warnings.warn(  # warn only once
INFO:torchtune.utils._logging:Optimizer is initialized.
INFO:torchtune.utils._logging:Loss is initialized.
Generating train split: 52002 examples [00:00, 192755.58 examples/s]
INFO:torchtune.utils._logging:No learning rate scheduler configured. Using constant learning rate.
WARNING:torchtune.utils._logging: Profiling disabled.
INFO:torchtune.utils._logging: Profiler config after instantiation: {'enabled': False}
1|1625|Loss: 1.5839784145355225: 100%|██████████| 1625/1625 [21:54<00:00,  1.23it/s]INFO:torchtune.utils._logging:Saving checkpoint. This may take some time. Retrieving full model state dict...
INFO:torchtune.utils._logging:Getting full model state dict took 1.69 secs
INFO:torchtune.utils._logging:Model checkpoint of size 2.30 GiB saved to /workspace/output/epoch_0/model-00001-of-00001.safetensors
INFO:torchtune.utils._logging:Saving final epoch checkpoint.
INFO:torchtune.utils._logging:The full model checkpoint, including all weights and configurations, has been saved successfully.You can now use this checkpoint for further training or inference.
INFO:torchtune.utils._logging:Saving checkpoint took 6.64 secs
1|1625|Loss: 1.5839784145355225: 100%|██████████| 1625/1625 [22:01<00:00,  1.23it/s]
Running with torchrun...
```

### Get the Fine-Tuned Model

After TrainJob completes the fine-tuning task, the fine-tuned model will be stored into the `/workspace/output` directory. The TrainJob automatically creates and manages a PVC to store the model and dataset, which can be shared across Pods. You can access the fine-tuned model in another Pod by mounting the same PVC under `/workspace`, where you'll find the output in `/workspace/output`.

## Parameters

### Runtime

For TorchTune BuiltinTrainer, you can just find the runtime you want in the [manifest folder](https://github.com/kubeflow/trainer/tree/master/manifests/base/runtimes/torchtune), and get it using `get_runtime`. Like:

```python
runtime=client.get_runtime("torchtune-llama3.2-1b")
```

### Initializer

We'll use parameters in [Initializer](https://github.com/kubeflow/sdk/blob/7614d748d3046750a5ae12a55ddac10db9fd8e7d/python/kubeflow/trainer/types/types.py#L227-L239) to download dataset and model from remote storage.

#### Dataset Initializer

Currently, we only support downloading datasets from HuggingFace by defining [`HuggingFaceDatasetInitializer`](https://github.com/kubeflow/sdk/blob/7614d748d3046750a5ae12a55ddac10db9fd8e7d/python/kubeflow/trainer/types/types.py#L212-L217):

:::{note}
For `storage_uri` in Dataset Initializer, you need to specify **the exact path to data files**. This means you need to set it to `hf:// / /path/to/data/files`.

Currently, we support:

1. Data Directory: Use all data files under this directory. For example, `hf://tatsu-lab/alpaca/data` uses all data files under the `/data` directory of `tatsu-lab/alpaca` repo in HuggingFace.
2. Single Data File: Use the single data file given the path. For example, `hf://tatsu-lab/alpaca/data/xxx.parquet` uses the single `/data/xxx.parquet` data file of `tatsu-lab/alpaca` repo in HuggingFace.

:::

#### Model Initializer

Currently, we only support downloading models from HuggingFace by defining [`HuggingFaceModelInitializer`](https://github.com/kubeflow/sdk/blob/7614d748d3046750a5ae12a55ddac10db9fd8e7d/python/kubeflow/trainer/types/types.py#L220-L224):

#### Example Usage

```python
initializer=Initializer(
    dataset=HuggingFaceDatasetInitializer(
        storage_uri="hf://tatsu-lab/alpaca/data",
        access_token="<YOUR_HF_TOKEN>"  # Replace with your Hugging Face token
    ),
    model=HuggingFaceModelInitializer(
        storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
        access_token="<YOUR_HF_TOKEN>"  # Replace with your Hugging Face token
    )
)
```

### TorchTune BuiltinTrainer

#### Description

The `TorchTuneConfig` class is used for configuring TorchTune BuiltinTrainer that already includes the fine-tuning logic. You can find the API definition [here](https://github.com/kubeflow/sdk/blob/7614d748d3046750a5ae12a55ddac10db9fd8e7d/python/kubeflow/trainer/types/types.py#L78-L106).

#### Example Usage

```python
torchtune_config = TorchTuneConfig(
    dtype=DataType.BF16,
    batch_size=10,
    epochs=10,
    loss=Loss.CEWithChunkedOutputLoss,
    num_nodes=1,
    peft_config=LoraConfig(
        lora_rank=8,
        lora_alpha=16,
        lora_dropout=0.1,
    ),
    dataset_preprocess_config=TorchTuneInstructDataset(
        source=DataFormat.PARQUET,
        split="train[:95%]",
        train_on_input=True,
        new_system_prompt="You are an AI assistant. ",
        column_map={"input": "incorrect", "output": "correct"},
    ),
    resources_per_node={"gpu": 1},
)
```

## Next Steps

- Run the example to [fine-tune the Llama-3.2-1B-Instruct LLM](https://github.com/kubeflow/trainer/blob/master/examples/torchtune/llama3_2/alpaca-trainjob-yaml.ipynb)

- Check out the example to [fine-tune the Qwen-2.5-1.5B LLM](https://github.com/kubeflow/trainer/blob/master/examples/torchtune/qwen2_5/qwen2.5-1.5B-with-alpaca.ipynb)
