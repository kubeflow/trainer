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

### Basic

| File | Description |
|------|-------------|
| [`basic/01-multi-node.yaml`](basic/01-multi-node.yaml) | Multi-node distributed training using the `torch-distributed` runtime |

### Advanced

| File | Description |
|------|-------------|
| [`advanced/01-runtime-patches.yaml`](advanced/01-runtime-patches.yaml) | Pod customization with the `runtimePatches` API (nodeSelector, tolerations, serviceAccountName, labels, annotations) |
| [`advanced/02-kueue-integration.yaml`](advanced/02-kueue-integration.yaml) | Queue-based scheduling with [Kueue](https://kueue.sigs.k8s.io/) |
| [`advanced/03-volcano-integration.yaml`](advanced/03-volcano-integration.yaml) | Gang scheduling with [Volcano](https://volcano.sh/) |
| [`advanced/04-multi-step.yaml`](advanced/04-multi-step.yaml) | Dataset-initializer step running before the trainer |

## Quick start

```bash
kubectl apply -f basic/01-multi-node.yaml
kubectl get trainjob multi-node-example
kubectl get pods -l jobset.sigs.k8s.io/jobset-name=multi-node-example
kubectl logs -l jobset.sigs.k8s.io/jobset-name=multi-node-example
kubectl delete trainjob multi-node-example
```

## See also

- [Runtime guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Job scheduling guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/)
- [Python SDK examples](../pytorch/)
