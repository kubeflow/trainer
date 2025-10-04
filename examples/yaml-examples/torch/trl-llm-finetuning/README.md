# TRL LLM Fine-tuning with LoRA Example

This example demonstrates advanced Large Language Model (LLM) fine-tuning using TRL (Transformers Reinforcement Learning) with LoRA (Low-Rank Adaptation) on Kubeflow Trainer. It includes a complete multi-stage pipeline with dataset initialization, model preparation, and distributed training.

## What's Included

- **PersistentVolumeClaim**: Shared storage for models, datasets, and checkpoints
- **ConfigMap**: Advanced TRL training script with distributed coordination
- **TrainingRuntime**: Multi-stage pipeline with initializers and training
- **TrainJob**: Complete LLM fine-tuning configuration with LoRA

## Quick Start

```bash
# Apply all resources
kubectl apply -f trainjob.yaml

# Monitor the pipeline stages
kubectl get trainjobs trl-demo
kubectl describe trainjob trl-demo

# Follow training logs (after initializers complete)
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=trainer --follow
```

## Configuration

### Model Configuration
Configure the base model and LoRA parameters:

```yaml
env:
  - name: MODEL_NAME
    value: "gpt2"        # Base model from HuggingFace
  - name: LORA_R
    value: "16"          # LoRA rank (lower = fewer parameters)
  - name: LORA_ALPHA  
    value: "32"          # LoRA scaling parameter
  - name: LORA_DROPOUT
    value: "0.1"         # LoRA dropout rate
  - name: MAX_SEQ_LENGTH
    value: "512"         # Maximum sequence length
```

### Training Hyperparameters
Adjust training settings:

```yaml
env:
  - name: LEARNING_RATE
    value: "5e-5"        # Learning rate for fine-tuning
  - name: BATCH_SIZE
    value: "1"           # Batch size per device
  - name: MAX_EPOCHS
    value: "5"           # Number of training epochs
  - name: GRADIENT_ACCUMULATION_STEPS
    value: "2"           # Gradient accumulation
  - name: WARMUP_STEPS
    value: "5"           # Learning rate warmup steps
  - name: SAVE_STEPS
    value: "5"           # Checkpoint saving frequency
  - name: LOGGING_STEPS
    value: "2"           # Logging frequency
```

### Dataset Configuration
Customize the training dataset:

```yaml
env:
  - name: DATASET_NAME
    value: "tatsu-lab/alpaca"    # HuggingFace dataset
  - name: DATASET_TRAIN_SPLIT
    value: "train[:500]"         # Training split
  - name: DATASET_TEST_SPLIT  
    value: "train[500:520]"      # Evaluation split
```

### Resource Configuration
Scale compute resources:

```yaml
trainer:
  numNodes: 1            # Number of training nodes
  resourcesPerNode:
    requests:
      cpu: "2"
      memory: "4Gi"
    limits:
      cpu: "4"
      memory: "8Gi"
      # nvidia.com/gpu: 1  # Uncomment for GPU training
```

## Pipeline Stages

### 1. Dataset Initializer
- Downloads and preprocesses the specified dataset
- Caches data to shared storage for training access
- Supports HuggingFace datasets with custom splits

### 2. Model Initializer  
- Downloads the base model from HuggingFace
- Caches model files to shared storage
- Prepares model for LoRA fine-tuning

### 3. Training Node
- Loads cached dataset and model
- Applies LoRA configuration for parameter-efficient fine-tuning
- Performs distributed training with checkpoint resumption
- Saves fine-tuned model and adapters

## Monitoring

### Check Pipeline Progress
```bash
# View all pipeline stages
kubectl get jobs -l trainer.kubeflow.org/trainjob-name=trl-demo

# Check specific stage status
kubectl describe job <job-name>

# Monitor TrainJob overall status
kubectl get trainjobs trl-demo -o yaml
```

### View Stage Logs
```bash
# Dataset initializer logs
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=dataset-initializer

# Model initializer logs  
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=model-initializer

# Training logs
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=trainer --follow
```

### Access Training Artifacts
```bash
# List cached models and datasets
kubectl exec -it <trainer-pod> -- ls -la /workspace/cache/

# View checkpoints
kubectl exec -it <trainer-pod> -- ls -la /workspace/checkpoints/

# Copy fine-tuned model locally
kubectl cp <trainer-pod>:/workspace/checkpoints ./fine-tuned-model
```

## Advanced Features

### Checkpoint Resumption
- Automatic detection and loading of latest checkpoint
- Preserves training state, optimizer state, and timing information
- Seamless resume from interruptions or preemptions

### Distributed Training
- Multi-GPU and multi-node support with torchrun
- Automatic distributed process group initialization
- Gradient synchronization and model parallelism

### Progression Tracking
- Real-time training metrics and progress updates
- ETA calculations and completion percentages
- Integration with Kubeflow Trainer progression API

### Preemption Handling
- Graceful shutdown on SIGTERM/SIGINT signals
- Emergency checkpoint saving on preemption
- Status preservation for job rescheduling

## Troubleshooting

### Common Issues

1. **Initializer Failures**
   - Check network connectivity to HuggingFace Hub
   - Verify dataset/model names and availability
   - Ensure sufficient storage space

2. **Out of Memory Errors**
   - Reduce `BATCH_SIZE` or `MAX_SEQ_LENGTH`
   - Increase `GRADIENT_ACCUMULATION_STEPS`
   - Use smaller LoRA rank (`LORA_R`)

3. **Checkpoint Loading Issues**
   - Check checkpoint directory permissions
   - Verify checkpoint file integrity
   - Clear corrupted checkpoints if needed

4. **Distributed Training Problems**
   - Check network connectivity between nodes
   - Verify NCCL/GLOO backend configuration
   - Review torchrun environment variables

### Debug Commands
```bash
# Check storage usage
kubectl exec -it <trainer-pod> -- df -h /workspace

# Test model loading
kubectl exec -it <trainer-pod> -- python -c "from transformers import AutoModel; print(AutoModel.from_pretrained('/workspace/cache/models--gpt2'))"

# Verify dataset access
kubectl exec -it <trainer-pod> -- python -c "from datasets import load_from_disk; print(load_from_disk('/workspace/dataset'))"

# Check GPU availability
kubectl exec -it <trainer-pod> -- python -c "import torch; print(f'CUDA: {torch.cuda.is_available()}, GPUs: {torch.cuda.device_count()}')"
```

## Expected Output

Successful fine-tuning will show:

1. **Dataset Initialization**: Dataset download and preprocessing completion
2. **Model Initialization**: Base model download and caching
3. **Training Start**: LoRA configuration and distributed setup
4. **Training Progress**: Loss reduction and checkpoint saving
5. **Completion**: Final model saving and training summary

Example training log output:
```
[Rank 0/1] Starting TRL training
[Rank 0/1] Model: gpt2 (LoRA enabled)
[Rank 0/1] Dataset loaded - Train: 500, Eval: 20
[Rank 0/1] ===== STARTING NEW TRAINING =====
[Rank 0/1] Total epochs planned: 5
[Rank 0/1] STARTING new training - Epochs: 5, Initial LR: 0.000050
{'loss': 2.1234, 'learning_rate': 5e-05, 'epoch': 1.0}
{'loss': 1.8765, 'learning_rate': 4.5e-05, 'epoch': 2.0}
...
[Rank 0/1] Training completed
[Rank 0/1] Model saved
```

## Related Examples

- [PyTorch MNIST](../pytorch-mnist/) - Basic distributed training example
- [Jupyter Notebooks](../../torchtune/) - Interactive LLM fine-tuning examples
- [TRL Documentation](https://huggingface.co/docs/trl/) - TRL library reference
