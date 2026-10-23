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

You can enable the default ClusterTrainingRuntimes together with the control plane in a single
step. A post-install Helm hook applies the runtimes once the CRDs and controller are ready:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v} \
    --set runtimes.defaultEnabled=true
```

To enable specific runtimes instead of all of them:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --namespace kubeflow-system \
    --create-namespace \
    --version ${VERSION#v} \
    --set runtimes.torchDistributed.enabled=true \
    --set runtimes.deepspeedDistributed.enabled=true
```

You can also enable runtimes on an existing installation with `helm upgrade` using the same
`--set` flags. The hook reconciles runtimes on every upgrade: newly enabled runtimes are applied
and disabled ones are removed. Disabling *all* runtimes removes the installer itself, so in that
case delete any remaining runtimes manually or with `helm uninstall`.

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

## TrainJob network isolation

Isolation is disabled by default. To enable it, set the following controller
Configuration (under `manager.config` in Helm values). For Kustomize, patch
`manifests/base/manager/controller_manager_config.yaml` through your overlay.
Restart the controller after configuration changes.

```yaml
networkPolicy:
  enabled: true
  ingress: []
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
```

This example allows CoreDNS. Add standard Kubernetes NetworkPolicy rules for
runtime dependencies: actual API-server IPs/ports, approved storage/download
IPs, status-server endpoints, monitoring, shared caches and service meshes.
NodeLocal DNS and Service NAT may require different destinations. Standard
NetworkPolicy cannot allow DNS names; allowing all TCP 443 is not API-only access.
Shared caches need matching ingress rules if independently isolated.

The controller creates two policies per job: `trainjob-<UID>` selects JobSet
pods (`jobset.sigs.k8s.io/jobset-name: <name>`), and `trainjob-cache-<UID>` selects
private cache pods (`leaderworkerset.sigs.k8s.io/name: <name>-cache`). Both allow
same-namespace traffic within that job and its cache, plus administrator rules.
A cache name longer than 63 characters cannot be a label value, so its policy
is omitted. Exceptions apply to every job and cache; there is no tenant opt-out
or TrainJob API/SDK change. An empty rule allows all traffic in that direction.

Policies precede new workloads; invalid rules or creation failures block new
workloads through normal controller errors and Events. Active jobs, including
already-running jobs, have edited/deleted policies repaired. Finished jobs keep
existing policies until TrainJob deletion; enabling does not create policies for
already-finished jobs. Disabling and restarting removes owned policies, including
for finished jobs. Conflicting foreign policies are never adopted or deleted.

Use a NetworkPolicy-capable CNI and verify allowed and denied connections.
Policies are additive and propagation is not atomic; existing connections are
CNI-specific. Prevent tenants from spoofing controller pod labels, modifying
operator configuration, or creating bypass policies. This is network isolation,
not authentication, encryption, or a boundary against namespace administrators.

Run the real-network regression on a disposable cluster configured with
`test/e2e/testdata/network-policy-values.yaml` (HTTP workloads and cache-label
fixtures, no GPUs):

```sh
TRAINER_NETWORK_POLICY_E2E=true go test ./test/e2e \
  -ginkgo.focus='TrainJob network isolation' -ginkgo.v
```

## Next Steps

- How to [migrate from Kubeflow Training Operator v1](migration).
- Explore [the Kubeflow Trainer Runtime guide](runtime).
