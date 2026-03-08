# Installation

This guide describes how to install Kubeflow Trainer control plane on a Kubernetes cluster.

You can skip these steps if [the Kubeflow platform](https://www.kubeflow.org/docs/started/installing-kubeflow/)
is already deployed using manifests or package distributions, as it includes Kubeflow Trainer by default.

## Prerequisites

These are the minimal requirements to install Kubeflow Trainer control plane:

- Kubernetes >= 1.31
- `kubectl` >= 1.31

:::{tip}
If you don't have Kubernetes cluster, you can quickly create one locally using [Kind](https://kind.sigs.k8s.io/docs/user/quick-start#installing-with-a-package-manager):

```bash
kind create cluster # or minikube start
```
:::

## Installing the Kubeflow Trainer Controller Manager

Run the following command to deploy a released version of Kubeflow Trainer controller manager:

```bash
export VERSION=v2.1.0
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=${VERSION}"
```

For the latest changes run:

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=master"
```

### Install with Helm Charts

You can install Kubeflow Trainer controller manager using Helm charts:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v}
```

For the latest changes run
([where `48e7a93`](https://github.com/kubeflow/trainer/commit/48e7a93) is the desired commit):

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version 0.0.0-sha-48e7a93
```

Ensure that the JobSet and Trainer controller manager pods are running:

```bash
$ kubectl get pods -n kubeflow-system

NAME                                                  READY   STATUS    RESTARTS   AGE
jobset-controller-manager-54968bd57b-88dk4            2/2     Running   0          65s
kubeflow-trainer-controller-manager-cc6468559-dblnw   1/1     Running   0          65s
```

## Installing the Kubeflow Training Runtimes

Run the following command to deploy a released version of Kubeflow Training Runtimes:

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=${VERSION}"
```

For the latest changes run:

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=master"
```

### Install with Helm Charts

You can also deploy ClusterTrainingRuntimes as part of the Helm installation.

To enable all default runtimes (torch, deepspeed, mlx, jax, torchtune):

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v} \
    --set runtimes.defaultEnabled=true
```

To enable specific runtimes:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v} \
    --set runtimes.torchDistributed.enabled=true \
    --set runtimes.deepspeedDistributed.enabled=true
```

For the available Helm values to configure runtimes, see the
[kubeflow-trainer Helm chart documentation](https://github.com/kubeflow/trainer/tree/master/charts/kubeflow-trainer).

## Next Steps

- How to [migrate from Kubeflow Training Operator v1](migration).
- Explore [the Kubeflow Trainer Runtime guide](runtime).
