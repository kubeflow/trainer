# KEP-2599: Decouple runtime lifecycle from TrainJobs to simplify updating runtimes

## Authors
- Rob Bell (Red Hat)


## Summary

This KEP proposes decoupling the lifecycle of TrainingRuntimes and ClusterTrainingRuntimes from the TrainJobs that reference them by introducing a configuration snapshot mechanism. TrainJobs create a snapshot of their runtime configuration on first reconciliation, allowing runtimes to become fully mutable while ensuring TrainJob behaviour remains unchanged.

**Note:** This diverges from the [Trainer v2 design](../2170-kubeflow-trainer-v2/README.md#the-training-runtime-api), which originally proposed making runtimes immutable with version control (see also [#2599](https://github.com/kubeflow/trainer/issues/2599)). Based on operational experience, this KEP takes an alternative approach that eliminates the friction enforced immutability creates for platform administrators.

## Motivation

The `TrainingRuntime` and `ClusterTrainingRuntime` APIs serve as blueprints for model training, managed by platform administrators. A `TrainJob` references a runtime and uses its configuration during reconciliation.

**Runtimes need updating:** Platform administrators periodically need to update runtimes, for example to update the training image to get a later version of PyTorch, or to apply security patches.

**Platform admins need safe updates:** Runtimes are managed by platform administrators, not end users. Admins need to be able to update runtimes when necessary without impacting running user workloads. For example, updating the training image in a runtime should not cause pods of a running TrainJob to be restarted.

**Current protections are implementation-level only:** Currently, TrainJobs fetch runtime configuration during each reconciliation. Implementation-level protections exist (e.g. JobSets are not modified if they already exist), but there are no design-level guarantees that updating runtimes is safe. If these protections have bugs, a runtime update can have a large blast radius, affecting many running TrainJobs.

**Workaround creates proliferation:** The safer practice is creating new runtime objects for each update (e.g., `pytorch-2.0`, `pytorch-2.1`, `pytorch-2.1.1`) rather than modifying existing runtimes. However, this leads to a proliferation of nearly-identical runtimes that confuses users and creates maintenance burden.

Decoupling TrainJob behavior from runtime objects would also enable removing the finalizers currently used to protect runtimes from deletion while in use—finalizers that can create operational challenges like orphaned resources blocking namespace deletion.

**Note:** The original [Trainer v2 design](../2170-kubeflow-trainer-v2/README.md#the-training-runtime-api) anticipated runtimes being immutable with version control (see also [#2599](https://github.com/kubeflow/trainer/issues/2599)). However, a versioning mechanism has not been implemented. This KEP takes an alternative approach that makes runtimes fully mutable while preserving TrainJob immutability.

### Goals

* **Decouple the lifecycle of runtimes from train jobs**: users and platform admins are able to update, add or remove `TrainingRuntimes` and `ClusterTrainingRuntimes` without impacting existing running or paused `TrainJobs`.
* **Self-contained TrainJob**: once a TrainJob is created, its configuration is entirely self-contained. It only depends on itself or on resources it has created and owns. It does not depend on any external resources.
* **Support for future runtime types**: the snapshot mechanism should work with any runtime schema, enabling new runtime types (e.g., SlurmRuntime) to be introduced without modifications to Trainer's snapshot logic, even if those runtimes are opaque to Trainer.

### Non-Goals

* **Mutable TrainJobs**: we are not proposing any changes to the existing immutable fields of TrainJobs. These fields will remain immutable.
* **Remove the existing implementation-level protections**: e.g. like not updating JobSets once they are created.
* **Remove finalizer on runtimes**: removing the finalizer from `TrainingRuntimes` and `ClusterTrainingRuntimes` is out of scope for this KEP. However, this KEP creates the foundation to enable finalizer removal in future work.

## Proposal

### User Stories

### Story 1

As a platform engineer, I want to be able to update or delete a training runtime without breaking any existing running or paused training jobs.

### Story 2

As a maintainer of Kubeflow Trainer, I want to be able to update or delete the default training runtimes included in a Kubeflow Trainer release without introducing breaking changes for users.

## Design details

We propose making the TrainJob only look up the runtime configuration on first reconciliation and store a "snapshot" of the runtime configuration in a configmap:

* Create a `ConfigMap` to store the runtime snapshot. This is an internal resource and should only be created or updated by the trainer controller. Each `TrainJob` will have one ConfigMap named `{trainjob-name}-runtime-snapshot` in the same namespace as the `TrainJob`. The ConfigMap would be annotated with the source runtime it was copied from to aid debugging, e.g. `trainer.kubeflow.org/source-runtime: ClusterTrainingRuntime/<runtime-name>` or `trainer.kubeflow.org/source-runtime: TrainingRuntime/<runtime-namespace>/<runtime-name>`.
* When a TrainJob is reconciled, the controller first tries to fetch the runtime snapshot ConfigMap for the job. If the snapshot does not exist, the controller looks up the `(Cluster)TrainingRuntime` referenced by the train job and creates a new ConfigMap containing a single data item with key `runtime` and value set to the yaml-formatted representation of the referenced `(Cluster)TrainingRuntime`.
* The `TrainJob` reconciliation logic uses the runtime configuration contained in the ConfigMap snapshot rather than the current configuration in the `(Cluster)TrainingRuntime`.
* The reconciliation is otherwise unchanged.
* The snapshot ConfigMap is automatically deleted when the train job is deleted using an `ownerReference` on the ConfigMap.

An example ConfigMap is:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <trainjob-name>-runtime-snapshot
  ownerReferences:
    - apiVersion: trainer.kubeflow.org/v1alpha1
      kind: TrainJob
      name: <trainjob-name>
      uid: 12345678-1234-1234-1234-123456789abc
      controller: true
      blockOwnerDeletion: true
data:
  runtime: |
    apiVersion: trainer.kubeflow.org/v1alpha1
    kind: ClusterTrainingRuntime
    metadata:
      name: torch-distributed
      labels:
        trainer.kubeflow.org/framework: torch
    spec:
      mlPolicy:
        numNodes: 1
        torch: {}
      template:
        spec:
          replicatedJobs:
            - name: node
              template:
                metadata:
                  labels:
                    trainer.kubeflow.org/trainjob-ancestor-step: trainer
                spec:
                  template:
                    spec:
                      containers:
                        - name: node
                          image: pytorch/pytorch:2.10.0-cuda12.8-cudnn9-runtime
```

### API and RBAC changes

No new CRDs or RBAC changes are required.
### Migrations for existing resources

**TrainJobs**: existing TrainJobs are automatically migrated on first reconciliation after upgrade. The controller creates a runtime snapshot ConfigMap for each non-finished TrainJob by copying the current state of its referenced runtime.

If a runtime was modified after a TrainJob was created but before the upgrade, the snapshot will capture the modified state not the original configuration the job used. This configuration will be the most recent runtime configuration that has been used to reconcile the TrainJob.

**Rollback:** No explicit rollback logic is required. On rollback, the controller will reconcile using the runtime; any snapshot ConfigMaps will be ignored and removed once the TrainJob is deleted.

### Test plan

#### E2E tests

* `test/e2e/e2e_test.go`
  * test updating TrainingRuntime does not affect a paused TrainJob: create runtime + train job, allow train job to start, pause train job, update the runtime, restart the train job. TrainJob should use the original configuration.
  * ensure existing tests still pass

#### Integration tests

* `test/integration/controller/trainjob_controller_test.go`
  * test snapshot ConfigMap is created with correct structure and ownerReference
  * test migration scenario: existing TrainJob without snapshot migrates successfully (snapshot ConfigMap created and used for reconciliation)
  * test that snapshot ConfigMap is used for reconciliation instead of runtime

## Alternatives considered

### Alternative 1: introduce a new TrainingRuntimeSnapshot custom resource to store the snapshot

Store the runtime configuration snapshot in a new namespaced custom resource `TrainingRuntimeSnapshot` with the below API. The CRD would be internal and should only be created by the trainer controller. Each TrainJob would have one `TrainingRuntimeSnapshot` with the same name and namespace.

```go
// TrainingRuntimeSnapshot contains a point-in-time snapshot of a TrainingRuntime or ClusterTrainingRuntime as it was
// observed when a TrainJob was first reconciled.
type TrainingRuntimeSnapshot struct {
	metav1.TypeMeta `json:",inline"`

	// metadata of the TrainingRuntimeSnapshot.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec of the TrainingRuntimeSnapshot.
	// +optional
	Spec TrainingRuntimeSpec `json:"spec,omitempty,omitzero"`
}
```

**Pros**
- Snapshot is typed, allowing for built-in validation via CRD schema
- Consistent with Kubernetes resource patterns
- Better handling when new fields are added to the TrainingRuntime API. If new fields are added with non-zero defaults, the API server will automatically populate these new fields with the correct default value when the custom resource object is loaded. With the configmap approach, any default values may need to be explicitly applied.

**Cons**
- Introduces a new custom resource API, adding to the API surface area and increasing complexity.
- Additional complexity for supporting future runtime types, e.g. "SlurmRuntime", particularly if that runtime type is opaque to the trainer.

**Adopting the configmap approach now does not prevent migrating to a new Custom Resource approach in the future.**

### Alternative 2: store runtime configuration in the train job status

Store the runtime configuration snapshot in a new field on the train job status, e.g. `status.runtimeConfiguration`. This pattern is used in other projects, e.g. [Tekton PipelineRuns](https://github.com/tektoncd/pipeline/blob/v1.11.0/pkg/apis/pipeline/v1/pipelinerun_types.go#L535-L539).

**Pros**
- Avoids introducing new custom resource API
- Avoids creating an additional resource per TrainJob.

**Cons**
- Adds bloat to the TrainJob status
- Makes TrainJob less readable: mixes observed state with configuration snapshots

### Alternative 3: store multiple versions of runtimes

Introduce a version control mechanism for runtimes. Runtime versions are immutable, and changes to a runtime trigger a new runtime "version" to be created. TrainJobs keep track of which runtime version they use, e.g. through a status field. TrainJob reconciliation uses the configuration of the version. The control plane is responsible for garbage collecting runtime versions that are no longer referenced by a TrainJob.

**Pros**
- Avoids creating an additional resource per TrainJob.
- Adds a minimal additional config to the TrainJob status or to etcd.

**Cons**
- Significant extra complexity to correctly manage the runtime version lifecycle.

### Alternative 4: immutable runtimes

Make runtimes immutable via webhook and CRD annotations.

**Pros**
- Prevents incompatible changes to runtimes.

**Cons**
- Adds friction to platform admins for maintaining and updating runtimes.
- Proliferation of similar runtimes that differ only in minor version details.
