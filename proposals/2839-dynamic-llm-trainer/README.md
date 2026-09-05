# KEP-2839: Kubeflow Dynamic LLM Trainer Framework

|                |                                                              |
| -------------- | ------------------------------------------------------------ |
| **Authors**    | @szaher, @YassinNouh21                                       |
| **Status**     | Draft                                                        |
| **Created**    | 2026-02-11                                                   |
| **Reviewers**  | @andreyvelich, @astefanutti, @kramaranya                     |
| **Supersedes** | [kubeflow/trainer#3263](https://github.com/kubeflow/trainer/pull/3263) |
| **Transferred from** | `kubeflow/sdk` `docs/proposals/285-specialized-trainers`, which was scoped SDK-only |
| **Relevant Issues** | [trainer#2839](https://github.com/kubeflow/trainer/issues/2839), [sdk#285](https://github.com/kubeflow/sdk/issues/285), [sdk#626](https://github.com/kubeflow/sdk/issues/626) (parked) |

## Table of Contents

<!-- toc -->
- [KEP-2839: Kubeflow Dynamic LLM Trainer Framework](#kep-2839-kubeflow-dynamic-llm-trainer-framework)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [Part I: Server Side (`kubeflow/trainer`)](#part-i-server-side-kubeflowtrainer)
    - [I.1 What the torch plugin already provides](#i1-what-the-torch-plugin-already-provides)
    - [I.2 `PET_*` is torchrun's private interface](#i2-pet_-is-torchruns-private-interface)
    - [I.3 Proof of concept](#i3-proof-of-concept)
    - [I.4 Limitation: the TRL CLI and `accelerate launch`](#i4-limitation-the-trl-cli-and-accelerate-launch)
    - [I.5 What changes in the plugin](#i5-what-changes-in-the-plugin)
    - [I.6 New artifacts](#i6-new-artifacts)
  - [Part II: Client Side (`kubeflow/sdk`)](#part-ii-client-side-kubeflowsdk)
    - [II.1 Current limitations](#ii1-current-limitations)
    - [II.2 `BaseTrainer`](#ii2-basetrainer)
    - [II.3 `FuncTrainer` and the framework trainers](#ii3-functrainer-and-the-framework-trainers)
    - [II.4 `ConfigTrainer`](#ii4-configtrainer)
    - [II.5 `RuntimeConfig`](#ii5-runtimeconfig)
    - [II.6 `TrainerClient` and runtime auto-discovery](#ii6-trainerclient-and-runtime-auto-discovery)
    - [II.7 `FrameworkConfig`, the registry, and `TRLConfig`](#ii7-frameworkconfig-the-registry-and-trlconfig)
    - [II.8 Which framework, and why TRL](#ii8-which-framework-and-why-trl)
  - [Migration and Backward Compatibility](#migration-and-backward-compatibility)
  - [Test Plan](#test-plan)
  - [Implementation Plan](#implementation-plan)
  - [Graduation Criteria](#graduation-criteria)
  - [Open Questions](#open-questions)
  - [Alternatives Considered](#alternatives-considered)
  - [References](#references)
<!-- /toc -->

---

## Overview

Kubeflow Trainer can run exactly one config-driven LLM framework: TorchTune, whose upstream
development stopped in July 2025. This proposal makes the framework axis extensible and
lands Hugging Face TRL as the second framework.

The work spans two repositories.

**Server side (`kubeflow/trainer`).** A framework becomes supportable by adding a runtime
image and a `ClusterTrainingRuntime`. The CRDs, the controller and the torch plugin are
unchanged — no Go at all. The torch plugin already publishes the cluster topology as
`PET_*` environment variables, and `PET_*` is torchrun's own env interface. So the TRL
runtime launches TRL's training script **with torchrun**
(`torchrun -m trl.scripts.sft`), and the topology flows through with no translation
([I.2](#i2-pet_-is-torchruns-private-interface) explains why this works). The `trl` CLI is
not used: it hands off to `accelerate launch`, which does not read `PET_*` and degrades to
single-process training while still reporting success. That failure is documented as a
limitation ([I.4](#i4-limitation-the-trl-cli-and-accelerate-launch)), and supporting
accelerate is deferred to a later phase.

**Client side (`kubeflow/sdk`).** Two additive changes: a trainer type hierarchy
(`BaseTrainer` → `FuncTrainer` / `ConfigTrainer`) with runtime auto-discovery by the
`trainer.kubeflow.org/framework` label, and a `RuntimeConfig` dataclass separating per-job
environment settings from training logic. `CustomTrainer`, `BuiltinTrainer` and
`TrainerClient.train()` keep working unchanged.

The contract between the two sides already exists and is narrow: the framework label, and
the container `command` the SDK appends rendered arguments to. Each side is independently
useful — the runtime is usable from raw YAML before any SDK change ships.

A proof of concept for the server side was built and run on a cluster; its results are in
[I.3](#i3-proof-of-concept) and they changed the design.

## Goals

**Server side**

1. Ship TRL as the second in-tree config-driven framework — a `cmd/trainers/trl/` image and
   manifests under `manifests/base/runtimes/trl/`, mirroring what TorchTune already has —
   launched with **torchrun**, the canonical launcher the torch plugin already serves.
2. Keep the control plane completely unchanged: no Go, no CRD change, no knowledge of TRL.
3. Document precisely why the `trl` CLI / `accelerate launch` route is excluded from phase
   one (it fails *silently* at world size 1), so the future accelerate discussion can start
   from measured results.

**Client side**

4. Define `BaseTrainer`, plus `FuncTrainer` (function-driven) and `ConfigTrainer`
   (config-driven), so the SDK and backends handle every trainer through one interface.
5. Implement `TorchTrainer`, `DeepSpeedTrainer`, `JAXTrainer` and `XGBoostTrainer` with
   runtime auto-discovery and validation by framework label.
6. Provide an extension point for community config-driven frameworks — a `FrameworkConfig`
   subclass in any package.
7. Introduce `RuntimeConfig`.
8. Keep 100% backward compatibility with `CustomTrainer`, `CustomTrainerContainer`,
   `BuiltinTrainer` and `TrainerClient.train()`.

## Non-Goals

1. **Any control-plane change.** No CRD or schema change, no version bump, and no Go: the
   server-side work is a new image and new manifests. `EnforceMLPolicy` and `Validate` are
   both unchanged — see [I.5](#i5-what-changes-in-the-plugin).
2. **A new framework plugin.** TRL reuses the torch plugin via `mlPolicy.torch`; no entry
   is added to `pkg/runtime/framework/plugins/registry.go`.
3. **New runtime labels or conventions.** `trainer.kubeflow.org/framework` is unchanged.
4. **Supporting `accelerate launch`.** torchrun is the only launcher in scope, and the
   plugin's existing `mlPolicy.torch` is the only mechanism used. Accelerate's launcher, and
   the launcher-level options only it offers, is a separate discussion together with
   dynamic registration. See
   [I.4](#i4-limitation-the-trl-cli-and-accelerate-launch) for why it is excluded and
   [Open Question #7](#open-questions) for what taking it up would involve.
5. **Deprecating `CustomTrainer` or `BuiltinTrainer`.** Both stay supported.
6. **In-tree Axolotl, LlamaFactory or Unsloth runtimes.** Only TRL is proposed here;
   further frameworks are follow-up work.
7. **Changes to `TrainJobTemplate`.**

---

## Part I: Server Side (`kubeflow/trainer`)

### I.1 What the torch plugin already provides

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

It also opens port 29500 for the headless service, and rejects a `TrainJob` that tries to
set any of those five variables itself.

There is exactly one framework-specific branch: when `spec.trainer.command` equals
`["tune", "run"]`, the plugin withholds `PET_MASTER_ADDR` / `PET_MASTER_PORT` and rewrites
the command with `--rdzv_endpoint=<host>:29500` plus the recipe, config and runtime-derived
overrides (`torch.go:175-201`). Every other command gets all five and no rewrite.

So a second framework inherits a complete description of the topology for free. **The only
question is whether its launcher reads it.**

### I.2 `PET_*` is torchrun's private interface

`PET_*` is not a Kubeflow convention. It is how torchrun reads its own flags from the
environment: for every flag, torchrun also accepts a `PET_`-prefixed env var
(`torch.distributed.argparse_util`), so `PET_NNODES` is simply `--nnodes`. That puts
`PET_*` at a specific layer:

```
1. Torch plugin        →  sets PET_NNODES, PET_NPROC_PER_NODE, PET_NODE_RANK,
                          PET_MASTER_ADDR, PET_MASTER_PORT
2. Launcher (torchrun) →  reads PET_*, decides how many processes to spawn
3. Launcher            →  spawns them, exports RANK, WORLD_SIZE, LOCAL_RANK,
                          MASTER_ADDR, MASTER_PORT
4. Training script     →  reads those standard variables
```

`PET_*` is the launcher's **input**; `RANK` / `WORLD_SIZE` are its **output**. A framework
that does not launch through torchrun never reaches step 2, so nothing produces step 3.

TRL's *CLI* is such a case: `trl sft` hands off to `accelerate launch`
(`trl/cli/accelerate_launcher.py` resolves the training script and calls accelerate's
`launch_command`), and **accelerate-the-launcher has no reference to `PET_` anywhere in its
source** — it is a *peer* of torchrun that takes topology from flags.

But the CLI is only a wrapper. What it wraps — `trl/scripts/sft.py` — is a **plain training
script**: `TrlParser` argument parsing plus `SFTTrainer(...).train()`, no launcher logic,
runnable as a module. And the accelerate *library* inside it (`PartialState`, a hard
dependency of TRL) detects torchrun's step-3 output from the environment: `LOCAL_RANK` set →
multi-GPU (nccl); `WORLD_SIZE > 1` on CPU → multi-CPU (gloo).

So TRL fits the diagram directly by making torchrun the launcher of TRL's own script:

```yaml
command: [torchrun, -m, trl.scripts.sft]
```

torchrun consumes the plugin's `PET_*` natively (including `numProcPerNode: "auto"`, which
it resolves itself), and the script's in-process accelerate reads what torchrun exports.
No translation of variables, no translation of commands, no plugin change — the same path
other torchrun-native frameworks in the Trainer already use, Megatron included. This is the design; the `accelerate launch`
route is a documented limitation ([I.4](#i4-limitation-the-trl-cli-and-accelerate-launch)).

### I.3 Proof of concept

Three runs — two on the cluster, one reproducible locally — all with the exact `PET_*`
variables the unmodified torch plugin injects. The only difference is what consumed them.

| Run | Where | Consumer of `PET_*` | Outcome |
|---|---|---|---|
| `probe-rendezvous` | cluster, 2 CPU nodes | bare `torchrun` | `[Gloo] Rank 1 is connected to 1 peer ranks`, then `rank=1 world_size=2 allreduce=2.0`. **Real rendezvous** from the plugin's injected env. |
| torchrun + TRL script | Linux container, 2 simulated nodes | `torchrun -m trl.scripts.sft` — the runtime's exact command | First-step `epoch` moved from `0.05882` (1/17, single-node control) to `0.1111` (1/9): the dataset sharded across 2 ranks. **World size 2**, both launches completed. |
| `trl-demo` | cluster, 2 CPU nodes | `trl sft` → `accelerate launch`, via a flag-translating entrypoint | Flags translated **correctly** per pod, yet `epoch` advanced `1/52002` per step instead of `2/52002`: **world size 1**. Both pods `Succeeded`, TrainJob `Complete` — a silent failure. |

The probe proves the plugin→torchrun half on real infrastructure: each rank contributes
`1.0` to a Gloo all-reduce, so a sum of `2.0` can only come from two processes that
exchanged data. The container run proves the torchrun→TRL half with TRL's real `sft.py`,
launched with **only** the five `PET_*` variables set — no topology flags, no CLI. The
epoch value cannot lie: 17 examples become 9 steps/epoch only when the data is split
across 2 workers, and a worker that failed to connect would hang rather than complete. The
run is committed as `examples/trl/local_torchrun_proof.sh` (needs only docker) and used
`--use_cpu`, so reproducing it — and the eventual cluster E2E — needs no GPUs.

**What is not yet proven.** The two halves were verified separately: cross-pod rendezvous on
the cluster, and the TRL script under torchrun locally. Running the TRL script across two
real pods is the remaining step, and it is the first item of the E2E phase. Nothing in the
two results suggests it will behave differently — the local run used the manifest's exact
command, and the pod-to-pod path is what the probe already exercised — but it has not been
run.

`trl-demo` is the failure case, and the reason the accelerate route is excluded from phase
one; [I.4](#i4-limitation-the-trl-cli-and-accelerate-launch) records its causes.

### I.4 Limitation: the TRL CLI and `accelerate launch`

The TRL CLI cannot be the runtime command. `trl-demo` failed for two causes, both in
accelerate's launcher and both **exiting 0** — JobSet observes completion, the TrainJob
reports `Succeeded`, and no signal reaches the control plane:

1. **`--use_cpu` disables every distributed path** *(demonstrated on the cluster)*. Each
   distributed branch in `launch_command` is guarded `... and not args.cpu`, so a CPU run
   falls through to `simple_launcher`, which never sets `RANK` / `WORLD_SIZE`.
2. **`--multi_gpu` is inferred only from local device count** *(from source)*. Accelerate
   auto-enables it when `torch.cuda.device_count() > 1` in the current process — never true
   at one GPU per pod.

Both are properties of the *launcher* and vanish when torchrun launches the script: the
container run in [I.3](#i3-proof-of-concept) used `--use_cpu` and still reached world
size 2, because in-process accelerate reads the env torchrun had already exported.

Choosing torchrun is also the pattern the Trainer already follows for other frameworks —
Megatron runs under the torchrun launcher today — so TRL joins an established path rather
than introducing a second one. Axolotl and LlamaFactory can both be launched by torchrun as
well, which is a useful signal for later frameworks but not part of this proposal.

**What torchrun gives up.** Most of what `accelerate launch` configures is also exposed by
HF `TrainingArguments`, so it survives as ordinary script flags: `--deepspeed`, `--fsdp`,
`--fsdp_config`, `--accelerator_config`, `--bf16` / `--fp16`, `--gradient_checkpointing`,
`--torch_compile`, `--ddp_backend`. DeepSpeed and FSDP are therefore **not** lost. What is
genuinely unavailable under torchrun:

| Not available | Impact |
|---|---|
| `--config_file` — one accelerate YAML per job | Cosmetic here: the same settings are reachable as script flags, and a runtime manifest is already the place operators express them. |
| `--use_megatron_lm` | The accelerate Megatron-LM plugin. Out of scope for TRL post-training. |
| `--tpu` | No TPU support in the Trainer's torch plugin either. |
| `--mixed_precision`, `--dynamo_backend` at launch time | Covered by `--bf16` / `--fp16` and `--torch_compile` on the script. |
| `--mpirun_hostfile` | Only needed because accelerate cannot do multi-process CPU without MPI. torchrun does it natively over gloo, so this is a limitation torchrun *removes*. |

**Elasticity and preemption are not lost either.** For multi-GPU and DeepSpeed,
`accelerate launch` runs torchrun itself — `import torch.distributed.run as distrib_run`,
then `distrib_run.run(args)` — and its elastic flags (`--max_restarts`,
`--monitor_interval`, `--rdzv_backend`, `--rdzv_conf`) are passed straight through to
torchrun's own parser. Calling torchrun directly gives the same behaviour with one less
layer. Elastic training is in any case not supported by the Trainer yet, for either
launcher: `torch.go:109` carries the TODO to add it once JobSet supports elastic jobs.

Supporting `accelerate launch` is **out of scope here** and left to a later discussion
([Open Question #7](#open-questions)).

**In short: we do not use accelerate's launcher — torchrun launches TRL's script directly.
accelerate is still in the image because TRL depends on it (`accelerate>=1.4.0`, and HF
`Trainer` cannot be imported without it), but in-process it only reads the variables
torchrun exported.** The distinction matters for review: what is excluded here is a second
launcher, not a library.

### I.5 What changes in the plugin

Nothing. `EnforceMLPolicy` today supports the torchrun launcher through the `PET_*`
variables, and that is what TRL needs — every run in [I.3](#i3-proof-of-concept) used an
unmodified `torch.go`. `Validate` is unchanged too.

The difference from TorchTune is where the training arguments are built:

| | TorchTune | TRL |
|---|---|---|
| Launcher | `tune run` | `torchrun` |
| Rendezvous | a command-line argument, so the plugin rewrites the command (`torch.go:175-201`) | environment, so the command is left alone |
| Who builds the arguments | the plugin, in Go: `torchtune.go` picks the recipe, adds `--config`, and appends the path overrides read from the runtime | the SDK, in Python: `TRLConfig.to_args()`, passed through unchanged as `args` |
| Go needed | the existing branch | none |

So supporting TRL adds no logic to the controller. Which module torchrun runs
(`trl.scripts.sft` vs `.dpo` vs `.grpo`) is declared in the runtime manifest and shipped in
the image, so a TRL upgrade is an image rebuild rather than a controller release.

### I.6 New artifacts

Two, mirroring what TorchTune already has. No extra script or binary in the image — the
command is torchrun invoking TRL's training module.

**1. `cmd/trainers/trl/`** — `Dockerfile` on the same PyTorch base as
`cmd/trainers/torchtune`, installing pinned `trl` + dependencies. accelerate is among them
— TRL requires it (`accelerate>=1.4.0`) and HF `Trainer` cannot be imported without it — but
it is used only in-process; its launcher is never invoked. With:

```dockerfile
ENTRYPOINT ["torchrun", "-m", "trl.scripts.sft"]
```

**2. `manifests/base/runtimes/trl/`** — one `ClusterTrainingRuntime` per base model,
mirroring `manifests/base/runtimes/torchtune/`. Initializer replicatedJobs are identical and
omitted here:

```yaml
kind: ClusterTrainingRuntime
metadata:
  name: trl-qwen2.5-1.5b
  labels:
    trainer.kubeflow.org/framework: trl   # the SDK's discovery key
spec:
  mlPolicy: {numNodes: 1, torch: {}}      # engages the torch plugin; PET_* injection
  # ...
                    - name: node
                      image: ghcr.io/kubeflow/trainer/trl-trainer
                      # torchrun reads PET_* natively; the trl CLI and
                      # accelerate launch are never invoked. Other methods
                      # swap the module: trl.scripts.dpo, trl.scripts.grpo.
                      command: [torchrun, -m, trl.scripts.sft]
                      args:
                        - --model_name_or_path=/workspace/model
                        - --dataset_name=/workspace/dataset
                        - --output_dir=/workspace/output
```

The command is not `["tune", "run"]`, so `EnforceMLPolicy` takes its default path. The
manifest's `sft` module is the raw-YAML default; an SDK-submitted `TrainJob` carries
`command: [torchrun, -m]` with the method's module as the first entry of `args`
(`TRLConfig.to_args()`, [II.7](#ii7-frameworkconfig-the-registry-and-trlconfig)), so one
runtime serves every method.

**The server side stops here** — an image and manifests, no control-plane code. One gap is
deferred rather than dismissed: none of this lets a runtime *report* its achieved world
size, so a silently degraded run (the `trl-demo` shape, in any framework) is still
indistinguishable from success at the control plane. See
[Open Question #6](#open-questions).

---

## Part II: Client Side (`kubeflow/sdk`)

### II.1 Current limitations

The SDK offers `CustomTrainer` (a flat dataclass taking `func`, `func_args`, `image`,
`packages_to_install`, `pip_index_urls`, `num_nodes`, `resources_per_node`, `env`) and
`BuiltinTrainer` (a single field, `config: TorchTuneConfig`).

| # | Limitation | Impact |
|---|---|---|
| 1 | **Missing middle abstraction** | Most workloads fall between `BuiltinTrainer` (too specific) and `CustomTrainer` (too generic) |
| 2 | **Mixed concerns in `CustomTrainer`** | Runtime env, scaling and training logic tangled in one dataclass |
| 3 | **No framework validation** | Mismatched trainer/runtime pairs fail at execution, not submission |
| 4 | **No typed framework arguments** | `max-restarts`, `deepspeed_config` have no home but the untyped `func_args` |
| 5 | **`BuiltinTrainer` is not extensible** | A new config-driven framework means changing the class |
| 6 | **No runtime auto-discovery** | `runtime=None` defaults to `torch-distributed` regardless of trainer type |

TorchTune is hardcoded at four points, and #3 is the one that blocks everything else:

| # | Coupling | Location |
|---|---|---|
| 1 | `BuiltinTrainer.config` annotated with the concrete `TorchTuneConfig` | `types.py:226-236` |
| 2 | Framework identifier derived by reflecting on that annotation | `types.py:239-240` |
| 3 | `trainer_type` and entrypoint selected by string-comparing the label against it | `utils.py:114-119`, `:140-158` |
| 4 | Config-to-argument translation guarded by `isinstance(..., TorchTuneConfig)` | `utils.py:451-452` |

`trainer_type` is not a field on the Runtime CR; the SDK computes it as `BUILTIN_TRAINER if
framework == TORCH_TUNE else CUSTOM_TRAINER`. A runtime labelled `trl` therefore resolves to
`CUSTOM_TRAINER` today and `ConfigTrainer.validate_runtime()` would reject it. Until
`get_runtime_trainer()` changes, no config-driven framework but TorchTune can run.

### II.2 `BaseTrainer`

```python
@dataclass(kw_only=True)
class BaseTrainer(ABC):
    """Common fields, runtime auto-discovery and framework validation.

    Subclasses use FuncTrainer or ConfigTrainer rather than inheriting directly.
    supported_frameworks must match `trainer.kubeflow.org/framework` values, and
    is ordered by preference — the first entry wins auto-discovery.
    """
    supported_frameworks: ClassVar[tuple[str, ...]]

    num_nodes: Optional[int] = None
    resources_per_node: Optional[dict] = None
    image: Optional[str] = None

    def __init_subclass__(cls, **kwargs: object) -> None:
        """Reject a concrete subclass that forgot to declare supported_frameworks,
        at class-definition time. Abstract intermediates (FuncTrainer,
        ConfigTrainer) are skipped."""

    def validate_runtime(self, runtime: "Runtime") -> None:
        """Raise ValueError if the runtime's framework is unsupported."""
        if runtime.trainer.framework not in self.supported_frameworks:
            raise ValueError(...)
```

- `supported_frameworks` is a `ClassVar[tuple[...]]` — immutable, class-level, ordered by
  preference. `__init_subclass__` catches a missing declaration at class-definition time.
- `@dataclass(kw_only=True)`: `BaseTrainer`'s fields have defaults, so without it
  `FuncTrainer.func` could not be required. Existing types keep their bare declarations —
  their signatures are public API.
- No shared argument-rendering method: `FuncTrainer` declares `get_framework_args()`, the
  config-driven path renders through `config.to_args()`. A dict-shaped accessor on
  `BaseTrainer` would duplicate `to_args()`.

### II.3 `FuncTrainer` and the framework trainers

```python
@dataclass(kw_only=True)
class FuncTrainer(BaseTrainer):
    """The user provides a training function, executed in the distributed
    environment the runtime configures.

    func_args should hold only user hyperparameters — rdzv_endpoint, nnodes and
    the rest are injected by the controller.
    """
    func: Callable
    func_args: Optional[dict] = None

    @abstractmethod
    def get_framework_args(self) -> dict:
        """Framework CLI/env arguments that do not overlap with controller-injected ones."""

    def get_train_func(self) -> Callable: return self.func
    def get_train_func_args(self) -> Optional[dict]: return self.func_args

    def validate_runtime(self, runtime: "Runtime") -> None:
        super().validate_runtime(runtime)
        # FuncTrainer additionally requires trainer_type == CUSTOM_TRAINER.
```

Concrete trainers add only `supported_frameworks`, typed fields, and
`get_framework_args()`:

```python
@dataclass
class TorchTrainer(FuncTrainer):
    supported_frameworks: ClassVar[tuple[str, ...]] = ("torch",)

    max_restarts: Optional[int] = None       # torchrun --max-restarts
    monitor_interval: Optional[float] = None # torchrun --monitor-interval

    def get_framework_args(self) -> dict:
        args = {}
        if self.max_restarts is not None:
            args["max-restarts"] = str(self.max_restarts)
        if self.monitor_interval is not None:
            args["monitor-interval"] = str(self.monitor_interval)
        return args
```

| Trainer | `supported_frameworks` | Typed fields | Notes |
|---|---|---|---|
| `TorchTrainer` | `("torch",)` | `max_restarts`, `monitor_interval` | |
| `DeepSpeedTrainer` | `("deepspeed", "torch")` | `deepspeed_config`, `num_proc_per_node` | Bootstraps via torchrun or mpirun, so it accepts both runtime types; when both exist the user picks explicitly. `deepspeed_config` accepts a path or a dict (JSON-encoded). |
| `JAXTrainer` | `("jax",)` | — | `get_framework_args()` returns `{}` |
| `XGBoostTrainer` | `("xgboost",)` | — | `get_framework_args()` returns `{}` |

### II.4 `ConfigTrainer`

```python
@dataclass(kw_only=True)
class ConfigTrainer(BaseTrainer):
    """Config-driven trainers take no training function. The config fully
    describes the job and the runtime's entrypoint executes it."""
    config: FrameworkConfig

    @property
    def supported_frameworks(self) -> tuple[str, ...]:
        """The framework is carried by the config, not fixed per class."""
        return (self.config.framework,)

    def validate_runtime(self, runtime: "Runtime") -> None:
        super().validate_runtime(runtime)
        # ConfigTrainer additionally requires trainer_type == BUILTIN_TRAINER.
```

A property rather than a `ClassVar`, because the framework comes from the instance's
config. The `__init_subclass__` guard does not inspect its value here — it does not need
to, since the registry already rejects a `FrameworkConfig` that declares no framework.

`ConfigTrainer` is **not** subclassed per framework. `BuiltinTrainer` stays exactly where it
is, keeping its construction signature; only its `config` field widens from `TorchTuneConfig`
to `FrameworkConfig`. Both entry points then accept any framework:

```python
BuiltinTrainer(config=TorchTuneConfig(...))   # today, still valid
ConfigTrainer(config=TRLConfig(...))          # new code, same machinery
```

| Aspect | `BuiltinTrainer` today | proposed |
|---|---|---|
| Runtime discovery | none (hardcoded) | auto via `config.framework` |
| Framework validation | none | `validate_runtime()` |
| `RuntimeConfig` support | no | yes |
| Extension model | modify the class | add a `FrameworkConfig` subclass |

### II.5 `RuntimeConfig`

```python
@dataclass
class RuntimeConfig:
    """Per-job runtime environment, separate from training-loop and scaling config.

    Passed to TrainerClient.train() and applied whatever the trainer type.
    """
    packages_to_install: Optional[list[str]] = None
    pip_index_urls: list[str] = field(
        default_factory=lambda: list(constants.DEFAULT_PIP_INDEX_URLS))
    env: Optional[dict[str, str]] = None
```

- A `@dataclass`, consistent with the rest of the SDK; field names match `CustomTrainer`'s
  and KFP's `PipelinesClient`. Pip options are flat rather than a nested `PipConfig`.
- A separate `train()` parameter rather than a trainer field: the same runtime settings
  apply whatever the trainer type, and this leaves room to apply env to initializers later
  without touching trainer classes.
- **Merge semantics** are field-level: a `RuntimeConfig` field overrides the corresponding
  `CustomTrainer` field only when it is not `None`. `RuntimeConfig(env=...)` alongside
  `CustomTrainer(packages_to_install=[...])` preserves the packages.

### II.6 `TrainerClient` and runtime auto-discovery

```python
def train(
    self,
    runtime: Optional[Union[str, "Runtime"]] = None,
    initializer: Optional["Initializer"] = None,
    trainer: Optional[Union[
        "CustomTrainer", "CustomTrainerContainer", "BuiltinTrainer",
        "BaseTrainer",                                   # NEW
    ]] = None,
    runtime_config: Optional["RuntimeConfig"] = None,    # NEW
    options: Optional[list] = None,
) -> str:
```

`_resolve_runtime()` lives on the client, not the backend, so behaviour is identical across
backends. Given an explicit `runtime`, it calls `trainer.validate_runtime()`. Given `None`,
it walks `supported_frameworks` in declaration order and takes the first framework with
matches: exactly one match is selected; zero raises listing the available runtimes; more
than one raises listing the candidates and asking for an explicit choice.

So `DeepSpeedTrainer(("deepspeed", "torch"))` selects `torch-distributed` on a cluster with
only that, and `deepspeed-mpi` without ambiguity on a cluster with both.

`runtime=None` behaviour is deliberately split: `CustomTrainer` and `BuiltinTrainer` keep
defaulting to `constants.DEFAULT_TRAINING_RUNTIME`, so existing code does not change;
`BaseTrainer` subclasses get auto-discovery.

**Validation is a hard fail, not a warning**, at three levels: framework label
(`BaseTrainer`), `trainer_type` (`FuncTrainer` / `ConfigTrainer`), and framework-specific
overrides. A `ValueError` before the CR is created beats a job that fails minutes later in
the controller. The SDK checks only what it can know for certain (the label and the trainer
type); the control plane still has the final say on quotas, policy and versions.

Trainers are **plain data, not builders**. They hold the function or config, framework
arguments, and scaling; the backend builds the `TrainJob` CR; the controller owns the
distributed topology. So a new trainer needs no backend change, and a new backend needs no
trainer change:

```python
def _build_trainer_cr(self, runtime, trainer):
    # num_nodes, resources_per_node, image copied across as today.
    if isinstance(trainer, FuncTrainer):
        trainer_cr.command = get_command_using_train_func(runtime, trainer.get_train_func(), ...)
        trainer_cr.args = [f"--{k}={v}" for k, v in trainer.get_framework_args().items()]
    elif isinstance(trainer, ConfigTrainer):
        # Entrypoint comes from the runtime, exactly as today. Each config
        # renders its own args, so there is no framework branch here.
        trainer_cr.command = list(runtime.trainer.command)
        trainer_cr.args = trainer.config.to_args()
```

### II.7 `FrameworkConfig`, the registry, and `TRLConfig`

A config knows three things nothing else knows: the framework label it claims, the
entrypoint that runs it, and how it renders itself as that entrypoint's arguments. Those are
couplings #1–#4 from [II.1](#ii1-current-limitations), moved onto one object.

```python
@dataclass(kw_only=True)
class FrameworkConfig(abc.ABC):
    framework: ClassVar[str] = ""
    command: ClassVar[tuple[str, ...]] = ()

    @abstractmethod
    def to_args(self) -> list[str]:
        """Render this config as arguments for `command`."""
```

`to_args()` renders only the config's own fields and takes no initializer: configs stay
plain dataclasses, so nothing in `types.py` needs to know how a backend stages data.

The framework label resolves through a registry rather than a constant. It serves exactly
one lookup — `get_runtime_trainer()`'s — and is the out-of-tree extension path:

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
`plugins/registry.go` ([sdk#310](https://github.com/kubeflow/sdk/pull/310) is the PoC for
this shape). There is no automatic discovery: a config must be imported before it can be
constructed, and importing it registers it.

`TorchTuneConfig` keeps every field and its signature, gaining
`framework = "torchtune"`, `command = ("tune", "run")`, and a `to_args()` delegating to the
existing emitter.

```python
# kubeflow/trainer/types/trl.py

class TRLMethod(Enum):
    """Post-training method; the value names the TRL script module
    (trl/scripts/<method>.py), launched as `torchrun -m trl.scripts.<method>`."""
    SFT = "sft"; DPO = "dpo"; GRPO = "grpo"

@register_framework
@dataclass
class TRLConfig(FrameworkConfig):
    framework: ClassVar[str] = "trl"
    # torchrun launches TRL's plain training script as a module (see I.2/I.6);
    # to_args() supplies the module name as the first positional argument, so
    # the full argv is `torchrun -m trl.scripts.<method> --flag value ...`.
    command: ClassVar[tuple[str, ...]] = ("torchrun", "-m")

    method: TRLMethod
    model_name_or_path: str
    dataset_name: str

    learning_rate: Optional[float] = None
    num_train_epochs: Optional[int] = None
    per_device_train_batch_size: Optional[int] = None
    bf16: Optional[bool] = None

    use_peft: Optional[bool] = None
    lora_r: Optional[int] = None
    lora_alpha: Optional[int] = None
    lora_target_modules: Optional[list[str]] = None

    beta: Optional[float] = None              # DPO, GRPO
    num_generations: Optional[int] = None     # GRPO
    reward_funcs: Optional[list[str]] = None  # GRPO

    # Fields valid only for a subset of methods; anything absent applies to all.
    _METHOD_SCOPED_FIELDS: ClassVar[dict[str, frozenset[TRLMethod]]] = {
        "beta": frozenset({TRLMethod.DPO, TRLMethod.GRPO}),
        "num_generations": frozenset({TRLMethod.GRPO}),
        "reward_funcs": frozenset({TRLMethod.GRPO}),
    }

    def __post_init__(self) -> None:
        self.validate()   # raises ValueError on a field set for the wrong method,
                          # or on method=grpo without reward_funcs

    def to_args(self) -> list[str]:
        """Render as [trl.scripts.<method>, --flag, value, ...]: the module name
        first (torchrun's -m positional), then the script flags — bools become
        store_true flags, lists expand after their flag, everything else is
        --name value."""
```

Users reach both through the trainer they already know:

```python
TrainerClient().train(
    trainer=BuiltinTrainer(
        config=TRLConfig(
            method=TRLMethod.DPO,
            model_name_or_path="Qwen/Qwen2.5-0.5B",
            dataset_name="trl-lib/ultrafeedback_binarized",
            beta=0.1,
        ),
        num_nodes=2,
        resources_per_node={"gpu": 1},
    ),
)
```

**Design decisions**

- **The framework is a config, not a trainer subclass.** A `BuiltinTrainer` holding a
  `TRLConfig` and one holding a `TorchTuneConfig` differ in data, not behaviour: the trainer
  hands `command` and `to_args()` to the backend either way. So `BuiltinTrainer(config=...)`
  stays the single config-driven entry point. (Alternative 8.)
- **Method is a field, not a config subclass.** TRL's methods differ only in which fields
  apply. Adding `kto` is one enum member and one map entry — no export, no backend change.
  (Alternative 7.)
- **`command` and `framework` are `ClassVar`s on the config, but `RuntimeTrainer.command`
  stays the field consumers read.** That field is not config-specific:
  `command[0] == "mpirun"` drives MPI detection, `get_runtime_packages` rewrites it for
  single-process, and the localprocess backend joins it into an entrypoint. Moving it onto
  the config would special-case all of those. Instead `get_runtime_trainer()` resolves it
  through the registry, replacing the `framework == TORCH_TUNE` branch at `utils.py:150-158`
  and deleting `constants.TORCH_TUNE_COMMAND`:

  ```python
  if config_cls := registry.get_framework(framework):
      trainer.set_command(config_cls.command)
  elif ml_policy.torch is not None:
      trainer.set_command(constants.TORCH_COMMAND)
  # ... mpi, default unchanged
  ```

  Adding a framework then adds no lines to `utils.py`.
- **`trainer_type` comes from the registry.** `BUILTIN_TRAINER` when `get_framework()` finds
  a registered config, `CUSTOM_TRAINER` otherwise — replacing `utils.py:114-119` and
  deleting the `TORCH_TUNE` constant. The framework label stays the sole discovery key.
- **Each config renders itself, so the backend has no framework branch.** One line —
  `trainer_cr.args = trainer.config.to_args()` — deletes the `isinstance` check at
  `utils.py:451-452` rather than relocating it. This is the one-time change that makes a
  *further* framework require none.
- **TorchTune's initializer-derived dataset overrides stay in the backend.** The
  `dataset.data_files=` / `data_dir=` args are computed from the HF dataset initializer,
  which is staging knowledge the backend owns; they are appended to whatever `to_args()`
  renders. `TRLConfig` needs nothing from the initializer.
- **`TorchTuneConfig.to_args()` delegates to the existing emitter.**
  `get_args_from_peft_config` maps `LoraConfig` onto nested `model.*` keys; a flat
  `key=value` walk would emit `lora_rank=8` instead of `model.lora_rank=8` and fail the job.
  The emitters move verbatim to `types/torchtune.py` — they hold no Kubernetes logic, and
  leaving them in the backend would make `types.py` import a backend module.
- **Registration happens at import time; a later registration wins.** `TRLConfig` must be exported from
  `kubeflow/trainer/__init__.py`, or a process that never imports it finds no claimant for
  `trl` and treats the runtime as function-driven. Shadowing lets a fork replace a stalled
  in-tree config without an upstream commit.
- **Named `FrameworkConfig`, not the earlier draft's `LLMBackend`.** `backends/` here means
  *execution* backend, and nothing about this object is LLM-specific.
- **`LoraConfig` is not reused for TRL.** It is TorchTune-shaped — `apply_lora_to_output`
  and `quantize_base` have no TRL analogue, and TRL's PEFT surface is `--use_peft` /
  `--lora_r` / `--lora_alpha` / `--lora_target_modules`. Sharing it needs a lossy translation.
- **Unsloth is a runtime-image concern.** An Unsloth-accelerated image is still labelled
  `framework: trl`, so it resolves to the same config and is chosen with `train(runtime=...)`.
  No SDK field.
- **Nothing is deprecated.** `BuiltinTrainer(config=TorchTuneConfig(...))` produces
  byte-identical `TrainJob` arguments. No successor class, no `FutureWarning`.

### II.8 Which framework, and why TRL

| Framework | Post-training methods | Maintenance | Argument shape | torchrun path |
|---|---|---|---|---|
| TRL | one plain script per method under `trl/scripts/` — `sft`, `dpo`, `grpo`, `kto`, `reward`, `rloo`; PPO moved to `trl.experimental` in 0.29 ([trl#4466](https://github.com/huggingface/trl/issues/4466)) | Active (Hugging Face) | flags | native: `torchrun -m trl.scripts.<method>` |
| TorchTune | SFT only, in the Kubeflow integration | Stopped 15 Jul 2025 ([#2883](https://github.com/meta-pytorch/torchtune/issues/2883)) | flags | via `tune run` (plugin rewrites the command) |
| LlamaFactory | Broad, but built on HF `Trainer` and PEFT | Active | config file | wrapped: needs `FORCE_TORCHRUN=1` and renamed env |
| Axolotl | Broad, but its GRPO is TRL's `GRPOTrainer` | Active | config file | native with `--launcher torchrun` (default is accelerate) |
| Unsloth | None of its own; an acceleration layer | Active | n/a | n/a — not a launcher |

TRL is selected on three grounds. Its per-method scripts cover what the SDK cannot express
at all today — DPO and GRPO. It is the engine rather than a wrapper: LlamaFactory and
Axolotl are both built on the HF `Trainer` and PEFT, so an in-tree TRL trainer adds
capability where an in-tree wrapper would add a second surface over the same engine. And its
scripts take flags, so `TRLConfig` renders straight into `args`; a file-first framework
would need the SDK to stage a ConfigMap or volume first. TRL is also the cleanest fit for
the torchrun-first pattern: its scripts are launched by torchrun directly, with no wrapper
CLI and no environment renaming in the path.

Choosing TRL first closes no doors: every alternative reaches the SDK out of tree as a
`FrameworkConfig` subclass, which is why LlamaFactory is the reference out-of-tree
framework. GRPO via TRL is already in flight in the Trainer
([#3508](https://github.com/kubeflow/trainer/issues/3508),
[#3718](https://github.com/kubeflow/trainer/pull/3718)); this proposal provides the surface
for that effort rather than a parallel track.

**Risk:** TRL's surface is not frozen, and a typed dataclass mirroring its flags will drift.
`TRLConfig` targets the script flags rather than the Python API, so a TRL upgrade changes
the image rather than the SDK type, and the drift is confined to the one config class.

---

## Migration and Backward Compatibility

**Server side.** Additive; no existing code path is touched.

| Aspect | Impact |
|---|---|
| CRDs | **No change.** No new or modified fields, no version bump. |
| Torch plugin | **No change.** Neither `EnforceMLPolicy` nor `Validate` is touched; TRL falls through the default path. A unit test asserts it stays that way. |
| Plugin registry | **No change.** TRL reuses the torch plugin via `mlPolicy.torch`. |
| Existing TorchTune runtimes | **No change.** Same images, manifests, rendered `TrainJob`s. |
| Default manifests | **Additive.** New runtimes under a new name prefix; nothing renamed or removed. |

**Client side.** Additive.

| Aspect | Impact |
|---|---|
| `CustomTrainer`, `CustomTrainerContainer` | **No change.** All fields retained. |
| `BuiltinTrainer` | **Construction signature unchanged**; produces byte-identical `TrainJob` arguments. `config` widens to `FrameworkConfig`. Nothing deprecated. |
| `TrainerClient.train()` | **Additive.** `runtime_config` is optional, defaulting to `None`; the `trainer` union gains `BaseTrainer`. |
| `TrainJobTemplate` | **No change** in this proposal. |
| Python version | No new floor. `kw_only=True` needs 3.10, already the SDK's minimum. |
| Public exports | New names only: `BaseTrainer`, `FuncTrainer`, `ConfigTrainer`, `TorchTrainer`, `DeepSpeedTrainer`, `JAXTrainer`, `XGBoostTrainer`, `RuntimeConfig`, `FrameworkConfig`, `TRLConfig`, `TRLMethod`. Nothing removed or renamed. |

## Test Plan

**Server-side Go unit tests** (`torch_test.go`, extending the table-driven cases). Two
cases, asserted alongside each other so the paths stay visibly distinct: a
`[torchrun, -m, trl.scripts.sft]` command is left alone and gets all five `PET_*`,
including the master vars the TorchTune path suppresses; a `[tune, run]` command is still
rewritten. The first is the regression guard for
[I.5](#i5-what-changes-in-the-plugin) — it fails the moment anyone adds a TRL branch to
the plugin.

**Manifest tests** — each `manifests/base/runtimes/trl/*.yaml` carries `framework: trl`,
`mlPolicy.torch: {}`, `command: [torchrun, -m, trl.scripts.<method>]`, and mounts
`initializer` at `/workspace`.

**Launch-path test** — `examples/trl/local_torchrun_proof.sh` (already committed and
passing, see [I.3](#i3-proof-of-concept)): two `torchrun -m trl.scripts.sft` launches with
only the five `PET_*` variables set must form world size 2, asserted by the per-step
epoch value (`0.0588` → `0.1111`); the single-node control run guards the assertion itself.
Re-run on every `trl` version bump in `requirements.txt`.

**E2E** — a two-node TRL `TrainJob` reaches `Complete` **and** its logs show the two-node
epoch increment. Asserting completion alone would have passed for the accelerate failure in
[I.4](#i4-limitation-the-trl-cli-and-accelerate-launch); the world-size assertion is the
part with teeth. Runs on CPU — no GPU gate needed for the launch path.

**Client-side unit tests** — type-hierarchy compliance; `validate_runtime()` positive,
negative framework, and negative `trainer_type`; `get_framework_args()` excludes
controller-injected arguments; `RuntimeConfig` defaults and merge precedence; auto-discovery
with one, zero and several matches (the last two raising, with names in the message).

**Client-side integration tests** — `TorchTrainer` end-to-end on the Kubernetes and
Container backends; `RuntimeConfig` packages installed and env set. Plus the **cross-repo
contract**: with the `trl-qwen2.5-1.5b` runtime installed,
`BuiltinTrainer(config=TRLConfig(...))` resolves it by label, yields
`trainer_type == BUILTIN_TRAINER`, and emits `command: [torchrun, -m]` with
`args == config.to_args()` (module name first). This single assertion ties the two parts
together and fails if either side changes the label or the launch shape.

**Backward compatibility** — every existing `CustomTrainer`, `BuiltinTrainer` and
`TrainJobTemplate` test passes unmodified.

## Implementation Plan

The parts are independent. Part I ships first because it is smaller and because Part II's
integration tests need a TRL runtime to run against.

**Server side**

| Phase | Contents |
|---|---|
| S1 | `cmd/trainers/trl/` with pinned versions, the launch-path proof script, image publishing in the existing workflow |
| S2 | `manifests/base/runtimes/trl/` + kustomization entry, manifest tests, an `examples/trl/` TrainJob |
| S3 | The `torch_test.go` regression cases, then the two-node cluster E2E (CPU is sufficient — the launch path is device-agnostic). |

S1 and S2 must ship together: a manifest referencing an unpublished image is untestable, and
an image with no manifest is unreachable.

**Client side**

| Phase | Contents |
|---|---|
| 1 | `BaseTrainer`, `FuncTrainer`, `ConfigTrainer`, `RuntimeConfig`, `_resolve_runtime()`, the `train()` signature, unit tests |
| 2 | `TorchTrainer`; `FuncTrainer` / `ConfigTrainer` dispatch in all three backends; integration tests; docs |
| 3 | `DeepSpeedTrainer` (multi-runtime), `JAXTrainer`, `XGBoostTrainer` |
| 4 | Public exports, sdk.kubeflow.org documentation, migration guide |

Phases 1 and 2 must ship in the same release: the type hierarchy without backend support
makes `TorchTrainer` importable but unusable, failing with a confusing `ValueError`.

## Graduation Criteria

**Alpha** — *Server:* the `trl-trainer` image is published and at least one runtime installs
by default; a two-node TRL job completes with `world_size == numNodes × numProcPerNode`,
verified in E2E rather than from job status; the torch plugin is unchanged, with a test
asserting it. *Client:* the type hierarchy and
`RuntimeConfig` are implemented and exported; `TorchTrainer` works on all three backends;
auto-discovery and `validate_runtime()` are functional and covered; merge semantics tested;
all existing tests pass.

**Beta** — `DeepSpeedTrainer`, `JAXTrainer`, `XGBoostTrainer`; `TRLConfig` alongside
`TorchTuneConfig`, proving the extension model with no new trainer class; TRL runtimes for
the model families TorchTune covers, with `dpo` and `grpo` exercised end-to-end, not only
`sft`; an out-of-tree config published **paired with an out-of-tree runtime image**, proving
both halves of the extension path (LlamaFactory is the reference — torchrun-wrapped rather
than torchrun-native); the torchrun-first launch pattern documented on kubeflow.org as
operator guidance for building a runtime image; SDK docs and a migration guide.

**GA** — Tier 1 trainers stable for a release; `BuiltinTrainer` and `CustomTrainer`'s
runtime-environment fields formally deprecated; at least one community `ConfigTrainer`
subclass; a runtime can report its achieved world size to `TrainJob` status, so a silently
degraded run is observable (Open Question #6).

## Open Questions

1. **DeepSpeed launcher detection.** `DeepSpeedTrainer` supports torchrun and mpirun; the
   `Runtime` type exposes no launcher metadata. Inspect `runtime.trainer.command`, add a
   launcher label, or defer to the controller?
2. **Does dynamic registration extend to the control plane?** *Partly answered.* The registry
   is SDK-side and stays there — Part I shows a new runtime needs no control-plane
   registration at all, only an image and a manifest. Open: whether the Trainer should
   *provision* runtimes for frameworks it does not ship (e.g. from a catalog CR).
3. **Observability.** Should the SDK log the auto-discovered runtime and framework at `INFO`?
4. **`resources_per_node` typing.** Should the untyped `Optional[dict]` become a structured
   `ResourceRequirements` dataclass?
5. **Backend validation safety net.** Should backends call `validate_runtime()` defensively,
   in case a trainer is passed to a backend directly?
6. **Should a runtime report its achieved world size?** The largest gap left open, and not
   TRL-specific. The `trl-demo` failure in
   [I.4](#i4-limitation-the-trl-cli-and-accelerate-launch) exited 0 on both pods, so a
   two-node job that trained two isolated copies was indistinguishable from one that trained
   jointly. torchrun-first removes that particular cause but not the class: any image whose
   launch degrades silently produces the same signature. A minimal mechanism: rank 0 reports
   the world size it joined with, and the controller surfaces it as a status condition,
   failing the job when it disagrees with `numNodes × numProcPerNode`. That is a
   control-plane change, out of scope here — but without it, "the TrainJob succeeded" is not
   evidence that distributed training happened.
   [KEP-2779](../2779-trainjob-progress/README.md) already establishes a runtime→status
   channel this could reuse.
7. **Should accelerate get its own plugin later?** The launcher-only options in
   [I.4](#i4-limitation-the-trl-cli-and-accelerate-launch) — an accelerate config file per
   job, the Megatron-LM plugin, TPU — are the reasons the question will return. The
   position from the SDK call was that a dedicated accelerate plugin is not objectionable in
   principle — the ask was to understand the torchrun limitations first, which
   [I.4](#i4-limitation-the-trl-cli-and-accelerate-launch) now records. It is deliberately
   kept separate from shipping TRL, along with dynamic registration
   (see #2): both would need their own design discussion, and neither blocks TRL.

## Alternatives Considered

| # | Alternative | Why rejected |
|---|---|---|
| 1 | Add a `framework` field to `CustomTrainer` instead of new classes | No home for typed framework arguments; no extension model; the class grows with every framework |
| 2 | Pydantic `BaseModel` instead of `@dataclass` | The SDK uses dataclasses exclusively; adds a dependency for validation `__post_init__` already covers |
| 3 | `RuntimeConfig` as a `BaseTrainer` field | The same runtime settings apply to every trainer type; as a `train()` parameter it can extend to initializers later |
| 4 | Scoring/ranking when several runtimes match | Implicit heuristics are fragile and hard to debug; multiple runtimes per framework is deliberate platform configuration, so the user should choose. A clear error listing them beats a wrong guess |
| 5 | Flat hierarchy, everything inherits `BaseTrainer` | Config trainers would carry `get_train_func()` returning `None`; func trainers would redeclare `func`/`func_args`; dispatch would test values instead of types |
| 6 | Specialized trainers inherit `CustomTrainer` | Forces them to carry the runtime-env fields this proposal is separating out |

**7. One config class per method (`SFTConfig`, `DPOConfig`, `GRPOConfig`).** The method axis
changes only *which fields apply*, while `to_args()`, `command` and the framework label are
per-framework — so once a second framework supports the same method, `SFTConfig` needs a
framework discriminator anyway. Methods are also the volatile axis: TRL moved PPO to
`trl.experimental` in a minor release. An enum member can be added or deprecated freely; an
exported class name cannot. The taxonomy is still captured, as `TRLMethod` and
`_METHOD_SCOPED_FIELDS`, where regrouping is a data change rather than an API change.

**8. One trainer class per framework (`TRLTrainer`, `TorchTuneTrainer`).** The trainer does
nothing different per framework — it hands `command` and `to_args()` to the backend — so a
subclass per framework encodes in the type hierarchy a distinction that exists only in data.
It makes every framework a new exported class name, frozen public API, and forces
`BuiltinTrainer` onto a deprecation path toward a `TorchTuneTrainer` successor for no
behaviour change. *What it gets right:* typed per-framework fields and a flatter call. The
accepted design keeps the typed fields — on the config — at the cost of one level of nesting.

## References

**Kubeflow**

- [KEP-2170: Trainer V2 API](../2170-kubeflow-trainer-v2/README.md)
- [KEP-2401: LLM Trainer V2](../2401-llm-trainer-v2/README.md) — the TorchTune integration
  this generalizes; its "Complement Torch Plugin" section originated the `EnforceMLPolicy`
  TorchTune branch
- [KEP-2779: TrainJob progress](../2779-trainjob-progress/README.md) — the runtime→status
  channel Open Question #6 would reuse
- Tracking issue [trainer#2839](https://github.com/kubeflow/trainer/issues/2839); GRPO via
  TRL in flight in [#3508](https://github.com/kubeflow/trainer/issues/3508) /
  [#3718](https://github.com/kubeflow/trainer/pull/3718)
- Earlier draft, superseded: [trainer#3263](https://github.com/kubeflow/trainer/pull/3263),
  [sdk#310 registry PoC](https://github.com/kubeflow/sdk/pull/310)
- [Runtime guide — the framework label](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [SDK types](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/types/types.py),
  [SDK TrainerClient](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py)

**Upstream sources cited in Part I**

- `torch.distributed.argparse_util` — the `env` action deriving `PET_{DEST}`; why `PET_*` is
  torchrun's interface and nothing else's. `torch.distributed.run` — `auto` handling and the
  `-m` / `--module` invocation the runtime command uses.
- `trl/scripts/sft.py` — a plain script (`TrlParser` + `SFTTrainer`, `__main__` guard, no
  launcher logic); one sibling module per method. `trl/cli/accelerate_launcher.py` —
  `launch_training_script` resolves `resources.files("trl.scripts")` and calls accelerate's
  `launch_command`, which is why the CLI is accelerate-bound.
- `accelerate.state.PartialState` — in-process env detection: `LOCAL_RANK != -1` → multi-GPU
  (nccl); `WORLD_SIZE > 1` on CPU → multi-CPU (gloo). This is what makes the script work
  under torchrun with no launcher flags.
- `accelerate.commands.launch` — the *launcher* limitations recorded in I.4:
  `launch_command`'s distributed branches are each guarded `and not args.cpu`;
  `prepare_simple_launcher_cmd_env` sets `MASTER_ADDR` / `MASTER_PORT` but never `RANK` /
  `WORLD_SIZE`; the `multi_gpu` auto-enable guard is keyed on
  `torch.cuda.device_count()`.
- [trl#4466](https://github.com/huggingface/trl/issues/4466) — PPO moved to experimental.
  [torchtune#2883](https://github.com/meta-pytorch/torchtune/issues/2883) — development
  halted, 15 July 2025.
