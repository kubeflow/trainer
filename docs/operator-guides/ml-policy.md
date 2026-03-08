# ML Policies

ML Policies define machine learning-specific configuration for training jobs in Kubeflow Trainer runtimes. They specify how training workloads are distributed across nodes and processes, and configure framework-specific settings.

## Overview

The `MLPolicy` API provides:
- **Node configuration** - Number of training nodes (pods) to launch
- **Process distribution** - Processes per node for data parallelism
- **Framework settings** - PyTorch, MPI, or framework-agnostic configurations
- **Distributed setup** - Automatic environment configuration for distributed training

ML policies are defined in `TrainingRuntime` or `ClusterTrainingRuntime` and can be overridden in individual `TrainJob` specifications.

## Policy Types

Kubeflow Trainer supports three ML policy types:

### PlainML

Default policy for framework-agnostic training jobs.

**Features:**
- Simple multi-node execution
- No specialized distributed framework
- Environment variables for node coordination
- Standard Kubernetes Jobs

**Use cases:**
- Custom distributed training implementations
- Simple parallel workloads
- Non-standard frameworks

**Example:**

```yaml
mlPolicy:
  numNodes: 4
```

This creates 4 pods with training environment variables:
- `PET_NNODES=4`
- `PET_NODE_RANK=0,1,2,3` (per pod)

### Torch

Policy for PyTorch distributed training using `torchrun`.

**Features:**
- Automatic PyTorch distributed setup
- GPU detection and allocation
- NCCL backend for GPUs, Gloo for CPUs
- Process-per-GPU or custom process counts

**Use cases:**
- PyTorch Distributed Data Parallel (DDP)
- PyTorch Fully Sharded Data Parallel (FSDP)
- Multi-node PyTorch training

**Example:**

```yaml
mlPolicy:
  numNodes: 2
  torch:
    numProcPerNode: auto  # Or specific number
```

### MPI

Policy for MPI-based training using `mpirun`.

**Features:**
- MPI launcher and worker pods
- SSH-based communication
- OpenMPI or Intel MPI support
- Compatible with DeepSpeed, Horovod

**Use cases:**
- DeepSpeed training
- Horovod distributed training
- Custom MPI workloads
- Legacy MPI applications

**Example:**

```yaml
mlPolicy:
  numNodes: 4
  mpi:
    numProcPerNode: 2
    mpiImplementation: OpenMPI
    sshAuthMountPath: /home/mpiuser/.ssh
```

## Configuration Parameters

### Common Parameters

#### numNodes

Number of training nodes (pods) to launch.

**Type:** Integer
**Required:** Yes
**Minimum:** 1

**Example:**

```yaml
mlPolicy:
  numNodes: 8
```

**Usage in TrainJob:**

```yaml
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8  # Can override runtime default
```

### PyTorch-Specific Parameters

#### numProcPerNode

Number of processes to launch per node.

**Type:** String or Integer
**Options:**
- `"auto"` - Automatically detect GPU count
- `"gpu"` - Use number of GPUs available
- `"cpu"` - Use CPU count
- Integer - Specific number of processes

**Example:**

```yaml
mlPolicy:
  torch:
    numProcPerNode: auto  # Recommended
```

**With specific count:**

```yaml
mlPolicy:
  torch:
    numProcPerNode: 4  # Force 4 processes per node
```

#### elasticPolicy

Configure PyTorch Elastic training (not commonly used in Kubeflow Trainer).

**Example:**

```yaml
mlPolicy:
  torch:
    numProcPerNode: auto
    elasticPolicy:
      minNodes: 2
      maxNodes: 8
      maxRestarts: 3
```

### MPI-Specific Parameters

#### numProcPerNode

Number of MPI processes per worker node.

**Type:** Integer
**Required:** Yes

**Example:**

```yaml
mlPolicy:
  mpi:
    numProcPerNode: 4
```

#### mpiImplementation

MPI implementation to use.

**Type:** String
**Options:** `OpenMPI`, `Intel`
**Default:** `OpenMPI`

**Example:**

```yaml
mlPolicy:
  mpi:
    mpiImplementation: OpenMPI
```

#### sshAuthMountPath

Path where SSH authentication keys are mounted.

**Type:** String
**Default:** `/home/mpiuser/.ssh`

**Example:**

```yaml
mlPolicy:
  mpi:
    sshAuthMountPath: /root/.ssh
```

#### runLauncherAsNode

Whether the launcher pod counts as a training node.

**Type:** Boolean
**Default:** `true`

**Example:**

```yaml
mlPolicy:
  mpi:
    runLauncherAsNode: false  # Launcher is coordinator only
```

## Complete Examples

### PyTorch DDP Training

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-ddp
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 4
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
```

**TrainJob using this runtime:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-ddp-job
spec:
  runtimeRef:
    name: torch-ddp
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command:
      - torchrun
      - train.py
      - --epochs
      - "50"
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "2"
```

### DeepSpeed with MPI

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: deepspeed-mpi
  labels:
    trainer.kubeflow.org/framework: deepspeed
spec:
  mlPolicy:
    numNodes: 8
    mpi:
      numProcPerNode: 4
      mpiImplementation: OpenMPI
      sshAuthMountPath: /home/mpiuser/.ssh
      runLauncherAsNode: true
  template:
    spec:
      replicatedJobs:
        - name: launcher
          template:
            spec:
              template:
                metadata:
                  labels:
                    trainer.kubeflow.org/job-role: launcher
                spec:
                  containers:
                    - name: launcher
        - name: worker
          template:
            spec:
              template:
                metadata:
                  labels:
                    trainer.kubeflow.org/job-role: worker
                spec:
                  containers:
                    - name: worker
                      volumeMounts:
                        - name: ssh-auth
                          mountPath: /home/mpiuser/.ssh
                  volumes:
                    - name: ssh-auth
                      secret:
                        secretName: mpi-ssh-key
```

**TrainJob:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: deepspeed-training
spec:
  runtimeRef:
    name: deepspeed-mpi
  trainer:
    numNodes: 8
    image: deepspeed/deepspeed:latest
    command:
      - deepspeed
      - --hostfile
      - /etc/mpi/hostfile
      - train.py
      - --deepspeed_config
      - ds_config.json
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "4"
```

### PlainML for Custom Framework

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: custom-distributed
spec:
  mlPolicy:
    numNodes: 3
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
                        - name: MASTER_ADDR
                          valueFrom:
                            fieldRef:
                              fieldPath: metadata.annotations['trainer.kubeflow.org/master-addr']
```

**TrainJob:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: custom-training
spec:
  runtimeRef:
    name: custom-distributed
  trainer:
    numNodes: 3
    image: myregistry/custom-trainer:latest
    command:
      - python
      - train.py
```

## ML Policy Overrides

TrainJobs can override runtime ML policies:

### Override numNodes

```yaml
# Runtime default: 2 nodes
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed
spec:
  mlPolicy:
    numNodes: 2
    torch:
      numProcPerNode: auto
```

```yaml
# TrainJob override: 8 nodes
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: large-scale-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8  # Overrides runtime default
    image: pytorch/pytorch:2.5.1
```

### Override Process Count

```yaml
# Override PyTorch processes
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: multi-process-training
spec:
  runtimeRef:
    name: torch-distributed
  mlPolicy:
    torch:
      numProcPerNode: 4  # Override auto-detection
  trainer:
    numNodes: 2
    image: pytorch/pytorch:2.5.1
```

### Override MPI Settings

```yaml
# Override MPI configuration
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: mpi-custom
spec:
  runtimeRef:
    name: deepspeed-distributed
  mlPolicy:
    mpi:
      numProcPerNode: 8  # Override default
      mpiImplementation: Intel  # Change implementation
  trainer:
    numNodes: 4
    image: deepspeed/deepspeed:latest
```

## Environment Variables

ML policies automatically configure environment variables for distributed training:

### PyTorch Variables

When using `torch` policy, the following variables are set:

- `PET_NNODES` - Number of nodes
- `PET_NPROC_PER_NODE` - Processes per node
- `PET_NODE_RANK` - Current node rank (0-indexed)
- `MASTER_ADDR` - Address of rank 0 node
- `MASTER_PORT` - Port for rank 0 node (default: 29500)

### MPI Variables

When using `mpi` policy:

- `OMPI_MCA_*` - OpenMPI configuration parameters
- `I_MPI_*` - Intel MPI configuration parameters
- `PET_NNODES` - Number of worker nodes
- `PET_NODE_RANK` - Current node rank

### PlainML Variables

With `plainml` policy:

- `PET_NNODES` - Number of nodes
- `PET_NODE_RANK` - Current node rank

## Best Practices

### 1. Use Auto Process Detection for PyTorch

```yaml
mlPolicy:
  torch:
    numProcPerNode: auto  # Recommended
```

This automatically detects GPU count and optimizes resource usage.

### 2. Match MPI Processes to GPUs

```yaml
mlPolicy:
  mpi:
    numProcPerNode: 4  # Match GPU count
```

### 3. Set Reasonable Node Counts

```yaml
mlPolicy:
  numNodes: 1  # Default to single node
```

Allow users to scale up via TrainJob overrides.

### 4. Enable Debug Logging in Development

```yaml
spec:
  mlPolicy:
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
                          value: INFO  # Enable in dev
                        - name: TORCH_DISTRIBUTED_DEBUG
                          value: DETAIL
```

### 5. Configure Shared Memory for Multi-GPU

```yaml
spec:
  mlPolicy:
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
                  volumes:
                    - name: dshm
                      emptyDir:
                        medium: Memory
                        sizeLimit: 8Gi
                  containers:
                    - name: trainer
                      volumeMounts:
                        - name: dshm
                          mountPath: /dev/shm
```

### 6. Use Labels for Policy Variants

```yaml
metadata:
  name: torch-distributed-2gpu
  labels:
    trainer.kubeflow.org/framework: torch
    gpu.count: "2"
    policy.type: ddp
```

## Validation

ML policies are validated at runtime creation and TrainJob submission.

### Common Validation Errors

**Invalid numNodes:**
```
Error: spec.mlPolicy.numNodes must be greater than 0
```

**Solution:**
```yaml
mlPolicy:
  numNodes: 1  # Must be >= 1
```

**Missing numProcPerNode for MPI:**
```
Error: spec.mlPolicy.mpi.numProcPerNode is required
```

**Solution:**
```yaml
mlPolicy:
  mpi:
    numProcPerNode: 4  # Required for MPI policy
```

**Invalid MPI implementation:**
```
Error: spec.mlPolicy.mpi.mpiImplementation must be OpenMPI or Intel
```

**Solution:**
```yaml
mlPolicy:
  mpi:
    mpiImplementation: OpenMPI  # Valid option
```

## Troubleshooting

### Processes Not Starting

**Check logs:**
```bash
kubectl logs -l trainer.kubeflow.org/job-name=<job-name>
```

Look for torchrun or mpirun errors.

**Verify GPU allocation:**
```bash
kubectl exec <pod-name> -- nvidia-smi
```

### Wrong Number of Processes

**Check effective configuration:**
```bash
kubectl exec <pod-name> -- env | grep PET_NPROC_PER_NODE
```

**Verify GPU count:**
```bash
kubectl describe pod <pod-name> | grep nvidia.com/gpu
```

### MPI Communication Failures

**Check SSH connectivity:**
```bash
kubectl exec <launcher-pod> -- ssh <worker-pod-hostname> hostname
```

**Verify SSH keys:**
```bash
kubectl exec <worker-pod> -- ls -la /home/mpiuser/.ssh/
```

**Check MPI hostfile:**
```bash
kubectl exec <launcher-pod> -- cat /etc/mpi/hostfile
```

### PyTorch NCCL Errors

**Enable debug logging:**
```yaml
env:
  - name: NCCL_DEBUG
    value: INFO
  - name: NCCL_DEBUG_SUBSYS
    value: ALL
```

**Check network connectivity:**
```bash
kubectl exec <pod-name> -- nc -vz <other-pod-ip> 29500
```

## Advanced Patterns

### Heterogeneous Node Configurations

Use PlainML policy with custom environment variables:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: heterogeneous-cluster
spec:
  mlPolicy:
    numNodes: 4
  template:
    spec:
      replicatedJobs:
        - name: gpu-node
          replicas: 2
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      resources:
                        limits:
                          nvidia.com/gpu: "4"
        - name: cpu-node
          replicas: 2
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: trainer
                      resources:
                        requests:
                          cpu: "16"
```

### Dynamic Process Scaling

```yaml
mlPolicy:
  torch:
    numProcPerNode: auto
    elasticPolicy:
      minNodes: 1
      maxNodes: 16
      maxRestarts: 5
```

### Mixed Precision Training Policy

```yaml
spec:
  mlPolicy:
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
                        - name: PYTORCH_ENABLE_AMP
                          value: "1"
```

## Next Steps

- **Configure Runtimes** - See [Training Runtimes](runtime) for runtime definitions
- **Customize Templates** - See [Job Templates](job-template) for JobSet configurations
- **Schedule Jobs** - See [Job Scheduling](job-scheduling/index) for gang scheduling
- **Submit Training** - See [User Guides](../user-guides/index) for creating TrainJobs
