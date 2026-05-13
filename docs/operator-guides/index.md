# Operator Guides

*Documentation for platform administrators deploying and managing Kubeflow Trainer in production.*

This section contains guides for installing, configuring, and operating Kubeflow Trainer in Kubernetes clusters.

----

## Installation & Migration

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Installation
:link: installation
:link-type: doc

Install Kubeflow Trainer using kubectl or Helm
::::

::::{grid-item-card} Migration from v1
:link: migration
:link-type: doc

Migrate from Kubeflow Training Operator v1 to Trainer v2
::::

:::::

## Configuration

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Training Runtimes
:link: runtime
:link-type: doc

Configure TrainingRuntime and ClusterTrainingRuntime resources
::::

::::{grid-item-card} ML Policies
:link: ml-policy
:link-type: doc

Define ML-specific policies for training workloads
::::

::::{grid-item-card} Job Templates
:link: job-template
:link-type: doc

Customize job templates for different frameworks
::::

::::{grid-item-card} Runtime Patches
:link: runtime-patches
:link-type: doc

Customize training runtime configuration with RuntimePatches
::::

:::::

## Advanced Configuration

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Extension Framework
:link: extension-framework
:link-type: doc

Understand the plugin-based extension architecture
::::

::::{grid-item-card} Job Scheduling
:link: job-scheduling/index
:link-type: doc

Integrate with Volcano, Kueue, and coscheduling
::::

:::::

----

```{toctree}
:hidden:
:maxdepth: 2

installation
migration
runtime
ml-policy
job-template
runtime-patches
extension-framework
job-scheduling/index
```
