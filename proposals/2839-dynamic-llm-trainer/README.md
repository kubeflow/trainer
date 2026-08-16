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
- [Overview](#overview)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Part I: Server Side (`kubeflow/trainer`)](#part-i-server-side-kubeflowtrainer)
  - [I.1 What the torch plugin already provides](#i1-what-the-torch-plugin-already-provides)
  - [I.2 `PET_*` is torchrun's private interface](#i2-pet_-is-torchruns-private-interface)
  - [I.3 Proof of concept](#i3-proof-of-concept)
  - [I.4 The adapter, and where it belongs](#i4-the-adapter-and-where-it-belongs)
  - [I.5 Why the plugin does not change](#i5-why-the-plugin-does-not-change)
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

The work spans two repositories, and the split between them is the load-bearing decision.

**Server side (`kubeflow/trainer`).** A framework becomes supportable by adding a runtime
image and a `ClusterTrainingRuntime`. The CRDs, the controller and the torch plugin's
`EnforceMLPolicy` are unchanged. The plugin already publishes the cluster topology as
`PET_*` environment variables; what a new framework needs is a **launcher adapter** in its
own image that translates `PET_*` into whatever its launcher reads. Part I specifies that
adapter for TRL and measures its cost for three other frameworks. No Go changes at all.

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

1. Establish the **launcher-adapter contract**: what a runtime image must do with `PET_*`
   so any config-driven framework can reuse the torch plugin without modifying it.
2. Ship TRL as the second in-tree config-driven framework — a `cmd/trainers/trl/` image and
   manifests under `manifests/base/runtimes/trl/`, mirroring what TorchTune already has.
3. Keep the whole framework-specific translation inside that image, so the control plane
   gains no knowledge of TRL and no Go changes.

**Client side**

4. Define `BaseTrainer`, plus `FuncTrainer` (function-driven) and `ConfigTrainer`
   (config-driven), so the SDK and backends handle any trainer polymorphically.
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
   both unchanged — see [I.5](#i5-why-the-plugin-does-not-change).
2. **A new framework plugin.** TRL reuses the torch plugin via `mlPolicy.torch`; no entry
   is added to `pkg/runtime/framework/plugins/registry.go`.
3. **New runtime labels or conventions.** `trainer.kubeflow.org/framework` is unchanged.
4. **A generic launcher adapter in the control plane.** The per-framework translation cost
   ranges from zero lines to flag synthesis, with no common core to hoist
   ([I.4](#i4-the-adapter-and-where-it-belongs)).
5. **Deprecating `CustomTrainer` or `BuiltinTrainer`.** Both stay supported.
6. **In-tree Axolotl, LlamaFactory or Unsloth runtimes.** Their adapter cost is measured
   here to check the contract generalizes; landing them is follow-up work.
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

It also opens port 29500 for the headless service (`torch.go:203`) and rejects a `TrainJob`
setting any of those five names itself (`torch.go:67-77`).

There is exactly one framework-specific branch: when `spec.trainer.command` equals
`["tune", "run"]`, the plugin withholds `PET_MASTER_ADDR` / `PET_MASTER_PORT` and rewrites
the command with `--rdzv_endpoint=<host>:29500` plus the recipe, config and runtime-derived
overrides (`torch.go:175-201`). Every other command gets all five and no rewrite.

So a second framework inherits a complete description of the topology for free. **The only
question is whether its launcher reads it.**

### I.2 `PET_*` is torchrun's private interface

`PET_*` is not a Kubeflow or general convention. `torch.distributed.argparse_util` defines
an `env` argparse action that falls back to `PET_{dest.upper()}` for each flag. `torchrun
--nnodes` reads `PET_NNODES` because torchrun asked it to. That puts `PET_*` at a specific
layer:

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

TRL is such a framework. `trl sft` forwards unrecognised arguments to `accelerate launch`,
and **accelerate has no reference to `PET_` anywhere in its source**. Accelerate is a *peer*
of torchrun, not a layer beneath it: it takes topology from flags (`--num_processes`,
`--num_machines`, `--machine_rank`, `--main_process_ip`, `--main_process_port`). The flags
exist and TRL forwards them, so the gap is bridgeable. Bridging it is the adapter.

### I.3 Proof of concept

Two `TrainJob`s on two CPU nodes against an **unmodified** torch plugin, both with
`mlPolicy.torch` so both got identical `PET_*`. The only variable was what consumed them.

| Run | Consumer of `PET_*` | Outcome |
|---|---|---|
| `trl-demo` | `trl sft` → `accelerate launch`, via a flag-translating entrypoint | Translation was **correct** — the pods got `--machine_rank 0` and `1` with matching `--main_process_ip`. Yet `epoch` advanced `1.923e-05 = 1/52002` per step instead of `2/52002`: **world size 1**. Both pods `Succeeded`, TrainJob `Complete`. |
| `probe-rendezvous` | `torchrun` reading `PET_*` directly | `[Gloo] Rank 1 is connected to 1 peer ranks`, then `rank=1 world_size=2 allreduce=2.0`. **Real rendezvous**, same configuration. |

The probe is a bare `torchrun` over a script that joins a Gloo group and all-reduces, each
rank contributing `1.0`; a sum of `2.0` can only come from two processes that exchanged
data. It cannot be faked by a misconfigured single process.

The pair is a controlled comparison: where accelerate produced world size 1, torchrun
reading the same variables produced 2. `trl-demo` ran two independent single-process
trainings and reported one distributed job. Two causes, both in accelerate:

1. **`--use_cpu` disables every distributed path** *(demonstrated)*. Each distributed branch
   in `launch_command` is guarded `... and not args.cpu`, so a CPU run falls through to
   `simple_launcher`, which spawns one subprocess and sets `MASTER_ADDR` / `MASTER_PORT` but
   never `RANK` / `WORLD_SIZE`. Multi-process CPU needs `--mpirun_hostfile`, unreachable
   from `PET_*`.
2. **`--multi_gpu` is inferred only from local device count** *(from source; not exercised
   on the cluster)*. Accelerate auto-enables it when `torch.cuda.device_count() > 1` in the
   current process. `numProcPerNode` is per pod and the usual layout is one GPU per pod, so
   it is never inferred and must be passed explicitly.

**Both failure modes exit 0.** JobSet observes completion, the TrainJob reports `Succeeded`,
and no signal reaches the control plane. This is the risk the rest of Part I is designed
around, and why the adapter is specified here rather than left to each image author.

### I.4 The adapter, and where it belongs

Adapter cost for the candidate frameworks, measured against the `PET_*` already injected.
The TRL row is from the runs above; the others are from reading each project's launcher
entrypoint — strong at the launcher layer, silent about dataset or checkpoint plumbing.

| Framework | Launcher invocation | Adapter | Trap |
|---|---|---|---|
| **Axolotl** | `axolotl train cfg.yaml --launcher torchrun` | **none** | Passes no topology flags of its own, so torchrun reads `PET_*` natively. Its *default* launcher is accelerate, which reproduces `trl-demo` exactly — `--launcher torchrun` is mandatory. |
| **LlamaFactory** | `llamafactory-cli train cfg.yaml` | ~6 lines, env rename | Reads `NNODES` / `NODE_RANK` / `NPROC_PER_NODE` / `MASTER_ADDR` / `MASTER_PORT`. `FORCE_TORCHRUN=1` is mandatory: at one GPU per pod it otherwise skips torchrun and trains single-process. |
| **TRL** | `trl <method>` → `accelerate launch` | ~20 lines, flag synthesis | Needs `--multi_gpu`; CPU multi-node unreachable without MPI. |
| **Unsloth** | none | n/a | Not a launcher. Patches TRL and pins `DistributedType.NO` on one device — a TRL *image* variant, not a framework. |

**The adapter belongs in the runtime image, per framework.** There is nothing shared to
hoist: the cost ranges from zero lines to flag synthesis and the traps are entirely
framework-specific. Hoisting would mean teaching a Go controller the CLI semantics of four
Python projects, and would require the plugin to know each runtime's framework —
reintroducing the coupling the framework label exists to avoid.

**The plugin's contract is already correct.** Axolotl needs zero adaptation: a framework
that speaks torchrun is supported today with no server-side code. That is the strongest
available evidence that `PET_*` is the right thing to publish and the gap is framework-side.

### I.5 Why the plugin does not change

The PoC ran against an unmodified `torch.go`, and `EnforceMLPolicy` needs no TRL branch:

- The TorchTune branch exists because `tune run` takes rendezvous as a *command-line
  argument*, so the plugin must rewrite the command and suppress the master env vars. TRL
  takes rendezvous from the environment, so it wants the default behaviour.
- A TRL branch would put accelerate flags into the Go controller. Those flags are a property
  of the TRL and accelerate versions pinned in the image and change on the image's cadence,
  not the controller's. In the image, a TRL upgrade is a rebuild rather than a release.

`Validate` is unchanged for the same reason: the conditions worth rejecting are properties
of the image's launcher, so the image is where they are checked.

### I.6 New artifacts

Two, mirroring what TorchTune already has. The adapter is named `trl-launch` and installed
on `PATH` rather than referenced by absolute path, so a runtime's `command` is
`["trl-launch"]` — short, stable across image layouts, and distinct from `trl` itself so it
is clear the container runs the adapter rather than the CLI directly.

**1. `cmd/trainers/trl/`** — `Dockerfile` (same PyTorch base as `cmd/trainers/torchtune`),
`requirements.txt` (pinned), and `trl-launch`:

```sh
#!/bin/sh
# The torch plugin publishes topology as PET_* for torchrun. `trl` hands off to
# accelerate, which reads topology from flags only. Translate, guard, exec.
set -eu

METHOD="${TRL_METHOD:-sft}"
NNODES="${PET_NNODES:-1}"; NODE_RANK="${PET_NODE_RANK:-0}"; NPROC="${PET_NPROC_PER_NODE:-1}"

# "auto" means one process per GPU. Resolving it to 1 with no GPU visible would
# turn a distributed request into a single-process run that looks deliberate.
if [ "${NPROC}" = "auto" ]; then
    PY=$(command -v python || command -v python3)
    NPROC=$("${PY}" -c 'import torch; print(torch.cuda.device_count())')
    [ "${NPROC}" = "0" ] && { echo "[trl-launch] ERROR: auto but no CUDA device." >&2; exit 1; }
fi

TOTAL=$((NPROC * NNODES))

# accelerate's distributed branches are guarded "and not args.cpu", so --use_cpu
# falls through to simple_launcher, which never sets RANK/WORLD_SIZE. Refuse it
# rather than train N isolated copies and report success.
for arg in "$@"; do
    case "${arg}" in
        --use_cpu|--use_cpu=*|--cpu|--cpu=*)
            [ "${TOTAL}" -gt 1 ] && {
                echo "[trl-launch] ERROR: ${TOTAL} processes with --use_cpu; needs MPI." >&2
                exit 1; } ;;
    esac
done

# --num_processes is the total across machines, not per node.
set -- --num_processes "${TOTAL}" --num_machines "${NNODES}" \
       --machine_rank "${NODE_RANK}" \
       --main_process_ip "${PET_MASTER_ADDR:-localhost}" \
       --main_process_port "${PET_MASTER_PORT:-29500}" "$@"

# accelerate infers --multi_gpu only from the *local* device count, which is 1 in
# the one-GPU-per-pod layout. Without this every pod runs at world_size 1, exit 0.
[ "${TOTAL}" -gt 1 ] && set -- --multi_gpu "$@"

exec trl "${METHOD}" "$@"
```

The three guards are the whole point. The PoC's first entrypoint had the flag translation
but none of them, and produced `trl-demo`; the translation being correct is what makes that
failure hard to spot.

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
                      env: [{name: TRL_METHOD, value: sft}]
                      command: [trl-launch]        # the adapter, not `trl` itself
                      args:
                        - --model_name_or_path=/workspace/model
                        - --dataset_name=/workspace/dataset
                        - --output_dir=/workspace/output
```

The command is not `["tune", "run"]`, so `EnforceMLPolicy` takes its default path. The SDK
appends `config.to_args()` to `args`, exactly as for TorchTune.

**The server side stops here** — an image and manifests, no control-plane code. Every trap
in [I.3](#i3-proof-of-concept) and [I.4](#i4-the-adapter-and-where-it-belongs)
(`--multi_gpu`, `FORCE_TORCHRUN`, `--launcher torchrun`) depends on flags and defaults
*inside* the image, which the control plane cannot inspect under any design. They are the
adapter's responsibility, which is why the adapter is specified in this KEP rather than left
to each image author.

Two things could still be added, and both are deferred rather than dismissed: an admission
check for the one case visible before launch ([Open Question #7](#open-questions)), and a
way for a runtime to *report* its achieved world size, which would close the gap for every
framework at once ([#6](#open-questions)).

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

TorchTune is hardcoded at four points, and #3 is the load-bearing one:

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
        super().__init_subclass__(**kwargs)
        # ABCMeta populates __abstractmethods__ only *after* __init_subclass__, and
        # it does not inherit through the MRO — so detect abstract intermediates by
        # inspecting cls.__dict__ instead. Reading __abstractmethods__ here would
        # fire on FuncTrainer/ConfigTrainer, which legitimately declare nothing.
        if any(getattr(v, "__isabstractmethod__", False) for v in cls.__dict__.values()):
            return
        if not getattr(cls, "supported_frameworks", None):
            raise TypeError(f"{cls.__name__} must define a non-empty 'supported_frameworks'")

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

A property rather than a `ClassVar`, because the framework is a property of the instance's
config. `__init_subclass__` reads `getattr(cls, "supported_frameworks", None)`, which here
returns the property object — always truthy — so the guard passes without inspecting a
value. That is fine: the value would come from `cls.config.framework`, and the registry
already rejects a `FrameworkConfig` declaring no framework. It is validated once, where it
is declared.

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
- A separate `train()` parameter rather than a trainer field, because runtime configuration
  is orthogonal to trainer type — and this leaves room to apply env to initializers later
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
the controller. The SDK validates only deterministic, known properties; the control plane
stays the final arbiter for quotas, policy and version compatibility.

Trainers are **data objects, not builders**. They expose the function or config, framework
arguments, and scaling; the backend serializes and builds the CR; the controller owns
distributed topology and policy. So a new trainer needs no backend change, and a new backend
needs no trainer change:

```python
def _build_trainer_cr(self, runtime, trainer):
    # num_nodes, resources_per_node, image copied across as today.
    if isinstance(trainer, FuncTrainer):
        trainer_cr.command = get_command_using_train_func(runtime, trainer.get_train_func(), ...)
        trainer_cr.args = [f"--{k}={v}" for k, v in trainer.get_framework_args().items()]
    elif isinstance(trainer, ConfigTrainer):
        # Entrypoint comes from the runtime, exactly as today. to_args() is
        # polymorphic, so there is no framework branch here.
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

An explicit decorator, because that is how this ecosystem registers plugins — the control
plane lists every framework plugin by hand in `plugins/registry.go`, and
[sdk#310](https://github.com/kubeflow/sdk/pull/310) is the PoC for this shape. It takes no
string argument, so there is no name to drift from the class. No entry-point or import-hook
discovery: a config must be imported before it can be constructed, and importing it
registers it.

`TorchTuneConfig` keeps every field and its signature, gaining
`framework = "torchtune"`, `command = ("tune", "run")`, and a `to_args()` delegating to the
existing emitter.

```python
# kubeflow/trainer/types/trl.py

class TRLMethod(Enum):
    """Post-training method; the value is the TRL CLI subcommand."""
    SFT = "sft"; DPO = "dpo"; GRPO = "grpo"

@register_framework
@dataclass
class TRLConfig(FrameworkConfig):
    framework: ClassVar[str] = "trl"
    # The launcher adapter, not `trl` itself — see I.6.
    command: ClassVar[tuple[str, ...]] = ("trl-launch",)

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
        """Render as [<subcommand>, --flag, value, ...]: bools become store_true
        flags, lists expand after their flag, everything else is --name value."""
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
  a claimant, `CUSTOM_TRAINER` otherwise — replacing `utils.py:114-119` and deleting the
  reflection-derived `TORCH_TUNE` constant. The framework label stays the sole discovery key.
- **Rendering is polymorphic, so the backend has no framework branch.** One line —
  `trainer_cr.args = trainer.config.to_args()` — deletes the `isinstance` guard at
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
- **Registration is import-time and last-writer-wins.** `TRLConfig` must be exported from
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

| Framework | Post-training methods | Maintenance | Argument shape | Launcher adapter |
|---|---|---|---|---|
| TRL | `sft`, `dpo`, `grpo`, `kto`, `reward`, `rloo` on the stable CLI; PPO moved to `trl.experimental` in 0.29 ([trl#4466](https://github.com/huggingface/trl/issues/4466)) | Active (Hugging Face) | flags | ~20 lines |
| TorchTune | SFT only, in the Kubeflow integration | Stopped 15 Jul 2025 ([#2883](https://github.com/meta-pytorch/torchtune/issues/2883)) | flags | none (plugin rewrites the command) |
| LlamaFactory | Broad, but built on HF `Trainer` and PEFT | Active | config file | ~6 lines |
| Axolotl | Broad, but its GRPO is TRL's `GRPOTrainer` | Active | config file | none |
| Unsloth | None of its own; an acceleration layer | Active | n/a | n/a |

TRL is selected on three grounds. Its stable CLI covers the methods the SDK cannot express
at all today — DPO and GRPO. It is the substrate rather than a wrapper: LlamaFactory and
Axolotl are both built on the HF `Trainer` and PEFT, so an in-tree TRL trainer adds
capability where an in-tree wrapper would add a second surface over the same engine. And its
CLI takes flags, so `TRLConfig` renders straight into `args`; a file-first CLI would need the
SDK to stage a ConfigMap or volume first.

Note the last two columns rank the frameworks in opposite orders — argument shape is the
SDK's concern, launcher adapter the image's. Choosing TRL trades a ~20-line adapter, written
once and shipped in one image, for a config surface needing no volume plumbing on the client.

Choosing TRL first forecloses nothing: every alternative reaches the SDK out of tree as a
`FrameworkConfig` subclass, which is why LlamaFactory is the reference out-of-tree
framework. GRPO via TRL is already in flight in the Trainer
([#3508](https://github.com/kubeflow/trainer/issues/3508),
[#3718](https://github.com/kubeflow/trainer/pull/3718)); this proposal provides the surface
for that effort rather than a parallel track.

**Risk:** TRL's surface is not frozen, and a typed dataclass mirroring its flags will drift.
`TRLConfig` targets the CLI rather than the Python API, so a TRL upgrade changes the image
rather than the SDK type, and the drift is confined to the one config class.

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
cases, asserted alongside each other so the paths stay visibly distinct: a `[trl-launch]`
command is left alone and gets all five `PET_*`, including the master vars the TorchTune
path suppresses; a `[tune, run]` command is still rewritten. The first is the regression
guard for [I.5](#i5-why-the-plugin-does-not-change) — it fails the moment anyone adds a TRL
branch to the plugin.

**Manifest tests** — each `manifests/base/runtimes/trl/*.yaml` carries `framework: trl`,
`mlPolicy.torch: {}`, `command: [trl-launch]`, and mounts `initializer` at `/workspace`.

**Adapter tests** — under `sh`, with `trl` stubbed by a script echoing its argv: flag
synthesis (`--num_processes` is the **total**, not per-node); `--multi_gpu` added when and
only when the total exceeds 1; `auto` with no CUDA device fails loudly; `--use_cpu` above
one process exits non-zero; user arguments preserved after the synthesized ones.

**E2E** (GPU-gated, like the TorchTune E2E) — a two-node TRL `TrainJob` reaches `Complete`
**and** its logs report `world_size=2`. Asserting completion alone would have passed for
`trl-demo`; the world-size assertion is the part with teeth.

**Client-side unit tests** — type-hierarchy compliance; `validate_runtime()` positive,
negative framework, and negative `trainer_type`; `get_framework_args()` excludes
controller-injected arguments; `RuntimeConfig` defaults and merge precedence; auto-discovery
with one, zero and several matches (the last two raising, with names in the message).

**Client-side integration tests** — `TorchTrainer` end-to-end on the Kubernetes and
Container backends; `RuntimeConfig` packages installed and env set. Plus the **cross-repo
contract**: with the `trl-qwen2.5-1.5b` runtime installed,
`BuiltinTrainer(config=TRLConfig(...))` resolves it by label, yields
`trainer_type == BUILTIN_TRAINER`, and emits `command: [trl-launch]` with
`args == config.to_args()`. This single assertion ties the two parts together and fails if
either side changes the label or the entrypoint name.

**Backward compatibility** — every existing `CustomTrainer`, `BuiltinTrainer` and
`TrainJobTemplate` test passes unmodified.

## Implementation Plan

The parts are independent. Part I ships first because it is smaller and because Part II's
integration tests need a TRL runtime to run against.

**Server side**

| Phase | Contents |
|---|---|
| S1 | `cmd/trainers/trl/` with pinned versions, adapter tests, image publishing in the existing workflow |
| S2 | `manifests/base/runtimes/trl/` + kustomization entry, manifest tests, an `examples/trl/` TrainJob |
| S3 | The `torch_test.go` regression cases; confirm on GPU hardware that omitting `--multi_gpu` degrades to world size 1 — cause #2 is source-derived only, and justifies one of the adapter's three guards. Then the two-node E2E. |

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
both halves of the extension path (LlamaFactory is the reference — the smallest non-zero
adapter); the launcher-adapter contract documented on kubeflow.org as operator guidance; SDK
docs and a migration guide.

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
   TRL-specific. Every failure in [I.3](#i3-proof-of-concept) exits 0, so a two-node job that
   trained two isolated copies is indistinguishable from one that trained jointly.
   Nothing catches this today: the adapter refuses the configurations it can see before
   launch, and the rest is decided inside the image. A minimal mechanism: rank 0 reports the
   world size it joined with, and the
   controller surfaces it as a status condition, failing the job when it disagrees with
   `numNodes × numProcPerNode`. That is a control-plane change, out of scope here — but
   without it, "the TrainJob succeeded" is not evidence that distributed training happened.
   [KEP-2779](../2779-trainjob-progress/README.md) already establishes a runtime→status
   channel this could reuse.
7. **Should the CPU multi-process case be rejected at admission?** It is the one failure from
   [I.3](#i3-proof-of-concept) visible from the `TrainJob` before any pod starts, so a
   `Validate` branch matching `command == ["trl-launch"]` could reject it with a clear error
   on `kubectl apply` instead of a `CrashLoopBackOff` one model download later. This
   proposal leaves it out: the adapter already refuses the configuration, and the branch
   would be the only Go in the change — teaching the control plane one framework's launcher
   semantics for a better error message. Worth revisiting once TRL runtimes see real use and
   we know whether users hit it.
8. **Should the adapter be shared rather than per-image?**
   [I.4](#i4-the-adapter-and-where-it-belongs) says no, on the evidence that the cost ranges
   from zero lines to flag synthesis with no common core. If a third and fourth framework
   both need TRL-shaped flag synthesis, a small shared helper installed into runtime images —
   still outside the control plane — becomes worth revisiting. One data point is not a
   pattern.

## Alternatives Considered

| # | Alternative | Why rejected |
|---|---|---|
| 1 | Add a `framework` field to `CustomTrainer` instead of new classes | No home for typed framework arguments; no extension model; the class grows with every framework |
| 2 | Pydantic `BaseModel` instead of `@dataclass` | The SDK uses dataclasses exclusively; adds a dependency for validation `__post_init__` already covers |
| 3 | `RuntimeConfig` as a `BaseTrainer` field | Runtime env is orthogonal to trainer type; as a `train()` parameter it can extend to initializers later |
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
  torchrun's interface and nothing else's. `torch.distributed.run` — `auto` handling.
- `accelerate.commands.launch` — `launch_command`, whose distributed branches are each
  guarded `and not args.cpu`; `prepare_simple_launcher_cmd_env`, which sets `MASTER_ADDR` /
  `MASTER_PORT` but never `RANK` / `WORLD_SIZE`; and the `multi_gpu` auto-enable guard keyed
  on `torch.cuda.device_count()`.
- `trl.cli` — argument forwarding by exclusion to `accelerate launch`.
- [trl#4466](https://github.com/huggingface/trl/issues/4466) — PPO moved to experimental.
  [torchtune#2883](https://github.com/meta-pytorch/torchtune/issues/2883) — development
  halted, 15 July 2025.
