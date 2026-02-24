# kubeflow-trainer

![Version: 2.1.0](https://img.shields.io/badge/Version-2.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)

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

### Install with ClusterTrainingRuntimes

You can optionally deploy ClusterTrainingRuntimes as part of the Helm installation. Runtimes are disabled by default to keep the chart lightweight.

To enable all default runtimes (torch, deepspeed, mlx, torchtune):

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
  --version 2.1.0 \
  --set runtimes.defaultEnabled=true
```

To enable specific runtimes:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
  --version 2.1.0 \
  --set runtimes.torchDistributed.enabled=true \
  --set runtimes.deepspeedDistributed.enabled=true
```

Or use a custom values file:

```yaml
# values.yaml
runtimes:
  torchDistributed:
    enabled: true
  deepspeedDistributed:
    enabled: true
  mlxDistributed:
    enabled: true

# For torch-distributed-with-cache, enable both dataCache.enabled and dataCache.runtimes.torchDistributed.enabled
dataCache:
  enabled: true
  cacheImage:
    tag: "v2.0.0"
  runtimes:
    torchDistributedWithCache:
      enabled: true
```

Then install with:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
  --version 2.1.0 \
  -f values.yaml
```

### Available Runtimes

- **torch-distributed**: PyTorch distributed training (no custom images)
- **torch-distributed-with-cache**: PyTorch with distributed data cache support (requires `dataCache.enabled=true`)
- **deepspeed-distributed**: DeepSpeed distributed training with MPI
- **mlx-distributed**: MLX distributed training with MPI

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
| image.tag | string | `""` | Image tag. Defaults to the chart version formatted for the appropriate image tag. |
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
| manager.config | object | `{"certManagement":{"enable":true,"webhookSecretName":"","webhookServiceName":""},"controller":{"groupKindConcurrency":{"clusterTrainingRuntime":1,"trainJob":5,"trainingRuntime":1}},"featureGates":{},"health":{"healthProbeBindAddress":":8081","livenessEndpointName":"healthz","readinessEndpointName":"readyz"},"leaderElection":{"leaderElect":true,"leaseDuration":"15s","renewDeadline":"10s","resourceName":"trainer.kubeflow.org","resourceNamespace":"","retryPeriod":"2s"},"metrics":{"bindAddress":":8443","secureServing":true},"webhook":{"host":"","port":9443}}` | Controller manager configuration. This configuration is used to generate the ConfigMap for the controller manager. |
| webhook.failurePolicy | string | `"Fail"` | Specifies how unrecognized errors are handled. Available options are `Ignore` or `Fail`. |
| dataCache.enabled | bool | `false` | Enable/disable data cache support (LWS dependency, ClusterRole). Set to `true` to install data cache components. |
| dataCache.lws.install | bool | `true` | Whether to install LeaderWorkerSet as a dependency. Set to `false` if LeaderWorkerSet is already installed in the cluster. |
| dataCache.lws.fullnameOverride | string | `"lws"` | String to fully override LeaderWorkerSet release name. |
| dataCache.cacheImage.registry | string | `"ghcr.io"` | Data cache image registry |
| dataCache.cacheImage.repository | string | `"kubeflow/trainer/data-cache"` | Data cache image repository |
| dataCache.cacheImage.tag | string | `""` | Data cache image tag. Defaults to chart version if empty. |
| dataCache.runtimes.torchDistributedWithCache | object | `{"enabled":false}` | PyTorch distributed training with data cache support |
| dataCache.runtimes.torchDistributedWithCache.enabled | bool | `false` | Enable deployment of torch-distributed-with-cache runtime |
| runtimes | object | `{"deepspeedDistributed":{"enabled":false,"image":{"registry":"ghcr.io","repository":"kubeflow/trainer/deepspeed-runtime","tag":""}},"defaultEnabled":false,"jaxDistributed":{"enabled":false},"mlxDistributed":{"enabled":false,"image":{"registry":"ghcr.io","repository":"kubeflow/trainer/mlx-runtime","tag":""}},"torchDistributed":{"enabled":false},"torchtuneDistributed":{"image":{"registry":"ghcr.io","repository":"kubeflow/trainer/torchtune-trainer","tag":""},"llama3_2_1B":{"enabled":false},"llama3_2_3B":{"enabled":false},"qwen2_5_1_5B":{"enabled":false}}}` | ClusterTrainingRuntimes configuration These are optional runtime templates that can be deployed with the Helm chart. Each runtime provides a blueprint for different ML frameworks and configurations. |
| runtimes.defaultEnabled | bool | `false` | Enable all default runtimes (torch, deepspeed, mlx, jax, torchtune) when set to true. Individual runtime settings will be ignored if this is enabled. |
| runtimes.torchDistributed | object | `{"enabled":false}` | PyTorch distributed training runtime (no custom images required) |
| runtimes.torchDistributed.enabled | bool | `false` | Enable deployment of torch-distributed runtime |
| runtimes.deepspeedDistributed | object | `{"enabled":false,"image":{"registry":"ghcr.io","repository":"kubeflow/trainer/deepspeed-runtime","tag":""}}` | DeepSpeed distributed training runtime |
| runtimes.deepspeedDistributed.enabled | bool | `false` | Enable deployment of deepspeed-distributed runtime |
| runtimes.deepspeedDistributed.image.registry | string | `"ghcr.io"` | DeepSpeed runtime image registry |
| runtimes.deepspeedDistributed.image.repository | string | `"kubeflow/trainer/deepspeed-runtime"` | DeepSpeed runtime image repository |
| runtimes.deepspeedDistributed.image.tag | string | `""` | DeepSpeed runtime image tag. Defaults to chart version if empty. |
| runtimes.mlxDistributed | object | `{"enabled":false,"image":{"registry":"ghcr.io","repository":"kubeflow/trainer/mlx-runtime","tag":""}}` | MLX distributed training runtime |
| runtimes.mlxDistributed.enabled | bool | `false` | Enable deployment of mlx-distributed runtime |
| runtimes.mlxDistributed.image.registry | string | `"ghcr.io"` | MLX runtime image registry |
| runtimes.mlxDistributed.image.repository | string | `"kubeflow/trainer/mlx-runtime"` | MLX runtime image repository |
| runtimes.mlxDistributed.image.tag | string | `""` | MLX runtime image tag. Defaults to chart version if empty. |
| runtimes.jaxDistributed | object | `{"enabled":false}` | JAX distributed training runtime (no custom images required) |
| runtimes.jaxDistributed.enabled | bool | `false` | Enable deployment of jax-distributed runtime |
| runtimes.torchtuneDistributed | object | `{"image":{"registry":"ghcr.io","repository":"kubeflow/trainer/torchtune-trainer","tag":""},"llama3_2_1B":{"enabled":false},"llama3_2_3B":{"enabled":false},"qwen2_5_1_5B":{"enabled":false}}` | TorchTune distributed training runtime |
| runtimes.torchtuneDistributed.image.registry | string | `"ghcr.io"` | TorchTune runtime image registry |
| runtimes.torchtuneDistributed.image.repository | string | `"kubeflow/trainer/torchtune-trainer"` | TorchTune runtime image repository |
| runtimes.torchtuneDistributed.image.tag | string | `""` | TorchTune runtime image tag. Defaults to chart version if empty. |
| runtimes.torchtuneDistributed.llama3_2_1B | object | `{"enabled":false}` | Llama 3.2 1B model configuration |
| runtimes.torchtuneDistributed.llama3_2_1B.enabled | bool | `false` | Enable deployment of Llama 3.2 1B runtime |
| runtimes.torchtuneDistributed.llama3_2_3B | object | `{"enabled":false}` | Llama 3.2 3B model configuration |
| runtimes.torchtuneDistributed.llama3_2_3B.enabled | bool | `false` | Enable deployment of Llama 3.2 3B runtime |
| runtimes.torchtuneDistributed.qwen2_5_1_5B | object | `{"enabled":false}` | Qwen 2.5 1.5B model configuration |
| runtimes.torchtuneDistributed.qwen2_5_1_5B.enabled | bool | `false` | Enable deployment of Qwen 2.5 1.5B runtime |

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
