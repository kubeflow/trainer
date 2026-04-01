# KEP-2839: Dynamic LLM Trainer Framework

|                |                                                              |
| -------------- | ------------------------------------------------------------ |
| **Authors**    | @NarayanaSabari                                              |
| **Status**     | Provisional                                                  |
| **Created**    | 2026-02-27                                                   |
| **Updated**    | 2026-03-31                                                   |
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
  - [Two Fundamentally Different Trainer Patterns](#two-fundamentally-different-trainer-patterns)
  - [Why Separate ABCs Instead of a Unified Hierarchy](#why-separate-abcs-instead-of-a-unified-hierarchy)
  - [Unified API Entry Point](#unified-api-entry-point)
  - [Shared Design Points](#shared-design-points)
- [Current State Analysis](#current-state-analysis)
  - [SDK Coupling](#sdk-coupling)
  - [Go Control Plane: Command-Sniffing](#go-control-plane-command-sniffing)
- [High-Level Design](#high-level-design)
  - [Architecture Overview](#architecture-overview)
  - [Component Interaction Flow](#component-interaction-flow)
  - [What Changes vs What Stays](#what-changes-vs-what-stays)
- [Design Details](#design-details)
  - [Python SDK: LLMTrainer Base Class](#python-sdk-configtrainer-base-class)
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
- [Alternatives Considered](#alternatives-considered)
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

1. A `LLMTrainer` ABC in the Python SDK — a **separate abstraction** from KEP-285's
   `BaseTrainer`, purpose-built for **config-driven trainers** where the framework's
   own CLI is the entrypoint (e.g., `trl sft ...`, `tune run ...`). Both ABCs are
   accepted through the same `TrainerClient.train(trainer=...)` parameter, giving
   data scientists a flat, unified API.

2. A `FrameworkStrategy` interface in the Go Torch plugin that replaces hardcoded
   command-sniffing with label-based dispatch via `trainer.kubeflow.org/framework`.

3. **TRL** as the first new backend with SFT and DPO support, alongside TorchTune
   refactored as a backward-compatible implementation.

This builds on [KEP-2401](../2401-llm-trainer-v2/README.md), the community consensus on
"Plan 3" in [#2752](https://github.com/kubeflow/trainer/issues/2752), and is designed to
complement [KEP-285](https://github.com/kubeflow/sdk/pull/308)'s function-based trainer
hierarchy.

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

### User Stories

**"I want to do DPO alignment, but Kubeflow only supports SFT via TorchTune."**

A data scientist wants to align a model using preference data (DPO). TorchTune
doesn't support DPO, and there's no way to plug in TRL without modifying the SDK
source code. They fall back to raw YAML or leave Kubeflow entirely.

**"I want to use a newer model that TorchTune doesn't have recipes for."**

TorchTune supports 4 models (Llama 3.2 1B/3B, Llama 3.3 70B, Qwen 2.5 1.5B).
When a user tries a model outside this list, the Go validation rejects it. TRL
works with any Hugging Face model out of the box.

**"I want to switch from TorchTune to TRL without relearning the SDK."**

A team that started with TorchTune wants to migrate to TRL for its active
development and broader algorithm support. Today this requires understanding
`BuiltinTrainer` internals. With this KEP, it's a one-line change:
`TorchTuneTrainer(...)` → `TRLTrainer(...)`.

### Why TRL as the First New Backend

| | TorchTune | TRL |
|--|-----------|-----|
| **Status** | Maintenance mode (July 2025) | Actively maintained by Hugging Face |
| **Algorithms** | SFT only | SFT, DPO, KTO, GRPO, PPO, RLOO |
| **Models** | 4 hardcoded models | Any Hugging Face model |
| **CLI** | `tune run <recipe>` | `trl sft \| dpo \| kto \| grpo` |
| **Distributed** | torchrun | accelerate (+ torchrun compat) |
| **PEFT** | Built-in LoRA/QLoRA/DoRA | Via `peft` library (LoRA/QLoRA) |
| **Community** | ~12k GitHub stars | ~13k GitHub stars, 250+ contributors |

TRL is the most requested alternative in
[#2839](https://github.com/kubeflow/trainer/issues/2839) and aligns with the
Hugging Face ecosystem that most Kubeflow users already use for models and datasets.

---

## Goals

1. Define a `LLMTrainer` ABC in the Python SDK as a separate abstraction for
   config-driven LLM trainers, complementing KEP-285's function-based `BaseTrainer`.
2. Refactor `TorchTuneConfig` into `TorchTuneTrainer` implementing `LLMTrainer`
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
5. Implementing function-based trainers (that is KEP-285's scope).

---

## Relationship to KEP-285 (Specialized Trainer Abstractions)

[KEP-285](https://github.com/kubeflow/sdk/pull/308) introduces a `BaseTrainer` ABC
with framework-specific trainers (`TorchTrainer`, `JAXTrainer`, etc.) for
**function-based** training — where the user passes a Python `train()` function.
This KEP addresses **config-driven** training — where the framework's own CLI is the
entrypoint.

### Two Fundamentally Different Trainer Patterns

| Pattern | Entrypoint | SDK Responsibility | Examples |
|---------|-----------|-------------------|----------|
| **Function-based** (KEP-285) | User's Python `train()` function | Package user code into a container | TorchTrainer, JAXTrainer |
| **Config-driven** (This KEP) | Framework's own CLI binary | Translate config fields into CLI args | TorchTuneTrainer, TRLTrainer |

These are architecturally distinct:
- Function-based trainers need `get_train_func()`, `get_train_func_args()`,
  `packages_to_install` — concepts that don't apply to config-driven trainers.
- Config-driven trainers need `command`, `to_args()`, framework-specific validation
  — concepts that don't apply to function-based trainers.

### Why Separate ABCs Instead of a Unified Hierarchy

Placing `LLMTrainer` under `BaseTrainer` would force config-driven trainers to
implement methods that don't apply (`get_train_func()` returning `None`,
`get_train_func_args()` returning `None`). This violates the
[Liskov Substitution Principle](https://en.wikipedia.org/wiki/Liskov_substitution_principle)
— any code calling `trainer.get_train_func()` would need null-checks, and the
interface would carry dead methods.

Separate ABCs allow each hierarchy to evolve independently:
- KEP-285 can add function-packaging features (e.g., dependency snapshotting) without
  affecting config-driven trainers.
- This KEP can add config-driven features (e.g., recipe selection, config file
  generation) without polluting function-based trainers.

```
    BaseTrainer (ABC)                    LLMTrainer (ABC)
    ├── get_train_func()                 ├── command (ClassVar)
    ├── get_train_func_args()            ├── to_args()
    ├── get_framework_args()             ├── validate()
    ├── validate_runtime()               └── supported_frameworks
    └── supported_frameworks
         │                                    │
    ┌────┴─────┐                    ┌─────────┴─────────┐
    │          │                    │                   │
  Torch     JAX                TorchTune            TRL
  Trainer   Trainer            Trainer              Trainer
  (KEP-285) (KEP-285)          (This KEP)          (This KEP)


    Existing (unchanged, backward compatible):

    CustomTrainer          BuiltinTrainer         CustomTrainerContainer
    (flat dataclass)       (config: LLMTrainer)  (image-based)
```

### Unified API Entry Point

Despite being separate ABCs, both are accepted through the **same API parameter**.
Data scientists see a single, flat interface:

```python
# Function-based (KEP-285)
client.train(trainer=TorchTrainer(func=my_train_fn, num_nodes=4))

# Config-driven (This KEP) — same parameter, same pattern
client.train(trainer=TRLTrainer(trainer_type=SFT, learning_rate=2e-5))
```

The `TrainerClient.train()` signature widens to accept both:

```python
def train(
    self,
    trainer: BaseTrainer | LLMTrainer | CustomTrainer
           | CustomTrainerContainer | BuiltinTrainer | None = None,
    ...
)
```

This gives the best of both worlds: **clean architecture** (separate ABCs, no LSP
violation, independent evolution) with **flat user experience** (one parameter, one
concept to learn, full IDE autocomplete).

### Shared Design Points

- Both KEPs use `trainer.kubeflow.org/framework` as the dispatch key. KEP-285 uses it
  for SDK runtime auto-discovery; this KEP uses it for Go strategy dispatch.
- Both support runtime auto-discovery via `supported_frameworks`.
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

#### Before vs After

```
  BEFORE (hardcoded)                          AFTER (pluggable)
  ══════════════════                          ═════════════════

  ┌─────────────────────┐                    ┌─────────────────────┐
  │    Python SDK        │                    │    Python SDK        │
  │                      │                    │                      │
  │  BuiltinTrainer      │                    │  BuiltinTrainer      │
  │   config:            │                    │   config:            │
  │    TorchTuneConfig   │ ← only option      │    LLMTrainer (ABC)  │ ← pluggable
  │                      │                    │    ├─ TorchTuneTrainer│
  │  get_args_using_     │                    │    └─ TRLTrainer      │
  │  torchtune_config()  │ ← hardcoded        │                      │
  └──────────┬───────────┘                    │  config.command       │
             │                                │  config.to_args()     │ ← generic
  ───────────┼────────────                    └──────────┬───────────┘
             │ TrainJob CR                               │ TrainJob CR
  ┌──────────▼───────────┐                    ┌──────────▼───────────┐
  │    Go Torch Plugin   │                    │    Go Torch Plugin   │
  │                      │                    │                      │
  │  if cmd == ["tune",  │                    │  label = info.Labels │
  │    "run"]:           │                    │    ["framework"]     │
  │    → TorchTune       │ ← cmd sniffing     │  strategies[label]   │
  │  else:               │                    │    .EnforceCommand() │ ← label dispatch
  │    → torchrun        │                    │                      │
  └──────────────────────┘                    └──────────────────────┘
```

#### SDK Type Hierarchy

```
   ┌──────────────────────────────────────────────────────────────────────────┐
   │                     TrainerClient.train(trainer=...)                      │
   │                                                                          │
   │  Accepts ANY of these — unified API, separate abstractions:              │
   └──┬────────────┬────────────────┬────────────────────┬───────────────────┘
      │            │                │                    │
      ▼            ▼                ▼                    ▼
   BaseTrainer   LLMTrainer     CustomTrainer       BuiltinTrainer
   (KEP-285)     (This KEP)     (existing)          (existing)
   func-based    config-driven   user function       wraps LLMTrainer
      │            │
      │            ├── TorchTuneTrainer
      │            │   command: ("tune", "run")
      │            │   framework: torchtune
      │            │
      │            └── TRLTrainer
      │                command: ("trl",)
      │                framework: trl
      │
      ├── TorchTrainer
      │   func: user train()
      │   framework: torch
      │
      └── JAXTrainer
          func: user train()
          framework: jax
```

#### End-to-End Flow: TRL SFT Job

```
  ┌──────────────────────────────────────────────────────────────────────┐
  │  DATA SCIENTIST                                                      │
  │                                                                      │
  │  client.train(                                                       │
  │    trainer=TRLTrainer(trainer_type=SFT, learning_rate=2e-5, ...),    │
  │    initializer=Initializer(model=HF("llama-3.2-1b"), dataset=...)    │
  │  )                                                                   │
  └──────────────────────────────┬───────────────────────────────────────┘
                                 │
                                 ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │  PYTHON SDK                                                          │
  │                                                                      │
  │  1. Auto-discover runtime: list_runtimes()                           │
  │     → filter by label trainer.kubeflow.org/framework: trl            │
  │     → selects "trl-llama3.2-1b"                                      │
  │                                                                      │
  │  2. TRLTrainer.validate() → ok                                       │
  │                                                                      │
  │  3. Build TrainJob CR:                                               │
  │     command: ["trl"]           ← from TRLTrainer.command             │
  │     args: ["sft", "--model_name_or_path", "/workspace/model", ...]   │
  │                                ← from TRLTrainer.to_args()           │
  │     runtimeRef: "trl-llama3.2-1b"                                    │
  └──────────────────────────────┬───────────────────────────────────────┘
                                 │ kubectl apply
                                 ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │  KUBERNETES API SERVER                                               │
  │                                                                      │
  │  Webhook → Torch plugin Validate()                                   │
  │    → label "trl" → TRLStrategy.Validate() → ok                      │
  └──────────────────────────────┬───────────────────────────────────────┘
                                 │ reconcile
                                 ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │  GO TORCH PLUGIN — EnforceMLPolicy()                                 │
  │                                                                      │
  │  Common (all frameworks):                                            │
  │    PET_NNODES=1, PET_NPROC_PER_NODE=auto, PET_NODE_RANK=...         │
  │                                                                      │
  │  Label dispatch:                                                     │
  │    strategies["trl"] → TRLStrategy.EnforceCommand():                 │
  │      + PET_MASTER_ADDR=<job>-node-0-0.<job>                          │
  │      + PET_MASTER_PORT=29500                                         │
  │      + MASTER_ADDR=<job>-node-0-0.<job>    ← accelerate compat      │
  │      + MASTER_PORT=29500                                             │
  │      + WORLD_SIZE=<N>                                                │
  │      + RANK=<JOB_COMPLETION_INDEX>                                   │
  └──────────────────────────────┬───────────────────────────────────────┘
                                 │ SSA
                                 ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │  KUBERNETES PODS                                                     │
  │                                                                      │
  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
  │  │ dataset-init     │  │ model-init       │  │ trainer node        │  │
  │  │ ──────────────── │  │ ──────────────── │  │ ─────────────────── │  │
  │  │ hf://tatsu-lab/  │→ │ hf://meta-llama/ │→ │ trl sft \           │  │
  │  │   alpaca         │  │   Llama-3.2-1B   │  │   --model ...  \    │  │
  │  │                  │  │                  │  │   --dataset ... \   │  │
  │  │ /workspace/      │  │ /workspace/      │  │   --bf16            │  │
  │  │   dataset/       │  │   model/         │  │   --lora_r 16       │  │
  │  └─────────────────┘  └─────────────────┘  └─────────────────────┘  │
  └──────────────────────────────────────────────────────────────────────┘
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
| SDK `BuiltinTrainer` | **Widen** | `TorchTuneConfig` → `LLMTrainer` |
| SDK `TorchTuneConfig` | **Refactor** | → `TorchTuneTrainer(LLMTrainer)`, backward compatible |
| SDK `TRLTrainer` | **New** | New config-driven trainer |
| SDK `TrainerClient.train()` | **Widen** | `trainer=` union accepts `LLMTrainer` directly |
| Container images | **New** | `trl-trainer` image |
| ClusterTrainingRuntimes | **New** | TRL-specific runtime manifests |

### Refactor Scope: What This Actually Touches

This KEP is designed to be **low-risk and minimal**. Here is the concrete scope:

**Python SDK (kubeflow/sdk) — ~200 lines changed across 3 files:**

| File | Change | Lines |
|------|--------|-------|
| `types/types.py` | Add `LLMTrainer` ABC (~30 lines), rename `TorchTuneConfig` class + alias (~5 lines), add `TRLTrainer` (~60 lines) | ~95 new |
| `backends/kubernetes/utils.py` | Replace `isinstance(config, TorchTuneConfig)` with generic `config.command` / `config.to_args()` | ~20 changed |
| `api/trainer_client.py` | Widen `trainer=` union type | ~3 changed |

**Go control plane (kubeflow/trainer) — ~150 lines moved, ~100 lines new:**

| File | Change | Lines |
|------|--------|-------|
| `torch/strategy.go` | New `FrameworkStrategy` interface | ~15 new |
| `torch/torchtune_strategy.go` | **Moved** from `torch.go` (no logic change) | ~80 moved |
| `torch/trl_strategy.go` | New `TRLStrategy` | ~50 new |
| `torch/torch.go` | Replace if/else with `strategies[label]` lookup | ~10 changed |
| `constants/constants.go` | Add `FrameworkLabel` constant | 1 new |

**Key point:** The TorchTune code path is **moved, not rewritten**. The
`TorchTuneStrategy` wraps the exact same functions (`getRecipeAndConfig`,
`extractOverridesFromRuntime`, `validateTorchTune`) that exist today. Existing
tests continue to pass without modification.

**New infrastructure:**
- 1 Dockerfile (~10 lines)
- 1 ClusterTrainingRuntime manifest (~70 lines YAML)
- Helm chart additions (~20 lines)

**Total: ~400 lines of new/changed code to unlock every future LLM backend.**

---

## Design Details

### Python SDK: LLMTrainer Base Class

`LLMTrainer` is a **standalone ABC** purpose-built for config-driven trainers. It
does not extend `BaseTrainer` — they are separate abstractions for separate patterns.

```python
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import ClassVar, Optional


@dataclass
class LLMTrainer(ABC):
    """Base class for config-driven LLM training backends.

    Config-driven trainers use the framework's own CLI as the entrypoint
    (e.g., `trl sft ...`, `tune run ...`) rather than a user-supplied
    Python function. Each implementation translates its config into a
    (command, args) pair that the Kubernetes backend writes into the
    TrainJob CR.

    This is a separate ABC from KEP-285's BaseTrainer. Both are accepted
    through TrainerClient.train(trainer=...) for a unified user experience.

    Class Attributes:
        command: The CLI entrypoint, e.g., ("tune", "run") or ("trl",).
        supported_frameworks: Framework identifiers this trainer supports.
            Must match values of the `trainer.kubeflow.org/framework` label
            on ClusterTrainingRuntime resources.
    """

    command: ClassVar[tuple[str, ...]]
    supported_frameworks: ClassVar[list[str]]

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

    def validate_runtime(self, runtime: "Runtime") -> None:
        """Validate that the given runtime is compatible with this trainer.

        Raises:
            ValueError: If the runtime's framework is not in supported_frameworks.
        """
        if runtime.trainer.framework not in self.supported_frameworks:
            raise ValueError(
                f"{type(self).__name__} supports frameworks "
                f"{self.supported_frameworks}, but runtime '{runtime.name}' "
                f"has framework '{runtime.trainer.framework}'"
            )
```

**Design rationale:**

- `LLMTrainer` does not inherit from `BaseTrainer` — avoids dead methods
  (`get_train_func() → None`) and LSP violations.
- `supported_frameworks` and `validate_runtime()` mirror KEP-285's pattern for
  runtime auto-discovery, ensuring both ABCs work with the same mechanism.
- `command` as a `ClassVar` — it's a property of the trainer *class*, not instances.

### Python SDK: TorchTuneTrainer (Refactored)

`TorchTuneConfig` is refactored into `TorchTuneTrainer` implementing `LLMTrainer`.
All existing fields are preserved. `TorchTuneConfig` becomes a type alias for backward
compatibility.

```python
@dataclass
class TorchTuneTrainer(LLMTrainer):
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
class TRLTrainer(LLMTrainer):
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
        # Produces: ["sft", "--model_name_or_path", "/workspace/model", ...]
        # Prefers initializer workspace paths over config values.
        # Full implementation in LLD.
        ...

    def validate(self) -> None:
        # e.g., lora_r required when use_peft=True
        ...
```

### Python SDK: TrainerClient Integration

`TrainerClient.train(trainer=...)` widens to accept `LLMTrainer` directly in the
union type, alongside `BaseTrainer` (KEP-285), `CustomTrainer`, and `BuiltinTrainer`.

When a `LLMTrainer` is passed:

1. **Runtime auto-discovery**: If `runtime` is `None`, the SDK calls
   `list_runtimes()` and filters by `trainer.kubeflow.org/framework` matching
   `supported_frameworks`. One match → auto-selected. Multiple → error with list.
2. **Validation**: `validate_runtime()` ensures the runtime's framework label matches.
3. **Generic dispatch**: The backend uses `config.command` and `config.to_args()`
   to build the TrainJob CR — no `isinstance` checks, no framework-specific code paths.

### Python SDK: Backward Compatibility

| Existing API | Status | Details |
|-------------|--------|---------|
| `BuiltinTrainer(config=TorchTuneConfig(...))` | **Works** | `TorchTuneConfig` is an alias for `TorchTuneTrainer` |
| `BuiltinTrainer(config=TRLTrainer(...))` | **New** | `BuiltinTrainer.config` type widens to `LLMTrainer` |
| `client.train(trainer=TRLTrainer(...))` | **New** | `LLMTrainer` accepted directly in `trainer=` |
| `CustomTrainer(func=...)` | **Unchanged** | No modifications |
| `CustomTrainerContainer(image=...)` | **Unchanged** | No modifications |

The `BuiltinTrainer.config` field type changes from `TorchTuneConfig` to
`LLMTrainer`. Since `TorchTuneConfig` is a type alias for `TorchTuneTrainer`
which extends `LLMTrainer`, all existing code continues to work.

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

TRL uses `accelerate` for distributed training, which reads standard env vars
(`MASTER_ADDR`, `MASTER_PORT`, `WORLD_SIZE`, `RANK`) rather than the `PET_*`
variants. `TRLStrategy.EnforceCommand()` injects **both sets** for compatibility:

| Env Var | Source | Purpose |
|---------|--------|---------|
| `PET_MASTER_ADDR` | Existing | torchrun compatibility |
| `PET_MASTER_PORT` | Existing | torchrun compatibility |
| `MASTER_ADDR` | **New** | accelerate/TRL |
| `MASTER_PORT` | **New** | accelerate/TRL |
| `WORLD_SIZE` | **New** | accelerate/TRL |
| `RANK` | **New** | From `JOB_COMPLETION_INDEX` |

`TRLStrategy.Validate()` is minimal — TRL config is fully constructed by the SDK,
so Go-side validation only checks structural constraints.

### Go Control Plane: Refactored Torch Plugin Dispatch

The `Torch` struct gains a `strategies map[string]FrameworkStrategy` and both
`EnforceMLPolicy` and `Validate` change from command-sniffing to a 3-line
label lookup:

```go
// BEFORE (torch.go:149)
if !slices.Equal(trainJob.Spec.Trainer.Command, constants.TorchTuneEntrypoint) {
    // torchrun path
} else {
    // TorchTune path
}

// AFTER
fw := info.Labels[constants.FrameworkLabel]
if strategy, ok := t.strategies[fw]; ok {
    return strategy.EnforceCommand(info, trainJob, trainerContainer)
}
// else: default torchrun path (unchanged)
```

New strategies are registered in the constructor — adding a future backend is
one line: `"unsloth": &UnslothStrategy{}`.

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

Config-driven trainer passed directly — no wrapper needed:

```python
from kubeflow.trainer import TrainerClient, TRLTrainer, TRLTrainerType
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

## Alternatives Considered

### 1. LLMTrainer as a subclass of BaseTrainer (unified hierarchy)

Place `LLMTrainer` under KEP-285's `BaseTrainer` so all trainers share one ABC.

**Rejected because:**
- Forces `LLMTrainer` to implement `get_train_func()` and
  `get_train_func_args()` returning `None` — dead methods that violate LSP.
- Any code processing `BaseTrainer` must null-check function-based methods,
  adding defensive logic throughout the backend.
- Couples the evolution of config-driven and function-based trainers — changes
  to one hierarchy's interface affect the other.

### 2. Keep config-driven trainers inside BuiltinTrainer only (no direct API)

Keep the current pattern where config-driven trainers are always wrapped in
`BuiltinTrainer(config=...)`.

**Rejected because:**
- Forces unnecessary nesting: `BuiltinTrainer(config=TRLTrainer(...))` vs
  `TRLTrainer(...)` directly.
- Poor IDE discoverability — data scientists must know about `BuiltinTrainer`
  as a wrapper concept.
- Doesn't enable runtime auto-discovery (BuiltinTrainer has no
  `supported_frameworks`).

### 3. Standalone LLMBackend ABC (original KEP-2839 design)

The original proposal used `LLMBackend` as the ABC name with no relationship to
KEP-285.

**Rejected because:**
- The name `LLMBackend` is too narrow — config-driven trainers could extend beyond
  LLM fine-tuning (e.g., XGBoost config-driven training).
- Didn't address the KEP-285 integration questions raised by maintainers.
- `LLMTrainer` better communicates the pattern (config-driven, trainer hierarchy).

---

## Implementation Plan

This proposal is scoped for 350 hours (GSoC Large) and can be implemented in phases:

**Phase 1: SDK Foundation (Weeks 1-4)**
- Add `LLMTrainer` ABC to `kubeflow/sdk`
- Refactor `TorchTuneConfig` → `TorchTuneTrainer(LLMTrainer)` with alias
- Update `KubernetesBackend` to use `LLMTrainer` interface
- Widen `BuiltinTrainer.config` type to `LLMTrainer`
- Widen `TrainerClient.train()` to accept `LLMTrainer` directly
- Unit tests for backward compatibility
- Coordinate with KEP-285 on shared patterns

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

- `LLMTrainer` interface compliance for `TorchTuneTrainer` and `TRLTrainer`
- `TorchTuneConfig` alias backward compatibility
- `TRLTrainer.to_args()` produces correct CLI arguments for SFT and DPO
- `TRLTrainer.validate()` catches invalid configs (e.g., `use_peft=True` without `lora_r`)
- `BuiltinTrainer(config=TRLTrainer(...))` constructs correctly
- `TrainerClient.train(trainer=TRLTrainer(...))` dispatches correctly
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
| KEP-285 design changes before this KEP lands | `LLMTrainer` is a separate ABC; no dependency on `BaseTrainer` internals |
| Scope creep from adding backends | Scoped to TorchTune + TRL only; other backends follow the same pattern |
| `trainer.kubeflow.org/framework` label not a Go constant | KEP adds `FrameworkLabel` constant; existing manifests already use the label |

---

## Implementation History

- **2025-09-19**: KEP-2839 tracking issue opened by @Electronic-Waste
- **2025-07-24**: Community consensus on Plan 3 (pluggable framework) in #2752
- **2026-01-08**: @andreyvelich reopened issue, looking for contributors
- **2026-02-27**: Initial KEP proposal submitted by @NarayanaSabari
- **2026-03-28**: KEP redesigned to align with KEP-285 BaseTrainer hierarchy
  (LLMTrainer as subclass of BaseTrainer)
- **2026-03-31**: KEP redesigned again based on mentor feedback — LLMTrainer as
  separate ABC from BaseTrainer (clean separation of concerns), with unified API
  entry point through TrainerClient.train(trainer=...)
