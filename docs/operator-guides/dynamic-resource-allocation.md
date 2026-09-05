# Dynamic Resource Allocation (DRA)

This guide describes how to allocate GPUs and other devices for TrainJobs with Kubernetes
[Dynamic Resource Allocation (DRA)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
instead of extended resources such as `nvidia.com/gpu`.

:::{important}
DRA support was introduced in Kubeflow Trainer v2.4.0 and requires the `DynamicResourceAllocation`
alpha feature gate to be enabled on the controller.
:::

:::{note}
Make sure to follow the [Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.
:::

## Prerequisites

- Kubeflow Trainer v2.4.0 or later installed on your cluster.
- Kubernetes v1.34 or later, where DRA is GA and the `Exactly` device request API shape
  (`resource.k8s.io/v1`) used by Trainer is available.
- A DRA driver installed for your hardware (for example, the NVIDIA DRA driver for GPUs).
- The `DynamicResourceAllocation` alpha feature gate enabled on the Trainer controller. To enable
  it, pass the flag to the controller at startup:

  ```bash
  --feature-gates=DynamicResourceAllocation=true
  ```

  Or, if deploying via Helm, set `manager.config.featureGates.DynamicResourceAllocation=true`.

  The command-line flag takes precedence over any value set in the controller config file.

## How it works

A `ResourceClaimTemplate` describes the devices a Pod needs. When a TrainJob references a
template, Kubernetes creates a separate `ResourceClaim` for every training node Pod, so each
node gets its own devices.

For every TrainJob, the Trainer controller:

1. Adds the claims from `spec.trainer.resourceClaimsPerNode` to the training Pod's
   `resourceClaims` and to the `node` container's `resources.claims`.
2. Reads the **first** claim referenced by the `node` container, resolves its
   `ResourceClaimTemplate` (or `ResourceClaim`), and counts the GPUs it requests. The count is
   used the same way as an extended GPU resource: PyTorch `numProcPerNode`, MPI slots per node,
   XGBoost workers per node, and Flux GPUs per node. Extended GPU resources, if present, take
   priority over the DRA count.

## Create a ResourceClaimTemplate

Create the template in the same namespace as the TrainJob. The following example requests
exactly 8 GPUs per training node:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: gpu-claim-template
spec:
  spec:
    devices:
      requests:
        - name: gpu
          exactly:
            deviceClassName: gpu.nvidia.com
            count: 8
```

:::{important}
For GPU auto-detection to work, the device request `name` must contain `gpu`
(for example `gpu` or `h100-gpu`). Only `exactly` requests with `allocationMode: ExactCount`
(the default) are counted. `firstAvailable` requests, `allocationMode: All`, and
`adminAccess: true` requests contribute 0. If no GPU count can be resolved, PyTorch falls back
to the CPU count and the admission webhook returns a warning; set `spec.trainer.numProcPerNode`
explicitly in that case.
:::

## Use DRA in a TrainJob

Set `spec.trainer.resourceClaimsPerNode` to attach a `ResourceClaimTemplate` to every training
node. Do not set `resourcesPerNode.claims`; the TrainJob is rejected if that field is set.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: llm-finetune-dra
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    image: docker.io/my-training:latest
    numNodes: 4
    resourceClaimsPerNode:
      - name: gpu
        resourceClaimTemplateName: gpu-claim-template
```

Each training node Pod gets a `gpu` claim created from `gpu-claim-template`, and the `node`
container consumes it. With the template above, the Torch runtime resolves 8 GPUs per node and
torchrun starts one worker per GPU.

If the runtime already defines a Pod-level claim with the same `name`, the TrainJob entry
replaces it.

## Attach claims to other containers

`resourceClaimsPerNode` only wires the claim into the `node` container. To share the same
devices with a sidecar or init container, or to reference a pre-created `ResourceClaim` with
`resourceClaimName`, use [Runtime Patches](runtime-patches). The following example assumes the
runtime's `node` job has a `model-initializer` init container:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: llm-finetune-dra-init
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    image: docker.io/my-training:latest
    numNodes: 4
    resourceClaimsPerNode:
      - name: gpu
        resourceClaimTemplateName: gpu-claim-template
  runtimePatches:
    - manager: trainer.kubeflow.org/kubeflow-sdk
      trainingRuntimeSpec:
        template:
          spec:
            replicatedJobs:
              - name: node
                template:
                  spec:
                    template:
                      spec:
                        initContainers:
                          - name: model-initializer
                            resources:
                              claims:
                                - name: gpu
```

Every container `resources.claims` entry must reference a Pod-level `resourceClaims` entry.
The Trainer admission webhook rejects TrainJobs with dangling references.

## Merge semantics

- Pod-level `resourceClaims` from the runtime, `runtimePatches`, and `resourceClaimsPerNode`
  are merged by `name`. `resourceClaimsPerNode` wins over the runtime and the patches.
- A container `resources.claims` list set through `runtimePatches` **replaces** the runtime's
  list for that container. For the `node` container, `resourceClaimsPerNode` entries are added
  on top of the patched list.
- `spec.trainer.resourcesPerNode` still controls the requests and limits of the `node`
  container, and `spec.trainer.resourceClaimsPerNode` controls its claims.
- `runtimePatches[].resourceClaims` is immutable after creation (CEL-enforced).
- The GPU count is resolved from the template every time the JobSet is built. Changing the
  template's `count` only affects a running TrainJob after it is suspended and resumed.

## RBAC

The Trainer controller reads `ResourceClaimTemplate` and `ResourceClaim` objects to resolve
the GPU count, and the first lookup starts an informer for them. The Helm chart and the
Kustomize manifests grant `get`, `list`, and `watch` on `resourceclaims` and
`resourceclaimtemplates` in the `resource.k8s.io` API group. If you manage RBAC yourself, add
the same rules. Without them the GPU count resolves to 0, so set `numProcPerNode` explicitly.

## Next steps

- [Runtime Patches](runtime-patches) -- customize training runtime configuration
- [Configure TrainingRuntimes](runtime) -- set up runtimes for your cluster
- [Kubernetes DRA documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
