# PodTemplate Overrides

This guide describes how to use
[the `PodTemplateOverrides` API](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#PodTemplateOverrides)
in the Kubeflow TrainJob.

Before exploring this guide, make sure to follow [the Runtime guide](runtime)
to understand the basics of Kubeflow Trainer Runtimes.

## PodTemplateOverrides Overview

The `PodTemplateOverrides` API allows you to customize Pod templates for specific jobs in your
TrainJob without modifying the TrainingRuntime. This is useful when you need to apply job-specific
configurations such as custom service accounts, node selectors, tolerations, or additional volumes.
Platform admins can also leverage custom admission mutating webhooks to configure TrainJob overrides
by using this API.

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
    spec:
      serviceAccountName: custom-sa
      nodeSelector:
        accelerator: nvidia-tesla-v100
```

## Configuration Options

The `PodTemplateOverrides` API supports various configuration options to customize Pod behavior.
You can specify multiple overrides in the array, with later entries taking precedence over earlier ones.
The overrides are applied in the following priority order: TrainJob (e.g. ML policy) > PodTemplateOverrides[n] > PodTemplateOverrides[n-1] > ... > PodTemplateOverrides[0] > TrainingRuntime, where n is the number of PodTemplateOverrides.

### TargetJobs

Specifies which jobs in the TrainingRuntime to apply the overrides to. Common target job names include:

- `node` - The main training node job
- `dataset-initializer` - The dataset initialization job
- `model-initializer` - The model initialization job

### Metadata Overrides

Override or merge Pod metadata such as labels and annotations:

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
    metadata:
      labels:
        team: ml-platform
      annotations:
        monitoring: enabled
```

### Spec Overrides

The `spec` field supports overriding various Pod specification fields including:

- **serviceAccountName** - Override the service account
- **nodeSelector** - Select specific nodes for placement
- **affinity** - Define Pod affinity and anti-affinity rules
- **tolerations** - Allow Pods to schedule on nodes with matching taints
- **volumes** - Add or override volume configurations
- **containers** - Override environment variables and volume mounts
- **schedulingGates** - Control when Pods are scheduled
- **imagePullSecrets** - Specify secrets for pulling private images

## Common Use Cases

The following examples demonstrate practical scenarios where PodTemplateOverrides can be used to customize training job behavior for specific requirements.

### Custom Service Account and Node Selection

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
    spec:
      serviceAccountName: ml-training-sa
      nodeSelector:
        accelerator: nvidia-tesla-v100
        node-pool: gpu-training
```

### Adding Persistent Storage

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
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

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
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

You cannot set environment variables for the following special containers using `PodTemplateOverrides`:

- `node` - Use the `Trainer` API instead
- `dataset-initializer` - Use the `Initializer.Dataset` API instead
- `model-initializer` - Use the `Initializer.Model` API instead

For these containers, use the appropriate dedicated APIs in the TrainJob specification.

Users also can't override `command`, `args`, `image`, and `resources` for the Trainer container in the `node` replicatedJob using `PodTemplateOverrides`.
