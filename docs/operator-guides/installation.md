# Installation

This guide covers installing Kubeflow Trainer on Kubernetes clusters using kubectl or Helm.

## Prerequisites

Before installing Kubeflow Trainer, ensure your environment meets the following requirements:

- **Kubernetes 1.31 or later** - A running Kubernetes cluster
- **kubectl 1.31 or later** - Configured to access your cluster
- **Cluster admin access** - Required for installing CRDs and cluster-scoped resources

:::{note}
For local development and testing, you can use [Kind](https://kind.sigs.k8s.io/) or [Minikube](https://minikube.sigs.k8s.io/) to create a lightweight Kubernetes cluster.
:::

## Installation Methods

Kubeflow Trainer can be installed using either kubectl with Kustomize or Helm charts. Choose the method that best fits your workflow.

### Method 1: kubectl with Kustomize

This method uses Kustomize overlays to deploy Kubeflow Trainer directly from the GitHub repository.

#### Install Controller Manager

Deploy the controller manager for a specific release version:

```bash
export VERSION=v2.1.0
kubectl apply --server-side -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=${VERSION}"
```

:::{tip}
To install the latest development version, use `ref=master` instead of a version tag:

```bash
kubectl apply --server-side -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=master"
```
:::

#### Verify Controller Installation

Check that the controller manager is running:

```bash
kubectl get pods -n kubeflow-system
```

You should see output similar to:

```
NAME                                        READY   STATUS    RESTARTS   AGE
trainer-controller-manager-xxxxx-yyyyy      1/1     Running   0          1m
```

### Method 2: Helm Charts

Helm provides a more flexible installation method with customizable configuration options.

#### Install Controller Manager

Install the Kubeflow Trainer controller using Helm:

```bash
export VERSION=v2.1.0
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v}
```

:::{note}
The `${VERSION#v}` syntax removes the `v` prefix from the version string, as Helm chart versions don't include it.
:::

#### Common Helm Configuration Options

You can customize the installation using Helm values. Here are some common options:

**Custom Controller Image:**
```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --set image.repository=myregistry/trainer-controller \
    --set image.tag=custom-tag
```

**Resource Limits:**
```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --set resources.limits.cpu=500m \
    --set resources.limits.memory=512Mi
```

**Enable Metrics:**
```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --set metrics.enabled=true
```

## Installing Training Runtimes

After installing the controller, you need to install the training runtimes that define how different ML frameworks execute.

### Using kubectl

Deploy all default runtimes:

```bash
export VERSION=v2.1.0
kubectl apply --server-side -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=${VERSION}"
```

This installs the following cluster-wide runtimes:
- **torch-distributed** - PyTorch distributed training
- **deepspeed-distributed** - DeepSpeed training
- **mlx-distributed** - MLX training (Apple Silicon)
- **jax-distributed** - JAX distributed training
- **torchtune-llama** - TorchTune fine-tuning for Llama models

### Using Helm

#### Enable All Default Runtimes

Install the controller with all default runtimes enabled:

```bash
export VERSION=v2.1.0
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v} \
    --set runtimes.defaultEnabled=true
```

#### Enable Specific Runtimes

Install only the runtimes you need:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v} \
    --set runtimes.torchDistributed.enabled=true \
    --set runtimes.deepspeedDistributed.enabled=true \
    --set runtimes.mlxDistributed.enabled=false
```

Available runtime flags:
- `runtimes.torchDistributed.enabled`
- `runtimes.deepspeedDistributed.enabled`
- `runtimes.mlxDistributed.enabled`
- `runtimes.jaxDistributed.enabled`
- `runtimes.torchtuneEnabled`

## Verification

After installation, verify that both the controller and runtimes are properly configured.

### Verify Controller Status

Check that the Kubeflow Trainer controller is running:

```bash
kubectl get pods -n kubeflow-system
```

Expected output:
```
NAME                                        READY   STATUS    RESTARTS   AGE
trainer-controller-manager-xxxxx-yyyyy      1/1     Running   0          5m
```

### Verify CRDs are Installed

Confirm that the Custom Resource Definitions are registered:

```bash
kubectl get crds | grep trainer.kubeflow.org
```

Expected output:
```
clustertrainingruntimes.trainer.kubeflow.org
trainjobs.trainer.kubeflow.org
trainingruntimes.trainer.kubeflow.org
```

### Verify Runtimes are Available

List the installed cluster training runtimes:

```bash
kubectl get clustertrainingruntimes
```

Expected output:
```
NAME                       AGE
torch-distributed          5m
deepspeed-distributed      5m
mlx-distributed            5m
jax-distributed            5m
torchtune-llama3.2-1b      5m
torchtune-llama3.2-3b      5m
```

### Test with a Sample TrainJob

Create a simple test job to verify the installation:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: test-pytorch
  namespace: default
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 1
    image: docker.io/pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime
    command:
      - python
      - -c
      - |
        import torch
        import torch.distributed as dist
        dist.init_process_group(backend="gloo")
        print(f"PyTorch version: {torch.__version__}")
        print(f"Rank: {dist.get_rank()}, World size: {dist.get_world_size()}")
        dist.destroy_process_group()
EOF
```

Check the job status:

```bash
kubectl get trainjob test-pytorch
```

View the logs:

```bash
kubectl logs -l trainer.kubeflow.org/job-name=test-pytorch
```

Clean up the test job:

```bash
kubectl delete trainjob test-pytorch
```

## Configuration Options

### Controller Configuration

The controller can be configured through Helm values or by modifying the deployment directly.

**Enable Webhook (Helm):**
```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --set webhook.enabled=true
```

**Configure Log Level:**
```bash
--set controller.logLevel=debug
```

**Set Leader Election:**
```bash
--set controller.leaderElection.enabled=true \
--set controller.leaderElection.resourceName=trainer-controller-lock
```

### Network Policies

For production environments, consider adding network policies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: trainer-controller
  namespace: kubeflow-system
spec:
  podSelector:
    matchLabels:
      control-plane: trainer-controller-manager
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector: {}
      ports:
        - protocol: TCP
          port: 9443  # Webhook port
  egress:
    - to:
        - namespaceSelector: {}
```

## Uninstallation

### Using kubectl

Remove the controller and runtimes:

```bash
export VERSION=v2.1.0

# Remove runtimes
kubectl delete -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=${VERSION}"

# Remove controller
kubectl delete -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=${VERSION}"
```

:::{warning}
This will delete all TrainJobs and associated resources. Ensure you have backed up any important training jobs before proceeding.
:::

### Using Helm

Uninstall the Helm release:

```bash
helm uninstall kubeflow-trainer -n kubeflow-system
```

### Clean Up CRDs

Helm does not automatically remove CRDs. To completely remove Kubeflow Trainer:

```bash
kubectl delete crd trainjobs.trainer.kubeflow.org
kubectl delete crd trainingruntimes.trainer.kubeflow.org
kubectl delete crd clustertrainingruntimes.trainer.kubeflow.org
```

:::{danger}
Deleting CRDs will permanently remove all TrainJob, TrainingRuntime, and ClusterTrainingRuntime resources from your cluster. This action cannot be undone.
:::

## Integration with Kubeflow Platform

:::{note}
If you have the full Kubeflow platform installed using manifests or package distributions, Kubeflow Trainer is included by default. You can skip the installation steps above.
:::

To verify Trainer is included in your Kubeflow installation:

```bash
kubectl get pods -n kubeflow | grep trainer
```

## Troubleshooting

### Controller Pod Not Starting

Check the pod events and logs:

```bash
kubectl describe pod -n kubeflow-system -l control-plane=trainer-controller-manager
kubectl logs -n kubeflow-system -l control-plane=trainer-controller-manager
```

Common issues:
- **Image pull errors**: Check your registry credentials and network connectivity
- **RBAC errors**: Ensure you have cluster admin permissions
- **Resource constraints**: Verify the node has sufficient CPU and memory

### CRDs Not Installed

If CRDs are missing, manually apply them:

```bash
kubectl apply --server-side -k \
  "https://github.com/kubeflow/trainer.git/manifests/base/crds?ref=${VERSION}"
```

### Runtime Not Found

List available runtimes:

```bash
kubectl get clustertrainingruntimes -A
kubectl get trainingruntimes -A
```

If runtimes are missing, reinstall them using the runtime installation steps above.

### Version Compatibility

Ensure your kubectl version matches or exceeds your Kubernetes cluster version:

```bash
kubectl version --short
```

## Next Steps

Now that Kubeflow Trainer is installed, you can:

- **Create custom runtimes** - See [Training Runtimes](runtime)
- **Configure ML policies** - See [ML Policies](ml-policy)
- **Set up job scheduling** - See [Job Scheduling](job-scheduling/index)
- **Submit training jobs** - See [User Guides](../user-guides/index)
