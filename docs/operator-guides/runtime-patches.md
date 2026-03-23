# Runtime Patches

This guide describes how to use the `RuntimePatches` API in the Kubeflow TrainJob to customize
the training runtime without modifying the underlying TrainingRuntime.

Before exploring this guide, make sure to follow [the Runtime guide](runtime)
to understand the basics of Kubeflow Trainer Runtimes.

## RuntimePatches Overview

The `RuntimePatches` API allows multiple controllers — the user, Kueue, and admission webhooks —
to each contribute configuration patches to a TrainJob's runtime spec. Each contributor owns a
single entry keyed by its `manager` name, which prevents conflicts when multiple systems need to
customize the same TrainJob.

This API replaces the previous `PodTemplateOverrides` API. The key difference is the multi-owner
model: rather than a single flat list of overrides, each controller writes to its own named entry
and the Trainer admission webhook merges all entries into the final JobSet before creation.

```yaml
apiVersion: trainer.kubeflow.org/v2alpha1
kind: TrainJob
metadata:
  name: pytorch-distributed
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/custom-training
  runtimePatches:
    - manager: trainer.kubeflow.org/kubeflow-sdk
      trainingRuntimeSpec:
        template:
          spec:
            replicatedJobs:
              - name: node
                template:
                  spec:
                    template:
                      spec:
                        nodeSelector:
                          accelerator: nvidia-tesla-v100
```

## Migration from PodTemplateOverrides

The old `PodTemplateOverrides` API used a flat `targetJobs` list to specify which jobs to patch.
The new `RuntimePatches` API uses a `manager`-keyed list with a hierarchical spec structure.

**Before (PodTemplateOverrides):**

```yaml
spec:
  podTemplateOverrides:
    - targetJobs:
        - name: node
      spec:
        serviceAccountName: ml-training-sa
        nodeSelector:
          accelerator: nvidia-tesla-v100
        volumes:
          - name: training-data
            persistentVolumeClaim:
              claimName: ml-team-training-pvc
        containers:
          - name: trainer
            volumeMounts:
              - name: training-data
                mountPath: /workspace/data
```

**After (RuntimePatches):**

```yaml
spec:
  runtimePatches:
    - manager: trainer.kubeflow.org/kubeflow-sdk
      trainingRuntimeSpec:
        template:
          spec:
            replicatedJobs:
              - name: node
                template:
                  spec:
                    template:
                      spec:
                        serviceAccountName: ml-training-sa
                        nodeSelector:
                          accelerator: nvidia-tesla-v100
                        volumes:
                          - name: training-data
                            persistentVolumeClaim:
                              claimName: ml-team-training-pvc
                        containers:
                          - name: trainer
                            volumeMounts:
                              - name: training-data
                                mountPath: /workspace/data
```

## API Structure

A `RuntimePatch` entry is structured as a nested hierarchy that maps from the TrainJob down to
individual Pod containers. Each level lets you patch metadata (labels/annotations) or spec fields.

```yaml
runtimePatches:
  - manager: trainer.kubeflow.org/kubeflow-sdk  # who owns this entry (immutable)
    time: "2026-05-01T15:20:00Z"                # auto-stamped by the admission webhook
    trainingRuntimeSpec:                         # patches for ClusterTrainingRuntime-based jobs
      template:
        metadata:                                # JobSet-level labels/annotations
          labels:
            team: ml-platform
        spec:
          replicatedJobs:
            - name: node
              template:
                metadata:                        # Job-level labels/annotations
                  labels:
                    job-label: value
                spec:
                  template:                      # Pod template
                    metadata:                    # Pod-level labels/annotations
                      annotations:
                        monitoring: enabled
                    spec:                        # Pod spec patches
                      serviceAccountName: custom-sa
                      nodeSelector:
                        accelerator: nvidia-tesla-v100
                      containers:
                        - name: trainer          # matches container name in the Runtime
                          volumeMounts:
                            - name: workspace
                              mountPath: /workspace
                      volumes:
                        - name: workspace
                          persistentVolumeClaim:
                            claimName: user-pvc
```

## Ownership Model

### The manager field

The `manager` field identifies who owns a patch entry. It must be a non-empty string of up to 253
characters (typically a domain-prefixed name such as `kueue.x-k8s.io/manager`). Kubeflow SDK
automatically sets `manager: trainer.kubeflow.org/kubeflow-sdk` when user sets options. The
`manager` field is **immutable** after creation.

Each `manager` value maps to exactly one entry in the `runtimePatches` list. When a controller
updates its entry, the previous content is replaced in full. This means controllers cannot
accidentally overwrite each other's patches.

### The time field

The `time` field records when the patch entry was last written. It is automatically set by the
Trainer admission webhook on every create or update — you do not need to set it manually. Its
purpose is observability only: it is not used as a list key.

### Mutability Constraints

Some fields in `runtimePatches` can be modified when the TrainJob is **suspended**. This prevents
runtime modifications that could cause inconsistency between running Pods and the declared spec.

## Common Use Cases

### Custom Service Account and Node Selection

Override the service account and pin training Pods to specific GPU nodes:

```yaml
runtimePatches:
  - manager: trainer.kubeflow.org/kubeflow-sdk
    trainingRuntimeSpec:
      template:
        spec:
          replicatedJobs:
            - name: node
              template:
                spec:
                  template:
                    spec:
                      serviceAccountName: ml-training-sa
                      nodeSelector:
                        accelerator: nvidia-tesla-v100
                        node-pool: gpu-training
```

### Adding Persistent Storage

Mount a PersistentVolumeClaim into the trainer container:

```yaml
runtimePatches:
  - manager: trainer.kubeflow.org/kubeflow-sdk
    trainingRuntimeSpec:
      template:
        spec:
          replicatedJobs:
            - name: node
              template:
                spec:
                  template:
                    spec:
                      volumes:
                        - name: training-data
                          persistentVolumeClaim:
                            claimName: ml-team-training-pvc
                      containers:
                        - name: trainer
                          volumeMounts:
                            - name: training-data
                              mountPath: /workspace/data
```

### Tolerations for Specialized Hardware

Allow Pods to schedule on nodes with GPU and priority taints:

```yaml
runtimePatches:
  - manager: trainer.kubeflow.org/kubeflow-sdk
    trainingRuntimeSpec:
      template:
        spec:
          replicatedJobs:
            - name: node
              template:
                spec:
                  template:
                    spec:
                      tolerations:
                        - key: nvidia.com/gpu
                          operator: Exists
                          effect: NoSchedule
                        - key: training-workload
                          operator: Equal
                          value: high-priority
                          effect: NoSchedule
```

## Restrictions

### Environment Variables for Special Containers

You cannot set environment variables for the following containers via `RuntimePatches`:

- `node` — use the Trainer API instead
- `dataset-initializer` and `model-initializer` - use the Initializer API instead

For these containers, the dedicated APIs in the TrainJob `spec` take precedence and are the correct
way to pass environment variables.

### Trainer Container Fields

You cannot override `command`, `args`, `image`, or `resources` for the `trainer` container in the
`node` replicatedJob using `RuntimePatches`. Use the `Trainer` API fields in the TrainJob spec for
those.
