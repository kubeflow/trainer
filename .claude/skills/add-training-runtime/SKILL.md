# Add a Training Runtime Plugin

Step-by-step guide for adding a new runtime plugin to the Kubeflow Trainer extension framework.

## Steps

### 1. Create the plugin package

Create a new directory under `pkg/runtime/framework/plugins/yourplugin/`.

Create `yourplugin.go` with the following structure:

```go
package yourplugin

import (
    "context"

    "sigs.k8s.io/controller-runtime/pkg/client"

    configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
    "github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

const Name = "YourPlugin"

// Compile-time interface assertions - add one per interface you implement
var _ framework.EnforceMLPolicyPlugin = (*YourPlugin)(nil)

type YourPlugin struct{}

func New(ctx context.Context, c client.Client, indexer client.FieldIndexer, cfg *configapi.Configuration) (framework.Plugin, error) {
    return &YourPlugin{}, nil
}

func (p *YourPlugin) Name() string { return Name }
```

### 2. Choose which interfaces to implement

All interfaces are defined in `pkg/runtime/framework/interface.go`:

- **EnforceMLPolicyPlugin** - configure ML framework-specific settings (env vars, node counts). Most runtime plugins implement this.
- **CustomValidationPlugin** - validate TrainJob fields specific to your framework. Implement if your plugin has constraints (e.g., reserved env vars, required PodSets).
- **ComponentBuilderPlugin** - generate additional Kubernetes resources (Secrets, ConfigMaps). Implement if your framework needs auxiliary resources (e.g., MPI needs SSH keys and hostfiles).
- **WatchExtensionPlugin** - watch additional Kubernetes resources. Implement if your plugin creates resources outside the normal JobSet.
- **EnforcePodGroupPolicyPlugin** - configure gang scheduling. Only for scheduler integrations.
- **PodNetworkPlugin** - configure pod networking. Only for network topology plugins.

Use compile-time assertions (`var _ framework.XPlugin = (*YourPlugin)(nil)`) for each interface.

### 3. Implement EnforceMLPolicy (most common)

The `Info` object (`pkg/runtime/runtime.go`) carries data through the pipeline. Use its methods:

- `info.FindPodSetByAncestor("trainer")` - find the training PodSet
- `info.FindContainerByPodSetName("Node", "trainer")` - find a specific container
- `info.RuntimePolicy.MLPolicy` - access the ML policy from the runtime definition

Pattern from existing plugins (torch, mpi, jax):

```go
func (p *YourPlugin) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
    if info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.YourFramework == nil {
        return nil
    }
    // Modify PodSets in info to inject env vars, update counts, etc.
    return nil
}
```

### 4. Register the plugin

Edit `pkg/runtime/framework/plugins/registry.go`:

1. Add import: `"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/yourplugin"`
2. Add entry in `NewRegistry()`: `yourplugin.Name: yourplugin.New,`

If the plugin should be feature-gated, wrap it like `trainjobstatus`:

```go
if features.Enabled(features.YourFeature) {
    registry[yourplugin.Name] = yourplugin.New
}
```

### 5. Add ML policy types (if needed)

If your plugin introduces a new ML framework, add the policy type to `pkg/apis/trainer/v1alpha1/trainingruntime_types.go`:

1. Add a new struct (e.g., `YourFrameworkPolicy`)
2. Add a field to `MLPolicy` struct pointing to your new type
3. Run `make generate`

### 6. Write tests

Create `yourplugin_test.go` in the same directory. Follow the table-driven pattern:

```go
func TestYourPluginEnforceMLPolicy(t *testing.T) {
    cases := map[string]struct {
        info     *runtime.Info
        trainJob *trainer.TrainJob
        wantInfo *runtime.Info
        wantErr  error
    }{
        "case description": {
            // Use utiltesting wrappers to build test objects
        },
    }
    for name, tc := range cases {
        t.Run(name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

Use test utilities from `pkg/util/testing/`:
- `utiltesting.MakeTrainJobWrapper()` - build TrainJob objects
- `utiltesting.MakeMLPolicyWrapper()` - build MLPolicy objects
- `utiltesting.NewClientBuilder()` - create fake Kubernetes clients
- `cmp.Diff()` with `cmpopts` for deep comparison

### 7. Verify

```bash
make generate       # If you modified API types
make fmt
make vet
make golangci-lint
go test ./pkg/runtime/framework/plugins/yourplugin/...
make test
```

## Common Mistakes

- Not adding compile-time interface assertions (`var _ framework.XPlugin = ...`)
- Forgetting to register the plugin in `registry.go`
- Modifying the Info object incorrectly - always use the provided methods (`FindPodSetByAncestor`, etc.)
- Not running `make generate` after adding new API types
- Assuming plugin execution order within the same interface type (it's non-deterministic)
