# Distributed Training Runtimes: FSDP and DeepSpeed

This guide describes how platform administrators can configure `TrainingRuntime` and `ClusterTrainingRuntime` resources for advanced distributed training paradigms: **PyTorch FSDP** (Fully Sharded Data Parallel) and **DeepSpeed**.

Before exploring this guide, make sure to read [the Runtime Guide](runtime) and [the ML Policy Guide](ml-policy).

## Overview

Kubeflow Trainer allows users to scale distributed AI workloads from single-GPU jobs to multi-node clusters. While standard Distributed Data Parallel (DDP) replicates model parameters across all GPUs, training large language models (LLMs) and foundation models requires memory-efficient sharding techniques.

Kubeflow Trainer supports two primary strategies for distributed model and optimizer sharding:

1. **PyTorch FSDP (Fully Sharded Data Parallel)**: Built directly into PyTorch (`torch.distributed.fsdp` and FSDP2). FSDP shards model parameters, gradients, and optimizer states across processes, reconstructing them on the fly during forward and backward passes. It is orchestrated natively using the **Torch plugin** (`torchrun`).
2. **DeepSpeed ZeRO (Zero Redundancy Optimizer)**: Developed by Microsoft to eliminate memory redundancies in data-parallel training across three progressive stages (ZeRO-1, ZeRO-2, and ZeRO-3). Kubeflow Trainer can run DeepSpeed workloads through either the **MPI plugin** (`mpirun`) or the **Torch plugin** (`torchrun`).

---

## Strategy Comparison Matrix

The following table summarizes the operational differences, hardware considerations, and trade-offs between distributed training runtimes:

| Strategy | Orchestration Plugin | Launcher | Configuration Mechanism | Sharding Granularity | Recommended Hardware Topology | Target Model Size | Key Trade-offs |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **PyTorch DDP** | Torch | `torchrun` | Python code / environment variables | Replicated (no sharding) | Ethernet or InfiniBand | < 1B parameters | High throughput; limited by single-GPU VRAM capacity. |
| **PyTorch FSDP (Full Shard)** | Torch | `torchrun` | Python code / environment variables / ConfigMap | Model parameters, gradients, optimizer states | High-bandwidth interconnect (InfiniBand / RoCE) | 1B to 70B+ parameters | Eliminates memory redundancy; introduces all-gather communication overhead. |
| **PyTorch FSDP (Hybrid Shard)** | Torch | `torchrun` | Python code / environment variables | Full sharding intra-node, replication inter-node | Dual-tier (NVLink intra-node, standard Ethernet inter-node) | 1B to 30B parameters | Minimizes inter-node communication at the cost of higher per-node memory usage. |
| **DeepSpeed ZeRO-2** | MPI or Torch | `mpirun` or `torchrun` | JSON configuration via ConfigMap or file | Optimizer states and gradients | Ethernet or InfiniBand | 1B to 13B parameters | Up to 8x memory reduction with minimal communication overhead; parameters remain unsharded. |
| **DeepSpeed ZeRO-3** | MPI or Torch | `mpirun` or `torchrun` | JSON configuration via ConfigMap or file | Optimizer states, gradients, and model parameters | High-bandwidth interconnect (InfiniBand / RoCE) | > 13B to 100B+ parameters | Maximum memory reduction; optional CPU/NVMe offload; higher communication latency. |

---

## PyTorch FSDP via Torch Plugin

PyTorch FSDP operates natively under the `torchrun` utility. Kubeflow Trainer's Torch plugin injects the necessary topology variables (`PET_NNODES`, `PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`, and `PET_MASTER_PORT`) to establish process groups across training nodes.

### TrainingRuntime Manifest for FSDP

The following `TrainingRuntime` defines an FSDP-ready template. It configures the Torch plugin, allocates high-performance shared memory (`/dev/shm`), and provides baseline environment flags for FSDP execution:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: pytorch-fsdp
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 2
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: pytorch/pytorch:2.15.0-cuda13.4-cudnn9-runtime
                      env:
                        # FSDP sharding configuration flags
                        - name: FSDP_SHARDING_STRATEGY
                          value: "FULL_SHARD"
                        - name: FSDP_BACKWARD_PREFETCH
                          value: "BACKWARD_PRE"
                        - name: FSDP_FORWARD_PREFETCH
                          value: "true"
                        - name: FSDP_LIMIT_ALL_GATHERS
                          value: "true"
                        - name: FSDP_CPU_OFFLOAD
                          value: "false"
                        - name: NCCL_DEBUG
                          value: "INFO"
                        - name: NCCL_IB_DISABLE
                          value: "0"
                      resources:
                        limits:
                          nvidia.com/gpu: 8
                        requests:
                          cpu: "16"
                          memory: "64Gi"
                          nvidia.com/gpu: 8
                      volumeMounts:
                        - mountPath: /dev/shm
                          name: dshm
                  volumes:
                    - name: dshm
                      emptyDir:
                        medium: Memory
                        sizeLimit: 16Gi
```

### Key Configuration Points for FSDP

- **`mlPolicy.torch.numProcPerNode: auto`**: Automatically sets the process count equal to the number of allocated GPUs on the node.
- **Shared Memory Volume (`/dev/shm`)**: Distributed PyTorch with NCCL relies on shared memory for fast intra-node inter-process communication. Mounting an `emptyDir` with `medium: Memory` avoids bus errors or out-of-memory crashes.
- **`FSDP_SHARDING_STRATEGY`**:
  - `FULL_SHARD`: Model parameters, gradients, and optimizer states are fully sharded across all GPUs. Best for maximum memory savings.
  - `SHARD_GRAD_OP`: Gradients and optimizer states are sharded, while parameters are retained after forward pass. Useful for throughput optimization when memory permits.
  - `HYBRID_SHARD`: Applies `FULL_SHARD` across GPUs within the same physical node (via NVLink), and replicates parameters across nodes. Ideal for clusters without high-speed inter-node fabrics.
- **`FSDP_BACKWARD_PREFETCH`**: Setting to `BACKWARD_PRE` overlaps communication with backward pass computation by prefetching next parameter shards before current computation ends.
- **`FSDP_LIMIT_ALL_GATHERS`**: Reduces peak memory usage by limiting concurrent in-flight all-gather operations.

### Consuming the FSDP Runtime in TrainJob

ML practitioners reference the runtime through `spec.runtimeRef`:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: llama3-fsdp-training
spec:
  runtimeRef:
    name: pytorch-fsdp
    kind: TrainingRuntime
  trainer:
    resourcesPerNode:
      requests:
        nvidia.com/gpu: 8
      limits:
        nvidia.com/gpu: 8
```

---

## DeepSpeed via Torch Plugin

While the default `deepspeed-distributed` runtime uses OpenMPI (`mpirun`), DeepSpeed can also run on top of `torchrun`. This approach avoids OpenSSH setup, hostfiles, and SSH secret management.

### DeepSpeed Configuration ConfigMap

Create a Kubernetes ConfigMap containing the `ds_config.json` file for ZeRO Stage 3 optimization:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: deepspeed-zero3-config
data:
  ds_config.json: |
    {
      "train_batch_size": 64,
      "train_micro_batch_size_per_gpu": 2,
      "gradient_accumulation_steps": 4,
      "bf16": {
        "enabled": true
      },
      "zero_optimization": {
        "stage": 3,
        "overlap_comm": true,
        "contiguous_gradients": true,
        "sub_group_size": 1e9,
        "reduce_bucket_size": "auto",
        "stage3_prefetch_bucket_size": "auto",
        "stage3_param_persistence_threshold": "auto",
        "stage3_max_live_parameters": 1e9,
        "stage3_max_reuse_distance": 1e9,
        "stage3_gather_16bit_weights_on_model_save": true
      },
      "optimizer": {
        "type": "AdamW",
        "params": {
          "lr": 2e-5,
          "betas": [0.9, 0.999],
          "eps": 1e-8,
          "weight_decay": 0.01
        }
      },
      "wall_clock_breakdown": false
    }
```

### TrainingRuntime Manifest for DeepSpeed with Torch Plugin

The following `TrainingRuntime` launches DeepSpeed via `torchrun` and mounts the configuration ConfigMap into `/etc/deepspeed`:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: deepspeed-torch
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 2
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: ghcr.io/kubeflow/trainer/deepspeed-runtime
                      env:
                        - name: DEEPSPEED_CONFIG_FILE
                          value: "/etc/deepspeed/ds_config.json"
                        - name: NCCL_DEBUG
                          value: "INFO"
                      volumeMounts:
                        - mountPath: /dev/shm
                          name: dshm
                        - mountPath: /etc/deepspeed
                          name: deepspeed-config
                          readOnly: true
                  volumes:
                    - name: dshm
                      emptyDir:
                        medium: Memory
                        sizeLimit: 16Gi
                    - name: deepspeed-config
                      configMap:
                        name: deepspeed-zero3-config
```

### Consuming DeepSpeed in TrainJob

The training script receives the path to the mounted configuration file via command-line flags:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: deepspeed-zero3-job
spec:
  runtimeRef:
    name: deepspeed-torch
    kind: TrainingRuntime
  trainer:
    args:
      - "--deepspeed"
      - "/etc/deepspeed/ds_config.json"
      - "--model_name_or_path"
      - "meta-llama/Llama-3.1-8B"
    resourcesPerNode:
      requests:
        nvidia.com/gpu: 4
      limits:
        nvidia.com/gpu: 4
```

---

## When to Choose MPI vs Torch Plugin for DeepSpeed

Kubeflow Trainer supports both launchers for DeepSpeed:

- **Choose the Torch Plugin (`torchrun`)** when:
  - You prefer a unified Kubernetes security posture without OpenSSH daemons or SSH host keys.
  - You are training Hugging Face models using `Trainer` or `Accelerate`, which natively recognize PyTorch distributed rendezvous endpoints.
  - Your infrastructure restricts inter-pod SSH connections.

- **Choose the MPI Plugin (`mpirun`)** when:
  - You have legacy DeepSpeed launch scripts configured for MPI hostfiles.
  - You utilize specialized MPI communication backends or InfiniBand fabrics tuned for OpenMPI.
  - You require centralized logging where output from all worker nodes streams to the launcher container (`node-0`).

---

## Related Guides

- [Runtime Guide](runtime) for foundational TrainingRuntime concepts.
- [ML Policy Guide](ml-policy) for `torchrun` and `mpirun` configuration options.
- [PyTorch User Guide](../user-guides/pytorch) for running PyTorch training jobs.
- [DeepSpeed User Guide](../user-guides/deepspeed) for DeepSpeed ZeRO concepts and Python SDK examples.
