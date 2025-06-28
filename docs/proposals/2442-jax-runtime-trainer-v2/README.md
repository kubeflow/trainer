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
- [Alternatives](#alternatives)
- [Implementation History](#implementation-history)

## Summary

This document outlines a proposal to support JAX Runtime in Kubeflow Trainer V2.
Built upon the Kubernetes JobSet API, the JAX runtime focuses on creating the TrainingRuntime and ClusterTrainingRuntime for the JAX framework. These runtimes will serve as blueprints for model training (including LLMs) within cloud-native ML pipelines. This abstraction allows Data Scientists and MLOps Engineers to easily reuse standardized runtimes and launch training jobs, particularly via the SDK, without needing deep knowledge of underlying Kubernetes complexities.
Thanks to the Kubeflow Trainer Pipeline Framework, we can seamlessly support JAX Runtime in Kubeflow Trainer as a runtime plugin.

## Motivation

JAX is a powerful ML framework created by Google. It is widely used in the machine learning research and ranks as the third most widely used deep learning frameworks. JAX is not only a deep learning framework but suggests its potential in differential programming, large-scale physics simulations and many more.

These usecases added on top of the new Runtime API for distributed training or calculation of objectives enables new users on top of Kubeflow Trainer, like distributed simulation or training of LLM prototypes developed with JAX, like vast models from Google DeepMind.

In general the motivation is to enable users to use Single-Program Multi-Data (SPMD) pattern with JAX Framework.

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
- No Custom MLPolicy, since using OpenMPI, it can handle required parameters
- Complex end-to-end examples demonstrating the runtimes (focus is on the runtime implementation itself; examples may require specific infrastructure)

## Proposal

### User Stories

#### Story 1

As a MLOps Engineer or Platform Engineer, I want to manage JAX distributed training jobs using the Kubeflow Trainer V2, so then I can provide blueprints for training of machine learning models on a kubernetes cluster to engineering teams.

The ClusterTrainingRuntime may look as follows:

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


#### Story 2

As a Data Scientist, I want to use the Trainer V2 SDK to run a distributed training job from notebook, in this way I can incorporate multiple devices for my training task.

```python
def jax_train_mnist():
    # TODO: implement your objective function here

# list available run-times and get jax-runtime
from kubeflow.trainer import TrainerClient, CustomTrainer

for r in TrainerClient().list_runtimes():
    print(f"Name: {r.name}, Framework: {r.trainer.framework.value}, Trainer Type: {r.trainer.trainer_type.value}\n")
    print(f"Entrypoint: {r.trainer.entrypoint[:3]}\n")

    if r.name == "jax-distributed":
        jax_runtime = r

# request training with jax runtime
job_id = TrainerClient().train(
    trainer=CustomTrainer(
        func=jax_train_mnist,
        func_args=args,
        num_nodes=3,
    ),
    runtime=jax_runtime,
)
```

#### Story 3

As a Research Scientist, I want to train prototype of my new LLM model coded with JAX on a distributed training setup on my company Kubernetes cluster, Kubeflow Trainer V2 with JAX ClusterTrainingRuntime will enable this for me.

## Design Details

In order to address this functionality, we propose the following design:

### Communication Backend

#### OpenMPI

**Pros:**

* Compatible with existing MPI runtime in Kubeflow Trainer v2, making deployment easier.

**Cons:**

* Typically requires more complex environment setup compared to simpler backends like Gloo.

#### Gloo

**Pros:**

* Lightweight and simple to use.

**Cons:**

* Significantly slower than OpenMPI (10–20×) for distributed JAX training on CPUs and GPUs.
* Less optimized for multi-node scaling and lacks native support for high-speed interconnects like InfiniBand.

### Defining JAX Processes with MLPolicy

The number of JAX processes can be defined using the `numNodes` field within the `mlPolicy` section of the `ClusterTrainingRuntime` configuration. This allows for specifying how many JAX processes/controller run in the distributed setup.


![user-roles](./drawing.drawio.svg)

## Implementation History

- 2025-05-28: Initial KEP draft created.
