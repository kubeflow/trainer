# Kubeflow Trainer YAML Examples

Standalone manifests that can be applied directly with `kubectl`. For an end-to-end conceptual walkthrough, see the [Kubeflow Trainer documentation](https://www.kubeflow.org/docs/components/trainer/).

## Prerequisites

- Kubernetes cluster with Kubeflow Trainer installed
- `kubectl` configured against the cluster
- The default ClusterTrainingRuntimes installed (shipped with Kubeflow Trainer):

  ```bash
  kubectl get clustertrainingruntimes
  ```

## Examples

| File | Description |
|------|-------------|
| [`01-multi-node.yaml`](01-multi-node.yaml) | Multi-node distributed PyTorch training launched with `torchrun` |
| [`02-runtime-patches.yaml`](02-runtime-patches.yaml) | Pod customization with the `runtimePatches` API (nodeSelector, tolerations, serviceAccountName, labels, annotations) |
| [`03-kueue-integration.yaml`](03-kueue-integration.yaml) | Queue-based scheduling with [Kueue](https://kueue.sigs.k8s.io/) |
| [`04-volcano-integration.yaml`](04-volcano-integration.yaml) | Gang scheduling with [Volcano](https://volcano.sh/) |
| [`05-multi-step.yaml`](05-multi-step.yaml) | Dataset-initializer step running before the trainer |

## Quick start

```bash
kubectl apply -f 01-multi-node.yaml
kubectl get trainjob multi-node-example
kubectl get pods -l jobset.sigs.k8s.io/jobset-name=multi-node-example
kubectl logs -l jobset.sigs.k8s.io/jobset-name=multi-node-example
kubectl delete trainjob multi-node-example
```

## See also

- [Runtime guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Job scheduling guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/)
- [Python SDK examples](../pytorch/)
