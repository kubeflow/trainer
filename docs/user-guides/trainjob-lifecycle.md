# Configure TrainJob Lifecycle

This guide describes how to configure execution deadlines, retention after completion or failure,
and suspension for TrainJobs.

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

The controller records the current active window in `status.startTime`. A TrainJob created suspended
has no start time until it is resumed. Suspension clears the timestamp, and each resume starts a
new window with the full deadline duration. The `activeDeadlineSeconds` field is immutable after
creation and the minimum allowed value is `1`.

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

## Clean Up Finished TrainJobs

Set `spec.ttlSecondsAfterFinished` to retain a TrainJob for a bounded time after it becomes
`Complete` or `Failed`. This includes failures caused by an exceeded active deadline.

For example, retain a finished TrainJob for one day:

```sh
kubectl patch trainjob my-trainjob --type=merge -p '{"spec":{"ttlSecondsAfterFinished":86400}}'
```

The controller calculates expiry from the true terminal condition's `lastTransitionTime` plus the
TTL. At expiry it requests foreground deletion: the TrainJob stays, marked for deletion, until
Kubernetes garbage collection removes its JobSet, Jobs, Pods, Services, and runtime snapshot. A Pod
stuck terminating, or any other finalizer, keeps the TrainJob visible longer. Pod-local logs
disappear with their Pods; logs copied to external storage follow that storage's retention policy.

The TTL is mutable, including after completion. Increasing it extends retention if deletion has
not started; reducing it can make the TrainJob eligible for immediate cleanup. Updating the TTL
does not change the recorded finish time. A value of `0` means immediate cleanup eligibility after
finishing, not disabled cleanup. An unfinished TrainJob is never deleted by TTL.

TTL cleanup is skipped for TrainJobs controlled by an OptimizationJob, which needs trial records
to track progress and results.

## Configure Runtime Defaults

Platform administrators can configure both lifecycle defaults on an existing runtime:

```sh
kubectl patch clustertrainingruntime torch-distributed --type=merge \
  -p '{"spec":{"runPolicy":{"activeDeadlineSeconds":28800,"ttlSecondsAfterFinished":3600}}}'
```

The same `spec.runPolicy` applies to a namespaced `TrainingRuntime`. Deadlines must be positive;
TTLs must be non-negative. Set lifecycle defaults in `runPolicy`; JobSet-level TTL, Job-level TTL,
and Job- or Pod-level deadlines are rejected in runtime templates.

Before upgrading, remove these fields from existing runtime templates; otherwise subsequent
runtime spec updates can fail validation.

Each TrainJob field overrides its matching runtime default independently. A TrainJob can raise or
lower a default, but cannot opt out of an inherited value. An omitted TrainJob field inherits the
runtime snapshot value. If neither source supplies the field, that policy is disabled.

Runtime defaults are captured in the TrainJob's snapshot during its first runtime reconciliation.
Editing the live runtime does not change defaults in an existing snapshot. Both runtime policy
fields remain mutable for future snapshots.

Unfinished TrainJobs created before runtime snapshots were available capture the live runtime on
their first reconciliation after upgrade. Check those runs before adding runtime deadline defaults:
their active window may already exceed the new deadline. TrainJobs that finished before the upgrade
never get a snapshot, so runtime TTL defaults do not apply to them. Set
`spec.ttlSecondsAfterFinished` on them or delete them yourself.

To clear an explicit TTL and restore snapshot inheritance:

```sh
kubectl patch trainjob my-trainjob --type=merge -p '{"spec":{"ttlSecondsAfterFinished":null}}'
```

Cleanup is disabled only if the snapshot also has no TTL default. Clearing a TrainJob TTL does not
disable an inherited runtime TTL. Use a longer TTL before cleanup starts when you need more time
to inspect a failed run.

### MultiKueue

Leave `spec.runPolicy.ttlSecondsAfterFinished` unset on runtimes in MultiKueue worker clusters.
Runtimes must be pre-created there; MultiKueue does not copy them from the manager cluster.

Until your Kueue version clears `ttlSecondsAfterFinished` from worker TrainJob copies
([kueue#14779](https://github.com/kubernetes-sigs/kueue/issues/14779)), also leave
`spec.ttlSecondsAfterFinished` unset on MultiKueue TrainJobs.

Otherwise a worker cluster can delete a finished TrainJob before its terminal status reaches the
manager cluster, and MultiKueue dispatches the workload again.

On the manager cluster, a finished MultiKueue TrainJob is cleaned up only by its own
`spec.ttlSecondsAfterFinished`. Runtime defaults do not apply there because the manager keeps no
runtime snapshot for it.

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
