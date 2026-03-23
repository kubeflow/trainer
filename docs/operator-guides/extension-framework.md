# Kubeflow Trainer Extension Framework

This guide describes the Kubeflow Trainer Extension Framework's core concepts and its four execution phases: startup, pre-execution, build, and post-execution.

## Overview

The Kubeflow Trainer Extension Framework is an internal mechanism that enables flexible expansion of Runtime and TrainJob combinations in the Kubeflow ecosystem. The framework provides extension points and manages component lifecycle through four phases.

![Kubeflow Trainer Extension Framework Architecture](images/KubeflowTrainerExtensionFrameworkOverview.drawio.svg)

### Purpose and Intended Users

The primary purpose of the Kubeflow Trainer Extension Framework is to provide a flexible and extensible mechanism for managing and executing machine learning training jobs within the Kubeflow Trainer. It is designed for platform administrators who need to extend Kubeflow Trainer with their custom plugins to fit their specific requirements.

### Examples

To illustrate how the framework can be used, consider the following scenarios:

1. **Custom Validation**: A user wants to add specific validation logic to ensure that certain fields in the TrainJob are correctly configured before execution.
2. **Dynamic Resource Deployment**: A user needs to deploy Kubernetes resources dynamically based on ML framework specific requirements.

## Core Concepts

### Phases

The Kubeflow Trainer Extension Framework follows a structured, step-by-step execution flow. Each phase represents a logical part of the workflow:

- **Startup Phase**: Executes once during the initialization of the `kubeflow-trainer-controller-manager`. This phase sets up necessary internal components.
- **PreExecution Phase**: Triggered when a TrainJob is created or updated. This phase validates and prepares the job for execution.
- **Build Phase**: Builds and deploys the required Kubernetes resources for training jobs.
- **PostExecution Phase**: Runs after the job has been built and executed, checking the status and applying relevant conditions to the job.

### APIs and Extension Points

In each phase, there are two types of components:

1. **Internal APIs**: These APIs are used internally by the framework and cannot be extended or modified by the user.
2. **Extension Points**: These points are exposed to the user and allow for customization through plugins that can be added to the framework.

![Kubeflow Trainer Extension Framework](images/KubeflowTrainerExtensionFramework.drawio.svg)

## Phase Details

### 1. Startup Phase

**Purpose**: Initialize the Kubeflow Trainer Extension Framework and set up necessary components for managing training jobs.

- **Internal APIs**:
  - **Initialize Kubeflow Trainer Extension Framework**: Sets up the entire Kubeflow Trainer Extension Framework.
  - **TrainJobController**: Configures the TrainJob controller and registers it with the Manager.
  - **Built-in Webhook Servers**: Initializes Validation Webhook Servers that handle job creation and updates.
  - **Start Manager**: Starts the main management process.

- **Extension Point**:
  - **WatchExtension**: Registers custom reconciler builders that watch specific Kubernetes resources and trigger TrainJob reconciliations as necessary.

### 2. PreExecution Phase

**Purpose**: Triggered when a TrainJob is created or updated, this phase validates the job and prepares it for execution.

- **Extension Point**:
  - **CustomValidation**: Registers custom validation logic to validate resources before a TrainJob is executed. This can include checking specific fields or configurations before proceeding.

### 3. Build Phase

**Purpose**: In this phase, the required Kubernetes resources are built and deployed to the cluster for execution.

- **Internal API**:
  - **TrainJobController**: Deploys the built components (Kubernetes resources) to the cluster as part of the reconciliation process.

- **Extension Points**:
  - **EnforcePodGroupPolicy**: Configures pod-specific parameters, such as those specified in the `TrainingRuntime.spec.podGroupPolicy`, for any relevant resources (like PodSpecs).
  - **EnforceMLPolicy**: Configures Machine Learning-specific parameters from the `TrainingRuntime.spec.mlPolicy` to adjust the deployment of training resources.
  - **ComponentBuilder**: Builds Kubernetes resources using the `RuntimeInfo` and TrainJob objects, allowing for a dynamic, runtime-specific deployment configuration.

### 4. PostExecution Phase

**Purpose**: After the TrainJob has been executed, the framework checks the state of the job and applies terminal conditions if necessary.
