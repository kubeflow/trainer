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

### Install the Helm Chart

Install the released version (e.g. 2.1.0):

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer --version 2.1.0
```

Alternatively, you can install the latest version from the master branch (e.g. `bfccb7b` commit):

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer --version 0.0.0-sha-bfccb7b
```

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
| jobset.fullnameOverride | string | `"jobset"` | String to fully override jobset release name. |
| commonLabels | object | `{}` | Common labels to add to the resources. |
| image.registry | string | `"ghcr.io"` | Image registry. |
| image.repository | string | `"kubeflow/trainer/trainer-controller-manager"` | Image repository. |
| image.tag | string | `""` | Image tag. Defaults to the chart appVersion. |
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
| manager.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Security context for manager containers. |
| manager.config | object | `{"certManagement":{"enable":true,"webhookSecretName":"","webhookServiceName":""},"controller":{"groupKindConcurrency":{"clusterTrainingRuntime":1,"trainJob":5,"trainingRuntime":1}},"health":{"healthProbeBindAddress":":8081","livenessEndpointName":"healthz","readinessEndpointName":"readyz"},"leaderElection":{"leaderElect":true,"leaseDuration":"15s","renewDeadline":"10s","resourceName":"trainer.kubeflow.org","resourceNamespace":"","retryPeriod":"2s"},"metrics":{"bindAddress":":8443","secureServing":true},"webhook":{"host":"","port":9443}}` | Controller manager configuration. This configuration is used to generate the ConfigMap for the controller manager. |
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
