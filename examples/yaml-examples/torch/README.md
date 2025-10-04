# PyTorch MNIST Distributed Training Example

This example demonstrates distributed PyTorch training using Fashion-MNIST dataset with Kubeflow Trainer. It includes comprehensive progression tracking, checkpointing, and resume capabilities.

## What's Included

- **PersistentVolumeClaim**: Shared storage for checkpoints and data
- **ConfigMap**: Complete training script with progression tracking
- **TrainingRuntime**: PyTorch distributed training configuration
- **TrainJob**: Job definition with hyperparameter configuration

## Quick Start

```bash
# Apply all resources
kubectl apply -f mnist_training.yaml

# Monitor the job
kubectl get trainjobs mnist-demo
kubectl describe trainjob mnist-demo

# Follow training logs
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=trainer --follow
```

## Configuration

### Training Hyperparameters
Modify these environment variables in the TrainJob spec:

```yaml
env:
  - name: LEARNING_RATE
    value: "0.01"        # Learning rate
  - name: BATCH_SIZE  
    value: "64"          # Batch size per device
  - name: NUM_EPOCHS
    value: "5"           # Number of training epochs
  - name: PROGRESSION_UPDATE_INTERVAL
    value: "10"          # Progress update frequency (seconds)
```

### Resource Configuration
Adjust compute resources in the TrainJob:

```yaml
trainer:
  numNodes: 1            # Number of training nodes
  resourcesPerNode:
    requests:
      cpu: "1"
      memory: "2Gi"
    limits:
      cpu: "2" 
      memory: "4Gi"
      # nvidia.com/gpu: 1  # Uncomment for GPU training
```

### Storage Configuration
Update the PVC storage class and size:

```yaml
spec:
  accessModes:
    - ReadWriteMany      # Required for multi-node training
  resources:
    requests:
      storage: 10Gi      # Adjust storage size as needed
  storageClassName: nfs-csi  # Update to your storage class
```

## Features

### Progression Tracking
- Real-time training progress updates
- ETA calculations and completion percentages
- Metrics tracking (loss, accuracy, learning rate)
- JSON status file at `/tmp/training_progression.json`

### Checkpointing
- Automatic checkpoint saving after each epoch
- Resume training from latest checkpoint
- Best model preservation
- Checkpoint metadata with timing information

### Distributed Training
- Multi-process training with torchrun
- Automatic distributed setup
- NCCL backend for GPU communication
- Graceful handling of single-node fallback

## Monitoring

### Check Job Status
```bash
# View all TrainJobs
kubectl get trainjobs

# Detailed job information
kubectl describe trainjob mnist-demo

# Check progression status
kubectl exec -it <trainer-pod> -- cat /tmp/training_progression.json
```

### View Logs
```bash
# Training logs from all ranks
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=trainer --follow

# Specific pod logs
kubectl logs <pod-name> --follow
```

### Access Checkpoints
```bash
# List checkpoints
kubectl exec -it <trainer-pod> -- ls -la /workspace/checkpoints/

# Copy checkpoint locally
kubectl cp <trainer-pod>:/workspace/checkpoints ./local-checkpoints
```

## Troubleshooting

### Common Issues

1. **Storage Class Not Found**
   - Update `storageClassName` in PVC to match your cluster
   - Ensure storage class supports `ReadWriteMany`

2. **Insufficient Resources**
   - Reduce `batch_size` or `memory` requests
   - Check node capacity with `kubectl describe nodes`

3. **Image Pull Issues**
   - Verify PyTorch image availability
   - Check image pull secrets if using private registry

4. **Training Stuck**
   - Check logs for NCCL/distributed training errors
   - Verify network connectivity between nodes
   - Consider disabling distributed features for debugging

### Debug Commands
```bash
# Check pod events
kubectl describe pod <trainer-pod>

# Check storage mount
kubectl exec -it <trainer-pod> -- df -h /workspace

# Test distributed setup
kubectl exec -it <trainer-pod> -- python -c "import torch; print(torch.cuda.is_available())"
```

## Expected Output

Successful training will show:
- Distributed training initialization across ranks
- Progressive loss reduction over epochs
- Checkpoint saving confirmations
- Final training completion with timing summary

Example log output:
```
[Rank 0/2] [Local 0] Starting distributed training
[Rank 0/2] Using GPU 0: Tesla V100-SXM2-16GB
[Rank 0/2] Starting training loop from epoch 1 to epoch 5
Train Epoch: 1 [0/60000 (0%)]	Loss: 2.302585
...
TRAINING COMPLETED SUCCESSFULLY!
Total Training Time: 5m23s
Epochs Completed: 5
Final Best Accuracy: 0.8945
```

## Related Examples

- [TRL LLM Fine-tuning](../trl-llm-finetuning/) - Advanced transformer fine-tuning
- [Jupyter Notebooks](../../pytorch/) - Interactive training examples
