# Kubeflow Trainer YAML Examples

Standalone YAML examples for Kubeflow Trainer that can be applied directly with `kubectl`.

## Prerequisites

- Kubernetes cluster with Kubeflow Trainer installed
- `kubectl` configured to access your cluster
- ClusterTrainingRuntimes installed (included with default Kubeflow Trainer installation)

Verify installation:

```bash
kubectl get clustertrainingruntimes
```

## Directory Structure

```
yaml/
├── basic/          # Simple examples for getting started
└── advanced/       # Advanced configurations (scheduling, overrides, etc.)
```

## Examples Overview

### Basic

| File | Description |
|------|-------------|
| [`basic/01-multi-node.yaml`](basic/01-multi-node.yaml) | Multi-node distributed training with torch-distributed runtime |

### Advanced

| File | Description |
|------|-------------|
| [`advanced/01-podspec-overrides.yaml`](advanced/01-podspec-overrides.yaml) | Pod customization with `podTemplateOverrides` |
| [`advanced/02-kueue-integration.yaml`](advanced/02-kueue-integration.yaml) | Job scheduling with Kueue |
| [`advanced/03-volcano-integration.yaml`](advanced/03-volcano-integration.yaml) | Gang scheduling with Volcano |
| [`advanced/04-multi-step.yaml`](advanced/04-multi-step.yaml) | Multi-step pipeline with dataset initialization |

## Quick Start

```bash
# Apply multi-node example
kubectl apply -f basic/01-multi-node.yaml

# Check status
kubectl get trainjobs

# View logs
kubectl logs -l trainer.kubeflow.org/job-name=multi-node-example

# Clean up
kubectl delete trainjob multi-node-example
```

## Additional Resources

- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)
- [Runtime Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Job Scheduling Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/)
- [Python SDK Examples](../pytorch/)
