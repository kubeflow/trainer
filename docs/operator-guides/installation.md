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

:::git

## Installing the Kubeflow Trainer Controller Manager

Run the following command to deploy a released version of Kubeflow Trainer control plane:

```bash
export VERSION=v2.1.0
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

:::{note}
The Trainer CRDs (`TrainJob`, `TrainingRuntime`, `ClusterTrainingRuntime`) are installed by the chart by default.
If you manage the CRDs out-of-band (previously via Helm's `--skip-crds` flag), set `--set crds.enabled=false` to skip
installing them with the chart.
:::

You can also deploy the default ClusterTrainingRuntimes after installing the control plane with the
following command:

```bash
helm upgrade kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --set runtimes.defaultEnabled=true \
    --version ${VERSION#v}
```

To enable specific runtimes:

```bash
helm upgrade kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --version ${VERSION#v} \
    --set runtimes.torchDistributed.enabled=true \
    --set runtimes.deepspeedDistributed.enabled=true
```

For the available Helm values to configure runtimes, see the
[kubeflow-trainer Helm chart documentation](https://github.com/kubeflow/trainer/tree/master/charts/kubeflow-trainer).

## Install with Kustomize

Run the following command to deploy Kubeflow Trainer control plane with kustomize:

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=${VERSION}"
```

For the latest changes run:

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=master"
```

Run the following command to deploy Kubeflow Trainer built-in runtimes

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=master"
```

## Verify the Control Plane

Ensure that the JobSet and Trainer controller manager pods are running:

```bash
$ kubectl get pods -n kubeflow-system

NAME                                                  READY   STATUS    RESTARTS   AGE
jobset-controller-manager-54968bd57b-88dk4            2/2     Running   0          65s
kubeflow-trainer-controller-manager-cc6468559-dblnw   1/1     Running   0          65s
```

## Next Steps

- How to [migrate from Kubeflow Training Operator v1](migration).
- Explore [the Kubeflow Trainer Runtime guide](runtime).
