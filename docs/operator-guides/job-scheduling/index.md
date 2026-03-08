# Job Scheduling

Advanced job scheduling enables efficient resource utilization and coordinated pod startup for distributed training workloads in Kubeflow Trainer.

## Overview

Distributed training jobs often require all training pods to start simultaneously to avoid wasting expensive GPU resources. Job scheduling in Kubeflow Trainer provides:

- **Gang scheduling** - All pods start together or none start
- **Queue management** - Priority-based job queuing and admission control
- **Resource quotas** - Team and namespace-based resource limits
- **Topology awareness** - Optimize pod placement for network performance

:::{important}
Gang scheduling is critical for multi-node training with expensive accelerators like GPUs. It ensures that a group of related training pods only start when all required resources are available, preventing partial job starts that waste resources.
:::

## Why Gang Scheduling?

Consider a 4-node training job where each pod requires 1 GPU:

**Without gang scheduling:**
- Pod 1 starts on node A (1 GPU allocated)
- Pod 2 starts on node B (1 GPU allocated)
- Pod 3 waits (no GPUs available)
- Pod 4 waits (no GPUs available)
- Result: 2 GPUs idle, training cannot proceed

**With gang scheduling:**
- All 4 pods wait until 4 GPUs are available
- All 4 pods start simultaneously
- Training begins immediately
- No wasted resources

## PodGroupPolicy Framework

Kubeflow Trainer implements scheduling through the `PodGroupPolicy` API, which creates PodGroups for gang scheduling support in TrainJob resources.

### Basic Structure

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: distributed-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
  podGroupPolicy:
    # Scheduler-specific configuration
```

## Supported Schedulers

Kubeflow Trainer integrates with three scheduling solutions:

### Coscheduling

**Purpose:** Basic gang scheduling

**Best for:**
- Simple multi-pod coordination
- Standard Kubernetes environments
- Getting started with gang scheduling

**Features:**
- Lightweight plugin for kube-scheduler
- Minimal additional components
- Easy to set up

**Documentation:** [Coscheduling Guide](coscheduling)

### Volcano

**Purpose:** Advanced scheduling and resource management

**Best for:**
- Complex scheduling policies
- Network topology awareness
- Production environments with diverse workloads

**Features:**
- Gang scheduling with advanced policies
- Queue-based job management
- Fair-share scheduling
- Network topology-aware placement
- Preemption support

**Documentation:** [Volcano Guide](volcano)

### Kueue

**Purpose:** Job queueing and quota management

**Best for:**
- Multi-tenant environments
- Resource quota enforcement
- Job prioritization and quotas
- Batch workload management

**Features:**
- Cluster queue management
- Resource flavor support
- Fair sharing across teams
- Quota management
- Workload prioritization

**Documentation:** [Kueue Guide](kueue)

## Comparison

| Feature | Coscheduling | Volcano | Kueue |
|---------|-------------|---------|-------|
| **Gang Scheduling** | Yes | Yes | Yes |
| **Queue Management** | No | Yes | Yes |
| **Resource Quotas** | No | Yes | Yes |
| **Topology Awareness** | No | Yes | No |
| **Preemption** | No | Yes | Yes |
| **Fair Sharing** | No | Yes | Yes |
| **Setup Complexity** | Low | Medium | Medium |
| **Resource Overhead** | Low | Medium | Medium |

## Choosing a Scheduler

### Use Coscheduling if:
- You need basic gang scheduling
- Simplicity is a priority
- You're getting started with distributed training
- You have a single-tenant cluster

### Use Volcano if:
- You need advanced scheduling policies
- Network topology matters (InfiniBand, RDMA)
- You run diverse workload types
- You need fine-grained scheduling control

### Use Kueue if:
- You have multiple teams sharing resources
- Resource quotas are critical
- You need job prioritization
- You want centralized queue management

## Configuration Examples

### Coscheduling

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-gang
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 300
```

### Volcano

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-volcano
  annotations:
    scheduling.volcano.sh/queue-name: "high-priority"
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
  podGroupPolicy:
    volcano:
      networkTopology:
        mode: hard
        highestTierAllowed: 1
```

### Kueue

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-kueue
  labels:
    kueue.x-k8s.io/queue-name: team-a-queue
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
```

## Runtime-Level Configuration

You can configure pod group policies at the runtime level:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-with-volcano
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  podGroupPolicy:
    volcano:
      networkTopology:
        mode: soft
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
```

TrainJobs using this runtime will inherit the pod group policy.

## Best Practices

### 1. Always Use Gang Scheduling for Multi-Node Training

```yaml
# Good: Gang scheduling enabled
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 300

# Avoid: No gang scheduling (pods may start partially)
# (omitting podGroupPolicy)
```

### 2. Set Appropriate Timeouts

```yaml
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 600  # 10 minutes for large jobs
```

Timeout should be long enough to accommodate cluster autoscaling.

### 3. Use Queues for Priority

```yaml
annotations:
  scheduling.volcano.sh/queue-name: "urgent-training"
```

### 4. Configure Resource Quotas

For multi-tenant environments, use Kueue with quotas:

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: a100-gpu
spec:
  nodeLabels:
    gpu.type: a100
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: team-a-cluster-queue
spec:
  namespaceSelector: {}
  resourceGroups:
    - coveredResources: ["cpu", "memory", "nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 16
```

### 5. Enable Topology Awareness for Large-Scale Training

```yaml
podGroupPolicy:
  volcano:
    networkTopology:
      mode: hard  # Strict topology placement
      highestTierAllowed: 1  # Same rack/switch
```

## Troubleshooting

### Pods Stuck in Pending

**Check PodGroup status:**

```bash
# For Coscheduling
kubectl get podgroup <job-name> -o yaml

# For Volcano
kubectl get podgroup.scheduling.volcano.sh <job-name> -o yaml
```

Look for scheduling events and conditions.

**Check scheduler logs:**

```bash
# Coscheduling
kubectl logs -n kube-system -l component=kube-scheduler

# Volcano
kubectl logs -n volcano-system -l app=volcano-scheduler
```

### Timeout Exceeded

Increase timeout or check resource availability:

```yaml
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 900  # Increase timeout
```

```bash
# Check available resources
kubectl describe nodes | grep -A 5 "Allocated resources"
```

### Queue Not Admitting Jobs

For Kueue, check queue status:

```bash
kubectl get clusterqueue
kubectl describe clusterqueue <queue-name>
```

Verify resource quotas and usage:

```bash
kubectl get resourceflavor
```

### Network Topology Violations

For Volcano, check pod placement:

```bash
kubectl get pods -o wide -l trainer.kubeflow.org/job-name=<job-name>
```

Verify nodes are in the same network tier:

```bash
kubectl get nodes --show-labels | grep topology
```

## Migration Between Schedulers

### From Default Scheduler to Coscheduling

1. Install coscheduling plugin
2. Add podGroupPolicy to TrainJobs:

```yaml
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 300
```

### From Coscheduling to Volcano

1. Install Volcano
2. Update podGroupPolicy:

```yaml
# Old
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 300

# New
podGroupPolicy:
  volcano:
    queue: default-queue
```

### From Volcano to Kueue

1. Install Kueue
2. Create cluster queues and resource flavors
3. Remove Volcano configuration:

```yaml
# Remove podGroupPolicy.volcano

# Add Kueue labels
metadata:
  labels:
    kueue.x-k8s.io/queue-name: team-queue
```

## Next Steps

- [Coscheduling Setup](coscheduling) - Configure basic gang scheduling
- [Volcano Integration](volcano) - Advanced scheduling and topology awareness
- [Kueue Configuration](kueue) - Queue management and resource quotas
- [Training Runtimes](../runtime) - Configure runtime-level scheduling policies

```{toctree}
:hidden:
:maxdepth: 1

coscheduling
volcano
kueue
```
