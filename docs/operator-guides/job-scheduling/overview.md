# Overview

Kubeflow Trainer integrates with Kubernetes schedulers and queueing systems to control when and
where the TrainJob nodes run. These integrations enable gang scheduling, which ensures that a group
of related training nodes (e.g. Pods), only start when all required resources are available. Having
this is crucial when working with expensive and limited GPU accelerators.

Before exploring this guide, make sure to follow [the Runtime guide](../runtime.md)
to understand the basics of Kubeflow Trainer Runtimes.

## Supported Integrations

Kubeflow Trainer integrates with the following frameworks for job scheduling:

| Framework | Capabilities | Configuration |
| --------- | ------------ | ------------- |
| [Kueue](https://kueue.sigs.k8s.io/docs/tasks/run/trainjobs/) | Job queueing, quota-based admission, workload priorities | `kueue.x-k8s.io/queue-name` label on the TrainJob |
| [Slurm Bridge](slurm-bridge.md) | Scheduling on hybrid Kubernetes and Slurm clusters | `PodGroupPolicy` in the runtime |
| [KAI Scheduler](kai.md) | Gang scheduling, queue-based resource management | `schedulerName` in the runtime |
| [Coscheduling](coscheduling.md) | Gang scheduling | `PodGroupPolicy` in the runtime |
| [Volcano Scheduler](volcano.md) | Gang scheduling, queue-based resource management, network topology-aware scheduling | `PodGroupPolicy` in the runtime |

## PodGroupPolicy Overview

The [`PodGroupPolicy` API](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#PodGroupPolicy)
defines the configuration for gang scheduling. When this API is used Kubeflow Trainer controller
creates the appropriate PodGroup to enable gang scheduling for TrainJob.

## Types of PodGroupPolicy

The `PodGroupPolicy` API supports multiple policies, known as `PodGroupPolicySources`. Each policy
represents plugin configuration to enable gang scheduling using that specific integration. You can
specify one of the supported policies in the `PodGroupPolicy` API to enable gang scheduling with
supported plugins.

The `coscheduling` and `volcano` policies are direct `PodGroupPolicy` sources. Slurm Bridge requires
the `coscheduling` policy to create a PodGroup for gang scheduling. Kueue and KAI Scheduler use
different mechanisms: Kueue admits the TrainJob with its own quota and suspend mechanism, and KAI
Scheduler creates its PodGroup with the `podgrouper` component.

## Next Steps

- Learn how to configure job queueing and resource management with [Kueue](https://kueue.sigs.k8s.io/docs/tasks/run/trainjobs/).
- Learn how to schedule TrainJobs with [Slurm Bridge](slurm-bridge.md).
- Learn how to configure gang scheduling with [KAI Scheduler](kai.md).
- Learn how to enable gang scheduling with the [Coscheduling plugin](coscheduling.md).
- Learn how to configure advanced scheduling with [Volcano Scheduler](volcano.md).
