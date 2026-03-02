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
  - [High-Level Design](#high-level-design)
    - [Architecture Overview](#architecture-overview)
    - [Component Interaction Flow](#component-interaction-flow)
    - [What Changes vs What Stays](#what-changes-vs-what-stays)
  - [Risks and Mitigations](#risks-and-mitigations)
<!-- /toc -->

---

## Summary

Decouple the `BuiltinTrainer` from TorchTune by introducing a pluggable `LLMBackend`
interface in the SDK and a corresponding `LLMBackendStrategy` in the Go control plane.
TorchTune becomes the first backend implementation (preserving backward compatibility),
and TRL is added as the first new backend with SFT/DPO support.

This builds on [KEP-2401](../2401-llm-trainer-v2/README.md) and the community consensus
on "Plan 3" in [#2752](https://github.com/kubeflow/trainer/issues/2752).
TorchTune stopped adding features in July 2025
([pytorch/torchtune#2883](https://github.com/pytorch/torchtune/issues/2883)).

## Goals

1. Define an `LLMBackend` abstract interface in the Python SDK.
2. Implement a backend registry with `@register_backend` decorator.
3. Refactor `TorchTuneConfig` to implement `LLMBackend` with zero breaking changes.
4. Implement `TRLConfig` backend supporting SFT and DPO.
5. Create TRL container image and `ClusterTrainingRuntime` manifests.
6. Generalize the Go Torch plugin to dispatch via `LLMBackendStrategy` instead of
   hardcoded TorchTune command-sniffing.
7. Support external (out-of-tree) backend registration.

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
                          │ to_args()                        │ to_command() / to_args()
                          ▼                                 ▼
                   get_args_using_                   backend.to_command()
                   torchtune_config()                backend.to_args()
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
         TRLConfig.to_command() → ("trl",)
         TRLConfig.to_args()   → ["sft", "--model_name_or_path", "/workspace/model", ...]
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
| SDK registry | **New** | `@register_backend` decorator |
| Container images | **New** | `trl-trainer` image |
| ClusterTrainingRuntimes | **New** | TRL-specific runtime manifests |

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| TRL CLI changes across versions | Pin version range in requirements.txt; version compat tests |
| TRL uses accelerate, not torchrun, for distributed | TRLStrategy injects both `PET_*` and standard env vars; accelerate reads `MASTER_ADDR`, `MASTER_PORT`, `WORLD_SIZE`, `RANK`; validated in E2E |
| Multi-node TRL untested at scale | Phase 1 scoped to single-node multi-GPU; multi-node added in Phase 2 with dedicated E2E |
| SDK type widening affects static analysis | TorchTuneConfig is a subtype of LLMBackend; passes type checks |
| Scope creep from adding backends | Scoped to TorchTune + TRL only |
| `trainer.kubeflow.org/framework` label not a Go constant | KEP adds `RuntimeFrameworkLabel` constant; existing manifests already use the label |