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

This proposal implements key components of KEP-2170, introducing the Kubeflow Training V2 API. Specifically, it focuses on creating the TrainingRuntime and ClusterTrainingRuntime for the JAX framework, built upon the Kubernetes JobSet API. These runtimes will serve as blueprints for model training (including LLMs) within cloud-native ML pipelines. This abstraction allows Data Scientists and MLOps Engineers to easily reuse standardized runtimes and launch training jobs, particularly via the SDK, without needing deep knowledge of underlying Kubernetes complexities.

## Motivation

JAX is a powerful Computation library created by Google, It is widely used in machine learning research and ranks as the third most wide used deep learning framework. JAX is not only a deep learning framework but suggests its potential in 
differential programming, large-scale physics simulations and many more. These usecases added on top of the new Runtime API for distributed training or calculation of objectives enables new users on top of kubeflow trainer, like distributed simulation or training of LLM prototypes developed with JAX, like vast models from Google Deep mind.

In General the motivation it to enable users to use Single-Program Multi-Data (SPMD) pattern with JAX Framework. 

### Goals

- Implement ClusterTrainingRuntime for JAX, supporting multi-controller JAX
- Build the necessary Docker images for JAX worker nodes used by the runtime
- Implement the solution to work on CPU and GPU using OpenMPI backend
- Document user guides for utilizing JAX ClusterTrainingRuntimes
- Test the implementation thoroughly using unit tests and end-to-end (E2E) tests

### Non-Goals

- No TPU support (duo to lack of available TPU testing infrastructure)
- No GPU testing, tests will use CPUs
- No Custom MLPolicy, since using OpenMPI, it can handle required parameters
- No mechanism for finding processes, since with OpenMPI backend JAX can find all processes automatically
- Complex end-to-end examples demonstrating the runtimes (focus is on the runtime implementation itself; examples may require specific infrastructure)

## Proposal

### User Stories

#### Story 1

As a MLOps Engineer or Platform Engineer, I want to manage JAX distributed training jobs using the Kubeflow Trainer V2, so then I can provide blueprints for training of machine learning models on a kubernetes cluster to engineering teams.

#### Story 2

As a Data Scientist, I want to use the Trainer V2 SDK to run a distributed training job from notebook, in this way I can incorporate multiple devices for my training task.

#### Story 3

As a Research Scientist, I want to train prototype of my new LLM model coded with JAX on a distributed training setup on my company Kubernetes cluster, Kubeflow Trainer V2 with JAX ClusterTrainingRuntime will enable this for me.

## Design Details

In order to address this functionality, we propose the following design:

####  Choosing OpenMPI over Gloo

- OpenMPI is **10–20× faster than Gloo** for distributed JAX training on CPUs and GPUs, thanks to optimized communication and bandwidth usage.  
- Better multi-node scaling with lower latency and support for high-speed interconnects like InfiniBand.  
- Compatible with existing **MPI runtime in Kubeflow Trainer v2**, making deployment easier.

#### Defining JAX Processes with MLPolicy

- The number of JAX processes can be defined using the `numNodes` field within the `mlPolicy` section of the ClusterTrainingRuntime configuration. This allows for specifying how many JAX processes/controller run in the distributed setup.


![user-roles](./drawing.drawio.svg)

### JAX Distributed Runtime Configuration

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

## Implementation History

- 2025-05-28: Initial KEP draft created.
