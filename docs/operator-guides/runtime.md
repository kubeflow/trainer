# Training Runtimes

Training runtimes are template configurations managed by platform administrators that define how training jobs execute in Kubernetes. They serve as reusable blueprints that separate infrastructure concerns from training code.

## Overview

A runtime defines:
- **ML framework configuration** - PyTorch, DeepSpeed, JAX, MLX settings
- **Job structure** - How pods are organized and orchestrated
- **Resource templates** - Default resource allocations and constraints
- **Environment setup** - Container configurations, volume mounts, init containers

Runtimes enable platform teams to standardize training infrastructure while allowing data scientists to focus on model development.

## Runtime Types

Kubeflow Trainer provides two types of runtime resources:

### ClusterTrainingRuntime

Cluster-scoped runtimes available across all namespaces in the cluster.

**Use cases:**
- Organization-wide standard configurations
- Centralized control of training infrastructure
- Shared runtimes across multiple teams
- Enforcing company policies and best practices

**Example:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      env:
                        - name: NCCL_DEBUG
                          value: INFO
```

:::{tip}
Use `ClusterTrainingRuntime` for organization-wide standards that should be available to all teams.
:::

### TrainingRuntime

Namespace-scoped runtimes available only within a specific namespace.

**Use cases:**
- Team-specific customizations
- Experimental configurations
- Namespace-isolated environments
- Per-project runtime variations

**Example:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: custom-pytorch
  namespace: ml-team
  labels:
    trainer.kubeflow.org/framework: torch
    team: ml-team
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: gpu
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      resources:
                        requests:
                          cpu: "4"
                          memory: "16Gi"
```

:::{note}
When referencing a `TrainingRuntime` from a TrainJob, both resources must be in the same namespace.
:::

## Comparison

| Aspect | ClusterTrainingRuntime | TrainingRuntime |
|--------|------------------------|-----------------|
| **Scope** | Cluster-wide | Namespace-scoped |
| **Visibility** | All namespaces | Single namespace |
| **Use Case** | Standardized templates | Team customizations |
| **Management** | Centralized | Decentralized |
| **Access Control** | Cluster admin | Namespace admin |
| **Best For** | Production standards | Experimentation |

## Required Labels

Every runtime must include the framework label for SDK compatibility:

```yaml
metadata:
  labels:
    trainer.kubeflow.org/framework: <framework-name>
```

Valid framework values:
- `torch` - PyTorch distributed training
- `deepspeed` - DeepSpeed training
- `mlx` - MLX (Apple Silicon) training
- `jax` - JAX distributed training
- `torchtune` - TorchTune fine-tuning

This label enables the Kubeflow Python SDK to automatically discover and select appropriate runtimes for built-in trainers.

## Runtime Structure

A runtime consists of two main sections:

### 1. ML Policy

Defines ML-specific configuration:

```yaml
spec:
  mlPolicy:
    numNodes: 2  # Default number of nodes
    torch:       # Framework-specific settings
      numProcPerNode: auto
```

See [ML Policies](ml-policy) for detailed configuration options.

### 2. Template

Defines the JobSet template structure:

```yaml
spec:
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                metadata:
                  labels:
                    trainer.kubeflow.org/job-role: trainer
                spec:
                  containers:
                    - name: trainer
                      # Container configuration
```

## Built-in Runtimes

Kubeflow Trainer provides several pre-configured cluster runtimes:

### torch-distributed

PyTorch distributed training with torchrun:

```bash
kubectl get clustertrainingruntime torch-distributed -o yaml
```

**Features:**
- Automatic distributed setup
- GPU/CPU detection
- NCCL backend for GPUs
- Gloo backend for CPUs

**Usage:**

```yaml
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
```

### deepspeed-distributed

DeepSpeed training with MPI backend:

```bash
kubectl get clustertrainingruntime deepspeed-distributed -o yaml
```

**Features:**
- MPI-based communication
- SSH authentication
- ZeRO optimization support
- Multi-node coordination

**Usage:**

```yaml
spec:
  runtimeRef:
    name: deepspeed-distributed
  trainer:
    numNodes: 8
```

### mlx-distributed

MLX training for Apple Silicon:

```bash
kubectl get clustertrainingruntime mlx-distributed -o yaml
```

**Features:**
- Apple Silicon optimization
- Metal GPU support
- Unified memory architecture

**Usage:**

```yaml
spec:
  runtimeRef:
    name: mlx-distributed
  trainer:
    numNodes: 2
```

### jax-distributed

JAX distributed training:

```bash
kubectl get clustertrainingruntime jax-distributed -o yaml
```

**Features:**
- JAX distributed runtime
- TPU and GPU support
- XLA compilation

### torchtune-llama

TorchTune fine-tuning for Llama models:

```bash
kubectl get clustertrainingruntime torchtune-llama3.2-1b -o yaml
kubectl get clustertrainingruntime torchtune-llama3.2-3b -o yaml
```

**Features:**
- Pre-configured for Llama fine-tuning
- Memory-efficient training
- LoRA and QLoRA support

## Creating Custom Runtimes

### Basic Custom Runtime

Create a simple PyTorch runtime with custom settings:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: pytorch-custom
  labels:
    trainer.kubeflow.org/framework: torch
    company.com/gpu-type: a100
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                metadata:
                  labels:
                    trainer.kubeflow.org/job-role: trainer
                spec:
                  containers:
                    - name: trainer
                      env:
                        - name: NCCL_DEBUG
                          value: INFO
                        - name: NCCL_IB_DISABLE
                          value: "0"
                      volumeMounts:
                        - name: shm
                          mountPath: /dev/shm
                  volumes:
                    - name: shm
                      emptyDir:
                        medium: Memory
                        sizeLimit: 8Gi
```

### Runtime with Init Containers

Add initialization steps before training:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-with-init
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  initContainers:
                    - name: setup
                      image: busybox
                      command:
                        - sh
                        - -c
                        - |
                          echo "Preparing environment..."
                          mkdir -p /workspace/checkpoints
                      volumeMounts:
                        - name: workspace
                          mountPath: /workspace
                  containers:
                    - name: trainer
                      volumeMounts:
                        - name: workspace
                          mountPath: /workspace
                  volumes:
                    - name: workspace
                      emptyDir: {}
```

### Runtime with Resource Defaults

Set default resource requests and limits:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-gpu-a100
  labels:
    trainer.kubeflow.org/framework: torch
    gpu.type: a100
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: gpu
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      resources:
                        requests:
                          cpu: "8"
                          memory: "64Gi"
                          nvidia.com/gpu: "1"
                        limits:
                          cpu: "16"
                          memory: "128Gi"
                          nvidia.com/gpu: "1"
```

### Runtime with Node Affinity

Target specific node types:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-gpu-node
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  affinity:
                    nodeAffinity:
                      requiredDuringSchedulingIgnoredDuringExecution:
                        nodeSelectorTerms:
                          - matchExpressions:
                              - key: node.kubernetes.io/instance-type
                                operator: In
                                values:
                                  - p4d.24xlarge
                                  - p5.48xlarge
                  tolerations:
                    - key: nvidia.com/gpu
                      operator: Exists
                      effect: NoSchedule
                  containers:
                    - name: trainer
```

### Multi-Container Runtime

Include sidecar containers:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-with-sidecar
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      ports:
                        - containerPort: 29500
                          name: master
                    - name: metrics-exporter
                      image: prom/node-exporter:latest
                      ports:
                        - containerPort: 9100
                          name: metrics
```

## Referencing Runtimes from TrainJobs

### ClusterTrainingRuntime Reference

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-training-job
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    kind: ClusterTrainingRuntime
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
```

### TrainingRuntime Reference

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-training-job
  namespace: ml-team
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    kind: TrainingRuntime
    name: custom-pytorch
  trainer:
    numNodes: 2
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
```

:::{important}
The TrainJob and TrainingRuntime must be in the same namespace when using namespace-scoped runtimes.
:::

## Runtime Overrides

TrainJobs can override specific runtime settings:

### Override Resources

```yaml
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
    resourcesPerNode:  # Overrides runtime defaults
      requests:
        cpu: "8"
        memory: "32Gi"
      limits:
        nvidia.com/gpu: "2"
```

### Override ML Policy

```yaml
spec:
  runtimeRef:
    name: torch-distributed
  mlPolicy:  # Overrides runtime ML policy
    torch:
      numProcPerNode: 2  # Instead of auto
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
```

## Runtime Discovery

### List All Runtimes

```bash
# Cluster-scoped runtimes
kubectl get clustertrainingruntimes

# Namespace-scoped runtimes
kubectl get trainingruntimes -n <namespace>

# All runtimes across namespaces
kubectl get trainingruntimes -A
```

### Filter by Labels

```bash
# Find PyTorch runtimes
kubectl get clustertrainingruntimes -l trainer.kubeflow.org/framework=torch

# Find GPU-specific runtimes
kubectl get clustertrainingruntimes -l gpu.type=a100
```

### Using Python SDK

```python
from kubeflow.trainer import TrainerClient

client = TrainerClient()

# List all available runtimes
for runtime in client.list_runtimes():
    print(f"Runtime: {runtime.name}")
    print(f"  Framework: {runtime.framework}")
    print(f"  Scope: {runtime.scope}")
```

## Runtime Versioning

### Version Labeling

Label runtimes with version information:

```yaml
metadata:
  name: torch-distributed-v2
  labels:
    trainer.kubeflow.org/framework: torch
    version: "2.0"
    pytorch.version: "2.5.1"
    cuda.version: "12.4"
```

### Runtime Updates

When updating runtimes, create new versions instead of modifying existing ones:

```yaml
# Old version - keep for compatibility
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-v1
  labels:
    trainer.kubeflow.org/framework: torch
    version: "1.0"

---
# New version - add new features
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-v2
  labels:
    trainer.kubeflow.org/framework: torch
    version: "2.0"
```

This allows gradual migration of training jobs.

## Deprecation Policy

Deprecated runtimes are marked with a label:

```yaml
metadata:
  labels:
    trainer.kubeflow.org/support: "deprecated"
```

Deprecated runtimes are eligible for removal **starting from two minor releases** after deprecation.

**Example timeline:**
- Runtime deprecated in v2.1.0
- Can be removed starting from v2.3.0
- Users should migrate to newer runtimes

## Best Practices

### 1. Use Descriptive Names

```yaml
# Good
name: torch-distributed-a100-optimized

# Bad
name: runtime1
```

### 2. Add Comprehensive Labels

```yaml
labels:
  trainer.kubeflow.org/framework: torch
  gpu.type: a100
  network.type: infiniband
  environment: production
  team: ml-platform
  version: "2.0"
```

### 3. Document Runtime Purpose

```yaml
metadata:
  name: torch-distributed-production
  annotations:
    description: "Production PyTorch runtime with A100 optimizations"
    usage: "For large-scale distributed training on 8+ nodes"
    maintainer: "ml-platform@company.com"
```

### 4. Set Resource Defaults

Always provide reasonable defaults:

```yaml
spec:
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      resources:
                        requests:
                          cpu: "4"
                          memory: "16Gi"
```

### 5. Enable Observability

Include monitoring and logging:

```yaml
spec:
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      env:
                        - name: NCCL_DEBUG
                          value: INFO
                    - name: metrics-sidecar
                      image: prom/node-exporter
```

### 6. Use Shared Memory for Multi-GPU

```yaml
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
                    - name: trainer
                      volumeMounts:
                        - name: dshm
                          mountPath: /dev/shm
```

### 7. Implement Health Checks

```yaml
spec:
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      livenessProbe:
                        exec:
                          command:
                            - python
                            - -c
                            - "import torch; print(torch.cuda.is_available())"
                        initialDelaySeconds: 30
                        periodSeconds: 60
```

## Validation

Runtimes are validated on creation and update. Common validation errors:

### Missing Required Labels

```
Error: missing required label trainer.kubeflow.org/framework
```

**Solution:** Add the framework label:

```yaml
metadata:
  labels:
    trainer.kubeflow.org/framework: torch
```

### Invalid ML Policy

```
Error: numNodes must be >= 1
```

**Solution:** Set valid numNodes:

```yaml
spec:
  mlPolicy:
    numNodes: 1  # Must be positive
```

### Missing Container Name

```
Error: container name must be "trainer" for training pods
```

**Solution:** Use correct container name:

```yaml
containers:
  - name: trainer  # Required name
```

## Troubleshooting

### Runtime Not Found

```bash
# Check if runtime exists
kubectl get clustertrainingruntime <name>
kubectl get trainingruntime <name> -n <namespace>

# Check TrainJob events
kubectl describe trainjob <job-name>
```

### Runtime Validation Failed

View detailed error messages:

```bash
kubectl describe clustertrainingruntime <name>
```

Check the status conditions for validation errors.

### Jobs Not Using Updated Runtime

Existing TrainJobs continue using the runtime configuration from creation time. To use updated runtime:

1. Delete and recreate the TrainJob
2. Or create a new runtime version and update the TrainJob's `runtimeRef`

## Advanced Patterns

### Multi-Framework Runtime

Support multiple execution modes:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: flexible-runtime
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      env:
                        - name: TRAINING_FRAMEWORK
                          value: pytorch  # Can be changed per job
```

### Runtime with Persistent Storage

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-with-pvc
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      volumeMounts:
                        - name: training-data
                          mountPath: /data
                        - name: checkpoints
                          mountPath: /checkpoints
                  volumes:
                    - name: training-data
                      persistentVolumeClaim:
                        claimName: training-data-pvc
                    - name: checkpoints
                      persistentVolumeClaim:
                        claimName: checkpoints-pvc
```

## Next Steps

- **Configure ML Policies** - See [ML Policies](ml-policy) for framework-specific settings
- **Customize Job Templates** - See [Job Templates](job-template) for advanced JobSet configurations
- **Override Pod Templates** - See [Pod Templates](pod-template) for per-pod customizations
- **Set Up Scheduling** - See [Job Scheduling](job-scheduling/index) for gang scheduling integration
