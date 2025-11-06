# Advanced YAML Examples

Production-ready examples demonstrating advanced Kubeflow Trainer features.

## Examples

### 1. PodSpec Overrides (`01-podspec-overrides.yaml`)

Comprehensive example of customizing pod specifications.

**Features:**
- Custom resource limits and requests
- Environment variables
- Node selectors and tolerations
- Security context
- Annotations and labels
- Volume mounts

**Use cases:**
- Production deployments
- Resource management
- Custom pod configurations
- Security requirements

**Run it:**
```bash
kubectl apply -f 01-podspec-overrides.yaml
kubectl describe trainjob podspec-example
kubectl delete trainjob podspec-example
```

### 2. Kueue Integration (`02-kueue-integration.yaml`)

Demonstrates job scheduling with Kueue queue manager.

**Features:**
- Queue-based job scheduling
- Resource quota management
- Priority handling
- Fair resource allocation

**Prerequisites:**
- Kueue must be installed
- LocalQueue and ClusterQueue configured

**Run it:**
```bash
kubectl apply -f 02-kueue-integration.yaml
kubectl get trainjobs kueue-example
kubectl delete trainjob kueue-example
```

**Learn more:**
- [Kueue Documentation](https://kueue.sigs.k8s.io/)
- [Kubeflow Trainer Kueue Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/kueue/)

### 3. Volcano Gang Scheduling (`03-volcano-integration.yaml`)

Gang scheduling for multi-node training jobs.

**Features:**
- All-or-nothing pod scheduling
- Prevents resource deadlocks
- Optimized for distributed training
- PodGroup management

**Prerequisites:**
- Volcano scheduler installed

**Run it:**
```bash
kubectl apply -f 03-volcano-integration.yaml
kubectl get trainjobs volcano-example
kubectl get podgroup -l trainer.kubeflow.org/job-name=volcano-example
kubectl delete trainjob volcano-example
```

**Learn more:**
- [Volcano Documentation](https://volcano.sh/)
- [Kubeflow Trainer Volcano Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/volcano/)

### 4. Multi-Step Pipeline (`04-multi-step.yaml`)

Training pipeline with dataset initialization.

**Features:**
- Dataset download and preparation
- Multi-step job execution
- PVC for dataset storage
- Dataset sharing across nodes

**Use cases:**
- Large dataset downloads
- Data preprocessing
- Model initialization
- Complex training pipelines

**Run it:**
```bash
kubectl apply -f 04-multi-step.yaml
# Watch dataset init
kubectl logs -l trainer.kubeflow.org/job-name=multi-step-example,trainer.kubeflow.org/step-name=dataset -f
# Watch training
kubectl logs -l trainer.kubeflow.org/job-name=multi-step-example,trainer.kubeflow.org/step-name=trainer -f
kubectl delete trainjob multi-step-example
```

## Production Best Practices

### Resource Management
```yaml
podSpecOverrides:
  spec:
    containers:
      - name: node
        resources:
          requests:
            cpu: "2000m"
            memory: "4Gi"
            nvidia.com/gpu: 1
          limits:
            cpu: "4000m"
            memory: "8Gi"
            nvidia.com/gpu: 1
```

### Security
```yaml
podSpecOverrides:
  spec:
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
      fsGroup: 1000
    serviceAccountName: training-sa
```

### High Availability
```yaml
podSpecOverrides:
  spec:
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                trainer.kubeflow.org/job-name: my-job
            topologyKey: kubernetes.io/hostname
```

### Monitoring and Logging
```yaml
podSpecOverrides:
  metadata:
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "8080"
  spec:
    containers:
      - name: node
        env:
          - name: PYTHONUNBUFFERED
            value: "1"
          - name: LOG_LEVEL
            value: "INFO"
```

## Next Steps

- Review [basic examples](../basic/) if you're new to Kubeflow Trainer
- Check the [main README](../README.md) for complete documentation
- Explore the [Python SDK](https://www.kubeflow.org/docs/components/trainer/getting-started/) for better developer experience
