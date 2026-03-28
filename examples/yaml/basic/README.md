# Basic YAML Examples

Simple examples to get started with Kubeflow Trainer using `kubectl`.

## Examples

### Multi-Node Distributed Training (`01-multi-node.yaml`)

Demonstrates multi-node distributed training using the `torch-distributed` ClusterTrainingRuntime.

**What it shows:**
- Multi-node configuration with `numNodes`
- Using the default runtime image (no custom image needed)
- `PET_*` environment variables set by the trainer controller

**Run it:**
```bash
kubectl apply -f 01-multi-node.yaml
kubectl get trainjobs multi-node-example
kubectl logs -l trainer.kubeflow.org/job-name=multi-node-example
kubectl delete trainjob multi-node-example
```

## Next Steps

- Try [advanced examples](../advanced/) for production patterns
- See the [Kubeflow Trainer docs](https://www.kubeflow.org/docs/components/trainer/) for full documentation
