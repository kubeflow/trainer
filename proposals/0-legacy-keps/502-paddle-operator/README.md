# KEP-502: Paddle Operator Proposal

## Motivation

[PaddlePaddle](https://github.com/paddlepaddle) is a widely used machine learning framework in China. However there is no easy way to launch PaddlePaddle training jobs on Kubernetes. By providing a CRD and a custom controller, we can make PaddlePaddle distributed training simple for end users. For more information about PaddlePaddle, please check our [website](https://www.paddlepaddle.org.cn/).

## Goals

Kubeflow user should be able to run training using PaddlePaddle easily on Kubernetes. This will be implemented by using Kubernetes operator. An end user can run training jobs with PaddlePaddle on local or cloud.

The proposal defines the followings:

- Provide a Custom Resource Definition (CRD) for defining PaddlePaddle training job, currently supports running two distributed tasks, ParameterServer (PS) and Collective.
- Implement a controller to manage the CRD, create dependent resources, and reconcile to the desired states.
- The script for operator and controller deployment.
- Several distributed PaddlePaddle training examples.

## Non-Goals

For the model serving part, it will not be included in the paddle-operator.

## API (CRD and resulting objects)

```
    deploy
    |-- examples
    |   |-- resnet.yaml
    |   |-- wide_and_deep.yaml
    |   |-- wide_and_deep_podip.yaml
    |   |-- wide_and_deep_service.yaml
    |   `-- wide_and_deep_volcano.yaml
    |-- v1
    |   |-- crd.yaml
    |   `-- operator.yaml
    `-- v1beta1
        |-- crd.yaml
        `-- operator.yaml
```

### Custom Resource Definition

The custom resource definition yaml example is as following:

```yaml
apiVersion: batch.paddlepaddle.org/v1
kind: PaddleJob
metadata:
  name: wide-ande-deep
spec:
  withGloo: 1
  intranet: PodIP
  cleanPodPolicy: OnCompletion
  worker:
    replicas: 2
    template:
      spec:
        containers:
          - name: paddle
            image: registry.baidubce.com/paddle-operator/demo-wide-and-deep:v1
  ps:
    replicas: 2
    template:
      spec:
        containers:
          - name: paddle
            image: registry.baidubce.com/paddle-operator/demo-wide-and-deep:v1
```

- The optional configuration of withGloo is 0 not enabled, 1 only starts the worker side, 2 starts all (worker and server), it is recommended to set 1;

- The cleanPodPolicy can be optionally configured as Always/Never/OnFailure/OnCompletion, which indicates whether to delete the pod when the task is terminated (failed or successful). It is recommended to Never during debugging and OnCompletion during production;

- The intranet can be optionally configured as Service/PodIP, which means the communication method between pods. The user does not need to configure it, and PodIP is used by default;

- The content of ps and worker is podTemplateSpec. Users can add more content according to the Kubernetes specification, such as GPU configuration.

We also provide PaddlePaddle collective mode with GPU.

```
apiVersion: batch.paddlepaddle.org/v1
kind: PaddleJob
metadata:
  name: resnet
spec:
  cleanPodPolicy: Never
  worker:
    replicas: 2
    template:
      spec:
        containers:
          - name: paddle
            image: registry.baidubce.com/paddle-operator/demo-resnet:v1
            command:
            - python
            args:
            - "-m"
            - "paddle.distributed.launch"
            - "train_fleet.py"
            volumeMounts:
            - mountPath: /dev/shm
              name: dshm
            resources:
              limits:
                nvidia.com/gpu: 1
        volumes:
        - name: dshm
          emptyDir:
            medium: Memory
```

- Here you need to add shared memory to mount to prevent cache errors;

- This example uses the built-in data set. After the program is started, it will be downloaded. Depending on the network environment, it may wait a long time.

### Resulting Master

```yaml
apiVersion: v1
kind: Service
metadata:
  name: wide-ande-deep-service-ps-0
  namespace: paddle-system
  ownerReferences:
    - apiVersion: batch.paddlepaddle.org/v1
      blockOwnerDeletion: true
      controller: true
      kind: PaddleJob
      name: wide-ande-deep-service
      uid: 8f432e67-8cda-482c-b147-91f9d4400067
  resourceVersion: "9513616"
  selfLink: /api/v1/namespaces/paddle-system/services/wide-ande-deep-service-ps-0
  uid: e274db1e-ee7f-4b6d-bc0c-034c32f4b7a1
spec:
  clusterIP: None
  ports:
    - port: 2379
      protocol: TCP
      targetPort: 2379
  selector:
    paddle-res-name: wide-ande-deep-service-ps-0
  sessionAffinity: None
  type: ClusterIP
```

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: wide-ande-deep-ps-0
  namespace: paddle-system
  ownerReferences:
    - apiVersion: batch.paddlepaddle.org/v1
      blockOwnerDeletion: true
      controller: true
      kind: PaddleJob
      name: wide-ande-deep
      uid: f206587f-5dee-46f5-9399-e835bde02487
  resourceVersion: "9506900"
  selfLink: /api/v1/namespaces/paddle-system/pods/wide-ande-deep-ps-0
  uid: 36b27c8f-9712-474b-b21b-dd6b54aaef29
spec:
  containers:
    - env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        - name: PADDLE_TRAINER_ID
          value: "0"
        - name: TRAINING_ROLE
          value: PSERVER
        - name: PADDLE_TRAINING_ROLE
          value: PSERVER
      envFrom:
        - configMapRef:
            name: wide-ande-deep
      image: registry.baidubce.com/paddle-operator/demo-wide-and-deep:v1
```

### Resulting Worker

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: wide-ande-deep-worker-0
  namespace: paddle-system
  ownerReferences:
    - apiVersion: batch.paddlepaddle.org/v1
      blockOwnerDeletion: true
      controller: true
      kind: PaddleJob
      name: wide-ande-deep
      uid: f206587f-5dee-46f5-9399-e835bde02487
  resourceVersion: "9507629"
  selfLink: /api/v1/namespaces/paddle-system/pods/wide-ande-deep-worker-0
  uid: e8534fe6-7c2e-4849-9a99-ffdcd5df76bb
spec:
  containers:
    - env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        - name: PADDLE_TRAINER_ID
          value: "0"
        - name: TRAINING_ROLE
          value: TRAINER
        - name: PADDLE_TRAINING_ROLE
          value: TRAINER
      envFrom:
        - configMapRef:
            name: wide-ande-deep
      image: registry.baidubce.com/paddle-operator/demo-wide-and-deep:v1
```

The worker spec generates a pod. Currently worker will communicate to the master through the master's service name, we'll use a service registry for service discovery.

## Design

Here are some original design docs for paddle-perator on Kubernetes.

- Paddle Operator Architecture on Kubernetes, please check out [design-arch](https://github.com/PaddleFlow/paddle-operator/blob/main/docs/design-arch.md)
- Paddle training job instance fault tolerant, please check out [design-fault-tolerant](https://github.com/PaddleFlow/paddle-operator/blob/main/docs/design-fault-tolerant.md)
- Co-scheduling training job to prevent job instances from resource deadlock, please check out [design-coschedule](https://github.com/PaddleFlow/paddle-operator/blob/main/docs/design-coschedule.md)

## Alternatives Considered

One option is to add PaddlePaddle support to the existing tf-operator, but the parameters and operations between two frameworks are quite different. Combining them may make the user experience unnecessarily complicated.

## Current Status

We recently refactored the paddle-operator for better performance and code readability. And we'll merge the dev branch back into main branch soon, so our real code branch is `dev` at this moment.
