# TorchTune

Fine-tune large language models with Meta's TorchTune framework using configuration-driven training on Kubernetes.

## Overview

TorchTune is Meta's PyTorch-native library for fine-tuning large language models. Kubeflow Trainer provides builtin trainer support for TorchTune, enabling you to fine-tune LLMs with simple configuration instead of writing custom training code.

Key features:
- **Configuration-driven**: Specify hyperparameters and datasets without writing training loops
- **LoRA/PEFT support**: Parameter-efficient fine-tuning for memory efficiency
- **Multiple model families**: Pre-configured runtimes for Llama, Qwen, and other models
- **Automated setup**: Model and dataset initialization handled automatically
- **Production-ready**: Optimized training configurations and checkpointing

:::{note}
Multi-node fine-tuning with TorchTune is not currently supported. However, LoRA (PEFT) fine-tuning is fully supported starting from Kubeflow Trainer V2.1.0 and SDK v0.2.0.
:::

## Prerequisites

Before following this guide, ensure you have:

- Completed the [Getting Started](../../getting-started/index) guide
- Access to a Kubernetes cluster with Kubeflow Trainer installed
- HuggingFace account and access token for model downloads
- Basic understanding of LLM fine-tuning concepts

## Supported Models

Kubeflow Trainer provides pre-configured runtimes for multiple model families. Each runtime is optimized for a specific model architecture.

### Available Runtimes

List available TorchTune runtimes:

```python
from kubeflow.trainer import TrainerClient

client = TrainerClient()

# Find TorchTune runtimes
for runtime in client.list_runtimes():
    if runtime.name.startswith("torchtune"):
        print(f"Runtime: {runtime.name}")
```

Common runtimes include:
- `torchtune-llama3.2-1b`: Llama 3.2 1B model
- `torchtune-llama3.2-3b`: Llama 3.2 3B model
- `torchtune-llama3.1-8b`: Llama 3.1 8B model
- `torchtune-qwen2.5-1.5b`: Qwen 2.5 1.5B model

See the [examples directory](https://github.com/kubeflow/trainer/tree/master/examples) for the complete list of supported models.

## Quick Start

Here's a minimal example to fine-tune Llama 3.2 1B on the Alpaca dataset:

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

job_id = client.train(
    runtime="torchtune-llama3.2-1b",

    # Initialize model and dataset
    initializer=Initializer(
        model=HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
            access_token="<YOUR_HF_TOKEN>"
        ),
        dataset=HuggingFaceDatasetInitializer(
            storage_uri="hf://tatsu-lab/alpaca/data",
        )
    ),

    # Configure training
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            batch_size=16,
            epochs=10,
            peft_config=LoraConfig(
                lora_rank=8,
                lora_alpha=16,
            ),
            dataset_preprocess_config=TorchTuneInstructDataset(
                source=DataFormat.PARQUET,
            ),
            resources_per_node={"gpu": 1},
        )
    )
)

print(f"Training job started: {job_id}")
```

## Configuration Components

### 1. Model Initialization

Specify where to load the pre-trained model from:

```python
from kubeflow.trainer import HuggingFaceModelInitializer, Initializer

# From HuggingFace Hub
model_init = HuggingFaceModelInitializer(
    storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
    access_token="<YOUR_HF_TOKEN>"
)

# Use in initializer
initializer = Initializer(model=model_init)
```

**Storage URI format:**
- `hf://<org>/<repo>` - Download entire repository
- `hf://<org>/<repo>/path/to/file` - Download specific file or directory

**Access token:**
- Required for gated models (Llama, Qwen, etc.)
- Get token from [HuggingFace Settings](https://huggingface.co/settings/tokens)

### 2. Dataset Initialization

Specify where to load training data from:

```python
from kubeflow.trainer import HuggingFaceDatasetInitializer

# From HuggingFace Hub
dataset_init = HuggingFaceDatasetInitializer(
    storage_uri="hf://tatsu-lab/alpaca/data",
    access_token="<YOUR_HF_TOKEN>"  # Optional for public datasets
)

# Use in initializer
initializer = Initializer(
    model=model_init,
    dataset=dataset_init
)
```

**Storage URI examples:**
- `hf://tatsu-lab/alpaca/data` - Specific directory in repository
- `hf://tatsu-lab/alpaca/data/train.parquet` - Specific file

The storage URI must specify the exact path to your data files (directories or individual files are supported).

### 3. TorchTune Configuration

Configure training hyperparameters and behavior:

```python
from kubeflow.trainer import (
    TorchTuneConfig,
    LoraConfig,
    TorchTuneInstructDataset,
    DataFormat,
    DataType,
    Loss,
)

config = TorchTuneConfig(
    # Data type for training
    dtype=DataType.BF16,  # or DataType.FP16, DataType.FP32

    # Training hyperparameters
    batch_size=16,
    epochs=10,

    # Loss function
    loss=Loss.CEWithChunkedOutputLoss,  # Memory-efficient cross-entropy

    # Number of nodes (always 1 for TorchTune)
    num_nodes=1,

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
        new_system_prompt="You are a helpful AI assistant.",
        column_map={"input": "question", "output": "answer"},
    ),

    # Resource allocation
    resources_per_node={"gpu": 1, "cpu": 8, "memory": "32Gi"},
)
```

### TorchTuneConfig Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `dtype` | DataType | Training precision (BF16/FP16/FP32) | BF16 |
| `batch_size` | int | Batch size per GPU | 8 |
| `epochs` | int | Number of training epochs | 1 |
| `loss` | Loss | Loss function | CEWithChunkedOutputLoss |
| `num_nodes` | int | Number of nodes (must be 1) | 1 |
| `peft_config` | LoraConfig | LoRA configuration | None |
| `dataset_preprocess_config` | TorchTuneInstructDataset | Dataset preprocessing | Required |
| `resources_per_node` | dict | Resource requests | `{"gpu": 1}` |

### LoRA Configuration

Configure parameter-efficient fine-tuning:

```python
from kubeflow.trainer import LoraConfig

lora = LoraConfig(
    lora_rank=8,          # Rank of LoRA matrices (4, 8, 16, 32)
    lora_alpha=16,        # LoRA scaling factor (typically 2x rank)
    lora_dropout=0.1,     # Dropout for LoRA layers
)
```

**LoRA parameter guidelines:**

| Model Size | Recommended Rank | Recommended Alpha | Memory Usage |
|------------|------------------|-------------------|--------------|
| 1B - 3B | 8 | 16 | Low |
| 7B - 13B | 16 | 32 | Medium |
| 30B+ | 32 | 64 | Higher |

Higher rank = more parameters to train = better quality but more memory.

### Dataset Preprocessing Configuration

Configure how TorchTune processes your dataset:

```python
from kubeflow.trainer import TorchTuneInstructDataset, DataFormat

dataset_config = TorchTuneInstructDataset(
    # Data format
    source=DataFormat.PARQUET,  # or DataFormat.JSON

    # Dataset split
    split="train[:95%]",  # Use first 95% for training

    # Training behavior
    train_on_input=True,  # Include input in loss calculation

    # System prompt
    new_system_prompt="You are a helpful AI assistant specialized in Python.",

    # Column mapping
    column_map={
        "input": "question",      # Map 'question' column to 'input'
        "output": "answer",       # Map 'answer' column to 'output'
        "instruction": "system",  # Map 'system' column to 'instruction'
    },
)
```

**Dataset format requirements:**

Your dataset should contain columns for:
- `input` or `instruction`: The prompt or instruction
- `output` or `response`: The expected completion

If your columns have different names, use `column_map` to map them.

## Complete Training Example

Here's a complete example fine-tuning Llama 3.2 1B with custom configuration:

```python
from kubeflow.trainer import (
    TrainerClient,
    BuiltinTrainer,
    TorchTuneConfig,
    LoraConfig,
    TorchTuneInstructDataset,
    DataFormat,
    DataType,
    Loss,
    Initializer,
    HuggingFaceModelInitializer,
    HuggingFaceDatasetInitializer,
)
from kubeflow.trainer.constants import constants

# Initialize client
client = TrainerClient()

# Configure model initialization
model_init = HuggingFaceModelInitializer(
    storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
    access_token="<YOUR_HF_TOKEN>"
)

# Configure dataset initialization
dataset_init = HuggingFaceDatasetInitializer(
    storage_uri="hf://tatsu-lab/alpaca/data",
)

# Configure LoRA
lora_config = LoraConfig(
    lora_rank=8,
    lora_alpha=16,
    lora_dropout=0.1,
)

# Configure dataset preprocessing
dataset_config = TorchTuneInstructDataset(
    source=DataFormat.PARQUET,
    split="train[:95%]",
    train_on_input=True,
    new_system_prompt="You are a helpful AI assistant.",
    column_map={
        "input": "instruction",
        "output": "output",
    },
)

# Configure training
trainer_config = TorchTuneConfig(
    dtype=DataType.BF16,
    batch_size=10,
    epochs=10,
    loss=Loss.CEWithChunkedOutputLoss,
    num_nodes=1,
    peft_config=lora_config,
    dataset_preprocess_config=dataset_config,
    resources_per_node={
        "gpu": 1,
        "cpu": 8,
        "memory": "32Gi",
    },
)

# Launch training
job_id = client.train(
    runtime="torchtune-llama3.2-1b",
    initializer=Initializer(
        model=model_init,
        dataset=dataset_init,
    ),
    trainer=BuiltinTrainer(config=trainer_config)
)

print(f"Training job started: {job_id}")

# Monitor initialization
print("\nDataset initialization logs:")
for log in client.get_job_logs(job_id, step=constants.DATASET_INITIALIZER):
    print(log)

print("\nModel initialization logs:")
for log in client.get_job_logs(job_id, step=constants.MODEL_INITIALIZER):
    print(log)

# Monitor training
print("\nTraining logs:")
for log in client.get_job_logs(job_id, follow=True):
    print(log)

# Wait for completion
final_job = client.wait_for_job_status(job_id)
print(f"\nTraining completed with status: {final_job.status}")
```

## Monitoring Training

### View Initialization Logs

Monitor dataset and model initialization:

```python
from kubeflow.trainer.constants import constants

# Dataset initialization
print("Dataset initialization:")
for log in client.get_job_logs(job_id, step=constants.DATASET_INITIALIZER):
    print(log)

# Model initialization
print("\nModel initialization:")
for log in client.get_job_logs(job_id, step=constants.MODEL_INITIALIZER):
    print(log)
```

### View Training Logs

Stream training progress in real-time:

```python
# Follow training logs
for log in client.get_job_logs(job_id, follow=True):
    print(log)

# Or get all logs at once
logs = client.get_job_logs(job_id)
for log in logs:
    print(log)
```

### Check Job Status

Monitor job progress:

```python
# Get current status
job = client.get_job(name=job_id)
print(f"Status: {job.status}")
print(f"Steps:")
for step in job.steps:
    print(f"  {step.name}: {step.status}")

# Wait for completion
final_job = client.wait_for_job_status(
    job_id,
    timeout=3600  # Wait up to 1 hour
)
print(f"Final status: {final_job.status}")
```

## Accessing Fine-tuned Models

After training completes, your fine-tuned model is saved to `/workspace/output` in a persistent volume claim.

### Retrieve Model Weights

The PVC is shared across all pods in the TrainJob. To access your model:

1. **Find the PVC name:**

```bash
kubectl get pvc -l trainjob-name=<job-name>
```

2. **Create a pod to access the PVC:**

```bash
kubectl run access-model --rm -it --image=python:3.11 \
  --overrides='
{
  "spec": {
    "containers": [{
      "name": "access-model",
      "image": "python:3.11",
      "command": ["/bin/bash"],
      "volumeMounts": [{
        "mountPath": "/workspace",
        "name": "workspace"
      }]
    }],
    "volumes": [{
      "name": "workspace",
      "persistentVolumeClaim": {
        "claimName": "<pvc-name>"
      }
    }]
  }
}'

# Inside the pod
ls /workspace/output
# Copy model files or upload to storage
```

3. **Download model files:**

You can copy files out using `kubectl cp`:

```bash
kubectl cp access-model:/workspace/output ./local-output
```

## Advanced Configuration

### Multiple Datasets

Train on multiple datasets by concatenating them:

```python
# This requires preprocessing the datasets first
# Combine datasets before passing to HuggingFaceDatasetInitializer
```

### Custom System Prompts

Customize the system prompt for instruction tuning:

```python
dataset_config = TorchTuneInstructDataset(
    source=DataFormat.PARQUET,
    new_system_prompt="You are a Python expert. Provide concise, accurate answers.",
)
```

### Validation Split

Use part of your data for validation:

```python
dataset_config = TorchTuneInstructDataset(
    source=DataFormat.PARQUET,
    split="train[:80%]",  # Use 80% for training
)

# Validation requires custom implementation
```

### Higher Precision Training

Use FP32 for higher precision (slower, more memory):

```python
config = TorchTuneConfig(
    dtype=DataType.FP32,
    batch_size=4,  # Reduce batch size for FP32
    # ... other config
)
```

### Resource Optimization

Optimize GPU memory usage:

```python
config = TorchTuneConfig(
    dtype=DataType.BF16,  # Use BF16 for memory efficiency
    batch_size=8,         # Reduce batch size if OOM
    peft_config=LoraConfig(
        lora_rank=4,      # Lower rank = less memory
        lora_alpha=8,
    ),
    resources_per_node={
        "gpu": 1,
        "memory": "24Gi",  # Adjust based on GPU VRAM
    },
)
```

## Examples and Resources

### Complete Examples

- **[Llama 3.2 1B Fine-tuning](https://github.com/kubeflow/trainer/tree/master/examples/builtin-trainer/torchtune-llama3.2-1b)**: Complete notebook with Alpaca dataset
- **[Qwen 2.5 Fine-tuning](https://github.com/kubeflow/trainer/tree/master/examples/builtin-trainer/torchtune-qwen2.5-1.5b)**: Qwen model fine-tuning example

### API Documentation

- [TrainerClient API Reference](../../api-reference/python-sdk/index): Complete SDK documentation
- [TorchTune Documentation](https://pytorch.org/torchtune/): Official TorchTune documentation

### Related Guides

- [Builtin Trainer Overview](index): Understanding builtin trainers
- [Getting Started](../../getting-started/index): Initial setup
- [Custom Trainer](../../getting-started/index): Writing custom training functions

## Troubleshooting

### Common Issues

**HuggingFace authentication fails:**

Ensure your access token has the correct permissions:

```python
# Verify token works
from huggingface_hub import login
login(token="<YOUR_HF_TOKEN>")
```

**Out of memory errors:**

Reduce memory usage:

- Decrease batch size: `batch_size=4`
- Use lower LoRA rank: `lora_rank=4`
- Use BF16: `dtype=DataType.BF16`
- Reduce model size (use smaller model variant)

**Dataset column mapping errors:**

Verify your dataset columns match the configuration:

```python
# Check dataset structure
from datasets import load_dataset
ds = load_dataset("tatsu-lab/alpaca")
print(ds["train"].column_names)

# Update column_map accordingly
dataset_config = TorchTuneInstructDataset(
    column_map={"input": "actual_column_name"},
)
```

**Model download fails:**

For gated models (Llama, Qwen):
1. Request access on HuggingFace
2. Wait for approval
3. Use access token with read permissions

**Training logs show no progress:**

Check initialization logs first:

```python
# Dataset init logs
logs = client.get_job_logs(job_id, step=constants.DATASET_INITIALIZER)
print("\n".join(logs))

# Model init logs
logs = client.get_job_logs(job_id, step=constants.MODEL_INITIALIZER)
print("\n".join(logs))
```

**Job stays in pending state:**

Check resource availability:

```bash
# Check if GPU nodes are available
kubectl get nodes -l nvidia.com/gpu.present=true

# Check resource requests
kubectl describe trainjob <job-name>
```

## Limitations

Current limitations of TorchTune builtin trainer:

- **Single-node only**: Multi-node training not supported
- **LoRA only**: Full fine-tuning not supported
- **Limited customization**: Can't modify training script
- **Predefined models**: Only models with pre-configured runtimes

For advanced use cases requiring customization, use [CustomTrainer](../../getting-started/index) instead.
