# Extension Framework

The Kubeflow Trainer Extension Framework provides a plugin-based architecture for extending runtime and TrainJob functionality. It's designed for platform administrators who need to customize Kubeflow Trainer behavior.

## Overview

The extension framework enables:
- **Custom validation logic** for TrainJob specifications
- **Dynamic resource generation** based on runtime and job configurations
- **Pod-level customizations** through policy plugins
- **Custom reconcilers** for additional Kubernetes resources

The framework manages the component lifecycle through four structured execution phases.

## Architecture

The framework distinguishes between two component types:

### Internal APIs

Framework-only operations that cannot be extended:
- Core controller reconciliation logic
- CRD schema management
- Status condition management
- Basic resource creation

### Extension Points

User-customizable components accessible via plugins:
- Custom validation rules
- Dynamic resource builders
- Policy enforcement plugins
- Resource watchers and reconcilers

:::{note}
Extension points provide well-defined interfaces for customization without modifying core controller code.
:::

## Execution Phases

The framework operates in four distinct phases:

### 1. Startup Phase

Initializes the framework during controller-manager startup.

**Internal Operations:**
- Sets up the entire extension framework
- Configures and registers the TrainJob controller
- Initializes validation webhook servers

**Extension Point: WatchExtension**

Registers custom reconcilers that monitor Kubernetes resources and trigger TrainJob reconciliations.

**Use case:** Watch external ConfigMaps or Secrets and reconcile affected TrainJobs.

**Interface:**

```go
type WatchExtension interface {
    Watch(mgr ctrl.Manager, controller controller.Controller) error
}
```

**Example:**

```go
type ConfigMapWatcher struct{}

func (w *ConfigMapWatcher) Watch(mgr ctrl.Manager, c controller.Controller) error {
    return c.Watch(
        &source.Kind{Type: &corev1.ConfigMap{}},
        handler.EnqueueRequestsFromMapFunc(w.mapConfigMapToTrainJob),
    )
}

func (w *ConfigMapWatcher) mapConfigMapToTrainJob(obj client.Object) []reconcile.Request {
    // Logic to find affected TrainJobs
}
```

### 2. PreExecution Phase

Triggered when a TrainJob is created or updated, before resource deployment.

**Extension Point: CustomValidation**

Implements validation logic to check resource configurations before execution.

**Use case:** Enforce organization-specific policies, validate custom annotations, check resource quotas.

**Interface:**

```go
type CustomValidation interface {
    Validate(info *runtime.Info) error
}
```

**Example:**

```go
type ResourceQuotaValidator struct{}

func (v *ResourceQuotaValidator) Validate(info *runtime.Info) error {
    // Check if requested GPUs exceed team quota
    gpuRequest := extractGPURequest(info)
    quota := getTeamQuota(info.TrainJob.Namespace)

    if gpuRequest > quota {
        return fmt.Errorf("GPU request %d exceeds team quota %d",
            gpuRequest, quota)
    }
    return nil
}
```

### 3. Build Phase

Deploys required Kubernetes resources to the cluster.

**Extension Point: EnforcePodGroupPolicy**

Configures pod group parameters for gang scheduling from TrainingRuntime specs.

**Use case:** Set up PodGroups for Volcano, Coscheduling, or Kueue.

**Interface:**

```go
type EnforcePodGroupPolicy interface {
    Enforce(info *runtime.Info) error
}
```

**Extension Point: EnforceMLPolicy**

Applies machine learning-specific deployment parameters.

**Use case:** Configure PyTorch distributed settings, MPI hostfiles, framework environment variables.

**Interface:**

```go
type EnforceMLPolicy interface {
    Enforce(info *runtime.Info) error
}
```

**Extension Point: ComponentBuilder**

Dynamically constructs Kubernetes resources using RuntimeInfo and TrainJob objects.

**Use case:** Build custom JobSets, Services, ConfigMaps based on job requirements.

**Interface:**

```go
type ComponentBuilder interface {
    Build(info *runtime.Info) ([]client.Object, error)
}
```

**Example:**

```go
type CustomServiceBuilder struct{}

func (b *CustomServiceBuilder) Build(info *runtime.Info) ([]client.Object, error) {
    var objects []client.Object

    // Create headless service for distributed training
    svc := &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      info.TrainJob.Name + "-headless",
            Namespace: info.TrainJob.Namespace,
        },
        Spec: corev1.ServiceSpec{
            ClusterIP: "None",
            Selector: map[string]string{
                "trainer.kubeflow.org/job-name": info.TrainJob.Name,
            },
            Ports: []corev1.ServicePort{
                {Port: 29500, Name: "master"},
            },
        },
    }
    objects = append(objects, svc)

    return objects, nil
}
```

### 4. PostExecution Phase

Monitors job state and applies terminal conditions after execution completes.

**Internal Operations:**
- Monitor JobSet completion status
- Update TrainJob status conditions
- Clean up temporary resources
- Record metrics and events

:::{note}
PostExecution phase currently has no extension points but may support custom completion handlers in future versions.
:::

## Plugin Architecture

```{mermaid}
graph TB
    subgraph "Startup Phase"
        A[Controller Manager Start] --> B[Framework Init]
        B --> C[Register Controllers]
        B --> D[WatchExtension Plugins]
    end

    subgraph "PreExecution Phase"
        E[TrainJob Created/Updated] --> F[CustomValidation Plugins]
        F --> G{Valid?}
        G -->|No| H[Reject]
        G -->|Yes| I[Continue]
    end

    subgraph "Build Phase"
        I --> J[EnforcePodGroupPolicy]
        J --> K[EnforceMLPolicy]
        K --> L[ComponentBuilder Plugins]
        L --> M[Create Resources]
    end

    subgraph "PostExecution Phase"
        M --> N[Monitor JobSet]
        N --> O[Update Status]
        O --> P[Cleanup]
    end

    style A fill:#e1f5ff
    style E fill:#e1f5ff
    style I fill:#e1f5ff
    style M fill:#e1f5ff
    style F fill:#ffe1e1
    style J fill:#ffe1e1
    style K fill:#ffe1e1
    style L fill:#ffe1e1
    style D fill:#ffe1e1
```

## Built-in Plugins

Kubeflow Trainer includes several built-in plugins:

### PyTorch Plugin

**Phase:** Build (EnforceMLPolicy)

**Purpose:** Configure PyTorch distributed training environment

**Configuration:**
- Sets `torchrun` as entrypoint
- Configures MASTER_ADDR, MASTER_PORT
- Sets NPROC_PER_NODE based on GPU count
- Configures NCCL/Gloo backend

### MPI Plugin

**Phase:** Build (EnforceMLPolicy)

**Purpose:** Configure MPI-based training

**Configuration:**
- Creates launcher and worker pods
- Sets up SSH authentication
- Generates MPI hostfile
- Configures mpirun command

### JobSet Plugin

**Phase:** Build (ComponentBuilder)

**Purpose:** Create JobSet resources

**Configuration:**
- Converts TrainJob to JobSet spec
- Applies pod template overrides
- Sets up job dependencies
- Configures success policies

### Coscheduling Plugin

**Phase:** Build (EnforcePodGroupPolicy)

**Purpose:** Create PodGroups for gang scheduling

**Configuration:**
- Creates PodGroup resources
- Sets minMember based on numNodes
- Configures schedule timeout

### Volcano Plugin

**Phase:** Build (EnforcePodGroupPolicy)

**Purpose:** Create Volcano PodGroups

**Configuration:**
- Creates Volcano PodGroup CRD
- Configures queue assignments
- Sets network topology preferences

### Kueue Plugin

**Phase:** Build (ComponentBuilder)

**Purpose:** Integrate with Kueue workload management

**Configuration:**
- Adds Kueue labels to JobSet
- Configures queue admission
- Sets resource quotas

## Creating Custom Plugins

### Plugin Structure

```go
package myplugin

import (
    "github.com/kubeflow/trainer/pkg/runtime/framework"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

type MyCustomPlugin struct {
    // Plugin state
}

func (p *MyCustomPlugin) Enforce(info *framework.RuntimeInfo) error {
    // Plugin logic
    return nil
}

func New() framework.EnforceMLPolicy {
    return &MyCustomPlugin{}
}
```

### Registering Plugins

In controller-manager main.go:

```go
import (
    "github.com/kubeflow/trainer/pkg/runtime/framework"
    myplugin "github.com/myorg/trainer-plugins/myplugin"
)

func main() {
    // ... controller setup ...

    // Register custom plugin
    framework.RegisterMLPolicyPlugin("myplugin", myplugin.New())

    // Start controller
    mgr.Start(ctx)
}
```

### Example: Custom Validation Plugin

Validate GPU types match model requirements:

```go
package gpuvalidator

import (
    "fmt"
    "github.com/kubeflow/trainer/pkg/runtime/framework"
)

type GPUTypeValidator struct{}

func (v *GPUTypeValidator) Validate(info *framework.RuntimeInfo) error {
    // Check TrainJob annotation for required GPU type
    requiredGPU := info.TrainJob.Annotations["gpu.type/required"]
    if requiredGPU == "" {
        return nil // No requirement specified
    }

    // Extract runtime GPU configuration
    runtimeGPU := extractGPUType(info.Runtime)

    if runtimeGPU != requiredGPU {
        return fmt.Errorf(
            "runtime GPU type %s does not match required type %s",
            runtimeGPU, requiredGPU,
        )
    }

    return nil
}

func extractGPUType(runtime *runtime.Runtime) string {
    // Parse runtime labels or affinity rules
    if gpuType, ok := runtime.Labels["gpu.type"]; ok {
        return gpuType
    }
    return "unknown"
}

func New() framework.CustomValidation {
    return &GPUTypeValidator{}
}
```

### Example: Custom Resource Builder

Create a ConfigMap for each TrainJob:

```go
package configmapbuilder

import (
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "github.com/kubeflow/trainer/pkg/runtime/framework"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapBuilder struct{}

func (b *ConfigMapBuilder) Build(info *framework.RuntimeInfo) ([]client.Object, error) {
    var objects []client.Object

    // Create ConfigMap with training metadata
    cm := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      info.TrainJob.Name + "-config",
            Namespace: info.TrainJob.Namespace,
            Labels: map[string]string{
                "trainer.kubeflow.org/job-name": info.TrainJob.Name,
            },
        },
        Data: map[string]string{
            "num_nodes":         fmt.Sprintf("%d", info.TrainJob.Spec.Trainer.NumNodes),
            "runtime":           info.Runtime.Name,
            "framework":         info.Runtime.Labels["trainer.kubeflow.org/framework"],
            "created_timestamp": metav1.Now().Format(time.RFC3339),
        },
    }

    objects = append(objects, cm)
    return objects, nil
}

func New() framework.ComponentBuilder {
    return &ConfigMapBuilder{}
}
```

## RuntimeInfo Object

The `RuntimeInfo` object carries information through the plugin chain:

```go
type RuntimeInfo struct {
    // TrainJob being processed
    TrainJob *trainv1alpha1.TrainJob

    // Resolved runtime (ClusterTrainingRuntime or TrainingRuntime)
    Runtime *trainv1alpha1.Runtime

    // Effective ML policy (runtime + overrides)
    MLPolicy *trainv1alpha1.MLPolicy

    // JobSet template to be created
    JobSetTemplate *jobsetv1alpha2.JobSet

    // Additional resources to create
    AdditionalResources []client.Object
}
```

Plugins can read and modify this object to influence resource generation.

## Best Practices

### 1. Keep Plugins Focused

Each plugin should handle one specific concern:

```go
// Good: Single responsibility
type GPUValidator struct{}

// Avoid: Multiple responsibilities
type SuperPlugin struct{}  // Validates, builds, enforces
```

### 2. Validate Input

Always validate RuntimeInfo before processing:

```go
func (p *MyPlugin) Enforce(info *framework.RuntimeInfo) error {
    if info.TrainJob == nil {
        return fmt.Errorf("TrainJob cannot be nil")
    }
    if info.Runtime == nil {
        return fmt.Errorf("Runtime cannot be nil")
    }
    // ... plugin logic ...
}
```

### 3. Use Descriptive Errors

```go
// Good: Descriptive error
return fmt.Errorf("GPU type %s not supported, expected %s or %s",
    gpuType, "a100", "v100")

// Avoid: Vague error
return fmt.Errorf("invalid GPU")
```

### 4. Document Plugin Behavior

```go
// GPUAffinityPlugin ensures training pods are scheduled on nodes
// with the correct GPU type based on TrainJob annotations.
//
// Annotations:
//   - gpu.type/required: Specifies required GPU type (e.g., "a100")
//
// Example:
//   annotations:
//     gpu.type/required: "a100"
type GPUAffinityPlugin struct{}
```

### 5. Handle Edge Cases

```go
func (p *MyPlugin) Enforce(info *framework.RuntimeInfo) error {
    // Handle nil values
    if info.TrainJob.Spec.Trainer == nil {
        return nil  // Nothing to enforce
    }

    // Handle missing annotations
    gpuType, exists := info.TrainJob.Annotations["gpu.type"]
    if !exists {
        // Use default or skip
        gpuType = "default"
    }

    // ... plugin logic ...
}
```

### 6. Test Plugins Thoroughly

```go
func TestGPUValidator(t *testing.T) {
    tests := []struct {
        name    string
        info    *framework.RuntimeInfo
        wantErr bool
    }{
        {
            name: "valid GPU type",
            info: &framework.RuntimeInfo{
                TrainJob: &trainv1alpha1.TrainJob{
                    ObjectMeta: metav1.ObjectMeta{
                        Annotations: map[string]string{
                            "gpu.type/required": "a100",
                        },
                    },
                },
                Runtime: &trainv1alpha1.Runtime{
                    ObjectMeta: metav1.ObjectMeta{
                        Labels: map[string]string{
                            "gpu.type": "a100",
                        },
                    },
                },
            },
            wantErr: false,
        },
        // ... more test cases ...
    }

    validator := New()
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validator.Validate(tt.info)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Troubleshooting

### Plugin Not Called

Check plugin registration:

```go
// Verify plugin is registered
framework.RegisterMLPolicyPlugin("myplugin", myplugin.New())
```

Check controller logs for plugin initialization messages.

### Plugin Errors Not Surfaced

Ensure errors are returned properly:

```go
func (p *MyPlugin) Enforce(info *framework.RuntimeInfo) error {
    if err := p.validate(); err != nil {
        return err  // Must return error, not just log
    }
    return nil
}
```

### Resource Not Created

Check ComponentBuilder return values:

```go
func (b *MyBuilder) Build(info *framework.RuntimeInfo) ([]client.Object, error) {
    objects := []client.Object{myResource}
    return objects, nil  // Must return objects
}
```

## Example Use Cases

### Use Case 1: Enforce Resource Limits

Prevent jobs from requesting excessive resources:

```go
type ResourceLimitEnforcer struct {
    maxGPUPerJob int
    maxMemoryPerNode string
}

func (e *ResourceLimitEnforcer) Validate(info *framework.RuntimeInfo) error {
    // Check GPU limit
    if info.TrainJob.Spec.Trainer.ResourcesPerNode.Limits.Nvidia_gpu > e.maxGPUPerJob {
        return fmt.Errorf("GPU request exceeds maximum %d", e.maxGPUPerJob)
    }

    // Check memory limit
    memory := info.TrainJob.Spec.Trainer.ResourcesPerNode.Limits.Memory
    maxMem := resource.MustParse(e.maxMemoryPerNode)
    if memory.Cmp(maxMem) > 0 {
        return fmt.Errorf("memory request exceeds maximum %s", e.maxMemoryPerNode)
    }

    return nil
}
```

### Use Case 2: Auto-Configure Network Policies

Create network policies for training jobs:

```go
type NetworkPolicyBuilder struct{}

func (b *NetworkPolicyBuilder) Build(info *framework.RuntimeInfo) ([]client.Object, error) {
    np := &networkingv1.NetworkPolicy{
        ObjectMeta: metav1.ObjectMeta{
            Name:      info.TrainJob.Name + "-netpol",
            Namespace: info.TrainJob.Namespace,
        },
        Spec: networkingv1.NetworkPolicySpec{
            PodSelector: metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "trainer.kubeflow.org/job-name": info.TrainJob.Name,
                },
            },
            Ingress: []networkingv1.NetworkPolicyIngressRule{
                {
                    From: []networkingv1.NetworkPolicyPeer{
                        {
                            PodSelector: &metav1.LabelSelector{
                                MatchLabels: map[string]string{
                                    "trainer.kubeflow.org/job-name": info.TrainJob.Name,
                                },
                            },
                        },
                    },
                },
            },
        },
    }

    return []client.Object{np}, nil
}
```

## Next Steps

- [Training Runtimes](runtime) - Define runtime templates
- [ML Policies](ml-policy) - Configure ML-specific settings
- [Contributor Guides](../contributor-guides/index) - Contribute to Kubeflow Trainer
- [GitHub Repository](https://github.com/kubeflow/trainer) - Source code and examples
