# Job Scheduling

This guide describes how to enable gang scheduling with Kubeflow Trainer. It ensures that a group of
related training nodes (e.g. Pods), only start when all required resources are available. Having
this is crucial when working with expensive and limited GPU accelerators.

Before exploring this guide, make sure to follow [the Runtime guide](../runtime)
to understand the basics of Kubeflow Trainer Runtimes.

## PodGroupPolicy Overview

The [`PodGroupPolicy` API](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#PodGroupPolicy)
defines the configuration for gang scheduling. When this API is used Kubeflow Trainer controller
creates the appropriate PodGroup to enable gang scheduling for TrainJob.

## Types of PodGroupPolicy

The `PodGroupPolicy` API supports multiple policies, known as `PodGroupPolicySources`. Each policy
represents plugin configuration to enable gang scheduling using that specific integration. You can
specify one of the supported policies in the `PodGroupPolicy` API to enable gang scheduling with
supported plugins.

## Next Steps

- Learn how to enable gang scheduling with the [Coscheduling plugin](coscheduling).
- Learn how to configure advanced scheduling with [Volcano Scheduler](volcano).
- Learn how to configure job queueing and resource management with [Kueue](kueue).

```{toctree}
:hidden:
:maxdepth: 1

coscheduling
volcano
kueue
```
