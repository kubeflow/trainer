<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

**Table of Contents** _generated with [DocToc](https://github.com/thlorenz/doctoc)_

- [Motivation](#motivation)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [API (CRD and resulting objects)](#api-crd-and-resulting-objects)
  - [Custom Resource Definition](#custom-resource-definition)
  - [Container Image](#container-image)
  - [Resulting Master/Workers](#resulting-masterworkers)
    - [Master](#master)
  - [Resulting Workers](#resulting-workers)
- [Design](#design)
- [Alternatives Considered](#alternatives-considered)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

_Status_

- 2018-06-01 - Accepted
- 2018-06-14 - Implementation Started

# KEP-141: Chainer Operator

## Motivation

[Chainer][Chainer] is a Python-based, standalone open source framework for deep learning models. Chainer provides a flexible, intuitive, and high-performance means of implementing a full range of deep learning models, including state-of-the-art models such as recurrent neural networks and variational autoencoders.

[ChainerMN][ChainerMN] is an additional package for [Chainer][Chainer], which enables multi-node distributed deep learning in a scalable, flexible and easy way. [ChainerMN][ChainerMN] currently supports MPI to initialize process groups or do collective communications(e.g. broadcast, all-reduce, etc.) among processes attending the distributed learning. They are now planning to extend the support to other communication backends (e.g. [gloo][gloo] or other custom ones).

Moreover, [Chainer][Chainer]/[ChainerMN][ChainerMN] achieved to [train ResNet-50 on ImageNet in 15 Minutes](https://arxiv.org/pdf/1711.04325.pdf) in the environment equipped with GPUs and InfiniBand FDR. [The recent research](https://chainer.org/general/2018/05/25/chainermn-v1-3.html) revealed that [ChainerMN][ChainerMN]'s latest feature (Double-buffering and All-Reduce in half-precision float values) enables users to expect _almost_ linear scalability without sacrificing model accuracy even in environments (e.g. AWS) which doesn't equip InfiniBand.

However, [Chainer][Chainer]/[ChainerMN][ChainerMN] currently does not have an operator/controller for Kubernetes. This proposal is aimed at defining what the operator should behave, and add it to Kubeflow.

## Goals

A Kubeflow user should be able to run training using [Chainer][Chainer]/[ChainerMN][ChainerMN] as easily as then can using Tensorflow/PyTorch. This proposal is centered around a Kubernetes operator for [Chainer]/[ChainerMN]. A user should be able to run both single node with [Chainer][Chainer] and distributed training jobs with [ChainerMN][ChainerMN].

This proposal defines the following:

- A Chainer operator
- A way to deploy the operator with ksonnet
- A single pod Chainer example
- A distributed (multiple pods) Chainer example

## Non-Goals

Currently, for the scope of this proposal, we won't be addressing the method for serving the model.

## API (CRD and resulting objects)

### Custom Resource Definition

```yaml
apiVersion: kubeflow.org/v1alpha1
kind: ChainerJob
metadata:
  name: example-chainermn-job
spec:
  # only "mpi" is supported in the first scope
  # "gloo", or custom backend will be supported in the future.
  backend: mpi
  # chief would be better like TfJorb?
  master:
    # replicas of master can be ommitted but must be 1 in master.
    replicas: 1
    # In master, only backoffLimit/activeDeadlineSeconds
    # are supported for customizing resulting master `Job` behavior
    backoffLimit: 5
    activeDeadlineSeconds: 100
    template:
      spec:
        containers:
          - image: everpeace/chainermn:1.3.0
            name: master
            imagePullPolicy: IfNotPresent
            command: ["mpiexec"]
            args:
              [
                "-n",
                "3",
                "-N",
                "1",
                "python3",
                "/train_mnist.py",
                "-e",
                "2",
                "-b",
                "100",
                "-g",
              ]
        restartPolicy: OnFailure
  worker:
    replicas: 3
    template:
      spec:
        containers:
          - image: everpeace/chainermn:1.3.0
            name: worker
        restartPolicy: OnFailure
```

This `ChainerJob` resembles the existing `TfJob`/`PyTorchJob`. The main differences are being the omission of `masterPort` options.

`backend` defines the protocol the [ChainerMN][ChainerMN] processes will use to communicate when initializing the worker group. As stated above, [ChainerMN][ChainerMN] currently support MPI only for backend. But they are now planning to extend the support to other communication backend (e.g. [gloo][gloo] or other custom ones).

### Container Image

When `backend: mpi`, the same assumption with [mpi-operator](mpi-operator-proposal.md) would be applied. In addition, to bring out the best performance with CUDA and NVIDIA GPU power, CUDA-aware MPI should be built and installed in the container image.

### Resulting Master/Workers

This resulting master/workers resembles ones in [mpi-operator](mpi-operator-proposal.md) very much. It is because that when `backend: mpi`, the main mission of chainer operator would be a setup of MPI cluster on Kubernetes which is failt-tolerant in some extent.

The difference is that one of master's initContainers makes sure all the cluster pods are up and can connect to them with `kubectl exec`. It is because that it makes chainer-operator not to needs to watch failure of jobs or StatefulSets. This simplifies implementation of chainer-operator.

#### Master

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job-id}-master
spec:
  backoffLimit: 5
  activeDeadlineSeconds: 100
  template:
    spec:
      initContainers:
        # download the latest kubectl
      - name: chainer-operator-kubectl-downloader
        image: alpine:latest # it can be any container image with sh
        command: [ "$(ASSETS_DIR)/download_kubectl.sh" ]
        args: [ "$(TARGET_DIR)" ]
        volumeMounts:
        - name: chainer-operator-kubectl
          mountPath: /kubeflow/chainer-operator/kube
        - name: chainer-operator-assets
        env:
        - name: TARGET_DIR
          value: /kubeflow/chainer-operator/kube
        # ensure all pods are up and can connect.
      - name: chainer-operator-cluster-waiter
        image: alpine:latest # it can be any container image with sh
        volumeMounts:
        - name: chainer-operator-kubectl
          mountPath: /kubeflow/chainer-operator/kube
        - name: chainer-operator-assets
          mountPath: /kubeflow/chainer-operator/assets
        env:
        - name: ASSETS_DIR
          value: /kubeflow/chainer-operator/assets
        - name: KUBECTL_DIR
          value: /kubeflow/chainer-operator/kube
        command: [ "$(ASSETS_DIR)/cluster_waiter.sh" ]
        args: [ "$(TARGET_DIR)/hostfile" ]
      containers:
      - name: master
        image: everpeace/chainermn:1.3.0
        volumeMounts:
        - name: chainer-operator-kubectl
          mountPath: /kubeflow/chainer-operator/kube
        - name: chainer-operator-assets
          mountPath: /kubeflow/chainer-operator/assets
        env:
        - name: OMPI_MCA_plm_rsh_agent
          value: /kubeflow/chainer-operator/assets/kubexec.sh
        - name: OMPI_MCA_orte_default_hostfile
          value: /kubeflow/chainer-operator/assets/hostfile
        - name: KUBECTL_DIR
          value: /kubeflow/chainer-operator/kube
      restartPolicy: OnFailure
      serviceAccountName: ${job-id}-launcher
      volumes:
      - name: chainer-operator-kubectl
        emptyDir: {}
      - name: chainer-operator-assets
        configMap:
          name: ${job-id}-chainer-operator-assets
          items:
          - key: kubexec.sh
            path: kubexec.sh
            mode: 365
          - key: cluster_waiter.sh
            path: cluster_waiter.sh
            mode: 365
          - key: hostfile
            path: hostfile
            mode: 292
```

### Resulting Workers

```yaml
# Service resource is omitted.
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${job-id}-worker
spec:
  podManagementPolicy: Parallel
  replicas: 3
  selector:
    matchLabels:
      app: ${job-id}-worker
  serviceName: ${job-id}-worker
  template:
    metadata:
      labels:
        app: ${job-id}-worker
    spec:
      initContainers:
      - name: chainer-operator-kubectl-downloader
        # .. this is identical with master's one.
      containers:
      - name: worker
        image: everpeace/chainermn:1.3.0
        command:
        - sh
        - -c
        - while true; do sleep & wait; done
        env:
        - name: KUBECTL_DIR
          value: /kubeflow/chainer-operator/kube
        volumeMounts:
        - name: chainer-operator-assets
          mountPath: /kubeflow/chainer-operator/assets
        - name: chainer-operator-kubectl
          mountPath: /kubeflow/chainer-operator/kube
    serviceAccountName: ${job-id}-launcher
    volumes:
    - name: chainer-operator-kubectl
      emptyDir: {}
    - name: chainer-operator-assets
      configMap:
        name: ${job-id}-chainer-operator-assets
        items:
        - key: kubexec.sh
          path: kubexec.sh
          mode: 365
```

## Design

The sequence is very similar to [mpi-operator](mpi-operator-proposal.md#Design).

- create `ConfigMap` which includes assets (`kubexec.sh`, `hostfile`, `cluster_waiter.sh`).
- create `ServiceAccount` and `RoleBinding` for the job which only can `kube exec` to cluster pods `{job-id}-master, {job-id}-worker-[0,...,n-1]`.
  - this means service account of chainer-operator must have permission to create them.
- create `Job` for `master` and `StatefulSet` for `workers`
  - the cluster will be fault-tolerant on pod level (which means it can be retried automatically).
  - chainer-operator needs not to wait for pods in the `StatefulSet` are up and can connect to them because `master` pod has `initContainer` to do it.
- When `Job` finishes (even when `DeadlineExceeded`), it will scale `StatefulSet` to `0`.

## Alternatives Considered

We know [mpi-operator](mpi-operator-proposal.md) is already proposed. As a design alternative, chiner-operator could emit `kind: MPIJob` custom resource instead of emitting similar constructs.

Please be noted that [ChainerMN][ChainerMN] is now planning to expand backend support other than MPI. So, even in the case which chainer-operator just emmits `kind: MPIJob` resources, chainer-operator would be worth to introduce.

[ChainerMN]: https://github.com/chainer/chainermn
[Chainer]: https://chainer.org
[gloo]: https://github.com/facebookincubator/gloo
