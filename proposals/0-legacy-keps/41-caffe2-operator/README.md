<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

**Table of Contents** _generated with [DocToc](https://github.com/thlorenz/doctoc)_

- [Motivation](#motivation)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [API (CRD and resulting objects)](#api-crd-and-resulting-objects)
  - [Custom Resource Definition](#custom-resource-definition)
  - [Resulting Master](#resulting-master)
  - [Resulting Worker](#resulting-worker)
- [Design](#design)
- [Other backends](#other-backends)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

_Status_

- 2018-04-06 - Accepted

# KEP-41: Caffe2 Operator

## Motivation

Caffe2 is a popular machine learning framework which currently does not have an operator/controller for Kubernetes. This proposal is aimed at defining what that operator should look like, and adding it to Kubeflow.

For distributed training, Caffe2 has no parameter server compared with Tensorflow, so it has to use Redis/MPI to find the other nodes to communicate.

## Goals

A Kubeflow user should be able to run training using Caffe2 as easily as they can using Tensorflow. This proposal is centered around a Kubernetes operator for Caffe2. A user should be able to run both single node and distributed training jobs with Caffe2.

This proposal defines the following:

- A Caffe2 operator
- A way to deploy the operator with kubectl
- A single pod Caffe2 example
- A distributed Caffe2 example
- A distributed Caffe2 proposal with [batchd scheduler](https://github.com/kubernetes-incubator/kube-arbitrator)

## Non-Goals

For the scope of this proposal, we won't be addressing the method for serving the model.

## API (CRD and resulting objects)

### Custom Resource Definition

The custom resource submitted to the Kubernetes API would look something like this:

```yaml
apiVersion: "kubeflow.org/v1alpha1"
kind: "Caffe2Job"
metadata:
  name: "example-job"
spec:
  backend: "redis"
  replicaSpecs:
    - replicas: 1
      ReplicaType: MASTER
      template:
        spec:
          hostNetwork: true
          containers:
            - image: carmark/caffe2:latest
              name: caffe2
              securityContext:
                capabilities:
                  add: ["ALL"]
          restartPolicy: Never
    - replicas: 2
      ReplicaType: WORKER
      template:
        spec:
          hostNetwork: true
          containers:
            - image: carmark/caffe2:latest
              securityContext:
                capabilities:
                  add: ["ALL"]
              name: caffe2
          restartPolicy: Never
    - replicas: 1
      ReplicaType: HELPER
      template:
        spec:
          containers:
            - image: redis:latest
              name: redis
              ports:
                - containerPort: 6379
          restartPolicy: Never
```

This Caffe2Job resembles the existing TFJob for the tf-operator. The main differences being the omission of the parameter server replica type, and the addition of `backend` options and `HELPER` replica type.

`backend` Defines the distributed type the Caffe2 master and workers will use to communicate when initializing the worker group. Information on the different backends (and the functions they support) can be found [here](https://caffe2.ai/docs/distributed-training.html).

`HELPER` replica type will be used to service finding for `redis` backend, and will be useless for `gloo` backend.

### Resulting Master

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: caffe2-master-${job_id}
  labels: app=caffe2-job-xx
    caffe2_job_name=example-job
    controller-uid=dc3669c6-29f1-11e8-9ccd-ac1f6b8040c6
    job-name=example-job-master-20lm-1
    job_type=MASTER
    kubeflow.org=
    runtime_id=20lm
    task_index=0
spec:
  containers:
    - image: carmark/caffe2:latest
      imagePullPolicy: IfNotPresent
      name: caffe2
  restartPolicy: Never
```

The labels variables provided are used when initializing a distributed process group with Caffe2. `task_index` is determined by adding the number of replicas in each 'MASTER' and 'WORKER' replicaSpecs. `job_type` is `MASTER` for the master.

### Resulting Worker

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: caffe2-worker-${job_id}
  labels: app=caffe2-job-xx
    caffe2_job_name=example-job
    controller-uid=dc3669c6-29f1-11e8-9ccd-ac1f6b8040c6
    job-name=example-job-worker-20lm-0
    job_type=WORKER
    kubeflow.org=
    runtime_id=20lm
    task_index=0
spec:
  containers:
    - image: carmark/caffe2:latest
      imagePullPolicy: IfNotPresent
      name: caffe2
  restartPolicy: Never
```

The worker spec generates a pod. They will communicate to the master through the redis's service name.

## Design

This is an implementaion of the Caffe2 distributed design patterns, found [here](https://caffe2.ai/docs/SynchronousSGD.html), via the lense of TFJob found [here](https://github.com/kubeflow/tf-operator).

Diagram pending

## Other backends

Form [here](https://caffe2.ai/docs/distributed-training.html), Caffe2 also support [Gloo](https://github.com/facebookincubator/gloo) which is another communications library for multi-machine training. For Gloo with MPI, we do not neet the redis to communicate, the master and workers will communicate by ssh. So it should better to define another sshd port to communicate in container, then start the works first and then master container.

To finish this start process, we may invole the [batchd scheduler](https://github.com/kubernetes-incubator/kube-arbitrator) and use priority class to define the priority.
