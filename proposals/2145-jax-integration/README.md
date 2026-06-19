# KEP-2145: Integrate JAX with Kubeflow Training Operator for Distributed Training on Kubernetes

<!-- toc -->

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories (Optional)](#user-stories-optional)
    - [Story 1](#story-1)
    - [Story 2](#story-2)
    - [Story 3](#story-3)
- [Design Details](#design-details)
- [Alternatives](#alternatives)
- [Implementation History](#implementation-history)
<!-- /toc -->

## Summary

This Kubeflow Enhancement Proposal (KEP) aims to integrate [JAX](http://jax.readthedocs.io/), a popular machine learning framework, into the Kubeflow Training Operator to enable distributed training and fine-tuning jobs on Kubernetes. This will involve creating a new Kubernetes Custom Resource Definition (CRD) for JAX (JAXJob) and updating the Training Operator controller to support it. The enhancement will also include integrating JAX with the Training Operator Python SDK to provide simple APIs for Data Scientists to create and manage JAXJob on Kubernetes clusters.

## Motivation

JAX has emerged as a popular machine learning framework for high-performance numerical computing and accelerated training on GPUs and TPUs. With its "multi-controller" programming model, JAX is particularly well-suited for distributed training using the Single Program, Multiple Data (SPMD) paradigm. However, running distributed JAX jobs on Kubernetes requires robust orchestration to handle the complexities of multi-process coordination.

Integrating JAX into the Kubeflow Training Operator will simplify distributed JAX training on Kubernetes, providing Data Scientists and ML Engineers with seamless APIs to deploy and manage JAX jobs. This proposal aims to create a new Kubernetes Custom Resource (CR) for JAX, update the Training Operator controller to support it, and provide an intuitive Python SDK for managing JAX jobs.

### Goals

- Introduce a new Custom Resource Definition (CRD) called `JAXJob` for managing JAX distributed training jobs on Kubernetes.
- Update the Kubeflow Training Operator to support the `JAXJob` CRD.
- Extend the Training Operator Python SDK to support JAXjob creation and management.
- Implement the solution to work in CPU environments using the Gloo backend, as GPU environments are not available.

### Non-Goals

- Support for GPU environments is not included in this proposal due to the current limitation of not having GPU resources available for testing.

## Proposal

### User Stories (Optional)

#### Story 1

As a Data Scientist, I want to use the Kubeflow Training Operator to run distributed JAX training jobs on a Kubernetes cluster so that I can leverage Kubernetes' scalability and resource management capabilities.

#### Story 2

As a Machine Learning Engineer, I want to use a simple Python SDK to define and launch JAX training jobs on Kubernetes, abstracting away the complexity of Kubernetes configurations.

#### Story 3

As a DevOps engineer, I want to manage JAX distributed training jobs using the Kubeflow Training Operator, so I can provide a consistent and scalable infrastructure for machine learning workloads.

## Design Details

- Create a new Custom Resource Definition (CRD) for JAX jobs (e.g., `JaxJob`).
- Update the Kubeflow Training Operator to manage `JaxJob` resources.
- Implement webhook validations for the `JAXJob`
- Implement a mechanism to initialize and manage JAX distributed training processes using [`jax.distributed.initialize`](https://jax.readthedocs.io/en/latest/_autosummary/jax.distributed.initialize.html#jax.distributed.initialize).
- Extend the Training Operator Python SDK to simplify the creation and management of `JaxJob` resources.
- Configure JAX to use the Gloo backend for CPU-based distributed training.

| Environment Variable       | JAX Parameter          | Description                                                                                               | How to Obtain/Configure                       |
|----------------------------|------------------------|-----------------------------------------------------------------------------------------------------------|-----------------------------------------------|
| `JAX_COORDINATOR_ADDRESS`  | `coordinator_address (str)`  | the IP address of process 0 in your cluster, together with a port available on that process. Process 0 will start a JAX service exposed via that IP address and port, to which the other processes in the cluster will connect.        | Set this in the coordinator pod spec and ensure it's the same for all worker pods. Example: `localhost:1234`. |
| `JAX_NUM_PROCESSES`        | `num_processes (int) `        | The number of processes in the cluster.                                                         | Define in both the coordinator and worker pod specs. Example: `2`.               |
| `JAX_PROCESS_ID`           | `process_id (int)`           | The ID number of the current process. Each process should have a unique ID, , in the range `[0 .. num_processes)`.                                | Set this in each pod spec. The coordinator is usually `0`, workers are `1, 2, ...`. |
| `JAX_LOCAL_DEVICE_IDS`     | `local_device_ids (int)`     | Restricts the visible devices of the current process to `local_device_ids`.                                                      | Optional. Set in the pod spec if device visibility needs to be restricted.      |
| `JAX_INITIALIZATION_TIMEOUT`| `initialization_timeout (int)` | Time period (in seconds) for which connection will be retried. If the initialization takes more than the timeout specified, the initialization will error. Defaults to 300 secs i.e. 5 mins.        | Optional. Can be set in the pod spec if a different timeout is needed.          |
| `JAX_COORDINATOR_BIND_ADDRESS` | `coordinator_bind_address (str)` | The IP address and port to which the JAX service on process 0 in your cluster will bind. By default, it will bind to all available interfaces using the same port as `coordinator_address`.                               | Optional. Can be set in the coordinator pod spec. Default binds to all available addresses. |

#### Validations for JaxJob

##### Key Validations

1. **Worker Role Validation**:
   - Ensure at least one Worker replica with `processId` set to `0` that will work as coordinator.
   - Ensure the `replicas` field is greater than `0`.
2. **JAX Parameters Validation**:
   - Ensure `coordinatorAddress`, `numProcesses`, and `processId` are set and valid across all roles.
3. **Pod Specification Validation**:
   - Ensure necessary container specifications and `restartPolicy` are correctly set.
   - Validate `coordinatorAddress` follows the `host:port` format.

#### API (CRD and resulting objects)

##### Custom Resource Definition

The JAXJob CRD will define the schema for JAX distributed training jobs, including specifications for the coordinator, worker processes.

```yaml
apiVersion: kubeflow.org/v1
kind: JAXJob
metadata:
  name: example-jaxjob
spec:
  jaxReplicaSpecs:
    Worker:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          containers:
            - name: jax-worker
              image: ghcr.io/kubeflow/jax:latest
```

`JAX API Definition`

```go
// Copyright 2024 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// JAXJobDefaultPortName is name of the port used to communicate between Coordinator and Workers.
	JAXJobDefaultPortName = "jaxjob-port"
	// JAXJobDefaultContainerName is the name of the JAXJob container.
	JAXJobDefaultContainerName = "jax"
	// JAXJobDefaultPort is default value of the port.
	JAXJobDefaultPort = 6666
	// JAXJobDefaultRestartPolicy is default RestartPolicy for JAXReplicaSpecs.
	JAXJobDefaultRestartPolicy = RestartPolicyNever
	// JAXJobKind is the kind name.
	JAXJobKind = "JAXJob"
	// JAXJobPlural is the JAXJobPlural for JAXJob.
	JAXJobPlural = "jaxjobs"
	// JAXJobSingular is the singular for JAXJob.
	JAXJobSingular = "jaxjob"
	// JAXJobFrameworkName is the name of the ML Framework
	JAXJobFrameworkName = "jax"
	// JAXJobReplicaTypeWorker is the type for workers of distributed JAX.
	JAXJobReplicaTypeWorker ReplicaType = "Worker"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=jaxjob
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.conditions[-1:].type`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:subresource:scale:specpath=.spec.jaxReplicaSpecs.Worker.replicas,statuspath=.status.replicaStatuses.Worker.active,selectorpath=.status.replicaStatuses.Worker.selector

// JAXJob Represents a JAXJob resource.
type JAXJob struct {
	// Standard Kubernetes type metadata.
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the JAXJob.
	Spec JAXJobSpec `json:"spec,omitempty"`

	// Most recently observed status of the JAXJob.
	// Read-only (modified by the system).
	Status JobStatus `json:"status,omitempty"`
}

// JAXJobSpec is a desired state description of the JAXJob.
type JAXJobSpec struct {
	// RunPolicy encapsulates various runtime policies of the distributed training
	// job, for example how to clean up resources and how long the job can stay
	// active.
	//+kubebuilder:validation:Optional
	RunPolicy RunPolicy `json:"runPolicy"`

	// A map of JAXReplicaType (type) to ReplicaSpec (value). Specifies the JAX cluster configuration.
	// For example,
	//   {
	//     "Worker": JAXReplicaSpec,
	//   }
	JAXReplicaSpecs map[ReplicaType]*ReplicaSpec `json:"jaxReplicaSpecs"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=jaxjobs
//+kubebuilder:object:root=true

// JAXJobList is a list of JAXJobs.
type JAXJobList struct {
	// Standard type metadata.
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of JAXJobs.
	Items []JAXJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JAXJob{}, &JAXJobList{})
	SchemeBuilder.SchemeBuilder.Register(addJAXDefaultingFuncs)
}

```

##### Resulting Worker

Upon creating a `JaxJob`, the Training Operator will generate the necessary Kubernetes resources, such as Pods and Services, to facilitate distributed training. Each pod will be configured with environment variables required for JAX's `initialize` function.

- **Coordinator Pod:** The pod with `JAX_PROCESS_ID=0` will act as the coordinator.
- **Worker Pods:** The remaining pods will act as workers, connecting to the coordinator.

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: jaxjob-worker-${job_id}
spec:
  containers:
    - image: ghcr.io/kubeflow/jax:latest
      imagePullPolicy: IfNotPresent
      name: worker
      env:
        - name: JAX_COORDINATOR_ADDRESS
          value: "127.0.0.1:6666"
        - name: JAX_NUM_PROCESSES
          value: 1
        - name: JAX_PROCESS_ID
          value: 0
          # process 0 is coordinator
  restartPolicy: OnFailure
```

## Alternatives

- Integrate JAX to Training Operator with JobSet and `TrainJob` after `TrainJob` API is implemented in Training Operator.
- Using MPI instead of Gloo: While MPI is a mature solution for distributed computing, it adds additional complexity in terms of setup and management. Gloo, being simpler and more lightweight, is preferred for the initial implementation.

## Implementation History

- 2024-05-22: Initial KEP draft created.
