# KEP-4064: MPI Node Groups

## Summary

The MPI plugin supports two replicatedJobs, `launcher` and `node`, sized by one `numNodes` and given one `numProcPerNode`. This KEP lets an MPI runtime declare several node groups, each a replicatedJob with its own pod template, and lets a TrainJob set the size, slots, image and resources of each group and of the launcher with one shared schema. The plugin finds the launcher and the groups through `mlPolicy.mpi`, not through replicatedJob names or the `trainer.kubeflow.org/trainjob-ancestor-step` label. Runtimes that do not use the new fields keep today's behaviour.

## Motivation

At CERN, scientific users need one MPI job whose members differ:

1. Different node classes in one job, across GPU vendors and including CPU only members. For example one high core count CPU node, two NVIDIA L40S nodes and one AMD W7900 node.
2. Slots per member, for example 2 on MI300X members and 4 on L40S members.
3. Several single GPU pods of one group, possibly on the same node, so that each GPU is its own rank.

This is not possible today, as discussed in [kubeflow/mpi-operator#791](https://github.com/kubeflow/mpi-operator/issues/791#issuecomment-5639030676). The plugin identifies MPI members by the replicatedJob names `node` and `launcher` ([`mpi.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/mpi/mpi.go#L141-L145), [`mpi.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/mpi/mpi.go#L342-L344)), so an extra group such as `node-mi300x` starts but gets no SSH secret and no hostfile line, and `mpirun` ignores it while the job looks healthy ([repro](https://github.com/kubeflow/mpi-operator/issues/791#issuecomment-5637596190)). `spec.trainer.numNodes` sizes only `node` ([`mpi.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/mpi/mpi.go#L117-L125)), and one `numProcPerNode` is written on every hostfile line ([`mpi.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/mpi/mpi.go#L314-L328)). `spec.trainer.image`, `command` and `args` reach only the replicatedJob with the ancestor label, and putting the label on the workers to change their image also replaces their SSH server command ([#4030](https://github.com/kubeflow/trainer/issues/4030)).

A runtime could declare every flavour as its own replicatedJob, but a runtime is all or nothing: a TrainJob cannot size, disable or retune the groups it declares. Every flavour combination then needs its own runtime, and the mix becomes a menu curated by admins instead of a job parameter. Our current workaround is admission policies that rewrite the pods and the hostfile ConfigMap by pod index, which fails silently when the two rewrites disagree on the index.

### Goals

- One MPI TrainJob with several node groups, each with its own pod template, pod count and slots.
- One TrainJob schema for the launcher and the node groups.
- Runtimes that offer groups which stay disabled unless a TrainJob enables them.
- No change for existing runtimes and TrainJobs.

### Non-Goals

- Changing other frameworks or removing the ancestor label for them, although the same shape could later replace it.
- Placement fields in the new API. Placement stays in `runtimePatches`, which already targets replicatedJobs by name.
- Checking that the images of different groups ship compatible MPI versions.
- More than one JobSet replica per group.

## Proposal

Every group stays a replicatedJob in the runtime template with its full pod template, so groups can differ in node selectors, tolerations, images and sidecars. Two new fields in `MPIMLPolicySource`, `launcher` and `nodeGroups`, reference those replicatedJobs by name, like Torch's [`envInjection`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/apis/trainer/v1alpha1/trainingruntime_types.go#L219), and name the container that runs MPI in each. On the TrainJob, `spec.trainer.launcher` and `spec.trainer.nodeGroups` override the runtime values per group with the same set of fields. A group's `numNodes` is its number of MPI hosts, and a group with `numNodes` 0 is left out of the JobSet.

### User Stories (Optional)

As a user, I submit one TrainJob with one CPU node, two L40S pods and one W7900 node against a runtime that offers all three flavours, and set the slots of each group, without asking an admin for a new runtime.

### Notes/Constraints/Caveats (Optional)

Groups can use different images, for example CUDA and ROCm builds, but they must ship compatible MPI versions, since the launcher starts the MPI daemons on every host over SSH.

`env` on a node group reaches its pod but not its ranks, which inherit the environment of the daemon started over SSH. Variables for the ranks belong on the launcher, with `mpirun -x` for Open MPI.

### Risks and Mitigations

A wrong `jobName`, `containerName` or group `name` would silently drop hosts, which is the failure this KEP removes. The runtime and TrainJob webhooks reject names that do not resolve.

MPI runtimes can be configured in two ways until the flat fields are deprecated for MPI. The webhooks reject a TrainJob that mixes them.

## Design Details

The new fields in `trainingruntime_types.go`:

```go
// +kubebuilder:validation:XValidation:rule="has(self.launcher) == has(self.nodeGroups)",message="launcher and nodeGroups must be set together"
// +kubebuilder:validation:XValidation:rule="!(has(self.launcher) && has(self.runLauncherAsNode) && self.runLauncherAsNode)",message="use launcher.runAsNode instead of runLauncherAsNode"
type MPIMLPolicySource struct {
	// Existing fields are unchanged. With nodeGroups, numProcPerNode is the
	// default for the launcher and the groups that do not set their own.

	// launcher declares the replicatedJob that runs mpirun.
	// +optional
	Launcher *MPILauncher `json:"launcher,omitempty"`

	// nodeGroups declares the replicatedJobs whose pods are MPI hosts.
	// +listType=map
	// +listMapKey=jobName
	// +kubebuilder:validation:MinItems=1
	// +optional
	NodeGroups []MPINodeGroup `json:"nodeGroups,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!has(self.numProcPerNode) || (has(self.runAsNode) && self.runAsNode)",message="numProcPerNode requires runAsNode"
type MPILauncher struct {
	// jobName is the name of the launcher replicatedJob.
	// +kubebuilder:validation:MinLength=1
	// +required
	JobName string `json:"jobName"`

	// containerName is the container that runs mpirun.
	// Defaults to node.
	// +kubebuilder:default=node
	// +optional
	ContainerName *string `json:"containerName,omitempty"`

	// runAsNode adds the launcher to the hostfile. It replaces runLauncherAsNode.
	// Defaults to false.
	// +kubebuilder:default=false
	// +optional
	RunAsNode *bool `json:"runAsNode,omitempty"`

	// numProcPerNode is the number of slots of the launcher.
	// +kubebuilder:validation:XValidation:rule="self >= 1",message="numProcPerNode must be greater than or equal to 1"
	// +optional
	NumProcPerNode *int32 `json:"numProcPerNode,omitempty"`
}

type MPINodeGroup struct {
	// jobName is the name of the replicatedJob.
	// +kubebuilder:validation:MinLength=1
	// +required
	JobName string `json:"jobName"`

	// containerName is the container that runs sshd.
	// Defaults to node.
	// +kubebuilder:default=node
	// +optional
	ContainerName *string `json:"containerName,omitempty"`

	// numNodes is the number of pods in the group, one MPI host each.
	// 0 leaves the group out unless a TrainJob enables it.
	// Defaults to 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	NumNodes *int32 `json:"numNodes,omitempty"`

	// numProcPerNode is the number of slots of each host in the group.
	// +kubebuilder:validation:XValidation:rule="self >= 1",message="numProcPerNode must be greater than or equal to 1"
	// +optional
	NumProcPerNode *int32 `json:"numProcPerNode,omitempty"`
}
```

The new fields in `trainjob_types.go`:

```go
// +kubebuilder:validation:XValidation:rule="!(has(self.launcher) || has(self.nodeGroups)) || !(has(self.image) || has(self.command) || has(self.args) || has(self.env) || has(self.numNodes) || has(self.resourcesPerNode) || has(self.numProcPerNode))",message="launcher and nodeGroups cannot be combined with the other trainer fields"
type Trainer struct {
	// Existing fields are unchanged.

	// launcher overrides mlPolicy.mpi.launcher.
	// +optional
	Launcher *MPILauncherOverride `json:"launcher,omitempty"`

	// nodeGroups overrides mlPolicy.mpi.nodeGroups.
	// +listType=map
	// +listMapKey=name
	// +optional
	NodeGroups []MPINodeGroupOverride `json:"nodeGroups,omitempty"`
}

type MPILauncherOverride struct {
	// runAsNode overrides mlPolicy.mpi.launcher.runAsNode.
	// +optional
	RunAsNode *bool `json:"runAsNode,omitempty"`

	MPIGroupOverride `json:",inline"`
}

type MPINodeGroupOverride struct {
	// name is the jobName of a group in mlPolicy.mpi.nodeGroups.
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// numNodes overrides the number of pods in the group. 0 disables the group.
	// +kubebuilder:validation:Minimum=0
	// +optional
	NumNodes *int32 `json:"numNodes,omitempty"`

	MPIGroupOverride `json:",inline"`
}

// MPIGroupOverride holds the fields shared by the launcher and the node groups.
// The container fields apply only to the container named by containerName.
type MPIGroupOverride struct {
	// numProcPerNode overrides the number of slots of each host in the group.
	// +kubebuilder:validation:XValidation:rule="self >= 1",message="numProcPerNode must be greater than or equal to 1"
	// +optional
	NumProcPerNode *int32 `json:"numProcPerNode,omitempty"`

	// The fields below have the same markers and merge semantics as the
	// fields with the same name in Trainer.
	Image            *string                      `json:"image,omitempty"`
	Command          []string                     `json:"command,omitempty"`
	Args             []string                     `json:"args,omitempty"`
	Env              []corev1.EnvVar              `json:"env,omitempty"`
	ResourcesPerNode *corev1.ResourceRequirements `json:"resourcesPerNode,omitempty"`
}
```

Matching and overrides:

- `launcher.jobName` and `nodeGroups[].jobName` name a replicatedJob, and `containerName` names the MPI container in it. A replicatedJob has at most one role, and it must have `replicas` 1, because the hostfile enumerates replicas times pods ([`jobset.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/jobset/jobset.go#L296-L302)).
- `spec.trainer.launcher` targets the runtime launcher. Each `spec.trainer.nodeGroups[].name` must match a runtime `jobName`, so a TrainJob sizes the groups a runtime offers but cannot add new ones.
- Values resolve field by field: the TrainJob entry, then the runtime entry, then `mlPolicy.mpi.numProcPerNode` for slots. As today ([`mpi.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/mpi/mpi.go#L128-L139)), slots that resolve to 1 are replaced by the GPU count of the group.
- `image`, `command`, `args`, `env` and `resourcesPerNode` go only to the MPI container of the group, so sidecars and init containers are untouched. `env` is upserted and `resourcesPerNode` merged as the flat fields are today.
- A group whose `numNodes` resolves to 0 is removed from the JobSet, and `dependsOn` entries that point to it are dropped. At least one MPI host must remain.
- When the runtime declares `nodeGroups`, the flat `spec.trainer` fields are rejected, and `mlPolicy.numNodes` must be unset. The new TrainJob fields are rejected on runtimes without `nodeGroups`.

With groups `cpu` (32 slots), `l40s` and `w7900` and a launcher that runs as a node, a TrainJob `mixed` that enables one `cpu` host, two single GPU `l40s` pods and one `w7900` host with 2 slots gets this hostfile, in replicatedJob order:

```
mixed-cpu-0-0.mixed slots=32
mixed-l40s-0-0.mixed slots=1
mixed-l40s-0-1.mixed slots=1
mixed-w7900-0-0.mixed slots=2
mixed-launcher-0-0.mixed slots=1
```

Controller changes:

- The MPI plugin resolves every group in `EnforceMLPolicy`, sets the PodSet count of each group, mounts the SSH secret on every group and the launcher, and writes each group's slots on its own hostfile lines. `Validate` checks the TrainJob against the runtime.
- The JobSet builder and the runtime merge in `trainingruntime.go` apply the group fields to the group's container and find the launcher by `launcher.jobName`, as does the TrainJob status plugin ([`trainjobstatus.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/trainjobstatus/trainjobstatus.go#L92)). Runtimes that use the new fields need no ancestor label.
- The TrainingRuntime webhook checks that every `jobName` and `containerName` exists and that each replicatedJob has at most one role.

### Test Plan

[x] I/we understand the owners of the involved components may require updates to existing tests to make this code solid enough prior to committing the changes necessary to implement this enhancement.

#### Prerequisite testing updates

#### Unit Tests

Current coverage on 2026-09-30, from `go test -cover`, and what each package gains:

- `pkg/runtime/framework/plugins/mpi`, 87.4%: resolution order, per group slots and GPU fallback, hostfile lines, groups with `numNodes` 0, TrainJob validation.
- `pkg/runtime/framework/plugins/jobset`, 85.5%: container fields reach only the named container, removed groups and their `dependsOn` entries.
- `pkg/runtime/core`, 68.4%: per group resources merged into the named container.
- `pkg/webhooks`, 69.6%: every rejected combination listed above.

#### E2E tests

A TrainJob against a runtime with two CPU node groups of different slots, checking that every host runs the expected number of ranks.

#### Integration tests

A TrainJob with node groups in `test/integration/controller`, checking the JobSet, the SSH secret and the hostfile ConfigMap.

### Graduation Criteria

The new fields are optional additions to `v1alpha1` and graduate with the API.

## Implementation History

- 2026-09-14: Issue [#4064](https://github.com/kubeflow/trainer/issues/4064) opened.
- 2026-09-30: Alternative with a flat launcher proposed in [#4064](https://github.com/kubeflow/trainer/issues/4064#issuecomment-5901663962).
- 2026-09-30: KEP drafted. A prototype with the same input shape, built on plain Jobs, runs at CERN, including on AMD W7900 nodes.

## Drawbacks

`Trainer` is shared by all frameworks and gains two MPI only fields. The webhook rejects them on runtimes without `nodeGroups`.

## Alternatives

Flat launcher fields with the ancestor label ([comment](https://github.com/kubeflow/trainer/issues/4064#issuecomment-5901663962)). `spec.trainer` stays the launcher and `nodeGroups` sits next to it, which needs fewer new fields. However the flat fields already have mixed targets: `image`, `command` and `args` reach only the launcher ([`builder.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/jobset/builder.go#L120-L141)), while `env` and `resourcesPerNode` also reach `node` when `runLauncherAsNode` is set ([`builder.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/framework/plugins/jobset/builder.go#L142-L155), [`trainingruntime.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/runtime/core/trainingruntime.go#L207-L214)). Next to `nodeGroups`, `resourcesPerNode` would mean every node in `deepspeed-distributed` but only the launcher in a runtime with groups, and the label would have to stay in the beta API.

Role labels on replicatedJobs, with per group settings in the runtime only ([kubeflow/mpi-operator#791](https://github.com/kubeflow/mpi-operator/issues/791)). This needs no TrainJob change, but a TrainJob cannot size, retune or disable a group, so every flavour combination needs its own runtime.

Per group fields in `runtimePatches`. `ReplicatedJobPatch` has no pod count, and `ContainerPatch` has only `name`, `env`, `volumeMounts` and `securityContext`, with `env` not allowed on `node` ([`trainjob_types.go`](https://github.com/kubeflow/trainer/blob/a08836af1a9b1b03db4176930f94f0df15c25b7d/pkg/apis/trainer/v1alpha1/trainjob_types.go#L468-L496)). It would need a count, slots and the container fields, which turns a list of patches keyed by manager into the main MPI API.

Admission policies that rewrite the pods and the hostfile, which is what we run today. It needs no upstream change, but the pod and ConfigMap rewrites must agree on the pod index, or ranks land on the wrong hosts with no error.
