# Kubeflow Trainer YAML Examples

This directory contains standalone YAML examples for Kubeflow Trainer that can be applied directly with `kubectl`.

## Prerequisites

- Kubernetes cluster with Kubeflow Trainer installed
- kubectl configured to access your cluster
- ClusterTrainingRuntimes installed (comes with default Kubeflow Trainer installation)

## Directory Structure

```
yaml/
├── basic/          # Simple examples for getting started
└── advanced/       # Advanced configurations (PodSpecOverrides, scheduling, etc.)
```

## Quick Start

### 1. Verify Installation

Check that ClusterTrainingRuntimes are available:

```bash
kubectl get clustertrainingruntimes
```

Expected output should include: `torch-distributed`, `deepspeed-distributed`, `mlx-distributed`

### 2. Run Your First TrainJob

Apply a simple hello-world example:

```bash
kubectl apply -f basic/01-hello-world.yaml
```

Check the status:

```bash
kubectl get trainjobs
kubectl describe trainjob hello-world
```

View logs:

```bash
kubectl logs -l trainer.kubeflow.org/job-name=hello-world
```

Clean up:

```bash
kubectl delete trainjob hello-world
```

## Examples Overview

### Basic Examples

| File | Description | GPU Required |
|------|-------------|--------------|
| `01-hello-world.yaml` | Simple single-node job with echo | No |
| `02-multi-node.yaml` | Multi-node distributed job | No |
| `03-with-runtime.yaml` | Custom TrainingRuntime (namespace-scoped) | No |
| `04-pytorch-simple.yaml` | PyTorch MNIST training | Yes (optional) |

### Advanced Examples

| File | Description | Features |
|------|-------------|----------|
| `01-podspec-overrides.yaml` | Custom resource limits, env vars | PodSpecOverrides |
| `02-kueue-integration.yaml` | Job scheduling with Kueue | Queue management |
| `03-volcano-integration.yaml` | Gang scheduling with Volcano | Gang scheduling |
| `04-multi-step.yaml` | Multi-step training pipeline | Dataset init, training |

## Common Patterns

### Viewing Job Status

```bash
# List all TrainJobs
kubectl get trainjobs

# Describe specific job
kubectl describe trainjob <job-name>

# Watch job progress
kubectl get trainjobs -w

# Get job in different namespace
kubectl get trainjobs -n <namespace>
```

### Viewing Logs

```bash
# View logs for all pods in a TrainJob
kubectl logs -l trainer.kubeflow.org/job-name=<job-name>

# Follow logs
kubectl logs -f -l trainer.kubeflow.org/job-name=<job-name>

# View logs from specific replica
kubectl logs <pod-name>
```

### Debugging

```bash
# Get events for a TrainJob
kubectl get events --field-selector involvedObject.name=<job-name>

# Check pod status
kubectl get pods -l trainer.kubeflow.org/job-name=<job-name>

# Describe a pod for more details
kubectl describe pod <pod-name>
```

### Cleanup

```bash
# Delete a specific TrainJob
kubectl delete trainjob <job-name>

# Delete all TrainJobs in namespace
kubectl delete trainjobs --all

# Delete with YAML file
kubectl delete -f <file>.yaml
```

## Tips for Production

1. **Use namespaces**: Organize jobs by team or project
2. **Set resource limits**: Use PodSpecOverrides to set CPU/memory limits
3. **Add labels**: Use labels for better organization and filtering
4. **Configure retries**: Set appropriate restart policies
5. **Use secrets**: Store credentials in Kubernetes secrets
6. **Monitor resources**: Use resource quotas to prevent overconsumption

## Additional Resources

- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)
- [Runtime Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Migration Guide (v1 to v2)](https://www.kubeflow.org/docs/components/trainer/operator-guides/migration/)
- [Python SDK Examples](../pytorch/) (recommended for AI practitioners)

## Feedback

If you encounter issues or have suggestions for additional examples, please [open an issue](https://github.com/kubeflow/training-operator/issues/new).
