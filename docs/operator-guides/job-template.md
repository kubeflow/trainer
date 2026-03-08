# Job Templates

Job templates define how Kubeflow Trainer orchestrates training workloads using JobSet specifications. They control the structure, dependencies, and execution patterns of training jobs.

## Overview

Job templates in Kubeflow Trainer use the [JobSet API](https://github.com/kubernetes-sigs/jobset) to coordinate multiple replicated jobs. The Trainer controller generates JobSet instances based on:

- TrainJob specification
- TrainingRuntime template
- ML policy configuration
- Dataset and model initializers

## JobSet Structure

A runtime's template defines one or more replicated jobs:

```yaml
spec:
  template:
    spec:
      replicatedJobs:
        - name: node
          replicas: 1  # Overridden by ML policy numNodes
          template:
            spec:
              completions: 1
              parallelism: 1
              template:
                metadata:
                  labels:
                    trainer.kubeflow.org/job-role: trainer
                spec:
                  restartPolicy: Never
                  containers:
                    - name: trainer
                      image: pytorch/pytorch:2.5.1
                      command: ["torchrun", "train.py"]
```

## Required Ancestor Labels

Each replicated job must include an ancestor label for value injection:

### Trainer Jobs

```yaml
metadata:
  labels:
    trainer.kubeflow.org/trainjob-ancestor-step: trainer
```

This label enables the controller to inject values from the TrainJob's `trainer` section into the corresponding replicated job.

### Dataset Initializers

```yaml
metadata:
  labels:
    trainer.kubeflow.org/trainjob-ancestor-step: dataset-initializer
```

Used for jobs that initialize datasets before training.

### Model Initializers

```yaml
metadata:
  labels:
    trainer.kubeflow.org/trainjob-ancestor-step: model-initializer
```

Used for jobs that download or prepare models before training.

## Complete Runtime Example

Here's a complete runtime with dataset initializer, model initializer, and trainer:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-with-initializers
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        # Dataset Initializer
        - name: dataset-initializer
          replicas: 1
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: dataset-initializer
            spec:
              template:
                spec:
                  restartPolicy: Never
                  containers:
                    - name: dataset-initializer
                      image: kubeflow/dataset-initializer:latest
                      volumeMounts:
                        - name: dataset
                          mountPath: /data
                  volumes:
                    - name: dataset
                      persistentVolumeClaim:
                        claimName: shared-dataset

        # Model Initializer
        - name: model-initializer
          replicas: 1
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: model-initializer
            spec:
              template:
                spec:
                  restartPolicy: Never
                  containers:
                    - name: model-initializer
                      image: kubeflow/model-initializer:latest
                      volumeMounts:
                        - name: model
                          mountPath: /model
                  volumes:
                    - name: model
                      persistentVolumeClaim:
                        claimName: shared-model

        # Training Job
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  restartPolicy: OnFailure
                  containers:
                    - name: trainer
                      volumeMounts:
                        - name: dataset
                          mountPath: /data
                          readOnly: true
                        - name: model
                          mountPath: /model
                          readOnly: true
                  volumes:
                    - name: dataset
                      persistentVolumeClaim:
                        claimName: shared-dataset
                    - name: model
                      persistentVolumeClaim:
                        claimName: shared-model
```

## Job Dependencies

JobSet supports dependencies between replicated jobs using `successPolicy`:

```yaml
spec:
  template:
    spec:
      successPolicy:
        operator: All
        targetReplicatedJobs:
          - trainer
      replicatedJobs:
        - name: dataset-initializer
          # Runs first
        - name: trainer
          # Runs after dataset-initializer completes
```

## Resource Configuration

### Per-Container Resources

```yaml
replicatedJobs:
  - name: node
    template:
      spec:
        template:
          spec:
            containers:
              - name: trainer
                resources:
                  requests:
                    cpu: "4"
                    memory: "16Gi"
                    nvidia.com/gpu: "1"
                  limits:
                    cpu: "8"
                    memory: "32Gi"
                    nvidia.com/gpu: "1"
```

### Shared Volumes

```yaml
spec:
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  volumes:
                    - name: dshm
                      emptyDir:
                        medium: Memory
                        sizeLimit: 8Gi
                    - name: dataset
                      persistentVolumeClaim:
                        claimName: training-data
                  containers:
                    - name: trainer
                      volumeMounts:
                        - name: dshm
                          mountPath: /dev/shm
                        - name: dataset
                          mountPath: /data
```

## Advanced Patterns

### Multi-Role Training

Define separate launcher and worker roles:

```yaml
spec:
  template:
    spec:
      replicatedJobs:
        - name: launcher
          replicas: 1
          template:
            metadata:
              labels:
                trainer.kubeflow.org/job-role: launcher
            spec:
              template:
                spec:
                  containers:
                    - name: launcher
                      command: ["mpirun", "-np", "4", "python", "train.py"]

        - name: worker
          replicas: 4
          template:
            metadata:
              labels:
                trainer.kubeflow.org/job-role: worker
            spec:
              template:
                spec:
                  containers:
                    - name: worker
                      command: ["sleep", "infinity"]
```

### Init Containers

```yaml
replicatedJobs:
  - name: node
    template:
      spec:
        template:
          spec:
            initContainers:
              - name: setup
                image: busybox
                command:
                  - sh
                  - -c
                  - |
                    echo "Preparing environment..."
                    mkdir -p /workspace/logs
                volumeMounts:
                  - name: workspace
                    mountPath: /workspace
            containers:
              - name: trainer
                volumeMounts:
                  - name: workspace
                    mountPath: /workspace
            volumes:
              - name: workspace
                emptyDir: {}
```

### Sidecar Containers

```yaml
replicatedJobs:
  - name: node
    template:
      spec:
        template:
          spec:
            containers:
              - name: trainer
                ports:
                  - containerPort: 29500
                    name: master

              - name: metrics-exporter
                image: prom/node-exporter:latest
                ports:
                  - containerPort: 9100
                    name: metrics

              - name: log-shipper
                image: fluent/fluent-bit:latest
                volumeMounts:
                  - name: logs
                    mountPath: /logs
                    readOnly: true
            volumes:
              - name: logs
                emptyDir: {}
```

## Best Practices

### 1. Use Restart Policies Appropriately

```yaml
# Training pods - retry on failure
spec:
  restartPolicy: OnFailure

# Initializer pods - don't retry
spec:
  restartPolicy: Never
```

### 2. Set Resource Limits

```yaml
resources:
  requests:
    cpu: "4"
    memory: "16Gi"
  limits:
    cpu: "8"
    memory: "32Gi"
```

### 3. Use Labels for Organization

```yaml
metadata:
  labels:
    trainer.kubeflow.org/job-role: trainer
    app.kubernetes.io/component: training
    app.kubernetes.io/part-of: ml-pipeline
```

### 4. Configure Shared Memory

```yaml
volumes:
  - name: dshm
    emptyDir:
      medium: Memory
      sizeLimit: 8Gi
```

### 5. Add Health Checks

```yaml
containers:
  - name: trainer
    livenessProbe:
      exec:
        command:
          - python
          - -c
          - "import torch; assert torch.cuda.is_available()"
      initialDelaySeconds: 60
      periodSeconds: 30
```

## Troubleshooting

### Jobs Not Starting

Check the JobSet status:

```bash
kubectl get jobset -l trainer.kubeflow.org/trainjob-name=<job-name>
kubectl describe jobset <jobset-name>
```

### Pods Pending

```bash
kubectl get pods -l trainer.kubeflow.org/job-name=<job-name>
kubectl describe pod <pod-name>
```

Look for resource constraints or scheduling issues.

### Wrong Number of Replicas

Verify ML policy configuration:

```bash
kubectl get trainjob <job-name> -o jsonpath='{.spec.trainer.numNodes}'
```

## Next Steps

- [ML Policies](ml-policy) - Configure ML-specific settings
- [Pod Templates](pod-template) - Override pod configurations
- [Training Runtimes](runtime) - Complete runtime definitions
