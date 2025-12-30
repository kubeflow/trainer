# Basic YAML Examples

Simple examples to get started with Kubeflow Trainer using kubectl.

## Prerequisites

- Kubernetes cluster with Kubeflow Trainer installed
- kubectl configured to access your cluster
- Default ClusterTrainingRuntimes installed

## Examples

### 1. Hello World (`01-hello-world.yaml`)

The simplest possible TrainJob - just echoes "Hello from Kubeflow Trainer!".

**What you'll learn:**
- Basic TrainJob structure
- Using ClusterTrainingRuntime
- Single-node job execution

**Run it:**
```bash
kubectl apply -f 01-hello-world.yaml
kubectl get trainjobs hello-world
kubectl logs -l trainer.kubeflow.org/job-name=hello-world
kubectl delete trainjob hello-world
```

**No GPU required** ✓

---

### 2. Multi-Node Training (`02-multi-node.yaml`)

Demonstrates multi-node distributed training simulation with 3 nodes.

**What you'll learn:**
- Multi-node configuration (`numNodes: 3`)
- Environment variables for distributed training
- Node coordination

**Run it:**
```bash
kubectl apply -f 02-multi-node.yaml
kubectl get trainjobs multi-node-example
kubectl logs -l trainer.kubeflow.org/job-name=multi-node-example -f
kubectl delete trainjob multi-node-example
```

**No GPU required** ✓

---

### 3. Custom TrainingRuntime (`03-with-runtime.yaml`)

Shows how to create and use a namespace-scoped TrainingRuntime.

**What you'll learn:**
- Creating custom TrainingRuntime (namespace-scoped)
- Difference between TrainingRuntime and ClusterTrainingRuntime
- Custom container images and commands

**Run it:**
```bash
kubectl apply -f 03-with-runtime.yaml
kubectl get trainingruntimes custom-torch-runtime
kubectl get trainjobs custom-runtime-example
kubectl logs -l trainer.kubeflow.org/job-name=custom-runtime-example
kubectl delete trainjob custom-runtime-example
kubectl delete trainingruntime custom-torch-runtime
```

**No GPU required** ✓

---

### 4. PyTorch MNIST Training (`04-pytorch-simple.yaml`)

Real PyTorch training job using the MNIST dataset.

**What you'll learn:**
- Real ML training workload
- PyTorch integration
- Training parameters
- GPU support (optional)

**Run it:**
```bash
kubectl apply -f 04-pytorch-simple.yaml
kubectl get trainjobs pytorch-mnist
kubectl logs -l trainer.kubeflow.org/job-name=pytorch-mnist -f
kubectl delete trainjob pytorch-mnist
```

**GPU optional** - Works with or without GPU

---

## Quick Reference

### Common Commands

```bash
# List all TrainJobs
kubectl get trainjobs

# Watch job progress
kubectl get trainjobs -w

# View job details
kubectl describe trainjob <job-name>

# View logs (all pods)
kubectl logs -l trainer.kubeflow.org/job-name=<job-name>

# Follow logs
kubectl logs -f -l trainer.kubeflow.org/job-name=<job-name>

# Delete a job
kubectl delete trainjob <job-name>
```

### Troubleshooting

**Job stuck in Pending?**
```bash
kubectl describe trainjob <job-name>
kubectl get pods -l trainer.kubeflow.org/job-name=<job-name>
kubectl describe pod <pod-name>
```

**Need to see events?**
```bash
kubectl get events --field-selector involvedObject.name=<job-name>
```

**Check runtime availability:**
```bash
kubectl get clustertrainingruntimes
kubectl get trainingruntimes
```

## Next Steps

- Try [advanced examples](../advanced/) for production patterns
- Explore [Python SDK](../../pytorch/) for better developer experience
- Read the [main YAML README](../README.md) for complete documentation

## GPU-Poor? No Problem!

All basic examples (1-3) run perfectly without GPUs. Example 4 works with or without GPU - it will automatically use CPU if GPU is not available.
