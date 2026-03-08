# Kueue Integration

Kueue provides job queueing and resource management for Kubeflow Trainer, with advanced quota management and multi-tenancy support.

## Overview

Kueue is a Kubernetes-native job queueing system that provides:
- **Job queuing** - FIFO and priority-based queue management
- **Resource quotas** - Fine-grained resource limits per team/namespace
- **Resource flavors** - Different resource types (GPU models, node types)
- **Fair sharing** - Balanced resource allocation across teams
- **Workload management** - Admission control and scheduling policies

:::{tip}
Kueue is ideal for multi-tenant environments where teams need isolated resource quotas and fair access to cluster resources.
:::

## Prerequisites

### Install Kueue

Follow the official [Kueue installation guide](https://kueue.sigs.k8s.io/docs/installation/):

```bash
kubectl apply -f https://github.com/kubernetes-sigs/kueue/releases/latest/download/manifests.yaml
```

### Verify Installation

```bash
kubectl get pods -n kueue-system
```

Expected output:
```
NAME                                        READY   STATUS    RESTARTS   AGE
kueue-controller-manager-xxxxx-yyyyy        2/2     Running   0          1m
```

## Configuration

Kueue integration with Kubeflow Trainer is handled through labels on the TrainJob, rather than the `podGroupPolicy` field.

### Basic Setup

#### 1. Create Resource Flavors

Define resource types available in your cluster:

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: a100-gpu
spec:
  nodeLabels:
    gpu.type: a100
    cloud.provider: aws
    instance.type: p4d.24xlarge
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: v100-gpu
spec:
  nodeLabels:
    gpu.type: v100
    cloud.provider: aws
    instance.type: p3.8xlarge
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: cpu-only
spec:
  nodeLabels:
    workload.type: cpu
```

#### 2. Create Cluster Queues

Define resource pools with quotas:

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: ml-cluster-queue
spec:
  namespaceSelector: {}  # All namespaces
  resourceGroups:
    - coveredResources: ["cpu", "memory", "nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: cpu
              nominalQuota: 400
            - name: memory
              nominalQuota: 2048Gi
            - name: nvidia.com/gpu
              nominalQuota: 32
              borrowingLimit: 16  # Can borrow up to 16 more GPUs
        - name: v100-gpu
          resources:
            - name: cpu
              nominalQuota: 800
            - name: memory
              nominalQuota: 4096Gi
            - name: nvidia.com/gpu
              nominalQuota: 64
```

#### 3. Create Local Queues

Create namespace-specific queues:

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata:
  name: team-a-queue
  namespace: team-a
spec:
  clusterQueue: ml-cluster-queue
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata:
  name: team-b-queue
  namespace: team-b
spec:
  clusterQueue: ml-cluster-queue
```

### TrainJob Configuration

Reference the local queue in your TrainJob:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-training
  namespace: team-a
  labels:
    kueue.x-k8s.io/queue-name: team-a-queue
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
      requests:
        cpu: "8"
        memory: "32Gi"
        nvidia.com/gpu: "1"
      limits:
        cpu: "16"
        memory: "64Gi"
        nvidia.com/gpu: "1"
```

:::{note}
Unlike Volcano or Coscheduling, Kueue configuration is done through labels and separate CRDs, not through `podGroupPolicy`.
:::

## Complete Examples

### Example 1: Multi-Team Resource Management

**Cluster Queue with Team Quotas:**

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: shared-ml-cluster
spec:
  namespaceSelector: {}
  resourceGroups:
    - coveredResources: ["cpu", "memory", "nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 64
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata:
  name: team-a-queue
  namespace: team-a
spec:
  clusterQueue: shared-ml-cluster
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata:
  name: team-b-queue
  namespace: team-b
spec:
  clusterQueue: shared-ml-cluster
```

**Team A TrainJob:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: team-a-llm-training
  namespace: team-a
  labels:
    kueue.x-k8s.io/queue-name: team-a-queue
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train_llm.py"]
    resourcesPerNode:
      requests:
        nvidia.com/gpu: "4"
      limits:
        nvidia.com/gpu: "4"
```

**Team B TrainJob:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: team-b-cv-training
  namespace: team-b
  labels:
    kueue.x-k8s.io/queue-name: team-b-queue
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train_cv.py"]
    resourcesPerNode:
      requests:
        nvidia.com/gpu: "2"
      limits:
        nvidia.com/gpu: "2"
```

### Example 2: Priority-Based Scheduling

**Cluster Queue with Preemption:**

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: priority-queue
spec:
  namespaceSelector: {}
  preemption:
    reclaimWithinCohort: Any
    withinClusterQueue: LowerPriority
  resourceGroups:
    - coveredResources: ["nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 32
```

**High Priority Queue:**

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata:
  name: high-priority
  namespace: production
spec:
  clusterQueue: priority-queue
---
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: high-priority-training
value: 1000
globalDefault: false
```

**High Priority TrainJob:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: urgent-training
  namespace: production
  labels:
    kueue.x-k8s.io/queue-name: high-priority
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    priorityClassName: high-priority-training
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
    resourcesPerNode:
      requests:
        nvidia.com/gpu: "2"
```

### Example 3: Resource Flavors for Different GPU Types

**Multiple Resource Flavors:**

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: a100-40gb
spec:
  nodeLabels:
    gpu.model: a100
    gpu.memory: 40gb
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: a100-80gb
spec:
  nodeLabels:
    gpu.model: a100
    gpu.memory: 80gb
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: multi-flavor-queue
spec:
  namespaceSelector: {}
  resourceGroups:
    - coveredResources: ["nvidia.com/gpu"]
      flavors:
        - name: a100-80gb
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 16
        - name: a100-40gb
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 32
```

**TrainJob Requesting Specific Flavor:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: llm-training
  namespace: ml-team
  labels:
    kueue.x-k8s.io/queue-name: ml-queue
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 8
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
    resourcesPerNode:
      requests:
        nvidia.com/gpu: "4"
    # Request specific GPU type via node affinity
  podTemplateOverrides:
    - targetReplicatedJob: node
      podTemplateSpec:
        spec:
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: gpu.memory
                        operator: In
                        values: ["80gb"]
```

## Monitoring

### Check Workload Status

Kueue creates a Workload resource for each TrainJob:

```bash
kubectl get workloads -A
```

Output:
```
NAMESPACE   NAME                AGE
team-a      pytorch-training    2m
team-b      cv-training         1m
```

### Describe Workload

```bash
kubectl describe workload pytorch-training -n team-a
```

Output shows admission status, resource requests, and conditions.

### Check Queue Status

```bash
kubectl get clusterqueue
```

Output:
```
NAME                COHORT   PENDING WORKLOADS   ADMITTED WORKLOADS
ml-cluster-queue             0                   2
```

```bash
kubectl get localqueue -A
```

Output:
```
NAMESPACE   NAME            CLUSTERQUEUE       PENDING   ADMITTED
team-a      team-a-queue    ml-cluster-queue   0         1
team-b      team-b-queue    ml-cluster-queue   0         1
```

### View Queue Details

```bash
kubectl describe clusterqueue ml-cluster-queue
```

Shows quota usage, admitted workloads, and pending workloads.

## Quotas and Limits

### Nominal Quota

Base resource allocation for a queue:

```yaml
resources:
  - name: nvidia.com/gpu
    nominalQuota: 32  # Base allocation
```

### Borrowing Limit

Additional resources that can be borrowed when available:

```yaml
resources:
  - name: nvidia.com/gpu
    nominalQuota: 32
    borrowingLimit: 16  # Can use up to 48 total (32 + 16)
```

### Lending Limit

Resources that can be lent to other queues:

```yaml
resources:
  - name: nvidia.com/gpu
    nominalQuota: 32
    lendingLimit: 16  # Up to 16 can be borrowed by others
```

### Example with All Limits

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: flexible-queue
spec:
  resourceGroups:
    - coveredResources: ["nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 32      # Guaranteed
              borrowingLimit: 16    # Can borrow up to 16 more
              lendingLimit: 16      # Can lend up to 16
```

## Cohorts for Resource Sharing

Cohorts allow multiple ClusterQueues to share resources:

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: team-a-queue
spec:
  cohort: ml-cohort  # Part of ml-cohort
  resourceGroups:
    - coveredResources: ["nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 16
              borrowingLimit: 16
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: team-b-queue
spec:
  cohort: ml-cohort  # Part of same cohort
  resourceGroups:
    - coveredResources: ["nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 16
              borrowingLimit: 16
```

Teams can borrow unused resources from each other within the cohort.

## Troubleshooting

### Workload Not Admitted

**Check workload status:**

```bash
kubectl describe workload <workload-name> -n <namespace>
```

Look for admission conditions and events.

**Common causes:**
- Quota exceeded
- No matching resource flavor
- Queue full
- Resource requests don't match available flavors

**Check queue capacity:**

```bash
kubectl describe clusterqueue <queue-name>
```

### Quota Exceeded

**Check quota usage:**

```bash
kubectl get clusterqueue -o yaml
```

Look at `status.reservingWorkloads` and `status.admittedWorkloads`.

**Solutions:**
1. Increase nominal quota
2. Enable borrowing
3. Wait for jobs to complete
4. Use a different queue

### Wrong Resource Flavor Selected

**Check workload resource requests:**

```bash
kubectl get workload <workload-name> -n <namespace> -o yaml
```

**Ensure node affinity matches flavor:**

```yaml
spec:
  podTemplateOverrides:
    - podTemplateSpec:
        spec:
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: gpu.type
                        operator: In
                        values: ["a100"]
```

## Best Practices

### 1. Define Clear Resource Flavors

```yaml
# Good: Specific flavors
- name: a100-80gb-nvlink
- name: a100-40gb-pcie
- name: v100-32gb

# Avoid: Generic flavors
- name: gpu
```

### 2. Set Appropriate Quotas

```yaml
# Start conservative
nominalQuota: 16
borrowingLimit: 8

# Adjust based on usage patterns
```

### 3. Use Cohorts for Flexibility

```yaml
# Group related teams
spec:
  cohort: ml-workloads

# Allows resource sharing while maintaining quotas
```

### 4. Configure Preemption Carefully

```yaml
preemption:
  reclaimWithinCohort: LowerPriority  # Only preempt lower priority
  withinClusterQueue: LowerOrNewerEqualPriority
```

### 5. Monitor Queue Metrics

```yaml
# Prometheus metrics
- kueue_cluster_queue_resource_reservation
- kueue_admitted_workloads_total
- kueue_pending_workloads
```

### 6. Use Fair Sharing

```yaml
spec:
  fairSharing:
    enable: true
    weight: 1.0
```

## Advanced Features

### Workload Priority

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: WorkloadPriorityClass
metadata:
  name: high-priority
value: 1000
description: "High priority ML training"
```

### Resource Reservations

```yaml
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: reserved-queue
spec:
  resourceGroups:
    - coveredResources: ["nvidia.com/gpu"]
      flavors:
        - name: a100-gpu
          resources:
            - name: nvidia.com/gpu
              nominalQuota: 32
              # Reserve 8 GPUs always available
              reservationQuota: 8
```

## Integration with Kubeflow Platform

For comprehensive documentation on Kueue integration with Kubeflow Trainer, see the official [Kueue documentation for TrainJobs](https://kueue.sigs.k8s.io/docs/tasks/run/trainjobs/).

## Next Steps

- [Job Scheduling Overview](index) - Compare scheduling solutions
- [Volcano Integration](volcano) - Alternative advanced scheduler
- [Coscheduling](coscheduling) - Simple gang scheduling
- [Kueue Official Docs](https://kueue.sigs.k8s.io/) - Comprehensive Kueue documentation
