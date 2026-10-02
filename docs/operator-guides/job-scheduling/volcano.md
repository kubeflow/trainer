# Volcano Scheduler

This guide describes how to enable **gang scheduling** and **advanced resource management** with
the [Volcano Scheduler](https://volcano.sh/en/docs/) in Kubeflow Trainer.

By integrating Volcano, you can ensure that all Pods of a training job start together (gang scheduling),
and take advantage of advanced AI-specific scheduling capabilities like priority scheduling, queue-based resource management, and
network topology–aware scheduling.

## Prerequisites

You have to [install Volcano](https://volcano.sh/en/docs/installation/) in your Kubernetes cluster before enabling the Volcano gang scheduling policy.

## Enable Volcano Plugin

Volcano scheduling can be enabled through the `podGroupPolicy` field in your `TrainJob` specification.

### Gang Scheduling

To enable gang scheduling, specify the `volcano` policy in your runtime:

```yaml
podGroupPolicy:
  volcano: {}
```

This configuration automatically creates Volcano `PodGroups` for your training job.

### Topology Aware Scheduling

Volcano also supports **network topology–aware scheduling**, which helps place Pods close to each other
to minimize communication latency in distributed training. You can configure this behavior under the volcano policy:

```yaml
podGroupPolicy:
  volcano:
    networkTopology:
      mode: hard
      highestTierAllowed: 1
```

### Using Queues for Priority Scheduling

Volcano supports queue-based resource management, where multiple PodGroups are placed in queues
and scheduled based on their priority and available capacity.

First, you have to [create a custom queue](https://volcano.sh/en/docs/tutorials/#step-1-create-a-custom-queue).

Then, reference this queue in the annotations of `TrainJob`:

```yaml
spec:
  annotations:
    scheduling.volcano.sh/queue-name: "high-priority-queue"
```

Alternatively, you can specify the queue in the annotations of **runtime** for multiple `TrainJobs`:

```yaml
spec:
  podGroupPolicy:
    volcano: {}
  template:
    metadata:
      annotations:
        scheduling.volcano.sh/queue-name: "high-priority-queue"
```

### Setting a Priority Class

A `PodGroup` is scheduled as a single gang and carries one `priorityClassName` for all of its Pods,
so a runtime declares the priority class once, on the ReplicatedJob labelled as the trainer
ancestor:

```yaml
spec:
  podGroupPolicy:
    volcano: {}
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
                  priorityClassName: "high-priority"
```

The [`PriorityClass`](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass)
must exist before the `TrainJob` is created, otherwise the `TrainJob` is rejected at admission.

Kubeflow Trainer propagates this value to the Pod template of every other ReplicatedJob in the
runtime, so the whole gang runs at the same priority rather than leaving the remaining Pods at the
cluster default. Setting `priorityClassName` on any other ReplicatedJob is rejected when the
runtime is created, since a gang has no meaningful way to run at two priorities at once.
