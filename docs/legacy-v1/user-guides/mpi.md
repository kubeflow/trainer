# MPI Training (MPIJob)

Instructions for using MPI for training

:::{admonition} Beta
:class: note
This Kubeflow component has beta status. See the [Kubeflow versioning policies](https://www.kubeflow.org/docs/started/support/#application-status). The Kubeflow team is interested in your [feedback](https://github.com/kubeflow/mpi-operator/issues) about the usability of the feature.
:::

This guide walks you through using MPI for training.

The MPI Operator, `MPIJob`, makes it easy to run allreduce-style distributed training on Kubernetes. Please check out [this blog post](https://medium.com/kubeflow/introduction-to-kubeflow-mpi-operator-and-industry-adoption-296d5f2e6edc) for an introduction to MPI Operator and its industry adoption.

Note: `MPIJob` doesn't work in a user namespace by default because of Istio [automatic sidecar injection](https://istio.io/v1.3/docs/setup/additional-setup/sidecar-injection/#automatic-sidecar-injection). In order to get it running, it needs annotation `sidecar.istio.io/inject: "false"` to disable it for either the `MPIJob` pods or namespace. To view an example of how to add this annotation to your `yaml` file, see the [TFJob documentation](tensorflow.md).

## Installation

You can deploy the operator with default settings by running the following commands:

```bash
git clone https://github.com/kubeflow/mpi-operator
cd mpi-operator
kubectl apply -f deploy/v2beta1/mpi-operator.yaml
```

Alternatively, follow the [getting started guide](https://www.kubeflow.org/docs/started/installing-kubeflow/) to deploy Kubeflow.

An alpha version of MPI support was introduced with Kubeflow 0.2.0. You must be using a version of Kubeflow newer than 0.2.0.

You can check whether the MPI Job custom resource is installed via:

```bash
kubectl get crd
```

The output should include `mpijobs.kubeflow.org` like the following:

```
NAME                                       AGE
...
mpijobs.kubeflow.org                       4d
...
```

If it is not included, you can add it as follows using [kustomize](https://github.com/kubernetes-sigs/kustomize):

```bash
git clone https://github.com/kubeflow/mpi-operator
cd mpi-operator
kustomize build manifests/overlays/kubeflow | kubectl apply -f -
```

Note that since Kubernetes v1.14, `kustomize` became a subcommand in `kubectl` so you can also run the following command instead:

Since Kubernetes v1.21, you can use:

```bash
kubectl apply -k manifests/overlays/kubeflow
```

```bash
kubectl kustomize base | kubectl apply -f -
```

## Creating an MPI Job

You can create an MPI job by defining an `MPIJob` config file. See [TensorFlow benchmark example](https://github.com/kubeflow/mpi-operator/blob/master/examples/v2beta1/tensorflow-benchmarks/tensorflow-benchmarks.yaml) config file for launching a multi-node TensorFlow benchmark training job. You may change the config file based on your requirements.

```bash
cat examples/v2beta1/tensorflow-benchmarks/tensorflow-benchmarks.yaml
```

Deploy the `MPIJob` resource to start training:

```bash
kubectl apply -f examples/v2beta1/tensorflow-benchmarks/tensorflow-benchmarks.yaml
```

## Scheduling Policy

The MPI Operator supports [gang-scheduling](job-scheduling.md). If you want to modify the PodGroup parameters, you can configure in the following:

```yaml
apiVersion: kubeflow.org/v2beta1
kind: MPIJob
metadata:
  name: tensorflow-benchmarks
spec:
  slotsPerWorker: 1
  runPolicy:
    cleanPodPolicy: Running
    schedulingPolicy:
      minAvailable: 10
      queue: test-queue
      minResources:
        cpu: 3000m
      priorityClass: high
      scheduleTimeoutSeconds: 180
  mpiReplicaSpecs:
...
```

In addition, those fields are passed to the PodGroup for the volcano or the coscheduling plugin according to the following:

- `.spec.runPolicy.schedulingPolicy.scheduleTimeoutSeconds` defines the maximal time of members to wait before run the PodGroup. However, if you use the coscheduling plugin as a gang scheduler, this is passed to `.spec.scheduleTimeoutSeconds`.
- `.spec.runPolicy.schedulingPolicy.priorityClass` defines the PodGroup's PriorityClass. However, if you use the volcano as a gang scheduler, this is passed to `.spec.priorityClassName`.
- `.spec.runPolicy.schedulingPolicy.minResources` defines the minimal resources of members to run the PodGroup and is passed to `.spec.minResources`.
- `.spec.runPolicy.schedulingPolicy.queue` defines the queue name to allocate resource for the PodGroup. However, if you use the volcano as a gang scheduler, this is passed to `.spec.queue`.
- `.spec.runPolicy.schedulingPolicy.minAvailable` defines the minimal number of members to run the PodGroup and is passed to `.spec.minMember`. When using this field, you must ensure the application supports resizing (e.g., Elastic Horovod).

Also, if you don't set the fields, the MPI Operator populates them based on the following:

volcano:

- `.spec.runPolicy.schedulingPolicy.priorityClass`: It uses the priorityClass for the launcher. If one for the launcher doesn't set, it uses one for the workers.
- `.spec.runPolicy.schedulingPolicy.minResources`: Nothing is set.
- `.spec.runPolicy.schedulingPolicy.queue`: A value of the `scheduling.volcano.sh/group-name` in `.spec.annotations`.
- `.spec.runPolicy.schedulingPolicy.minAvailable`: The number of a launcher and workers.

scheduler-plugins:

- `.spec.runPolicy.schedulingPolicy.scheduleTimeoutSeconds`: 0
- `.spec.runPolicy.schedulingPolicy.minResources`: The sum of resources defined in all containers. However, if the number of replicas (`.spec.mpiReplicaSpecs[Launcher].replicas`+`.spec.mpiReplicaSpecs[Worker].replicas`) is more of minMembers, it reorders replicas according to each priorityClass setting in `podSpec.priorityClassName` and then resources with a priority less than minMember will not be added to minResources. Note that it doesn't account for the priorityClass specified in podSpec.priorityClassName if the priorityClass doesn't exist in the cluster when it reorders replicas.
- `.spec.runPolicy.schedulingPolicy.minAvailable`: The number of a launcher and workers.

## Monitoring an MPI Job

Once the `MPIJob` resource is created, you should now be able to see the created pods matching the specified number of GPUs. You can also monitor the job status from the status section. Here is sample output when the job is successfully completed.

```bash
kubectl get -o yaml mpijobs tensorflow-benchmarks
```

```yaml
apiVersion: kubeflow.org/v2beta1
kind: MPIJob
metadata:
  creationTimestamp: "2019-07-09T22:15:51Z"
  generation: 1
  name: tensorflow-benchmarks
  namespace: default
  resourceVersion: "5645868"
  selfLink: /apis/kubeflow.org/v1alpha2/namespaces/default/mpijobs/tensorflow-benchmarks
  uid: 1c5b470f-a297-11e9-964d-88d7f67c6e6d
spec:
  runPolicy:
    cleanPodPolicy: Running
  mpiReplicaSpecs:
    Launcher:
      replicas: 1
      template:
        spec:
          containers:
          - command:
            - mpirun
            - --allow-run-as-root
            - -np
            - "2"
            - -bind-to
            - none
            - -map-by
            - slot
            - -x
            - NCCL_DEBUG=INFO
            - -x
            - LD_LIBRARY_PATH
            - -x
            - PATH
            - -mca
            - pml
            - ob1
            - -mca
            - btl
            - ^openib
            - python
            - scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py
            - --model=resnet101
            - --batch_size=64
            - --variable_update=horovod
            image: mpioperator/tensorflow-benchmarks:latest
            name: tensorflow-benchmarks
    Worker:
      replicas: 1
      template:
        spec:
          containers:
          - image: mpioperator/tensorflow-benchmarks:latest
            name: tensorflow-benchmarks
            resources:
              limits:
                nvidia.com/gpu: 2
  slotsPerWorker: 2
status:
  completionTime: "2019-07-09T22:17:06Z"
  conditions:
  - lastTransitionTime: "2019-07-09T22:15:51Z"
    lastUpdateTime: "2019-07-09T22:15:51Z"
    message: MPIJob default/tensorflow-benchmarks is created.
    reason: MPIJobCreated
    status: "True"
    type: Created
  - lastTransitionTime: "2019-07-09T22:15:54Z"
    lastUpdateTime: "2019-07-09T22:15:54Z"
    message: MPIJob default/tensorflow-benchmarks is running.
    reason: MPIJobRunning
    status: "False"
    type: Running
  - lastTransitionTime: "2019-07-09T22:17:06Z"
    lastUpdateTime: "2019-07-09T22:17:06Z"
    message: MPIJob default/tensorflow-benchmarks successfully completed.
    reason: MPIJobSucceeded
    status: "True"
    type: Succeeded
  replicaStatuses:
    Launcher:
      succeeded: 1
    Worker: {}
  startTime: "2019-07-09T22:15:51Z"
```

Training should run for 100 steps and takes a few minutes on a GPU cluster. You can inspect the logs to see the training progress. When the job starts, access the logs from the `launcher` pod:

```bash
PODNAME=$(kubectl get pods -l mpi_job_name=tensorflow-benchmarks,mpi_role_type=launcher -o name)
kubectl logs -f ${PODNAME}
```

```
TensorFlow:  1.14
Model:       resnet101
Dataset:     imagenet (synthetic)
Mode:        training
SingleSess:  False
Batch size:  128 global
             64 per device
Num batches: 100
Num epochs:  0.01
Devices:     ['horovod/gpu:0', 'horovod/gpu:1']
NUMA bind:   False
Data format: NCHW
Optimizer:   sgd
Variables:   horovod

...

40	images/sec: 154.4 +/- 0.7 (jitter = 4.0)	8.280
40	images/sec: 154.4 +/- 0.7 (jitter = 4.1)	8.482
50	images/sec: 154.8 +/- 0.6 (jitter = 4.0)	8.397
50	images/sec: 154.8 +/- 0.6 (jitter = 4.2)	8.450
60	images/sec: 154.5 +/- 0.5 (jitter = 4.1)	8.321
60	images/sec: 154.5 +/- 0.5 (jitter = 4.4)	8.349
70	images/sec: 154.5 +/- 0.5 (jitter = 4.0)	8.433
70	images/sec: 154.5 +/- 0.5 (jitter = 4.4)	8.430
80	images/sec: 154.8 +/- 0.4 (jitter = 3.6)	8.199
80	images/sec: 154.8 +/- 0.4 (jitter = 3.8)	8.404
90	images/sec: 154.6 +/- 0.4 (jitter = 3.7)	8.418
90	images/sec: 154.6 +/- 0.4 (jitter = 3.6)	8.459
100	images/sec: 154.2 +/- 0.4 (jitter = 4.0)	8.372
100	images/sec: 154.2 +/- 0.4 (jitter = 4.0)	8.542
----------------------------------------------------------------
total images/sec: 308.27
```

For a sample that uses Intel MPI, see:

```bash
cat examples/pi/pi-intel.yaml
```

## Exposed Metrics

| Metric name | Metric type | Description | Labels |
| --- | --- | --- | --- |
| mpi_operator_jobs_created_total | Counter | Counts number of MPI jobs created | |
| mpi_operator_jobs_successful_total | Counter | Counts number of MPI jobs successful | |
| mpi_operator_jobs_failed_total | Counter | Counts number of MPI jobs failed | |
| mpi_operator_job_info | Gauge | Information about MPIJob | `launcher`=<launcher-pod-name>, `namespace`=<job-namespace> |

### Join Metrics

With kube-state-metrics, one can join metrics by labels. For example `kube_pod_info * on(pod,namespace) group_left label_replace(mpi_operator_job_infos, "pod", "$0", "launcher", ".*")`

## Docker Images

We push Docker images of the mpioperator on Dockerhub for every release. You can use the following Dockerfile to build the image yourself:

- [mpi-operator](https://github.com/kubeflow/mpi-operator)

Alternative, you can build the image using make:

```bash
make RELEASE_VERSION=dev IMAGE_NAME=registry.example.com/mpi-operator images
```

This will produce an image with the tag `registry.example.com/mpi-operator:dev`.

## Contributing

Learn more in [CONTRIBUTING](https://github.com/kubeflow/mpi-operator/blob/master/CONTRIBUTING.md).
