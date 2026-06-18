# KAI Scheduler

This guide describes how to enable **gang scheduling** and advanced resource management with the
[NVIDIA KAI Scheduler](https://github.com/kai-scheduler/KAI-Scheduler) in Kubeflow Trainer.

By integrating KAI Scheduler, you ensure "all-or-nothing" scheduling for distributed training jobs.
This means the job only starts if all requested GPU resources are available simultaneously, preventing
resource deadlocks in multi-node training.

## Prerequisites

1. **Install KAI Scheduler:** Follow the
   [KAI Installation Guide](https://github.com/kai-scheduler/KAI-Scheduler/blob/main/README.md)
   to set up the scheduler and the `podgrouper` service in your Kubernetes cluster.
2. **Define a Queue:** KAI uses queues to manage resources. Ensure you have a KAI Queue created
   (e.g., `training-queue`) or use the `default-queue` created during installation.

## Enable KAI Plugin

KAI scheduling can be enabled by setting the `schedulerName` to `kai-scheduler` in the pod template
of your `TrainingRuntime` or `ClusterTrainingRuntime` specification.

:::{note}
KAI integrates externally via its PodGrouper component, which monitors pods requesting the `kai-scheduler`.
:::

### Example: ClusterTrainingRuntime with KAI

You can enforce KAI scheduling at the runtime level. This ensures that every job using this runtime
automatically utilizes KAI gang-scheduling.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: pytorch-kai-runtime
spec:
  mlPolicy:
    torch:
      numNodes: 1
  template:
    spec:
      schedulerName: kai-scheduler
      containers:
        - name: train
          image: pytorch/pytorch:latest
```

### Example: TrainJob with KAI

Once your runtime is created, you can submit a `TrainJob` that references it. You can also add the
`kai.scheduler/queue` label to your job to route it to a specific resource queue in KAI.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-kai-job
  labels:
    kai.scheduler/queue: "prod-queue" # KAI Scheduler uses this to route the job
spec:
  runtimeRef:
    name: pytorch-kai-runtime
  trainer:
    numNodes: 4
    resourcesPerNode:
      limits:
        nvidia.com/gpu: 1
```

## How it Works

When a `TrainJob` is created using a runtime configured with the kai-scheduler:

1. **Metadata Propagation:** The Trainer Operator applies the necessary labels and annotations to
   the underlying `JobSet`.
2. **Pod Grouping:** The KAI `podgrouper` component detects the training pods via the
   `OwnerReference` chain and automatically creates a KAI `PodGroup` resource.
3. **Gang Scheduling:** The KAI Scheduler identifies the `PodGroup` and ensures all replicas
   (workers) are scheduled at once on nodes assigned to the specified queue.
