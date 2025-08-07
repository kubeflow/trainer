# KEP-2442: JAX Runtime for Trainer V2

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories-optional)
    - [Story 1](#story-1)
    - [Story 2](#story-2)
- [Design Details](#design-details)
    - [Key Concepts in JAX Distributed Training](#key-concepts-in-jax-distributed-training)
    - [JAX Training Workflow](#jax-training-workflow-flow)
    - [Defining JAX Processes with MLPolicy](#defining-jax-processes-with-mlpolicy)
    - [Communication Backend](#communication-backend)
        - [OpenMPI](#openmpi)
        - [Gloo](#gloo)
        - [NCCL](#nccl)
- [Test Plan](#test-plan)
    - [End-to-End (E2E) Tests](#end-to-end-e2e-tests)
    - [Working Examples](#working-examples)
    - [Unit and Integration Tests](#unit-and-integration-tests)
- [Implementation History](#implementation-history)

## Summary

This document outlines a proposal to support the JAX Runtime in Kubeflow Trainer V2. Built upon the Kubernetes JobSet API, the JAX Runtime enables training and fine-tuning workloads using the JAX framework on Kubernetes. Instead of relying on framework-specific CRDs, Trainer V2 introduces a unified abstraction through TrainingRuntime and TrainJob. The JAX Runtime implements this abstraction to serve as a reusable blueprint for model training tasks, including large language models (LLMs). With the Kubeflow Trainer Pipeline Framework, we can easily integrate the JAX runtime into Kubeflow Trainer V2 as a runtime plugin.


## Motivation

JAX is a high-performance numerical computing framework created by Google. It is widely used in the machine learning research and ranks as the third most widely used deep learning frameworks. JAX also suggests its potential in differential programming, large-scale physics simulations and many more.

These usecases added on top of the new Runtime API for distributed training or calculation of objectives enables new users on top of Kubeflow Trainer, like distributed simulation or training of LLM prototypes developed with JAX, like vast models from Google DeepMind.

In general the motivation is to enable users to use Single-Program Multi-Data (SPMD) pattern with JAX Framework, however there are other reasons like ensure backward compatibility with Trainer V1, which previously included JAX support, allowing existing users to transition smoothly while taking advantage of the enhanced Runtime API.

Finally with this design, Platform Admins can define standardized training runtimes, while AI Practitioners can easily customize them, through a simple SDK interface, without needing to understand Kubernetes internals.

**Benefits**

1. Leverage JAX for differential programming and large-scale simulations
2. Enable distributed training or objective computation using the new Runtime API
3. Support prototyping and training of large JAX-based LLMs within Kubeflow Trainer

---

### Goals

- Implement ClusterTrainingRuntime for JAX, supporting distributed training with JAX (e.g. multi-controller JAX)
- Build the necessary Docker images for JAX worker nodes used by the runtime
- Implement the solution to work on CPU and GPU
- Integrate with SDK and address any necessary enhancements
- Document user guides for utilizing JAX ClusterTrainingRuntimes
- Test the implementation thoroughly using unit tests and end-to-end (E2E) tests

### Non-Goals

- No TPU testing, tests will use CPU
- No GPU testing, tests will use CPU

## Proposal

### User Stories

#### Story 1

As a Platform Admin, I want to manage JAX distributed training jobs using the Kubeflow Trainer V2, so then I can provide blueprints for training of machine learning models on a kubernetes cluster to engineering teams.

#### Story 2

As an AI Practitioner, I want to use the Trainer V2 SDK to run a distributed training job from notebook, in this way I can incorporate multiple devices for my training task.

The Python SDK with JAXRuntime may look as follows:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

# Add logic using JAX methods
def jax_train_mnist(args):
    raise NotImplementedError

# Select the JAX runtime
client = TrainerClient()
jax_runtime = next(r for r in client.list_runtimes() if r.name == "jax-distributed")

# Custom parameters passed as arguments
args = {
 "epoch": "20"
 "loss": "MSE"
}

# Launch training job
job_id = client.train(
    trainer=CustomTrainer(func=jax_train_mnist, func_args=args, num_nodes=3),
    runtime=jax_runtime,
)
```

## Design Details

In order to address this functionality, we propose the following design:

### Key Concepts in JAX Distributed Training

To understand the **JAX runtime** in Kubeflow Trainer V2, it's important to clarify the terminology used in JAX for distributed training:

#### Host

In JAX, a **host** refers to a physical or virtual machine that participates in distributed training. Each host runs a **single JAX process**, which manages all the local devices (e.g., GPUs or CPUs). JAX automatically detects and utilizes all available devices on a host.

>In Kubernetes, a host maps to a Node, and in the runtime implementation, one Pod is typically scheduled per host.

**JAX Process/Controller**

A **JAX process** (or sometimes called a **controller**) is a Python process running the JAX program. There is exactly one JAX process per host. This process is responsible for executing the training loop, managing all local devices, and communicating with other JAX processes over the network for synchronization

JAX uses **Single Program Multiple Data (SPMD)** across these processes.

**Devices**

Devices refer to individual compute units on a host (like CPU cores, GPUs, or TPUs). JAX handles device detection automatically and runs parallel computations using `jax.pmap`, `jax.shard_map`, or `pjit`.

Each JAX process has access to **all devices on its host**. There is **no need to spawn multiple processes per GPU**, unlike PyTorch.


**Pod**

A **Pod** in Kubernetes runs a JAX process. It is scheduled on a node and may use one or more GPUs depending on the resource specification.

**Node**

In Kubernetes, a **Node** is a worker machine in the cluster. When we run multi-node JAX jobs, each node typically runs one pod, which maps to one JAX host.

### JAX Training Workflow

This section explains the architecture and flow of executing a distributed JAX training job using Kubeflow, as depicted in the diagram.

![user-roles](./drawing.drawio.svg)

#### 1. Platform Admins Prepares the Training Environment
- A **Platform Admins** sets up the **Cluster Training Runtime** with details like:
  - Container image
  - Entrypoint
  - Framework (e.g., JAX)
  - Resource needs
- This setup can be reused by others to run training jobs.

#### 2. Training Runtime is Retrieved
- When a user requests a training job, the system **fetches the runtime spec** to know how to run the job.

#### 3. AI Practitioner Creates the Training Job
- A **AI Practitioners** creates a training job using:
  - The **Kubeflow Python SDK**, or
  - A `kubectl` command.
- They provide the training function (e.g., `jax_train_mnist`), any needed arguments, and settings like how many nodes to use.

#### 4. JobSet is Created and Submitted
- The training job uses the runtime spec to create a **JobSet**, a group of jobs working together to train the model.

#### 5. Distributed Jobs Start Running
- The **JobSet** launches multiple **Kubernetes Jobs**.
- Each job runs one instance of the **JAX training process** in its own pod.

#### 6. Headless Service Connects the Jobs
- A **Headless Service** allows the pods to **communicate directly** for tasks like sharing gradients and coordinating training.

#### 7. Training Runs Across the Cluster
- Each pod runs the training code using **JAX and Python**.
- The pods work together to complete the distributed training on the available hardware.


### Defining Distributed JAX with MLPolicy

The number of **JAX hosts** is configured using the `numNodes` field in the **MLPolicy** section of the **ClusterTrainingRuntime**. Each host runs a single JAX process inside a Pod.

### Communication Backend

#### OpenMPI

**Pros:**

* Compatible with existing MPI runtime in Kubeflow Trainer v2, making deployment easier.
* Leverage `mpi4jax` for HPC application

**Cons:**

* Typically requires more complex environment setup compared to simpler backends like Gloo.

**ClusterTrainingRuntime Design**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: jax-distributed
spec:
  mlPolicy:
    numNodes: 1
    mpi:
      numProcPerNode: 1
      mpiImplementation: OpenMPI
      sshAuthMountPath: /home/mpiuser/.ssh
      runLauncherAsNode: true
  template:
    spec:
      network:
        publishNotReadyAddresses: true
      successPolicy:
        operator: All
        targetReplicatedJobs:
          - launcher
      replicatedJobs:
        - name: launcher
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: ghcr.io/kubeflow/trainer/jax-runtime
                      securityContext:
                        runAsUser: 1000
                      command:
                        - mpirun
                        - -n
                        - "1"
                        - bash
                        - -c
                        - |
                          echo "JAX Distributed Runtime"

                          echo "--------------------------------------"
                          set -e
                          mpirun --version
                          python --version
                          pip list
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: ghcr.io/kubeflow/trainer/jax-runtime
                      securityContext:
                        runAsUser: 1000
                      command:
                        - /usr/sbin/sshd
                      args:
                        - -De
                        - -f
                        - /home/mpiuser/.sshd_config
                      readinessProbe:
                        tcpSocket:
                          port: 2222
                        initialDelaySeconds: 5
```


#### Gloo

**Pros:**

* Lightweight and simple to use.
* Compatible with the Trainer v1

**Cons:**

* Significantly slower than OpenMPI (10–20×) for distributed JAX training on CPUs and GPUs.
* Less optimized for multi-node scaling and lacks native support for high-speed interconnects like InfiniBand.

**ClusterTrainingRuntime Design**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: jax-distributed
spec:
  mlPolicy:
    numNodes: 4
    jax:
      backend: gloo
  template:
    spec:
      replicatedJobs:
        - name: process
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: ghcr.io/kubeflow/trainer/jax-runtime
                      securityContext:
                        runAsUser: 1000
                      command:
                        - bash
                        - -c
                        - |
                          echo "JAX Distributed Runtime"
                          echo "--------------------------------------"
                          set -e
                          python --version
                          pip list | grep jax
```

#### NCCL

**Pros**
* Minimal setup; JAX/XLA usually auto-configures it.
* Optimized GPU collectives (all-reduce, etc.) that leverage NVLink/PCIe topology.
* Can use GPUDirect (incl. RDMA) for fast inter-node transfers when fabric supports it.

**Cons**
* Performance drops on CPU-only or host-staged gradients; MPI often faster there.
* High latency for many small messages; can trail tuned OpenMPI runs (env-dependent, sometimes 10–20× in CPU-centric tests).
* Debug tooling limited; transport or fabric misconfig can silently degrade throughput.


**ClusterTrainingRuntime Design**

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: jax-distributed
spec:
  mlPolicy:
    numNodes: 4
    jax:
      backend: nccl
      envs:
        - name: NCCL_DEBUG
          value: "WARN"
        - name: NCCL_IB_DISABLE
          value: "1"
        - name: NCCL_SOCKET_IFNAME
          value: "eth0"
  template:
    spec:
      replicatedJobs:
        - name: process
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: ghcr.io/kubeflow/trainer/jax-runtime
                      securityContext:
                        runAsUser: 1000
                      command:
                        - bash
                        - -c
                        - |
                          echo "JAX Distributed Runtime with NCCL"
                          echo "--------------------------------------"
                          set -e
                          python --version
                          pip list | grep jax
```

## Test Plan

The testing strategy will focus on validating functionality, usability, and integration of the proposed `TrainingRuntime` mechanism for distributed training workloads. It includes the following components:

### End-to-End (E2E) Tests

* **Environment**: Deploy workloads in lightweight local Kubernetes clusters using tools like `kind` or `minikube`.
* **Workloads**: Run simple distributed training examples such as MNIST **JAX**.
* **Validation Goals**:

  * Ensure correct creation of `JobSet` resources.
  * Validate successful job execution and error handling paths.
  * Confirm compatibility with `TrainingRuntime` configurations.

### Working Examples

* Provide clear, runnable examples:

  * **Kubeflow SDK and notebook examples** that demonstrate creating and running training jobs using the new interface.
* These examples will serve as both test cases and documentation to support user onboarding.

### Unit and Integration Tests

* For any controller or plugin logic introduced:

  * Write targeted **unit tests** in Go to validate business logic and failure scenarios.
  * Use mocks/fakes where needed to simulate cluster conditions and resource state.
* Ensure **controller reconciliation logic** is tested thoroughly.

## Implementation History

- 2025-05-28: Initial KEP draft created.
