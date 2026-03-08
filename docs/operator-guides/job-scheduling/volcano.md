# Volcano Scheduler

Volcano provides advanced gang scheduling and resource management for Kubeflow Trainer, with support for queue-based scheduling and network topology awareness.

## Overview

Volcano is a Kubernetes-native batch scheduler that enables:
- **Gang scheduling** - Coordinated pod startup for distributed training
- **Queue management** - Priority-based job queues with resource quotas
- **Network topology awareness** - Optimize pod placement to reduce communication latency
- **Fair-share scheduling** - Balanced resource allocation across teams
- **Preemption** - Reclaim resources for higher-priority jobs

:::{tip}
Volcano is ideal for production environments with complex scheduling requirements, large-scale training, or high-performance networking like InfiniBand.
:::

## Prerequisites

Before enabling Volcano in Kubeflow Trainer, you must install Volcano in your Kubernetes cluster.

### Install Volcano

Follow the official [Volcano installation guide](https://volcano.sh/en/docs/installation/):

```bash
kubectl apply -f https://raw.githubusercontent.com/volcano-sh/volcano/master/installer/volcano-development.yaml
```

### Verify Installation

Check that Volcano components are running:

```bash
kubectl get pods -n volcano-system
```

Expected output:
```
NAME                                  READY   STATUS    RESTARTS   AGE
volcano-admission-xxxxx-yyyyy         1/1     Running   0          1m
volcano-controllers-xxxxx-yyyyy       1/1     Running   0          1m
volcano-scheduler-xxxxx-yyyyy         1/1     Running   0          1m
```

## Configuration

### Basic Gang Scheduling

Enable gang scheduling with Volcano:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-volcano
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command:
      - torchrun
      - train.py
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "1"
  podGroupPolicy:
    volcano: {}
```

This automatically generates a Volcano PodGroup resource for gang scheduling.

### Network Topology-Aware Scheduling

Optimize pod placement to reduce communication latency:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-topology
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "2"
  podGroupPolicy:
    volcano:
      networkTopology:
        mode: hard
        highestTierAllowed: 1
```

**Network topology modes:**
- `hard` - Strict enforcement, job fails if topology constraints cannot be met
- `soft` - Best-effort placement, job runs even if constraints cannot be met

**Tier levels:**
- `0` - Same host (for multi-GPU nodes)
- `1` - Same rack/switch (low latency)
- `2` - Same zone/datacenter
- `3` - Different zones

:::{note}
Network topology requires nodes to be labeled with topology information. See [Volcano Network Topology](https://volcano.sh/en/docs/network_topology/) for setup details.
:::

### Queue-Based Scheduling

Assign jobs to queues with priority and resource limits:

#### Create a Volcano Queue

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: high-priority-queue
spec:
  weight: 100
  capability:
    cpu: "400"
    memory: "1024Gi"
    nvidia.com/gpu: "32"
  reclaimable: true
```

#### Reference Queue in TrainJob

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-queued
  annotations:
    scheduling.volcano.sh/queue-name: "high-priority-queue"
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "2"
  podGroupPolicy:
    volcano: {}
```

#### Runtime-Level Queue Configuration

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-high-priority
  labels:
    trainer.kubeflow.org/framework: torch
  annotations:
    scheduling.volcano.sh/queue-name: "high-priority-queue"
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  podGroupPolicy:
    volcano: {}
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                metadata:
                  annotations:
                    scheduling.volcano.sh/queue-name: "high-priority-queue"
                spec:
                  containers:
                    - name: trainer
```

## Complete Examples

### Example 1: Large-Scale PyTorch Training

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-llm-training
  annotations:
    scheduling.volcano.sh/queue-name: "gpu-intensive"
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 32
    image: pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command:
      - torchrun
      - --nproc_per_node=8
      - train_llm.py
      - --model=llama-70b
      - --batch-size=4
    resourcesPerNode:
      requests:
        cpu: "64"
        memory: "512Gi"
      limits:
        cpu: "96"
        memory: "768Gi"
        nvidia.com/gpu: "8"
  podGroupPolicy:
    volcano:
      networkTopology:
        mode: hard
        highestTierAllowed: 1
```

### Example 2: Multiple Queues for Different Teams

**Team A Queue (GPU-focused):**

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: team-a-gpu
spec:
  weight: 80
  capability:
    nvidia.com/gpu: "64"
  reclaimable: true
```

**Team B Queue (CPU-focused):**

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: team-b-cpu
spec:
  weight: 50
  capability:
    cpu: "800"
    memory: "2048Gi"
  reclaimable: true
```

**TrainJob for Team A:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: team-a-training
  namespace: team-a
  annotations:
    scheduling.volcano.sh/queue-name: "team-a-gpu"
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "4"
  podGroupPolicy:
    volcano: {}
```

### Example 3: Priority-Based Scheduling

**High Priority Queue:**

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: urgent
spec:
  weight: 100
  capability:
    nvidia.com/gpu: "16"
  reclaimable: false  # Don't preempt jobs in this queue
```

**Normal Priority Queue:**

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: normal
spec:
  weight: 50
  capability:
    nvidia.com/gpu: "48"
  reclaimable: true  # Can be preempted for higher priority
```

## Volcano PodGroup

Kubeflow Trainer automatically creates Volcano PodGroups:

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: pytorch-volcano
  namespace: default
spec:
  minMember: 4
  queue: high-priority-queue
  priorityClassName: high-priority
```

### Check PodGroup Status

```bash
kubectl get podgroup.scheduling.volcano.sh
```

Output:
```
NAME              STATUS    RUNNING   MIN   AGE
pytorch-volcano   Running   4         4     2m
```

## Monitoring

### Check Queue Status

```bash
kubectl get queue
```

Output:
```
NAME                  WEIGHT   CAPABILITY                   STATUS   AGE
high-priority-queue   100      cpu:400,memory:1Ti,gpu:32   Open     10m
normal-queue          50       cpu:800,memory:2Ti,gpu:64   Open     10m
```

### Describe Queue

```bash
kubectl describe queue high-priority-queue
```

Output:
```
Name:         high-priority-queue
Namespace:
API Version:  scheduling.volcano.sh/v1beta1
Kind:         Queue
Spec:
  Capability:
    Cpu:               400
    Memory:            1024Gi
    Nvidia.com/gpu:    32
  Weight:              100
  Reclaimable:         true
Status:
  State:       Open
  Allocated:
    Cpu:               320
    Memory:            768Gi
    Nvidia.com/gpu:    24
  Pending:       0
Events:          <none>
```

### View Volcano Scheduler Logs

```bash
kubectl logs -n volcano-system -l app=volcano-scheduler
```

## Troubleshooting

### Pods Not Scheduling

**Check PodGroup status:**

```bash
kubectl describe podgroup.scheduling.volcano.sh <podgroup-name>
```

**Check queue capacity:**

```bash
kubectl get queue -o wide
```

**Common issues:**
- Queue capacity exceeded
- Topology constraints cannot be satisfied
- Insufficient cluster resources

### Queue Full

**Symptom:** PodGroup in "Pending" state, queue at capacity.

**Check queue allocation:**

```bash
kubectl describe queue <queue-name>
```

Look at `Status.Allocated` vs `Spec.Capability`.

**Solutions:**

1. **Increase queue capacity:**

```yaml
spec:
  capability:
    nvidia.com/gpu: "64"  # Increased from 32
```

2. **Wait for jobs to complete**
3. **Use a different queue**
4. **Enable preemption** for lower-priority jobs

### Network Topology Violations

**Symptom:** Jobs fail with topology errors.

**Check node labels:**

```bash
kubectl get nodes --show-labels | grep topology
```

Nodes should have labels like:
```
volcano.sh/network-topology-tier-0: node1
volcano.sh/network-topology-tier-1: rack1
volcano.sh/network-topology-tier-2: zone1
```

**Solution:** Label nodes with topology information:

```bash
kubectl label node node1 volcano.sh/network-topology-tier-0=node1
kubectl label node node1 volcano.sh/network-topology-tier-1=rack1
kubectl label node node1 volcano.sh/network-topology-tier-2=zone1
```

### Preemption Issues

**Symptom:** Lower-priority jobs keep getting preempted.

**Check queue configuration:**

```yaml
spec:
  reclaimable: false  # Disable preemption for this queue
```

Or set minimum guarantees:

```yaml
spec:
  guarantee:
    nvidia.com/gpu: "16"  # Always guarantee 16 GPUs
```

## Best Practices

### 1. Use Queues for Resource Management

```yaml
# Production jobs
scheduling.volcano.sh/queue-name: "production"

# Development jobs
scheduling.volcano.sh/queue-name: "development"

# Urgent/time-sensitive
scheduling.volcano.sh/queue-name: "urgent"
```

### 2. Enable Topology Awareness for Large Jobs

```yaml
# For jobs with 8+ nodes
podGroupPolicy:
  volcano:
    networkTopology:
      mode: hard
      highestTierAllowed: 1
```

### 3. Set Appropriate Queue Weights

```yaml
# High priority: weight 100
# Normal priority: weight 50
# Low priority: weight 25
```

### 4. Configure Resource Guarantees

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: team-queue
spec:
  weight: 50
  capability:
    nvidia.com/gpu: "32"
  guarantee:
    nvidia.com/gpu: "8"  # Always available
```

### 5. Monitor Queue Metrics

```yaml
# Prometheus metrics
- volcano_queue_allocated_gpu
- volcano_queue_capability_gpu
- volcano_podgroup_status
```

### 6. Use PriorityClasses

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: high-priority-training
value: 1000
globalDefault: false
description: "High priority training jobs"
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: urgent-training
spec:
  trainer:
    priorityClassName: high-priority-training
```

## Advanced Features

### Fair-Share Scheduling

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: shared-queue
spec:
  weight: 100
  capability:
    nvidia.com/gpu: "64"
  # Fair share across users
  allocatable:
    nvidia.com/gpu: "64"
```

### Resource Reservation

```yaml
spec:
  guarantee:
    nvidia.com/gpu: "16"  # Reserved resources
  capability:
    nvidia.com/gpu: "32"  # Maximum burst capacity
```

### Queue Hierarchies

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: parent-queue
spec:
  weight: 100
  capability:
    nvidia.com/gpu: "64"
---
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: child-queue-1
spec:
  parent: parent-queue
  weight: 60
  capability:
    nvidia.com/gpu: "32"
---
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: child-queue-2
spec:
  parent: parent-queue
  weight: 40
  capability:
    nvidia.com/gpu: "32"
```

## Performance Tuning

### Scheduler Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: volcano-scheduler-configmap
  namespace: volcano-system
data:
  volcano-scheduler.conf: |
    actions: "enqueue, allocate, backfill, preempt"
    tiers:
    - plugins:
      - name: priority
      - name: gang
      - name: conformance
    - plugins:
      - name: drf
      - name: predicates
      - name: proportion
      - name: nodeorder
      - name: binpack
```

### Batch Size Tuning

For large clusters, increase batch size:

```yaml
--scheduler-worker-threads=16
--scheduler-worker-queue-size=1000
```

## Migration from Coscheduling

**Before (Coscheduling):**

```yaml
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 300
```

**After (Volcano):**

```yaml
podGroupPolicy:
  volcano: {}
```

Or with topology:

```yaml
podGroupPolicy:
  volcano:
    networkTopology:
      mode: soft
      highestTierAllowed: 2
```

## Next Steps

- [Kueue Integration](kueue) - Alternative queue management system
- [Coscheduling](coscheduling) - Simpler gang scheduling option
- [Job Scheduling Overview](index) - Compare scheduling solutions
- [Volcano Documentation](https://volcano.sh/en/docs/) - Official Volcano docs
