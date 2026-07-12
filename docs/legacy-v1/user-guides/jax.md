# JAX Training (JAXJob)

Using JAXJob to train a model with JAX

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This page describes `JAXJob` for training a machine learning model with [JAX](https://jax.readthedocs.io/en/latest/).

The `JAXJob` is a Kubernetes [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) to run JAX training jobs on Kubernetes. The Kubeflow implementation of the `JAXJob` is in the [training-operator](https://github.com/kubeflow/training-operator).

The current custom resource for JAX has been tested to run multiple processes on CPUs using [gloo](https://github.com/facebookincubator/gloo) for communication between CPUs. Worker with replica 0 is recognized as a JAX coordinator. Process 0 will start a JAX coordinator service exposed via the IP address of process 0 in your cluster, together with a port available on that process, to which the other processes in the cluster will connect. We are looking for user feedback to run JAXJob on GPUs and TPUs.

## Creating a JAX training job

You can create a training job by defining a `JAXJob` config file. See the manifests for the [simple JAXJob example](https://github.com/kubeflow/training-operator/blob/release-1.9/examples/jax/cpu-demo/demo.yaml). You may change the Job config file based on your requirements.

Deploy the `JAXJob` resource to start training:

```bash
kubectl create -f https://raw.githubusercontent.com/kubeflow/training-operator/refs/heads/release-1.9/examples/jax/cpu-demo/demo.yaml
```

You should now be able to see the created pods matching the specified number of replicas.

```bash
kubectl get pods -n kubeflow -l training.kubeflow.org/job-name=jaxjob-simple
```

Distributed computation takes several minutes on a CPU cluster. Logs can be inspected to see its progress.

```bash
PODNAME=$(kubectl get pods -l training.kubeflow.org/job-name=jaxjob-simple,training.kubeflow.org/replica-type=worker,training.kubeflow.org/replica-index=0 -o name -n kubeflow)
kubectl logs -f ${PODNAME} -n kubeflow
```

```
I1016 14:30:28.956959 139643066051456 distributed.py:106] Starting JAX distributed service on [::]:6666
I1016 14:30:28.959352 139643066051456 distributed.py:119] Connecting to JAX distributed service on jaxjob-simple-worker-0:6666
I1016 14:30:30.633651 139643066051456 xla_bridge.py:895] Unable to initialize backend 'rocm': module 'jaxlib.xla_extension' has no attribute 'GpuAllocatorConfig'
I1016 14:30:30.638316 139643066051456 xla_bridge.py:895] Unable to initialize backend 'tpu': INTERNAL: Failed to open libtpu.so: libtpu.so: cannot open shared object file: No such file or directory
JAX process 0/1 initialized on jaxjob-simple-worker-0
JAX global devices:[CpuDevice(id=0), CpuDevice(id=131072)]
JAX local devices:[CpuDevice(id=0)]
JAX device count:2
JAX local device count:1
[2.]
```

## Monitoring a JAXJob

```bash
kubectl get -o yaml jaxjobs jaxjob-simple -n kubeflow
```

See the status section to monitor the job status. Here is sample output when the job is successfully completed.

```yaml
apiVersion: kubeflow.org/v1
kind: JAXJob
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"kubeflow.org/v1","kind":"JAXJob","metadata":{"annotations":{},"name":"jaxjob-simple","namespace":"kubeflow"},"spec":{"jaxReplicaSpecs":{"Worker":{"replicas":2,"restartPolicy":"OnFailure","template":{"spec":{"containers":[{"command":["python3","train.py"],"image":"docker.io/kubeflow/jaxjob-simple:latest","imagePullPolicy":"Always","name":"jax"}]}}}}}}
  creationTimestamp: "2024-09-22T20:07:59Z"
  generation: 1
  name: jaxjob-simple
  namespace: kubeflow
  resourceVersion: "1972"
  uid: eb20c874-44fc-459b-b9a8-09f5c3ff46d3
spec:
  jaxReplicaSpecs:
    Worker:
      replicas: 2
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - command:
                - python3
                - train.py
              image: docker.io/kubeflow/jaxjob-simple:latest
              imagePullPolicy: Always
              name: jax
status:
  completionTime: "2024-09-22T20:11:34Z"
  conditions:
    - lastTransitionTime: "2024-09-22T20:07:59Z"
      lastUpdateTime: "2024-09-22T20:07:59Z"
      message: JAXJob jaxjob-simple is created.
      reason: JAXJobCreated
      status: "True"
      type: Created
    - lastTransitionTime: "2024-09-22T20:11:28Z"
      lastUpdateTime: "2024-09-22T20:11:28Z"
      message: JAXJob kubeflow/jaxjob-simple is running.
      reason: JAXJobRunning
      status: "False"
      type: Running
    - lastTransitionTime: "2024-09-22T20:11:34Z"
      lastUpdateTime: "2024-09-22T20:11:34Z"
      message: JAXJob kubeflow/jaxjob-simple successfully completed.
      reason: JAXJobSucceeded
      status: "True"
      type: Succeeded
  replicaStatuses:
    Worker:
      selector: training.kubeflow.org/job-name=jaxjob-simple,training.kubeflow.org/operator-name=jaxjob-controller,training.kubeflow.org/replica-type=worker
      succeeded: 2
  startTime: "2024-09-22T20:07:59Z"
```
