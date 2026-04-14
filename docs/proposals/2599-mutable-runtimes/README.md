# KEP-2599: Mutable Runtimes

## Authors
- Rob Bell (Red Hat)

Assisted by Claude Code (Sonnet 4.5)

## Summary

This document proposes a design to allow Cluster Training Runtimes and Training Runtimes to be fully mutable.

This KEP introduces a new `TrainingRuntimeSnapshot` CRD for containing a point-in-time snapshot of the runtime configuration. TrainJobs create a snapshot of their runtime configuration on first reconciliation, decoupling job execution from runtime changes.

## Motivation

The `TrainingRuntime` and `ClusterTrainingRuntime` APIs were designed as blueprints for model training. A `TrainJob` references a runtime, and uses its configuration during reconciliation.

Currently, runtimes are protected by finalizers to prevent deletion while in use by TrainJobs. This design creates operational friction:

- **No safe way to update runtimes:** Updating a runtime affects all TrainJobs referencing it because runtimes are fetched during each reconciliation. While implementation-level protections exist (e.g., existing JobSets aren't modified), there are no design-level immutability guarantees. The safest practice becomes creating new runtime objects for each update (e.g., `pytorch-2.0`, `pytorch-2.1`, `pytorch-2.1.1`), leading to a proliferation of nearly-identical runtimes that confuses users.
- **Finalizers block uninstallation:** If the Trainer controller is uninstalled before all TrainJobs are removed, runtime finalizers become orphaned and cannot be removed. This prevents namespace deletion (stuck in "Terminating") and complicates controller reinstallation.

While the original [Trainer v2 design](../2170-kubeflow-trainer-v2/README.md#the-training-runtime-api) anticipated runtimes being immutable with version control, a versioning mechanism has not yet been implemented. Instead, the current finalizer-based approach prevents deletion of runtimes that are referenced but does not provide the intended benefits of versioning.

### Goals

* **Mutable Runtimes**: users and platform admins are able to update, add or remove `TrainingRuntimes` and `ClusterTrainingRuntimes` without impacting existing running or paused `TrainJobs`.
* **Self-contained TrainJob**: once a TrainJob is created, its configuration is entirely self-contained. It only depends on itself or on resources it has created and owns. It does not depend on any external resources.
* **Remove finalizer on runtimes**: `TrainingRuntimes` and `ClusterTrainingRuntimes` should no longer need a finalizer.

### Non-Goals

* **Mutable TrainJobs**: we are not proposing any changes to the existing immutable fields of TrainJobs. These fields will remain immutable.

## Proposal

### User Stories

### Story 1

As a platform engineer, I want to be able to update or delete a training runtime without breaking any existing running or paused training jobs.

### Story 2

As a maintainer of Kubeflow Trainer, I want to be able to update or delete the default training runtimes included in a Kubeflow Trainer release without introducing breaking changes for users.

## Design details

We propose making the TrainJob only look up the runtime configuration on first reconciliation and instead store a "snapshot" of the runtime configuration in a separate object:

* create a new namespaced custom resource `TrainingRuntimeSnapshot` with the same API as the `TrainingRuntime` resource. This is an internal resource and should only be created or updated by the trainer controller. Each `TrainJob` will have one `TrainingRuntimeSnapshot` with the same name and namespace as the `TrainJob`.
* when a train job is reconciled, the controller first tries to fetch the `TrainingRuntimeSnapshot` for the job. If the snapshot does not exist, it looks up the `(Cluster)TrainingRuntime` referenced by the train job and creates a new `TrainingRuntimeSnapshot` resource. The snapshot resource has the same name and namespace as the `TrainJob`, and the same spec copied from the referenced `(Cluster)TrainingRuntime`.
* the `TrainJob` reconciliation logic gets the runtime configuration from the snapshot rather than the `(Cluster)TrainingRuntime`.
* the reconciliation is otherwise unchanged.
* the `TrainingRuntimeSnapshot` is automatically deleted when the train job is deleted using an `ownerReference` on the snapshot.

Additional changes:
* Remove the finalizer on the runtimes. It is no longer necessary as TrainJobs only need to reference runtimes on first reconciliation.
* Remove the `TrainJobWatcher` interface and associated implementations and boilerplate.

### API and RBAC changes

The `TrainingRuntimeSnapshot` will have the following API:

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

The following additional RBAC permissions will be granted:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
rules:
- apiGroups:
  - trainer.kubeflow.org
  resources:
  - trainingruntimesnapshots
  verbs:
  - create
  - get
  - list
  - patch
  - update
  - watch
```

### Migrations for existing resources

**TrainJobs**: existing TrainJobs are automatically migrated on first reconciliation after upgrade. The controller creates a `TrainingRuntimeSnapshot` for each non-finished TrainJob by copying the current state of its referenced runtime.

If a runtime was modified after a TrainJob was created but before the upgrade, the snapshot will capture the modified state not the original configuration the job used. This is acceptable, however, because any incompatible changes would have already caused the TrainJob reconciliation to fail before the upgrade.

**ClusterTrainingRuntimes** and **TrainingRuntimes**: all runtime finalizers need removing which can be done using the runtime controllers. This finalizer removal logic can be removed in a future version after sufficient time has passed for all clusters to migrate. Release notes should document this and warn users they must upgrade through the migration release(s) and cannot skip directly to later versions.

**Rollback:** No explicit rollback logic is required. On rollback, the controller will reconcile using the runtime; any `TrainingRuntimeSnapshot` objects will be ignored and removed once the TrainJob is deleted.

### Test plan

#### E2E tests

* `test/e2e/e2e_test.go`
  * test updating TrainingRuntime does not affect a paused TrainJob: create runtime + train job, allow train job to start, pause train job, update the runtime, restart the train job. TrainJob should use the original configuration.
  * ensure existing tests still pass

#### Integration tests

* `test/integration/controller/clustertrainingruntime_controller_test.go`
  * remove test file. Only contains tests relating to the finalizer which is being removed.
* `test/integration/controller/trainingruntime_controller_test.go`
  * remove test file. Only contains tests relating to the finalizer which is being removed.
* `test/integration/controller/trainjob_controller_test.go`
  * test snapshot resource is created
  * test migration scenario: existing TrainJob without snapshot migrates successfully (snapshot created and used for reconciliation)

#### Unit tests

* `pkg/controller/clustertrainingruntime_controller_test.go`
  * test finalizer is always removed. Updates existing
  * remove existing tests
* `pkg/controller/trainingruntime_controller_test.go`
  * test finalizer is always removed.
  * remove existing tests. Not required.

## Open Questions

* **Are there use-cases where an update to a runtime should propagate to a TrainJob (e.g. updating a training image to address CVEs)?** Given this is currently unsupported, this could be considered out of scope.
* **Should `TrainingRuntimeSnapshot` be immutable?** Given the resource is internal and should only be edited by the controller, this may be unnecessary.
* **Should `TrainingRuntimeSnapshot` have a finalizer to prevent deletion while the TrainJob exists?** Similarly, given the resource is internal, this may be unnecessary.

## Alternatives considered

### Alternative 1: store runtime configuration in the train job status

Store the runtime configuration snapshot in a new field on the train job status, e.g. `status.runtimeConfiguration`. This pattern is used in other projects, e.g. [Tekton PipelineRuns](https://github.com/tektoncd/pipeline/blob/v1.11.0/pkg/apis/pipeline/v1/pipelinerun_types.go#L535-L539).

**Pros**
- Avoids introducing new custom resource API
- Avoids creating an additional resource per TrainJob.

**Cons**
- Adds bloat to the TrainJob status
- Makes TrainJob less readable: mixes observed state with configuration snapshots

### Alternative 2: store multiple versions of runtimes

Introduce a version control mechanism for runtimes. Runtime versions are immutable, and changes to a runtime trigger a new runtime "version" to be created. TrainJobs keep track of which runtime version they use, e.g. through a status field. TrainJob reconciliation uses the configuration of the version. The control plane is responsible for garbage collecting runtime versions that are no longer referenced by a TrainJob.

**Pros**
- Avoids creating an additional resource per TrainJob.
- Adds a minimal additional config to the TrainJob status or to etcd.

**Cons**
- Significant extra complexity to correctly manage the runtime version lifecycle.


### Alternative 3: immutable runtimes

Make runtimes immutable via webhook and CRD annotations.

**Pros**
- Prevents incompatible changes to runtimes.

**Cons**
- Adds friction to platform admins for maintaining and updating runtimes.
- Proliferation of similar runtimes that differ only in minor version details.
