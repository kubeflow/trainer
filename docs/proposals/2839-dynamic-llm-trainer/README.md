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
  - [Design Details](#design-details)
    - [SDK: LLMBackend Interface](#sdk-llmbackend-interface)
    - [SDK: Backend Registry](#sdk-backend-registry)
    - [SDK: BuiltinTrainer Change](#sdk-builtintrainer-change)
    - [SDK: TorchTune Backend (Refactored)](#sdk-torchtune-backend-refactored)
    - [SDK: TRL Backend](#sdk-trl-backend)
    - [Go Control Plane: LLMBackendStrategy](#go-control-plane-llmbackendstrategy)
    - [Go Control Plane: Strategy Dispatch](#go-control-plane-strategy-dispatch)
    - [Go Control Plane: Constants](#go-control-plane-constants)
    - [Container Images](#container-images)
    - [ClusterTrainingRuntimes](#clustertrainingruntimes)
    - [Helm Chart Changes](#helm-chart-changes)
  - [Test Plan](#test-plan)
  - [Risks and Mitigations](#risks-and-mitigations)
<!-- /toc -->

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
2. CRD schema changes -- operates within existing `.spec.trainer.command`/`.spec.trainer.args`.
3. New Kubernetes resource topologies (e.g., launcher/worker patterns).
4. Go-side distributed training plugins per backend (backends use existing torchrun infra).

## Design Details

### SDK: LLMBackend Interface

New file: `kubeflow/trainer/types/backends/__init__.py`

```python
import abc
from dataclasses import dataclass


@dataclass
class LLMBackend(abc.ABC):

    @abc.abstractmethod
    def to_command(self) -> tuple[str, ...]:
        """Container entrypoint command."""
        ...

    @abc.abstractmethod
    def to_args(self) -> list[str]:
        """CLI arguments for .spec.trainer.args."""
        ...

    @abc.abstractmethod
    def framework(self) -> str:
        """Framework identifier matching trainer.kubeflow.org/framework label."""
        ...

    def validate(self) -> None:
        """Optional config validation. Raise ValueError on invalid config."""
        pass

    @property
    def num_nodes(self) -> int | None:
        return None

    @property
    def resources_per_node(self) -> dict | None:
        return None
```

### SDK: Backend Registry

New file: `kubeflow/trainer/types/backends/registry.py`

```python
from collections.abc import Callable

_BACKEND_REGISTRY: dict[str, type] = {}


def register_backend(name: str) -> Callable:
    def decorator(cls):
        if name in _BACKEND_REGISTRY:
            raise ValueError(
                f"Backend '{name}' already registered by {_BACKEND_REGISTRY[name].__name__}."
            )
        _BACKEND_REGISTRY[name] = cls
        return cls
    return decorator


def get_registered_backends() -> dict[str, type]:
    return dict(_BACKEND_REGISTRY)


def get_backend(name: str) -> type | None:
    return _BACKEND_REGISTRY.get(name)
```

### SDK: BuiltinTrainer Change

In `kubeflow/trainer/types/types.py`:

```python
# BEFORE
@dataclass
class BuiltinTrainer:
    config: TorchTuneConfig

# AFTER
@dataclass
class BuiltinTrainer:
    config: LLMBackend
```

`TorchTuneConfig` implements `LLMBackend`, so existing
`BuiltinTrainer(config=TorchTuneConfig(...))` code is unchanged.

The SDK's `KubernetesBackend` dispatch becomes generic:

```python
def _get_trainer_spec(self, trainer: BuiltinTrainer) -> dict:
    backend = trainer.config
    backend.validate()
    return {
        "command": list(backend.to_command()),
        "args": backend.to_args(),
    }
```

This replaces the current `get_args_using_torchtune_config()`.

### SDK: TorchTune Backend (Refactored)

New file: `kubeflow/trainer/types/backends/torchtune.py`

```python
@register_backend("torchtune")
@dataclass
class TorchTuneConfig(LLMBackend):
    dtype: DataType | None = None
    batch_size: int | None = None
    epochs: int | None = None
    loss: Loss | None = None
    _num_nodes: int | None = field(default=None, repr=True)
    peft_config: LoraConfig | None = None
    dataset_preprocess_config: TorchTuneInstructDataset | None = None
    _resources_per_node: dict | None = field(default=None, repr=True)

    def to_command(self) -> tuple[str, ...]:
        return ("tune", "run")

    def to_args(self) -> list[str]:
        args = []
        if self.dtype is not None:
            args.append(f"dtype={self.dtype.value}")
        if self.batch_size is not None:
            args.append(f"batch_size={self.batch_size}")
        if self.epochs is not None:
            args.append(f"epochs={self.epochs}")
        if self.loss is not None:
            args.append(f"loss={self.loss.value}")
        if self.peft_config is not None:
            args.extend(_get_args_from_peft_config(self.peft_config))
        if self.dataset_preprocess_config is not None:
            args.extend(
                _get_args_from_dataset_preprocess_config(self.dataset_preprocess_config)
            )
        return args

    def framework(self) -> str:
        return "torchtune"

    @property
    def num_nodes(self) -> int | None:
        return self._num_nodes

    @property
    def resources_per_node(self) -> dict | None:
        return self._resources_per_node
```

Helper functions `_get_args_from_peft_config()` and
`_get_args_from_dataset_preprocess_config()` are extracted from the current
`get_args_using_torchtune_config()` in `backends/kubernetes/utils.py` unchanged.

### SDK: TRL Backend

New file: `kubeflow/trainer/types/backends/trl.py`

```python
class TRLTrainerType(Enum):
    SFT = "sft"
    DPO = "dpo"
    PPO = "ppo"
    ORPO = "orpo"
    KTO = "kto"


@dataclass
class TRLPeftConfig:
    r: int = 16
    lora_alpha: int = 32
    lora_dropout: float = 0.05
    target_modules: list[str] = field(default_factory=lambda: ["q_proj", "v_proj"])
    use_rslora: bool = False
    use_dora: bool = False


@dataclass
class TRLSFTConfig:
    max_seq_length: int = 2048
    packing: bool = False
    dataset_text_field: str | None = None


@dataclass
class TRLDPOConfig:
    beta: float = 0.1
    max_length: int = 1024
    max_prompt_length: int = 512
    loss_type: str = "sigmoid"


@register_backend("trl")
@dataclass
class TRLConfig(LLMBackend):
    trainer_type: TRLTrainerType = TRLTrainerType.SFT
    model_name_or_path: str = "/workspace/model"
    learning_rate: float = 2e-5
    num_train_epochs: int = 3
    per_device_train_batch_size: int = 4
    gradient_accumulation_steps: int = 1
    bf16: bool = True
    fp16: bool = False
    peft_config: TRLPeftConfig | None = None
    sft_config: TRLSFTConfig | None = None
    dpo_config: TRLDPOConfig | None = None
    _num_nodes: int | None = None
    _resources_per_node: dict | None = None
    output_dir: str = "/workspace/output"

    def to_command(self) -> tuple[str, ...]:
        return ("python", "-m", "trl")

    def to_args(self) -> list[str]:
        args = [self.trainer_type.value]
        args.extend(["--model_name_or_path", self.model_name_or_path])
        args.extend(["--output_dir", self.output_dir])
        args.extend(["--learning_rate", str(self.learning_rate)])
        args.extend(["--num_train_epochs", str(self.num_train_epochs)])
        args.extend(["--per_device_train_batch_size", str(self.per_device_train_batch_size)])
        args.extend(["--gradient_accumulation_steps", str(self.gradient_accumulation_steps)])
        if self.bf16:
            args.append("--bf16")
        if self.fp16:
            args.append("--fp16")
        if self.peft_config is not None:
            args.append("--use_peft")
            args.extend(["--lora_r", str(self.peft_config.r)])
            args.extend(["--lora_alpha", str(self.peft_config.lora_alpha)])
            args.extend(["--lora_dropout", str(self.peft_config.lora_dropout)])
            if self.peft_config.target_modules:
                args.extend(["--lora_target_modules", *self.peft_config.target_modules])
        if self.trainer_type == TRLTrainerType.SFT and self.sft_config:
            args.extend(["--max_seq_length", str(self.sft_config.max_seq_length)])
            if self.sft_config.packing:
                args.append("--packing")
            if self.sft_config.dataset_text_field:
                args.extend(["--dataset_text_field", self.sft_config.dataset_text_field])
        if self.trainer_type == TRLTrainerType.DPO and self.dpo_config:
            args.extend(["--beta", str(self.dpo_config.beta)])
            args.extend(["--max_length", str(self.dpo_config.max_length)])
            args.extend(["--max_prompt_length", str(self.dpo_config.max_prompt_length)])
            args.extend(["--loss_type", self.dpo_config.loss_type])
        return args

    def framework(self) -> str:
        return "trl"

    def validate(self) -> None:
        if self.bf16 and self.fp16:
            raise ValueError("Cannot enable both bf16 and fp16.")
        if self.trainer_type == TRLTrainerType.DPO and self.sft_config is not None:
            raise ValueError("sft_config should not be set when trainer_type is DPO.")
        if self.trainer_type == TRLTrainerType.SFT and self.dpo_config is not None:
            raise ValueError("dpo_config should not be set when trainer_type is SFT.")

    @property
    def num_nodes(self) -> int | None:
        return self._num_nodes

    @property
    def resources_per_node(self) -> dict | None:
        return self._resources_per_node
```

### Go Control Plane: LLMBackendStrategy

New file: `pkg/runtime/framework/plugins/torch/backend.go`

Replace the current command-sniffing in `torch.go` and `torchtune.go` with a strategy
interface:

```go
type LLMBackendStrategy interface {
    Name() string
    EnforceCommand(info *runtime.Info, trainJob *trainer.TrainJob) error
    Validate(info *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList)
}
```

`TorchTuneStrategy` wraps the existing `torchtune.go` logic (getRecipeAndConfig,
extractOverridesFromRuntime, validateTorchTune) with no behavioral changes:

```go
type TorchTuneStrategy struct{}

func (s *TorchTuneStrategy) Name() string { return "torchtune" }

func (s *TorchTuneStrategy) EnforceCommand(info *runtime.Info, trainJob *trainer.TrainJob) error {
    // Existing torchtune.go logic: rdzv_endpoint, recipe, config, overrides
    return nil
}

func (s *TorchTuneStrategy) Validate(info *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
    return validateTorchTune(info, newObj)
}
```

`TRLStrategy` is minimal -- TRL config is fully constructed by the SDK:

```go
type TRLStrategy struct{}

func (s *TRLStrategy) Name() string { return "trl" }

func (s *TRLStrategy) EnforceCommand(info *runtime.Info, trainJob *trainer.TrainJob) error {
    // Inject rendezvous endpoint for multi-node. SDK provides full args.
    return nil
}

func (s *TRLStrategy) Validate(info *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
    return nil, nil
}
```

### Go Control Plane: Strategy Dispatch

The Torch plugin struct holds registered strategies and dispatches via the runtime's
framework label instead of command-sniffing:

```go
type Torch struct {
    backends map[string]LLMBackendStrategy
}

func New(ctx context.Context, client client.Client, indexer client.FieldIndexer) (framework.Plugin, error) {
    return &Torch{
        backends: map[string]LLMBackendStrategy{
            "torchtune": &TorchTuneStrategy{},
            "trl":       &TRLStrategy{},
        },
    }, nil
}

func (t *Torch) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
    // Common torch distributed setup (PET_NUM_NODES, PET_NPROC_PER_NODE, etc.)
    // ...

    framework := info.Labels[constants.RuntimeFrameworkLabel]
    if strategy, ok := t.backends[framework]; ok {
        return strategy.EnforceCommand(info, trainJob)
    }

    // Default: standard torchrun path
    // ...
}

func (t *Torch) Validate(ctx context.Context, info *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
    // Common torch validation ...

    framework := info.Labels[constants.RuntimeFrameworkLabel]
    if strategy, ok := t.backends[framework]; ok {
        warnings, errs := strategy.Validate(info, oldObj, newObj)
        // append ...
    }
    // ...
}
```

### Go Control Plane: Constants

```go
// pkg/constants/constants.go

TRLEntrypoint        = []string{"python", "-m", "trl"}
TRLFrameworkLabel    = "trl"
RuntimeFrameworkLabel = "trainer.kubeflow.org/framework"
```

### Container Images

```dockerfile
# cmd/trainers/trl/Dockerfile
FROM pytorch/pytorch:2.9.1-cuda12.8-cudnn9-runtime
WORKDIR /workspace
RUN apt update && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*
COPY cmd/trainers/trl/requirements.txt .
RUN pip install -r requirements.txt
```

```
# cmd/trainers/trl/requirements.txt
trl>=0.15.0
transformers>=4.48.0
datasets>=3.0.0
accelerate>=1.2.0
peft>=0.14.0
bitsandbytes>=0.41.1
```

Published as `ghcr.io/kubeflow/trainer/trl-trainer`.

### ClusterTrainingRuntimes

Example: `manifests/base/runtimes/trl/llama3_2/llama3_2_1B.yaml`

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
                        - mountPath: /workspace/dataset
                          name: workspace
                  volumes:
                    - name: workspace
                      persistentVolumeClaim:
                        claimName: workspace
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
                        - mountPath: /workspace/model
                          name: workspace
                  volumes:
                    - name: workspace
                      persistentVolumeClaim:
                        claimName: workspace
        - name: node
          dependsOn:
            - dataset-initializer
            - model-initializer
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
                        - python
                        - -m
                        - trl
                      args:
                        - sft
                        - --model_name_or_path
                        - /workspace/model
                        - --output_dir
                        - /workspace/output
                        - --dataset_name
                        - /workspace/dataset
                      resources:
                        limits:
                          nvidia.com/gpu: 2
                      volumeMounts:
                        - mountPath: /workspace
                          name: workspace
                  volumes:
                    - name: workspace
                      persistentVolumeClaim:
                        claimName: workspace
```

Directory structure:

```
manifests/base/runtimes/
├── torchtune/           # Existing (unchanged)
└── trl/                 # NEW
    ├── kustomization.yaml
    ├── llama3_2/
    │   ├── llama3_2_1B.yaml
    │   └── llama3_2_3B.yaml
    └── qwen2_5/
        └── qwen2_5_1.5B.yaml
```

### Helm Chart Changes

```yaml
# charts/kubeflow-trainer/values.yaml (additions)
runtimes:
  trlDistributed:
    image:
      registry: ghcr.io
      repository: kubeflow/trainer/trl-trainer
      tag: ""
    llama3_2_1B:
      enabled: false
    llama3_2_3B:
      enabled: false
    qwen2_5_1_5B:
      enabled: false
```

## Test Plan

**Unit tests**:
- SDK backend registry: registration, duplicate detection, lookup.
- `TorchTuneConfig` backward compat: `to_args()` identical to current `get_args_using_torchtune_config()`.
- `TRLConfig`: `to_args()`, `to_command()`, `validate()` for SFT, DPO, error cases.
- Go Torch plugin: strategy dispatch, `TorchTuneStrategy` (existing cases), `TRLStrategy`.

**Integration tests**:
- SDK creates valid TRL TrainJob CR that controller reconciles into JobSet.
- `client.list_runtimes()` returns both TorchTune and TRL runtimes.
- Existing TorchTune examples execute unchanged.

**E2E tests**:
- TRL SFT with Llama 3.2 1B on Alpaca (GPU).
- TRL DPO with preference dataset (GPU).
- TorchTune regression.

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| TRL CLI changes across versions | Pin version in requirements.txt; version compat tests |
| TRL uses accelerate, not torchrun | TRL supports torchrun-compatible launch; validate in E2E |
| SDK type widening affects static analysis | TorchTuneConfig is a subtype of LLMBackend; passes type checks |
| Scope creep from adding backends | Scoped to TorchTune + TRL only |