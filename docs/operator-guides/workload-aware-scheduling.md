# Workload-Aware Scheduling

This guide describes how to use Kubernetes
[Workload-Aware Scheduling (WAS)](https://kubernetes.io/docs/concepts/workloads/workload-api/)
with Kubeflow Trainer.

WAS is built into `kube-scheduler` and doesn't require a third-party scheduler, like coscheduling or
Volcano. The Kubernetes WAS supports:

- **Gang scheduling**: schedule a group of Pods only when all of them can be placed, configured
  with `schedulingPolicy`.
- **Topology-aware scheduling**: place a group of Pods within the same topology domain, such as a
  zone or a rack, configured with `schedulingConstraints`.
- **Workload-aware preemption**: preempt a group of Pods together or individually, configured with
  `disruptionMode`.
- **Shared ResourceClaims**: allocate DRA devices shared by a group of Pods, configured with
  `resourceClaims`.

## Scheduling Overview

When a ClusterTrainingRuntime or TrainingRuntime configures `spec.scheduling`, Kubeflow Trainer
creates a `Workload` and its `PodGroups` for every TrainJob, and assigns the TrainJob Pods to the
PodGroups. `kube-scheduler` then schedules, places, and preempts the Pods of each PodGroup
according to its configuration.

:::{important}
Workload-Aware Scheduling support was introduced in Kubeflow Trainer v2.4.0 behind the
`TrainJobWorkloadAwareScheduling` alpha feature gate. With the gate disabled, runtimes that set
`spec.scheduling` are rejected.

Currently, TrainJob supports a single scheduling level per ReplicatedJob. `CompositePodGroups` will
be supported in the next phases It will allow to combine the PodGroups of a TrainJob into a hierarchy
(for example, a gang of gangs across ReplicatedJobs).
You can track the progress in [the KEP-3015](https://github.com/kubeflow/trainer/tree/master/proposals/3015-workload-aware-scheduling).
:::

:::{warning}
The `spec.scheduling` API is in alpha and may change in backward-incompatible ways in future
Kubeflow Trainer releases. It also depends on the Kubernetes Workload and PodGroup APIs, which are
still evolving.
:::

## Prerequisites

- Kubeflow Trainer v2.4.0 or later installed on your cluster.
- Kubernetes v1.37 or later with:
  - The `GenericWorkload` feature gate enabled on `kube-apiserver` and `kube-scheduler`.
  - The `scheduling.k8s.io/v1beta1` API enabled on `kube-apiserver` with
    `--runtime-config=scheduling.k8s.io/v1beta1=true`.
  - The `TopologyAwareWorkloadScheduling` feature gate to use `schedulingConstraints`, and the
    `DRAWorkloadResourceClaims` feature gate to use `resourceClaims`. Otherwise, `kube-apiserver`
    drops these fields from the PodGroups.
- The `TrainJobWorkloadAwareScheduling` alpha feature gate enabled on the Trainer controller. To
  enable it, pass the flag to the controller at startup:

  ```bash
  --feature-gates=TrainJobWorkloadAwareScheduling=true
  ```

  Or, if deploying via Helm, set `manager.config.featureGates.TrainJobWorkloadAwareScheduling=true`.

To verify that your cluster serves the Workload and PodGroup APIs, run:

```bash
$ kubectl api-resources --api-group=scheduling.k8s.io

NAME              SHORTNAMES   APIVERSION                  NAMESPACED   KIND
podgroups                      scheduling.k8s.io/v1beta1   true         PodGroup
priorityclasses   pc           scheduling.k8s.io/v1        false        PriorityClass
workloads                      scheduling.k8s.io/v1beta1   true         Workload

```

:::{important}
The Trainer controller fails to start when the `TrainJobWorkloadAwareScheduling` feature gate is
enabled on a cluster that doesn't serve the `scheduling.k8s.io/v1beta1` Workload and PodGroup APIs,
and logs an error such as:

```
the TrainJobWorkloadAwareScheduling feature gate requires the cluster to serve the scheduling.k8s.io/v1beta1, Kind=Workload API
```

Enable the APIs on `kube-apiserver` as described above, or disable the feature gate.
:::

## Scheduling Levels

[The `scheduling` API](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#TrainingRuntimeScheduling)
configures PodGroups at one of the following levels:

| Level                   | Configuration                     | PodGroups                                          | When to use                                                   |
| ----------------------- | --------------------------------- | -------------------------------------------------- | ------------------------------------------------------------- |
| Level 1 – TrainJob      | `scheduling`                      | One PodGroup for all Pods of the TrainJob          | All Pods must be scheduled, placed, and preempted as one unit |
| Level 2 – ReplicatedJob | `scheduling.replicatedJobs[]`     | One PodGroup per `replicatedJobs` entry            | ReplicatedJobs need different configurations                  |
| Level 3 – Job           | `scheduling.replicatedJobs[].job` | One PodGroup per Job of the targeted ReplicatedJob | Each Job needs its own gang or topology constraints           |

The TrainJob level can't be combined with `replicatedJobs`. Within `replicatedJobs`, each entry
can use either level 2 or level 3, and every ReplicatedJob of the runtime must be targeted by
exactly one entry.

The scheduling policy defaults to `gang`. Kubeflow Trainer sets the gang `minCount` to the number of
Pods in the PodGroup, excluding the Pods of ReplicatedJobs that depend on another ReplicatedJob of
the same PodGroup, so you don't set it in the runtime. Use the `basic` policy to schedule Pods
individually.

### TrainJob-Level Scheduling

The following runtime schedules all Pods of the TrainJob as one gang, places them in the same
zone, and preempts them together:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-gang
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  scheduling:
    schedulingPolicy:
      gang: {}
    schedulingConstraints:
      topology:
        - key: topology.kubernetes.io/zone
    disruptionMode:
      all: {}
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
                      image: pytorch/pytorch:2.13.0-cuda13.0-cudnn9-runtime
```

Create a TrainJob that references the runtime:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-trainjob
spec:
  runtimeRef:
    name: torch-distributed-gang
  trainer:
    numNodes: 2
```

Kubeflow Trainer creates a Workload for the TrainJob with a single PodGroupTemplate, and one
PodGroup from it that both `node` Pods are assigned to. The PodGroup uses the gang policy with
`minCount: 2`, the `topology.kubernetes.io/zone` topology constraint, and the `all` disruption mode.

The TrainJob-level gang can't be used when the runtime uses `dependsOn`, since the dependent
Jobs are created after the others and the gang could never reach its `minCount`. Excluding the
dependent ReplicatedJobs from `minCount` doesn't help here, since it would leave the main workers,
such as the trainer nodes that depend on the initializers, outside of the gang.
Use ReplicatedJob-level scheduling for such runtimes.

### ReplicatedJob-Level Scheduling

In the `deepspeed-distributed` runtime the `launcher` depends on the `node` ReplicatedJob. The
following `scheduling` configuration schedules the `node` Pods as a gang, and the `launcher` Pod
individually:

```yaml
spec:
  scheduling:
    replicatedJobs:
      - targetReplicatedJobs:
          - node
        schedulingPolicy:
          gang: {}
      - targetReplicatedJobs:
          - launcher
        schedulingPolicy:
          basic: {}
```

Create a TrainJob that references the runtime:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-trainjob
spec:
  runtimeRef:
    name: deepspeed-distributed
  trainer:
    numNodes: 3
```

Since the `launcher` runs as a node, the `node` ReplicatedJob has 2 Pods and the `launcher` has 1
Pod. Kubeflow Trainer creates a Workload for the TrainJob with a PodGroupTemplate and a PodGroup per
entry: a gang PodGroup with `minCount: 2` for the `node` Pods, and a basic PodGroup for the
`launcher` Pod.

When an entry lists multiple ReplicatedJobs, they share a single PodGroup. The ReplicatedJobs
that depend on another ReplicatedJob in the same entry are excluded from the gang `minCount`, since
JobSet creates them only after their dependencies reach the required status. Their Pods join the
PodGroup once created. For example, if the entry targets both `launcher` and `node`, the gang
`minCount` is 2 for the `node` Pods, and the `launcher` Pod joins the PodGroup after the `node`
Pods are ready.

### Job-Level Scheduling

Set `job` on a `replicatedJobs` entry to create a PodGroup per Job of the targeted ReplicatedJob:

```yaml
spec:
  scheduling:
    replicatedJobs:
      - targetReplicatedJobs:
          - node
        job:
          schedulingPolicy:
            gang: {}
          schedulingConstraints:
            topology:
              - key: topology.kubernetes.io/zone
```

Create a TrainJob that references a `torch-distributed` runtime with this configuration:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-trainjob
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 2
```

Kubeflow Trainer creates a Workload for the TrainJob with a PodGroupTemplate and a PodGroup for the
single Job of the `node` ReplicatedJob. The PodGroup uses the gang policy with `minCount: 2` and the
`topology.kubernetes.io/zone` topology constraint.

The Job level also supports `resourceClaims`, which are shared by all Pods of the Job's PodGroup.
A Pod uses the shared claim only when it declares a matching claim, so add the same claim to the
Pod `resourceClaims` of the targeted ReplicatedJob, and reference it from the containers that use
the devices:

```yaml
spec:
  scheduling:
    replicatedJobs:
      - targetReplicatedJobs:
          - node
        job:
          resourceClaims:
            - name: tpu-slice
              resourceClaimTemplateName: tpu-slice-template
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            spec:
              template:
                spec:
                  resourceClaims:
                    - name: tpu-slice
                      resourceClaimTemplateName: tpu-slice-template
                  containers:
                    - name: node
                      resources:
                        claims:
                          - name: tpu-slice
```

TrainJobs that reference such a runtime can't set their own Pod `resourceClaims`.

:::{important}
Currently, the targeted ReplicatedJob must have at most 1 replica, since Kubernetes v1.37 can't
assign a distinct PodGroup to the Pods of each replica.
:::

## Workload Lifecycle

Kubeflow Trainer creates the Workload and PodGroups when the TrainJob starts. It deletes them when
the TrainJob is suspended, complete, or failed, so they stop reserving scheduling capacity, and
recreates them when a suspended TrainJob is resumed. When the TrainJob is deleted, Kubernetes
garbage collects them through their owner references.

## Validation Rules

Kubeflow Trainer rejects runtimes that:

- Set both `scheduling` and `podGroupPolicy`.
- Set the JobSet `scheduling`, the Job `scheduling`, or the Pod `schedulingGroup` fields in the
  template, since the Trainer controller owns them.
- Target a ReplicatedJob that doesn't exist, target a ReplicatedJob from multiple entries, or
  leave a ReplicatedJob untargeted.
- Use different `priorityClassName` values for the ReplicatedJobs that share a PodGroup.
- Set Job-level `resourceClaims` without a matching Pod `resourceClaims` entry in the targeted
  ReplicatedJob.

## Next Steps

- Learn more about the design in the
  [Workload-Aware Scheduling KEP](https://github.com/kubeflow/trainer/blob/master/proposals/3015-workload-aware-scheduling/README.md).
- Learn how to use [Dynamic Resource Allocation](dynamic-resource-allocation.md) for TrainJobs.
