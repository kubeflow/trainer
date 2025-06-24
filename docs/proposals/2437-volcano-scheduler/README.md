# KEP-2437: Support Volcano Scheduler

## Summary

This document outlines a proposal to support Volcano for gang-scheduling in Kubeflow Trainer, so as to provide users with more AI-specific scheduling capacities like priority scheduling and queue resource management. Thanks to the [Kubeflow Trainer Pipeline Framework](https://github.com/kubeflow/trainer/tree/master/docs/proposals/2170-kubeflow-trainer-v2#pipeline-framework), we can seamlessly integrate Volcano into Kubeflow Trainer as a runtime plugin.

## Motivation

**Kubeflow Trainer** is a core component of the Kubeflow ecosystem, responsible for managing and executing distributed training jobs. In distributed training scenarios, an efficient **scheduling mechanism** is crucial:

- A distributed training job typically involves multiple pods (such as `torchrun`-based training) running in coordination. To avoid the resource wastage, all pods need to be started at the same time. That’s why **Gang Scheduling** matters.
- The default Kubernetes scheduler was initially designed for long-running services. It uses a **pod-by-pod** scheduling approach, lacking support for batch tasks. As a result, it fails to support Gang Scheduling, which is strongly required  in AI and big data scenarios.

Kubeflow Trainer V2 currently uses the **Coscheduling** plugin to provide  the Gang Scheduling support. However, it has some limitations, such as the inability to perform priority scheduling.

Introducing the **Volcano** scheduler will enhance Trainer's scheduling capabilities.This will provide users with more flexible and efficient scheduling algorithms. Specifically, it can bring the following needs and values:

1. **Provide advanced AI-specific features**
   The existing Coscheduling plugin only supports basic Gang Scheduling functions. **Volcano**, a widely adopted scheduler in the industry, offers rich AI-specific scheduling capabilities, such as priority scheduling with **Queues** for and more detailed resource management.
2. **Enrich Kubeflow Ecosystem**
   Volcano is a well-known and widely used scheduler in Kubernetes. Many users are familiar with it. We provided a Volcano scheduling option in Training Operator V1. Continuing to support Volcano in Trainer will help users migrate to Kubeflow Trainer V2 smoothly.
   Additionally, Volcano's [official documentation](https://volcano.sh/en/docs/kubeflow_on_volcano/) highlights Kubeflow as a key collaborator within its ecosystem.
3. **Mitigating limitations in edge cases**
   For example, the KubeEdge Sedna project ([kubeedge/sedna\#463](https://github.com/kubeedge/sedna/issues/463)) faced limitations when implementing edge-cloud federated learning. It was unable to set independent parameters for each Worker due to the homogeneous scheduling restrictions of the current Coscheduling setup.

### Goals

1. **Integrate Volcano Scheduler into Kubeflow Trainer.** Integrate the **Volcano** scheduler plugin into Trainer to support Gang Scheduling and resource management for distributed training jobs.
2. **Support some advanced scheduling features**. Provide some advanced scheduling features, such as prioritizing high-priority jobs and assigning specific queues.
3. **Provide user guidance**. Update the user documentation with appropriate use cases.

### Non-Goals

1. **Replace the existing Coscheduling plugin**. We maintain compatibility with the current scheduling plugin, allowing users to opt for Volcano scheduling.
2. **Modifying Volcano's core scheduling logic.** No modifications or control over the internal scheduling algorithms or mechanisms of the Volcano scheduler itself.
3. **Integration with VolcanoJob (vcjob).** This proposal will not integrate with vcjob or manage the lifecycle of vcjob within the Volcano ecosystem. We support only PodGroup-based scheduling.

## Proposal

We plan to integrate Volcano into Kubeflow Trainer as a runtime plugin, following the best practice of [Kubeflow Trainer Pipeline Framework](https://github.com/kubeflow/trainer/tree/master/docs/proposals/2170-kubeflow-trainer-v2#pipeline-framework). This plugin-based design allows users to switch to Volcano scheduler without reinstalling or modifying the core Trainer component, making the integration more modular, flexible, and user-friendly.

PodGroup is the basic scheduling unit. It is created based on the scheduling parameters specified in *Training Runtime*, after which Volcano will manage and schedule the pods specified in the PodGroup. This is similar to the approach used in Training Operator V1.

The diagram below shows how Volcano is integrated into the TrainJob creation workflow.

![user-roles](./user-roles-scheduler.drawio.svg)

As shown in the diagram, advanced scheduling is applied through a two-stage workflow:

1. Platform engineers define the scheduling strategy when customizing *ClusterTrainingRuntime* / *TrainRuntime*. This step requires familiarity with the Kubernetes API and the Volcano scheduler.
2. Data scientists then submit TrainJobs by choosing a *TrainingRuntime* with a specific scheduling method in the *TrainJob*. They don't need to understand the underlying implementation details.

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
    volcano:
      minResources:
    	  cpu: "1"
  template:
    spec:
      replicatedJobs:
        - name: Node
          template:
            spec:
              template:
                spec:
                  schedulerName: volcano
                  containers:
                    - name: trainer
                      image: docker.io/kubeflow/pytorch-mnist
                      resources:
                        limits:
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
podGroupPolicy:
    volcano:
      queue: high-priority-queue
```


## Design Details

As shown in the workflow diagram above, the Volcano plugin's work includes:

- Build PodGroups based on the *Training Runtime* configuration and calculate resource limits (e.g., `MinResource`).
- Manage PodGroups.
   - Update PodGroups and perform rescheduling when there are changes in cluster resource demands (e.g., changes in `LimitRange`).
   - Support scheduling for suspended and resumed training jobs, with special handling of suspended jobs to ensure no new pods are started. (TrainJob may be set to suspend in its configuration or manually paused by the user.)
- Bind PodGroups to TrainJobs, with their life cycle controlled by the TrainJob. For example, when a TrainJob is deleted, the associated PodGroup is also deleted.
- Integrate with Volcano control components, submitting tasks to the Volcano Scheduler for scheduling.

Note: The plugin is responsible only for configuring scheduling parameters, building and managing PodGroups. The actual scheduling management is handled by external schedulers (**volcano-controller**).

### Volcano Scheduling API

Currently, scheduling strategy parameters are set in the `PodGroupPolicy` of the `TrainingRuntimeSpec`. We need to integrate Volcano as an optional scheduler plugin.

Specifically, we create a new structure, `VolcanoPodPolicySource`, to store the Volcano scheduling configuration in `pkg/api/trainer/trainingruntime_type.go`. It will be added as an additional option within the `PodGroupPolicySource`, alongside Coscheduling. The key fields to configure are as follows:

* `Queue`: The queue name used in Volcano. Defaults to the “default” queue, which has the lowest weight.
* `PriorityClassName`: If specified, this indicates the PodGroup’s priority. (For example, "system-node-critical" and "system-cluster-critical" are special keywords that indicate the highest priorities, with the former being the highest.) This field is optional.

### Volcano Runtime Plugin

Similar to the Coscheduling plugin, we define the Volcano plugin struct in `pkg/runtime/framework/plugins/volcano/volcano.go`. This struct includes key fields like `client`, `restMapper`, `scheme`, and `logger`.During initialization, we need to set indexes for *TrainingRuntime* and *ClusterTrainingRuntime* to support efficient queries.

The **PodGroupInterface** is defined in `volcano.sh/apis/pkg/client/clientset/versioned/typed/scheduling/v1beta1`, which provides methods to work with **PodGroup** resources. The PodGroup CRD is managed by the **volcano-controller**.

Now, let’s dive into the specific functionality the Volcano plugin provides.

#### Create PodGroup

**PodGroup** is created based on the policy defined in `runtime.Info`. First, we need to check the existing PodGroup and corresponding TrainJob’s runtime status to determine whether to update the PodGroup. (Update the PodGroup only if it exists and the TrainJob is not suspended.)

Note that the PodGroup spec in **Volcano** differs from the one defined in the Kubernetes **scheduler-plugins**. In the Volcano plugin, the following parameters need to be calculated:

- `MinMember`: Defines the minimum number of members/tasks required to run the PodGroup. This is the total count of all Pods in the PodSet.
- `MinResources`: Defines the minimal resource of members/tasks to run the pod group. This is the sum of resource requests (such as CPU and memory) for all Pods in the PodSet.
- `MinTaskMember`: Defines the minimum number of Pods required to run each task in the PodGroup. If not specified, the default is the PodSet.Count.

#### Handle Resource Events

Referring to implement of **Coscheduling**, we update the scheduling queue in the following two cases:

- `RuntimeClass` changes. If a RuntimeClass is updated or deleted, we check for any associated **TrainJob** that is suspended. If it exists, the job will be added to the reconciliation queue.
- `LimitRange` changes. When LimitRange is created, updated, or deleted, we also check for any suspended **TrainJobs** in the affected namespace. These jobs are added to the reconciliation queue to ensure they are re-evaluated based on the new limit range.

Specifically, the Volcano plugin uses `Owns()` and `WatchRawResource()` to register event handlers for the *PodGroup* and other related Kubernetes resources (e.g. *LimitRange*) to TrainJob's Controller Manager. When changes occur in these monitored resources, it triggers the `Reconcile` loop of the TrainJob, which rebuilds objects like *JobSet* and *PodGroup*, and applies the updates to the cluster.

Additionally, we should make sure that the PodGroup is automatically cleaned up when the TrainJob is deleted. We can use Kubernetes `OwnerReferences()` to bind the PodGroup to the TrainJob, ensuring their life cycles are synchronized.

### Installation of Volcano plugin

The main installation steps are as follows:

1. **Install Volcano** (users must install it beforehand). A deployment YAML file ([volcano-development.yaml](https://raw.githubusercontent.com/volcano-sh/volcano/release-1.10/installer/volcano-development.yaml)) is provided. The key CRDs include *PodGroup*, *Queue*. The main control components include *controller-manager*, *admission*, and *scheduler*.
2. **Modify the RBAC permissions in the manifest.** We should ensure that Trainer has the necessary management rights for the Volcano PodGroup, Queue CRD.


### Test Plan

- [x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.


#### Unit Tests

// TODO: to update

- `<package>`: `<date>` - `<test coverage>`

#### E2E tests

<!--
Describe what E2E tests will be added to ensure proper quality of the enhancement.
After the implementation PR is merged, add the names of the tests here.
-->

#### Integration tests

Referring to the [Training Operator V1 strategy](https://github.com/kubeflow/trainer/blob/release-1.9/.github/workflows/integration-tests.yaml), integration tests validate Trainer's scheduling behavior under different **Gang-Scheduler** configurations (`none`, `coscheduling`, `volcano`). Additionally, tests cover multiple **Kubernetes** and **Python** versions.
The test flow includes:

1. **Checkout**: Clone the repository.
2. **Setup E2E Tests**: Configure the test environment, install the specified **Kubernetes** and **Python** versions, and the corresponding Gang-Scheduler.
3. **Create Custom Resources**:
   * Create `ClusterTrainingRuntime` and `TrainingRuntime` CRs with different scheduling configurations.
   * For **Volcano**, create the required `Queue` resources.
4. **Run E2E Tests**:
   * Use the Python SDK to create `TrainJob` instances and verify expected behavior across different scheduling environments.



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

