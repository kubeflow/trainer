# Job Scheduling

How to schedule a job with gang-scheduling

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This guide describes how to use [Kueue](https://kueue.sigs.k8s.io/), [Volcano Scheduler](https://github.com/volcano-sh/volcano), [KAI Scheduler](https://github.com/NVIDIA/KAI-Scheduler) and [Scheduler Plugins with coscheduling](https://github.com/kubernetes-sigs/scheduler-plugins/blob/2502825c671063af5b2aa78a1d34b24917f2def4/pkg/coscheduling/README.md) to support gang-scheduling in Kubeflow, to allow jobs to run multiple pods at the same time.

## Running jobs with gang-scheduling

The Training Operator and the MPI Operator support running jobs with gang-scheduling using Kueue, Volcano Scheduler, KAI Scheduler and Scheduler Plugins with coscheduling.

### Using Kueue with Training Operator Jobs

Follow [this guide to learn](https://kueue.sigs.k8s.io/docs/tasks/run/kubeflow/) how to use Kueue with Training Operator Jobs and manage queues for your ML training jobs

### Scheduler Plugins with coscheduling

You have to install the Scheduler Plugins with coscheduling in your cluster first as the default scheduler or a secondary scheduler of Kubernetes and configure the operator to select the scheduler name for gang-scheduling in the following:

- training-operator

```yaml
...
    spec:
      containers:
        - command:
            - /manager
            - --gang-scheduler-name=scheduler-plugins
          image: kubeflow/training-operator
          name: training-operator
...
```

- mpi-operator (installed scheduler-plugins as a default scheduler)

```yaml
...
    spec:
      containers:
      - args:
        - --gang-scheduling=default-scheduler
        - -alsologtostderr
        - --lock-namespace=mpi-operator
        image: mpioperator/mpi-operator:0.4.0
        name: mpi-operator
...
```

- mpi-operator (installed scheduler-plugins as a secondary scheduler)

```yaml
...
    spec:
      containers:
      - args:
        - --gang-scheduling=scheduler-plugins-scheduler
        - -alsologtostderr
        - --lock-namespace=mpi-operator
        image: mpioperator/mpi-operator:0.4.0
        name: mpi-operator
...
```

- Follow the [instructions in the kubernetes-sigs/scheduler-plugins repository](https://github.com/kubernetes-sigs/scheduler-plugins/blob/2502825c671063af5b2aa78a1d34b24917f2def4/doc/install.md#install-release-v0249-and-use-coscheduling) to install the Scheduler Plugins with coscheduling.

Note: The Scheduler Plugins and operator in Kubeflow achieve gang-scheduling by using [PodGroup](https://github.com/kubernetes-sigs/scheduler-plugins/blob/2502825c671063af5b2aa78a1d34b24917f2def4/pkg/coscheduling/README.md#podgroup). The Operator will create the PodGroup of the job automatically.

If you install the Scheduler Plugins in your cluster as a secondary scheduler, you need to specify the scheduler name in the CustomJob resources (e.g., TFJob), for example:

```yaml
apiVersion: "kubeflow.org/v1"
kind: TFJob
metadata:
  name: tfjob-simple
  namespace: kubeflow
spec:
  tfReplicaSpecs:
    Worker:
      replicas: 2
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: scheduler-plugins-scheduler
          containers:
            - name: tensorflow
              image: kubeflow/tf-mnist-with-summaries:latest
              command:
                - "python"
                - "/var/tf_mnist/mnist_with_summaries.py"
```

If you install the Scheduler Plugins as a default scheduler, you don't need to specify the scheduler name in CustomJob resources (e.g., TFJob).

### Volcano Scheduler

You have to install volcano scheduler in your cluster first as a secondary scheduler of Kubernetes and configure the operator to select the scheduler name for gang-scheduling in the following:

- training-operator

```yaml
...
    spec:
      containers:
        - command:
            - /manager
            - --gang-scheduler-name=volcano
          image: kubeflow/training-operator
          name: training-operator
...
```

- mpi-operator

```yaml
...
    spec:
      containers:
      - args:
        - --gang-scheduling=volcano
        - -alsologtostderr
        - --lock-namespace=mpi-operator
        image: mpioperator/mpi-operator:0.4.0
        name: mpi-operator
...
```

- Follow the [instructions in the volcano repository](https://github.com/volcano-sh/volcano) to install Volcano.

Note: Volcano scheduler and the operator in Kubeflow achieve gang-scheduling by using [PodGroup](https://volcano.sh/en/docs/podgroup/). Operator will create the PodGroup of the job automatically.

The yaml to use volcano scheduler to schedule your job as a gang is the same as non-gang-scheduler, for example:

```yaml
apiVersion: "kubeflow.org/v1beta1"
kind: "TFJob"
metadata:
  name: "tfjob-gang-scheduling"
spec:
  tfReplicaSpecs:
    Worker:
      replicas: 1
      template:
        spec:
          containers:
            - args:
                - python
                - tf_cnn_benchmarks.py
                - --batch_size=32
                - --model=resnet50
                - --variable_update=parameter_server
                - --flush_stdout=true
                - --num_gpus=1
                - --local_parameter_device=cpu
                - --device=gpu
                - --data_format=NHWC
              image: gcr.io/kubeflow/tf-benchmarks-gpu:v20171202-bdab599-dirty-284af3
              name: tensorflow
              resources:
                limits:
                  nvidia.com/gpu: 1
              workingDir: /opt/tf-benchmarks/scripts/tf_cnn_benchmarks
          restartPolicy: OnFailure
    PS:
      replicas: 1
      template:
        spec:
          containers:
            - args:
                - python
                - tf_cnn_benchmarks.py
                - --batch_size=32
                - --model=resnet50
                - --variable_update=parameter_server
                - --flush_stdout=true
                - --num_gpus=1
                - --local_parameter_device=cpu
                - --device=cpu
                - --data_format=NHWC
              image: gcr.io/kubeflow/tf-benchmarks-cpu:v20171202-bdab599-dirty-284af3
              name: tensorflow
              resources:
                limits:
                  cpu: "1"
              workingDir: /opt/tf-benchmarks/scripts/tf_cnn_benchmarks
          restartPolicy: OnFailure
```

### KAI Scheduler

Install KAI Scheduler in your cluster by following [the installation steps](https://github.com/NVIDIA/KAI-Scheduler/tree/main?tab=readme-ov-file#installation). Upon installation, KAI Scheduler automatically creates a default-queue for workload submission. KAI Scheduler's podgrouper component automatically detects Training Operator jobs and creates PodGroup resources for gang-scheduling.

See the [batch workload guide](https://github.com/NVIDIA/KAI-Scheduler/blob/main/docs/batch/README.md#pytorchjob) for an example. Once you submit, KAI scheduler automatically;

- Assigns them to default-queue for gang-scheduling
- Creates PodGroup resources
- Detects your jobs

#### Optional: Custom Queues

For advanced resource management with multiple queues:

1. Assign jobs to specific queues by adding the kai.scheduler/queue label:
2. Create queues by following the [queue configuration guide](https://github.com/NVIDIA/KAI-Scheduler/blob/main/docs/queues/README.md)

```yaml
...
    metadata:
      name: my-training-job
      labels:
        kai.scheduler/queue: my-queue-name
...
```

#### Elastic Training

KAI Scheduler supports elastic workloads, where you can specify minimum (gang threshold) and maximum pod counts. If the number of running pods falls below the minimum threshold, the entire workload is evicted. KAI Scheduler intelligently manages pod roles, prioritizing eviction of non-leader pods when possible. For elastic training with PyTorchJob, see [the elastic training guide](https://github.com/NVIDIA/KAI-Scheduler/tree/main/docs/elastic).

## About gang-scheduling

When using Volcano Scheduler, KAI Scheduler or the Scheduler Plugins with coscheduling to apply gang-scheduling, a job can run only if there are enough resources for all the pods of the job. Otherwise, all of the pods will be in a pending state waiting for enough resources. For example, if a job requiring N pods is created and there are only enough resources to schedule N-2 pods, then N pods of the job will stay pending.

Note: when under high workloads, if a pod of the job dies when the job is still running, it might give other pods a chance to occupy the resources and cause deadlock.

## Troubleshooting

If you keep getting problems related to RBAC in your volcano scheduler.

You can try to add the following rules into your clusterrole of scheduler used by the volcano scheduler.

```yaml
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
```
