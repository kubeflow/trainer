# KEP-2401: Kubeflow LLM Trainer V2

## Authors

- Shao Wang - [@Electronic-Waste](https://github.com/Electronic-Waste)

Creation date: 2025-01-31

Google doc: http://bit.ly/4gp8JGd

## Overview

This document discusses the design of LLM Trainer for [Kubeflow Trainer v2](../2170-kubeflow-trainer-v2/README.md), tracked by [this issue](https://github.com/kubeflow/training-operator/issues/2401).

**We decided to implement a custom Trainer to fine-tune LLMs, which will be supported officially via TrainingRuntimes in Kubeflow upstream**. This will greatly ease the workload of writing fine-tuning scripts, and provide an in-box toolkit to fine-tune the LLMs with custom datasets and models for Data Scientists.

(The following picture comes from the slides of [Andrey and Yuki’s talk in KubeCon NA 2024](https://kccncna2024.sched.com/event/1i7nV?iframe=no), which explains the workflow of LLM fine-tuning very well)

![](./llm-fine-tuning-lifecycle.png)

## Motivation

Fine-tuning LLMs on Kubernetes is challenging for Data Scientists due to the complex Kubernetes configurations, diverse fine-tuning techniques, and different distributed strategies like data and model-parallelism. It’s crucial to hide the complex infrastructure configurations from users, and allow them to gracefully shift among diverse fine-tuning techniques and distributed strategies.

By now, Kubeflow Training Operator V1 has implemented a [Trainer for LLM](../2003-train-api/README.md) based on the HuggingFace library. However, it is **integrated with data and model initialization**, which are separated in Kubeflow Trainer V2, and has **limited fine-tuning mechanism support**. We need to migrate it to V2 and support more fine-tuning configurations and distributed strategies in accordance with the growing needs in custom LLM fine-tuning.

### Goals

- Build LLM fine-tuning recipes with `torchtune` and Kubeflow Trainer ClusterTrainingRuntime.
- Update the Kubeflow Python SDK to support flexible LLM fine-tuning with configurable parameters.
- Create community-supported `ClusterTrainingRuntime` for fine-tuning various foundational models (e.g. Mistral, LLama-70b, Gemma-7b).

### Non-Goals

- Upgrade LLM Trainer V1.
- Support training launchers besides `torchtune` (we can add more launchers after the initial implementation).
- Support frameworks like HuggingFace Transformers, Nvidia NeMo.

## Proposal

`torchtune` is a PyTorch-native library for easily authoring, fine-tuning and experimenting with LLMs. It provides rich support for LLM fine-tuning:

1. Modular native-PyTorch implementations of popular LLMs
2. Training [recipes](https://pytorch.org/torchtune/main/overview.html#key-concepts) for a variety of fine-tuning techniques
3. Support for distributed training using [FSDP2](https://github.com/pytorch/torchtitan/blob/main/docs/fsdp.md)
4. YAML [configs](https://pytorch.org/torchtune/main/overview.html#key-concepts) for easily configuring training runs

An example for using `torchtune`:

```bash
$ tune ls
RECIPE                                   CONFIG
full_finetune_single_device              llama2/7B_full_low_memory
                                         mistral/7B_full_low_memory
full_finetune_distributed                llama2/7B_full
                                         llama2/13B_full
                                         mistral/7B_full
lora_finetune_single_device              llama2/7B_lora_single_device
                                         llama2/7B_qlora_single_device
                                         mistral/7B_lora_single_device

$ tune run lora_finetune_single_device --config llama2/7B_lora_single_device epochs=1
INFO:torchtune.utils.logging:Running LoRAFinetuneRecipeSingleDevice with resolved config:
Writing logs to /tmp/lora_finetune_output/log_1713194212.txt
INFO:torchtune.utils.logging:Model is initialized with precision torch.bfloat16.
INFO:torchtune.utils.logging:Tokenizer is initialized from file.
INFO:torchtune.utils.logging:Optimizer and loss are initialized.
INFO:torchtune.utils.logging:Loss is initialized.
INFO:torchtune.utils.logging:Dataset and Sampler are initialized.
INFO:torchtune.utils.logging:Learning rate scheduler is initialized.
1|52|Loss: 2.3697006702423096:   0%|▏                     | 52/25880 [00:24<3:55:01,  1.83it/s]
```

By adopting `torchtune` as the low-level runtime for LLM fine-tuning, we can easily obtain the flexibility, efficiency and scalability brought by its unique "recipe-config" design, which will surely streamline and scale LLM fine-tuning on Kubernetes.

To hide users from complex Kubernetes configuations, we will provide a simple yet flexible Python SDK wrapping all specifications of models, datasets, training runtime and fine-tuning configs. Like this:

```python
job_id = TrainerClient().train(
    runtime=Runtime(
      name="torchtune-llama3.2-1b",
    ),
    initializer=Initializer(
        dataset=HuggingFaceDatasetInitializer(
            storage_uri="tatsu-lab/alpaca/data",
        ),
        model=HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
            access_token="<YOUR_HF_TOKEN>",  # Replace with your Hugging Face token,
        ),
    ),
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            batch_size=1,
            dataset_preprocess_config=TorchTuneInstructDataset(
                source=DataFormat.PARQUET,
            ),
            dtype="bf16",
            epochs=1,
            num_nodes=5,
            peft_config=LoraConfig(
                lora_rank=64,
                lora_alpha=128,
            ),
        ),
    ),
)
```

## Design Details

![](./llm-lifcycle-torchtune.png)

### `Torchtune` Config Override in `TrainJob` and `TrainingRuntime`

As is shown in the official guides, we can pass distributed arguments to `torchtune` and override some parameters in the config, like:

```bash
# Note that tune run only supports launching distributed runs by passing through arguments preceding the recipe directly to torchrun.
# Official document: https://pytorch.org/torchtune/main/tune_cli.html#run-a-recipe
# Proof from source code: https://github.com/pytorch/torchtune/blob/75965d4281b9b76c454630d015221b9933c77bf3/torchtune/_cli/run.py#L113-L118
tune run --nnodes=1 --nproc-per-node=2 lora_finetune_distributed \
    --config llama3_2/1B_lora \
    model.lora_attn_modules=[q_proj,k_proj,v_proj] \
    model.apply_lora_to_mlp=True \
    model.lora_rank=64 \
    model.lora_alpha=128

```

The CLI-based way allows us to mutate these parameters by overriding the `command` and `args` fields in the Trainer Node. By specifying override parameters in the TrainJob `.spec.trainer.command` and `.spec.trainer.args` fields, we can easily mutate these parameters, just like:

```YAML
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torchtune-llama3.2-1b
  labels:
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: Node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      image: ghcr.io/kubeflow/trainer/torchtune-trainer
                      command:
                        - tune run
                      args:
                        - full_finetune_single_device
                        - --config
                        - llama3_2/1B_full_single_device
...
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: tune-llama-lora-with-alpaca
  namespace: tenant-alpha
spec:
  runtimeRef:
    name: torchtune-llama3.2-1B
  trainer:
    command:
      - tune run
    args:
      - --nnodes=1
      - --nproc_per_node=2
      - lora_finetune_distributed
      - --config
      - llama3_2/1B_full
      - model.lora_attn_modules=[q_proj,k_proj,v_proj]
      - model.apply_lora_to_mlp=True
      - model.lora_rank=64
      - model.lora_alpha=128
    resourcesPerNode:
      requests:
        nvidia.com/gpu: 2
    numProcPerNode: 2
...
```

The trainer image will be built with packages related to PyTorch, CUDA, TorchTune. And we will maintain its Dockerfile under `cmd/trainers/torchtune/`.

### Propagte `torchtune` Settings with SDK

To provide a better user experience, we need to offer a simple SDK that allows users to easily modify config files. So, we introduce `TorchTuneConfig` dataclass in SDK. Then, users can fine-tune their LLMs without any knowledge about `torchtune`.

```python
# By default we can fine-tune models without any additional configurations from users
TrainerClient().train(
    runtime=client.get_runtime("torchtune-llama3.2-1b"),
)
```

#### Modify the `train()` API

As we discussed in [Github](https://github.com/kubeflow/trainer/pull/2410#discussion_r1963832826), the `train()` API mainly executes two types of training tasks:

1. **Type 1: Training with custom function/image**: A self-contained function/image that encapsulates the entire model training process (e.g. `CustomTrainer`).
2. **Type 2: Config-driven approach with existing Trainer**: A trainer that already includes training process, requiring only parameter adjustments (e.g. `BuiltinTrainer`).

So we plan to modify the `train()` API to:

```python
def train(
    self,
    runtime: types.Runtime = types.DEFAULT_RUNTIME,
    trainer: Optional[Union[CustomTrainer, BuiltinTrainer]] = None,
    initializer: Optional[Initializer] = None,
) -> str:
    pass

@dataclass
class CustomTrainer:
    func: Callable
    func_args: Optional[Dict] = None
    packages_to_install: Optional[List[str]] = None
    pip_index_url: Optional[str] = constants.DEFAULT_PIP_INDEX_URL
    num_nodes: Optional[int] = None
    resources_per_node: Optional[Dict] = None

@dataclass
class BuiltinTrainer:
    config: TorchTuneConfig

```

`CustomTrainer` defines the parameters for Type 1 tasks, and will add support for custom function in the initial stage.

`BuiltinTrainer` defines the parameters for Type 2 tasks, and will add support for post-training with `torchtune` in the early stage.

We natively support all `recipe` and `config` supported by `torchtune`, since `torchtune` has already provided us with default `config`. We just cannot mutate them if we do not support the corresponding mutation config.

#### `TorchTuneConfig` API Design

| Parameters | Type | What is it? |
| - | - | - |
| batch_size | Optional[int] | The number of samples processed before updating model weights. |
| dataset_preprocess_config | Optional[Union[TorchTuneInstructDataset, TorchTuneChatDataset, TorchTuneMultimodalDataset]] | Configuration for dataset preprocessing. |
| dtype | Optional[DataType] | The underlying data type used to represent the model and optimizer parameters. Currently, we only support `bf16` and `fp32`. |
| epochs | Optional[int] | The number of times the entire dataset is processed during training. |
| loss | Optional[Loss] | The loss algorithm we use to fine-tune the LLM, e.g. `torchtune.modules.loss.CEWithChunkedOutputLoss` |
| num_nodes | Optional[int] | The number of PyTorch Nodes in training |
| peft_config | Optional[Union[LoraConfig]] | Configuration for the PEFT(Parameter-Efficient Fine-Tuning), including LoRA/QLoRA/DoRA, etc. |
| resources_per_node | Optional[Dict] | The resource for each PyTorch Node |

```python
# Loss function for the TorchTune LLM Trainer.
class Loss(Enum):
    CEWithChunkedOutputLoss = "torchtune.modules.loss.CEWithChunkedOutputLoss"


# Data type for the TorchTune LLM Trainer.
class DataType(Enum):
    BF16 = "bf16"
    FP32 = "fp32"


# TorchTuneConfig DataClass
@dataclass
class TorchTuneConfig:
    dtype: Optional[DataType] = None
    batch_size: Optional[int] = None
    epochs: Optional[int] = None
    loss: Optional[Loss] = None
    peft_config: Optional[Union[LoraConfig]] = None
    dataset_preprocess_config: Optional[
        Union[TorchTuneInstructDataset, TorchTuneChatDataset, TorchTuneMultimodalDataset],
    ] = None
    num_nodes: Optional[int] = None,
    resources_per_node: Optional[Dict] = None,

```

#### `list_runtimes()` API

We should allow users to easily get the available TrainingRuntimes/ClusterTrainingRuntimes with the SDK, followed with information related to the trainer type, framework, model, and device. When users execute:

```python
TrainingClient().list_runtimes()
```

They are expected to get:

```
Runtime                             Trainer Type          Framework   Pretrained Model    Accelerator       Count

pytorch-distributed                 custom-trainer        pytorch     <undefined>         nvidia.com/GPU    2
torchtune-llama3.1-8B-finetuning    builtin-trainer       torchtune   Llama3.1-8B         nvidia.com/GPU    4
```

In that case, we plan to design `Runtime` dataclass as the following:

| Fields | Type | What is it? |
| - | - | - |
| name | str | The name of TrainingRuntime/ClusterTrainingRuntime. |
| pretrained_model | Optional[str] | The pretrained model specified for this runtime, e.g. `llama3.1-8B`. |
| trainer_type | str | The training method of this runtime, chosen from `custom-trainer`, `builtin-trainer`. |


```python
# Runtime DataClass
@dataclass
class Runtime:
    name: str
    trainer: Trainer
    pretrained_model: Optional[str] = None

```

`Trainer` dataclass:

| Fields | Type | What is it? |
| accelerator | str | The type of devices, e.g. `nvidia.com/gpu`. |
| accelerator_count | Union[str, float, int] | The number of devices. |
| entrypoint | Optional[List[str]] | Entrypoint for the trainer node. |
| framework | Framework | The ML framework used for training, e.g. `torch`, `torchtune`. |
| trainer_type | TrainerType | Selected from CustomTrainer and BuiltinTrainer. |

```python
class TrainerType(Enum):
    CUSTOM_TRAINER = CustomTrainer.__name__
    BUILTIN_TRAINER = BuiltinTrainer.__name__


class Framework(Enum):
    TORCH = "torch"
    DEEPSPEED = "deepspeed"
    MLX = "mlx"
    TORCHTUNE = "torchtune"


# Representation for the Trainer of the runtime.
@dataclass
class Trainer:
    trainer_type: TrainerType
    framework: Framework
    entrypoint: Optional[List[str]] = None
    accelerator: str = constants.UNKNOWN
    accelerator_count: Union[str, float, int] = constants.UNKNOWN

```

### Complement `torch` Plugin

#### Perform Mutation in `torch` Plugin

We need to modify the exsiting torch plugin to handle config override for `torchtune`, since currently torch plugin passes the distributed arguments by environment variables that begins with `PET_`, which is not allowed by `torchtune`. However, `torchtune` can share the same ML Policy with `torchrun` because `torchtune` is fully compatible with these distributed parameters.

We will add CLI-based parameters mutation to `EnforceMLPolicy()` in `pkg/runtime/framework/torch/torch.go` while still supporting env-based parameters mutation with `PET_`:

Related changes:

1. Add `Args` fields to internal runtime Information (`pkg/runtime/runtime.go`).

2. Insert distributed parameters to Trainer Args (`pkg/runtime/framework/plugins/torch/torch.go`)

#### Create Map from `TorchTuneConfig` to Specific Recipes and Configs

We will create a map from (`TorchTuneConfig`, `num_nodes`, `nproc_per_node`, `runtime`) to dedicated `recipe` and `config` in the server side. This will allow users to fine-tune their LLMs without knowing about `torchtune`'s recipes and configs and prevent SDK from changing frequently.

- How to Select `recipe`

1. `peft_config == None` && `num_nodes == 1` && `nproc_per_node == 1`: Use `full_finetune_single_device`.
2. `peft_config == None` && (`num_nodes >= 1` || `nproc_per_node > 1`): Use `full_finetune_distributed`.
3. `type(peft_confg) == LoraConfig` && `nproc_per_node == 1`: Use `lora_finetune_single_device`.
4. `type(peft_confg) == LoraConfig` && `nproc_per_node > 1`: Use `lora_finetune_distributed`.

- How to Select `config`

We will create one `ClusterTrainingRuntime` for one model. In this way, we can extract the model info in the `.spec.runtimeRef.name` field path of Trainjob, and select corresponding config file according to the `recipe`.

#### Validate Fine-Tuning Configurations

In order to ensure the validity of the configurations propagated by SDK, we plan to add some validating requirements to the TrainJob Webhook. We'll implement validations in torch plugin [`CustomValidationPlugin`](https://github.com/kubeflow/trainer/blob/1f443729ebdb3e792d4b9cd2b242f33b0e86fe14/pkg/runtime/framework/interface.go#L34-L37):

1. The ClusterTrainingRuntime referenced by `runtime` exists in the control plane.

### Maintain ClusterTrainingRuntimes in Manifests

According to our design in [this section](#propagate-torchtune-settings-with-sdk), we will create one `ClusterTrainingRuntime` for one model. We can maintain these `ClusterTraingRuntimes` under `manifests/base/runtimes`:

```
manifests/
|-- base/
|   |-- runtimes/
|   |   |-- kustomization.yaml
|   |   |-- mpi_distributed.yaml                    # MPI Distributed Runtime
|   |   |-- torch_distributed.yaml                  # PyTorch Distributed Runtime
|   |   |-- torchtune/                              # TorchTune Fine-tuning Runtimes
|   |   |   |-- kustomization.yaml
|   |   |   |-- gemma2/
|   |   |   |   |-- gemma_2B.yaml
|   |   |   |   |-- gemma_7B.yaml
|   |   |   |   |-- gemma_27B.yaml
|   |   |   |-- llama3_1/
|   |   |   |   |-- llama3_1_8B.yaml
|   |   |   |   |-- llama3_1_70B.yaml
|   |   |   |-- llama3_2/
|   |   |   |   |-- llama3_2_1B.yaml
|   |   |   |   |-- llama3_2_3B.yaml
|   |   |   |-- llama3_2_vision/
|   |   |   |   |-- llama3_2_vision_11B.yaml
|   |   |   |   |-- llama3_2_vision_90B.yaml
|   |   |   |-- llama3_3/
|   |   |   |   |-- llama3_3_70B.yaml
|   |   |   |-- mistral/
|   |   |   |   |-- mistral_7B.yaml
|   |   |   |-- qwen2_5/
|   |   |   |   |-- qwen2_5_0.5B.yaml
|   |   |   |   |-- qwen2_5_1.5B.yaml
|   |   |   |   |-- qwen2_5_3B.yaml
|   |   |   |   |-- qwen2_5_7B.yaml
|   |   |   |-- . . .
|   |-- crds/
|   |-- . . .
|-- overlays/
|-- third-party/
```

### Support Some Common PEFT Mechanisms

We need to support some common PEFT mechnisms like LoRA, QLoRA, DoRA to allow users to optimize the memory usage when they are fine-tuning the LLMs. This is crucial for users who have limited resources and want to fine-tune their model at the minimum cost.

The *LoraConfig* represents the config of LoRA we use to fine-tune the model.

| Parameters | Type | What is it? |
| - | - | - |
| apply_lora_to_mlp | Optional[bool] | Whether to apply LoRA to the MLP in each transformer layer |
| apply_lora_to_output | Optional[bool] | Whether to apply LoRA to the model’s final output projection |
| lora_attn_modules | Optional[List[str]] | A list of strings specifying which layers of the model to apply LoRA: 1. `q_proj` applies LoRA to the query projection layer. 2. `k_proj` applies LoRA to the key projection layer. 3. `v_proj` applies LoRA to the value projection layer. 4. `output_proj` applies LoRA to the attention output projection layer. |
| lora_rank | Optional[int] | The rank of the low rank decomposition. |
| lora_alpha | Optional[int] | The scaling factor that adjusts the magnitude of the low-rank matrices’ output |
| lora_dropout | Optional[float] | The probability of applying Dropout to the low rank updates |
| quantize_base | Optional[bool] | Whether to enable model quantization |
| use_dora | Optional[bool] | Whether to enable DoRA |

```python
@dataclass
class LoraConfig:
    apply_lora_to_mlp: Optional[bool] = None
    apply_lora_to_output: Optional[bool] = None
    lora_attn_modules: Optional[List[str]] = None
    lora_rank: Optional[int] = None
    lora_alpha: Optional[int] = None
    lora_dropout: optional[float] = None
    quantize_base: optional[bool] = None
    use_dora: Optional[bool] = None

```

### Model & Dataset Initialization / Model Exporting

For model and dataset initialization, we'll reuse the existing [Kubeflow Trainer Initializers](https://github.com/kubeflow/trainer/tree/master/pkg/initializer). For example:

```yaml
# Dataset Initialization
containers:
  - name: dataset-initializer
    image: ghcr.io/kubeflo/trainer/dataset-initializer
    env:
      - name: STORAGE_URI
        value: hf://tatsu-lab/alpaca
    volumeMounts:
      - mountPath: /workspace/dataset
        name: initializer
volumes:
  - name: initializer
    persistentVolumeClaim:
      claimName: initializer

```

```yaml
# Model Initialization
containers:
  - name: model-initializer
    image: ghcr.io/kubeflow/trainer/model-initializer
    env:
      - name: STORAGE_URI
        value: hf://meta-llama/Llama-3.2-1B-Instruct
    volumeMounts:
      - mountPath: /workspace/model
        name: initializer
volumes:
  - name: initializer
    persistentVolumeClaim:
      claimName: initializer

```

As for the [model exporter](https://github.com/kubeflow/trainer/issues/2245), we haven't implemented it yet in Kubeflow Trainer. But we'll use it as the exporter for our fine-tuned LLMs once it's ready. For example:

```yaml
# Model Exporting
containers:
  - name: model-exporter
    image: ghcr.io/kubeflow/trainer/model-exporter
    volumeMounts:
      - mountPath: /workspace/adapters
        name: model-exporter
volumes:
  - name: model-exporter
    persistentVolumeClaim:
      claimName: model-exporter

```

### Dataset Preprocess / Tokenizer

`torchtune` has supported several types of [dataset classes](https://pytorch.org/torchtune/main/basics/datasets_overview.html), including Instruct, Chat, Multimodal, which will preprocess the dataest for us automatically. We just need to configure it in the SDK and override it to the config:

**Instruct Dataset**

```python
@datasetclass
class TorchTuneInstructDataset:
    source: Optional[str] = None
    split: Optional[str] = None
    train_on_input: Optional[bool] = None
    new_system_prompt: Optional[str] = None
    column_map: Optional[Dict[str, str]] = None

```

**Chat Dataset**

```python
@datasetclass
class TorchTuneChatDataset:
    source: Optional[str] = None
    split: Optional[str] = None
    conversation_column: Optional[str] = None
    conversation_style: Optional[str] = None
    train_on_input: Optional[bool] = None
    new_system_prompt: Optional[str] = None

```

**Multimodal Dataset**

```python
@datasetclass
class TorchTuneMultimodalDataset:
    source: Optional[str] = None
    split: Optional[str] = None
    column_map: Optional[Dict[str, str]] = None
    image_dir: Optional[str] = None
    image_tag: Optional[str] = None

```

As for the tokenzier, `torchtune` has specified it in every training configs, such as [this](https://github.com/pytorch/torchtune/blob/e6cba2532d51a53936c7646bd4cdaa6b2b57ed66/recipes/configs/llama3_2/1B_lora_single_device.yaml#L29-L33). We're going to gracefully add custom tokenizer support in the future, once users request it.

## Test Plan

### Testing Environment

For the GPU testing environment, hopefully, we will accquire GPU infra ([#2432](https://github.com/kubeflow/trainer/issues/2432)) from CNCF.

### E2e Test

For e2e tests, we will initially create a set of Jupyter Notebook examples including full fine-tuning and LoRA fine-tuning.
We will use [papermill](https://github.com/nteract/papermill) to execute these notebooks as e2e tests, ensuring the functionality of both the control plane and the SDK.

## Implementation History

- 2025-01-31: Create KEP-2401 doc
- 2025-03-11: KEP-2401 1st version & Start implementation
- 2025-03-24: Add new `Runtime` dataclass design

## Alternatives

### Native PyTorch Launcher - `torchrun`

We decide to adopt `torchrun` as the launcher of LLM Trainer V2, and support multiple frameworks and fine-tuning techniques.

Supported frameworks:

1. HuggingFace Transformers: HuggingFace Transformers provides unified trainers class `Trainer` for model training, and `SFTTrainer` for supervised fine-tuning. It has rich in-box support for features like PEFT (`huggingface.peft`), FSDP (`huggingface.accelerate`), and model quantization (`huggingface.optimum`). These libraries are fully compatible with `torchrun` CLI. Since most well-known open-source LLMs lie in Huggingface and support the Transformer library, it’s convenient and acceptable to implement our LLM Trainer with the Transformer library. Our LLM Trainer V1 is also implemented in this way.

2. Native PyTorch: A backend that does not use the Trainer or SFTTrainer library in HF Transformers. This will provide users who don’t want to rely on the HF Transformers library with an alternative for fine-tuning.

3. Nvidia NeMo: To be added.

Supported fine-tuning techniques:

1. PEFT(Parameter Efficient Fine Tuning): Including LoRA, QLoRA, Adapter Prompt and Prefix Tuning.

2. Sharding Policies: Including FSDP and DeepSeed ZeRO.

The workflow of LLM Trainer V2 is shown in the graph below:

![](./llm-trainer-v2-workflow.png)

To hide users from complex Kubernetes configuations, we will provide a simple yet flexible Python SDK wrapping all specifications of models, datasets, training runtime and fine-tuning configs. Like this:

```python
TrainingClient().train(
    dataset_config=HuggingFaceDatasetConfig(
        storage_uri="tatsu-lab/alpaca",
    ),
    trainer=Trainer(
        fine_tuning_config=FineTuningConfig(
            framework="huggingface",
            dataset_class="Instruction",
            peft_config=LoraConfig(r=4),
            sharding_config=FsdpConfig(...),
            kwargs={},
        ),
        num_nodes=5,
    ),
    runtime_ref=llm_runtime,
)
```

And it's worthwhile to notice that we'll preprocess dataset for users with builtin dataset classes or a customized one. If users want to preprocess datasets by themselves, they need to implement a customized data class with specified methods implemented and pass it to the Python SDK.

In the future, we'll provide users with more options on launchers (`torchtune`, `accelerate`), frameworks (TensorFlow, Jax, etc.) and fine-tuning techniques (RLHF, Distilation, etc.).

### HF Accelerate CLI - `accelerate`

Huggingface Accelerate CLI is a simplified distributed training launch tool, which is **targeted to junior users not familiar with distributed training**. The official slogan for Huggingface Accelerate is “Run your raw PyTorch training script on any kind of device”. There are several advantages to adopt it:

1. In-box support for HuggingFace Transformers and Datasets libraries
2. Set many proper default values for mixed-precision training and device allocation
3. Hide some complex configurations like communication backend & env variables.

**If we decide to support HF Accelerate CLI, we need to implement a new runtime plugin**. And also, users will lose their control over the training process, which is unacceptable if they want to have more flexibility and set up their own configurations in the distributed training phase.

If time permitted, it would be great to build our impacts among junior learners by a simple distributed training backend.

```python
  import torch
  import torch.nn.functional as F
  from datasets import load_dataset
+ from accelerate import Accelerator

- device = 'cpu'
+ accelerator = Accelerator()

- model = torch.nn.Transformer().to(device)
+ model = torch.nn.Transformer()
  optimizer = torch.optim.Adam(model.parameters())

  dataset = load_dataset('my_dataset')
  data = torch.utils.data.DataLoader(dataset, shuffle=True)

+ model, optimizer, data = accelerator.prepare(model, optimizer, data)

  model.train()
  for epoch in range(10):
      for source, targets in data:
-         source = source.to(device)
-         targets = targets.to(device)

          optimizer.zero_grad()
          output = model(source)
          loss = F.cross_entropy(output, targets)

-         loss.backward()
+         accelerator.backward(loss)

          optimizer.step()

```

```bash
$ accelerate config
-------------------------------------------------------------------------------------------------------------------------------------In which compute environment are you running?
This machine
-------------------------------------------------------------------------------------------------------------------------------------Which type of machine are you using?
multi-CPU
How many different machines will you use (use more than 1 for multi-node training)? [1]:
Should distributed operations be checked while running for errors? This can avoid timeout issues but will be slower. [yes/NO]: yes
Do you want to use Intel PyTorch Extension (IPEX) to speed up training on CPU? [yes/NO]:yes
Do you want accelerate to launch mpirun? [yes/NO]: no
Do you wish to optimize your script with torch dynamo?[yes/NO]:yes
-------------------------------------------------------------------------------------------------------------------------------------Which dynamo backend would you like to use?
Please select a choice using the arrow or number keys, and selecting with enter
inductor
Do you want to customize the defaults sent to torch.compile? [yes/NO]: yes
-------------------------------------------------------------------------------------------------------------------------------------Which mode do you want to use?
default
Do you want the fullgraph mode or it is ok to break model into several subgraphs? [yes/NO]: yes
Do you want to enable dynamic shape tracing? [yes/NO]: yes
How many processes should be used for distributed training? [1]:1
-------------------------------------------------------------------------------------------------------------------------------------Do you wish to use mixed precision?
fp16
accelerate configuration saved at /home/xxx/.cache/huggingface/accelerate/default_config.yaml

$ accelerate launch {my_script.py}
```

### Design Details - `torchtune`

#### Allow users to specify `recipe` and `config` in `TorchTuneConfig`

> This option is abandoned because we prefer to fine-tune LLMs with just `runtime` specified, which would provide better user experience.

| Parameters | Type | What is it? |
| - | - | - |
| recipe | str | The name of recipe in `torchtune` we choose. |
| config | str | The name of config in `torchtune` we chooose. |
| dtype | Optional[str] | The underlying data type used to represent the model and optimizer parameters. Currently, we only support `bf16` and `fp32`. |
| batch_size | Optional[int] | The number of samples processed before updating model weights. |
| epochs | Optional[int] | The number of samples processed before updating model weights. |
| loss | Optional[str] | The loss algorithm we use to fine-tune the LLM, e.g. `torchtune.modules.loss.CEWithChunkedOutputLoss` |
| peft_config | Optional[Union[LoraConfig]] | Configuration for the PEFT(Parameter-Efficient Fine-Tuning), including LoRA/QLoRA/DoRA, etc. |
| dataset_preprocess_config | Optional[Union[TorchTuneInstructDataset, TorchTuneChatDataset, TorchTuneMultimodalDataset]] | Configuration for dataset preprocessing. |
| num_nodes | Optional[int] | The number of PyTorch Nodes in training |
| resource_per_node | Optional[Dict] | The resource for each PyTorch Node |

```python
# TorchTuneConfig DataClass
@dataclass
class TorchTuneConfig:
    recipe: str
    config: str
    dtype: Optional[str] = None
    batch_size: Optional[int] = None
    epochs: Optional[int] = None
    loss: Optional[str] = None
    peft_config: Optional[Union[LoraConfig]] = None
    dataset_preprocess_config: Optional[
        Union[TorchTuneInstructDataset, TorchTuneChatDataset, TorchTuneMultimodalDataset],
    ] = None
    num_nodes: Optional[int] = None,
    resources_per_node: Optional[Dict] = None,

```

In this way, we just need to create one `ClusterTrainingRuntime` for LLM fine-tuning with `torchtune`. Like we showed in the [example](#compliment-torch-plugin). And we will put this `ClusterTrainingRuntime` under:

```
manifests/
|-- base/
|   |-- runtimes/
|   |   |-- kustomization.yaml
|   |   |-- mpi_distributed.yaml                    # MPI Distributed Runtime
|   |   |-- torch_distributed.yaml                  # PyTorch Distributed Runtime
|   |   |-- torchtune_llm_finetuning.yaml           # TorchTune LLM Fine-tuning Runtime
|   |-- crds/
|   |-- . . .
|-- overlays/
|-- third-party/
```

### Design Details - `torchrun`

#### Multiple Frameworks Support

We decide to implement multiple backends for LLM Trainer launched by `torchrun`, which would be friendly with users of different fine-tuning frameworks, such as HuggingFace Transformers, Nvidia NeMo, Native PyTorch etc.

p.s. This idea comes from [Deepanker Gupta](https://github.com/deepanker13): [Support multiple backends](https://docs.google.com/document/d/1KCNqE66vc1lkecsX_Bcpz3l3d0M-jDYrwVq2nUq_5dc/edit?usp=sharing)

```python
# Registry to map backends to their respective handlers
BACKEND_HANDLERS = {}

def register_backend(name):
    def decorator(cls):
        BACKEND_HANDLERS[name] = cls()
        return cls
    return decorator

# Abstract base class for all backends
class BaseFineTuner:
    def fine_tune(self, model_name, dataset, **kwargs):
        raise NotImplementedError

# Native PyTorch Implementation
@register_backend('pytorch')
class NativePyTorchFineTuner(BaseFineTuner):
    def fine_tune(self, model_name, dataset, **kwargs):
        from torch.nn.parallel import DistributedDataParallel as DDP
        # Implement training logic here
        print(f"Training HuggingFace model {model_name}...")



# HuggingFace Implementation
@register_backend('huggingface')
class HuggingFaceFineTuner(BaseFineTuner):
    def fine_tune(self, model_name, dataset, **kwargs):
        from transformers import Trainer, TrainingArguments
        # Example: Parse Hugging Face-specific parameters
        training_args = TrainingArguments(
            output_dir=kwargs.get("output_dir", "./results"),
            learning_rate=kwargs.get("learning_rate", 5e-5),
            num_train_epochs=kwargs.get("epochs", 3),
            per_device_train_batch_size=kwargs.get("batch_size", 16),
        )
        # Implement training logic here
        print(f"Training HuggingFace model {model_name}...")

# NVIDIA NeMo Implementation
@register_backend('nemo')
class NeMoFineTuner(BaseFineTuner):
    def fine_tune(self, model_name, dataset, **kwargs):
        import nemo.collections.asr as nemo_asr
        # Example: Parse NeMo-specific parameters
        asr_model = nemo_asr.models.EncDecCTCModel.from_pretrained(model_name)
        # Implement NeMo fine-tuning logic here
        print(f"Fine-tuning NVIDIA NeMo model {model_name}...")

# Unified function
def fine_tune(model_name, dataset, backend, **kwargs):
    if backend not in BACKEND_HANDLERS:
        raise ValueError(f"Unsupported backend: {backend}")
    BACKEND_HANDLERS[backend].fine_tune(model_name, dataset, **kwargs)

```

#### Data Preprocess

Different datasets have vastly different keys and usage. For example, instruction datasets (e.g. [tatsu-lab/alpaca](https://huggingface.co/datasets/tatsu-lab/alpaca)) always include keys like `instruction`, `input`, `output` and `text`. However, question answering datasets (e.g. [openai/gsm8k](https://huggingface.co/datasets/openai/gsm8k)) contain columns like `question` and `answer`. It’s impossible to implement a unified dataset class suitable for every datasets on HuggingFace. **Different types of tasks need different implementations** so that they can preprocess data in a specific way.

Based on the reasons above, we decide to **provide multiple built-in dataset classes** for data processing. They can be used directly by specifying the `dataset_class` parameter in Python SDK (e.g. `dataset_class="instruction"`). Meanwhile, we also **allow users to define customized dataset classes with specified methods implemented** and pass it to the `dataset_class` parameter in Python SDK (e.g. `dataset_class=CustomDatasetClass`).

```python
from torch.utils.data import Dataset

DATASET_REGISTRY = {}

def register_dataset(name):
    def decorator(cls):
        DATASET_REGISTRY[name] = cls
        return cls
    return decorator

# Abstract Dataset Class
class InitMethod(ABC):
    @abstractmethod
    def __init__(self, dataset_config, tokenizer, partition="train"):
        raise NotImplementedError()

@register_dataset("instruction")
class InstructionDataset(Dataset, InitMethod):
    def __init__(self, dataset_config, tokenizer, partition="train"):
        # Some code here

    def __len__(self):
        # Some code here

    def __getitem__(self, index):
        # Some code here

```

### Fine-Tuning Config - `torchrun`

**AdapterPrompt Config(TBD)**

The *AdapterConfig* represents the config of AdapterPrompt we use to fine-tune the model.

| Parameters | What is it? |
| - | - |
| adapter_len | The length of adapter |
| adapter_layers | The number of layers that we insert adapter |

```python
# AdapterConfig DataClass
@dataclass
class AdapterConfig:
    adapter_len: int = 10
    adapter_layers: int = 30

```

**PrefixTuning Config(TBD)**

The *PrefixConfig* represents the config of PrefixTuning we use to fine-tune the model.

| Parameters | What is it? |
| - | - |
| num_virtual_tokens | The number of virtual tokens |

```python
# PrefixConfig DataClass
@dataclass
class PrefixConfig:
    num_virtual_tokens: int = 30

```

**FSDP Config(TBD)**

The *FsdpConfig* represents the config of FSDP we use to fine-tune the model.

| Parameters | What is it? |
| - | - |
| mixed_precision | Whether to enable mixed precision training |
| use_fp16 | Whether to use FP16 during the mixed precision training |
| fsdp_cpu_offload | Whether to offload some weights and optimizer states to cpu |
| sharding_strategy | The sharding strategy for FSDP, e.g. FULL_SHARD (default), HYBRID_SHARD, SHARD_GRAD_OP, NO_SHARD. |
| hsdp | Whether to enable Hybrid Shard Data Parallel (HSDP) |
| sharding_group_size | Specify the GPU num in the sharding group when hsdp set to true |
| replica_group_size | The number of sharding groups |
| checkpoint_type | Specify the type of model checkpoints |
| fsdp_activation_checkpointing | Whether to enable Activation Checkpointing |

```python
# FsdpConfig DataClass
@dataclass
class FsdpConfig:
    mixed_precision: bool = True
    use_fp16: bool = False
    fsdp_cpu_offload: bool=False
    sharding_strategy: ShardingStrategy = ShardingStrategy.FULL_SHARD
    hsdp: bool = False
    sharding_group_size: int = 0 # requires hsdp to be set.
    replica_group_size: int = 0 #requires hsdp to be set.
    checkpoint_type: StateDictType = StateDictType.SHARDED_STATE_DICT
    fsdp_activation_checkpointing: bool = True

```

**ZeRO Config(TBD)**

The *ZeroConfig* represents the config of DeepSeed ZeRO we use to fine-tune the model.

| Parameters | What is it? |
| - | - |
| stage | The stage of DeepSeed ZeRO. |
| zero_cpu_offload | Whether to offload some weights and optimizer states to cpu |
| checkpoint_type | Specify the type of model checkpoints |
| mixed_precision | Whether to enable mixed precision training |
| use_fp16 | Whether to use FP16 during the mixed precision training |

```python
# ZeroConfig DataClass
@dataclass
class ZeroConfig:
    stage: int = 0
    zero_cpu_offload: bool = False
    checkpoint_type: StateDictType = StateDictType.SHARDED_STATE_DICT
    mixed_precision: bool = True
    use_fp16 : bool = False

```
