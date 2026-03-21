# KEP-2839: Dynamic LLM Trainer Framework

**Authors**: NarayanaSabari

**Status**: Provisional

**Creation date**: 2026-02-27

**Tracking issue**: [kubeflow/trainer#2839](https://github.com/kubeflow/trainer/issues/2839)

**Upstream KEP**: [KEP-2401: Kubeflow LLM Trainer V2](../2401-llm-trainer-v2/README.md)

## Table of Contents

<!-- toc -->
- [KEP-2839: Dynamic LLM Trainer Framework](#kep-2839-dynamic-llm-trainer-framework)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [Current State Analysis](#current-state-analysis)
    - [How TorchTune Is Wired Today](#how-torchtune-is-wired-today)
    - [SDK Coupling](#sdk-coupling)
    - [Why This Must Change](#why-this-must-change)
  - [Relationship to KEP-285 (Specialized Trainer Abstractions)](#relationship-to-kep-285-specialized-trainer-abstractions)
  - [High-Level Design](#high-level-design)
    - [Architecture Overview](#architecture-overview)
    - [Component Interaction Flow](#component-interaction-flow)
    - [What Changes vs What Stays](#what-changes-vs-what-stays)
  - [Design Details](#design-details)
    - [Python SDK: `LLMBackend` Interface](#python-sdk-llmbackend-interface)
    - [Python SDK: `TRLConfig`](#python-sdk-trlconfig)
    - [Python SDK: Integration into `KubernetesBackend`](#python-sdk-integration-into-kubernetesbackend)
    - [Go Control Plane: `LLMBackendStrategy` Interface](#go-control-plane-llmbackendstrategy-interface)
    - [Go Control Plane: `TorchTuneStrategy`](#go-control-plane-torchtunestrategy)
    - [Go Control Plane: `TRLStrategy`](#go-control-plane-trlstrategy)
    - [Go Control Plane: Refactored Torch Plugin Dispatch](#go-control-plane-refactored-torch-plugin-dispatch)
    - [Go Control Plane: New Constant](#go-control-plane-new-constant)
    - [TRL Container Image](#trl-container-image)
    - [TRL `ClusterTrainingRuntime` Manifest](#trl-clustertrainingruntime-manifest)
    - [SDK Usage Example](#sdk-usage-example)
  - [Risks and Mitigations](#risks-and-mitigations)
<!-- /toc -->

---

## Summary

Decouple the `BuiltinTrainer` from TorchTune by introducing a pluggable `LLMBackend`
interface in the SDK and a corresponding `LLMBackendStrategy` in the Go control plane.
TorchTune becomes the first backend implementation (preserving backward compatibility),
and TRL is added as the first new backend with SFT/DPO support. Config-driven backends
sit alongside [KEP-285](https://github.com/kubeflow/sdk/pull/308)'s function-based
trainers as Tier 2 extensions; see
[Relationship to KEP-285](#relationship-to-kep-285-specialized-trainer-abstractions).

This builds on [KEP-2401](../2401-llm-trainer-v2/README.md) and the community consensus
on "Plan 3" in [#2752](https://github.com/kubeflow/trainer/issues/2752).
TorchTune stopped adding features in July 2025
([pytorch/torchtune#2883](https://github.com/pytorch/torchtune/issues/2883)).

## Goals

1. Define an `LLMBackend` abstract interface in the Python SDK for config-driven trainers.
2. Refactor `TorchTuneConfig` to implement `LLMBackend` with zero breaking changes.
3. Implement `TRLConfig` backend supporting SFT and DPO.
4. Create TRL container image and `ClusterTrainingRuntime` manifests.
5. Generalize the Go Torch plugin to dispatch via `LLMBackendStrategy` instead of
   hardcoded TorchTune command-sniffing.

## Non-Goals

1. Unsloth or LlamaFactory backends (future work).
2. CRD schema changes — operates within existing `.spec.trainer.command`/`.spec.trainer.args`.
3. New Kubernetes resource topologies (e.g., launcher/worker patterns).
4. Go-side distributed training plugins per backend (backends use existing torchrun infra).

---

## Current State Analysis

### How TorchTune Is Wired Today

The Torch plugin (`pkg/runtime/framework/plugins/torch/torch.go`) is the only ML policy
plugin that handles LLM fine-tuning. It hardcodes TorchTune support via **command-sniffing**:

```go
// torch.go:149 — the branching point
if !slices.Equal(trainJob.Spec.Trainer.Command, constants.TorchTuneEntrypoint) {
    // Standard torchrun path: inject PET_MASTER_ADDR, PET_MASTER_PORT
} else {
    // TorchTune path: mutate command with recipe, config, rdzv_endpoint
}
```

`constants.TorchTuneEntrypoint` is `[]string{"tune", "run"}`. When the trainer command
matches this, the plugin enters the TorchTune branch (torch.go:159-183) which:

1. Builds the rendezvous endpoint: `--rdzv_endpoint={name}-node-0-0.{name}:29500`
2. Calls `getRecipeAndConfig()` (torchtune.go:78) to select a recipe/config pair
   from a matrix of `numNodes × numGPUs × LoRA/QLoRA` combinations.
3. Calls `extractOverridesFromRuntime()` (torchtune.go:131) to pull immutable config
   overrides from the ClusterTrainingRuntime's node container command.
4. Appends all of this to `trainJob.Spec.Trainer.Command`.

The validation path (torch.go:88) also sniffs the same entrypoint to decide whether
to run `validateTorchTune()`.

### SDK Coupling

In the Python SDK (`kubeflow/sdk` repo), `BuiltinTrainer` has a single field:

```python
@dataclass
class BuiltinTrainer:
    config: TorchTuneConfig  # No other option
```

The `KubernetesBackend.train()` method calls `get_args_using_torchtune_config()` in
`backends/kubernetes/utils.py` to translate the config into CLI args. There is no
abstraction — adding a new backend means modifying this function and the type annotation.

### Why This Must Change

- **TorchTune stopped adding features** in July 2025. The project is in maintenance mode.
- **The command-sniffing pattern doesn't scale.** Each new backend would require another
  `slices.Equal` check, another branch in `EnforceMLPolicy`, and another branch in `Validate`.
- **Community consensus on Plan 3** (pluggable framework) from #2752 was unanimous.
- **TRL is actively maintained** by HuggingFace with native CLI support (`trl sft`, `trl dpo`, etc.)
  and built-in accelerate integration for multi-GPU/multi-node.

---

## Relationship to KEP-285 (Specialized Trainer Abstractions)

[KEP-285](https://github.com/kubeflow/sdk/pull/308) introduces a `BaseTrainer` ABC for
function-based trainers (`TorchTrainer`, `JAXTrainer`, etc.) and a `RuntimeConfig`
dataclass. This KEP is complementary — it addresses **config-driven trainers** where the
framework's own CLI is the entrypoint (e.g., `trl sft ...`, `tune run ...`), not a
user-supplied Python function.

In KEP-285's terminology, `LLMBackend` implementations are **Tier 2 config-driven
trainers**. If KEP-285 merges first, `LLMBackend` configs can be passed through
KEP-285's `TorchTrainer` instead of `BuiltinTrainer` — the interface is the same
(`command` class var / `to_args()`), only the entry point changes.

**Shared design points**:

- Both use `trainer.kubeflow.org/framework` as the dispatch key — KEP-285 for SDK
  runtime auto-discovery, this KEP for Go strategy dispatch.
- Both KEPs are compatible with either keeping or deprecating `BuiltinTrainer`.
- If the framework label is promoted to a Runtime API spec field (as discussed in the
  KEP-285 review), both KEPs benefit with no changes.

---

## High-Level Design

### Architecture Overview

The change is a **localized refactor** of two coupling points. No new CRDs, no new
controllers, no changes to the plugin framework itself.

```
                        BEFORE                              AFTER
                   ┌──────────────┐                  ┌──────────────┐
  SDK              │BuiltinTrainer│                  │BuiltinTrainer│
                   │ config:      │                  │ config:      │
                   │  TorchTune   │                  │  LLMBackend  │
                   │  Config      │                  │  (abstract)  │
                   └──────┬───────┘                  └──────┬───────┘
                           │                                 │
                           │ to_args()                       │ config.command / to_args()
                           ▼                                 ▼
                    get_args_using_                   config.command
                    torchtune_config()                config.to_args()
                          │                                 │
                          │ creates TrainJob CR              │ creates TrainJob CR
                          ▼                                 ▼
  ┌────────────────────────────────────────────────────────────────────────┐
  │                         Kubernetes API                                 │
  └────────────────────────────────┬───────────────────────────────────────┘
                                   │
  Go                               ▼
  Torch          ┌─────────────────────────────────┐
  Plugin         │ EnforceMLPolicy()                │
                 │                                  │
   BEFORE:       │ if cmd == ["tune","run"]:        │
                 │   → TorchTune branch             │
                 │ else:                            │
                 │   → torchrun branch              │
                 │                                  │
   AFTER:        │ // common: PET env vars          │
                 │ label = info.Labels[framework]   │
                 │ if strategy = backends[label]:   │
                 │   → strategy.EnforceCommand()    │
                 │ else:                            │
                 │   → default torchrun branch      │
                 └─────────────────────────────────┘
```

### Component Interaction Flow

End-to-end for a TRL SFT job:

```
1. User: TrainerClient.train(builtin_trainer=BuiltinTrainer(config=TRLConfig(
       trainer_type=TRLTrainerType.SFT, ...)))

2. SDK:  TRLConfig.validate() → ok
         TRLConfig.command   → ("trl",)
         TRLConfig.to_args() → ["sft", "--model_name_or_path", "/workspace/model", ...]
         Build TrainJob CR with:
           runtimeRef: { name: "trl-llama3.2-1b" }
           trainer: { command: ["trl"], args: ["sft", ...] }

3. K8s:  Webhook validates TrainJob
         Torch plugin Validate() → label=trl → TRLStrategy.Validate() → ok

4. Go:   TrainJob controller reconciles:
         Torch EnforceMLPolicy():
           a) Common: set PET_NNODES, PET_NPROC_PER_NODE, PET_NODE_RANK
           b) Label "trl" → TRLStrategy.EnforceCommand():
              inject PET_MASTER_ADDR, PET_MASTER_PORT
              inject MASTER_ADDR, MASTER_PORT, WORLD_SIZE, RANK (accelerate-compatible)
           c) Add container port

5. K8s:  Controller SSA → JobSet → ReplicatedJobs → Pods
         Init: dataset-initializer downloads dataset
         Init: model-initializer downloads model
         Main: trl sft --model_name_or_path /workspace/model ...
```

### What Changes vs What Stays

| Component | Changes? | Details |
|-----------|----------|---------|
| CRD schemas | **No** | No new fields, no new types |
| Plugin framework interfaces | **No** | Same 7 interfaces |
| Controller reconciliation | **No** | Same SSA flow |
| Webhooks | **No** | Same validation hooks (Torch plugin gains strategy dispatch) |
| Torch plugin (common path) | **No** | PET env var injection stays |
| Torch plugin (TorchTune path) | **Refactor** | Extract inline code → `TorchTuneStrategy` |
| Torch plugin (dispatch) | **New** | Label-based strategy lookup replaces command-sniffing |
| TRL strategy | **New** | `TRLStrategy` for TRL-specific env vars |
| SDK `BuiltinTrainer` | **Widen** | `TorchTuneConfig` → `LLMBackend` |
| SDK `TorchTuneConfig` | **Implement** | Implements `LLMBackend` (backward compatible) |
| SDK `TRLConfig` | **New** | New backend class |
| Container images | **New** | `trl-trainer` image |
| ClusterTrainingRuntimes | **New** | TRL-specific runtime manifests |

---

## Design Details

### Python SDK: `LLMBackend` Interface

Today `BuiltinTrainer.config` is typed as `TorchTuneConfig` directly. This introduces an
abstract base class that every config-driven backend must implement.

```python
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import ClassVar


class LLMBackend(ABC):
    """Abstract base for config-driven LLM training backends.

    Each implementation translates its config into a (command, args) pair
    that the Kubernetes backend writes into the TrainJob CR.
    """

    # Subclasses set this to their CLI entrypoint, e.g. ("tune", "run") or ("trl",)
    command: ClassVar[tuple[str, ...]]

    # Common fields shared by all backends
    num_nodes: int | None = None
    resources_per_node: dict | None = None

    @abstractmethod
    def to_args(self, initializer: "Initializer | None" = None) -> list[str]:
        """Return CLI arguments for the entrypoint."""
        ...

    @abstractmethod
    def validate(self) -> None:
        """Raise ValueError if the config is invalid."""
        ...
```

`BuiltinTrainer` widens its type annotation:

```python
@dataclass
class BuiltinTrainer:
    """Builtin Trainer configuration."""
    config: LLMBackend  # was: TorchTuneConfig
```

`TorchTuneConfig` implements `LLMBackend` with no field changes — backward compatible:

```python
@dataclass
class TorchTuneConfig(LLMBackend):
    command = ("tune", "run")

    dtype: DataType | None = None
    batch_size: int | None = None
    epochs: int | None = None
    loss: Loss | None = None
    peft_config: LoraConfig | None = None
    dataset_preprocess_config: TorchTuneInstructDataset | None = None

    def to_args(self, initializer=None) -> list[str]:
        # Existing get_args_using_torchtune_config() logic moves here
        ...

    def validate(self) -> None:
        ...
```

### Python SDK: `TRLConfig`

```python
from enum import Enum


class TRLTrainerType(Enum):
    """Training algorithms available via the TRL CLI."""
    SFT = "sft"
    DPO = "dpo"
    KTO = "kto"
    GRPO = "grpo"


@dataclass
class TRLConfig(LLMBackend):
    """TRL LLM Trainer configuration.

    Args:
        trainer_type: Training algorithm (SFT, DPO, KTO, GRPO).
        model_name_or_path: HuggingFace model ID or local path.
        dataset_name: HuggingFace dataset ID or local path.
        learning_rate: Learning rate.
        num_train_epochs: Number of training epochs.
        per_device_train_batch_size: Batch size per device.
        gradient_checkpointing: Enable gradient checkpointing.
        bf16: Use bfloat16 precision.
        use_peft: Enable LoRA via PEFT.
        lora_r: LoRA rank.
        lora_alpha: LoRA alpha.
        lora_target_modules: Comma-separated target modules for LoRA.
        extra_args: Additional CLI arguments passed through verbatim.
    """

    command = ("trl",)

    trainer_type: TRLTrainerType = TRLTrainerType.SFT
    model_name_or_path: str | None = None
    dataset_name: str | None = None
    learning_rate: float | None = None
    num_train_epochs: int | None = None
    per_device_train_batch_size: int | None = None
    gradient_checkpointing: bool = True
    bf16: bool = True
    use_peft: bool = False
    lora_r: int | None = None
    lora_alpha: int | None = None
    lora_target_modules: str | None = None
    extra_args: dict[str, str] | None = None

    def to_args(self, initializer=None) -> list[str]:
        args = [self.trainer_type.value]  # subcommand: "sft", "dpo", etc.

        # Model path: prefer initializer workspace, fall back to config
        model_path = self.model_name_or_path
        if initializer and initializer.model:
            model_path = "/workspace/model"
        if model_path:
            args.extend(["--model_name_or_path", model_path])

        # Dataset: prefer initializer workspace, fall back to config
        dataset = self.dataset_name
        if initializer and initializer.dataset:
            dataset = "/workspace/dataset"
        if dataset:
            args.extend(["--dataset_name", dataset])

        if self.learning_rate is not None:
            args.extend(["--learning_rate", str(self.learning_rate)])
        if self.num_train_epochs is not None:
            args.extend(["--num_train_epochs", str(self.num_train_epochs)])
        if self.per_device_train_batch_size is not None:
            args.extend(["--per_device_train_batch_size", str(self.per_device_train_batch_size)])
        if self.gradient_checkpointing:
            args.append("--gradient_checkpointing")
        if self.bf16:
            args.append("--bf16")
        if self.use_peft:
            args.append("--use_peft")
            if self.lora_r is not None:
                args.extend(["--lora_r", str(self.lora_r)])
            if self.lora_alpha is not None:
                args.extend(["--lora_alpha", str(self.lora_alpha)])
            if self.lora_target_modules:
                args.extend(["--lora_target_modules", self.lora_target_modules])

        # Pass-through extra args
        if self.extra_args:
            for k, v in self.extra_args.items():
                args.extend([f"--{k}", v])

        return args

    def validate(self) -> None:
        if self.use_peft and self.lora_r is None:
            raise ValueError("lora_r is required when use_peft=True")
```

### Python SDK: Integration into `KubernetesBackend`

The current `get_trainer_cr_from_builtin_trainer()` hardcodes `isinstance(trainer.config, TorchTuneConfig)`.
This changes to use the `LLMBackend` interface directly:

```python
# backends/kubernetes/utils.py (modified)

def get_trainer_cr_from_builtin_trainer(
    runtime: types.Runtime,
    trainer: types.BuiltinTrainer,
    initializer: types.Initializer | None = None,
) -> models.TrainerV1alpha1Trainer:
    config = trainer.config
    config.validate()

    trainer_cr = models.TrainerV1alpha1Trainer()
    if config.num_nodes:
        trainer_cr.num_nodes = config.num_nodes
    if config.resources_per_node:
        trainer_cr.resources_per_node = get_resources_per_node(config.resources_per_node)

    trainer_cr.command = list(config.command)
    trainer_cr.args = config.to_args(initializer)
    return trainer_cr
```

### Go Control Plane: `LLMBackendStrategy` Interface

Inside the Torch plugin package, a strategy interface replaces the inline if/else:

```go
// pkg/runtime/framework/plugins/torch/strategy.go

package torch

import (
    "k8s.io/apimachinery/pkg/util/validation/field"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

    trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
    "github.com/kubeflow/trainer/v2/pkg/runtime"
)

// LLMBackendStrategy defines backend-specific behavior for the Torch plugin.
// Each strategy handles the portion of EnforceMLPolicy and Validate that differs
// between backends (e.g., command mutation, env var injection, validation rules).
type LLMBackendStrategy interface {
    // EnforceCommand mutates the trainer container's command, args, and env vars
    // with backend-specific values (e.g., rendezvous args for TorchTune,
    // accelerate env vars for TRL).
    EnforceCommand(info *runtime.Info, trainJob *trainer.TrainJob, container *runtime.Container) error

    // Validate performs backend-specific validation on the TrainJob.
    Validate(runtimeInfo *runtime.Info, trainJob *trainer.TrainJob) (admission.Warnings, field.ErrorList)
}
```

### Go Control Plane: `TorchTuneStrategy`

Extracts the existing inline code from `torch.go:159-183` and `torchtune.go`:

```go
// pkg/runtime/framework/plugins/torch/torchtune_strategy.go

type TorchTuneStrategy struct{}

func (s *TorchTuneStrategy) EnforceCommand(
    info *runtime.Info,
    trainJob *trainer.TrainJob,
    container *runtime.Container,
) error {
    // Moved from torch.go:159-183
    // 1. Build rendezvous endpoint args
    // 2. Call getRecipeAndConfig() for recipe/config selection
    // 3. Call extractOverridesFromRuntime() for immutable overrides
    // 4. Append to trainJob.Spec.Trainer.Command
    return nil
}

func (s *TorchTuneStrategy) Validate(
    runtimeInfo *runtime.Info,
    trainJob *trainer.TrainJob,
) (admission.Warnings, field.ErrorList) {
    // Calls existing validateTorchTune()
    return validateTorchTune(runtimeInfo, trainJob)
}
```

### Go Control Plane: `TRLStrategy`

```go
// pkg/runtime/framework/plugins/torch/trl_strategy.go

type TRLStrategy struct{}

func (s *TRLStrategy) EnforceCommand(
    info *runtime.Info,
    trainJob *trainer.TrainJob,
    container *runtime.Container,
) error {
    trainerPS := info.FindPodSetByAncestor(constants.AncestorTrainer)
    numNodes := ptr.Deref(ptr.Deref(trainerPS, runtime.PodSet{}).Count, 1)
    masterAddr := fmt.Sprintf("%s-%s-0-0.%s", trainJob.Name, constants.Node, trainJob.Name)
    masterPort := fmt.Sprintf("%d", constants.ContainerTrainerPort)
    worldSize := fmt.Sprintf("%d", numNodes * numProcPerNode)

    // TRL uses accelerate, which reads standard env vars (not PET_* variants).
    // Inject both sets for compatibility.
    apply.UpsertEnvVars(&container.Env,
        // PET env vars (for torchrun compatibility)
        *corev1ac.EnvVar().WithName(constants.TorchEnvMasterAddr).WithValue(masterAddr),
        *corev1ac.EnvVar().WithName(constants.TorchEnvMasterPort).WithValue(masterPort),
        // Standard env vars (for accelerate/TRL)
        *corev1ac.EnvVar().WithName("MASTER_ADDR").WithValue(masterAddr),
        *corev1ac.EnvVar().WithName("MASTER_PORT").WithValue(masterPort),
        *corev1ac.EnvVar().WithName("WORLD_SIZE").WithValue(worldSize),
        *corev1ac.EnvVar().WithName("RANK").WithValueFrom(
            corev1ac.EnvVarSource().WithFieldRef(
                corev1ac.ObjectFieldSelector().WithFieldPath(constants.JobCompletionIndexFieldPath),
            ),
        ),
    )
    return nil
}

func (s *TRLStrategy) Validate(
    runtimeInfo *runtime.Info,
    trainJob *trainer.TrainJob,
) (admission.Warnings, field.ErrorList) {
    // TRL validation: check that trainer_type subcommand is valid, etc.
    return nil, nil
}
```

### Go Control Plane: Refactored Torch Plugin Dispatch

The `Torch` struct gains a `backends` map, and `EnforceMLPolicy` dispatches by label:

```go
// pkg/runtime/framework/plugins/torch/torch.go (modified)

type Torch struct {
    backends map[string]LLMBackendStrategy
}

func New(ctx context.Context, c client.Client, fi client.FieldIndexer) (framework.Plugin, error) {
    return &Torch{
        backends: map[string]LLMBackendStrategy{
            "torchtune": &TorchTuneStrategy{},
            "trl":       &TRLStrategy{},
        },
    }, nil
}
```

The dispatch logic in `EnforceMLPolicy` changes from command-sniffing to label lookup:

```go
func (t *Torch) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
    // ... (existing common logic: numNodes, numProcPerNode, PET_NNODES,
    //       PET_NPROC_PER_NODE, PET_NODE_RANK — unchanged) ...

    // NEW: label-based dispatch replaces command-sniffing
    framework := info.Labels[constants.RuntimeFrameworkLabel]  // "trainer.kubeflow.org/framework"
    if strategy, ok := t.backends[framework]; ok {
        if err := strategy.EnforceCommand(info, trainJob, trainerContainer); err != nil {
            return err
        }
    } else {
        // Default: standard torchrun path (PET_MASTER_ADDR, PET_MASTER_PORT)
        apply.UpsertEnvVars(&trainerContainer.Env,
            *corev1ac.EnvVar().WithName(constants.TorchEnvMasterAddr).WithValue(...),
            *corev1ac.EnvVar().WithName(constants.TorchEnvMasterPort).WithValue(...),
        )
    }

    // ... (existing: add container port) ...
    return nil
}
```

The same pattern applies to `Validate`:

```go
func (t *Torch) Validate(ctx context.Context, runtimeInfo *runtime.Info, _, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
    // ... (existing common validation: numProcPerNode, reserved envs) ...

    // NEW: label-based dispatch replaces command-sniffing
    framework := runtimeInfo.Labels[constants.RuntimeFrameworkLabel]
    if strategy, ok := t.backends[framework]; ok {
        warnings, errs := strategy.Validate(runtimeInfo, newObj)
        allErrs = append(allErrs, errs...)
        return warnings, allErrs
    }
    return nil, allErrs
}
```

### Go Control Plane: New Constant

```go
// pkg/constants/constants.go (addition)

// RuntimeFrameworkLabel is the label on ClusterTrainingRuntime manifests
// that identifies which LLM framework the runtime belongs to.
// Existing manifests already use this label (e.g., "torchtune").
const RuntimeFrameworkLabel string = "trainer.kubeflow.org/framework"
```

### TRL Container Image

A minimal Dockerfile for the TRL trainer image:

```dockerfile
FROM python:3.11-slim

RUN pip install --no-cache-dir \
    trl>=0.15.0,<1.0.0 \
    torch>=2.5.0 \
    peft>=0.8.0

ENTRYPOINT ["trl"]
```

The image is published as `ghcr.io/kubeflow/trainer/trl-trainer` alongside the existing
`ghcr.io/kubeflow/trainer/torchtune-trainer`.

### TRL `ClusterTrainingRuntime` Manifest

Example runtime for Llama 3.2 1B SFT with TRL (modeled on the existing TorchTune runtime):

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: trl-llama3.2-1b
  labels:
    trainer.kubeflow.org/framework: trl
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      volumeClaimPolicies:
        - templates:
            - metadata:
                name: initializer
              spec:
                accessModes: ["ReadWriteOnce"]
                resources:
                  requests:
                    storage: 20Gi
      replicatedJobs:
        - name: dataset-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: dataset-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: dataset-initializer
                      image: ghcr.io/kubeflow/trainer/dataset-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://tatsu-lab/alpaca
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
        - name: model-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: model-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: model-initializer
                      image: ghcr.io/kubeflow/trainer/model-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://meta-llama/Llama-3.2-1B-Instruct
                      volumeMounts:
                        - name: initializer
                          mountPath: /workspace
        - name: node
          dependsOn:
            - name: dataset-initializer
              status: Complete
            - name: model-initializer
              status: Complete
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: ghcr.io/kubeflow/trainer/trl-trainer
                      command:
                        - trl
                      args:
                        - sft
                        - --model_name_or_path
                        - /workspace/model
                        - --dataset_name
                        - /workspace/dataset
                        - --output_dir
                        - /workspace/output
                        - --gradient_checkpointing
                        - --bf16
                      resources:
                        limits:
                          nvidia.com/gpu: 2
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
```

### SDK Usage Example

End-to-end TRL SFT fine-tuning from the Python SDK:

```python
from kubeflow.trainer import TrainerClient, types

client = TrainerClient()

client.train(
    runtime="trl-llama3.2-1b",
    initializer=types.Initializer(
        model=types.HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
        ),
        dataset=types.HuggingFaceDatasetInitializer(
            storage_uri="hf://tatsu-lab/alpaca",
        ),
    ),
    trainer=types.BuiltinTrainer(
        config=types.TRLConfig(
            trainer_type=types.TRLTrainerType.SFT,
            num_train_epochs=3,
            per_device_train_batch_size=4,
            learning_rate=2e-5,
            bf16=True,
            gradient_checkpointing=True,
            use_peft=True,
            lora_r=16,
            lora_alpha=32,
        ),
    ),
)
```

For DPO, the `trainer_type` changes and the dataset must be a preference dataset
with chosen/rejected pairs:

```python
client.train(
    runtime="trl-llama3.2-1b",
    initializer=types.Initializer(
        model=types.HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
        ),
        dataset=types.HuggingFaceDatasetInitializer(
            storage_uri="hf://argilla/ultrafeedback-binarized-preferences",
        ),
    ),
    trainer=types.BuiltinTrainer(
        config=types.TRLConfig(
            trainer_type=types.TRLTrainerType.DPO,
            learning_rate=1e-6,
            bf16=True,
        ),
    ),
)
```

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| TRL CLI changes across versions | Pin version range in requirements.txt; version compat tests |
| TRL uses accelerate, not torchrun, for distributed | TRLStrategy injects both `PET_*` and standard env vars; accelerate reads `MASTER_ADDR`, `MASTER_PORT`, `WORLD_SIZE`, `RANK`; validated in E2E |
| Multi-node TRL untested at scale | Initial implementation scoped to single-node multi-GPU; multi-node validated with dedicated E2E before GA |
| SDK type widening affects static analysis | TorchTuneConfig is a subtype of LLMBackend; passes type checks |
| Scope creep from adding backends | Scoped to TorchTune + TRL only |
| `trainer.kubeflow.org/framework` label not a Go constant | KEP adds `RuntimeFrameworkLabel` constant; existing manifests already use the label |
| KEP-285 `BaseTrainer` hierarchy merges before this KEP | `LLMBackend` is a separate ABC for config-driven trainers; if `BuiltinTrainer` is deprecated, `LLMBackend` implementations migrate to a config-driven Tier 2 trainer with minimal changes |