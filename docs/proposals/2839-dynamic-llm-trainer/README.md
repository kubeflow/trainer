# KEP-2839: Dynamic LLM Trainer Framework

|                |                                                              |
| -------------- | ------------------------------------------------------------ |
| **Authors**    | @NarayanaSabari                                              |
| **Status**     | Provisional                                                  |
| **Created**    | 2026-02-27                                                   |
| **Updated**    | 2026-03-28                                                   |
| **Reviewers**  | @tariq-hasan, @andreyvelich, @Electronic-Waste               |
| **Tracking**   | [kubeflow/trainer#2839](https://github.com/kubeflow/trainer/issues/2839) |

## Table of Contents

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Background](#background)
  - [Why This Must Change](#why-this-must-change)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Relationship to KEP-285 (Specialized Trainer Abstractions)](#relationship-to-kep-285-specialized-trainer-abstractions)
  - [The ConfigTrainer vs FuncTrainer Question](#the-configtrainer-vs-functrainer-question)
  - [Unified Type Hierarchy](#unified-type-hierarchy)
  - [Shared Design Points](#shared-design-points)
- [Current State Analysis](#current-state-analysis)
  - [SDK Coupling](#sdk-coupling)
  - [Go Control Plane: Command-Sniffing](#go-control-plane-command-sniffing)
- [High-Level Design](#high-level-design)
  - [Architecture Overview](#architecture-overview)
  - [Component Interaction Flow](#component-interaction-flow)
  - [What Changes vs What Stays](#what-changes-vs-what-stays)
- [Design Details](#design-details)
  - [Python SDK: ConfigTrainer Base Class](#python-sdk-configtrainer-base-class)
  - [Python SDK: TorchTuneTrainer (Refactored)](#python-sdk-torchtunetrainer-refactored)
  - [Python SDK: TRLTrainer](#python-sdk-trltrainer)
  - [Python SDK: TrainerClient Integration](#python-sdk-trainerclient-integration)
  - [Python SDK: Backward Compatibility](#python-sdk-backward-compatibility)
  - [Go Control Plane: FrameworkStrategy Interface](#go-control-plane-frameworkstrategy-interface)
  - [Go Control Plane: TorchTuneStrategy](#go-control-plane-torchtunestrategy)
  - [Go Control Plane: TRLStrategy](#go-control-plane-trlstrategy)
  - [Go Control Plane: Refactored Torch Plugin Dispatch](#go-control-plane-refactored-torch-plugin-dispatch)
  - [Go Control Plane: New Constant](#go-control-plane-new-constant)
  - [TRL Container Image](#trl-container-image)
  - [TRL ClusterTrainingRuntime Manifests](#trl-clustertrainingruntime-manifests)
- [User-Facing API Examples](#user-facing-api-examples)
  - [TRL SFT Fine-Tuning](#trl-sft-fine-tuning)
  - [TRL DPO Alignment](#trl-dpo-alignment)
  - [TorchTune (Backward Compatible)](#torchtune-backward-compatible)
  - [Backward Compatible: BuiltinTrainer Still Works](#backward-compatible-builtintrainer-still-works)
- [Implementation Plan](#implementation-plan)
- [Test Plan](#test-plan)
- [Risks and Mitigations](#risks-and-mitigations)
- [Implementation History](#implementation-history)
<!-- /toc -->

---

## Summary

This KEP introduces a **pluggable config-driven trainer framework** for LLM fine-tuning
in Kubeflow Trainer. It decouples the SDK and Go control plane from TorchTune by
introducing:

1. A `ConfigTrainer` base class in the Python SDK that sits within
   [KEP-285](https://github.com/kubeflow/sdk/pull/308)'s `BaseTrainer` hierarchy as
   the foundation for all **config-driven trainers** (where the framework's own CLI is
   the entrypoint, not a user-supplied Python function).

2. A `FrameworkStrategy` interface in the Go Torch plugin that replaces hardcoded
   command-sniffing with label-based dispatch via `trainer.kubeflow.org/framework`.

3. **TRL** as the first new backend with SFT and DPO support, alongside TorchTune
   refactored as a backward-compatible implementation.

This builds on [KEP-2401](../2401-llm-trainer-v2/README.md), the community consensus on
"Plan 3" in [#2752](https://github.com/kubeflow/trainer/issues/2752), and aligns with
the `BaseTrainer` hierarchy being designed in
[KEP-285](https://github.com/kubeflow/sdk/pull/308).

---

## Motivation

### Background

Kubeflow Trainer V2 introduced LLM fine-tuning support through
[KEP-2401](../2401-llm-trainer-v2/README.md), using TorchTune as the backend. The
implementation was successful for its scope, but the architecture hardcodes TorchTune
at two coupling points:

- **SDK**: `BuiltinTrainer.config` is typed as `TorchTuneConfig` with no abstraction.
- **Go Torch plugin**: `EnforceMLPolicy()` uses command-sniffing
  (`slices.Equal(trainJob.Spec.Trainer.Command, constants.TorchTuneEntrypoint)`) to
  decide between the torchrun path and the TorchTune path.

### Why This Must Change

- **TorchTune stopped adding features** in July 2025
  ([pytorch/torchtune#2883](https://github.com/pytorch/torchtune/issues/2883)). New
  models and post-training methods (DPO, PPO, ORPO) will not be supported.
- **The command-sniffing pattern doesn't scale.** Each new backend would require
  another `slices.Equal` check, another branch in `EnforceMLPolicy`, and another
  branch in `Validate`.
- **Community consensus on Plan 3** (pluggable framework) from
  [#2752](https://github.com/kubeflow/trainer/issues/2752) was unanimous.
- **TRL is actively maintained** by Hugging Face with native CLI support
  (`trl sft`, `trl dpo`, etc.) and built-in accelerate integration for multi-GPU and
  multi-node training.
- **KEP-285 is actively designing** the `BaseTrainer` hierarchy and the maintainers
  are [asking exactly how config-driven trainers fit in](https://github.com/kubeflow/sdk/pull/308#discussion_r2912976804).
  This KEP provides that answer.

---

## Goals

1. Define a `ConfigTrainer` base class within KEP-285's `BaseTrainer` hierarchy for
   config-driven LLM trainers.
2. Refactor `TorchTuneConfig` into `TorchTuneTrainer` implementing `ConfigTrainer`
   with zero breaking changes to existing workflows.
3. Implement `TRLTrainer` supporting SFT and DPO training algorithms.
4. Create TRL container image and `ClusterTrainingRuntime` manifests.
5. Generalize the Go Torch plugin to dispatch via `FrameworkStrategy` instead of
   hardcoded command-sniffing.
6. Maintain full backward compatibility with existing `BuiltinTrainer` API.

## Non-Goals

1. Unsloth, LlamaFactory, or other backends (future work following the same pattern).
2. CRD schema changes -- operates within existing `.spec.trainer.command`/`.spec.trainer.args`.
3. New Kubernetes resource topologies (e.g., launcher/worker patterns).
4. Deprecating `BuiltinTrainer` or `CustomTrainer` (both remain supported).
5. Implementing function-based trainers (that is KEP-285's Tier 1 scope).

---

## Relationship to KEP-285 (Specialized Trainer Abstractions)

[KEP-285](https://github.com/kubeflow/sdk/pull/308) introduces a `BaseTrainer` ABC
with framework-specific Tier 1 trainers (`TorchTrainer`, `JAXTrainer`, etc.) and
community-contributed Tier 2 extensions. This KEP is designed to integrate directly
into that hierarchy.

### The ConfigTrainer vs FuncTrainer Question

In the KEP-285 review, @andreyvelich
[asked](https://github.com/kubeflow/sdk/pull/308#discussion_r2912976804):

> "How are we going to refactor the BuiltinTrainer interface once we implement the
> BaseTrainer? And how can we dynamically register new LLM fine-tuning framework
> backends?"

And @tariq-hasan
[asked](https://github.com/kubeflow/sdk/pull/308#discussion_r2901688930):

> "How do we handle config-driven trainers for post-training LLM fine-tuning? Do we
> segregate them outside BaseTrainer scope?"

And @szaher
[proposed](https://github.com/kubeflow/sdk/pull/308#discussion_r2955718123):

> "Should I rename the proposal to have two main abstract classes `ConfigTrainer` and
> `FuncTrainer`?"

This KEP answers these questions. There are two fundamentally different trainer
patterns in Kubeflow:

| Pattern | Entrypoint | Examples | KEP |
|---------|-----------|----------|-----|
| **Function-based** (`FuncTrainer`) | User's Python `train()` function | TorchTrainer, JAXTrainer | KEP-285 Tier 1 |
| **Config-driven** (`ConfigTrainer`) | Framework's own CLI | TorchTune, TRL, Unsloth | This KEP (Tier 2) |

### Unified Type Hierarchy

```
                         BaseTrainer (ABC)          ← KEP-285
                         ├── get_train_func()
                         ├── get_framework_args()
                         ├── validate_runtime()
                         └── supported_frameworks
                              │
             ┌────────────────┼────────────────┐
             │                │                │
       TorchTrainer      JAXTrainer      ConfigTrainer (ABC)   ← This KEP
       (Tier 1)          (Tier 1)        ├── command
                                         ├── to_args()
                                         └── validate()
                                              │
                              ┌───────────────┼───────────────┐
                              │               │               │
                        TorchTuneTrainer  TRLTrainer     (future: Unsloth,
                        (Tier 2)          (Tier 2)        LlamaFactory)


       Existing (unchanged, backward compatible):

       CustomTrainer          BuiltinTrainer         CustomTrainerContainer
       (flat dataclass)       (config: ConfigTrainer)  (image-based)
```

`ConfigTrainer` extends `BaseTrainer` by adding:
- A `command` class variable (the CLI entrypoint, e.g., `("trl",)` or `("tune", "run")`)
- A `to_args()` method that translates config into CLI arguments
- A `validate()` method for config-level validation

`ConfigTrainer.get_train_func()` returns `None` (there is no user function -- the
framework's CLI **is** the entrypoint). `ConfigTrainer.get_framework_args()` delegates
to `to_args()`.

### Shared Design Points

- Both KEPs use `trainer.kubeflow.org/framework` as the dispatch key. KEP-285 uses it
  for SDK runtime auto-discovery; this KEP uses it for Go strategy dispatch.
- Both KEPs are compatible with either keeping or deprecating `BuiltinTrainer`.
- If the framework label is
  [promoted to a Runtime API spec field](https://github.com/kubeflow/sdk/pull/308#discussion_r2894627115)
  (as discussed in KEP-285), both KEPs benefit with no changes.

---

## Current State Analysis

### SDK Coupling

In the Python SDK, `BuiltinTrainer` has a single field
([types.py:226-237](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/types/types.py#L226)):

```python
@dataclass
class BuiltinTrainer:
    config: TorchTuneConfig  # No other option
```

The comment at line 240 explicitly signals readiness for change:
```python
# Change it to list: BUILTIN_CONFIGS, once we support more Builtin Trainer configs.
```

The `KubernetesBackend` calls `get_args_using_torchtune_config()`
([utils.py:467-521](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/backends/kubernetes/utils.py#L467))
with no abstraction -- adding a new backend means modifying this function and the
type annotation.

### Go Control Plane: Command-Sniffing

The Torch plugin
([torch.go:149](https://github.com/kubeflow/trainer/blob/master/pkg/runtime/framework/plugins/torch/torch.go#L149))
uses command-sniffing to branch:

```go
if !slices.Equal(trainJob.Spec.Trainer.Command, constants.TorchTuneEntrypoint) {
    // Standard torchrun path
} else {
    // TorchTune path: recipe selection, config overrides, rdzv_endpoint
}
```

This pattern requires a new `slices.Equal` check for every new backend. The
validation path ([torch.go:88](https://github.com/kubeflow/trainer/blob/master/pkg/runtime/framework/plugins/torch/torch.go#L88))
similarly sniffs the entrypoint to decide whether to run `validateTorchTune()`.

---

## High-Level Design

### Architecture Overview

The change is a **localized refactor** of two coupling points. No new CRDs, no new
controllers, no changes to the plugin framework itself.

```
                      BEFORE                              AFTER
                 ┌──────────────┐                  ┌──────────────┐
  SDK            │BuiltinTrainer│                  │BuiltinTrainer│
                 │ config:      │                  │ config:      │
                 │  TorchTune   │                  │  Config      │
                 │  Config      │                  │  Trainer     │
                 └──────┬───────┘                  └──────┬───────┘
                         │                                 │
                         │ hardcoded                        │ config.command
                         │ get_args_using_                  │ config.to_args()
                         │ torchtune_config()               │
                         ▼                                 ▼
                  creates TrainJob CR               creates TrainJob CR
                         │                                 │
  ┌────────────────────────────────────────────────────────────────────┐
  │                       Kubernetes API                               │
  └──────────────────────────┬─────────────────────────────────────────┘
                             │
  Go                         ▼
  Torch        ┌─────────────────────────────┐
  Plugin       │ EnforceMLPolicy()            │
               │                              │
   BEFORE:     │ if cmd == ["tune","run"]:    │
               │   → TorchTune branch         │
               │ else:                        │
               │   → torchrun branch          │
               │                              │
   AFTER:      │ label = info.Labels          │
               │   [framework]                │
               │ if strategy = backends       │
               │   [label]:                   │
               │   → strategy.Enforce()       │
               │ else:                        │
               │   → default torchrun         │
               └─────────────────────────────┘
```

### Component Interaction Flow

End-to-end for a TRL SFT job:

```
1. User: TrainerClient.train(
       trainer=TRLTrainer(trainer_type=SFT, ...),
       runtime="trl-llama3.2-1b")

   -- OR with auto-discovery --

   User: TrainerClient.train(
       trainer=TRLTrainer(trainer_type=SFT, ...))
       # SDK finds runtime with label trainer.kubeflow.org/framework: trl

2. SDK:  TRLTrainer.validate() → ok
         TRLTrainer.command   → ("trl",)
         TRLTrainer.to_args() → ["sft", "--model_name_or_path", ...]
         Build TrainJob CR with:
           runtimeRef: { name: "trl-llama3.2-1b" }
           trainer: { command: ["trl"], args: ["sft", ...] }

3. K8s:  Webhook validates TrainJob
         Torch plugin Validate() → label=trl → TRLStrategy.Validate()

4. Go:   TrainJob controller reconciles:
         Torch EnforceMLPolicy():
           a) Common: set PET_NNODES, PET_NPROC_PER_NODE, PET_NODE_RANK
           b) Label "trl" → TRLStrategy.EnforceCommand():
              inject PET_MASTER_ADDR, PET_MASTER_PORT
              inject MASTER_ADDR, MASTER_PORT, WORLD_SIZE, RANK
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
| SDK `BuiltinTrainer` | **Widen** | `TorchTuneConfig` → `ConfigTrainer` |
| SDK `TorchTuneConfig` | **Refactor** | → `TorchTuneTrainer(ConfigTrainer)`, backward compatible |
| SDK `TRLTrainer` | **New** | New config-driven trainer |
| Container images | **New** | `trl-trainer` image |
| ClusterTrainingRuntimes | **New** | TRL-specific runtime manifests |

---

## Design Details

### Python SDK: ConfigTrainer Base Class

`ConfigTrainer` extends KEP-285's `BaseTrainer` for config-driven trainers where the
framework's own CLI is the entrypoint. It bridges the gap between function-based
Tier 1 trainers and the existing `BuiltinTrainer`.

```python
from abc import abstractmethod
from dataclasses import dataclass
from typing import Callable, ClassVar, Optional


@dataclass
class ConfigTrainer(BaseTrainer):
    """Base class for config-driven LLM training backends.

    Config-driven trainers use the framework's own CLI as the entrypoint
    (e.g., `trl sft ...`, `tune run ...`) rather than a user-supplied
    Python function. Each implementation translates its config into a
    (command, args) pair that the Kubernetes backend writes into the
    TrainJob CR.

    This class sits in KEP-285's BaseTrainer hierarchy as the foundation
    for Tier 2 config-driven trainers.
    """

    # Subclasses set this to their CLI entrypoint.
    # e.g., ("tune", "run") for TorchTune, ("trl",) for TRL.
    command: ClassVar[tuple[str, ...]]

    # Common fields shared by all config-driven trainers.
    num_nodes: Optional[int] = None
    resources_per_node: Optional[dict] = None

    @abstractmethod
    def to_args(self, initializer: Optional["Initializer"] = None) -> list[str]:
        """Return CLI arguments for the entrypoint."""
        ...

    @abstractmethod
    def validate(self) -> None:
        """Raise ValueError if the config is invalid."""
        ...

    # --- BaseTrainer interface implementation ---

    def get_train_func(self) -> Optional[Callable]:
        """Config-driven trainers have no user function."""
        return None

    def get_train_func_args(self) -> Optional[dict]:
        """Config-driven trainers have no function args."""
        return None

    def get_framework_args(self) -> dict:
        """Delegate to to_args() for CLI argument generation."""
        return {"_config_args": self.to_args()}
```

### Python SDK: TorchTuneTrainer (Refactored)

`TorchTuneConfig` is refactored into `TorchTuneTrainer` implementing `ConfigTrainer`.
All existing fields are preserved. `TorchTuneConfig` becomes a type alias for backward
compatibility.

```python
@dataclass
class TorchTuneTrainer(ConfigTrainer):
    """TorchTune LLM Trainer configuration.

    Supports runtimes labeled with trainer.kubeflow.org/framework: torchtune.
    """

    supported_frameworks: ClassVar[list[str]] = ["torchtune"]
    command: ClassVar[tuple[str, ...]] = ("tune", "run")

    # All existing TorchTuneConfig fields preserved.
    dtype: Optional[DataType] = None
    batch_size: Optional[int] = None
    epochs: Optional[int] = None
    loss: Optional[Loss] = None
    peft_config: Optional[LoraConfig] = None
    dataset_preprocess_config: Optional[TorchTuneInstructDataset] = None

    def to_args(self, initializer=None) -> list[str]:
        # Existing get_args_using_torchtune_config() logic moves here.
        ...

    def validate(self) -> None:
        # Validate supported model, LoRA config, etc.
        ...


# Backward compatibility alias.
TorchTuneConfig = TorchTuneTrainer
```

### Python SDK: TRLTrainer

```python
from enum import Enum


class TRLTrainerType(Enum):
    """Training algorithms available via the TRL CLI."""
    SFT = "sft"
    DPO = "dpo"
    KTO = "kto"
    GRPO = "grpo"


@dataclass
class TRLTrainer(ConfigTrainer):
    """TRL LLM Trainer configuration.

    Supports runtimes labeled with trainer.kubeflow.org/framework: trl.
    TRL is maintained by Hugging Face with native CLI support and built-in
    accelerate integration for multi-GPU/multi-node training.

    Args:
        trainer_type: Training algorithm (SFT, DPO, KTO, GRPO).
        model_name_or_path: Hugging Face model ID or local path.
        dataset_name: Hugging Face dataset ID or local path.
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

    supported_frameworks: ClassVar[list[str]] = ["trl"]
    command: ClassVar[tuple[str, ...]] = ("trl",)

    trainer_type: TRLTrainerType = TRLTrainerType.SFT
    model_name_or_path: Optional[str] = None
    dataset_name: Optional[str] = None
    learning_rate: Optional[float] = None
    num_train_epochs: Optional[int] = None
    per_device_train_batch_size: Optional[int] = None
    gradient_checkpointing: bool = True
    bf16: bool = True
    use_peft: bool = False
    lora_r: Optional[int] = None
    lora_alpha: Optional[int] = None
    lora_target_modules: Optional[str] = None
    extra_args: Optional[dict[str, str]] = None

    def to_args(self, initializer=None) -> list[str]:
        args = [self.trainer_type.value]  # subcommand: "sft", "dpo", etc.

        # Model path: prefer initializer workspace, fall back to config.
        model_path = self.model_name_or_path
        if initializer and initializer.model:
            model_path = "/workspace/model"
        if model_path:
            args.extend(["--model_name_or_path", model_path])

        # Dataset: prefer initializer workspace, fall back to config.
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
            args.extend(["--per_device_train_batch_size",
                         str(self.per_device_train_batch_size)])
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

        # Pass-through extra args.
        if self.extra_args:
            for k, v in self.extra_args.items():
                args.extend([f"--{k}", v])

        return args

    def validate(self) -> None:
        if self.use_peft and self.lora_r is None:
            raise ValueError("lora_r is required when use_peft=True")
```

### Python SDK: TrainerClient Integration

The `TrainerClient.train()` method gains support for `ConfigTrainer` through KEP-285's
`BaseTrainer` interface. When a `ConfigTrainer` is passed:

1. If `runtime` is `None`, the SDK auto-discovers a runtime by matching the
   `trainer.kubeflow.org/framework` label against `supported_frameworks` (using
   KEP-285's `_resolve_runtime()` mechanism).
2. `validate_runtime()` ensures the runtime's framework label matches.
3. The backend uses `config.command` and `config.to_args()` to build the TrainJob CR.

```python
# In KubernetesBackend — unified handler for ConfigTrainer.

def get_trainer_cr(
    runtime: types.Runtime,
    trainer: ConfigTrainer,
    initializer: Optional[types.Initializer] = None,
) -> models.TrainerV1alpha1Trainer:
    trainer.validate()

    trainer_cr = models.TrainerV1alpha1Trainer()
    if trainer.num_nodes:
        trainer_cr.num_nodes = trainer.num_nodes
    if trainer.resources_per_node:
        trainer_cr.resources_per_node = get_resources_per_node(
            trainer.resources_per_node
        )

    trainer_cr.command = list(trainer.command)
    trainer_cr.args = trainer.to_args(initializer)
    return trainer_cr
```

### Python SDK: Backward Compatibility

| Existing API | Status | Details |
|-------------|--------|---------|
| `BuiltinTrainer(config=TorchTuneConfig(...))` | **Works** | `TorchTuneConfig` is an alias for `TorchTuneTrainer` |
| `BuiltinTrainer(config=TRLTrainer(...))` | **New** | `BuiltinTrainer.config` type widens to `ConfigTrainer` |
| `CustomTrainer(func=...)` | **Unchanged** | No modifications |
| `CustomTrainerContainer(image=...)` | **Unchanged** | No modifications |
| `TrainerClient.train(trainer=TRLTrainer(...))` | **New** | Direct `BaseTrainer` subclass via KEP-285 |

The `BuiltinTrainer.config` field type changes from `TorchTuneConfig` to
`ConfigTrainer`. Since `TorchTuneConfig` is a type alias for `TorchTuneTrainer`
which extends `ConfigTrainer`, all existing code continues to work.

### Go Control Plane: FrameworkStrategy Interface

Inside the Torch plugin package, a strategy interface replaces the inline if/else.
The naming follows the existing `trainer.kubeflow.org/framework` label convention.

```go
// pkg/runtime/framework/plugins/torch/strategy.go

package torch

import (
    "k8s.io/apimachinery/pkg/util/validation/field"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

    trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
    "github.com/kubeflow/trainer/v2/pkg/runtime"
)

// FrameworkStrategy defines backend-specific behavior for the Torch plugin.
// Each strategy handles the portion of EnforceMLPolicy and Validate that
// differs between frameworks (e.g., command mutation, env var injection,
// validation rules).
type FrameworkStrategy interface {
    // EnforceCommand mutates the trainer container's command, args, and
    // env vars with framework-specific values.
    EnforceCommand(
        info *runtime.Info,
        trainJob *trainer.TrainJob,
        container *runtime.Container,
    ) error

    // Validate performs framework-specific validation on the TrainJob.
    Validate(
        runtimeInfo *runtime.Info,
        trainJob *trainer.TrainJob,
    ) (admission.Warnings, field.ErrorList)
}
```

### Go Control Plane: TorchTuneStrategy

Extracts the existing inline code from
[torch.go:149-183](https://github.com/kubeflow/trainer/blob/master/pkg/runtime/framework/plugins/torch/torch.go#L149)
and the validation from
[torchtune.go](https://github.com/kubeflow/trainer/blob/master/pkg/runtime/framework/plugins/torch/torchtune.go):

```go
// pkg/runtime/framework/plugins/torch/torchtune_strategy.go

type TorchTuneStrategy struct{}

func (s *TorchTuneStrategy) EnforceCommand(
    info *runtime.Info,
    trainJob *trainer.TrainJob,
    container *runtime.Container,
) error {
    // Moved from torch.go:149-183 (unchanged logic):
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
    // Delegates to existing validateTorchTune() (torchtune.go:35-74).
    return validateTorchTune(runtimeInfo, trainJob)
}
```

### Go Control Plane: TRLStrategy

TRL uses Hugging Face's `accelerate` for distributed training, which reads standard
environment variables (`MASTER_ADDR`, `MASTER_PORT`, `WORLD_SIZE`, `RANK`) rather
than the `PET_*` variants used by torchrun. The strategy injects both sets for
maximum compatibility.

```go
// pkg/runtime/framework/plugins/torch/trl_strategy.go

type TRLStrategy struct{}

func (s *TRLStrategy) EnforceCommand(
    info *runtime.Info,
    trainJob *trainer.TrainJob,
    container *runtime.Container,
) error {
    trainerPS := info.FindPodSetByAncestor(constants.AncestorTrainer)
    numNodes := ptr.Deref(
        ptr.Deref(trainerPS, runtime.PodSet{}).Count, 1,
    )
    masterAddr := fmt.Sprintf(
        "%s-%s-0-0.%s",
        trainJob.Name, constants.Node, trainJob.Name,
    )
    masterPort := fmt.Sprintf("%d", constants.ContainerTrainerPort)
    worldSize := fmt.Sprintf("%d", numNodes*numProcPerNode)

    // Inject both PET_* (torchrun compat) and standard env vars
    // (accelerate/TRL).
    apply.UpsertEnvVars(&container.Env,
        *corev1ac.EnvVar().
            WithName(constants.TorchEnvMasterAddr).
            WithValue(masterAddr),
        *corev1ac.EnvVar().
            WithName(constants.TorchEnvMasterPort).
            WithValue(masterPort),
        *corev1ac.EnvVar().
            WithName("MASTER_ADDR").WithValue(masterAddr),
        *corev1ac.EnvVar().
            WithName("MASTER_PORT").WithValue(masterPort),
        *corev1ac.EnvVar().
            WithName("WORLD_SIZE").WithValue(worldSize),
        *corev1ac.EnvVar().WithName("RANK").WithValueFrom(
            corev1ac.EnvVarSource().WithFieldRef(
                corev1ac.ObjectFieldSelector().WithFieldPath(
                    constants.JobCompletionIndexFieldPath,
                ),
            ),
        ),
    )
    return nil
}

func (s *TRLStrategy) Validate(
    runtimeInfo *runtime.Info,
    trainJob *trainer.TrainJob,
) (admission.Warnings, field.ErrorList) {
    // TRL validation is minimal -- config is fully constructed by the SDK.
    return nil, nil
}
```

### Go Control Plane: Refactored Torch Plugin Dispatch

The `Torch` struct gains a `strategies` map, and `EnforceMLPolicy` dispatches by
the `trainer.kubeflow.org/framework` label:

```go
// pkg/runtime/framework/plugins/torch/torch.go (modified)

type Torch struct {
    strategies map[string]FrameworkStrategy
}

func New(
    ctx context.Context,
    c client.Client,
    fi client.FieldIndexer,
) (framework.Plugin, error) {
    return &Torch{
        strategies: map[string]FrameworkStrategy{
            "torchtune": &TorchTuneStrategy{},
            "trl":       &TRLStrategy{},
        },
    }, nil
}
```

The dispatch logic in `EnforceMLPolicy` changes from command-sniffing to label
lookup:

```go
func (t *Torch) EnforceMLPolicy(
    info *runtime.Info,
    trainJob *trainer.TrainJob,
) error {
    // ... (existing common logic: numNodes, numProcPerNode, PET_NNODES,
    //       PET_NPROC_PER_NODE, PET_NODE_RANK — unchanged) ...

    // Label-based dispatch replaces command-sniffing.
    fw := info.Labels[constants.FrameworkLabel]
    if strategy, ok := t.strategies[fw]; ok {
        return strategy.EnforceCommand(info, trainJob, trainerContainer)
    }

    // Default: standard torchrun path (PET_MASTER_ADDR, PET_MASTER_PORT).
    apply.UpsertEnvVars(&trainerContainer.Env,
        *corev1ac.EnvVar().
            WithName(constants.TorchEnvMasterAddr).WithValue(masterAddr),
        *corev1ac.EnvVar().
            WithName(constants.TorchEnvMasterPort).WithValue(masterPort),
    )
    return nil
}
```

The same pattern applies to `Validate`:

```go
func (t *Torch) Validate(
    ctx context.Context,
    runtimeInfo *runtime.Info,
    _, newObj *trainer.TrainJob,
) (admission.Warnings, field.ErrorList) {
    // ... (existing common validation: numProcPerNode, reserved envs) ...

    fw := runtimeInfo.Labels[constants.FrameworkLabel]
    if strategy, ok := t.strategies[fw]; ok {
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

// FrameworkLabel is the label on ClusterTrainingRuntime manifests that
// identifies which framework the runtime belongs to.
// Existing manifests already use this label (e.g., "torchtune", "torch",
// "deepspeed", "jax", "mlx", "xgboost").
const FrameworkLabel string = "trainer.kubeflow.org/framework"
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

Published as `ghcr.io/kubeflow/trainer/trl-trainer` alongside the existing
`ghcr.io/kubeflow/trainer/torchtune-trainer`.

### TRL ClusterTrainingRuntime Manifests

Example runtime for Llama 3.2 1B SFT with TRL:

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

---

## User-Facing API Examples

### TRL SFT Fine-Tuning

Using KEP-285's `BaseTrainer` interface directly:

```python
from kubeflow.trainer import TrainerClient, TRLTrainer, TRLTrainerType, RuntimeConfig
from kubeflow.trainer.types import Initializer, HuggingFaceModelInitializer, HuggingFaceDatasetInitializer

client = TrainerClient()

# Runtime auto-discovered via trainer.kubeflow.org/framework: trl
client.train(
    initializer=Initializer(
        model=HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
        ),
        dataset=HuggingFaceDatasetInitializer(
            storage_uri="hf://tatsu-lab/alpaca",
        ),
    ),
    trainer=TRLTrainer(
        trainer_type=TRLTrainerType.SFT,
        num_train_epochs=3,
        per_device_train_batch_size=4,
        learning_rate=2e-5,
        bf16=True,
        gradient_checkpointing=True,
        use_peft=True,
        lora_r=16,
        lora_alpha=32,
    ),
    runtime_config=RuntimeConfig(
        packages=["flash-attn"],
    ),
)
```

### TRL DPO Alignment

```python
client.train(
    initializer=Initializer(
        model=HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
        ),
        dataset=HuggingFaceDatasetInitializer(
            storage_uri="hf://argilla/ultrafeedback-binarized-preferences",
        ),
    ),
    trainer=TRLTrainer(
        trainer_type=TRLTrainerType.DPO,
        learning_rate=1e-6,
        bf16=True,
    ),
)
```

### TorchTune (Backward Compatible)

Existing TorchTune code continues to work unchanged:

```python
client.train(
    runtime="torch-llama3.2-1b",
    initializer=Initializer(
        model=HuggingFaceModelInitializer(
            storage_uri="hf://meta-llama/Llama-3.2-1B-Instruct",
        ),
        dataset=HuggingFaceDatasetInitializer(
            storage_uri="hf://tatsu-lab/alpaca",
        ),
    ),
    trainer=TorchTuneTrainer(
        epochs=3,
        batch_size=4,
        peft_config=LoraConfig(lora_rank=16, lora_alpha=32),
    ),
)
```

### Backward Compatible: BuiltinTrainer Still Works

```python
# This existing code continues to work with no changes.
client.train(
    runtime="torch-llama3.2-1b",
    initializer=Initializer(...),
    trainer=BuiltinTrainer(
        config=TorchTuneConfig(
            epochs=3,
            batch_size=4,
        ),
    ),
)
```

---

## Implementation Plan

This proposal is scoped for 350 hours (GSoC Large) and can be implemented in phases:

**Phase 1: SDK Foundation (Weeks 1-4)**
- Add `ConfigTrainer` base class to `kubeflow/sdk`
- Refactor `TorchTuneConfig` → `TorchTuneTrainer(ConfigTrainer)` with alias
- Update `KubernetesBackend` to use `ConfigTrainer` interface
- Update `BuiltinTrainer.config` type to `ConfigTrainer`
- Unit tests for backward compatibility
- Coordinate with KEP-285 on `BaseTrainer` integration

**Phase 2: Go Control Plane Refactor (Weeks 5-8)**
- Add `FrameworkLabel` constant to `pkg/constants/constants.go`
- Implement `FrameworkStrategy` interface
- Extract `TorchTuneStrategy` from existing inline code
- Refactor Torch plugin dispatch from command-sniffing to label lookup
- Unit tests for strategy dispatch and TorchTune regression
- Integration tests

**Phase 3: TRL Backend (Weeks 9-14)**
- Implement `TRLTrainer` in SDK
- Implement `TRLStrategy` in Go Torch plugin
- Build TRL container image (`cmd/trainers/trl/`)
- Create TRL `ClusterTrainingRuntime` manifests
- E2E tests for TRL SFT on GPU
- Documentation and examples

**Phase 4: Polish and DPO (Weeks 15-18)**
- Add DPO support and E2E tests
- Helm chart additions for TRL runtimes
- SDK documentation on sdk.kubeflow.org
- TorchTune regression E2E validation

---

## Test Plan

### Unit Tests (SDK)

- `ConfigTrainer` interface compliance for `TorchTuneTrainer` and `TRLTrainer`
- `TorchTuneConfig` alias backward compatibility
- `TRLTrainer.to_args()` produces correct CLI arguments for SFT and DPO
- `TRLTrainer.validate()` catches invalid configs (e.g., `use_peft=True` without `lora_r`)
- `BuiltinTrainer(config=TRLTrainer(...))` constructs correctly
- Runtime auto-discovery for `supported_frameworks=["trl"]`

### Unit Tests (Go)

- `FrameworkStrategy` dispatch: label `"torchtune"` → `TorchTuneStrategy`
- `FrameworkStrategy` dispatch: label `"trl"` → `TRLStrategy`
- `FrameworkStrategy` dispatch: label `"torch"` → default torchrun path
- `TorchTuneStrategy.EnforceCommand()` produces same output as current inline code
- `TRLStrategy.EnforceCommand()` injects correct env vars (`MASTER_ADDR`, `WORLD_SIZE`, etc.)
- `TRLStrategy.Validate()` passes for valid TRL configs

### Integration Tests

- TRL TrainJob reconciliation with `ClusterTrainingRuntime` labeled `trl`
- TorchTune regression: existing TorchTune workflows produce identical TrainJobs

### E2E Tests

- TRL SFT fine-tuning on GPU
- TRL DPO alignment on GPU
- TorchTune regression on GPU (existing tests)

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| TRL CLI changes across versions | Pin version range in container image; version compat tests |
| TRL uses accelerate, not torchrun, for distributed | TRLStrategy injects both `PET_*` and standard env vars; validated in E2E |
| Multi-node TRL untested at scale | Initial scope: single-node multi-GPU; multi-node validated before GA |
| SDK type widening breaks static analysis | `TorchTuneConfig` alias ensures existing type checks pass |
| KEP-285 design changes before this KEP lands | `ConfigTrainer` is designed to adapt to either `BaseTrainer` integration or standalone use |
| Scope creep from adding backends | Scoped to TorchTune + TRL only; other backends follow the same pattern |
| `trainer.kubeflow.org/framework` label not a Go constant | KEP adds `FrameworkLabel` constant; existing manifests already use the label |
