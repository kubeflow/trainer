# TensorFlow Training (TFJob)

Using TFJob to train a model with TensorFlow

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This page describes `TFJob` for training a machine learning model with [TensorFlow](https://www.tensorflow.org/).

## What is TFJob?

`TFJob` is a Kubernetes [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) to run TensorFlow training jobs on Kubernetes. The Kubeflow implementation of `TFJob` is in the [training-operator](https://github.com/kubeflow/training-operator).

Note: `TFJob` doesn't work in a user namespace by default because of Istio [automatic sidecar injection](https://istio.io/v1.3/docs/setup/additional-setup/sidecar-injection/#automatic-sidecar-injection). In order to get `TFJob` running, it needs the annotation `sidecar.istio.io/inject: "false"` to disable it for `TFJob` pods.

A `TFJob` is a resource with a YAML representation like the one below (edit to use the container image and command for your own training code):

```yaml
apiVersion: kubeflow.org/v1
kind: TFJob
metadata:
  generateName: tfjob
  namespace: your-user-namespace
spec:
  tfReplicaSpecs:
    PS:
      replicas: 1
      restartPolicy: OnFailure
      template:
        metadata:
          annotations:
            sidecar.istio.io/inject: "false"
        spec:
          containers:
            - name: tensorflow
              image: gcr.io/your-project/your-image
              command:
                - python
                - -m
                - trainer.task
                - --batch_size=32
                - --training_steps=1000
    Worker:
      replicas: 3
      restartPolicy: OnFailure
      template:
        metadata:
          annotations:
            sidecar.istio.io/inject: "false"
        spec:
          containers:
            - name: tensorflow
              image: gcr.io/your-project/your-image
              command:
                - python
                - -m
                - trainer.task
                - --batch_size=32
                - --training_steps=1000
```

If you want to give your `TFJob` pods access to credentials secrets, such as the Google Cloud credentials automatically created when you do a GKE-based Kubeflow installation, you can mount and use a secret like this:

```yaml
apiVersion: kubeflow.org/v1
kind: TFJob
metadata:
  generateName: tfjob
  namespace: your-user-namespace
spec:
  tfReplicaSpecs:
    PS:
      replicas: 1
      restartPolicy: OnFailure
      template:
        metadata:
          annotations:
            sidecar.istio.io/inject: "false"
        spec:
          containers:
            - name: tensorflow
              image: gcr.io/your-project/your-image
              command:
                - python
                - -m
                - trainer.task
                - --batch_size=32
                - --training_steps=1000
              env:
                - name: GOOGLE_APPLICATION_CREDENTIALS
                  value: "/etc/secrets/user-gcp-sa.json"
              volumeMounts:
                - name: sa
                  mountPath: "/etc/secrets"
                  readOnly: true
          volumes:
            - name: sa
              secret:
                secretName: user-gcp-sa
    Worker:
      replicas: 1
      restartPolicy: OnFailure
      template:
        metadata:
          annotations:
            sidecar.istio.io/inject: "false"
        spec:
          containers:
            - name: tensorflow
              image: gcr.io/your-project/your-image
              command:
                - python
                - -m
                - trainer.task
                - --batch_size=32
                - --training_steps=1000
              env:
                - name: GOOGLE_APPLICATION_CREDENTIALS
                  value: "/etc/secrets/user-gcp-sa.json"
              volumeMounts:
                - name: sa
                  mountPath: "/etc/secrets"
                  readOnly: true
          volumes:
            - name: sa
              secret:
                secretName: user-gcp-sa
```

If you are not familiar with Kubernetes resources please refer to the page [Understanding Kubernetes Objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/).

What makes `TFJob` different from built in [controllers](https://kubernetes.io/docs/concepts/workloads/controllers/) is that the `TFJob` spec is designed to manage [distributed TensorFlow training jobs](https://www.tensorflow.org/guide/distributed_training).

A distributed TensorFlow job typically contains 0 or more of the following processes

- **Evaluator** The evaluators can be used to compute evaluation metrics as the model is trained.
- **Worker** The workers do the actual work of training the model. In some cases, worker 0 might also act as the chief.
- **Ps** The ps are parameter servers; these servers provide a distributed data store for the model parameters.
- **Chief** The chief is responsible for orchestrating training and performing tasks like checkpointing the model.

The field tfReplicaSpecs in `TFJob` spec contains a map from the type of replica (as listed above) to the TFReplicaSpec for that replica. TFReplicaSpec consists of 3 fields

- **replicas** The number of replicas of this type to spawn for this `TFJob`.
- **template** A [PodTemplateSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#podtemplatespec-v1-core) that describes the pod to create for each replica. The pod must include a container named `tensorflow`.
- **restartPolicy** Determines whether pods will be restarted when they exit. The allowed values are as follows:
  - **Always** means the pod will always be restarted. This policy is good for parameter servers since they never exit and should always be restarted in the event of failure.
  - **OnFailure** means the pod will be restarted if the pod exits due to failure. This policy is good for chief and workers. An exit code of 0 indicates success and the pod will not be restarted. A non-zero exit code indicates a failure.
  - **ExitCode** means the restart behavior is dependent on the exit code of the `tensorflow` container as follows:
    - Exit code `0` indicates the process completed successfully and will not be restarted.
    - The following exit codes indicate a permanent error and the container will not be restarted: `139`, `128`, `127`, `126`, `2`, `1`.
    - The following exit codes indicate a retryable error and the container will be restarted: `143`, `137`, `130`, `138`.
  - **Never** means pods that terminate will never be restarted. This policy should rarely be used because Kubernetes will terminate pods for any number of reasons (e.g. node becomes unhealthy) and this will prevent the job from recovering.

For background information on exit codes, see the [GNU guide to termination signals](https://www.gnu.org/software/libc/manual/html_node/Termination-Signals.html) and the [Linux Documentation Project](https://tldp.org/LDP/abs/html/exitcodes.html).

## Running the Mnist example

See the manifests for the [distributed MNIST example](https://github.com/kubeflow/training-operator/blob/release-1.9/examples/tensorflow/simple.yaml). You may change the config file based on your requirements.

Deploy the `TFJob` resource to start training:

```bash
kubectl create -f https://raw.githubusercontent.com/kubeflow/training-operator/refs/heads/release-1.9/examples/tensorflow/simple.yaml
```

Monitor the job (see the [detailed guide below](#monitoring-your-job)):

```bash
kubectl -n kubeflow get tfjob tfjob-simple -o yaml
```

Delete it

```bash
kubectl -n kubeflow delete tfjob tfjob-simple
```

## Customizing the TFJob

Typically you can change the following values in the `TFJob` yaml file:

- Change the image to point to the docker image containing your code
- Change the number and types of replicas
- Change the resources (requests and limits) assigned to each resource
- Set any environment variables (e.g., configure various environment variables to talk to datastores like GCS or S3)
- Attach PVs if you want to use PVs for storage.

## Using GPUs

To use GPUs your cluster must be configured to use GPUs.

For more information:

- [EKS instructions](https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html)
- [GKE instructions](https://cloud.google.com/kubernetes-engine/docs/concepts/gpus)
- [Kubernetes instructions for scheduling GPUs](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/)

To attach GPUs specify the GPU resource on the container in the replicas that should contain the GPUs; for example:

```yaml
apiVersion: "kubeflow.org/v1"
kind: "TFJob"
metadata:
  name: "tf-smoke-gpu"
spec:
  tfReplicaSpecs:
    PS:
      replicas: 1
      template:
        metadata:
          creationTimestamp: null
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
              ports:
                - containerPort: 2222
                  name: tfjob-port
              resources:
                limits:
                  cpu: "1"
              workingDir: /opt/tf-benchmarks/scripts/tf_cnn_benchmarks
          restartPolicy: OnFailure
    Worker:
      replicas: 1
      template:
        metadata:
          creationTimestamp: null
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
              ports:
                - containerPort: 2222
                  name: tfjob-port
              resources:
                limits:
                  nvidia.com/gpu: 1
              workingDir: /opt/tf-benchmarks/scripts/tf_cnn_benchmarks
          restartPolicy: OnFailure
```

Follow TensorFlow's [instructions](https://www.tensorflow.org/guide/gpu) for using GPUs.

## Monitoring your job

To get the status of your job

```bash
kubectl -n kubeflow get -o yaml tfjobs tfjob-simple
```

Here is sample output for an example job

```yaml
apiVersion: kubeflow.org/v1
kind: TFJob
metadata:
  creationTimestamp: "2021-09-06T11:48:09Z"
  generation: 1
  name: tfjob-simple
  namespace: kubeflow
  resourceVersion: "5764004"
  selfLink: /apis/kubeflow.org/v1/namespaces/kubeflow/tfjobs/tfjob-simple
  uid: 3a67a9a9-cb89-4c1f-a189-f49f0b581e29
spec:
  tfReplicaSpecs:
    Worker:
      replicas: 2
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - command:
                - python
                - /var/tf_mnist/mnist_with_summaries.py
              image: gcr.io/kubeflow-ci/tf-mnist-with-summaries:1.0
              name: tensorflow
status:
  completionTime: "2021-09-06T11:49:30Z"
  conditions:
    - lastTransitionTime: "2021-09-06T11:48:09Z"
      lastUpdateTime: "2021-09-06T11:48:09Z"
      message: TFJob tfjob-simple is created.
      reason: TFJobCreated
      status: "True"
      type: Created
    - lastTransitionTime: "2021-09-06T11:48:12Z"
      lastUpdateTime: "2021-09-06T11:48:12Z"
      message: TFJob kubeflow/tfjob-simple is running.
      reason: TFJobRunning
      status: "False"
      type: Running
    - lastTransitionTime: "2021-09-06T11:49:30Z"
      lastUpdateTime: "2021-09-06T11:49:30Z"
      message: TFJob kubeflow/tfjob-simple successfully completed.
      reason: TFJobSucceeded
      status: "True"
      type: Succeeded
  replicaStatuses:
    Worker:
      succeeded: 2
  startTime: "2021-09-06T11:48:10Z"
```

### Conditions

A `TFJob` has a `TFJobStatus`, which has an array of `TFJobConditions` through which the `TFJob` has or has not passed. Each element of the `TFJobCondition` array has six possible fields:

The type field is a string with the following possible values:

- **TFJobFailed** means the job has failed.
- **TFJobSucceeded** means the job completed successfully.
- **TFJobRestarting** means one or more sub-resources (e.g. services/pods) of this `TFJob` had a problem and is being restarted.
- **TFJobRunning** means all sub-resources (e.g. services/pods) of this `TFJob` have been successfully scheduled and launched and the job is running.
- **TFJobCreated** means the `TFJob` has been accepted by the system, but one or more of the pods/services has not been started.

Success or failure of a job is determined as follows

If the restartPolicy allows for restarts then the process will just be restarted and the `TFJob` will continue to execute.

- If the restartPolicy doesn't allow restarts a non-zero exit code is considered a permanent failure and the job is marked failed.
- For the restartPolicy ExitCode the behavior is exit code dependent.

### tfReplicaStatuses

tfReplicaStatuses provides a map indicating the number of pods for each replica in a given state. There are three possible states

- **Failed** is the number of pods that completed with an error.
- **Succeeded** is the number of pods that completed successfully.
- **Active** is the number of currently running pods.

### Events

During execution, `TFJob` will emit events to indicate whats happening such as the creation/deletion of pods and services. Kubernetes doesn't retain events older than 1 hour by default. To see recent events for a job run

```bash
kubectl -n kubeflow describe tfjobs tfjob-simple
```

## TensorFlow Logs

Logging follows standard K8s logging practices.

You can use kubectl to get standard output/error for any pods that haven't been deleted.

First find the pod created by the job controller for the replica of interest. Pods will be named

```
${JOBNAME}-${REPLICA-TYPE}-${INDEX}
```

Once you've identified your pod you can get the logs using kubectl.

```bash
kubectl logs ${PODNAME}
```

The CleanPodPolicy in the `TFJob` spec controls deletion of pods when a job terminates. The policy can be one of the following values

- **None** means that no pods will be deleted when the job completes.
- **All** means all pods even completed pods will be deleted immediately when the job finishes.
- **Running** means that only pods still running when a job completes (e.g. parameter servers) will be deleted immediately; completed pods will not be deleted so that the logs will be preserved. This is the default value.

If your cluster takes advantage of Kubernetes [cluster logging](https://kubernetes.io/docs/concepts/cluster-administration/logging/) then your logs may also be shipped to an appropriate data store for further analysis.

## Troubleshooting

Here are some steps to follow to troubleshoot your job

1. Is a status present for your job? Run the command

```bash
kubectl -n ${USER_NAMESPACE} get tfjobs -o yaml ${JOB_NAME}
```

USER_NAMESPACE is the namespace created for your user profile.

If the resulting output doesn't include a status for your job then this typically indicates the job spec is invalid.

If the `TFJob` spec is invalid there should be a log message in the tf operator logs

```bash
kubectl -n ${KUBEFLOW_NAMESPACE} logs `kubectl get pods --selector=name=tf-job-operator -o jsonpath='{.items[0].metadata.name}'`
```

KUBEFLOW_NAMESPACE is the namespace you deployed the `TFJob` operator in.

2. Check the events for your job to see if the pods were created

There are a number of ways to get the events; if your job is less than 1 hour old then you can do

```bash
kubectl -n ${USER_NAMESPACE} describe tfjobs ${JOB_NAME}
```

The bottom of the output should include a list of events emitted by the job.

Kubernetes only preserves events for 1 hour (see [kubernetes/kubernetes#52521](https://github.com/kubernetes/kubernetes/issues/52521))

3. If the pods and services aren't being created then this suggests the `TFJob` isn't being processed; common causes are

- The `TFJob` operator isn't running
- The `TFJob` spec is invalid (see above)

4. Check the events for the pods to ensure they are scheduled.

```bash
kubectl -n ${USER_NAMESPACE} describe pods ${POD_NAME}
```

Some common problems that can prevent a container from starting are

- The docker image doesn't exist or can't be accessed (e.g due to permission issues)
- The pod tries to mount a volume (or secret) that doesn't exist or is unavailable
- Insufficient resources to schedule the pod

5. If the containers start; check the logs of the containers following the instructions in the previous section.

## Next steps

Learn about [distributed training](../reference/distributed-training.md) in Training Operator.

See how to [run a job with gang-scheduling](job-scheduling.md).
