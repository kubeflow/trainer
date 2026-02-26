# Local Development with GPU and Gang Scheduling

This guide explains how to set up a local development environment for the Training Operator that supports both GPU passthrough and Gang Scheduling (Volcano).

## Prerequisites

- **Linux Environment** (Fedora/Ubuntu)
- **Docker** (v20.10+) with NVIDIA Container Runtime configured
- **Minikube** (Using Docker driver)
- **NVIDIA GPU** (drivers installed)

## 1. Cluster Setup

Minikube must be started with the Docker driver to support GPU passthrough. The `podman` driver does not currently support the `--gpus` flag.

```bash
minikube start --driver=docker --gpus=all
```

Verify GPU visibility in the node:
```bash
kubectl get node minikube -o jsonpath='{.status.allocatable}'
# Output should contain "nvidia.com/gpu": "1"
```

## 2. Install Volcano Scheduler

The standalone Training Operator manifests do not include a scheduler. For Gang Scheduling to work, you must install Volcano.

```bash
kubectl apply -f https://raw.githubusercontent.com/volcano-sh/volcano/master/installer/volcano-development.yaml
```

Verify that `PodGroup` CRDs are present:
```bash
kubectl get crds | grep podgroups.scheduling.volcano.sh
```

## 3. Install Training Operator with Gang Scheduling Enabled

By default, the standalone installation **does not** enable Gang Scheduling. You must enable it by passing the `--gang-scheduler-name` flag to the operator binary.

### Option A: Patching an Existing Installation
If you have already installed the operator via manifests, patch the deployment:

```bash
kubectl patch deployment training-operator -n kubeflow --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args", "value": ["--gang-scheduler-name=volcano"]}]'
```

### Option B: Verification
Check logs to ensure the operator initialized the Volcano controller:

```bash
kubectl logs -n kubeflow -l control-plane=kubeflow-training-operator
```

## 4. Verifying Gang Scheduling (Deadlock Prevention)

To verify that gang scheduling is active, you can submit a job that requires resources exceeding the cluster capacity.

1. Create a `PyTorchJob` with `spec.schedulerName: volcano`.
2. Ensure the `PodGroup` is created:
   ```bash
   kubectl get podgroups
   ```
3. If resources are insufficient, the PodGroup should remain `Pending`, and **no pods should be created**. This prevents resource deadlocks where partial jobs consume cluster capacity.
