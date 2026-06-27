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
- [Test Plan](#test-plan)
    - [End-to-End (E2E) Tests](#end-to-end-e2e-tests)
    - [Working Examples](#working-examples)
    - [Unit and Integration Tests](#unit-and-integration-tests)
- [Future Work](#future-work)
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

### Goals

- Implement ClusterTrainingRuntime for JAX, supporting distributed training with JAX (e.g. multi-controller JAX)
- Build the necessary Docker images for JAX worker nodes used by the runtime
- Implement the solution to work on CPU, GPU and TPU
- Integrate with SDK and address any necessary enhancements
- Document user guides for utilizing JAX ClusterTrainingRuntimes
- Test the implementation thoroughly using unit tests and end-to-end (E2E) tests

### Non-Goals

- Set up test in TPU cluster
- Add advanced JAX APIs

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
def jax_train_mnist(epoch = 10, loss: str = None):
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

| Concept | Description |
|---------|-------------|
| **Host** | A physical or virtual machine participating in distributed training. Each host runs a **single JAX process**, which manages all local devices (e.g., GPUs, CPUs). JAX auto-detects and utilizes all available devices. (In Kubernetes, a host maps to a **Node**, and typically one **Pod** is scheduled per host.) |
| **JAX Process / Controller** | A Python process running the JAX program (exactly one per host). Responsible for executing the training loop, managing all local devices, and synchronizing with other JAX processes over the network. Uses **SPMD** across processes. |
| **Devices** | Compute units on a host (CPU cores, GPUs, TPUs). JAX detects devices automatically and runs parallel computations via `jax.pmap`, `jax.shard_map`, or `pjit`. Each JAX process accesses all devices on its host. **No need to spawn multiple processes per GPU** (unlike PyTorch). |
| **Pod** | A Kubernetes Pod runs a single JAX process. Scheduled on a node and may use one or more GPUs depending on resource specifications. |
| **Node** | A Kubernetes Node is a worker machine. In multi-node JAX jobs, each node typically runs one pod, mapping to one JAX host. |


### JAX Training Workflow

This section explains the architecture and flow of executing a distributed JAX training job using Kubeflow, as depicted in the diagram.

![user-roles](./drawing.drawio.svg)

| **Component**   | **Action**                                | **Details**                                                                                                                     |
| ----------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Platform Admin          | Prepares the Cluster Training Runtime | Defines container image, entrypoint, framework (e.g., JAX), and resource needs. Setup reusable for training jobs.               |
| Trainer Controller Manager                  | Retrieves the Training Runtime Spec, Creates and Submits a JobSet   | Fetched automatically when a user requests a training job to determine execution details. Training job spec is translated into a JobSet (group of coordinated jobs).                                       |
| AI Practitioner         | Creates the Training Job              | Uses Kubeflow Python SDK or `kubectl`. Provides training function (e.g., `jax_train_mnist`), arguments, and node configuration. |
| Runtime | Creates and Submits a JobSet          | Training job spec is translated into a JobSet (group of coordinated jobs).                                                      |
| JobSet Controller | Launches Distributed Jobs             | JobSet spawns multiple Kubernetes Jobs, each pod runs a JAX training process instance.                                          |
| Headless Service        | Connects Pods for Communication           | Enables direct pod-to-pod communication for gradient sharing and coordination.                                                  |
| Cluster (Pods)    | Executes Distributed Training         | Each pod runs JAX+Python code, collaborating to complete training across available hardware.                                    |


### Defining Distributed JAX with MLPolicy

The number of **JAX hosts** is configured using the `numNodes` field in the **MLPolicy** section of the **ClusterTrainingRuntime**. Each host runs a single JAX process inside a Pod.

#### JAXMLPolicySource

`JAXMLPolicySource` indicates that the JAX plugin should be activated. The extension framework will set the appropriate values for JAX distributed environment, backend, devices, and precision.

```golang
type MLPolicySource struct {
  [...]

  JAX *JAXMLPolicySource `json:"jax,omitempty"`
}
```

```golang
type JAXMLPolicySource struct {}
```

#### JAX Distributed System

The plugin enables JAX distributed training and handles distributed initialization internally, allowing seamless execution of training jobs with multiple backend configurations for multi-GPU and Cloud TPU.

| Backend | Parameters | Notes                                                                                                             |
| ------- | ---------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| NCCL    | None                               | No additional configuration needed.                                                                               |
| LibTPU  | None                               | No additional configuration needed.                                                                               |
| Gloo    | None                               | Environment variables (`COORDINATOR_ADDRESS`, `NUM_PROCESSES`, `PROCESS_ID`) are automatically set by the policy. |

## Test Plan

The testing strategy focuses on validating functionality and integration of the `TrainingRuntime` mechanism.

* **Environment**: Run workloads in a lightweight Kubernetes cluster in **CI actions** (e.g., using `kind` or `minikube`).
* **Workloads**: Execute simple distributed training examples such as MNIST **JAX**.
* **Validation Goals**: Ensure correct creation of `JobSet` resources, successful job execution, and compatibility with `TrainingRuntime` configurations.
* **Working Examples**: Provide runnable notebook examples demonstrating how to create and run training jobs. These notebooks double as test cases and user documentation.
* **Unit Tests**: Add unit tests for `JAXMLPolicySource` to validate correct backend selection, environment variable setup, and distributed initialization logic.

## Future Work

While it is possible to configure a specific communication backend (e.g., NCCL, MPI, Gloo) for the runtime by a parameter in `JAXMLPolicy`, we have deferred this decision to simplify the current implementation. By default, JAX uses `Gloo` as the communication backend. The design ensures the system remains extensible, allowing backend selection and integration to be added in response to future feedback without major changes.

## Implementation History

- 2025-05-28: Initial KEP draft created.
- 2025-09-19: Update design detail section and add future work
