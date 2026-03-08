# Coscheduling

This guide describes how to enable **gang scheduling** with the
[Coscheduling plugin](https://github.com/kubernetes-sigs/scheduler-plugins/tree/master?tab=readme-ov-file#plugins)
in Kubeflow Trainer.

The Coscheduling plugin ensures that a group of Pods in the same training job
start together only when all required resources are available,

## Prerequisites

You have to [install and enable the Coscheduling plugin](https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/doc/install.md)
in your Kubernetes cluster before enabling the Coscheduling gang scheduling policy.

## Enable Gang Scheduling with Coscheduling

[The `Coscheduling` policy](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#CoschedulingPodGroupPolicySource)
configures gang scheduling with
[Coscheduling plugin](https://github.com/kubernetes-sigs/scheduler-plugins/tree/master?tab=readme-ov-file#plugins)

```yaml
podGroupPolicy:
  coscheduling:
    scheduleTimeoutSeconds: 30
```
