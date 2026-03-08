# Coscheduling

Coscheduling provides gang scheduling for Kubeflow Trainer, ensuring that all pods in a training job start together when resources are available.

## Overview

Coscheduling is a lightweight scheduler plugin for Kubernetes that implements gang scheduling. It ensures that a group of pods in the same training job start together only when all required resources are available, preventing partial job starts and resource waste.

**Key benefits:**
- **Resource efficiency** - No wasted GPU time from partial starts
- **Predictable scheduling** - All-or-nothing pod scheduling
- **Simple setup** - Minimal additional components
- **Low overhead** - Lightweight plugin architecture

## Prerequisites

Before enabling coscheduling in Kubeflow Trainer, you must install and activate the Coscheduling plugin in your Kubernetes cluster.

### Install Coscheduling Plugin

Follow the official [Kubernetes scheduler-plugins documentation](https://github.com/kubernetes-sigs/scheduler-plugins) to install the coscheduling plugin.

**Quick installation (example):**

```bash
# Install scheduler-plugins including coscheduling
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/scheduler-plugins/master/manifests/install/all-in-one.yaml
```

### Verify Installation

Check that the scheduler-plugins controller is running:

```bash
kubectl get pods -n scheduler-plugins-system
```

Expected output:
```
NAME                                           READY   STATUS    RESTARTS   AGE
scheduler-plugins-controller-xxxxx-yyyyy       1/1     Running   0          1m
scheduler-plugins-scheduler-xxxxx-yyyyy        1/1     Running   0          1m
```

## Configuration

### Basic Coscheduling

Enable gang scheduling with a simple configuration:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-coscheduled
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
    coscheduling:
      scheduleTimeoutSeconds: 300
```

### Configuration Parameters

#### scheduleTimeoutSeconds

Time in seconds to wait for all pods to be schedulable before timing out.

**Type:** Integer
**Default:** 60
**Recommended:** 300-600 for production

**Example:**

```yaml
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 600  # 10 minutes
```

:::{tip}
Set timeout high enough to account for cluster autoscaling. If your cluster uses autoscaling, allow time for new nodes to start up (typically 5-10 minutes).
:::

## How It Works

### PodGroup Creation

When a TrainJob with coscheduling policy is created, Kubeflow Trainer automatically creates a PodGroup resource:

```yaml
apiVersion: scheduling.sigs.k8s.io/v1alpha1
kind: PodGroup
metadata:
  name: pytorch-coscheduled
  namespace: default
spec:
  scheduleTimeoutSeconds: 300
  minMember: 4  # From trainer.numNodes
```

### Scheduling Process

1. **Job submission** - TrainJob created with podGroupPolicy.coscheduling
2. **PodGroup creation** - Controller creates PodGroup with minMember = numNodes
3. **Pod creation** - Pods are created with PodGroup annotation
4. **Waiting phase** - Pods wait in Pending until all can be scheduled
5. **Simultaneous scheduling** - All pods scheduled together when resources available
6. **Training starts** - All pods start simultaneously

```{mermaid}
sequenceDiagram
    participant User
    participant Controller
    participant PodGroup
    participant Scheduler
    participant Nodes

    User->>Controller: Create TrainJob
    Controller->>PodGroup: Create PodGroup (minMember=4)
    Controller->>Scheduler: Create 4 Pods
    Scheduler->>Scheduler: Check resource availability
    alt Resources available for all pods
        Scheduler->>Nodes: Schedule all 4 pods
        Nodes->>User: Training starts
    else Resources insufficient
        Scheduler->>Scheduler: Wait for resources
        Note over Scheduler: Pods remain Pending
    end
```

## Complete Examples

### Example 1: Multi-Node PyTorch Training

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-ddp
  namespace: ml-team
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8
    image: pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command:
      - torchrun
      - /workspace/train_resnet.py
      - --epochs
      - "100"
      - --batch-size
      - "256"
    resourcesPerNode:
      requests:
        cpu: "8"
        memory: "32Gi"
      limits:
        cpu: "16"
        memory: "64Gi"
        nvidia.com/gpu: "2"
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 600
```

### Example 2: DeepSpeed Training

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: deepspeed-training
spec:
  runtimeRef:
    name: deepspeed-distributed
  trainer:
    numNodes: 16
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
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 900  # 15 minutes for large job
```

### Example 3: Runtime-Level Configuration

Configure coscheduling at the runtime level:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-with-coscheduling
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 300
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

All TrainJobs using this runtime will automatically use coscheduling:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: auto-coscheduled
spec:
  runtimeRef:
    name: torch-with-coscheduling
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
  # No need to specify podGroupPolicy - inherited from runtime
```

## Monitoring

### Check PodGroup Status

```bash
kubectl get podgroup
```

Output:
```
NAME                   PHASE       MIN MEMBER   SCHEDULED   AGE
pytorch-coscheduled    Scheduled   4            4           2m
```

### Describe PodGroup

```bash
kubectl describe podgroup pytorch-coscheduled
```

Output:
```
Name:         pytorch-coscheduled
Namespace:    default
API Version:  scheduling.sigs.k8s.io/v1alpha1
Kind:         PodGroup
Spec:
  Min Member:                4
  Schedule Timeout Seconds:  300
Status:
  Phase:      Scheduled
  Scheduled:  4
  Running:    4
Events:
  Type    Reason     Age   From                 Message
  ----    ------     ----  ----                 -------
  Normal  Scheduled  2m    coscheduling-plugin  PodGroup is scheduled
```

### Check Pod Status

```bash
kubectl get pods -l trainer.kubeflow.org/job-name=pytorch-coscheduled
```

### View Events

```bash
kubectl get events --sort-by='.lastTimestamp' | grep coscheduling
```

## Troubleshooting

### Pods Stuck in Pending

**Symptom:** All pods remain in Pending state.

**Check available resources:**

```bash
kubectl describe nodes | grep -A 5 "Allocated resources"
```

**Check PodGroup events:**

```bash
kubectl describe podgroup <podgroup-name>
```

**Common causes:**
- Insufficient cluster resources
- Resource requests exceed node capacity
- Scheduling timeout too short

**Solution:**

```yaml
# Increase timeout
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 900

# Or reduce resource requests
trainer:
  resourcesPerNode:
    requests:
      cpu: "4"  # Reduced from 8
      memory: "16Gi"  # Reduced from 32Gi
```

### Timeout Exceeded

**Symptom:** PodGroup shows "Timeout" phase.

```bash
kubectl get podgroup
```

Output:
```
NAME                  PHASE     MIN MEMBER   SCHEDULED   AGE
pytorch-coscheduled   Timeout   4            0           6m
```

**Solution:**

1. **Check cluster capacity:**

```bash
kubectl describe nodes | grep "Allocatable" -A 5
```

2. **Enable cluster autoscaling** or increase timeout:

```yaml
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 1200  # 20 minutes
```

3. **Delete and recreate the job:**

```bash
kubectl delete trainjob pytorch-coscheduled
kubectl apply -f pytorch-coscheduled.yaml
```

### Partial Scheduling

**Symptom:** Some pods running, others pending.

This should NOT happen with coscheduling. If it does:

1. **Verify coscheduling is enabled:**

```bash
kubectl get pods -n scheduler-plugins-system
```

2. **Check pod annotations:**

```bash
kubectl get pod <pod-name> -o yaml | grep podgroup
```

Should show:
```yaml
annotations:
  scheduling.sigs.k8s.io/group-name: pytorch-coscheduled
```

3. **Check scheduler-plugins logs:**

```bash
kubectl logs -n scheduler-plugins-system -l component=scheduler
```

### PodGroup Not Created

**Symptom:** No PodGroup resource exists.

**Check TrainJob status:**

```bash
kubectl describe trainjob <job-name>
```

Look for events indicating PodGroup creation failures.

**Verify API is installed:**

```bash
kubectl api-resources | grep podgroup
```

Should show:
```
podgroups   scheduling.sigs.k8s.io/v1alpha1   true   PodGroup
```

## Best Practices

### 1. Set Appropriate Timeouts

```yaml
# Development/testing
scheduleTimeoutSeconds: 300  # 5 minutes

# Production with autoscaling
scheduleTimeoutSeconds: 900  # 15 minutes

# Large-scale jobs
scheduleTimeoutSeconds: 1800  # 30 minutes
```

### 2. Monitor PodGroup Status

Add monitoring alerts for PodGroup timeout:

```yaml
# Prometheus alert example
- alert: PodGroupTimeout
  expr: |
    kube_podgroup_status_phase{phase="Timeout"} > 0
  annotations:
    summary: "PodGroup {{ $labels.podgroup }} timed out"
```

### 3. Use with Cluster Autoscaler

Configure cluster autoscaler to respect PodGroups:

```yaml
# Cluster autoscaler configuration
--balance-similar-node-groups=true
--skip-nodes-with-system-pods=false
```

### 4. Set Resource Requests Accurately

```yaml
# Good: Accurate requests
resourcesPerNode:
  requests:
    cpu: "8"
    memory: "32Gi"
    nvidia.com/gpu: "1"

# Avoid: Over-requesting
resourcesPerNode:
  requests:
    cpu: "32"  # If job only needs 8
    memory: "128Gi"  # If job only needs 32Gi
```

### 5. Clean Up Failed Jobs

Implement automatic cleanup:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-training
spec:
  # ... other config ...
  ttlSecondsAfterFinished: 3600  # Clean up after 1 hour
```

## Performance Considerations

### Scheduler Overhead

Coscheduling has minimal overhead compared to default scheduler:
- **Memory:** ~50-100MB per scheduler-plugins pod
- **CPU:** <0.1 core under normal load
- **Latency:** <100ms additional scheduling time

### Scalability

Coscheduling scales well for typical training workloads:
- **Job count:** 100+ concurrent training jobs
- **Pod count:** 1000+ pods per job (tested)
- **Cluster size:** 1000+ nodes

## Migration Guide

### From Default Scheduler

**Before (no gang scheduling):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
```

**After (with coscheduling):**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 300
```

## Comparison with Other Schedulers

| Feature | Coscheduling | Volcano | Kueue |
|---------|-------------|---------|-------|
| Gang scheduling | ✅ | ✅ | ✅ |
| Queue management | ❌ | ✅ | ✅ |
| Priority scheduling | ❌ | ✅ | ✅ |
| Topology awareness | ❌ | ✅ | ❌ |
| Setup complexity | Low | Medium | Medium |
| Resource overhead | Low | Medium | Medium |

**When to use:**
- **Coscheduling:** Simple gang scheduling needs
- **Volcano:** Advanced scheduling + topology
- **Kueue:** Multi-tenant resource management

## Next Steps

- [Volcano Integration](volcano) - Advanced scheduling with topology awareness
- [Kueue Integration](kueue) - Queue management and resource quotas
- [Job Scheduling Overview](index) - Compare scheduling options
- [Training Runtimes](../runtime) - Configure runtime-level scheduling
