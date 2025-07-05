# KEP-2442: JAX Runtime for Trainer V2

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories-optional)
    - [Story 1](#story-1)
    - [Story 2](#story-2)
    - [Story 3](#story-3)
- [Design Details](#design-details)
    - [Communication Backend](#communication-backend)
        - [OpenMPI](#openmpi)
        - [Gloo](#gloo)
    - [Defining JAX Processes with MLPolicy](#defining-jax-processes-with-mlpolicy)
    - [JAX Training Workflow Flow](#jax-training-workflow-flow)
- [Test Plan](#test-plan)
    - [End-to-End (E2E) Tests](#end-to-end-e2e-tests)
    - [Working Examples](#working-examples)
    - [Unit and Integration Tests](#unit-and-integration-tests)
- [Implementation History](#implementation-history)

## Summary

This document outlines a proposal to support the JAX Runtime in Kubeflow Trainer V2. Built upon the Kubernetes JobSet API, the JAX Runtime enables training and fine-tuning workloads using the JAX framework on Kubernetes. Instead of relying on framework-specific CRDs, Trainer V2 introduces a unified abstraction through TrainingRuntime and TrainJob. The JAX Runtime implements this abstraction to serve as a reusable blueprint for model training tasks, including large language models (LLMs). Thanks to Kubeflow Trainer Pipeline Framework, we can seamlessly integrate the JAX runtime into Kubeflow Trainer V2 as a runtime plugin.

## Motivation

JAX is a powerful ML framework created by Google. It is widely used in the machine learning research and ranks as the third most widely used deep learning frameworks. JAX is not only a deep learning framework but suggests its potential in differential programming, large-scale physics simulations and many more.

These usecases added on top of the new Runtime API for distributed training or calculation of objectives enables new users on top of Kubeflow Trainer, like distributed simulation or training of LLM prototypes developed with JAX, like vast models from Google DeepMind.

In general the motivation is to enable users to use Single-Program Multi-Data (SPMD) pattern with JAX Framework.

With this design, Platform Engineers can define standardized training runtimes, while Data Scientists can easily customize them, through a simple SDK interface, without needing to understand Kubernetes internals.

**Benefits**

1. Leverage JAX for differential programming and large-scale simulations
2. Enable distributed training or objective computation using the new Runtime API
3. Support prototyping and training of large JAX-based LLMs within Kubeflow Trainer

### Goals

- Implement ClusterTrainingRuntime for JAX, supporting multi-controller JAX
- Build the necessary Docker images for JAX worker nodes used by the runtime
- Implement the solution to work on CPU and GPU
- Document user guides for utilizing JAX ClusterTrainingRuntimes
- Test the implementation thoroughly using unit tests and end-to-end (E2E) tests

### Non-Goals

- No TPU support (duo to lack of available TPU testing infrastructure)
- No GPU testing, tests will use CPUs

## Proposal

### User Stories

#### Story 1

As a Platform Engineer, I want to manage JAX distributed training jobs using the Kubeflow Trainer V2, so then I can provide blueprints for training of machine learning models on a kubernetes cluster to engineering teams.

#### Story 2

As a Data Scientist, I want to use the Trainer V2 SDK to run a distributed training job from notebook, in this way I can incorporate multiple devices for my training task.

The Python SDK with JAXRuntime may look as follows:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

def jax_train_mnist():
    # TODO: Add training logic using JAX
    pass

# Select the JAX runtime
client = TrainerClient()
jax_runtime = next(r for r in client.list_runtimes() if r.name == "jax-distributed")

# Launch training job
job_id = client.train(
    trainer=CustomTrainer(func=jax_train_mnist, func_args=args, num_nodes=3),
    runtime=jax_runtime,
)
```

## Design Details

In order to address this functionality, we propose the following design:

### Communication Backend

### OpenMPI

**Pros:**

* Compatible with existing MPI runtime in Kubeflow Trainer v2, making deployment easier.

**Cons:**

* Typically requires more complex environment setup compared to simpler backends like Gloo.

#### ClusterTrainingRuntime Design:

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


### Gloo

**Pros:**

* Lightweight and simple to use.

**Cons:**

* Significantly slower than OpenMPI (10–20×) for distributed JAX training on CPUs and GPUs.
* Less optimized for multi-node scaling and lacks native support for high-speed interconnects like InfiniBand.

### Defining JAX Processes with MLPolicy

The number of JAX hosts can be defined using the `numNodes` field within the `mlPolicy` section of the `ClusterTrainingRuntime` configuration. This allows for specifying how many JAX processes/controller run in the distributed setup.

### JAX Training Workflow Flow

This section explains the architecture and flow of executing a distributed JAX training job using Kubeflow, as depicted in the diagram.

![user-roles](./drawing.drawio.svg)

#### 1. Platform Engineer Sets Up the Cluster Training Runtime

* A **Platform Engineer** registers and configures the **Cluster Training Runtime**.
* This runtime includes details like the container image, entrypoint, framework (e.g., JAX), and resource requirements.
* These runtimes are reusable components available to end users for launching training jobs.

#### 2. Runtime Specification is Fetched

* When a user initiates a training request, the **Cluster Training Runtime** is queried to **fetch its runtime specification**.
* This spec defines how the job should be executed across the cluster.

#### 3. Data Scientist or ML Researcher Creates a TrainJob

* A **Data Scientist / ML Research Scientist** defines a training job using either:

  * The **Kubeflow Python SDK**, or
  * A `kubectl` command.
* The training function (e.g., `jax_train_mnist`) is provided, along with any arguments and configuration (e.g., `num_nodes` for distributed training).

#### 4. TrainJob Submits a JobSet

* Once submitted, the **TrainJob** uses the runtime spec to create a **JobSet**.
* A **JobSet** is a group of Kubernetes Jobs that work together to complete the training process.

#### 5. Distributed Jobs Are Launched

* The **JobSet** schedules and launches multiple **Kubernetes Jobs**.
* Each job represents an individual **JAX training process**, running in its own pod.

#### 6. Headless Service Enables Communication

* A **Headless Kubernetes Service** is created to allow **direct communication** between the JAX pods.
* This is critical for JAX’s distributed training features, such as gradient synchronization and parallel execution.

#### 7. JAX Processes Execute the Training

* Each pod runs the user-defined training logic using JAX and Python.
* The processes communicate and synchronize through the headless service, executing distributed training on the assigned hardware (e.g., Cloud GPUs).

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
