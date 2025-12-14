# KEP-2841: Support Flux Framework for HPC in Kubeflow Trainer

**Authors**: [@vsoch](https://github.com/vsoch), [@milroy](https://github.com/milroy)

**Status**: Provisional

## Summary

This document outlines a proposal to integrate the Flux Framework as a high-performance computing (HPC) backend within the Kubeflow Trainer. This integration will empower users to run MPI-based and other distributed workloads with advanced scheduling, topology awareness, and a more robust bootstrapping mechanism than traditional SSH-based methods. The proposal introduces a new, extensible `FluxMLPolicySource` in the `TrainJob` API, allowing users to select and configure the HPC workload manager Flux.

## Motivation

**Kubeflow Trainer** is a core component of the Kubeflow ecosystem, responsible for managing and executing distributed training jobs. However, as AI/ML workloads grow in scale and complexity, they often intersect with the needs of traditional HPC. Currently, users face several challenges:

*   **Fragile MPI Bootstrapping:** Distributed training jobs that use MPI are often required to bootstrap over SSH, which can be complex to configure (requiring shared keys, consistent user IDs, complicated permissions, and the SSH client/server is notoriously hard to get configure in terms of correct permissions) and is limited to specific MPI variants and implementations supported by tools like the MPI Operator.
*   **Lack of Topology Awareness:** Performance for HPC workloads is often dependent on how processes are mapped to the physical hardware. Workloads that require fine-grained, topology-aware placement logic are challenging to run on Kubernetes.
*   **Limited Scheduling Features:** Kubernetes scheduling does not natively support advanced HPC concepts like custom job queues, graph-based scheduling for complex workflows, or resource reservations that are crucial for managing shared, high-demand computing environments.
*   **Scheduling Throughput**: Kubernetes is limited by API interactions, and etcd performance. Throughput in standard Kubernetes clusters can range between 10-100 Pods per second, and it can be much higher for HPC workload managers (especially Flux). In Flux, high throughput is enabled via submitting jobs to a hierarchy of Flux instances. We have experiments underway to provide updated throughput numbers for comparison.

By integrating Flux Framework, a next-generation workload manager, we can address these gaps directly. Flux provides a graph-based scheduling model and a robust `zeromq`-based bootstrapping mechanism, offering a superior alternative for demanding distributed jobs.

This KEP proposes a design to integrate Flux into Kubeflow Trainer by extending the `JobSet` backend, providing a seamless user experience for running HPC-style workloads on Kubernetes.

### Goals

1.  **Integrate Flux Framework into Kubeflow Trainer** by creating a new plugin that extends the `JobSet` backend. This plugin will dynamically inject and configure a Flux cluster "on-the-fly" for a given `TrainJob`. While the Flux Operator [MiniCluster](https://github.com/flux-framework/flux-operator) will not be used directly, it's design strategy will be.
2.  **Provide a Robust, SSH-less MPI Bootstrap Mechanism.** Enable users to run any MPI-enabled application without the overhead of configuring SSH keys or a dedicated MPI user, simply by leveraging Flux.
3.  **Expose Advanced HPC Scheduling Capabilities.** Lay the groundwork for users to leverage Flux's features, such as fine-grained resource mapping, reservations, hierarchical management, and job queueing, within their Kubeflow training jobs.
4.  **Introduce an Extensible API Policy.** Define a new `FluxMLPolicySource` to support deployment of Flux Framework for MPI, simulation, or converged HPC/AI/ML workloads.

### Non-Goals

1.  **Replace JobSet:** This proposal extends and enhances `JobSet`, not replaces it. `JobSet` remains the core abstraction for managing the group of pods.
2.  **Support Additional HPC Schedulers:** This implementation focuses exclusively on Flux Framework. Support for other managers can be added by respective parties that have interest.
3.  **Re-implement the MPI Operator:** This proposal provides an alternative to the MPI Operator's launcher/worker model by leveraging Flux's native capabilities, rather than replicating its logic.

### User Stories

**Story 1** I am an HPC practitioner using Kubernetes, and I want to deploy one of my on-premises AI/ML simulations that uses MPI. I can use the Kubeflow Trainer with the Flux Policy backend to reproduce the work.

**Story 2** I am an HPC practitioner using Kubernetes, and I want to use a flavor of MPI (such as PMIx) that is not supported by the current MPI plugin. I can use the Flux Policy to gain this functionality.

**Story 3** As an AI/ML researcher, binding and topology is important to me. I want to use a workload manager that supports fine-grained topology within an HPC cluster with nodes deployed across Kubernetes.

**Story 4** As a scientist, I want to deploy workloads that need to interact closely (e.g., under the same headless service) but have different scheduling algorithms. I can achieve this with the Flux workload manager, a choice of Flux Policy.

## Proposal

The core of this proposal is to introduce a new Kubeflow Trainer plugin named `Flux`. This plugin will implement the `ComponentBuilderPlugin` interface to modify the `JobSet` specification generated for a `TrainJob`. The mechanism for creating the Flux cluster (the set of pods mapped to physical nodes) is dynamic and non-intrusive to the user's container image:

1.  **API Trigger**: The user enables the feature by defining `flux` in their `TrainJob` runtime specification as an ML policy source.
2.  **Plugin Activation**: The Kubeflow Trainer controller invokes the `Flux` plugin's `Build` method.
3.  **JobSet Modification**: The plugin modifies the `JobSet` specification before it is created:
    *   An **`initContainer`** is added to the "trainer" replicated job. This container uses a pre-built "flux-view" image containing a Spack installation of Flux.
    * A **pod affinity** is added that enforces a soft requirement to schedule one pod per node to support Flux controlling the mapping of all node resources. An optional **node affinity** can enforce that the cluster pods are only scheduled to specific nodes.
    *   A **shared memory mount** that ensures the pod can utilize all shared memory on the node (or a reasonable subset). By default most container runtimes will mount only 64M, and this can negatively impact MPI performance.
    *   **Shared `emptyDir` Volumes** are mounted into both the `initContainer` and the main application container to move the Flux install from the initContainer to the application container. The `initContainer` copies the Flux installation and necessary software from its own image into these shared volumes, and generates configuration for the cluster based on the user-preferences provided.
    *   A **ConfigMap** is generated containing two scripts: `init.sh` (for the init container) and `entrypoint.sh` (for the main container). This ConfigMap is mounted into the pods.
4.  **Execution Wrap**: The command of the user's main application container is overridden to use the `entrypoint.sh` script. This script first sets up the Flux environment (using the files from the shared volumes) and then uses `flux start` and `flux submit` to launch the user's original command within the now-active Flux cluster.
5.  **Networking**: The plugin ensures the `JobSet` is configured with a headless service and DNS hostnames enabled, which Flux uses for its broker communication. High speed network for MPI can be used by way of extending pods to use a bypass mechanism to support Infiniband (Azure) or the Elastic Fabric Adapter (AWS).

This approach provides an HPC environment without requiring the user to build a custom Docker image with Flux pre-installed, significantly improving the user experience.

### API Design

The proposed changes will integrate into the existing `v1alpha1` API structures. We will add a new field, `flux`, to the `MLPolicySource` struct. This aligns Flux with other runtimes like Torch and MPI.

#### 1. `ClusterTrainingRuntime` and `TrainingRuntime`

The proposal will leverage the existing `ClusterTrainingRuntime` (cluster-scoped) and `TrainingRuntime` (namespace-scoped) CRDs. These objects serve as reusable templates, and no changes are needed for their top-level structure.

#### 2. Enhancing `MLPolicySource`

We will add the `Flux` field to the `MLPolicySource` struct, which is embedded within the `MLPolicy`.

```go
type MLPolicySource struct {
    Torch *TorchMLPolicySource `json:"torch,omitempty"`
    MPI   *MPIMLPolicySource   `json:"mpi,omitempty"`

    // FluxMLPolicy defines policy only for Flux
    // +optional
    Flux  *FluxMLPolicySource  `json:"flux,omitempty"`
}
```

For the `FluxMLPolicySource`, we define the minimum required parameters needed for Flux and installing the view. The view must be compatible with the application container.

```go
// FluxMLPolicySource represents a Flux HPC runtime configuration.
type FluxMLPolicySource struct {

    // numNodes is the number of physical nodes for the job.
    // This is defined a level up on the Trainer

    // numProcPerNode is the number of processes per node.
    // Defaults to 1.
    // +kubebuilder:default=1
    // +optional
    NumProcPerNode *int32 `json:"numProcPerNode,omitempty"`

    // Container image to use for Flux view that installs Flux
    // This must be compatible with the application container
    // Get the flux view container (these are choices)
    // ghcr.io/converged-computing/flux-view-rocky:arm-9
    // ghcr.io/converged-computing/flux-view-rocky:arn-8
    // ghcr.io/converged-computing/flux-view-rocky:tag-9
    // ghcr.io/converged-computing/flux-view-rocky:tag-8
    // ghcr.io/converged-computing/flux-view-ubuntu:tag-noble
    // ghcr.io/converged-computing/flux-view-ubuntu:tag-jammy
    // ghcr.io/converged-computing/flux-view-ubuntu:tag-focal
    // ghcr.io/converged-computing/flux-view-ubuntu:arm-jammy
    // ghcr.io/converged-computing/flux-view-ubuntu:arm-focal
    // We use an ubuntu (more recent) default since it is common
    // +kubebuilder:default="ghcr.io/converged-computing/flux-view-ubuntu:arm-jammy"
    Image string `json:"image,omitempty"`

    // Network device for flux to use
    // +kubebuilder:default="eth0"
    NetworkDevice string `json:"networkDevice,omitempty"`

    // Queue policy for Flux to use
    // +kubebuilder:default="fcfs"
    QueuePolicy string `json:"queuePolicy,omitempty"`
}
```

Note that we are not exposing interactive or other options that are widely used, but not required.

**Example `ClusterTrainingRuntime`:**

This is an example created by an administrator.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: flux-runtime
  labels:
    trainer.kubeflow.org/framework: flux
spec:
  mlPolicy:
    flux:
      numNodes: 1
  template:
    spec:
      replicatedJobs:
        - name: trainer
          template:
            spec:
              completionMode: Indexed
              completions: 1
              parallelism: 1
              template:
                spec:
                  containers:
                    - name: node
                      image: "placeholder-image"
```

**Example Consuming `TrainJob` YAML:**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: lammps-flux-interactive
spec:
  # Reference the pre-defined runtime by name
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    name: flux-runtime
    kind: ClusterTrainingRuntime
  trainer:
    numNodes: 4
    image: ghcr.io/converged-computing/metric-lammps:latest
```

### Implementation Details

1.  **Controller Logic:** When reconciling a `TrainJob`, the controller fetches the referenced `TrainingRuntime` or `ClusterTrainingRuntime`. It then passes the entire `spec` of the runtime, including the `mlPolicy`, to the plugin framework via the `runtime.Info` struct.

2.  **Plugin Logic:** We add components of Flux via `Build` if the user has defined a Flux policy. The design integrates with the existing Kubeflow Trainer API.

- 2025.10.29: KEP Creation
