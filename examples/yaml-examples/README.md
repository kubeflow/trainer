# Kubeflow Trainer YAML Examples

This directory contains complete YAML examples for Kubeflow Trainer that work out-of-the-box with `kubectl`. 

## Available Examples

### 1. [PyTorch MNIST](./pytorch-mnist/)
- **Use Case**: Distributed PyTorch training with Fashion-MNIST dataset
- **Features**: 
  - Complete TrainingRuntime and TrainJob definitions
  - Progression tracking and checkpointing
  - Distributed training with torchrun
  - Configurable hyperparameters via environment variables

### 2. [TRL LLM Fine-tuning](./trl-llm-finetuning/)
- **Use Case**: Advanced LLM fine-tuning with LoRA using TRL (Transformers Reinforcement Learning)
- **Features**:
  - Multi-stage pipeline with dataset/model initializers
  - LoRA (Low-Rank Adaptation) configuration
  - Checkpoint resumption capabilities
  - Production-ready distributed training setup

### 3. [MPI Training](./mpi-training/)
- **Use Case**: MPI-based distributed training for high-performance computing environments
- **Features**:
  - Simple and advanced MPI training examples
  - Multi-node MPI distributed training
  - MPI backend integration with PyTorch
  - HPC-optimized training workflows

## Quick Start

1. **Validate examples** (optional):
   ```bash
   # Test all examples for YAML validity
   cd examples/yaml-examples/
   ./test-examples.sh
   ```

2. **Apply the examples directly**:
   ```bash
   # PyTorch MNIST example
   kubectl apply -f examples/yaml-examples/pytorch-mnist/mnist_training.yaml
   
   # TRL LLM fine-tuning example
   kubectl apply -f examples/yaml-examples/trl-llm-finetuning/trainjob.yaml
   
   # MPI training examples
   kubectl apply -f examples/yaml-examples/mpi-training/simple_mpi_training.yaml
   kubectl apply -f examples/yaml-examples/mpi-training/mpi_training.yaml
   ```

3. **Monitor the training**:
   ```bash
   # Check TrainJob status
   kubectl get trainjobs
   
   # View detailed status
   kubectl describe trainjob <job-name>
   
   # Follow training logs
   kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=trainer --follow
   ```

## Prerequisites

- Kubernetes cluster with Kubeflow Trainer V2 installed
- Storage class that supports `ReadWriteMany` (e.g., NFS)
- Sufficient compute resources (CPU/Memory, optionally GPU)

## Customization

Each example includes environment variables for easy customization:

- **Learning rate, batch size, epochs**
- **Model and dataset configuration** 
- **Resource requests and limits**
- **Checkpoint and progression tracking settings**

See individual example READMEs for detailed configuration options.

## Contributing

These examples were created in response to [GitHub Issue #2770](https://github.com/kubeflow/trainer/issues/2770) requesting YAML examples for TrainJob and ClusterTrainingRuntime CRDs.

To contribute additional examples:
1. Create a new directory under `examples/yaml-examples/`
2. Include complete YAML manifests with all required resources
3. Add comprehensive documentation
4. Test that examples work out-of-the-box with `kubectl apply`

## Additional Resources

- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)
- [TrainJob API Reference](https://github.com/kubeflow/trainer)
- [Jupyter Notebook Examples](../README.md)
