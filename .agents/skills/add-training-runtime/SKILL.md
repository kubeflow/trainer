---
name: add-training-runtime
description: Step-by-step guide for adding a new training runtime plugin to the Kubeflow Trainer extension framework (pkg/runtime/framework). Covers the plugin implementation, factory registration, MLPolicy API wiring, and the ClusterTrainingRuntime manifest. Use this when asked to add support for a new distributed ML framework (e.g. PyTorch, JAX, MPI, XGBoost) or to scaffold a new runtime plugin so it plugs into the runtime chain correctly.
---

# Add a Training Runtime Plugin

Kubeflow Trainer builds a TrainJob's underlying resources by running it through a
chain of plugins (the "extension framework"). A runtime plugin's job is to take
the runtime `Info` object and the user's `TrainJob`, and mutate the trainer
PodSet/containers so the framework's distributed training convention is applied
(process count, rank, coordinator address, env vars, etc.).

Adding a runtime means touching four places, all consistent with the existing
`torch`, `jax`, `mpi`, and `xgboost` plugins:

1. The MLPolicy API type (so a runtime can select your plugin).
2. The plugin implementation under `pkg/runtime/framework/plugins/<name>/`.
3. The plugin registry (`pkg/runtime/framework/plugins/registry.go`).
4. A `ClusterTrainingRuntime` manifest under `manifests/base/runtimes/`.

Read `pkg/runtime/framework/plugins/jax/jax.go` first — it's the smallest
complete example and the best template to copy.

## 1. Add the MLPolicy source

A plugin only runs when the runtime's `mlPolicy` selects it. Add a field to
`MLPolicySource` in `pkg/apis/trainer/v1alpha1/trainingruntime_types.go`:

```go
type MLPolicySource struct {
    // ... existing torch, mpi, flux, jax, xgboost ...

    // myframework defines the configuration for the MyFramework runtime.
    // +optional
    MyFramework *MyFrameworkMLPolicySource `json:"myframework,omitempty"`
}

// MyFrameworkMLPolicySource represents the MyFramework runtime configuration.
type MyFrameworkMLPolicySource struct{}
```

Use an empty struct if the runtime needs no extra config (like `JAXMLPolicySource`);
add fields only if the plugin actually reads them.

Because this is an API change, regenerate the deep-copy and CRD artifacts:

```bash
make generate manifests
```

Do not hand-edit generated files (`zz_generated.deepcopy.go`, the CRD YAML).

## 2. Implement the plugin

Create `pkg/runtime/framework/plugins/myframework/myframework.go`. Most runtimes
implement `EnforceMLPolicyPlugin` (and often `CustomValidationPlugin`). See the
interfaces in `pkg/runtime/framework/interface.go`.

```go
package myframework

import (
    "context"
    "fmt"

    corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
    "k8s.io/apimachinery/pkg/util/validation/field"
    "k8s.io/utils/ptr"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

    configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
    trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
    "github.com/kubeflow/trainer/v2/pkg/apply"
    "github.com/kubeflow/trainer/v2/pkg/constants"
    "github.com/kubeflow/trainer/v2/pkg/runtime"
    "github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

type MyFramework struct{}

// Assert which interfaces this plugin implements.
var _ framework.EnforceMLPolicyPlugin = (*MyFramework)(nil)
var _ framework.CustomValidationPlugin = (*MyFramework)(nil)

const Name = "MyFramework"

// New is the factory referenced by the registry. The signature is fixed.
func New(context.Context, client.Client, client.FieldIndexer, *configapi.Configuration) (framework.Plugin, error) {
    return &MyFramework{}, nil
}

func (m *MyFramework) Name() string {
    return Name
}

func (m *MyFramework) Validate(_ context.Context, info *runtime.Info, _, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
    // Return early unless this runtime selected the MyFramework policy.
    if info == nil || info.RuntimePolicy.MLPolicySource == nil || info.RuntimePolicy.MLPolicySource.MyFramework == nil {
        return nil, nil
    }
    return nil, nil
}

func (m *MyFramework) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
    // Guard: only act when the runtime selected MyFramework.
    if info == nil || info.RuntimePolicy.MLPolicySource == nil || info.RuntimePolicy.MLPolicySource.MyFramework == nil {
        return nil
    }

    // Propagate NumNodes from the TrainJob into the trainer PodSet count.
    trainerPS := info.FindPodSetByAncestor(constants.AncestorTrainer)
    if trainerPS != nil && trainerPS.Count != nil && trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
        *trainerPS.Count = *trainJob.Spec.Trainer.NumNodes
    }

    // Inject the distributed env vars into the trainer container.
    trainerContainer := info.FindContainerByPodSetAncestorContainerName(constants.AncestorTrainer, constants.Node)
    if trainerContainer != nil {
        numNodes := ptr.Deref(ptr.Deref(trainerPS, runtime.PodSet{}).Count, 1)
        apply.UpsertEnvVars(&trainerContainer.Env,
            *corev1ac.EnvVar().
                WithName("MYFRAMEWORK_NUM_NODES").
                WithValue(fmt.Sprintf("%d", numNodes)),
            // Per-node rank derived from the job completion index.
            *corev1ac.EnvVar().
                WithName("MYFRAMEWORK_NODE_RANK").
                WithValueFrom(corev1ac.EnvVarSource().
                    WithFieldRef(corev1ac.ObjectFieldSelector().
                        WithFieldPath(constants.JobCompletionIndexFieldPath))),
        )
        // If pods talk to each other, open the trainer port for the headless service.
        apply.UpsertPort(&trainerContainer.Ports, *corev1ac.ContainerPort().WithContainerPort(constants.ContainerTrainerPort))
    }

    return nil
}
```

Key `Info` helpers you'll reuse (see `pkg/runtime/runtime.go`):

- `FindPodSetByAncestor(constants.AncestorTrainer)` — the trainer PodSet.
- `FindContainerByPodSetAncestorContainerName(constants.AncestorTrainer, constants.Node)` — the main trainer container.
- `apply.UpsertEnvVars` / `apply.UpsertPort` — mutate via server-side-apply configs, never raw structs.

Coordinator/master address convention (from `torch`/`jax`): the first pod is
`<trainJob.Name>-<constants.Node>-0-0.<trainJob.Name>` on
`constants.ContainerTrainerPort`. Reuse it so the headless service resolves.

## 3. Register the plugin

Add the factory to `NewRegistry()` in
`pkg/runtime/framework/plugins/registry.go`. This is the step most often missed —
without it the plugin never runs.

```go
import (
    // ...
    "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/myframework"
)

func NewRegistry() Registry {
    registry := Registry{
        // ... existing entries ...
        myframework.Name: myframework.New,
    }
    // ...
}
```

## 4. Add the ClusterTrainingRuntime manifest

Add `manifests/base/runtimes/myframework_distributed.yaml` and list it in the
sibling `kustomization.yaml`. Model it on `jax_distributed.yaml`: set the
`mlPolicy.<myframework>` selector, and label the replicated job with the
`trainer.kubeflow.org/trainjob-ancestor-step: trainer` label so the ancestor
lookups in step 2 resolve.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: myframework-distributed
  labels:
    trainer.kubeflow.org/framework: myframework
spec:
  mlPolicy:
    numNodes: 1
    myframework: {}
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: <your-runtime-image>
```

## 5. Test and verify

Add `myframework_test.go` next to the plugin (copy `jax_test.go`'s table-driven
style) and run the checks from `AGENTS.md`:

```bash
make generate manifests fmt
go test ./pkg/runtime/framework/plugins/myframework/...
make test
```

## Pitfalls

- Every plugin method must early-return when its `MLPolicySource` field is nil —
  the whole chain runs for every TrainJob, so an unguarded plugin corrupts jobs
  that don't use it.
- Forgetting the registry entry (step 3) is silent: the code compiles and tests
  in the package pass, but the plugin never executes at runtime.
- Don't hand-edit generated deep-copy or CRD files; rerun `make generate manifests`.
- Mutate the `Info` object through the `apply.*` helpers, not by assigning to
  container structs directly — the framework builds resources from apply configs.
- The `New` factory signature is fixed by the `Registry` map type; don't change it
  even if you ignore the client/indexer/config arguments.
