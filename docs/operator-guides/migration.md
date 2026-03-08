# Migration from v1 to v2

This guide helps you migrate from Kubeflow Training Operator v1 to Kubeflow Trainer v2.

## Overview of Changes

Kubeflow Trainer v2 represents a significant redesign from the Training Operator v1, introducing new APIs, a unified resource model, and enhanced capabilities for distributed training.

### Key Improvements in v2

- **Unified API** - Single `TrainJob` CRD replaces framework-specific resources (`PyTorchJob`, `TFJob`, `MPIJob`)
- **Runtime Abstraction** - `TrainingRuntime` and `ClusterTrainingRuntime` separate infrastructure configuration from job definition
- **Python SDK** - First-class SDK support eliminates the need for YAML in most use cases
- **Custom Initializers** - Built-in dataset and model initialization across distributed nodes
- **Enhanced MPI** - Improved MPI-Operator v2 with SSH-based communication for better performance
- **Extension Framework** - Plugin architecture for custom integrations and policies
- **Better Scheduling** - Native integration with Volcano, Kueue, and coscheduling

:::{important}
v1 and v2 can coexist in the same cluster, allowing gradual migration. However, they use different CRDs and controllers.
:::

## API Comparison

### v1: Framework-Specific Resources

In v1, each ML framework required a dedicated CRD with framework-specific configurations:

#### PyTorchJob (v1)

```yaml
apiVersion: kubeflow.org/v1
kind: PyTorchJob
metadata:
  name: pytorch-distributed
spec:
  pytorchReplicaSpecs:
    Master:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - name: pytorch
              image: pytorch/pytorch:1.9.0-cuda11.1-cudnn8-runtime
              command:
                - python
                - /workspace/train.py
              resources:
                limits:
                  nvidia.com/gpu: 1
    Worker:
      replicas: 3
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - name: pytorch
              image: pytorch/pytorch:1.9.0-cuda11.1-cudnn8-runtime
              command:
                - python
                - /workspace/train.py
              resources:
                limits:
                  nvidia.com/gpu: 1
```

#### TFJob (v1)

```yaml
apiVersion: kubeflow.org/v1
kind: TFJob
metadata:
  name: tensorflow-distributed
spec:
  tfReplicaSpecs:
    Chief:
      replicas: 1
      template:
        spec:
          containers:
            - name: tensorflow
              image: tensorflow/tensorflow:2.9.0-gpu
              command:
                - python
                - /workspace/train.py
    Worker:
      replicas: 2
      template:
        spec:
          containers:
            - name: tensorflow
              image: tensorflow/tensorflow:2.9.0-gpu
              command:
                - python
                - /workspace/train.py
```

### v2: Unified TrainJob with Runtimes

In v2, a single `TrainJob` CRD works with all frameworks, using runtimes to define execution patterns:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-distributed
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    image: pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command:
      - torchrun
      - /workspace/train.py
    resourcesPerNode:
      requests:
        cpu: "4"
        memory: "16Gi"
      limits:
        nvidia.com/gpu: "1"
```

## Migration Patterns

### Pattern 1: PyTorchJob to TrainJob

**v1 PyTorchJob:**

```yaml
apiVersion: kubeflow.org/v1
kind: PyTorchJob
metadata:
  name: mnist-training
spec:
  pytorchReplicaSpecs:
    Master:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - name: pytorch
              image: pytorch/pytorch:1.9.0
              command: ["python", "train.py"]
              args: ["--epochs", "10"]
              resources:
                limits:
                  nvidia.com/gpu: 1
    Worker:
      replicas: 2
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - name: pytorch
              image: pytorch/pytorch:1.9.0
              command: ["python", "train.py"]
              args: ["--epochs", "10"]
              resources:
                limits:
                  nvidia.com/gpu: 1
```

**v2 TrainJob Equivalent:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: mnist-training
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 3  # 1 master + 2 workers
    image: pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command:
      - torchrun
      - train.py
      - --epochs
      - "10"
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "1"
```

**Key Changes:**
- No separate master/worker replica specs
- `numNodes` defines total node count
- Runtime reference (`torch-distributed`) defines execution pattern
- Simplified resource specification applies to all nodes

### Pattern 2: TFJob to TrainJob

**v1 TFJob:**

```yaml
apiVersion: kubeflow.org/v1
kind: TFJob
metadata:
  name: tf-distributed
spec:
  tfReplicaSpecs:
    Chief:
      replicas: 1
      template:
        spec:
          containers:
            - name: tensorflow
              image: tensorflow/tensorflow:2.9.0-gpu
              command: ["python", "train.py"]
    Worker:
      replicas: 3
      template:
        spec:
          containers:
            - name: tensorflow
              image: tensorflow/tensorflow:2.9.0-gpu
              command: ["python", "train.py"]
    PS:
      replicas: 2
      template:
        spec:
          containers:
            - name: tensorflow
              image: tensorflow/tensorflow:2.9.0-gpu
              command: ["python", "ps.py"]
```

**v2 Approach:**

TensorFlow training in v2 typically uses a custom runtime. For simple cases:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: tf-distributed
spec:
  runtimeRef:
    name: tensorflow-distributed  # Custom runtime
  trainer:
    numNodes: 4
    image: tensorflow/tensorflow:2.14.0-gpu
    command:
      - python
      - train.py
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "1"
```

:::{note}
For complex TensorFlow setups with parameter servers, you may need to create a custom `TrainingRuntime` that defines the worker and PS replica structure. See [Training Runtimes](runtime) for details.
:::

### Pattern 3: MPIJob to TrainJob

**v1 MPIJob:**

```yaml
apiVersion: kubeflow.org/v1
kind: MPIJob
metadata:
  name: mpi-training
spec:
  slotsPerWorker: 1
  mpiReplicaSpecs:
    Launcher:
      replicas: 1
      template:
        spec:
          containers:
            - name: mpi
              image: horovod/horovod:0.23.0
              command: ["mpirun"]
              args:
                - "-np"
                - "4"
                - "python"
                - "train.py"
    Worker:
      replicas: 4
      template:
        spec:
          containers:
            - name: mpi
              image: horovod/horovod:0.23.0
              resources:
                limits:
                  nvidia.com/gpu: 1
```

**v2 TrainJob Equivalent:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: mpi-training
spec:
  runtimeRef:
    name: deepspeed-distributed  # Uses MPI
  trainer:
    numNodes: 4
    image: deepspeed/deepspeed:latest
    command:
      - deepspeed
      - train.py
    resourcesPerNode:
      limits:
        nvidia.com/gpu: "1"
```

## Breaking Changes

### API Structure

| Aspect | v1 | v2 |
|--------|----|----|
| **Resource Kind** | Framework-specific (PyTorchJob, TFJob, MPIJob) | Unified `TrainJob` |
| **Replica Specs** | Per-role replicas (Master, Worker, PS) | Single `numNodes` configuration |
| **Runtime** | Embedded in job spec | External `TrainingRuntime` reference |
| **Container Spec** | Per-replica template | Unified `trainer` spec |
| **Status Fields** | Framework-specific conditions | Standardized conditions |

### Field Mappings

| v1 Field | v2 Equivalent |
|----------|---------------|
| `spec.pytorchReplicaSpecs.Master.replicas` | Included in `spec.trainer.numNodes` |
| `spec.pytorchReplicaSpecs.Worker.replicas` | Included in `spec.trainer.numNodes` |
| `spec.pytorchReplicaSpecs.Worker.template.spec.containers[0]` | `spec.trainer.image`, `spec.trainer.command` |
| `spec.pytorchReplicaSpecs.Worker.template.spec.containers[0].resources` | `spec.trainer.resourcesPerNode` |
| `spec.tfReplicaSpecs.PS` | Custom runtime with multiple replica types |
| `spec.mpiReplicaSpecs.slotsPerWorker` | `spec.trainer.mlPolicy.mpi.numProcPerNode` |

### Environment Variables

**v1 Environment Variables (PyTorchJob):**
- `MASTER_ADDR`
- `MASTER_PORT`
- `RANK`
- `WORLD_SIZE`

**v2 Environment Variables (TrainJob):**
- `PET_NNODES` - Number of nodes
- `PET_NPROC_PER_NODE` - Processes per node
- `PET_NODE_RANK` - Node rank
- Framework-specific variables are managed by the runtime

### Status Conditions

**v1 Conditions:**
```yaml
status:
  conditions:
    - type: Created
    - type: Running
    - type: Succeeded
    - type: Failed
  replicaStatuses:
    Master:
      active: 1
    Worker:
      active: 2
```

**v2 Conditions:**
```yaml
status:
  conditions:
    - type: DatasetInitializerCreated
    - type: ModelInitializerCreated
    - type: TrainerCreated
    - type: Succeeded
    - type: Failed
  runtimeStatus:
    phase: Running
    message: "Training in progress"
```

## Migration Steps

### Step 1: Install v2 Alongside v1

v1 and v2 can run in parallel, allowing gradual migration:

```bash
# v1 continues running in its namespace
kubectl get pytorchjobs -A

# Install v2 in a separate namespace or cluster-wide
export VERSION=v2.1.0
kubectl apply --server-side -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=${VERSION}"

# Install runtimes
kubectl apply --server-side -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=${VERSION}"
```

### Step 2: Identify Jobs to Migrate

List all v1 jobs:

```bash
# PyTorchJobs
kubectl get pytorchjobs -A

# TFJobs
kubectl get tfjobs -A

# MPIJobs
kubectl get mpijobs -A
```

### Step 3: Create Equivalent v2 Runtimes

For standard frameworks, use built-in runtimes:
- **PyTorch** → `torch-distributed`
- **DeepSpeed** → `deepspeed-distributed`
- **Horovod/MPI** → `deepspeed-distributed` (uses MPI)

For custom configurations, create a `TrainingRuntime`:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: custom-pytorch
  namespace: training
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
                      # Image, command will be overridden by TrainJob
```

### Step 4: Convert Job Definitions

Use the patterns above to convert your v1 jobs to v2 `TrainJobs`.

**Conversion Checklist:**
- [ ] Change `apiVersion` to `trainer.kubeflow.org/v1alpha1`
- [ ] Change `kind` to `TrainJob`
- [ ] Add `spec.runtimeRef` pointing to appropriate runtime
- [ ] Convert replica specs to `spec.trainer.numNodes`
- [ ] Move container spec to `spec.trainer`
- [ ] Convert per-replica resources to `spec.trainer.resourcesPerNode`
- [ ] Update environment variables if custom ones were used
- [ ] Test in development environment

### Step 5: Test in Development

Deploy converted jobs in a test namespace:

```bash
kubectl apply -f converted-job.yaml -n test-namespace
kubectl get trainjob -n test-namespace
kubectl describe trainjob <job-name> -n test-namespace
kubectl logs -l trainer.kubeflow.org/job-name=<job-name> -n test-namespace
```

### Step 6: Gradual Production Migration

Migrate jobs incrementally:

1. **New jobs** - Create all new training jobs using v2 `TrainJob`
2. **Low-priority jobs** - Migrate non-critical workloads first
3. **High-priority jobs** - Migrate after validating v2 in production
4. **Decommission v1** - Once all jobs are migrated, remove v1 operator

### Step 7: Clean Up v1 Resources

After successful migration:

```bash
# Delete v1 jobs
kubectl delete pytorchjobs --all -A
kubectl delete tfjobs --all -A
kubectl delete mpijobs --all -A

# Uninstall v1 operator (if not part of Kubeflow platform)
kubectl delete -f <v1-operator-manifest>
```

## Python SDK Migration

### v1: Direct YAML Submission

```python
from kubernetes import client, config

config.load_kube_config()
api = client.CustomObjectsApi()

job = {
    "apiVersion": "kubeflow.org/v1",
    "kind": "PyTorchJob",
    "metadata": {"name": "pytorch-job"},
    "spec": {
        "pytorchReplicaSpecs": {
            "Master": {"replicas": 1, ...},
            "Worker": {"replicas": 2, ...}
        }
    }
}

api.create_namespaced_custom_object(
    group="kubeflow.org",
    version="v1",
    namespace="default",
    plural="pytorchjobs",
    body=job
)
```

### v2: Kubeflow Python SDK

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

def train_fn():
    import torch
    # Training code here

client = TrainerClient()

job_id = client.train(
    trainer=CustomTrainer(
        func=train_fn,
        num_nodes=3,
        resources_per_node={
            "cpu": 4,
            "memory": "16Gi",
            "gpu": 1
        }
    )
)

print(f"Job created: {job_id}")
```

**Key Benefits:**
- No YAML required
- Type checking and validation
- Simplified API
- Better error messages
- Local execution support

## Common Migration Scenarios

### Scenario 1: Multi-Replica PyTorchJob with Different Configs

**v1 Problem:** Different master and worker configurations

```yaml
pytorchReplicaSpecs:
  Master:
    replicas: 1
    template:
      spec:
        containers:
          - name: pytorch
            resources:
              limits:
                memory: "32Gi"  # More memory for master
  Worker:
    replicas: 3
    template:
      spec:
        containers:
          - name: pytorch
            resources:
              limits:
                memory: "16Gi"
```

**v2 Solution:** Use PodTemplate overrides

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-job
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
  podTemplateOverrides:
    - targetReplicatedJob: node
      replicaIndex: 0  # Master node
      podTemplateSpec:
        spec:
          containers:
            - name: trainer
              resources:
                limits:
                  memory: "32Gi"
```

### Scenario 2: Jobs with Init Containers

**v1:**

```yaml
spec:
  pytorchReplicaSpecs:
    Worker:
      template:
        spec:
          initContainers:
            - name: download-data
              image: busybox
              command: ["sh", "-c", "wget -O /data/dataset.tar"]
          containers:
            - name: pytorch
              volumeMounts:
                - name: data
                  mountPath: /data
          volumes:
            - name: data
              emptyDir: {}
```

**v2:** Use dataset initializers

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-job
spec:
  runtimeRef:
    name: torch-distributed
  datasetConfig:
    storageUri: "s3://my-bucket/dataset.tar"
    env:
      - name: AWS_REGION
        value: us-west-2
  trainer:
    numNodes: 2
    image: pytorch/pytorch:2.5.1
    command: ["torchrun", "train.py", "--data-dir", "/data"]
```

## Troubleshooting

### Job Not Starting

**Check runtime exists:**
```bash
kubectl get clustertrainingruntimes
kubectl get trainingruntimes -n <namespace>
```

**Describe the TrainJob:**
```bash
kubectl describe trainjob <job-name>
```

Look for events indicating missing runtimes or validation errors.

### Different Behavior Than v1

**Compare environment variables:**

```bash
# v1
kubectl exec <v1-pod> -- env | grep MASTER

# v2
kubectl exec <v2-pod> -- env | grep PET
```

**Check runtime configuration:**
```bash
kubectl get clustertrainingruntime <runtime-name> -o yaml
```

### Performance Differences

v2 may show different performance characteristics:

- **MPI jobs**: v2 uses SSH-based communication (faster)
- **PyTorch jobs**: Check `numProcPerNode` configuration
- **Resource allocation**: Verify `resourcesPerNode` matches v1 allocations

## Migration Checklist

Use this checklist for each job migration:

- [ ] Identify v1 job type (PyTorchJob, TFJob, MPIJob)
- [ ] Select appropriate v2 runtime
- [ ] Convert replica specifications to `numNodes`
- [ ] Consolidate container specs into `trainer`
- [ ] Update resource specifications
- [ ] Migrate custom environment variables
- [ ] Test in development environment
- [ ] Validate logs and metrics
- [ ] Compare training performance
- [ ] Deploy to production
- [ ] Monitor for issues
- [ ] Document any custom configurations

## Best Practices

### 1. Use the Python SDK

Instead of YAML, prefer the Kubeflow Python SDK:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()
client.train(trainer=CustomTrainer(...))
```

### 2. Create Reusable Runtimes

Define organization-wide runtimes as `ClusterTrainingRuntime`:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: company-pytorch
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  # Standard configuration
```

### 3. Use Runtime Labels

Label runtimes for easy discovery:

```yaml
metadata:
  labels:
    trainer.kubeflow.org/framework: torch
    company.com/gpu-type: a100
    company.com/approved: "true"
```

### 4. Leverage ML Policies

Use ML policies for framework-specific settings:

```yaml
spec:
  mlPolicy:
    numNodes: 4
    torch:
      numProcPerNode: gpu
```

### 5. Standardize Initializers

Use built-in dataset and model initializers:

```yaml
spec:
  datasetConfig:
    storageUri: "hf://meta-llama/Llama-2-7b"
  modelConfig:
    storageUri: "hf://meta-llama/Llama-2-7b"
```

## Additional Resources

- [Installation Guide](installation) - Install v2 components
- [Training Runtimes](runtime) - Create custom runtimes
- [ML Policies](ml-policy) - Configure ML-specific settings
- [Python SDK Documentation](https://kubeflow.github.io/sdk) - SDK reference
- [Examples Repository](https://github.com/kubeflow/trainer/tree/master/examples) - Migration examples

## Getting Help

If you encounter issues during migration:

1. **Check the logs**: `kubectl logs -n kubeflow-system -l control-plane=trainer-controller-manager`
2. **Review events**: `kubectl describe trainjob <job-name>`
3. **Join the community**: [Kubeflow Slack](https://kubeflow.slack.com) (#kubeflow-trainer)
4. **File an issue**: [GitHub Issues](https://github.com/kubeflow/trainer/issues)
