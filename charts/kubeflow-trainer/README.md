# kubeflow-trainer

![Version: 2.0.0](https://img.shields.io/badge/Version-2.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)

A Helm chart for deploying Kubeflow Trainer on Kubernetes.

**Homepage:** <https://github.com/kubeflow/trainer>

## Introduction

This chart bootstraps a [Kubernetes Trainer](https://github.com/kubeflow/trainer) deployment using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Helm >= 3
- Kubernetes >= 1.29

## Usage

### Add Helm Repo

```bash
helm repo add kubeflow-trainer https://kubeflow.github.io/trainer

helm repo update
```

See [helm repo](https://helm.sh/docs/helm/helm_repo) for command documentation.

### Install the chart

```bash
helm install [RELEASE_NAME] kubeflow-trainer/kubeflow-trainer
```

For example, if you want to create a release with name `kubeflow-trainer` in the `kubeflow-system` namespace:

```shell
helm upgrade kubeflow-trainer kubeflow-trainer/kubeflow-trainer \
    --install \
    --namespace kubeflow-system \
    --create-namespace
```

Note that by passing the `--create-namespace` flag to the `helm install` command, `helm` will create the release namespace if it does not exist.
If you have already installed jobset controller/webhook, you can skip installing it by adding `--set jobset.install=false` to the command arguments.

See [helm install](https://helm.sh/docs/helm/helm_install) for command documentation.

### Upgrade the chart

```shell
helm upgrade [RELEASE_NAME] kubeflow-trainer/kubeflow-trainer [flags]
```

See [helm upgrade](https://helm.sh/docs/helm/helm_upgrade) for command documentation.

### Uninstall the chart

```shell
helm uninstall [RELEASE_NAME]
```

This removes all the Kubernetes resources associated with the chart and deletes the release, except for the `crds`, those will have to be removed manually.

See [helm uninstall](https://helm.sh/docs/helm/helm_uninstall) for command documentation.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| nameOverride | string | `""` | String to partially override release name. |
| fullnameOverride | string | `""` | String to fully override release name. |
| jobset.install | bool | `true` | Whether to install jobset as a dependency managed by trainer. This must be set to `false` if jobset controller/webhook has already been installed into the cluster. |
| commonLabels | object | `{}` | Common labels to add to the resources. |
| image.registry | string | `"docker.io"` | Image registry. |
| image.repository | string | `"kubeflow/trainer-controller-manager"` | Image repository. |
| image.tag | string | If not set, the chart version will be used. | Image tag. |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy. |
| image.pullSecrets | list | `[]` | Image pull secrets for private image registry. |
| manager.replicas | int | `1` | Number of replicas of manager. |
| manager.labels | object | `{}` | Extra labels for manager pods. |
| manager.annotations | object | `{}` | Extra annotations for manager pods. |
| manager.volumes | list | `[]` | Volumes for manager pods. |
| manager.nodeSelector | object | `{}` | Node selector for manager pods. |
| manager.affinity | object | `{}` | Affinity for manager pods. |
| manager.tolerations | list | `[]` | List of node taints to tolerate for manager pods. |
| manager.env | list | `[]` | Environment variables for manager containers. |
| manager.envFrom | list | `[]` | Environment variable sources for manager containers. |
| manager.volumeMounts | list | `[]` | Volume mounts for manager containers. |
| manager.resources | object | `{}` | Pod resource requests and limits for manager containers. |
| manager.securityContext | object | `{}` | Security context for manager containers. |
| webhook.failurePolicy | string | `"Fail"` | Specifies how unrecognized errors are handled. Available options are `Ignore` or `Fail`. |

## Maintainers

| Name | Url |
| ---- | --- |
| andreyvelich | <https://github.com/andreyvelich> |
| ChenYi015 | <https://github.com/ChenYi015> |
| gaocegege | <https://github.com/gaocegege> |
| Jeffwan | <https://github.com/Jeffwan> |
| johnugeorge | <https://github.com/johnugeorge> |
| tenzen-y | <https://github.com/tenzen-y> |
| terrytangyuan | <https://github.com/terrytangyuan> |
