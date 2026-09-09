# KEP-2839: Kubeflow Dynamic LLM Trainer Framework

Authors:

- Yassin Nouh - [@YassinNouh21](https://github.com/YassinNouh21)
- Saad Zaher - [@szaher](https://github.com/szaher)

Creation date: 2026-02-11

## Summary

Kubeflow Trainer can run exactly one config-driven LLM framework: TorchTune, whose upstream
development stopped in July 2025. This proposal makes the framework axis extensible and lands
Hugging Face TRL as the second framework, adding the post-training methods the SDK cannot
express today — DPO and GRPO.

The work spans two repositories, and each side is independently useful.

**Server side (`kubeflow/trainer`).** A framework becomes supportable by adding a runtime image
and a `ClusterTrainingRuntime`. The CRDs, the controller and the torch plugin are unchanged —
no Go at all. The torch plugin already publishes the cluster topology as `PET_*` environment
variables, and `PET_*` is torchrun's own env interface, so the TRL runtime launches TRL's
training script with torchrun (`torchrun -m trl.scripts.sft`) and the topology flows through
with no translation. The `trl` CLI is not used: it hands off to `accelerate launch`, which does
not read `PET_*` and degrades to single-process training while still reporting success. That
failure is recorded as a limitation, and supporting accelerate is deferred.

**Client side (`kubeflow/sdk`).** One additive change: `FrameworkConfig`, an abstract config
that knows its framework label, its entrypoint and how to render itself as arguments, plus a
registry keyed on the framework label. `TorchTuneConfig` becomes an instance of it and
`BuiltinTrainer.config` widens from the concrete `TorchTuneConfig` to `FrameworkConfig`. TRL
then arrives as `TRLConfig`, carrying a typed per-method config in its `method` slot.

The contract between the two sides already exists and is narrow: the framework label, and the
container `command` the SDK appends rendered arguments to. The runtime is usable from raw YAML
before any SDK change ships.

A proof of concept for the server side was built and run on a cluster; its results are in
[Proof of concept](#proof-of-concept) and they changed the design.

## Motivation

TorchTune is the only config-driven framework the Trainer supports, it covers supervised
fine-tuning only, and its upstream development halted on 15 July 2025
([torchtune#2883](https://github.com/meta-pytorch/torchtune/issues/2883)). A data scientist who
wants DPO or GRPO has no supported path: `BuiltinTrainer` accepts a `TorchTuneConfig` and
nothing else, and the SDK hardcodes TorchTune at four points, so a second framework cannot be
added without changing shared code each time.

Meanwhile LLM post-training has moved past SFT. Reward-driven alignment (DPO) and
group-relative policy optimization (GRPO) are the techniques users are asking for
([#3508](https://github.com/kubeflow/trainer/issues/3508)), and the Trainer has no surface for
either.

### Goals

**Server side**

1. Ship TRL as the second in-tree config-driven framework — a `cmd/trainers/trl/` image and a
   runtime under `manifests/base/runtimes/trl/`, launched with **torchrun**, the canonical
   launcher the torch plugin already serves.
2. Keep the control plane completely unchanged: no Go, no CRD change, no knowledge of TRL.
3. Document precisely why the `trl` CLI / `accelerate launch` route is excluded from phase one
   (it fails *silently* at world size 1), so the future accelerate discussion can start from
   measured results.

**Client side**

4. Introduce `FrameworkConfig` and a framework registry, so the SDK resolves the entrypoint and
   the argument rendering from the config rather than from a TorchTune branch — removing the
   four hardcoded TorchTune couplings.
5. Add `TRLConfig` with typed per-method configs for SFT, DPO and GRPO, proving the extension
   model with no new trainer class.
6. Provide an extension point for community config-driven frameworks — a `FrameworkConfig`
   subclass in any package, registered by import.
7. Keep 100% backward compatibility with `CustomTrainer`, `CustomTrainerContainer`,
   `BuiltinTrainer` and `TrainerClient.train()`.

### Non-Goals

1. **Any control-plane change.** No CRD or schema change, no version bump, and no Go: the
   server-side work is a new image and new manifests. `EnforceMLPolicy` and `Validate` are both
   unchanged — see [What changes in the plugin](#what-changes-in-the-plugin).
2. **A new framework plugin.** TRL reuses the torch plugin via `mlPolicy.torch`; no entry is
   added to `pkg/runtime/framework/plugins/registry.go`.
3. **New runtime labels or conventions.** `trainer.kubeflow.org/framework` is unchanged.
4. **Supporting `accelerate launch`.** torchrun is the only launcher in scope. Accelerate's
   launcher, and the launcher-level options only it offers, is a separate discussion together
   with dynamic registration. See
   [Limitation: the TRL CLI and `accelerate launch`](#limitation-the-trl-cli-and-accelerate-launch)
   for why it is excluded and [Open Questions](#open-questions) for what taking it up would
   involve.
5. **A general SDK trainer hierarchy.** An earlier draft of this KEP, inherited from
   [sdk#285](https://github.com/kubeflow/sdk/issues/285), also proposed `BaseTrainer` /
   `FuncTrainer` / `ConfigTrainer`, per-framework trainer classes (`TorchTrainer`,
   `DeepSpeedTrainer`, `JAXTrainer`, `XGBoostTrainer`), `RuntimeConfig` and runtime
   auto-discovery. None of that is needed to land TRL, and it is removed here so this KEP
   covers one feature. It remains tracked in sdk#285.
6. **Deprecating `CustomTrainer` or `BuiltinTrainer`.** Both stay supported, unchanged.
7. **In-tree Axolotl, LlamaFactory or Unsloth runtimes.** Only TRL is proposed here; further
   frameworks are follow-up work reached through the same extension point.
8. **Changes to `TrainJobTemplate`.**

## Proposal

Add a TRL runtime image and manifest on the server, and on the client replace the hardcoded
TorchTune branch with a framework registry that any config-driven framework can join. TRL is
the first framework to join it.

### User Stories

#### Story 1: Platform admin installs TRL

As a platform admin, I install the Trainer and get a `trl-distributed`
`ClusterTrainingRuntime` alongside the TorchTune runtimes. I did not have to build an image or
write a plugin, and I can verify the runtime is correct by submitting a raw-YAML `TrainJob`
with `spec.initializer` set against it before anyone uses the SDK.

#### Story 2: Data scientist runs supervised fine-tuning

As a data scientist, I fine-tune a model with TRL from the SDK using the same
`BuiltinTrainer` I already use for TorchTune, swapping only the config:

```python
TrainerClient().train(
    runtime=TrainerClient().get_runtime("trl-distributed"),
    initializer=Initializer(
        model=HuggingFaceModelInitializer(storage_uri="hf://Qwen/Qwen2.5-0.5B"),
        dataset=HuggingFaceDatasetInitializer(storage_uri="hf://trl-lib/Capybara"),
    ),
    trainer=BuiltinTrainer(
        config=TRLConfig(
            method=TRLSFTConfig(packing=True),
            learning_rate=2e-5,
            num_nodes=2,
        ),
    ),
)
```

The model and dataset are given exactly as for TorchTune, through the `Initializer`.

#### Story 3: Data scientist runs GRPO

As a data scientist, I run a post-training method the SDK cannot express today. My editor
tells me which parameters belong to GRPO, and passing a DPO-only parameter is an error I see
before the job is submitted, not a flag TRL silently ignores at runtime:

```python
TrainerClient().train(
    initializer=Initializer(
        model=HuggingFaceModelInitializer(storage_uri="hf://Qwen/Qwen2.5-0.5B"),
        dataset=HuggingFaceDatasetInitializer(storage_uri="hf://trl-lib/tldr"),
    ),
    trainer=BuiltinTrainer(
        config=TRLConfig(
            method=TRLGRPOConfig(
                beta=0.04,
                num_generations=8,
                reward_funcs=["think_format_reward"],
            ),
            learning_rate=1e-6,
            num_nodes=2,
            resources_per_node={"gpu": 1},
        ),
    ),
)
```

The config carries only TRL flags; where the model and dataset come from is unchanged.

#### Story 4: Community adds a framework out of tree

As a maintainer of another config-driven framework, I publish a runtime image labelled with my
framework and a `FrameworkConfig` subclass in my own package. Importing my package registers
it. Neither the Trainer controller nor the SDK needs a commit.

### Notes/Constraints/Caveats

**TRL loads models and datasets from local paths, not only the Hugging Face Hub.** TRL's
`--dataset_name` is documented as "Path or name of the dataset to load", and
`--model_name_or_path` is passed to `from_pretrained`, which accepts a local directory. So the
initializer-staged `/workspace/model` and `/workspace/dataset` work, and the Trainer's existing
dataset and model initializers apply unchanged. Two caveats: a local dataset *directory* is
loaded as parquet/JSON/CSV, so dataset repositories that ship a loading script are not
equivalent; and a dataset with several configurations additionally needs `--dataset_config`.

**A TRL upgrade is an image rebuild, not a controller release.** Which module torchrun runs
(`trl.scripts.sft` vs `.dpo` vs `.grpo`) is chosen by the SDK and shipped in the image, so the
control plane never learns a TRL version.

**Elastic training is not supported by the Trainer for either launcher.** `torch.go:109`
carries the TODO to add it once JobSet supports elastic jobs. Choosing torchrun does not change
this — see [What torchrun gives up](#what-torchrun-gives-up).

### Risks and Mitigations

| Risk | Mitigation |
|---|---|
| **A silently degraded run looks like success.** The `trl-demo` failure exited 0 on both pods, so a two-node job that trained two isolated copies was indistinguishable from one that trained jointly. torchrun-first removes that particular cause, not the class. | The E2E test asserts the achieved world size from the logs, not just `Complete`. Closing the class needs a runtime→status channel, which is out of scope here — see [Open Questions](#open-questions). |
| **TRL's surface is not frozen**, and typed configs mirroring its flags will drift. TRL moved PPO to `trl.experimental` and then dropped it from the package altogether. | The configs target the *script flags*, not the Python API, so a TRL upgrade changes the image rather than the SDK types, and drift is confined to the per-method config classes. The launch-path proof re-runs on every `trl` version bump. |
| **TRL ignores unknown flags.** `trl/scripts/*.py` parse with `fail_with_unknown_args=False`, so a misapplied flag is dropped rather than rejected. | The typed per-method configs make a wrong-method parameter a construction-time `TypeError` in the SDK, before the TrainJob is created. This is the primary reason for the config shape in [Choosing the config shape](#choosing-the-config-shape). |
| **`extra_args` reopens the silent-ignore hole** for anything passed through it, since those flags are not typed and TRL will not reject them. | Accepted by design: it is the escape hatch for the 161 shared flags, documented as unvalidated. Everything the KEP types is still checked at construction. |
| **A framework registered twice** (in-tree and out-of-tree) claims the same label. | Registration is explicit and import-ordered; a later registration wins, which is what lets a fork shadow a stalled in-tree config. Documented, not silent. |

## Design Details

The two sides are independent and are described separately. The contract between them is the
framework label `trainer.kubeflow.org/framework` and the container `command` that the SDK
appends rendered arguments to.

### Server Side (`kubeflow/trainer`)

#### What the torch plugin already provides

Any runtime declaring `mlPolicy.torch` is processed by the torch plugin
(`pkg/runtime/framework/plugins/torch/torch.go`). Its `EnforceMLPolicy` injects into the
trainer container:

| Env var | Value | Source |
|---|---|---|
| `PET_NNODES` | trainer PodSet count | `torch.go:147` |
| `PET_NPROC_PER_NODE` | `spec.trainer.numProcPerNode`, else `"auto"`, else the CPU count when no GPU | `torch.go:121-135`, `:150` |
| `PET_NODE_RANK` | field ref to the JobSet completion index | `torch.go:152-156` |
| `PET_MASTER_ADDR` | `<trainjob>-node-0-0.<trainjob>` | `torch.go:161-162` |
| `PET_MASTER_PORT` | `29500` | `torch.go:163-165` |

It also opens port 29500 for the headless service, and `Validate` rejects a `TrainJob` that tries
to set any of those five variables itself (`torch.go:67-77`, against
`constants.TorchRunReservedEnvNames`).

There is exactly one framework-specific branch: when `spec.trainer.command` equals
`["tune", "run"]`, the plugin withholds `PET_MASTER_ADDR` / `PET_MASTER_PORT` and rewrites the
command with `--rdzv_endpoint=<host>:29500` plus the recipe, config and runtime-derived
overrides (`torch.go:175-201`). Every other command gets all five and no rewrite.

So a second framework inherits a complete description of the topology for free. **The only
question is whether its launcher reads it.**

#### `PET_*` is torchrun's private interface

`PET_*` is not a Kubeflow convention. It is how torchrun reads its own flags from the
environment: for every flag, torchrun also accepts a `PET_`-prefixed env var
(`torch.distributed.argparse_util`), so `PET_NNODES` is simply `--nnodes`. That puts `PET_*` at
a specific layer:

```
1. Torch plugin        →  sets PET_NNODES, PET_NPROC_PER_NODE, PET_NODE_RANK,
                          PET_MASTER_ADDR, PET_MASTER_PORT
2. Launcher (torchrun) →  reads PET_*, decides how many processes to spawn
3. Launcher            →  spawns them, exports RANK, WORLD_SIZE, LOCAL_RANK,
                          MASTER_ADDR, MASTER_PORT
4. Training script     →  reads those standard variables
```

`PET_*` is the launcher's **input**; `RANK` / `WORLD_SIZE` are its **output**. A framework that
does not launch through torchrun never reaches step 2, so nothing produces step 3.

TRL's *CLI* is such a case: `trl sft` hands off to `accelerate launch`
(`trl/cli/accelerate_launcher.py` resolves the training script and calls accelerate's
`launch_command`), and **accelerate-the-launcher has no reference to `PET_` anywhere in its
source** — it is a *peer* of torchrun that takes topology from flags.

But the CLI is only a wrapper. What it wraps — `trl/scripts/sft.py` — is a **plain training
script**: `TrlParser` argument parsing plus `SFTTrainer(...).train()`, no launcher logic,
runnable as a module. And the accelerate *library* inside it (`PartialState`, a hard dependency
of TRL) detects torchrun's step-3 output from the environment: `LOCAL_RANK` set → multi-GPU
(nccl); `WORLD_SIZE > 1` on CPU → multi-CPU (gloo).

So TRL fits the diagram directly by making torchrun the launcher of TRL's own script:

```yaml
command: [torchrun, -m, trl.scripts.sft]
```

torchrun consumes the plugin's `PET_*` natively (including `numProcPerNode: "auto"`, which it
resolves itself), and the script's in-process accelerate reads what torchrun exports. No
translation of variables, no translation of commands, no plugin change — the same path other
torchrun-native frameworks in the Trainer already use, Megatron included.

#### Proof of concept

Three runs, all with the exact `PET_*` variables the unmodified torch plugin injects. The only
difference is what consumed them.

- **Cluster, bare torchrun.** Two CPU pods reached `world_size=2` with a Gloo all-reduce summing
  to `2.0` — real rendezvous from the plugin's injected env, so the plugin→torchrun half holds on
  real infrastructure.
- **Container, `torchrun -m trl.scripts.sft`** — the runtime's exact command, with only the five
  `PET_*` set and no topology flags. The first-step `epoch` moved from `0.05882` (1/17, single-node
  control) to `0.1111` (1/9), so the dataset sharded across 2 ranks: **world size 2**.
- **Cluster, `trl sft` → `accelerate launch`.** Flags translated correctly per pod, yet the run
  stayed at **world size 1** while both pods reported `Succeeded` and the TrainJob `Complete` — the
  silent failure that rules the CLI out.

Full write-up, the reproduction script (docker only, `--use_cpu`, no GPUs) and the draft image and
manifests: [`examples/trl/README.md` on the PoC branch](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md).

**What is not yet proven.** The two halves were verified separately — cross-pod rendezvous on the
cluster, and the TRL script under torchrun in a container. Running the TRL script across two real
pods is the remaining step and the first item of the E2E phase.

#### Limitation: the TRL CLI and `accelerate launch`

The TRL CLI cannot be the runtime command. The `accelerate launch` run failed for two causes,
both in accelerate's launcher and both **exiting 0**, so JobSet observes completion, the TrainJob
reports `Succeeded`, and no signal reaches the control plane:

1. **`--use_cpu` disables every distributed path**, so a CPU run falls through to the simple
   launcher, which never sets `RANK` / `WORLD_SIZE`. Demonstrated on the cluster.
2. **Multi-GPU is inferred from the local device count** in the current process, which is never
   above one at one GPU per pod.

Both are properties of the *launcher* and vanish when torchrun launches the script: the container
run above used `--use_cpu` and still reached world size 2, because in-process accelerate reads the
env torchrun had already exported. The source-level detail is in the
[PoC write-up](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md).

Choosing torchrun is also the pattern the Trainer already follows — Megatron runs under the
torchrun launcher today — so TRL joins an established path rather than introducing a second one.

##### What torchrun gives up

Most of what `accelerate launch` configures is also exposed by HF `TrainingArguments`, so it
survives as ordinary script flags: `--deepspeed`, `--fsdp`, `--fsdp_config`, `--bf16` / `--fp16`,
`--gradient_checkpointing`, `--torch_compile`, `--ddp_backend`. DeepSpeed and FSDP are therefore
**not** lost. What is genuinely unavailable under torchrun:

| Not available | Impact |
|---|---|
| `--config_file` — one accelerate YAML per job | Cosmetic here: the same settings are reachable as script flags, and a runtime manifest is already the place operators express them. |
| `--use_megatron_lm` | The accelerate Megatron-LM plugin. Out of scope for TRL post-training. |
| `--tpu` | No TPU support in the Trainer's torch plugin either. |
| `--mixed_precision`, `--dynamo_backend` at launch time | Covered by `--bf16` / `--fp16` and `--torch_compile` on the script. |
| `--mpirun_hostfile` | Only needed because accelerate cannot do multi-process CPU without MPI. torchrun does it natively over gloo, so this is a limitation torchrun *removes*. |

Elasticity is not lost either: for multi-GPU and DeepSpeed, `accelerate launch` runs torchrun
itself and passes its elastic flags straight through, so calling torchrun directly gives the same
behaviour with one less layer. Elastic training is in any case unsupported by the Trainer for
either launcher (`torch.go:109`).

**In short: we do not use accelerate's launcher — torchrun launches TRL's script directly.**
accelerate is still in the image because TRL depends on it (`accelerate>=1.4.0`, and HF `Trainer`
cannot be imported without it), but in-process it only reads the variables torchrun exported. What
is excluded here is a second launcher, not a library.

#### What changes in the plugin

Nothing. `EnforceMLPolicy` today supports the torchrun launcher through the `PET_*` variables,
and that is what TRL needs — every run in [Proof of concept](#proof-of-concept) used an
unmodified `torch.go`. `Validate` is unchanged too.

The difference from TorchTune is where the training arguments are built:

| | TorchTune | TRL |
|---|---|---|
| Launcher | `tune run` | `torchrun` |
| Rendezvous | a command-line argument, so the plugin rewrites the command (`torch.go:175-201`) | environment, so the command is left alone |
| Who builds the arguments | the plugin, in Go: `torchtune.go` picks the recipe, adds `--config`, and appends the path overrides read from the runtime | the SDK, in Python: `TRLConfig.to_args()`, passed through unchanged as `args` |
| Go needed | the existing branch | none |

So supporting TRL adds no logic to the controller.

#### New artifacts

Two, mirroring what TorchTune already has. No extra script or binary in the image — the command
is torchrun invoking TRL's training module.

**1. `cmd/trainers/trl/`** — `Dockerfile` on the same PyTorch base as `cmd/trainers/torchtune`,
installing pinned `trl` + dependencies. accelerate is among them — TRL requires it
(`accelerate>=1.4.0`) and HF `Trainer` cannot be imported without it — but it is used only
in-process; its launcher is never invoked.

**2. `manifests/base/runtimes/trl/`** — a **single** `ClusterTrainingRuntime`, `trl-distributed`.
It keeps the same two initializer replicatedJobs as the TorchTune runtimes, but pins no
`STORAGE_URI`; the TrainJob supplies both. They are omitted here:

```yaml
kind: ClusterTrainingRuntime
metadata:
  name: trl-distributed
  labels:
    trainer.kubeflow.org/framework: trl   # the SDK's discovery key
spec:
  mlPolicy: {numNodes: 1, torch: {}}      # engages the torch plugin; PET_* injection
  # ...
                    - name: node
                      image: ghcr.io/kubeflow/trainer/trl-trainer
                      # torchrun reads PET_* natively; the trl CLI and
                      # accelerate launch are never invoked.
                      command: [torchrun, -m, trl.scripts.sft]
                      args:
                        - --model_name_or_path=/workspace/model
                        - --dataset_name=/workspace/dataset
                        - --output_dir=/workspace/output
```

##### Why one runtime, not one per model family

TorchTune ships one runtime per base model (`llama3_2`, `qwen2_5`) because its recipes and
configs *are* per model: `tune run` selects a recipe and a model-specific config file, so the
model family is baked into the runtime.

TRL has no such axis. Its scripts are model-agnostic, and the model and dataset are ordinary
flags the SDK sets per job. Mirroring the TorchTune layout would therefore produce N runtimes
that differ only in a default flag value. The method is not an axis either: the manifest's
`sft` module is the raw-YAML default, and an SDK-submitted `TrainJob` carries
`command: [torchrun, -m]` with the method's module as the first entry of `args`
(`TRLConfig.to_args()`), so one runtime serves SFT, DPO and GRPO alike.

A runtime is still required, because the image must carry `trl` and its dependencies; the
default `torch-distributed` runtime cannot run TRL.

If a genuine axis appears later, the candidates are the *compute profile* (CPU, single-GPU,
multi-GPU/vLLM for GRPO rollouts) rather than the model family. That would be additive: a
second runtime with the same framework label, selected with `train(runtime=...)`.

**Relationship to [#3718](https://github.com/kubeflow/trainer/pull/3718).** That PR proposes a
per-method GRPO runtime (`cmd/trainers/grpo/`, `manifests/base/runtimes/grpo/`) with its own
`train.py`. This KEP supersedes that layout: one `trl-trainer` image and one `trl-distributed`
runtime cover GRPO through `trl.scripts.grpo`, with no bespoke training script to maintain. The
GRPO work in #3718 lands as the `TRLGRPOConfig` and the GRPO E2E test rather than as a separate
image.

**The server side stops here** — an image and a manifest, no control-plane code.

### Client Side (`kubeflow/sdk`)

#### Current limitations

`BuiltinTrainer` has a single field, `config: TorchTuneConfig`. TorchTune is hardcoded at four
points:

| # | Coupling | Location |
|---|---|---|
| 1 | `BuiltinTrainer.config` annotated with the concrete `TorchTuneConfig` | `types.py:241` |
| 2 | Framework identifier derived by reflecting on that annotation (`BuiltinTrainer.__annotations__["config"].__name__`) | `types.py:245` |
| 3 | `trainer_type` and entrypoint selected by string-comparing the label against it | `utils.py:125-129`, `:152-159` |
| 4 | Config-to-argument translation guarded by `isinstance(..., TorchTuneConfig)` | `utils.py:465` |

Coupling #3 is the one that blocks everything else. `trainer_type` is not a field on the
Runtime CR; the SDK computes it as `BUILTIN_TRAINER if framework == TORCH_TUNE else
CUSTOM_TRAINER`. A runtime labelled `trl` therefore resolves to `CUSTOM_TRAINER` today, and the
SDK would try to build a training-function command for it. Until `get_runtime_trainer()`
changes, no config-driven framework but TorchTune can run.

#### `FrameworkConfig` and the registry

A config knows three things nothing else knows: the framework label it claims, the entrypoint
that runs it, and how it renders itself as that entrypoint's arguments. Those are all four
couplings above, moved onto one object.

```python
@dataclass(kw_only=True)
class FrameworkConfig(abc.ABC):
    framework: ClassVar[str] = ""
    command: ClassVar[tuple[str, ...]] = ()

    @abstractmethod
    def to_args(self) -> list[str]:
        """Render this config as arguments for `command`."""
```

`to_args()` renders only the config's own fields and takes no initializer: configs stay plain
dataclasses, so nothing in `types.py` needs to know how a backend stages data.

The framework label resolves through a registry rather than a constant. It serves exactly one
lookup — `get_runtime_trainer()`'s — and is the out-of-tree extension path:

```python
# kubeflow/trainer/types/registry.py
_FRAMEWORK_CONFIGS: dict[str, type["FrameworkConfig"]] = {}

def register_framework(cls):
    """Claim the framework label declared in cls.framework."""
    if not (cls.framework and cls.command):
        raise ValueError(f"{cls.__name__} must declare a framework and a command")
    _FRAMEWORK_CONFIGS[cls.framework] = cls
    return cls

def get_framework(framework: str) -> Optional[type["FrameworkConfig"]]:
    return _FRAMEWORK_CONFIGS.get(framework)
```

An explicit decorator, matching how the control plane registers its plugins by hand in
`plugins/registry.go` ([sdk#310](https://github.com/kubeflow/sdk/pull/310) is the PoC for this
shape). There is no automatic discovery: a config must be imported before it can be
constructed, and importing it registers it.

`TorchTuneConfig` keeps every field and its signature, gaining `framework = "torchtune"`,
`command = ("tune", "run")`, and a `to_args()` delegating to the existing emitter.

#### `TRLConfig` and the per-method configs

`TRLConfig` carries the fields shared by every TRL method, and one typed slot — `method` —
holding the config for the method being run. The subclass in that slot selects the script
module.

```python
# kubeflow/trainer/types/trl.py

@dataclass(kw_only=True)
class TRLMethodConfig(abc.ABC):
    """Per-method TRL config. The subclass selects trl.scripts.<module>."""
    module: ClassVar[str]

    @abstractmethod
    def to_args(self) -> list[str]:
        """Render the method-specific flags."""


@dataclass(kw_only=True)
class TRLSFTConfig(TRLMethodConfig):
    module: ClassVar[str] = "sft"
    loss_type: Optional[Literal["nll", "dft", "chunked_nll"]] = None
    packing: Optional[bool] = None
    completion_only_loss: Optional[bool] = None


@dataclass(kw_only=True)
class TRLDPOConfig(TRLMethodConfig):
    module: ClassVar[str] = "dpo"
    beta: Optional[float] = None                    # deviation from the reference model
    loss_type: Optional[list[str]] = None           # combinable; weighted by loss_weights
    loss_weights: Optional[list[float]] = None


@dataclass(kw_only=True)
class TRLGRPOConfig(TRLMethodConfig):
    module: ClassVar[str] = "grpo"
    beta: Optional[float] = None                    # KL coefficient
    loss_type: Optional[Literal[
        "grpo", "dr_grpo", "dapo", "bnpo", "cispo", "sapo", "luspo", "vespo"]] = None
    num_generations: Optional[int] = None
    # Upstream reward_funcs lives on GRPOScriptArguments, not GRPOConfig;
    # merged here on purpose so one object covers the method's whole surface.
    reward_funcs: Optional[list[str]] = None


@register_framework
@dataclass(kw_only=True)
class TRLConfig(FrameworkConfig):
    framework: ClassVar[str] = "trl"
    # torchrun launches TRL's plain training script as a module; to_args()
    # supplies the module name as the first positional argument, so the full
    # argv is `torchrun -m trl.scripts.<module> --flag value ...`.
    command: ClassVar[tuple[str, ...]] = ("torchrun", "-m")

    method: TRLMethodConfig                         # the typed slot

    # No model or dataset fields: both come from train(initializer=...),
    # exactly as for TorchTuneConfig.
    learning_rate: Optional[float] = None
    num_train_epochs: Optional[int] = None
    per_device_train_batch_size: Optional[int] = None
    bf16: Optional[bool] = None

    use_peft: Optional[bool] = None
    lora_r: Optional[int] = None
    lora_alpha: Optional[int] = None
    lora_target_modules: Optional[list[str]] = None

    # Kubeflow placement, never rendered as flags. Mirrors TorchTuneConfig.
    num_nodes: Optional[int] = None
    resources_per_node: Optional[dict] = None

    # Escape hatch for the shared TRL flags this class does not type.
    extra_args: Optional[dict[str, str]] = None

    def to_args(self) -> list[str]:
        """[trl.scripts.<module>, --flag, value, ...]: the module name first
        (torchrun's -m positional), then shared flags, the method's own, then
        extra_args. Bools become store_true flags, lists expand after their
        flag. num_nodes and resources_per_node are never rendered."""
        return (
            [f"trl.scripts.{self.method.module}"]
            + self._shared_args()
            + self.method.to_args()
            + self._extra_args()
        )
```

##### Why the shared fields sit on the outer config

The fields on `TRLConfig` are the ones TRL itself shares. `SFTConfig`, `DPOConfig` and
`GRPOConfig` all inherit HF `TrainingArguments`, and TRL's own `GRPOConfig` docstring says it
"includes only the parameters that are specific to GRPO training. For a full list of training
arguments, please refer to the `TrainingArguments` documentation." Splitting along the same
seam means each field is declared once, and the split matches upstream rather than inventing
one.

The split is also lopsided, which is what makes it worth making once: **161 flags are shared**
(`TrainingArguments` 133, `ModelConfig` 19, `ScriptArguments` 6, `DatasetMixtureConfig` 3),
against 11 to 79 that are method-specific, GRPO being the 79. Under a per-method design those
161 would be re-frozen in every exported constructor.

The model and dataset are not fields at all; see
[Where the model and dataset come from](#where-the-model-and-dataset-come-from).

`num_nodes` and `resources_per_node` are not TRL flags at all — they are Kubeflow placement
knobs, and they sit here because `TorchTuneConfig` already carries them and `BuiltinTrainer` has
no fields of its own. `to_args()` never renders them.

`extra_args` is the escape hatch for the shared flags this class does not type. 161 is too many
to enumerate, and a user who needs `--gradient_accumulation_steps` should not have to wait for an
SDK release. It is rendered last, so it can also override anything above it.

##### Where the model and dataset come from

From `train(initializer=...)`, and nowhere else — exactly as for TorchTune, whose config has no
model or dataset fields either. The SDK writes the `Initializer` into `TrainJob.spec.initializer`
as it does today, the runtime's initializer jobs download into `/workspace`, and the backend
appends `--model_name_or_path=/workspace/model` and `--dataset_name=/workspace/dataset/<subpath>`
after `to_args()`. The subpath derivation already exists for TorchTune's `dataset.data_files=`
override and becomes a shared helper rather than a second copy.

Three reasons for having no string fields on the config:

- **One habit for both frameworks.** A TorchTune user already knows how to give TRL a model.
- **A string is ambiguous.** `Qwen/Qwen2.5-0.5B` is a hub id, `/mnt/data/model` is a folder,
  `s3://bucket/model` needs credentials. The SDK would have to guess. `Initializer` already knows,
  and already covers S3, the data cache, and secrets for gated models.
- **No tie-break rule.** Two ways to name a model need a rule for when both are set. One way
  does not.

A `TRLConfig` submitted without an `Initializer` is a `ValueError` in the SDK, since the
runtime's initializer jobs would otherwise start with no `STORAGE_URI` and fail late. A shortcut
such as `Initializer.from_hf(model=..., dataset=...)` can come later and would help TorchTune
users equally; it is not part of this KEP.

#### Choosing the config shape

Four shapes were considered. The one above is C. The full comparison, with the upstream field
counts behind it, is in the [design note](https://claude.ai/code/artifact/29468417-11ca-42d8-a8d2-b23c0939a14c).

| | **A. Flat + method enum** | **B. Per-method configs** | **C. Typed slot** (chosen) | **D. C + shared model/dataset specs** |
|---|---|---|---|---|
| Call | `TRLConfig(method=TRLMethod.GRPO, beta=0.04)` | `BuiltinTrainer(config=TRLGRPOConfig(...))` | `TRLConfig(method=TRLGRPOConfig(...), ...)` | four nested objects |
| Objects per job | 1 | 1 | 2 | 4 |
| Wrong-method parameter | accepted, then **silently dropped by TRL** | `TypeError` at construction | `TypeError` at construction | as C |
| Conflicting field types | **cannot be typed** | each declared once, correctly | each declared once, correctly | as C |
| The 161 shared flags | one signature | **re-frozen in every export** | one signature | one signature |
| Registry | one config per framework ✓ | **N configs claim one label** | one config per framework ✓ | one config per framework ✓ |
| Adding a method | one enum member + one map entry | one dataclass + a new export + registry change | one dataclass | one dataclass |

**Why A cannot be typed.** The same parameter name has a different type and different valid
values per method, verified against TRL's config sources:

| Field | `SFTConfig` | `DPOConfig` | `GRPOConfig` |
|---|---|---|---|
| `loss_type` | `str \| None`; `'nll'`, `'dft'`, `'chunked_nll'` | `list[str]`, default `["sigmoid"]`; 15 values, combinable | `str`, default `"dapo"`; `'grpo'`, `'dr_grpo'`, `'dapo'`, `'bnpo'`, `'cispo'`, … |
| `beta` | — | `float = 0.1`, deviation from the **reference model** | `float = 0.0`, **KL coefficient** |
| `shuffle_dataset` | `bool = False` | — | `bool \| None = True` |

A flat dataclass must pick one annotation for `loss_type`, so it is either wrong for two
methods or degraded to `Any`. And `beta` is one name for two different quantities with
different defaults, which no annotation can disambiguate.

**Why the type error matters here specifically.** TRL's scripts call
`parser.parse_args_and_config(fail_with_unknown_args=False)`, so an argument that does not
belong to the method being run is **ignored, not rejected**. A user who sets `num_generations`
on an SFT job gets a successful run that silently ignored it. Shape A pushes that detection
nowhere; the typed slot turns it into a `TypeError` before the TrainJob is created.

The current draft's `_METHOD_SCOPED_FIELDS` map was an attempt to recover this at runtime. It
has to be maintained either way, so expressing it as subclasses removes a hand-maintained map
rather than adding one.

**Why not B.** It reads best at the call site and is closest to TRL's own shape, but the registry
is keyed one config per framework label, and three classes claiming `trl` would need it redesigned
to represent one framework's internal methods. The 161 shared flags would also be copied into
every public signature, so adding one shared field would touch three frozen constructors, then
six, then eighteen — or they get hoisted into a shared base, at which point the base *is* the
outer config, without the single entry point.

**Why not D.** Grouping the shared fields further into model, dataset and training specs is
tempting, but the surface is shared only as a concept: TRL wants `--dataset_name`, LlamaFactory
wants `--dataset`, and TorchTune wants Hydra overrides like `model.lora_rank=16` and could not use
the block at all. It also costs four objects per job for one framework, and it is a one-way door —
moving fields into `config.model` later is a breaking change.

**Precedent.** The typed slot is the ordinary shape for "one of N variants, plus shared
settings": Keras `compile(optimizer=...)` takes an `Optimizer` instance with the shared
hyperparameters on the base class; `tf.data.Options` holds nested typed option objects
(`autotune`, `threading`, `experimental_optimization`); scikit-learn meta-estimators take
estimator *objects* as parameters (`GridSearchCV(estimator=..., cv=...)`); and in the Trainer's
own API, `MLPolicy` carries an `MLPolicySource` union whose member selects the behaviour. TRL
itself is organized this way, one config class per trainer.

##### Naming the slot: `method`

Both `method` and `trainer` were proposed. This KEP uses **`method`**, because the user already
writes `train(trainer=BuiltinTrainer(...))`, so a nested `trainer=` inside that call would use
one word for two different things. "Method" is also the word the ecosystem uses for the SFT /
DPO / GRPO axis — TRL's README calls them "fine-tuning methods" reached "via trainers" — and it
matches how sibling KEPs name a variant slot after the concept
(KEP-3562's `algorithm=RandomSearch()`).

The classes are `TRLSFTConfig` rather than `SFTConfig` for two more reasons: it matches the
existing `TorchTuneConfig` naming, and it avoids shadowing `from trl import SFTTrainer` in the
same notebook. It also lets `LlamaFactorySFTConfig` land later without asymmetry.

#### Wiring it up

`BuiltinTrainer` stays exactly where it is and keeps its construction signature; only its
`config` field widens from `TorchTuneConfig` to `FrameworkConfig`. Both entry points then
accept any framework:

```python
BuiltinTrainer(config=TorchTuneConfig(...))   # today, still valid
BuiltinTrainer(config=TRLConfig(...))         # new, same machinery
```

Three call sites change, and each one *loses* a branch:

- **`get_runtime_trainer()` resolves the command through the registry**, replacing the
  `framework == TORCH_TUNE` branch at `utils.py:152-159` and deleting
  `constants.TORCH_TUNE_COMMAND`:

  ```python
  if config_cls := registry.get_framework(framework):
      trainer.set_command(config_cls.command)
  elif ml_policy.torch is not None:
      trainer.set_command(constants.TORCH_COMMAND)
  # ... mpi, default unchanged
  ```

- **`trainer_type` comes from the registry**: `BUILTIN_TRAINER` when `get_framework()` finds a
  registered config, `CUSTOM_TRAINER` otherwise — replacing `utils.py:125-129` and deleting the
  `TORCH_TUNE` constant. The framework label stays the sole discovery key.

- **The backend has no framework branch.** One line —
  `trainer_cr.args = trainer.config.to_args()` — deletes the `isinstance` check at
  `utils.py:465` rather than relocating it. `command` still comes from the runtime, exactly
  as today.

`RuntimeTrainer.command` stays the field consumers read. It is not config-specific:
`command[0] == "mpirun"` drives MPI detection, `get_runtime_packages` rewrites it for
single-process, and the localprocess backend joins it into an entrypoint. Moving it onto the
config would special-case all of those.

Adding a further framework then adds no lines to `utils.py` and none to the backends. That is
the one-time change this KEP is buying.

Two details preserved from the TorchTune path:

- **Initializer-derived path arguments stay in the backend.** The `dataset.data_files=` /
  `data_dir=` overrides for TorchTune and the `--model_name_or_path` / `--dataset_name` paths for
  TRL are both staging knowledge the backend owns; they are appended to whatever `to_args()`
  renders, through one shared subpath helper.
- **`TorchTuneConfig.to_args()` delegates to the existing emitter.** `get_args_from_peft_config`
  maps `LoraConfig` onto nested `model.*` keys; a flat `key=value` walk would emit `lora_rank=8`
  instead of `model.lora_rank=8` and fail the job. The emitters move verbatim to
  `types/torchtune.py` — they hold no Kubernetes logic, and leaving them in the backend would
  make `types.py` import a backend module.

`LoraConfig` is **not** reused for TRL. It is TorchTune-shaped — `apply_lora_to_output` and
`quantize_base` have no TRL analogue, and TRL's PEFT surface is `--use_peft` / `--lora_r` /
`--lora_alpha` / `--lora_target_modules`. Sharing it would need a lossy translation.

#### Which framework, and why TRL

| Framework | Post-training methods | Maintenance | Argument shape | torchrun path |
|---|---|---|---|---|
| TRL | one plain script per method under `trl/scripts/` — `sft`, `dpo`, `grpo`, `kto`, `reward`, `rloo`; PPO was moved to `trl.experimental` ([trl#4466](https://github.com/huggingface/trl/issues/4466)) and has since been removed entirely | Active (Hugging Face) | flags | native: `torchrun -m trl.scripts.<method>` |
| TorchTune | SFT only, in the Kubeflow integration | Stopped 15 Jul 2025 ([#2883](https://github.com/meta-pytorch/torchtune/issues/2883)) | flags | via `tune run` (plugin rewrites the command) |
| LlamaFactory | Broad, but built on HF `Trainer` and PEFT | Active | config file | wrapped: needs `FORCE_TORCHRUN=1` and renamed env |
| Axolotl | Broad, but its GRPO is TRL's `GRPOTrainer` | Active | config file | native with `--launcher torchrun` (default is accelerate) |
| Unsloth | None of its own; an acceleration layer | Active | n/a | n/a — not a launcher |

TRL is selected on three grounds. Its per-method scripts cover what the SDK cannot express at
all today — DPO and GRPO. It is the engine rather than a wrapper: LlamaFactory and Axolotl are
both built on the HF `Trainer` and PEFT, so an in-tree TRL trainer adds capability where an
in-tree wrapper would add a second surface over the same engine. And its scripts take flags, so
`TRLConfig` renders straight into `args`; a file-first framework would need the SDK to stage a
ConfigMap or volume first.

Choosing TRL first closes no doors: every alternative reaches the SDK out of tree as a
`FrameworkConfig` subclass, which is why LlamaFactory is the reference out-of-tree framework.
An Unsloth-accelerated image is still labelled `framework: trl`, so it resolves to the same
config and is chosen with `train(runtime=...)` — no SDK field.

### Migration and Backward Compatibility

**Server side.** Additive; no existing code path is touched.

| Aspect | Impact |
|---|---|
| CRDs | **No change.** No new or modified fields, no version bump. |
| Torch plugin | **No change.** Neither `EnforceMLPolicy` nor `Validate` is touched; TRL falls through the default path. A unit test asserts it stays that way. |
| Plugin registry | **No change.** TRL reuses the torch plugin via `mlPolicy.torch`. |
| Existing TorchTune runtimes | **No change.** Same images, manifests, rendered `TrainJob`s. |
| Default manifests | **Additive.** A new runtime under a new name; nothing renamed or removed. |

**Client side.** Additive.

| Aspect | Impact |
|---|---|
| `CustomTrainer`, `CustomTrainerContainer` | **No change.** All fields retained. |
| `BuiltinTrainer` | **Construction signature unchanged**; produces byte-identical `TrainJob` arguments for TorchTune. `config` widens to `FrameworkConfig`. Nothing deprecated. |
| `TorchTuneConfig` | **No change** to fields or signature; gains two `ClassVar`s and `to_args()`. |
| `TrainerClient.train()` | **No change** to the signature. |
| `TrainJobTemplate` | **No change.** |
| Python version | No new floor. `kw_only=True` needs 3.10, already the SDK's minimum. |
| Public exports | New names only: `FrameworkConfig`, `TRLConfig`, `TRLMethodConfig`, `TRLSFTConfig`, `TRLDPOConfig`, `TRLGRPOConfig`. Nothing removed or renamed. |

### Test Plan

**Server-side Go unit tests** (`torch_test.go`, extending the table-driven cases). Two cases,
asserted alongside each other so the paths stay visibly distinct: a
`[torchrun, -m, trl.scripts.sft]` command is left alone and gets all five `PET_*`, including
the master vars the TorchTune path suppresses; a `[tune, run]` command is still rewritten. The
first is the regression guard for
[What changes in the plugin](#what-changes-in-the-plugin) — it fails the moment anyone adds a
TRL branch to the plugin.

**Manifest tests** — `manifests/base/runtimes/trl/` carries `framework: trl`,
`mlPolicy.torch: {}`, `command: [torchrun, -m, trl.scripts.sft]`, and mounts `initializer` at
`/workspace`.

**Launch-path test** — the proof script from the [PoC branch](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md), moved into
`examples/trl/` as part of S1: two `torchrun -m trl.scripts.sft` launches with only the five
`PET_*` variables set must form world size 2, asserted by the per-step epoch value
(`0.0588` → `0.1111`); the single-node control run guards the assertion itself. Re-run on every
`trl` version bump in `requirements.txt`.

#### Unit Tests

Client side:

- `to_args()` for each of `TRLSFTConfig`, `TRLDPOConfig`, `TRLGRPOConfig`: the module name is
  the first element; bools render as store_true flags; lists expand after their flag.
- Passing a method-specific parameter to the wrong method config raises `TypeError`.
- A `TRLConfig` submitted without an `Initializer` raises `ValueError`; with one, the rendered
  args carry the `/workspace` model and dataset paths.
- `num_nodes` and `resources_per_node` never appear in `to_args()` output; `extra_args` renders
  last.
- `register_framework` rejects a config declaring no framework or no command.
- `get_runtime_trainer()` resolves `trainer_type` and `command` from the registry for both
  `torchtune` and `trl`, and falls through to torch/mpi/default for unregistered labels.
- `TorchTuneConfig.to_args()` emits nested `model.*` keys for `LoraConfig`, matching the
  current emitter byte for byte.

#### Integration tests

- With the `trl-distributed` runtime installed, `BuiltinTrainer(config=TRLConfig(...))` resolves
  it by label, yields `trainer_type == BUILTIN_TRAINER`, and emits `command: [torchrun, -m]`
  with `args == config.to_args()` (module name first). This is the **cross-repo contract**: it
  fails if either side changes the label or the launch shape.
- `BuiltinTrainer(config=TorchTuneConfig(...))` produces byte-identical `TrainJob` arguments to
  the current implementation.

#### E2E tests

- A two-node TRL `TrainJob` reaches `Complete` **and** its logs show the two-node epoch
  increment. Asserting completion alone would have passed for the accelerate failure in
  [Limitation: the TRL CLI and `accelerate launch`](#limitation-the-trl-cli-and-accelerate-launch);
  the world-size assertion is the part with teeth. Runs on CPU — no GPU gate needed for the
  launch path.
- `dpo` and `grpo` exercised end to end, not only `sft` (Beta).

**Backward compatibility** — every existing `CustomTrainer`, `BuiltinTrainer` and
`TrainJobTemplate` test passes unmodified.

### Implementation Plan

The two sides are independent. The server side ships first because it is smaller and because
the client-side integration tests need a TRL runtime to run against.

**Server side**

| Phase | Contents |
|---|---|
| S1 | `cmd/trainers/trl/` with pinned versions and the launch-path proof script, both promoted from the [PoC branch](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md); image publishing in the existing workflow |
| S2 | `manifests/base/runtimes/trl/` + kustomization entry, manifest tests, an `examples/trl/` TrainJob |
| S3 | The `torch_test.go` regression cases, then the two-node cluster E2E (CPU is sufficient — the launch path is device-agnostic) |

S1 and S2 must ship together: a manifest referencing an unpublished image is untestable, and an
image with no manifest is unreachable.

**Client side**

| Phase | Contents |
|---|---|
| C1 | `FrameworkConfig`, the registry, `TorchTuneConfig` migrated onto it, `BuiltinTrainer.config` widened, the three call-site changes; unit tests. No user-visible change yet. |
| C2 | `TRLConfig` and the three method configs; public exports; integration tests including the cross-repo contract |
| C3 | Docs on sdk.kubeflow.org and an operator guide for building a runtime image for a new framework |

C1 is a pure refactor with no behaviour change, so it can merge on its own; C2 is what makes
TRL reachable from Python.

### Graduation Criteria

**Alpha**

- *Server:* the `trl-trainer` image is published and the `trl-distributed` runtime installs by
  default; a two-node TRL job completes with `world_size == numNodes × numProcPerNode`, verified
  in E2E rather than from job status; the torch plugin is unchanged, with a test asserting it.
- *Client:* `FrameworkConfig` and the registry are implemented; `TorchTuneConfig` runs through
  them with byte-identical output; `TRLConfig` with SFT is exported and works end to end; all
  existing tests pass.

**Beta**

- `dpo` and `grpo` exercised end to end, not only `sft`.
- An out-of-tree config published **paired with an out-of-tree runtime image**, proving both
  halves of the extension path (LlamaFactory is the reference — torchrun-wrapped rather than
  torchrun-native).
- The torchrun-first launch pattern documented on kubeflow.org as operator guidance for building
  a runtime image.
- SDK reference docs for `TRLConfig` and the method configs.

**GA**

- `TRLConfig` stable for a release, with the TRL version pin exercised by CI on every bump.
- At least one community `FrameworkConfig` subclass in the wild.
- A runtime can report its achieved world size to `TrainJob` status, so a silently degraded run
  is observable (see [Open Questions](#open-questions)).

## Open Questions

1. **Should a runtime report its achieved world size?** The largest gap left open, and not
   TRL-specific. The `trl-demo` failure exited 0 on both pods, so a two-node job that trained
   two isolated copies was indistinguishable from one that trained jointly. torchrun-first
   removes that particular cause but not the class: any image whose launch degrades silently
   produces the same signature. A minimal mechanism: rank 0 reports the world size it joined
   with, and the controller surfaces it as a status condition, failing the job when it disagrees
   with `numNodes × numProcPerNode`. That is a control-plane change, out of scope here — but
   without it, "the TrainJob succeeded" is not evidence that distributed training happened.
   [KEP-2779](../2779-trainjob-progress/README.md) already establishes a runtime→status channel
   this could reuse.
2. **Does dynamic registration extend to the control plane?** *Partly answered.* The registry is
   SDK-side and stays there — the server side shows a new runtime needs no control-plane
   registration at all, only an image and a manifest. Open: whether the Trainer should
   *provision* runtimes for frameworks it does not ship (e.g. from a catalog CR).
3. **Should accelerate get its own plugin later?** The launcher-only options in
   [What torchrun gives up](#what-torchrun-gives-up) — an accelerate config file per job, the
   Megatron-LM plugin, TPU — are the reasons the question will return. The position from the SDK
   call was that a dedicated accelerate plugin is not objectionable in principle; the ask was to
   understand the torchrun limitations first, which this KEP now records. It is deliberately
   kept separate from shipping TRL, along with dynamic registration (see #2): both would need
   their own design discussion, and neither blocks TRL.
4. **Flat exports, or a `kubeflow.trainer.trl` submodule?** `TRLSFTConfig` in the existing flat
   `__all__` matches today's `__init__.py`; a submodule keeps the top level at one name per
   framework. Frozen once shipped, so it needs a decision before the first release rather than
   after.
5. **Do we expose experimental TRL methods?** PPO is the cautionary case: it was moved to
   `trl.experimental` ([trl#4466](https://github.com/huggingface/trl/issues/4466)) and has since
   been removed from the package entirely. Exporting a config for a method upstream later drops
   leaves us holding a public name. The proposal is to export only methods with a stable
   `trl/scripts/` module, and if experimental ones are ever wanted, to put them in a namespace
   carrying no stability promise.
6. **Does a compute-profile runtime axis appear for GRPO?** GRPO rollouts can use a vLLM server,
   which may justify a second TRL runtime. Deferred until there is a working GRPO E2E to measure.

## Implementation History

- **2026-02-11**: KEP drafted in `kubeflow/sdk` as `docs/proposals/285-specialized-trainers`,
  scoped SDK-only.
- **2026-08-16**: transferred to `kubeflow/trainer` as KEP-2839 and extended with the server
  side ([#3930](https://github.com/kubeflow/trainer/pull/3930)), superseding
  [#3263](https://github.com/kubeflow/trainer/pull/3263).
- **2026-08-23**: proof of concept built on the `poc/trl-torch-plugin` branch (pinned at
  [`01b96f3`](https://github.com/YassinNouh21/trainer/tree/01b96f38608467d96acfc01c7500d49ebeca5d6f)); torchrun confirmed as the
  launcher and the `accelerate launch` route recorded as a limitation.
- **2026-08-30**: config-shape options compared in a [design note](https://claude.ai/code/artifact/29468417-11ca-42d8-a8d2-b23c0939a14c); the typed slot chosen.
- **2026-09-09**: model and dataset come from `Initializer` only, as for TorchTune; no string
  fields on `TRLConfig`.
- **2026-09-02**: reviewed on the Kubeflow Trainer community call. The client-side design here
  supersedes the parallel SDK proposal in
  [sdk#627](https://github.com/kubeflow/sdk/pull/627), which covered the same ground. Agreed: restructure to the
  community KEP template with the server and client designs separated; drop the general SDK
  trainer hierarchy inherited from sdk#285; give `TRLConfig` a typed per-method slot instead of
  a flat config with a method enum; do **not** mirror TorchTune's per-model-family runtime
  layout, since the model family is not a TRL axis.

## Drawbacks

- **A second config-driven framework is a second thing to keep working.** The TRL version pin
  has to track upstream, and TRL moves fast enough to have relocated PPO in a minor release. The
  mitigation is that the pin lives in an image and a CI-exercised proof script, not in the
  control plane.
- **The SDK gains an indirection.** `BuiltinTrainer.config` is no longer one concrete type, so
  reading the code requires following the registry to find what a label resolves to. The
  alternative is a branch per framework in shared code, which is what this replaces.
- **Two repositories must stay in step** on the framework label and the launch shape. One
  integration test asserts the contract, but it can only run where both are installed.

## Alternatives

### A. A flat `TRLConfig` with a method enum

The shape in the previous draft: `TRLConfig(method=TRLMethod.GRPO, beta=0.04, ...)` with a
`_METHOD_SCOPED_FIELDS` map validating at construction. Rejected because `loss_type` and `beta`
have conflicting types and meanings across methods, so the flat class cannot be typed, and
because the runtime map has to be hand-maintained anyway. See
[Choosing the config shape](#choosing-the-config-shape).

### B. Top-level per-method configs

`BuiltinTrainer(config=TRLGRPOConfig(...))`, with no outer TRL config. Reads best at the call
site, but three classes would claim the `trl` label in a registry keyed one config per framework,
and the 161 shared flags would be re-frozen in every public signature or hoisted into a base that
then *is* the outer config. See [Choosing the config shape](#choosing-the-config-shape).

### D. Shared model, dataset and training specs

Grouping the shared fields into nested `ModelSpec` / `DatasetSpec` objects. Rejected because the
surface is shared only as a concept — TRL, LlamaFactory and TorchTune each name these differently,
and TorchTune's Hydra overrides could not use the block at all — and because it is a one-way door:
moving a field into `config.model` later is a breaking change.

### One trainer class per framework (`TRLTrainer`, `TorchTuneTrainer`)

The trainer does nothing different per framework — it hands `command` and `to_args()` to the
backend — so a subclass per framework would encode in the type hierarchy a distinction that
exists only in data. It also makes every framework a new exported class name and frozen public
API, and forces `BuiltinTrainer` onto a deprecation path toward a `TorchTuneTrainer` successor
for no behaviour change. *What it gets right:* typed per-framework fields and a flatter call.
The accepted design keeps the typed fields — on the config — at the cost of one level of
nesting.

### One runtime per base model, mirroring TorchTune

Rejected because TorchTune's per-model runtimes exist to select per-model recipes and configs,
and TRL has no equivalent: its scripts are model-agnostic and the model is a flag. See
[Why one runtime, not one per model family](#why-one-runtime-not-one-per-model-family).

### A dedicated TRL plugin in the control plane

A `pkg/runtime/framework/plugins/trl/` alongside the torch plugin. Rejected: the proof of
concept shows the torch plugin already injects everything torchrun needs, so a TRL plugin would
duplicate `EnforceMLPolicy` to produce identical output, and would put the TRL version in the
controller's release cycle.

## References

**Kubeflow**

- [KEP-2170: Trainer V2 API](../2170-kubeflow-trainer-v2/README.md)
- [KEP-2401: LLM Trainer V2](../2401-llm-trainer-v2/README.md) — the TorchTune integration this
  generalizes; its "Complement Torch Plugin" section originated the `EnforceMLPolicy` TorchTune
  branch
- [KEP-2779: TrainJob progress](../2779-trainjob-progress/README.md) — the runtime→status
  channel Open Question #1 would reuse
- Tracking issue [trainer#2839](https://github.com/kubeflow/trainer/issues/2839); GRPO via TRL
  in flight in [#3508](https://github.com/kubeflow/trainer/issues/3508) /
  [#3718](https://github.com/kubeflow/trainer/pull/3718)
- Earlier drafts, superseded: [trainer#3263](https://github.com/kubeflow/trainer/pull/3263),
  [sdk#285](https://github.com/kubeflow/sdk/issues/285),
  [sdk#627](https://github.com/kubeflow/sdk/pull/627) (KEP-626, the SDK-side twin of this
  proposal), [sdk#310 registry PoC](https://github.com/kubeflow/sdk/pull/310)
- Proof of concept: [`poc/trl-torch-plugin`](https://github.com/YassinNouh21/trainer/tree/01b96f38608467d96acfc01c7500d49ebeca5d6f)
  — the TRL image, runtime manifest, example TrainJob and the launch-path proof script
- Config-shape comparison: [design note](https://claude.ai/code/artifact/29468417-11ca-42d8-a8d2-b23c0939a14c)
- [Runtime guide — the framework label](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [SDK types](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/types/types.py),
  [SDK TrainerClient](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py)

**Upstream sources cited above**

- `torch.distributed.argparse_util` — the `env` action deriving `PET_{DEST}`; why `PET_*` is
  torchrun's interface and nothing else's. `torch.distributed.run` — `auto` handling and the
  `-m` / `--module` invocation the runtime command uses.
- `trl/scripts/sft.py` — a plain script (`TrlParser` + `SFTTrainer`, `__main__` guard, no
  launcher logic); one sibling module per method; parses with `fail_with_unknown_args=False`.
  `trl/cli/accelerate_launcher.py` — `launch_training_script` resolves
  `resources.files("trl.scripts")` and calls accelerate's `launch_command`, which is why the CLI
  is accelerate-bound.
- `trl/trainer/{sft,dpo,grpo}_config.py` — the per-method `loss_type` and `beta` declarations
  quoted in [Choosing the config shape](#choosing-the-config-shape); each inherits HF
  `TrainingArguments` for the shared arguments.
- `accelerate.state.PartialState` — in-process env detection: `LOCAL_RANK != -1` → multi-GPU
  (nccl); `WORLD_SIZE > 1` on CPU → multi-CPU (gloo). This is what makes the script work under
  torchrun with no launcher flags.
- `accelerate.commands.launch` — the source behind the two *launcher* limitations recorded above;
  the line-level detail is in the
  [PoC write-up](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md).
- [trl#4466](https://github.com/huggingface/trl/issues/4466) — PPO moved to experimental; it is
  absent from `trl/trainer/` and `trl/experimental/` at main.
  [torchtune#2883](https://github.com/meta-pytorch/torchtune/issues/2883) — development halted,
  15 July 2025.
