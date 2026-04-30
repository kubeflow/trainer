# Basic YAML Examples

Simple examples to get started with Kubeflow Trainer using `kubectl`.

## Examples

### Multi-Node Distributed Training (`01-multi-node.yaml`)

Multi-node distributed training on the `torch-distributed` ClusterTrainingRuntime, using the runtime's default PyTorch image. Demonstrates the `PET_*` env vars (`PET_NNODES`, `PET_NPROC_PER_NODE`, `PET_NODE_RANK`, `PET_MASTER_ADDR`, `PET_MASTER_PORT`) injected by the trainer controller for `torchrun`.

```bash
kubectl apply -f 01-multi-node.yaml
kubectl get trainjob multi-node-example
kubectl logs -l jobset.sigs.k8s.io/jobset-name=multi-node-example
kubectl delete trainjob multi-node-example
```

## Next steps

- [Advanced examples](../advanced/) for production patterns
- [Kubeflow Trainer documentation](https://www.kubeflow.org/docs/components/trainer/)
