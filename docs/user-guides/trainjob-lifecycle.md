# Configure TrainJob Lifecycle

This guide describes how to configure lifecycle policies for TrainJobs, including active deadlines
to automatically terminate long-running jobs and suspend/resume to pause and restart training.

## Prerequisites

Before exploring this guide, make sure to follow [the Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.

## Active Deadline Overview

The `activeDeadlineSeconds` field in the TrainJob spec specifies the maximum duration (in seconds)
that a TrainJob is allowed to run before the system automatically terminates it. This behavior
matches the
[Kubernetes Job `activeDeadlineSeconds`](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup)
semantics.

When the deadline is reached, all running Pods are terminated, the underlying JobSet is deleted,
and the TrainJob status is set to `Failed` with reason `DeadlineExceeded`.

The deadline timer starts from the TrainJob creation time. If the TrainJob is suspended and then
resumed, the timer resets from the resume time. The field is immutable after creation and the
minimum allowed value is `1`.

## Create TrainJob with Active Deadline

You can set `activeDeadlineSeconds` on a TrainJob to enforce a time limit. The following
example creates a TrainJob that is terminated if it runs longer than 1 hour:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-trainjob
  namespace: my-namespace
spec:
  activeDeadlineSeconds: 3600 # terminate the TrainJob after 1 hour
  runtimeRef:
    name: torch-distributed
    kind: ClusterTrainingRuntime
    apiGroup: trainer.kubeflow.org
  trainer:
    image: docker.io/my-training-image:latest
    command:
      - torchrun
      - train.py
```

### Verify the TrainJob Deadline Status

After the deadline is exceeded, the TrainJob transitions to a `Failed` state. Run the following
command to check the TrainJob conditions:

```sh
kubectl get trainjob my-trainjob -o jsonpath='{.status.conditions[?(@.status=="True")]}'
```

You should see a condition as follows:

```json
{
  "type": "Failed",
  "status": "True",
  "reason": "DeadlineExceeded",
  "message": "TrainJob exceeded its active deadline"
}
```

## Suspend and Resume TrainJob

The `suspend` field allows you to pause a running TrainJob without deleting it. When a TrainJob
is suspended, its Pods are terminated but the TrainJob resource and its configuration are preserved.
This is useful for temporarily freeing cluster resources or debugging training issues.

The following example creates a TrainJob in suspended state:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-trainjob
  namespace: my-namespace
spec:
  suspend: true
  runtimeRef:
    name: torch-distributed
    kind: ClusterTrainingRuntime
    apiGroup: trainer.kubeflow.org
```

To resume the TrainJob, update the `suspend` field to `false`:

```sh
kubectl patch trainjob my-trainjob --type=merge -p '{"spec":{"suspend":false}}'
```

:::{note}
When a TrainJob with ActiveDeadlineSeconds is resumed from suspension, the deadline timer
resets from the resume time and not the original creation time. This means the TrainJob gets
the full deadline duration after each resume.
:::

## Next Steps

- Learn more about [runtime patches](../../operator-guides/runtime-patches)
 for customizing TrainJob behavior.
- Check out [the TrainJob API reference](https://github.com/kubeflow/trainer/blob/master/pkg/apis/trainer/v1alpha1/trainjob_types.go)
 for the full list of `TrainJobSpec` fields.
