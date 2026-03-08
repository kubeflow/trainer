# Builtin Trainer

Configuration-driven training with predefined training scripts for rapid LLM fine-tuning and model experimentation.

## Overview

Kubeflow Trainer SDK's `train()` API supports two types of trainers:

1. **CustomTrainer**: Write complete Python functions controlling the entire training workflow
2. **BuiltinTrainer**: Use predefined training scripts with configurable parameters

BuiltinTrainer is designed for configuration-driven TrainJobs using predefined training scripts, making it especially well-suited for large language model fine-tuning tasks where you want to focus on hyperparameters and dataset configuration rather than training loop implementation.

## CustomTrainer vs BuiltinTrainer

### CustomTrainer

With CustomTrainer, you write a complete Python function that defines your entire training workflow:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

def my_training_function():
    """Complete control over training logic."""
    import torch
    import torch.distributed as dist

    # You write everything:
    # - Model initialization
    # - Data loading
    # - Training loop
    # - Optimization
    # - Logging
    # - Checkpointing

    dist.init_process_group(backend="nccl")
    model = MyModel()
    # ... complete training implementation

client = TrainerClient()
job = client.train(
    trainer=CustomTrainer(
        func=my_training_function,
        num_nodes=4,
    )
)
```

**Use CustomTrainer when:**
- You need full control over the training process
- Implementing custom architectures or training strategies
- Experimenting with novel training techniques
- Building from scratch or adapting existing code

### BuiltinTrainer

With BuiltinTrainer, you use pre-built training scripts and configure them with parameters:

```python
from kubeflow.trainer import TrainerClient, BuiltinTrainer, TorchTuneConfig

client = TrainerClient()
job = client.train(
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            dataset_preprocess_config=TorchTuneInstructDataset(...),
            peft_config=LoraConfig(lora_rank=8),
            batch_size=16,
            epochs=10,
            # ... just configure parameters
        )
    ),
    runtime="torchtune-llama3.2-1b",
)
```

**Use BuiltinTrainer when:**
- Fine-tuning large language models with standard approaches
- Rapid experimentation with different hyperparameters
- Following established training patterns
- Minimizing code complexity
- Focusing on configuration over implementation

## Key Benefits

### Fast Iteration

BuiltinTrainer enables rapid experimentation without rewriting training logic:

```python
# Experiment 1: LoRA rank 8
client.train(
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            peft_config=LoraConfig(lora_rank=8),
            batch_size=16,
        )
    )
)

# Experiment 2: LoRA rank 16 with larger batch
client.train(
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            peft_config=LoraConfig(lora_rank=16),
            batch_size=32,
        )
    )
)
```

### Standardized Workflows

Pre-built trainers follow best practices and proven patterns:

- Optimized training loops
- Proper distributed training setup
- Efficient memory management
- Standard checkpointing and logging

### Configuration-Driven

Focus on what matters most - the hyperparameters and data:

```python
config = TorchTuneConfig(
    # Model configuration
    dtype=DataType.BF16,

    # Training hyperparameters
    batch_size=16,
    epochs=10,
    loss=Loss.CEWithChunkedOutputLoss,

    # Optimization
    peft_config=LoraConfig(
        lora_rank=8,
        lora_alpha=16,
        lora_dropout=0.1,
    ),

    # Dataset preprocessing
    dataset_preprocess_config=TorchTuneInstructDataset(
        source=DataFormat.PARQUET,
        split="train[:95%]",
        train_on_input=True,
        column_map={"input": "question", "output": "answer"},
    ),
)
```

### Model-Specific Optimizations

Each builtin trainer is optimized for specific model families:

- Pre-configured runtimes matched to model architectures
- Optimized memory layouts and attention mechanisms
- Model-specific hyperparameter defaults
- Validated configurations

## Available Builtin Trainers

### TorchTune

Fine-tune large language models using Meta's TorchTune framework.

**Supported models:**
- Llama 3.2 (1B, 3B)
- Llama 3.1 (8B, 70B)
- Qwen 2.5 (1.5B, 7B)
- Additional models via custom runtimes

**Key features:**
- LoRA (PEFT) fine-tuning support
- Instruction dataset preprocessing
- Mixed precision training
- Automatic checkpoint management

**Learn more:** [TorchTune Builtin Trainer Guide](torchtune)

## Choosing the Right Approach

Use this decision tree to choose between CustomTrainer and BuiltinTrainer:

```{mermaid}
graph TD
    A[What are you training?] --> B{LLM Fine-tuning?}
    B -->|Yes| C{Standard approach?}
    B -->|No| D[CustomTrainer]

    C -->|Yes, LoRA/QLoRA| E[BuiltinTrainer<br/>TorchTuneConfig]
    C -->|No, custom method| D

    D --> F{Need distributed?}
    F -->|Yes| G[CustomTrainer +<br/>PyTorch/DeepSpeed]
    F -->|No| H[CustomTrainer +<br/>Single node]

    E --> I{Model supported?}
    I -->|Yes| J[Use matching runtime<br/>torchtune-llama3.2-1b]
    I -->|No| K[Check available<br/>runtimes or use<br/>CustomTrainer]
```

**Quick recommendations:**

| Scenario | Recommended Approach |
|----------|---------------------|
| Fine-tune Llama 3.2 with LoRA | BuiltinTrainer (TorchTune) |
| Fine-tune Qwen 2.5 with LoRA | BuiltinTrainer (TorchTune) |
| Custom transformer architecture | CustomTrainer |
| Distributed PyTorch training | CustomTrainer |
| DeepSpeed ZeRO optimization | CustomTrainer |
| MLX on Apple Silicon | CustomTrainer |
| Standard LLM instruction tuning | BuiltinTrainer (TorchTune) |
| Novel training algorithm | CustomTrainer |
| Reinforcement learning | CustomTrainer |

## Complete Example Comparison

### With CustomTrainer

```python
def finetune_llama():
    """Custom fine-tuning implementation."""
    import torch
    from transformers import AutoModelForCausalLM, AutoTokenizer, Trainer
    from datasets import load_dataset
    from peft import LoraConfig, get_peft_model

    # Load model and tokenizer
    model = AutoModelForCausalLM.from_pretrained("meta-llama/Llama-3.2-1B")
    tokenizer = AutoTokenizer.from_pretrained("meta-llama/Llama-3.2-1B")

    # Apply LoRA
    lora_config = LoraConfig(r=8, lora_alpha=16)
    model = get_peft_model(model, lora_config)

    # Load and preprocess data
    dataset = load_dataset("tatsu-lab/alpaca")
    # ... tokenization and preprocessing

    # Training configuration
    trainer = Trainer(
        model=model,
        train_dataset=dataset,
        # ... trainer arguments
    )

    # Train
    trainer.train()

# Launch training
client.train(
    trainer=CustomTrainer(
        func=finetune_llama,
        num_nodes=1,
        resources_per_node={"gpu": 1},
    )
)
```

### With BuiltinTrainer

```python
from kubeflow.trainer import (
    TrainerClient,
    BuiltinTrainer,
    TorchTuneConfig,
    LoraConfig,
    TorchTuneInstructDataset,
    DataFormat,
    Initializer,
    HuggingFaceModelInitializer,
    HuggingFaceDatasetInitializer,
)

client = TrainerClient()

# Same fine-tuning task, configuration-driven
job = client.train(
    runtime="torchtune-llama3.2-1b",
    initializer=Initializer(
        model=HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
            access_token="<YOUR_HF_TOKEN>"
        ),
        dataset=HuggingFaceDatasetInitializer(
            storage_uri="hf://tatsu-lab/alpaca/data",
        )
    ),
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            peft_config=LoraConfig(
                lora_rank=8,
                lora_alpha=16,
            ),
            dataset_preprocess_config=TorchTuneInstructDataset(
                source=DataFormat.PARQUET,
            ),
            batch_size=16,
            epochs=10,
        )
    )
)
```

The BuiltinTrainer approach is more concise and focuses on configuration rather than implementation details.

## Working with Builtin Trainers

### 1. List Available Runtimes

Find available builtin trainer runtimes:

```python
from kubeflow.trainer import TrainerClient

client = TrainerClient()

# List all TorchTune runtimes
for runtime in client.list_runtimes():
    if runtime.name.startswith("torchtune"):
        print(f"Runtime: {runtime.name}")
        print(f"  Model: {runtime.labels.get('model', 'N/A')}")
        print(f"  Framework: {runtime.labels.get('framework', 'N/A')}")
```

### 2. Initialize Data and Models

Use initializers to prepare your training inputs:

```python
from kubeflow.trainer import (
    Initializer,
    HuggingFaceModelInitializer,
    HuggingFaceDatasetInitializer,
)

initializer = Initializer(
    # Model from HuggingFace Hub
    model=HuggingFaceModelInitializer(
        storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
        access_token="<YOUR_HF_TOKEN>"
    ),

    # Dataset from HuggingFace Hub
    dataset=HuggingFaceDatasetInitializer(
        storage_uri="hf://tatsu-lab/alpaca/data",
    )
)
```

### 3. Configure Training

Create trainer configuration:

```python
from kubeflow.trainer import (
    BuiltinTrainer,
    TorchTuneConfig,
    LoraConfig,
    TorchTuneInstructDataset,
    DataFormat,
    DataType,
)

trainer = BuiltinTrainer(
    config=TorchTuneConfig(
        # Data type
        dtype=DataType.BF16,

        # Training parameters
        batch_size=16,
        epochs=10,

        # LoRA configuration
        peft_config=LoraConfig(
            lora_rank=8,
            lora_alpha=16,
            lora_dropout=0.1,
        ),

        # Dataset preprocessing
        dataset_preprocess_config=TorchTuneInstructDataset(
            source=DataFormat.PARQUET,
            split="train[:95%]",
            train_on_input=True,
        ),

        # Resources
        resources_per_node={"gpu": 1},
    )
)
```

### 4. Launch Training

Submit the training job:

```python
job_id = client.train(
    runtime="torchtune-llama3.2-1b",
    initializer=initializer,
    trainer=trainer,
)

print(f"Training job: {job_id}")
```

### 5. Monitor Progress

Track training progress:

```python
from kubeflow.trainer.constants import constants

# Monitor initialization
print("Dataset initialization:")
for log in client.get_job_logs(job_id, step=constants.DATASET_INITIALIZER):
    print(log)

print("\nModel initialization:")
for log in client.get_job_logs(job_id, step=constants.MODEL_INITIALIZER):
    print(log)

# Monitor training
print("\nTraining logs:")
for log in client.get_job_logs(job_id, follow=True):
    print(log)

# Wait for completion
job = client.wait_for_job_status(job_id)
print(f"Training completed: {job.status}")
```

## Extending Builtin Trainers

### Custom Runtimes

Create custom runtimes for additional models (advanced):

```yaml
apiVersion: kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torchtune-custom-model
spec:
  runtimeSpec:
    # Custom runtime specification
    # See operator guide for details
```

### Custom Configurations

Extend existing configurations for specific use cases:

```python
# Advanced TorchTune configuration
config = TorchTuneConfig(
    dtype=DataType.BF16,
    batch_size=8,
    epochs=20,
    loss=Loss.CEWithChunkedOutputLoss,

    # Advanced LoRA
    peft_config=LoraConfig(
        lora_rank=16,
        lora_alpha=32,
        lora_dropout=0.05,
        target_modules=["q_proj", "v_proj", "k_proj", "o_proj"],
    ),

    # Custom dataset preprocessing
    dataset_preprocess_config=TorchTuneInstructDataset(
        source=DataFormat.PARQUET,
        split="train",
        train_on_input=False,
        new_system_prompt="You are a helpful AI assistant.",
        column_map={
            "input": "question",
            "output": "answer",
            "instruction": "system",
        },
    ),

    # Resource allocation
    resources_per_node={"gpu": 2, "cpu": 16, "memory": "64Gi"},
)
```

## Next Steps

- **[TorchTune Guide](torchtune)**: Learn how to fine-tune LLMs with TorchTune
- **[Getting Started](../../getting-started/index)**: Set up your environment
- **API Reference**: Complete SDK documentation (coming soon)
- **[Examples](https://github.com/kubeflow/trainer/tree/master/examples)**: Browse complete examples

## FAQ

**Q: Can I mix CustomTrainer and BuiltinTrainer?**

A: No, each training job uses either CustomTrainer or BuiltinTrainer, not both. However, you can run multiple jobs with different trainer types.

**Q: How do I add support for a new model?**

A: Currently, you need to work with the Kubeflow Trainer team to add new models. Custom runtime creation is an advanced topic covered in the operator guides.

**Q: Can I use BuiltinTrainer for multi-node training?**

A: Currently, BuiltinTrainer supports single-node training only. For multi-node LLM training, use CustomTrainer with DeepSpeed or PyTorch FSDP.

**Q: Where are trained models saved?**

A: BuiltinTrainer saves models to `/workspace/output` in a persistent volume claim shared across pods. You can access the PVC to retrieve your models.

**Q: Can I customize the training script?**

A: BuiltinTrainer uses predefined scripts that can't be modified directly. For custom logic, use CustomTrainer instead.

## Next Steps

- [TorchTune Guide](torchtune) - Fine-tune LLMs with TorchTune

```{toctree}
:hidden:
:maxdepth: 2

torchtune
```
