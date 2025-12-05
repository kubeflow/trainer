# KEP-2841: Support Flux Framework for HPC in Kubeflow Trainer

**Authors**: [@vsoch](https://github.com/vsoch), [@milroy](https://github.com/milroy)

**Status**: Provisional

## Summary

This document outlines a proposal to integrate the Flux Framework as a high-performance computing (HPC) backend within the Kubeflow Trainer. This integration will empower users to run MPI-based and other distributed workloads with advanced scheduling, topology awareness, and a more robust bootstrapping mechanism than traditional SSH-based methods. The proposal introduces a new, extensible `hpcPolicy` in the `TrainJob` API, allowing users to select and configure an HPC workload manager, with Flux being the first implementation.

## Motivation

**Kubeflow Trainer** is a core component of the Kubeflow ecosystem, responsible for managing and executing distributed training jobs. However, as AI/ML workloads grow in scale and complexity, they often intersect with the needs of traditional HPC. Currently, users face several challenges:

*   **Fragile MPI Bootstrapping:** Distributed training jobs that use MPI are often required to bootstrap over SSH, which can be complex to configure (requiring shared keys, consistent user IDs, complicated permissions, and the SSH client/server is notoriously hard to get configure in terms of correct permissions) and is limited to specific MPI variants and implementations supported by tools like the MPI Operator.
*   **Lack of Topology Awareness:** Performance for HPC workloads is often dependent on how processes are mapped to the physical hardware. Workloads that require fine-grained, topology-aware placement logic are challenging to run on Kubernetes
*   **Limited Scheduling Features:** Kubernetes scheduling does not natively support advanced HPC concepts like custom job queues, graph-based scheduling for complex workflows, or resource reservations that are crucial for managing shared, high-demand computing environments.
*   **Scheduling Throughput**: Kubernetes is limited by API interactions, and etcd performance. Throughput in standard Kubernetes clusters can range between 10-100 Pods per second, and it can be much higher for HPC workload managers (especially Flux). In Flux, high throughput is enabled via submitting jobs to a hierarchy of Flux instances. We have experiments underway to provide updated throughput numbers for comparison.

By integrating Flux Framework, a next-generation workload manager, we can address these gaps directly. Flux provides a graph-based scheduling model and a robust `zeromq`-based bootstrapping mechanism, offering a superior alternative for demanding distributed jobs.

This KEP proposes a design to integrate Flux into Kubeflow Trainer by extending the `JobSet` backend, providing a seamless user experience for running HPC-style workloads on Kubernetes.

### Goals

1.  **Integrate Flux Framework into Kubeflow Trainer** by creating a new plugin that extends the `JobSet` backend. This plugin will dynamically inject and configure a Flux cluster "on-the-fly" for a given `TrainJob`. While the Flux Operator [MiniCluster](https://github.com/flux-framework/flux-operator) will not be used directly, it's design strategy will be.
2.  **Provide a Robust, SSH-less MPI Bootstrap Mechanism.** Enable users to run any MPI-enabled application without the overhead of configuring SSH keys or a dedicated MPI user, simply by leveraging Flux.
3.  **Expose Advanced HPC Scheduling Capabilities.** Lay the groundwork for users to leverage Flux's features, such as fine-grained resource mapping, reservations, hierarchical management, and job queueing, within their Kubeflow training jobs.
4.  **Introduce an Extensible API Policy.** Define a new `hpcPolicy` that is generic enough to support other HPC workload managers in the future (e.g., Slurm, LSF), with Flux as the initial, reference implementation.

### Non-Goals

1.  **Replace JobSet:** This proposal extends and enhances `JobSet`, not replaces it. `JobSet` remains the core abstraction for managing the group of pods.
2.  **Support Additional HPC Schedulers Immediately:** The initial implementation will focus exclusively on Flux Framework. Support for other managers can be added later under the same `hpcPolicy` API by respective parties that have interest.
3.  **Re-implement the MPI Operator:** This proposal provides an alternative to the MPI Operator's launcher/worker model by leveraging Flux's native capabilities, rather than replicating its logic.

### User Stories

**Story 1** I am an HPC practitioner using Kubernetes, and I want to deploy one of my on-premises AI/ML simulations that uses MPI. I can use the Kubeflow Trainer with the HPC Policy backend and my HPC scheduler of choice to reproduce the work.

**Story 2** I am an HPC practitioner using Kubernetes, and I want to use a flavor of MPI (such as PMIx) that is not supported by the current MPI plugin. I can use the HPC Policy with a workload manager backend like Flux to gain this functionality.

**Story 3** As an AI/ML researcher, binding and topology is important to me. I want to use a workload manager that supports fine-grained topology within an HPC cluster with nodes deployed across Kubernetes.

**Story 4** As a scientist, I want to deploy workloads that need to interact closely (e.g., under the same headless service) but have different scheduling algorithms. I can achieve this with the Flux workload manager, a choice of HPC Policy.

## Proposal

The core of this proposal is to introduce a new Kubeflow Trainer plugin named `Flux`. This plugin will implement the `ComponentBuilderPlugin` interface to modify the `JobSet` specification generated for a `TrainJob`. The mechanism for creating the Flux cluster (the set of pods mapped to physical nodes) is dynamic and non-intrusive to the user's container image:

1.  **API Trigger**: The user enables the feature by defining an `hpcPolicy` in their `TrainJob` runtime specification and setting the `manager` to `"flux"`.
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

The proposed changes will integrate into the existing `v1alpha1` API structures. We will add a new field, `hpc`, to the `MLPolicySource` struct. This aligns Flux with other runtimes like Torch and MPI.

#### 1. `ClusterTrainingRuntime` and `TrainingRuntime`

The proposal will leverage the existing `ClusterTrainingRuntime` (cluster-scoped) and `TrainingRuntime` (namespace-scoped) CRDs. These objects serve as reusable templates, and no changes are needed for their top-level structure.

#### 2. Enhancing `MLPolicySource`

We will add the `HPC` field to the `MLPolicySource` struct, which is embedded within the `MLPolicy`.

```go
// MLPolicySource is the runtime-specific configuration
// One of the following specs can be set.
type MLPolicySource struct {
    // torch defines the configuration for the PyTorch runtime.
    // +optional
	Torch *TorchMLPolicySource `json:"torch,omitempty"`

	// mpi defines the configuration for the MPI Runtime.
	// +optional
	MPI *MPIMLPolicySource `json:"mpi,omitempty"`

	// hpc defines the configuration for an hpc runtime (e.g., Flux Framework).
	// This is the new field being added.
	// +optional
	HPC *HPCMLPolicySource `json:"hpc,omitempty"`
}

// HPCMLPolicySource represents an HPC runtime configuration.
type HPCMLPolicySource struct {
	// Manager specifies the workload manager to use.
	// +kubebuilder:default="flux"
	// +optional
	Manager string `json:"manager,omitempty"`

    // Settings provides a key-value map for manager-specific configurations.
	// +optional
	Settings map[string]string `json:"settings,omitempty"`
}
```

**Example `ClusterTrainingRuntime`:**

This is an example created by an administrator.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: hpc-flux-runtime
  labels:
    trainer.kubeflow.org/framework: flux
spec:
  # The hpc policy is correctly nested under mlPolicy
  mlPolicy:
    hpc:
      manager: flux
      settings:
        flux-view-image: "ghcr.io/converged-computing/flux-view-ubuntu:tag-jammy"
        flux-network-device: "eth0"
        flux-queue-policy: "fcfs"

  # The base JobSet template that the Flux plugin will start with
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
    name: hpc-flux-runtime
    kind: ClusterTrainingRuntime
  trainer:
    numNodes: 4
    image: ghcr.io/converged-computing/metric-lammps:latest
```

### Implementation Details

1.  **Controller Logic:** When reconciling a `TrainJob`, the controller fetches the referenced `TrainingRuntime` or `ClusterTrainingRuntime`. It then passes the entire `spec` of the runtime, including the `mlPolicy`, to the plugin framework via the `runtime.Info` struct.

2.  **Plugin Logic:**

The access path within the plugin code must correctly navigate the established structure.

```go
func (f *Flux) Build(ctx context.Context, info *runtime.Info, trainJob *trainerapi.TrainJob) ([]any, error) {

	// Check for the existence of the MLPolicy and the HPC sub-field.
	if info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.HPC == nil {
		return nil, nil
	}
	policy := info.RuntimePolicy.MLPolicy.HPC

	// Check if the manager is "flux"
	if strings.ToLower(policy.Manager) != "flux" {
		return nil, nil
	}

	// The JobSetSpec is pre-populated from the runtime's template and
	// merged with the TrainJob's specifics.
	js, ok := runtime.TemplateSpecApply[v1alpha2.JobSetSpecApplyConfiguration](info)
	if !ok || js == nil {
		return nil, nil
    }

	// Example of accessing settings from the correct structure:
    fluxViewImage, ok := policy.Settings[fluxViewSetting]
	if !ok {
		fluxViewImage = defaultFluxView
	}

	// ...

	return []any{cm}, nil
}
```

This design integrates with the existing Kubeflow Trainer API, and is flexible to extension to other resource managers in that a different manager plugin can be written that receives the same API.

### Alternative Considered

#### A Hard-Coded fluxPolicy

An alternative approach is to create a hard-coded `fluxPolicy` directly within the `MLPolicySource` struct. This would involve defining explicit fields for each Flux configuration parameter we want to expose.

##### API Design for `fluxPolicy`

This would involve adding a new `Flux *FluxMLPolicySource` field to the `MLPolicySource` struct and defining the `FluxMLPolicySource` with strongly-typed fields.

```go
type MLPolicySource struct {
    Torch *TorchMLPolicySource `json:"torch,omitempty"`
    MPI   *MPIMLPolicySource   `json:"mpi,omitempty"`
    HPC   *HPCMLPolicySource   `json:"hpc,omitempty"`

    // FluxMLPolicy defines policy only for Flux
    Flux  *FluxMLPolicySource  `json:"flux,omitempty"`
}


// FluxMLPolicySource represents a Flux-specific runtime configuration
// Fields must be hard-coded fields.
type FluxMLPolicySource struct {
    // Tasks is the number of tasks for the Flux job.
    // +optional
    Tasks *int32 `json:"tasks,omitempty"`

    // ViewImage specifies the container image for the Flux init container.
    // +optional
    ViewImage string `json:"viewImage,omitempty"`

    // NetworkDevice is the network interface Flux should use for communication (e.g., "eth0").
    // +optional
    NetworkDevice string `json:"networkDevice,omitempty"`

    // QueuePolicy sets the scheduling policy for the Fluxion scheduler (e.g., "fcfs").
    // +kubebuilder:validation:Enum=fcfs;easy
    // +optional
    QueuePolicy string `json:"queuePolicy,omitempty"`

    // Interactive specifies whether to run an interactive Flux shell.
    // +optional
    Interactive *bool `json:"interactive,omitempty"`
}
```

There would likely be many more fields. The above is a subset as an example that we have implemented so far.

##### Example `ClusterTrainingRuntime` with `fluxPolicy`

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: hpc-flux-specific-runtime
spec:
  mlPolicy:
    flux:
      tasks: 4
      viewImage: "ghcr.io/converged-computing/flux-view-ubuntu:tag-jammy"
      networkDevice: "eth0"
      queuePolicy: "fcfs"
      interactive: false
```

While this approach offers the benefits of strong typing and explicit, self-documenting fields, there are several risks to be mitigated:

1.  **Tight Coupling and API Brittleness:** This design tightly couples the Kubeflow Trainer API to the specific implementation details of Flux Framework. If Flux itself introduces a new, important configuration option, it would require a formal change to the Kubeflow Trainer API. E.g., if we make additional options to Flux a string field, that serves as a mitigation for API brittleness. Requiring each flag or option to be a hardened field would make future development harder and require a new release. This creates a high maintenance burden and slows down the adoption of new features.

2.  **Lack of Extensibility:** The primary risk is inflexibility. The `hpcPolicy` with a generic `manager` and `settings` map was chosen because it can support other HPC workload managers (like Slurm, LSF, PBS) in the future *without any changes to the API*. A hard-coded approach would necessitate adding a new `slurmPolicy`, `lsfPolicy`, etc., for each new backend, leading to significant API bloat and violating the Open/Closed Principle.

3.  **Challenges of a Common API:** The number of potential configuration options across different HPC resource managers is vast and highly heterogeneous. Attempting to create a "universal" `hpcPolicy` with hard-coded fields that abstracts concepts from Slurm, Flux, and LSF simultaneously would be a significant undertaking. Slurm's `sbatch` flags, Flux's broker options, and LSF's `bsub` commands do not map cleanly to a single, unified API structure. The `manager` and `settings` map approach gracefully sidesteps this problem by delegating manager-specific configuration to a flexible key-value store, which is the only practical way to support a diverse ecosystem of backends.

- 2025.10.29: KEP Creation
