# DeepSpeed

Train and fine-tune large-scale AI models efficiently with DeepSpeed's ZeRO optimization on Kubernetes using Kubeflow Trainer.

## Overview

DeepSpeed is a deep learning optimization library that makes distributed training and inference easy, efficient, and effective. Kubeflow Trainer integrates DeepSpeed to enable scalable model training and fine-tuning on Kubernetes clusters with optimized memory usage and training throughput.

Key features:
- **ZeRO (Zero Redundancy Optimizer)**: Distributes optimizer states, gradients, and parameters across data-parallel processes
- **3D Parallelism**: Combines data, model, and pipeline parallelism for training massive models
- **Mixed Precision Training**: Supports FP16 and BF16 formats for faster training
- **Gradient Compression**: Reduces communication overhead in distributed training

## Prerequisites

Before following this guide, ensure you have:

- Completed the [Getting Started](../getting-started/index) guide
- Access to a Kubernetes cluster with Kubeflow Trainer installed
- Basic understanding of distributed training concepts
- Multiple GPUs for optimal DeepSpeed performance

## ZeRO Optimization Stages

DeepSpeed's ZeRO optimizer provides four stages of optimization, each with different memory efficiency and communication trade-offs:

### Stage 0: Disabled
Standard data parallel training without ZeRO optimizations. Each GPU maintains complete copies of model parameters, gradients, and optimizer states.

**Use when:**
- Your model fits comfortably in a single GPU
- You prioritize simplicity over memory efficiency

### Stage 1: Optimizer State Partitioning
Partitions optimizer states (e.g., Adam's momentum and variance) across data-parallel processes.

**Memory savings:** ~4x reduction in optimizer memory
**Use when:**
- Model fits in GPU memory but optimizer states are large
- Training medium-sized models with minimal communication overhead

### Stage 2: Gradient + Optimizer State Partitioning
Partitions both optimizer states and gradients across processes. Gradients are reduced and partitioned during backpropagation.

**Memory savings:** ~8x reduction in optimizer and gradient memory
**Use when:**
- Model parameters fit in GPU but gradients and optimizer states don't
- Training large models that exceed single-GPU capacity

### Stage 3: Parameter + Gradient + Optimizer State Partitioning
Partitions model parameters, gradients, and optimizer states across all processes. Parameters are gathered just-in-time during forward and backward passes.

**Memory savings:** Linear with number of GPUs (e.g., 64x with 64 GPUs)
**Use when:**
- Training very large models (billions of parameters)
- Model parameters don't fit in a single GPU even without gradients/optimizer
- You have sufficient inter-GPU bandwidth

:::{note}
Higher ZeRO stages provide better memory efficiency but increase communication overhead. Choose the lowest stage that fits your memory constraints.
:::

## The deepspeed-distributed Runtime

Kubeflow Trainer provides the `deepspeed-distributed` ClusterTrainingRuntime that uses MPI-based launchers to coordinate DeepSpeed training across multiple nodes.

### Automatic Environment Variables

The runtime automatically configures your distributed environment:

| Variable | Description | Access Method |
|----------|-------------|---------------|
| `WORLD_SIZE` | Total number of GPUs/processes | `dist.get_world_size()` |
| `RANK` | Global rank of current process | `dist.get_rank()` |
| `LOCAL_RANK` | Local rank within current node | `os.environ["LOCAL_RANK"]` |
| `MASTER_ADDR` | Address of rank 0 node | Automatically set |
| `MASTER_PORT` | Port for communication | Automatically set |

You don't need to manually configure these variables. DeepSpeed and PyTorch distributed will use them automatically.

## Training Function Pattern

To run DeepSpeed training with Kubeflow Trainer, follow this pattern:

### 1. Import Inside Function Body

All imports must be placed inside the function body:

```python
def train_with_deepspeed():
    """Train model with DeepSpeed ZeRO-2."""
    # All imports go here
    import torch
    import deepspeed
    from transformers import AutoModelForCausalLM, AutoTokenizer

    # Rest of your training code...
```

:::{note}
This requirement allows the SDK to serialize and transfer your training function to the cluster without dependency conflicts.
:::

### 2. Create DeepSpeed Configuration

Define your DeepSpeed configuration as a Python dictionary:

```python
def train_with_deepspeed():
    import deepspeed

    ds_config = {
        "train_micro_batch_size_per_gpu": 4,
        "gradient_accumulation_steps": 1,
        "optimizer": {
            "type": "AdamW",
            "params": {
                "lr": 3e-5,
                "betas": [0.9, 0.999],
                "eps": 1e-8,
                "weight_decay": 0.01
            }
        },
        "fp16": {
            "enabled": True,
            "loss_scale": 0,
            "initial_scale_power": 16,
            "loss_scale_window": 1000,
            "hysteresis": 2,
            "min_loss_scale": 1
        },
        "zero_optimization": {
            "stage": 2,
            "offload_optimizer": {
                "device": "cpu",
                "pin_memory": True
            },
            "allgather_partitions": True,
            "allgather_bucket_size": 2e8,
            "reduce_scatter": True,
            "reduce_bucket_size": 2e8,
            "overlap_comm": True,
            "contiguous_gradients": True
        }
    }
```

### 3. Initialize DeepSpeed Engine

Replace your standard optimizer and model initialization with DeepSpeed:

```python
def train_with_deepspeed():
    import torch
    import deepspeed
    from transformers import AutoModelForCausalLM

    # Load model
    model = AutoModelForCausalLM.from_pretrained("gpt2")

    # DeepSpeed configuration (from above)
    ds_config = {...}

    # Initialize DeepSpeed engine
    # DeepSpeed automatically handles distributed initialization
    model_engine, optimizer, _, _ = deepspeed.initialize(
        model=model,
        model_parameters=model.parameters(),
        config=ds_config
    )

    # Training loop using model_engine instead of model
    for epoch in range(num_epochs):
        for batch in train_loader:
            loss = model_engine(batch)
            model_engine.backward(loss)
            model_engine.step()
```

## Complete DeepSpeed Training Example

Here's a complete example fine-tuning a transformer model with DeepSpeed ZeRO-2:

```python
def train_gpt2_deepspeed():
    """Fine-tune GPT-2 with DeepSpeed ZeRO-2 optimization."""
    import os
    import torch
    import deepspeed
    from transformers import AutoModelForCausalLM, AutoTokenizer, default_data_collator
    from datasets import load_dataset
    from torch.utils.data import DataLoader

    # Get distributed rank
    local_rank = int(os.environ.get("LOCAL_RANK", 0))
    rank = int(os.environ.get("RANK", 0))
    world_size = int(os.environ.get("WORLD_SIZE", 1))

    # DeepSpeed configuration with ZeRO-2
    ds_config = {
        "train_micro_batch_size_per_gpu": 4,
        "gradient_accumulation_steps": 2,
        "optimizer": {
            "type": "AdamW",
            "params": {
                "lr": 3e-5,
                "betas": [0.9, 0.999],
                "eps": 1e-8,
                "weight_decay": 0.01
            }
        },
        "scheduler": {
            "type": "WarmupLR",
            "params": {
                "warmup_min_lr": 0,
                "warmup_max_lr": 3e-5,
                "warmup_num_steps": 100
            }
        },
        "fp16": {
            "enabled": True,
            "loss_scale": 0,
            "initial_scale_power": 16,
            "loss_scale_window": 1000,
            "hysteresis": 2,
            "min_loss_scale": 1
        },
        "zero_optimization": {
            "stage": 2,
            "offload_optimizer": {
                "device": "cpu",
                "pin_memory": True
            },
            "allgather_partitions": True,
            "allgather_bucket_size": 2e8,
            "reduce_scatter": True,
            "reduce_bucket_size": 2e8,
            "overlap_comm": True,
            "contiguous_gradients": True
        },
        "gradient_clipping": 1.0,
        "steps_per_print": 100,
        "wall_clock_breakdown": False
    }

    # Load model and tokenizer
    model_name = "gpt2"
    if rank == 0:
        print(f"Loading model: {model_name}")

    model = AutoModelForCausalLM.from_pretrained(model_name)
    tokenizer = AutoTokenizer.from_pretrained(model_name)
    tokenizer.pad_token = tokenizer.eos_token

    # Prepare dataset
    if rank == 0:
        print("Loading dataset...")

    dataset = load_dataset("wikitext", "wikitext-2-raw-v1", split="train")

    def tokenize_function(examples):
        return tokenizer(
            examples["text"],
            padding="max_length",
            truncation=True,
            max_length=512,
            return_tensors="pt"
        )

    tokenized_dataset = dataset.map(
        tokenize_function,
        batched=True,
        remove_columns=dataset.column_names
    )

    # Create dataloader
    train_loader = DataLoader(
        tokenized_dataset,
        batch_size=4,
        collate_fn=default_data_collator,
        shuffle=True
    )

    # Initialize DeepSpeed
    model_engine, optimizer, _, lr_scheduler = deepspeed.initialize(
        model=model,
        model_parameters=model.parameters(),
        config=ds_config
    )

    # Training loop
    num_epochs = 3
    if rank == 0:
        print(f"Starting training for {num_epochs} epochs...")

    model_engine.train()
    global_step = 0

    for epoch in range(num_epochs):
        epoch_loss = 0.0
        num_batches = 0

        for batch_idx, batch in enumerate(train_loader):
            # Move batch to device
            input_ids = batch["input_ids"].to(model_engine.local_rank)
            attention_mask = batch["attention_mask"].to(model_engine.local_rank)
            labels = input_ids.clone()

            # Forward pass
            outputs = model_engine(
                input_ids=input_ids,
                attention_mask=attention_mask,
                labels=labels
            )
            loss = outputs.loss

            # Backward pass
            model_engine.backward(loss)
            model_engine.step()

            epoch_loss += loss.item()
            num_batches += 1
            global_step += 1

            # Log progress
            if batch_idx % 100 == 0 and rank == 0:
                print(f"Epoch {epoch}, Step {global_step}, Batch {batch_idx}, Loss: {loss.item():.4f}")

        # Print epoch summary
        if rank == 0:
            avg_loss = epoch_loss / num_batches
            print(f"Epoch {epoch} completed. Average Loss: {avg_loss:.4f}")

    # Save model checkpoint (rank 0 only)
    if rank == 0:
        print("Saving model checkpoint...")
        model_engine.save_checkpoint("./checkpoints", tag="final")
        print("Training completed successfully!")
```

## Launching DeepSpeed Training with SDK

Use the TrainerClient SDK to launch your DeepSpeed training job:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()

job_id = client.train(
    trainer=CustomTrainer(
        func=train_gpt2_deepspeed,
        num_nodes=4,
        resources_per_node={
            "cpu": 8,
            "memory": "32Gi",
            "gpu": 2,  # 2 GPUs per node = 8 total GPUs
        },
        packages_to_install=["deepspeed", "transformers", "datasets"],
    ),
    runtime="deepspeed-distributed"
)

print(f"DeepSpeed training job created: {job_id}")

# Stream training logs
for log_line in client.get_job_logs(job_id, follow=True):
    print(log_line)
```

This launches a distributed DeepSpeed training job across 4 nodes with 2 GPUs each (8 GPUs total), automatically handling all distributed setup.

## Advanced DeepSpeed Configuration

### ZeRO-3 Configuration

For training very large models that don't fit in single GPU memory:

```python
ds_config = {
    "train_micro_batch_size_per_gpu": 1,
    "gradient_accumulation_steps": 4,
    "optimizer": {
        "type": "AdamW",
        "params": {"lr": 3e-5}
    },
    "bf16": {
        "enabled": True
    },
    "zero_optimization": {
        "stage": 3,
        "offload_optimizer": {
            "device": "cpu",
            "pin_memory": True
        },
        "offload_param": {
            "device": "cpu",
            "pin_memory": True
        },
        "overlap_comm": True,
        "contiguous_gradients": True,
        "sub_group_size": 1e9,
        "reduce_bucket_size": "auto",
        "stage3_prefetch_bucket_size": "auto",
        "stage3_param_persistence_threshold": "auto",
        "stage3_max_live_parameters": 1e9,
        "stage3_max_reuse_distance": 1e9,
        "stage3_gather_16bit_weights_on_model_save": True
    }
}
```

### Activation Checkpointing

Save memory by recomputing activations during backward pass:

```python
ds_config = {
    # ... other config ...
    "activation_checkpointing": {
        "partition_activations": True,
        "cpu_checkpointing": True,
        "contiguous_memory_optimization": True,
        "number_checkpoints": None,
        "synchronize_checkpoint_boundary": False,
        "profile": False
    }
}
```

### Communication Optimization

Optimize gradient communication patterns:

```python
ds_config = {
    # ... other config ...
    "zero_optimization": {
        "stage": 2,
        "allgather_partitions": True,
        "allgather_bucket_size": 5e8,
        "reduce_scatter": True,
        "reduce_bucket_size": 5e8,
        "overlap_comm": True,
        "contiguous_gradients": True
    },
    "communication_data_type": "fp16"  # or "bf16"
}
```

## Monitoring and Debugging

### Enable Detailed Logging

```python
ds_config = {
    # ... other config ...
    "steps_per_print": 10,
    "wall_clock_breakdown": True,
    "dump_state": True
}
```

### Monitor Memory Usage

```python
def train_with_monitoring():
    import deepspeed
    import torch

    # ... initialization code ...

    for batch in train_loader:
        loss = model_engine(batch)
        model_engine.backward(loss)
        model_engine.step()

        # Print memory stats periodically
        if model_engine.global_steps % 100 == 0:
            allocated = torch.cuda.memory_allocated() / 1024**3
            reserved = torch.cuda.memory_reserved() / 1024**3
            print(f"GPU Memory: {allocated:.2f}GB allocated, {reserved:.2f}GB reserved")
```

## Examples and Resources

### Complete Examples

- **[DeepSpeed T5 Fine-tuning](https://github.com/kubeflow/trainer/tree/master/examples/deepspeed)**: Complete notebook demonstrating T5 fine-tuning with DeepSpeed ZeRO-2
- **GPT-2 Training**: Large language model training with ZeRO-3 optimization

### API Documentation

- [TrainerClient API Reference](../api-reference/python-sdk/index): Complete SDK documentation
- [DeepSpeed Configuration Guide](https://www.deepspeed.ai/docs/config-json/): Official DeepSpeed configuration reference

### Related Guides

- [Getting Started](../getting-started/index): Initial setup and first training job
- [PyTorch Distributed](pytorch): Standard PyTorch distributed training
- [Local Execution](local-execution/index): Test DeepSpeed training locally

## Troubleshooting

### Common Issues

**DeepSpeed initialization fails:**

Ensure DeepSpeed is installed in your training environment:

```python
trainer = CustomTrainer(
    func=train_with_deepspeed,
    packages_to_install=["deepspeed==0.14.0"],
    # ... other params
)
```

**Out of memory errors:**

- Increase `gradient_accumulation_steps` to reduce memory per step
- Enable CPU offloading for optimizer and parameters
- Reduce `train_micro_batch_size_per_gpu`
- Use a higher ZeRO stage (2 → 3)

**Slow training speed:**

- Disable CPU offloading if you have sufficient GPU memory
- Increase bucket sizes for better communication efficiency
- Enable `overlap_comm` to overlap communication and computation
- Use `bf16` instead of `fp16` on supported hardware (A100+)

**NCCL timeout errors:**

Increase NCCL timeout for large models or slow networks:

```python
def train_with_deepspeed():
    import os
    os.environ["NCCL_TIMEOUT"] = "3600"  # 1 hour

    # ... rest of training code
```

**Checkpoint loading fails:**

Ensure you're loading checkpoints with the same ZeRO stage:

```python
# Save checkpoint
model_engine.save_checkpoint("./checkpoints")

# Load checkpoint later with same ZeRO stage config
_, client_state = model_engine.load_checkpoint("./checkpoints")
```
