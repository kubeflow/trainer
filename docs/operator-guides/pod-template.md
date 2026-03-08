# Pod Template Overrides

Pod template overrides allow you to customize specific pods within a TrainJob, providing fine-grained control over individual training nodes.

## Overview

While Training Runtimes define default pod configurations, `podTemplateOverrides` in TrainJob specifications enable:

- **Per-node customization** - Different configurations for specific replicas
- **Resource heterogeneity** - Varied resource allocations across nodes
- **Specialized configurations** - Custom environment variables, volumes, or affinity rules
- **Role-specific settings** - Different settings for master vs worker nodes

## Basic Structure

Pod template overrides are specified in the TrainJob:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: custom-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
  podTemplateOverrides:
    - targetReplicatedJob: node
      replicaIndex: 0
      podTemplateSpec:
        spec:
          # Custom pod spec for replica 0
```

## Override Fields

### targetReplicatedJob

Name of the replicated job to target (from runtime template).

**Type:** String
**Required:** Yes

**Example:**
```yaml
targetReplicatedJob: node  # Target the "node" replicated job
```

### replicaIndex

Index of the specific replica to override (0-indexed).

**Type:** Integer
**Required:** No (if omitted, applies to all replicas)

**Example:**
```yaml
replicaIndex: 0  # Override only the first replica (rank 0)
```

### podTemplateSpec

Kubernetes PodTemplateSpec with customizations.

**Type:** PodTemplateSpec
**Required:** Yes

## Common Use Cases

### 1. Different Resources for Master Node

Allocate more memory to the master node:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-master-custom
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]
    resourcesPerNode:
      limits:
        memory: "16Gi"
        nvidia.com/gpu: "1"
  podTemplateOverrides:
    - targetReplicatedJob: node
      replicaIndex: 0  # Master node
      podTemplateSpec:
        spec:
          containers:
            - name: trainer
              resources:
                limits:
                  memory: "32Gi"  # 2x memory for master
                  nvidia.com/gpu: "1"
```

### 2. Custom Environment Variables

Add debugging variables to a specific node:

```yaml
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    podTemplateSpec:
      spec:
        containers:
          - name: trainer
            env:
              - name: NCCL_DEBUG
                value: INFO
              - name: TORCH_DISTRIBUTED_DEBUG
                value: DETAIL
              - name: PYTORCH_CUDA_ALLOC_CONF
                value: max_split_size_mb:512
```

### 3. Node Affinity and Tolerations

Place specific replicas on particular nodes:

```yaml
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    podTemplateSpec:
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
        tolerations:
          - key: nvidia.com/gpu
            operator: Exists
            effect: NoSchedule
```

### 4. Additional Volume Mounts

Mount extra volumes for specific nodes:

```yaml
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    podTemplateSpec:
      spec:
        volumes:
          - name: checkpoint-storage
            persistentVolumeClaim:
              claimName: checkpoint-pvc
        containers:
          - name: trainer
            volumeMounts:
              - name: checkpoint-storage
                mountPath: /checkpoints
```

### 5. Custom Init Containers

Add initialization for specific nodes:

```yaml
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    podTemplateSpec:
      spec:
        initContainers:
          - name: download-checkpoint
            image: amazon/aws-cli:latest
            command:
              - sh
              - -c
              - |
                aws s3 sync s3://my-bucket/checkpoints /checkpoints
            volumeMounts:
              - name: checkpoints
                mountPath: /checkpoints
            env:
              - name: AWS_REGION
                value: us-west-2
        volumes:
          - name: checkpoints
            emptyDir: {}
```

### 6. Sidecar Containers

Add monitoring sidecars to specific nodes:

```yaml
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    podTemplateSpec:
      spec:
        containers:
          - name: trainer
            # Main container config

          - name: nvidia-dcgm-exporter
            image: nvcr.io/nvidia/k8s/dcgm-exporter:latest
            ports:
              - containerPort: 9400
                name: metrics
            securityContext:
              capabilities:
                add: ["SYS_ADMIN"]
```

## Complete Examples

### Example 1: Heterogeneous GPU Training

Different GPU types across nodes:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: mixed-gpu-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command: ["torchrun", "train.py"]
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "1"

  # Node 0-1: A100 GPUs (high memory)
  podTemplateOverrides:
    - targetReplicatedJob: node
      replicaIndex: 0
      podTemplateSpec:
        spec:
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: gpu.type
                        operator: In
                        values: ["a100"]
          containers:
            - name: trainer
              resources:
                limits:
                  memory: "80Gi"
                  nvidia.com/gpu: "1"

    - targetReplicatedJob: node
      replicaIndex: 1
      podTemplateSpec:
        spec:
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: gpu.type
                        operator: In
                        values: ["a100"]
          containers:
            - name: trainer
              resources:
                limits:
                  memory: "80Gi"
                  nvidia.com/gpu: "1"

    # Node 2-3: V100 GPUs (standard memory)
    - targetReplicatedJob: node
      replicaIndex: 2
      podTemplateSpec:
        spec:
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: gpu.type
                        operator: In
                        values: ["v100"]
          containers:
            - name: trainer
              resources:
                limits:
                  memory: "32Gi"
                  nvidia.com/gpu: "1"

    - targetReplicatedJob: node
      replicaIndex: 3
      podTemplateSpec:
        spec:
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                  - matchExpressions:
                      - key: gpu.type
                        operator: In
                        values: ["v100"]
          containers:
            - name: trainer
              resources:
                limits:
                  memory: "32Gi"
                  nvidia.com/gpu: "1"
```

### Example 2: Master Node with Checkpointing

Only the master node saves checkpoints:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-checkpointing
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1
    command:
      - torchrun
      - train.py
      - --checkpoint-dir
      - /checkpoints

  podTemplateOverrides:
    - targetReplicatedJob: node
      replicaIndex: 0  # Master saves checkpoints
      podTemplateSpec:
        spec:
          volumes:
            - name: checkpoint-storage
              persistentVolumeClaim:
                claimName: training-checkpoints
          containers:
            - name: trainer
              volumeMounts:
                - name: checkpoint-storage
                  mountPath: /checkpoints
              env:
                - name: SAVE_CHECKPOINTS
                  value: "true"
                - name: CHECKPOINT_INTERVAL
                  value: "100"
```

### Example 3: Debug Mode for Specific Node

Enable debugging on one node:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: debug-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 2
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py"]

  podTemplateOverrides:
    - targetReplicatedJob: node
      replicaIndex: 0
      podTemplateSpec:
        spec:
          containers:
            - name: trainer
              env:
                - name: NCCL_DEBUG
                  value: INFO
                - name: TORCH_DISTRIBUTED_DEBUG
                  value: DETAIL
                - name: TORCH_SHOW_CPP_STACKTRACES
                  value: "1"
                - name: PYTHONUNBUFFERED
                  value: "1"
              command:
                - python
                - -m
                - debugpy
                - --listen
                - 0.0.0.0:5678
                - -m
                - torch.distributed.run
                - train.py
              ports:
                - containerPort: 5678
                  name: debugpy
```

## Merge Behavior

Pod template overrides are **merged** with the runtime template, not replaced.

### Merge Strategy

- **Primitive values** (strings, numbers): Override replaces runtime value
- **Maps/Objects**: Deep merge (keys are merged recursively)
- **Lists**: Override replaces entire list

**Example:**

**Runtime template:**
```yaml
spec:
  containers:
    - name: trainer
      env:
        - name: VAR1
          value: "from-runtime"
      resources:
        limits:
          memory: "16Gi"
```

**Override:**
```yaml
podTemplateSpec:
  spec:
    containers:
      - name: trainer
        env:
          - name: VAR2
            value: "from-override"
        resources:
          limits:
            cpu: "8"
```

**Effective result:**
```yaml
spec:
  containers:
    - name: trainer
      env:
        - name: VAR2  # Override replaces entire env list
          value: "from-override"
      resources:
        limits:
          memory: "16Gi"  # From runtime
          cpu: "8"        # From override (merged)
```

:::{warning}
Environment variables and volume mount lists are **replaced**, not merged. Include all required variables in overrides.
:::

## Validation

### Common Validation Errors

**Invalid replica index:**
```
Error: replicaIndex 5 exceeds numNodes 4
```

**Solution:** Ensure replica index is less than numNodes.

**Target job not found:**
```
Error: replicatedJob "worker" not found in runtime template
```

**Solution:** Use correct replicated job name from runtime.

**Container name mismatch:**
```
Error: container "pytorch" not found, expected "trainer"
```

**Solution:** Match container names from runtime template.

## Best Practices

### 1. Minimize Overrides

Use overrides sparingly for special cases only:

```yaml
# Good: Override only when necessary
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0  # Only master node

# Avoid: Overriding every replica
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
  - targetReplicatedJob: node
    replicaIndex: 1
  # ... etc (consider creating a custom runtime instead)
```

### 2. Document Override Reasons

```yaml
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    # Reason: Master node requires more memory for gradient aggregation
    podTemplateSpec:
      spec:
        containers:
          - name: trainer
            resources:
              limits:
                memory: "64Gi"
```

### 3. Use Labels for Identification

```yaml
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    podTemplateSpec:
      metadata:
        labels:
          role: master
          debug: enabled
```

### 4. Test with Small Node Counts

Validate overrides with minimal replicas first:

```yaml
trainer:
  numNodes: 2  # Test with 2 nodes first
podTemplateOverrides:
  - replicaIndex: 0
    # Test override logic
```

### 5. Use Override Templates

Create reusable override patterns:

```yaml
# Common pattern for master node
podTemplateOverrides:
  - targetReplicatedJob: node
    replicaIndex: 0
    podTemplateSpec: &master-config
      spec:
        containers:
          - name: trainer
            resources:
              limits:
                memory: "64Gi"
```

## Troubleshooting

### Override Not Applied

**Check TrainJob status:**
```bash
kubectl describe trainjob <job-name>
```

Look for validation errors in events.

**Verify pod configuration:**
```bash
kubectl get pod <pod-name> -o yaml
```

Compare with expected override values.

### Wrong Pod Affected

**Check replica index:**
```bash
# List pods with indices
kubectl get pods -l trainer.kubeflow.org/job-name=<job-name> \
  --sort-by=.metadata.name
```

Pods are typically named `<job-name>-node-<index>-0-<hash>`.

### Resource Conflicts

**Check resource requests vs limits:**
```yaml
resources:
  requests:
    memory: "32Gi"
  limits:
    memory: "16Gi"  # ERROR: limit < request
```

**Solution:** Ensure limits >= requests.

## Next Steps

- [Job Templates](job-template) - Configure JobSet structure
- [Training Runtimes](runtime) - Define base runtime templates
- [ML Policies](ml-policy) - Configure ML-specific settings
