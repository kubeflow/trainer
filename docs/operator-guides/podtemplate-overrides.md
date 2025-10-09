# PodTemplateOverrides Operator Guide

This guide explains how to use the `PodTemplateOverrides` API in Kubeflow Trainer to customize Pod configurations for your training jobs.

## Overview

The `PodTemplateOverrides` API allows you to customize Pod templates for specific jobs in your TrainJob without modifying the TrainingRuntime. This is useful when you need to apply job-specific configurations such as:

- Custom service accounts
- Node selectors and affinity rules
- Tolerations for specialized hardware
- Additional volumes and volume mounts
- Environment variables for specific containers
- Scheduling gates and image pull secrets

The overrides are applied on top of the TrainingRuntime configuration, allowing you to maintain reusable runtime templates while still providing job-specific customizations.

## API Structure

The `PodTemplateOverrides` field is part of the `TrainJobSpec` and accepts an array of override configurations:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-train-job
spec:
  runtimeRef:
    name: my-training-runtime
  podTemplateOverrides:
    - targetJobs:
        - name: <job-name>
      metadata:
        labels: {...}
        annotations: {...}
      spec:
        serviceAccountName: <service-account>
        nodeSelector: {...}
        affinity: {...}
        tolerations: [...]
        volumes: [...]
        initContainers: [...]
        containers: [...]
        schedulingGates: [...]
        imagePullSecrets: [...]
```

### Key Components

#### TargetJobs

Specifies which jobs in the TrainingRuntime to apply the overrides to. Common target job names include:

- `node` - The main training node job
- `dataset-initializer` - The dataset initialization job
- `model-initializer` - The model initialization job

```yaml
targetJobs:
  - name: node
```

#### Metadata Overrides

Override or merge Pod metadata such as labels and annotations:

```yaml
metadata:
  labels:
    custom-label: custom-value
    team: ml-platform
  annotations:
    custom-annotation: custom-value
    monitoring: enabled
```

#### Spec Overrides

The `spec` field supports the following overrides:

- **serviceAccountName**: Override the service account used by the Pod
- **nodeSelector**: Select specific nodes for Pod placement
- **affinity**: Define Pod affinity and anti-affinity rules
- **tolerations**: Allow Pods to schedule on nodes with matching taints
- **volumes**: Add or override volume configurations
- **initContainers**: Override environment variables and volume mounts for init containers
- **containers**: Override environment variables and volume mounts for main containers
- **schedulingGates**: Control when Pods are scheduled
- **imagePullSecrets**: Specify secrets for pulling private container images

## Use Cases and Examples

### Example 1: Custom Service Account and Node Selector

This example shows how to use a custom service account and schedule Pods on GPU nodes:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-distributed-custom
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/myorg/custom-training:latest
  podTemplateOverrides:
    - targetJobs:
        - name: node
      spec:
        serviceAccountName: ml-training-sa
        nodeSelector:
          accelerator: nvidia-tesla-v100
          node-pool: gpu-training
```

### Example 2: Adding Volumes and Volume Mounts

Add a persistent volume for storing training data and mount it in the trainer container:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-with-persistent-storage
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/myorg/custom-training:latest
  podTemplateOverrides:
    - targetJobs:
        - name: node
      spec:
        volumes:
          - name: training-data
            persistentVolumeClaim:
              claimName: ml-team-training-pvc
          - name: model-cache
            emptyDir: {}
        containers:
          - name: trainer
            volumeMounts:
              - name: training-data
                mountPath: /workspace/data
              - name: model-cache
                mountPath: /workspace/cache
```

### Example 3: Environment Variables for Init Containers

Override environment variables in init containers for custom initialization logic:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-custom-init
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/myorg/custom-training:latest
  podTemplateOverrides:
    - targetJobs:
        - name: node
      spec:
        initContainers:
          - name: fetch-identity
            env:
              - name: USER_ID
                value: "12345"
              - name: WORKSPACE_ID
                value: "ml-team-workspace"
              - name: FETCH_TIMEOUT
                value: "300"
```

### Example 4: Tolerations for Specialized Hardware

Schedule training jobs on nodes with specific taints (e.g., expensive GPU nodes):

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-gpu-toleration
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/myorg/custom-training:latest
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

### Example 5: Pod Affinity Rules

Ensure training Pods are co-located on the same node or spread across different zones:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-with-affinity
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/myorg/custom-training:latest
  podTemplateOverrides:
    - targetJobs:
        - name: node
      spec:
        affinity:
          podAntiAffinity:
            preferredDuringSchedulingIgnoredDuringExecution:
              - weight: 100
                podAffinityTerm:
                  labelSelector:
                    matchExpressions:
                      - key: training.kubeflow.org/job-name
                        operator: In
                        values:
                          - pytorch-with-affinity
                  topologyKey: kubernetes.io/hostname
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                    - key: node.kubernetes.io/instance-type
                      operator: In
                      values:
                        - p3.8xlarge
                        - p3.16xlarge
```

### Example 6: Image Pull Secrets for Private Registries

Specify secrets for pulling images from private container registries:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-private-registry
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: private-registry.company.com/ml/training:v2.0
  podTemplateOverrides:
    - targetJobs:
        - name: node
      spec:
        imagePullSecrets:
          - name: private-registry-secret
          - name: backup-registry-secret
```

### Example 7: Multiple Overrides for Different Jobs

Apply different overrides to initializer and training nodes:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-multi-override
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/myorg/custom-training:latest
  podTemplateOverrides:
    # Override for dataset initializer
    - targetJobs:
        - name: dataset-initializer
      spec:
        serviceAccountName: dataset-reader-sa
        initContainers:
          - name: fetch-identity
            env:
              - name: DATA_SOURCE
                value: s3://my-bucket/datasets
        volumes:
          - name: dataset-cache
            emptyDir:
              sizeLimit: 10Gi
        containers:
          - name: dataset-initializer
            volumeMounts:
              - name: dataset-cache
                mountPath: /cache
    # Override for training nodes
    - targetJobs:
        - name: node
      spec:
        serviceAccountName: model-trainer-sa
        nodeSelector:
          gpu-type: nvidia-a100
        tolerations:
          - key: dedicated
            operator: Equal
            value: training
            effect: NoSchedule
        volumes:
          - name: model-output
            persistentVolumeClaim:
              claimName: model-storage-pvc
        containers:
          - name: trainer
            volumeMounts:
              - name: model-output
                mountPath: /workspace/output
```

### Example 8: Scheduling Gates for Custom Admission Control

Use scheduling gates to control when Pods are scheduled (requires Kubernetes 1.27+):

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-with-gates
  namespace: ml-team
spec:
  runtimeRef:
    name: pytorch-distributed-gpu
  trainer:
    image: docker.io/myorg/custom-training:latest
  podTemplateOverrides:
    - targetJobs:
        - name: node
      spec:
        schedulingGates:
          - name: example.com/data-validation
          - name: example.com/quota-check
```

## Override Behavior

### Merge Strategy

When multiple overrides target the same job, **later entries in the array override earlier values**. This allows you to compose overrides incrementally:

```yaml
podTemplateOverrides:
  # First override sets service account
  - targetJobs:
      - name: node
    spec:
      serviceAccountName: base-sa
  # Second override changes service account (this takes precedence)
  - targetJobs:
      - name: node
    spec:
      serviceAccountName: custom-sa  # This will be the final value
```

### Field-Level Merging

For most fields, the override values are **merged** with the TrainingRuntime values:

- **Labels and Annotations**: Merged (new keys added, existing keys overridden)
- **Environment Variables**: Merged by name (existing vars with same name are overridden)
- **Volumes**: Merged by name (volumes with same name are overridden)
- **VolumeMounts**: Merged by name (mounts with same name are overridden)

For replacement fields, the entire value is replaced:

- **serviceAccountName**: Replaced entirely
- **nodeSelector**: Replaced entirely
- **affinity**: Replaced entirely
- **tolerations**: Replaced entirely (array is not merged)
- **schedulingGates**: Replaced entirely
- **imagePullSecrets**: Replaced entirely

## Restrictions and Limitations

### Container Name Restrictions

You **cannot** set environment variables for the following special containers using `PodTemplateOverrides`:

- `node` - Use the `Trainer` API instead
- `dataset-initializer` - Use the `Initializer.Dataset` API instead
- `model-initializer` - Use the `Initializer.Model` API instead

For these containers, use the appropriate dedicated APIs:

```yaml
spec:
  # Use Trainer API for main training container
  trainer:
    image: docker.io/myorg/training:latest
    env:
      - name: TRAINING_ENV_VAR
        value: value
  
  # Use Initializer API for initializer containers
  initializer:
    dataset:
      env:
        - name: DATASET_ENV_VAR
          value: value
    model:
      env:
        - name: MODEL_ENV_VAR
          value: value
```

### Validation

The webhook validates that:

1. **TargetJob names exist** in the TrainingRuntime
2. **Container names exist** in the referenced job templates
3. Special container names (`node`, `dataset-initializer`, `model-initializer`) are not used in container overrides

## Best Practices

### 1. Keep Overrides Minimal

Only override what is necessary for the specific job. Let the TrainingRuntime define common configurations:

```yaml
# Good - minimal override
podTemplateOverrides:
  - targetJobs:
      - name: node
    spec:
      nodeSelector:
        gpu-type: nvidia-a100

# Avoid - overriding too much
podTemplateOverrides:
  - targetJobs:
      - name: node
    spec:
      serviceAccountName: custom-sa
      nodeSelector: {...}
      affinity: {...}
      tolerations: [...]
      # ... many other fields
```

### 2. Use Descriptive Names in Labels and Annotations

Add metadata that helps with debugging and monitoring:

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
    metadata:
      labels:
        team: ml-platform
        project: customer-churn
        experiment-id: exp-20250109-001
      annotations:
        description: "Training with hyperparameter set A"
        cost-center: ml-research
```

### 3. Organize Multiple Overrides Logically

When applying multiple overrides, group related configurations together:

```yaml
podTemplateOverrides:
  # All dataset initializer overrides together
  - targetJobs:
      - name: dataset-initializer
    spec:
      serviceAccountName: dataset-sa
      volumes: [...]
  
  # All training node overrides together
  - targetJobs:
      - name: node
    spec:
      serviceAccountName: training-sa
      nodeSelector: {...}
      tolerations: [...]
```

### 4. Document Complex Configurations

For non-obvious configurations, use annotations to document the purpose:

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
    metadata:
      annotations:
        config-purpose: "GPU nodes require special tolerations for cost control"
    spec:
      tolerations:
        - key: high-cost-gpu
          operator: Equal
          value: "true"
          effect: NoSchedule
```

### 5. Test Overrides in Development First

Always test your overrides in a development namespace before applying to production:

```bash
# Apply to dev namespace first
kubectl apply -f trainjob-with-overrides.yaml -n ml-team-dev

# Verify the Pod configuration
kubectl get pod -n ml-team-dev -l training.kubeflow.org/job-name=<job-name> -o yaml

# Check that overrides are applied correctly
kubectl describe pod -n ml-team-dev <pod-name>
```

### 6. Use ConfigMaps and Secrets for Sensitive Data

Instead of hardcoding values, reference ConfigMaps and Secrets:

```yaml
podTemplateOverrides:
  - targetJobs:
      - name: node
    spec:
      containers:
        - name: trainer
          env:
            - name: API_KEY
              valueFrom:
                secretKeyRef:
                  name: training-secrets
                  key: api-key
            - name: CONFIG_FILE
              valueFrom:
                configMapKeyRef:
                  name: training-config
                  key: config.yaml
```

## Troubleshooting

### Override Not Applied

**Problem**: Your override doesn't seem to be applied to the Pods.

**Solution**:
1. Verify the target job name matches the job in your TrainingRuntime
2. Check for validation errors: `kubectl describe trainjob <name>`
3. Ensure later overrides aren't overwriting your configuration
4. Check Pod spec: `kubectl get pod <name> -o yaml`

### Validation Error

**Problem**: You receive a validation error when creating the TrainJob.

**Solution**:
1. Verify target job names exist in the TrainingRuntime
2. Check that container names match containers in the runtime template
3. Ensure you're not trying to override restricted containers (`node`, `dataset-initializer`, `model-initializer`) with container overrides
4. Check the error message: `kubectl describe trainjob <name>`

### Pods Not Scheduling

**Problem**: Pods remain in Pending state after applying overrides.

**Solution**:
1. Check if node selectors or affinity rules are too restrictive
2. Verify tolerations match the node taints
3. Ensure requested resources are available: `kubectl describe pod <name>`
4. Check scheduling gates are being cleared (if used)

### Volume Mount Issues

**Problem**: Volume mounts are not working as expected.

**Solution**:
1. Verify the volume is defined in the `volumes` section
2. Check that volume names match between volume definition and volume mount
3. Ensure PVCs or other volume sources exist and are accessible
4. Verify permissions on mounted volumes

## Related Documentation

- [TrainJob API Reference](https://www.kubeflow.org/docs/components/trainer/api-reference/trainjob/)
- [TrainingRuntime Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/training-runtime/)
- [Kubeflow Trainer Installation](https://www.kubeflow.org/docs/components/trainer/operator-guides/installation/)

## Additional Resources

- [Kubernetes Pod Affinity and Anti-Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)
- [Kubernetes Tolerations and Taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
- [Kubernetes Volumes](https://kubernetes.io/docs/concepts/storage/volumes/)
- [Kubernetes Scheduling Gates](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-scheduling-readiness/)
