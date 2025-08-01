# KEP-2437: Support Volcano Scheduler

## Summary

This document outlines a proposal to support Volcano for gang-scheduling in Kubeflow Trainer, so as to provide users with more AI-specific scheduling capacities like priority scheduling and queue resource management. Thanks to the [Kubeflow Trainer Pipeline Framework](https://github.com/kubeflow/trainer/tree/master/docs/proposals/2170-kubeflow-trainer-v2#pipeline-framework), we can seamlessly integrate Volcano into Kubeflow Trainer as a runtime plugin.

## Motivation

**Kubeflow Trainer** is a core component of the Kubeflow ecosystem, responsible for managing and executing distributed training jobs. In distributed training scenarios, an efficient **scheduling mechanism** is crucial:

- A distributed training job typically involves multiple pods (such as distributed training with PyTorch or MPI) running in coordination. To avoid the resource fragmentation, all pods need to be started at the same time. That’s why **Gang Scheduling** matters.
- The default Kubernetes scheduler was initially designed for long-running services. It uses a **pod-by-pod** scheduling approach, lacking support for batch tasks. As a result, it fails to support Gang Scheduling, which is strongly required  in AI and big data scenarios.

Kubeflow Trainer V2 currently uses the **Coscheduling** plugin to provide  the Gang Scheduling support. However, it has some limitations, such as the inability to perform priority scheduling.

Introducing the **Volcano** scheduler will enhance Trainer's scheduling capabilities.This will provide users with more flexible and efficient scheduling algorithms. Specifically, it can bring the following needs and values:

1. **Provide advanced AI-specific features**
   The existing Coscheduling plugin only supports basic Gang Scheduling functions. **Volcano**, a widely adopted scheduler in the industry, offers rich AI-specific scheduling capabilities, such as priority scheduling with **Queues** for more detailed resource management.
2. **Enrich Kubeflow Ecosystem**
   Volcano is a well-known and widely used scheduler in Kubernetes. Many users are familiar with it. We provided a Volcano scheduling option in Training Operator V1. Continuing to support Volcano in Trainer will help users migrate to Kubeflow Trainer V2 smoothly.
   Additionally, Volcano's [official documentation](https://volcano.sh/en/docs/kubeflow_on_volcano/) highlights Kubeflow as a key collaborator within its ecosystem.
3. **Mitigating limitations in edge cases**
   The introduction of the Volcano scheduler enhances the scheduling flexibility for federated learning, as requested by the Sedna project ([kubeedge/sedna\#463](https://github.com/kubeedge/sedna/issues/463)), which needed gang-scheduling support in Trainer V2.

### Goals

1. **Integrate Volcano Scheduler into Kubeflow Trainer.** Integrate the **Volcano** scheduler into Trainer to support Gang Scheduling and resource management for distributed training jobs.
2. **Support some advanced scheduling features**. Provide some advanced scheduling features, such as prioritizing high-priority jobs and assigning specific queues.
3. **Provide user guidance**. Update the user documentation with appropriate use cases.

### Non-Goals

1. **Replace the existing Coscheduling plugin**. This proposal aim to provide an alternative scheduling option based on Volcano.
2. **Modifying Volcano's core scheduling logic.** No modifications or control over the internal scheduling algorithms or mechanisms of the Volcano scheduler itself.
3. **Integration with VolcanoJob (vcjob).** This proposal will not integrate with vcjob or manage the lifecycle of vcjob within the Volcano ecosystem. We support only PodGroup-based scheduling.

## Proposal

We plan to integrate Volcano into Kubeflow Trainer as a runtime plugin, following the best practice of [Kubeflow Trainer Pipeline Framework](https://github.com/kubeflow/trainer/tree/master/docs/proposals/2170-kubeflow-trainer-v2#pipeline-framework). This plugin-based design allows users to switch to Volcano scheduler without reinstalling or modifying the core Trainer component, making the integration more modular, flexible, and user-friendly.

PodGroup is the basic scheduling unit. It is created based on the scheduling parameters specified in *Training Runtime*, after which Volcano will manage and schedule the pods specified in the PodGroup. This is similar to the approach used in Training Operator V1.

The diagram below shows how Volcano is integrated into the TrainJob creation workflow.

![user-roles](./user-roles-scheduler.drawio.svg)

As shown in the diagram, advanced scheduling is applied through a two-stage workflow:

1. First, platform engineers define the scheduling strategy when customizing *ClusterTrainingRuntime* / *TrainRuntime*. This step requires familiarity with the Kubernetes API and the Volcano scheduler.
2. Then, data scientists will submit TrainJobs by choosing a *TrainingRuntime* with a specific scheduling method in the *TrainJob*. They don't need to understand the underlying implementation details.

### User Stories


#### Story 1

As a platform engineer, I am familiar with Kubernetes APIs. I want to implement Gang Scheduling for my distributed training jobs to ensure that all tasks within a training job are scheduled together on the cluster.

The ClusterTrainingRuntime may look as follows:

```yaml
apiVersion: trainer.kubeflow.org/v2alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-gang-scheduling
spec:
  mlPolicy:
    numNodes: 2
    torch:
      numProcPerNode: 5
  podGroupPolicy:
    volcano: {}
  template:
    spec:
      replicatedJobs:
        - name: Node
          template:
            spec:
              template:
                spec:
                  schedulerName: volcano
                  priorityClassName: "high-priority"
                  containers:
                    - name: trainer
                      image: docker.io/kubeflow/pytorch-mnist
                      resources:
                        requests:
                          cpu: "1000m"
                          memory: "2Gi"
                          nvidia.com/gpu: 1
                      env:
                        - name: MASTER_ADDR
                          value: "pytorch-node-0-0.pytorch"
                        - name: MASTER_PORT
                          value: 29400
                      command:
                        - torchrun train.py
```

#### Story 2

As a platform engineer, I am familiar with both Kubernetes APIs and Volcano scheduler. I want to optimize my distributed training jobs with advanced scheduling features. My goal is to ensure **high-priority training jobs** get scheduled first while efficiently managing cluster resources for multiple concurrent jobs.

First I will create my Queue in the cluster. The custom Queue may look as follows:

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: high-priority-queue
spec:
  weight: 1
  reclaimable: false
  capability:
    cpu: 2
```

Then I specify the Queue name in ClusterTrainingRuntime spec:

```yaml
spec:
  podGroupPolicy:
    volcano: {}
  template:
    metadata:
      scheduling.volcano.sh/queue-name: "high-priority-queue"
```


## Design Details

As shown in the workflow diagram above, we decide to implement a runtime plugin for Volcano with the Kubeflow Trainer Pipeline Framework. It will:

- **Build PodGroups** based on the *Training Runtime* configuration and calculate resource limits (e.g., `MinResource`).
- **Manage PodGroups**
   - Update: Update PodGroups and perform rescheduling when there are changes in cluster resource demands (e.g., changes in `LimitRange`).
   - Suspended/Resumed: Support scheduling for suspended and resumed training jobs, with special handling of suspended jobs to ensure no new pods are started. (TrainJob may be set to suspend in its configuration or manually paused by the user.)
- **Binding**: Bind PodGroups to TrainJobs, with their life cycle controlled by the TrainJob. For example, when a TrainJob is deleted, the associated PodGroup is also deleted.
- **Apply PodGroups to the Cluster**: Submit PodGroup resources and associated scheduling configurations, allowing Volcano to manage pod scheduling.

Note: The plugin is responsible only for configuring scheduling parameters, building and managing PodGroups. The actual scheduling management is handled by external schedulers (**volcano-controller**).

### Volcano Scheduling API

Currently, scheduling strategy parameters are set in the `PodGroupPolicy` of the `TrainingRuntimeSpec`. We introduce a new configuration struct, `VolcanoPodGroupPolicySource`, which extends the existing `PodGroupPolicySource`:

```golang
// Only one of its members may be specified.
type PodGroupPolicySource struct {
        // Coscheduling plugin from the Kubernetes scheduler-plugins for gang-scheduling.
	Coscheduling *CoschedulingPodGroupPolicySource `json:"coscheduling,omitempty"`

	// Volcano plugin from the Volcano scheduler for gang-scheduling and advanced queue-based scheduling.
	Volcano      *VolcanoPodGroupPolicySource      `json:"volcano,omitempty"`
}

// VolcanoPodPolicySource configures scheduling behavior for Volcano.
type VolcanoPodPolicySource struct {
        // NetworkTopology defines the NetworkTopology config, this field works in conjunction with network topology feature and hyperNode CRD.
        NetworkTopology *volcanov1beta1.NetworkTopologySpec `json:"networkTopology,omitempty"`
}
```

### Volcano Runtime Plugin

Similar to the Coscheduling plugin, we define the Volcano plugin struct in `pkg/runtime/framework/plugins/volcano/volcano.go`. This struct includes key fields like `client`, `restMapper`, `scheme`, and `logger`.During initialization, we need to set indexes for *TrainingRuntime* and *ClusterTrainingRuntime* to support efficient queries.

The Trainer declares the desired state of PodGroup based on the job configuration, and the Volcano controller acts on it to perform scheduling decisions.

Now, let’s dive into the specific functionality the Volcano plugin provides.

#### Create PodGroup

**PodGroup** is created based on the policy defined in `runtime.Info`. First, we need to check the existing PodGroup and corresponding TrainJob’s runtime status to determine whether to update the PodGroup. (Update the PodGroup only if it exists and the TrainJob is not suspended.)

The following parameters are calculated in both **Volcano** and **Coscheduling** plugin, with similar semantics:

- `MinMember`: Defines the minimum number of members/tasks required to run the PodGroup. This is the total count of all Pods in the PodSet.
- `MinResources`: Defines the minimal resource of members/tasks to run the pod group. This is the sum of resource requests (such as CPU and memory) for all Pods in the PodSet.

Volcano additionally supports the following parameters (**different from the Coscheduling plugin**):
- `Queue`: A collection of PodGroups, which adopts `FIFO`. It is also used as the basis for resource division.
  - It is configured via annotations `scheduling.volcano.sh/queue-name`. The field is initially set in TrainingRuntime, but can **be overridden by the TrainJob**.
- `PriorityClassName`: Represents the priority of the PodGroup and is used by the scheduler to sort all the PodGroups in the queue during scheduling.
  - It is inferred from the Pod template's `.spec.priorityClassName` field.
- `NetworkTopology`: Supports the Network Topology Aware Scheduling strategy for advanced scheduling.

> Since the current Trainer does not require fine-grained scheduling guarantees per task, we omit `minTaskMember` API and use only `minMember` to control the minimal number of Pods required to start scheduling.
#### Handle Resource Events

Referring to implement of **Coscheduling**, we update the scheduling queue in the following two cases:

- `RuntimeClass` changes. If a RuntimeClass is updated or deleted, we check for any associated **TrainJob** that is suspended. If it exists, the job will be added to the reconciliation queue.
- `LimitRange` changes. When LimitRange is created, updated, or deleted, we also check for any suspended **TrainJobs** in the affected namespace. These jobs are added to the reconciliation queue to ensure they are re-evaluated based on the new limit range.

Specifically, the Volcano plugin uses `Owns()` and `WatchRawResource()` to register event handlers for the *PodGroup* and other related Kubernetes resources (e.g. *LimitRange*) to TrainJob's Controller Manager. When changes occur in these monitored resources, it triggers the `Reconcile` loop of the TrainJob, which rebuilds objects like *JobSet* and *PodGroup*, and applies the updates to the cluster.

Additionally, we should make sure that the PodGroup is automatically cleaned up when the TrainJob is deleted. We can use Kubernetes `OwnerReferences()` to bind the PodGroup to the TrainJob, ensuring their life cycles are synchronized.

#### Configure RBAC Permissions

We should grant Trainer the necessary permissions to manage Volcano CRDs. Permissions can be declared using `+kubebuilder:rbac` annotations inside the runtime plugin code.

#### Volcano Scheduler Plugin Configuration

To enable features like [network topology-aware](https://volcano.sh/en/docs/network_topology_aware_scheduling/#configure-the-volcano-scheduler) scheduling, users need to modify the Volcano scheduler’s configuration and enable [plugins](https://volcano.sh/en/docs/plugins).
For example, the following configuration is recommended when using Trainer with Volcano integration:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: volcano-scheduler-configmap
  namespace: volcano-system
data:
  volcano-scheduler.conf: |
    actions: "enqueue, allocate, backfill"
    tiers:
    - plugins:
      - name: priority  # Handles job or task sorting based on PriorityClassName, createTime or id in turn
      - name: gang  # Gang scheduling strategy
    - plugins:
      - name: predicates  # Evaluate and pre-select jobs by the PredicateGPU
      - name: proportion  # Filter out those that require the GPU for centralized scheduling
      - name: binpack  # Help with compact task scheduling
      - name: network-topology-aware  # Enable network-topology-aware plugin
```

### Test Plan

- [x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.


#### Unit Tests

- **Volcano plugin logic**
  - PodGroup creation based on the *TrainingRuntime* spec
  - Resource calculations for `MinResources`, `MinMember`, etc.
  - PodGroup update conditions
- **Event handlers**
  - Handling of relevant Kubernetes events (e.g., LimitRange updates, RuntimeClass updates)
  - Triggering reconcile logic correctly


#### E2E tests

<!--
Describe what E2E tests will be added to ensure proper quality of the enhancement.
After the implementation PR is merged, add the names of the tests here.
-->

1. **Cluster Setup**
- Start Kind-based Kubernetes cluster
- Install Volcano from official manifest ([volcano-development.yaml](https://raw.githubusercontent.com/volcano-sh/volcano/master/installer/volcano-development.yaml))
- Deploy Trainer controller with Volcano plugin enabled
- Verify:
  - Volcano CRDs (PodGroup) are installed
  - Trainer controller is running successfully
2. **Training Job Execution**
- Submit TrainJob using Python SDK
- Verify:
  - PodGroup created and bound to job
  - Job enters Running state only when all pods are scheduled
  - Job completes successfully
  - PodGroup is deleted with job

## Implementation History

<!--
Major milestones in the lifecycle of a KEP should be tracked in this section.
Major milestones might include:
- KEP Creation
- KEP Update(s)
- Implementation Start
- First Component and Kubeflow version where the KEP is released
- Component and Kubeflow version where the KEP is graduated
- When the KEP was retired or superseded
-->

- 2025.6.2: KEP Creation
