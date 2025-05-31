# KEP-2442: JAX Runtimes for Trainer V2

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

## Summary

This proposal implements key components of KEP-2170, introducing the Kubeflow Training V2 API. Specifically, it focuses on creating the TrainingRuntime and ClusterTrainingRuntime for the JAX framework, built upon the Kubernetes JobSet API. These runtimes will serve as blueprints for model training (including LLMs) within cloud-native ML pipelines. This abstraction allows Data Scientists and MLOps Engineers to easily reuse standardized runtimes and launch training jobs, particularly via the SDK, without needing deep knowledge of underlying Kubernetes complexities.

## Motivation

JAX is a powerful Computation library created by Google, It is widely used in machine learning research and ranks as the third most wide used deep learning framework. JAX is more than a deep learning framework while suggests its potential in 
differential programming, large-scale physics simulations and many more. 

### Goals

- Implement ClusterTrainingRuntime for JAX, supporting single-node and multi-node configurations
- Build the necessary Docker images for JAX worker nodes used by the runtimes
- Document user guides for utilizing JAX TrainingRuntimes
- Test the implementation thoroughly using unit tests and end-to-end (E2E) tests

### Non-Goals

- GPU support (due to lack of available GPU testing infrastructure)
- Complex end-to-end examples demonstrating the runtimes (focus is on the runtime implementation itself; examples may require specific infrastructure)

## Proposal

### User Stories (Optional)

#### Story 1

As a MLOps Engineer or Platform Engineer, I want to manage JAX distributed training jobs using the Kubeflow Trainer V2, so then I can provide blueprints for training of machine learning models on a kubernetes cluster to engineering teams.

#### Story 2

As a Data Scientist, I want to use the Trainer V2 SDK to run a distributed training job from notebook, in this way I can incorporate multiple devices (CPUs, TPUs or GPUs) for my training task.

#### Story 3

As a Research Scientist, I want to train the prototype of my new LLM model written in JAX on multiple GPUs on Google Cloud Kubernetes Engine, Kubeflow Trainer V2 with JAX ClusterTrainingRuntime will enable this for me.

## Design Details

TODO

### Test Plan

<!--
The goal is to ensure that we don't accept enhancements with inadequate testing.
All code is expected to have adequate tests (eventually with coverage
expectations). Please adhere to the Kubeflow testing guidelines when drafting this test plan.
-->

[ ] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

## Implementation History

- 2025-05-28: Initial KEP draft created.

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->