# KEP-2628: Support KAI Scheduler in Kubeflow Trainer

This KEP proposes integrating NVIDIA's KAI Scheduler into Kubeflow Trainer V2 to enable gang-scheduling capabilities for TrainJob resources, extending the existing PodGroupPolicy API to support KAI alongside current schedulers like Co-Scheduling. The integration will leverage KAI's PodGrouper service to create scheduling queues and apply appropriate labels for efficient resource allocation in AI workloads.

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
- [Design Details](#design-details)
  - [API Extensions](#api-extensions)
  - [Plugin Implementation](#plugin-implementation)
  - [Integration with KAI PodGrouper](#integration-with-kai-podgrouper)
  - [Queue Management and Labeling](#queue-management-and-labeling)
  - [Validation and Error Handling](#validation-and-error-handling)

## Summary

This Kubeflow Enhancement Proposal (KEP) aims to integrate [KAI Scheduler](https://github.com/NVIDIA/KAI-Scheduler), NVIDIA's open-source scheduler designed specifically for GPU workloads, into Kubeflow Trainer V2. The integration will extend the existing PodGroupPolicy API to support KAI as a gang-scheduling option alongside the currently supported Volcano and Co-Scheduling schedulers. This enhancement will enable efficient resource allocation and scheduling for distributed training workloads using TrainJob resources in Kubeflow environments.

## Motivation

[KAI Scheduler](https://github.com/NVIDIA/KAI-Scheduler) has emerged as a specialized scheduling solution for GPU workloads, offering features specifically designed to optimize the scheduling of machine learning training jobs. Currently, KAI only integrates with Kubeflow Training Operator v1, limiting its accessibility to users who have migrated to or prefer using Kubeflow Trainer V2. The lack of KAI support in Trainer V2 creates a gap in scheduling options for users who require advanced scheduling capabilities.

The existing PodGroupPolicy API in Trainer V2 already supports multiple schedulers, demonstrating the framework's extensibility. Adding KAI support would provide users with another powerful scheduling option, particularly beneficial for environments where NVIDIA hardware is prevalent and where specialized GPU workload scheduling can significantly improve resource utilization and training performance.

### Goals

- Extend the PodGroupPolicy API in Trainer V2 to support KAI Scheduler as a gang-scheduling option
- Implement a KAI plugin that integrates with the existing framework architecture
- Enable automatic creation of scheduling queues and proper labeling for KAI-scheduled workloads
- Provide comprehensive documentation and examples for using KAI with TrainJob resources
- Ensure compatibility with JobSet orchestration used by Trainer V2
- Maintain consistency with existing scheduler integrations (Co-Scheduling)


## Design Details

The implementation will follow the established pattern used for other scheduler integrations in Trainer V2, extending the PodGroupPolicySource struct and creating a dedicated plugin for KAI Scheduler functionality.

### API Extensions

The PodGroupPolicySource struct in `trainingruntime_types.go` will be extended to include KAI Scheduler configuration. This follows the same pattern as existing scheduler integrations:

```go
type PodGroupPolicySource struct {
    CoScheduling *CoSchedulingConfig `json:"coscheduling,omitempty"`
    KAI          *KAIConfig          `json:"kai,omitempty"`
}

type KAIConfig struct {
    Queue string `json:"queue,omitempty"`
}
```

### Plugin Implementation

A new plugin will be created in `pkg/runtime/framework/plugins/kai` that implements the required interfaces for gang-scheduling functionality. The plugin will handle the integration with KAI's PodGrouper service and ensure proper labeling of pods and TrainJob resources.

The KAI plugin implementation includes several key components:

- The plugin implements the `EnforcePodGroupPolicyPlugin` interface to apply the necessary labels for KAI scheduling
- When a TrainJob specifies KAI as the scheduler, the plugin adds the `runai/queue` label with the specified queue name to ensure proper scheduling by the KAI Scheduler
- The plugin also implements the `WatchExtensionPlugin` and `ComponentBuilderPlugin` interfaces to maintain consistency with the framework architecture, even though KAI primarily relies on external PodGroup creation through its PodGrouper service

### Integration with KAI PodGrouper

KAI Scheduler uses a PodGrouper service that watches for pods and creates PodGroup resources by analyzing OwnerReferences chains. For Trainer V2 integration, a new plugin will need to be developed for the KAI PodGrouper to recognize TrainJob resources as valid top-level owners, similar to the existing PyTorchJob plugin.

The integration process involves two main steps:

1. **PodGrouper Extension**: The KAI PodGrouper must be extended with a TrainJob plugin that understands the TrainJob CRD structure and can extract the necessary scheduling information
2. **Labeling Integration**: The Trainer V2 KAI plugin must ensure that appropriate labels are applied to pods and the TrainJob resource itself to enable proper queue assignment

### Queue Management and Labeling

The implementation will support the creation of scheduling queues as a trainer plugin feature. Users will specify the queue name in the KAI configuration, and the plugin will automatically apply the `runai/queue` label to the appropriate resources. This label can be placed either on the top-level owner (TrainJob) or directly on individual pods, depending on the specific requirements and KAI configuration.

The queue configuration will be validated to ensure that specified queues exist in the KAI Scheduler configuration before allowing TrainJob creation. This prevents runtime errors and provides immediate feedback to users about configuration issues.

### Validation and Error Handling

The plugin will include comprehensive validation logic to ensure that KAI-specific configurations are valid and compatible with the overall TrainJob specification. This includes:

- Validating queue names
- Ensuring that required labels are properly formatted
- Checking for conflicts with other scheduler configurations

Error handling will provide clear, actionable messages to users when KAI-related configuration issues are detected. The plugin will fail fast during validation rather than allowing invalid configurations to proceed to scheduling, reducing debugging time and improving user experience.