# KEP-2839: Kubeflow Dynamic LLM Trainer Framework

Authors:

- Yassin Nouh - [@YassinNouh21](https://github.com/YassinNouh21)
- Saad Zaher - [@szaher](https://github.com/szaher)

Creation date: 2026-02-11

## Summary

Kubeflow Trainer can run exactly one config-driven LLM framework: TorchTune. Its upstream
development stopped in July 2025. This proposal makes the framework axis extensible. It adds
Hugging Face TRL as the second framework. TRL brings the post-training methods that the SDK cannot
express today: DPO and GRPO.

The work covers two repositories. Each side is useful on its own.

**Server side (`kubeflow/trainer`).** To support a framework, you add a runtime image and a
`ClusterTrainingRuntime`. The CRDs, the controller and the torch plugin do not change. This needs
no Go at all. The torch plugin already publishes the cluster topology as `PET_*` environment
variables. `PET_*` is torchrun's own env interface. So the TRL runtime launches TRL's training
script with torchrun (`torchrun -m trl.scripts.sft`), and the topology reaches the script with no
translation. The TRL runtime does not use the `trl` CLI. The CLI passes control to
`accelerate launch`, which does not read `PET_*`. It then runs single-process training and still
reports success. This KEP records that failure as a limitation and defers support for accelerate.

**Client side (`kubeflow/sdk`).** The client work has two phases.

- **Phase 1 (this KEP).** TRL ships through the existing `BuiltinTrainer`, next to
  `TorchTuneConfig`. We add `FrameworkConfig`, an abstract config that knows its framework
  label, its entrypoint and how to render itself as arguments. We also add a registry keyed on
  the framework label. `TorchTuneConfig` becomes a `FrameworkConfig`, and
  `BuiltinTrainer.config` widens from `TorchTuneConfig` to `FrameworkConfig`. TRL arrives as
  `TRLConfig`. Its typed `trainer` slot holds a Kubeflow-owned trainer config, such as
  `DPOTrainer`.
- **Phase 2 (later, or a follow-up KEP).** The SDK moves to the `BaseTrainer` /
  `ConfigTrainer` / `FuncTrainer` hierarchy. TRL then moves from `BuiltinTrainer` to
  `ConfigTrainer`. This is a breaking change for users. See
  [Phase 2: the move to `ConfigTrainer`](#phase-2-the-move-to-configtrainer).

The contract between the two sides already exists, and it is narrow. It contains the framework
label and the container `command` to which the SDK appends rendered arguments. The runtime is
usable from raw YAML before any SDK change ships.

We built a proof of concept for the server side and ran it on a cluster.
[Proof of concept](#proof-of-concept) gives its results. These results changed the design.

## Motivation

TorchTune is the only config-driven framework that the Trainer supports. It covers supervised
fine-tuning only. Its upstream development stopped on 15 July 2025
([torchtune#2883](https://github.com/meta-pytorch/torchtune/issues/2883)). A data scientist who
wants DPO or GRPO has no supported path. `BuiltinTrainer` accepts a `TorchTuneConfig` and nothing
else. Also, the SDK hardcodes TorchTune at four points. So you cannot add a second framework
without changes to shared code each time.

Meanwhile, LLM post-training now goes further than SFT. Users are asking for reward-driven
alignment (DPO) and group-relative policy optimization (GRPO)
([#3508](https://github.com/kubeflow/trainer/issues/3508)). The Trainer has no surface for either.

### Why TRL support is worth having

These are the practical cases that TRL support solves. The user stories in
[Proposal](#user-stories) show the SDK calls.

1. **Alignment on preference data.** A team has pairs of chosen and rejected answers and wants
   to align a chat model with DPO. Today, the Trainer has no supported way to do this, so the
   team must write, build and maintain its own training image. With TRL, the team writes a
   `DPOTrainer` config and uses the runtime that Kubeflow ships.
2. **Reward-based training for reasoning.** A team wants to train a model on math or code
   tasks with GRPO and a reward function. This is the request in #3508. The community PR
   [#3718](https://github.com/kubeflow/trainer/pull/3718) shows the cost today: it adds a
   separate GRPO image with its own `train.py`. With TRL, the same job is one `GRPOTrainer`
   config on one shared runtime.
3. **A supported path off TorchTune.** A team runs SFT with `TorchTuneConfig` today. TorchTune
   is end-of-life, so the team needs a maintained framework for the same job. With TRL, the
   team keeps `BuiltinTrainer` and the `Initializer`, and changes only the config.
4. **One stack for platform teams.** A platform team supports many data scientists. It wants
   one maintained post-training runtime for SFT, DPO and GRPO, not one custom image per
   technique. TRL gives one image and one `ClusterTrainingRuntime` for all three.

TRL is the most direct way to get these. Hugging Face maintains it. It has one script per
trainer. Other frameworks such as Axolotl use its trainers internally (see
[Which framework, and why TRL](#which-framework-and-why-trl)).

### Goals

**Server side**

1. Ship TRL as the second in-tree config-driven framework. This is a `cmd/trainers/trl/` image
   and a runtime under `manifests/base/runtimes/trl/`. The runtime uses **torchrun** as its
   launcher. torchrun is the standard launcher that the torch plugin already serves.
2. Keep the control plane completely unchanged: no Go, no CRD change, no knowledge of TRL.
3. Document precisely why phase one excludes the `trl` CLI / `accelerate launch` route. That
   route fails *silently* at world size 1. With this record, the future accelerate discussion
   can rely on measured results.

**Client side**

4. Introduce `FrameworkConfig` and a framework registry. Then the SDK resolves the entrypoint
   and how to render the arguments from the config, not from a TorchTune branch. This removes
   the four hardcoded TorchTune couplings.
5. Add `TRLConfig` with typed, Kubeflow-owned trainer configs (`SFTTrainer`, `DPOTrainer`,
   `GRPOTrainer`). This proves the extension model with no new top-level trainer class.
6. Provide an extension point for community config-driven frameworks. The extension point is a
   `FrameworkConfig` subclass in any package. Importing the package registers the subclass.
7. In Phase 1, keep full backward compatibility with `CustomTrainer`,
   `CustomTrainerContainer`, `BuiltinTrainer` and `TrainerClient.train()`. Phase 2 is the only
   planned breaking change, and this KEP documents it in advance.

### Non-Goals

1. **Any control-plane change.** No CRD or schema change, no version bump, and no Go. The
   server-side work is a new image and new manifests. `EnforceMLPolicy` and `Validate` both
   stay unchanged. See [What changes in the plugin](#what-changes-in-the-plugin).
2. **A new framework plugin.** TRL reuses the torch plugin through `mlPolicy.torch`. This KEP
   adds no entry to `pkg/runtime/framework/plugins/registry.go`.
3. **New runtime labels or conventions.** `trainer.kubeflow.org/framework` is unchanged.
4. **Supporting `accelerate launch`.** torchrun is the only launcher in scope. Accelerate's
   launcher and the launcher-level options that only it offers belong to a separate discussion,
   together with dynamic registration. See
   [Limitation: the TRL CLI and `accelerate launch`](#limitation-the-trl-cli-and-accelerate-launch)
   for why this KEP excludes it. See [Open Questions](#open-questions) for what support for it
   would involve.
5. **The SDK trainer hierarchy in Phase 1.** The `BaseTrainer` / `FuncTrainer` /
   `ConfigTrainer` refactor from [sdk#285](https://github.com/kubeflow/sdk/issues/285) is
   Phase 2. It happens later in this KEP or in a follow-up KEP. TRL does not need it to ship.
   sdk#285 still tracks the per-framework trainer classes (`TorchTrainer`, `DeepSpeedTrainer`,
   `JAXTrainer`, `XGBoostTrainer`), `RuntimeConfig` and runtime auto-discovery.
6. **Deprecating `CustomTrainer` or `BuiltinTrainer` in Phase 1.** Both stay supported and
   unchanged in Phase 1. Phase 2 replaces `BuiltinTrainer` with `ConfigTrainer`. See
   [Phase 2: the move to `ConfigTrainer`](#phase-2-the-move-to-configtrainer).
7. **In-tree Axolotl, LlamaFactory or Unsloth runtimes.** This KEP proposes only TRL. Other
   frameworks are follow-up work that uses the same extension point.
8. **Changes to `TrainJobTemplate`.**

## Proposal

Add a TRL runtime image and manifest on the server. On the client, replace the hardcoded
TorchTune branch with a framework registry that any config-driven framework can join. TRL is
the first framework to join it.

### User Stories

#### Story 1: Platform admin installs TRL

As a platform admin, I install the Trainer and get a `trl-distributed`
`ClusterTrainingRuntime` next to the TorchTune runtimes. I did not have to build an image or
write a plugin. Before anyone uses the SDK, I can check that the runtime is correct. To do this,
I submit a raw-YAML `TrainJob` against it, with `spec.initializer` set.

#### Story 2: Data scientist runs supervised fine-tuning

As a data scientist, I fine-tune a model with TRL from the SDK. I use the same
`BuiltinTrainer` that I already use for TorchTune, and I change only the config:

```python
from kubeflow.trainer import TrainerClient
from kubeflow.trainer.types import (
    BuiltinTrainer, HuggingFaceDatasetInitializer, HuggingFaceModelInitializer,
    Initializer, SFTTrainer, TRLConfig,
)

TrainerClient().train(
    runtime=TrainerClient().get_runtime("trl-distributed"),
    initializer=Initializer(
        model=HuggingFaceModelInitializer(storage_uri="hf://Qwen/Qwen2.5-0.5B"),
        dataset=HuggingFaceDatasetInitializer(storage_uri="hf://trl-lib/Capybara"),
    ),
    trainer=BuiltinTrainer(
        config=TRLConfig(
            trainer=SFTTrainer(packing=True),
            learning_rate=2e-5,
            num_nodes=2,
        ),
    ),
)
```

I give the model and dataset exactly as for TorchTune, through the `Initializer`.

#### Story 3: Data scientist runs GRPO

As a data scientist, I run a post-training method that the SDK cannot express today. My editor
tells me which parameters belong to GRPO. If I pass a DPO-only parameter, I see an error before
I submit the job. It does not become a flag that TRL silently ignores at runtime:

```python
from kubeflow.trainer import TrainerClient
from kubeflow.trainer.types import (
    BuiltinTrainer, HuggingFaceDatasetInitializer, HuggingFaceModelInitializer,
    GRPOTrainer, Initializer, TRLConfig,
)

TrainerClient().train(
    initializer=Initializer(
        model=HuggingFaceModelInitializer(storage_uri="hf://Qwen/Qwen2.5-0.5B"),
        dataset=HuggingFaceDatasetInitializer(storage_uri="hf://trl-lib/tldr"),
    ),
    trainer=BuiltinTrainer(
        config=TRLConfig(
            trainer=GRPOTrainer(
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

The config carries only TRL flags. The source of the model and dataset does not change.

#### Story 4: Community adds a framework out of tree

As a maintainer of another config-driven framework, I publish a runtime image labelled with my
framework. I also publish a `FrameworkConfig` subclass in my own package. Importing my package
registers it. Neither the Trainer controller nor the SDK needs a commit.

### Notes/Constraints/Caveats

**TRL loads models and datasets from local paths, not only the Hugging Face Hub.** TRL
documents its `--dataset_name` as "Path or name of the dataset to load". TRL passes
`--model_name_or_path` to `from_pretrained`, which accepts a local directory. So the
initializer-staged `/workspace/model` and `/workspace/dataset` work. The Trainer's existing
dataset and model initializers apply unchanged. Two caveats apply. First, TRL loads a local
dataset *directory* as parquet/JSON/CSV. So dataset repositories that ship a loading script are
not equivalent. Second, a dataset with several configurations also needs `--dataset_config`.

**A TRL upgrade is an image rebuild, not a controller release.** The SDK chooses which module
torchrun runs (`trl.scripts.sft` vs `.dpo` vs `.grpo`), and the image ships it. So the control
plane never learns a TRL version.

**The Trainer does not support elastic training for either launcher.** `torch.go:109` carries
the TODO to add it once JobSet supports elastic jobs. Choosing torchrun does not change this. See
[What torchrun gives up](#what-torchrun-gives-up).

### Risks and Mitigations

| Risk | Mitigation |
|---|---|
| **A silently degraded run looks like success.** The `trl-demo` failure exited 0 on both pods. So a two-node job that trained two isolated copies looked the same as one that trained jointly. The torchrun-first design removes that one cause, but not the whole class. | The E2E test checks the achieved world size from the logs, not just `Complete`. To close the class, the Trainer needs a runtime→status channel. That channel is out of scope here. See [Open Questions](#open-questions). |
| **TRL's surface is not frozen**, and typed configs that copy its flags will drift. TRL moved PPO to `trl.experimental`, then removed it from the package completely. | The configs target the *script flags*, not the Python API. So a TRL upgrade changes the image, not the SDK types. Drift stays inside the trainer config classes. The launch-path proof runs again on every `trl` version bump. |
| **TRL ignores unknown flags.** `trl/scripts/*.py` parse with `fail_with_unknown_args=False`. So TRL drops a misapplied flag and does not reject it. | With the typed trainer configs, a wrong-trainer parameter raises a `TypeError` in the SDK at construction time, before the TrainJob exists. This is the main reason for the config shape in [Choosing the config shape](#choosing-the-config-shape). |
| **`extra_args` reopens the silent-ignore hole** for any flag that goes through it. Those flags are not typed, and TRL will not reject them. | This is by design. It is the escape hatch for the 161 shared flags, and the docs mark it as unvalidated. The SDK still checks everything that the KEP types at construction. |
| **A framework registered twice** (in-tree and out-of-tree) claims the same label. | Registration is explicit and import-ordered. A later registration wins. This rule lets a fork shadow a stalled in-tree config. The behavior is documented, not silent. |

## Design Details

The two sides are independent, so this section describes them separately. The contract between
them is the framework label `trainer.kubeflow.org/framework` and the container `command`. The
SDK appends rendered arguments to this command.

### Server Side (`kubeflow/trainer`)

#### What the torch plugin already provides

The torch plugin (`pkg/runtime/framework/plugins/torch/torch.go`) processes any runtime that
declares `mlPolicy.torch`. Its `EnforceMLPolicy` injects these variables into the trainer
container:

| Env var | Value | Source |
|---|---|---|
| `PET_NNODES` | trainer PodSet count | `torch.go:147` |
| `PET_NPROC_PER_NODE` | `spec.trainer.numProcPerNode`, else `"auto"`, else the CPU count when no GPU | `torch.go:121-135`, `:150` |
| `PET_NODE_RANK` | field ref to the JobSet completion index | `torch.go:152-156` |
| `PET_MASTER_ADDR` | `<trainjob>-node-0-0.<trainjob>` | `torch.go:161-162` |
| `PET_MASTER_PORT` | `29500` | `torch.go:163-165` |

The plugin also opens port 29500 for the headless service. `Validate` rejects a `TrainJob` that
tries to set any of those five variables itself (`torch.go:67-77`, against
`constants.TorchRunReservedEnvNames`).

The plugin has exactly one framework-specific branch. When `spec.trainer.command` equals
`["tune", "run"]`, the plugin withholds `PET_MASTER_ADDR` / `PET_MASTER_PORT`. It also rewrites
the command with `--rdzv_endpoint=<host>:29500` plus the recipe, config and runtime-derived
overrides (`torch.go:175-201`). Every other command gets all five variables and no rewrite.

So a second framework inherits a complete description of the topology for free. **The only
question is whether its launcher reads it.**

#### `PET_*` is torchrun's private interface

`PET_*` is not a Kubeflow convention. It is how torchrun reads its own flags from the
environment. For every flag, torchrun also accepts a `PET_`-prefixed env var
(`torch.distributed.argparse_util`). So `PET_NNODES` is simply `--nnodes`. This puts `PET_*` at
a specific layer:

```
1. Torch plugin        →  sets PET_NNODES, PET_NPROC_PER_NODE, PET_NODE_RANK,
                          PET_MASTER_ADDR, PET_MASTER_PORT
2. Launcher (torchrun) →  reads PET_*, decides how many processes to spawn
3. Launcher            →  spawns them, exports RANK, WORLD_SIZE, LOCAL_RANK,
                          MASTER_ADDR, MASTER_PORT
4. Training script     →  reads those standard variables
```

`PET_*` is the launcher's **input**. `RANK` / `WORLD_SIZE` are its **output**. A framework that
does not launch through torchrun never reaches step 2. So nothing produces step 3.

TRL's *CLI* is such a case. `trl sft` passes control to `accelerate launch`. The file
`trl/cli/accelerate_launcher.py` resolves the training script and calls accelerate's
`launch_command`. **The accelerate launcher has no reference to `PET_` anywhere in its
source.** It is a *peer* of torchrun that takes topology from flags.

But the CLI is only a wrapper. The CLI wraps `trl/scripts/sft.py`, which is a **plain training
script**. It contains `TrlParser` argument parsing plus `SFTTrainer(...).train()`, and no
launcher logic. You can run it as a module. The accelerate *library* inside it (`PartialState`,
a hard dependency of TRL) detects torchrun's step-3 output from the environment. `LOCAL_RANK`
set → multi-GPU (nccl). `WORLD_SIZE > 1` on CPU → multi-CPU (gloo).

So TRL fits the diagram directly by making torchrun the launcher of TRL's own script:

```yaml
command: [torchrun, -m, trl.scripts.sft]
```

torchrun reads the plugin's `PET_*` natively. This includes `numProcPerNode: "auto"`, which
torchrun resolves itself. The in-process accelerate of the script reads what torchrun exports.
Nothing translates variables or commands, and the plugin does not change. This is the same path
that other torchrun-native frameworks in the Trainer already use, Megatron included.

#### Proof of concept

The proof of concept has three runs. All of them use the exact `PET_*` variables that the
unmodified torch plugin injects. The only difference is what consumed them.

- **Cluster, bare torchrun.** Two CPU pods reached `world_size=2`, and a Gloo all-reduce summed
  to `2.0`. This was real rendezvous from the env that the plugin injected. So the
  plugin→torchrun half holds on real infrastructure.
- **Container, `torchrun -m trl.scripts.sft`.** This is the exact command of the runtime. Only
  the five `PET_*` variables were set, with no topology flags. The first-step `epoch` moved from
  `0.05882` (1/17, single-node control) to `0.1111` (1/9). So the dataset sharded across 2
  ranks: **world size 2**.
- **Cluster, `trl sft` → `accelerate launch`.** The flags translated correctly per pod. But the
  run stayed at **world size 1**, while both pods reported `Succeeded` and the TrainJob reported
  `Complete`. This silent failure excludes the CLI.

Full write-up, the reproduction script (docker only, `--use_cpu`, no GPUs) and the draft image and
manifests: [`examples/trl/README.md` on the PoC branch](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md).

**What is not yet proven.** We verified the two halves separately: cross-pod rendezvous on the
cluster, and the TRL script under torchrun in a container. The remaining step is to run the TRL
script across two real pods. This step is the first item of the E2E phase.

#### Limitation: the TRL CLI and `accelerate launch`

The TRL CLI cannot be the runtime command. The `accelerate launch` run failed for two causes.
Both causes are in accelerate's launcher, and both **exit 0**. So JobSet observes completion,
the TrainJob reports `Succeeded`, and no signal reaches the control plane:

1. **`--use_cpu` disables every distributed path.** So a CPU run goes to the simple launcher,
   which never sets `RANK` / `WORLD_SIZE`. We demonstrated this on the cluster.
2. **accelerate infers multi-GPU from the local device count** in the current process. At one
   GPU per pod, this count is never above one.

Both are properties of the *launcher*. They disappear when torchrun launches the script. The
container run above used `--use_cpu` and still reached world size 2. This is because in-process
accelerate reads the env that torchrun already exported. The source-level detail is in the
[PoC write-up](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md).

Choosing torchrun also follows the pattern that the Trainer already uses. Megatron runs under the
torchrun launcher today. So TRL joins an existing path and does not introduce a second one.

##### What torchrun gives up

HF `TrainingArguments` also exposes most of what `accelerate launch` configures. So these settings
stay available as ordinary script flags: `--deepspeed`, `--fsdp`, `--fsdp_config`, `--bf16` /
`--fp16`, `--gradient_checkpointing`, `--torch_compile`, `--ddp_backend`. So DeepSpeed and FSDP
are **not** lost. What is really not available under torchrun:

| Not available | Impact |
|---|---|
| `--config_file` — one accelerate YAML per job | Cosmetic here. The same settings are available as script flags, and operators already express them in a runtime manifest. |
| `--use_megatron_lm` | The accelerate Megatron-LM plugin. Out of scope for TRL post-training. |
| `--tpu` | No TPU support in the Trainer's torch plugin either. |
| `--mixed_precision`, `--dynamo_backend` at launch time | Covered by `--bf16` / `--fp16` and `--torch_compile` on the script. |
| `--mpirun_hostfile` | Only needed because accelerate cannot do multi-process CPU without MPI. torchrun does it natively over gloo, so this is a limitation torchrun *removes*. |

Elasticity is not lost either. For multi-GPU and DeepSpeed, `accelerate launch` runs torchrun
itself and passes its elastic flags to torchrun without change. So a direct call to torchrun
gives the same behaviour with one less layer. In any case, the Trainer does not support elastic
training for either launcher (`torch.go:109`).

**In short: we do not use accelerate's launcher. torchrun launches TRL's script directly.**
accelerate is still in the image because TRL depends on it (`accelerate>=1.4.0`, and HF `Trainer`
cannot be imported without it). But in-process, it only reads the variables that torchrun
exported. This design excludes a second launcher, not a library.

#### What changes in the plugin

Nothing. `EnforceMLPolicy` today supports the torchrun launcher through the `PET_*` variables,
and that is what TRL needs. Every run in [Proof of concept](#proof-of-concept) used an
unmodified `torch.go`. `Validate` does not change either.

The difference from TorchTune is where the training arguments are built:

| | TorchTune | TRL |
|---|---|---|
| Launcher | `tune run` | `torchrun` |
| Rendezvous | a command-line argument, so the plugin rewrites the command (`torch.go:175-201`) | environment, so the command is left alone |
| Who builds the arguments | the plugin, in Go: `torchtune.go` picks the recipe, adds `--config`, and appends the path overrides read from the runtime | the SDK, in Python: `TRLConfig.to_args()`, passed through unchanged as `args` |
| Go needed | the existing branch | none |

So supporting TRL adds no logic to the controller.

#### New artifacts

There are two, and they mirror what TorchTune already has. The image has no extra script or
binary. The command is torchrun, which invokes TRL's training module.

**1. `cmd/trainers/trl/`** — a `Dockerfile` on the same PyTorch base as
`cmd/trainers/torchtune`. It installs pinned `trl` + dependencies. accelerate is one of these
dependencies. TRL requires it (`accelerate>=1.4.0`), and HF `Trainer` cannot be imported without
it. But the image uses accelerate only in-process and never invokes its launcher.

**2. `manifests/base/runtimes/trl/`** — a **single** `ClusterTrainingRuntime`, `trl-distributed`.
It keeps the same two initializer replicatedJobs as the TorchTune runtimes, but pins no
`STORAGE_URI`. The TrainJob supplies both. The YAML below omits them:

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

TorchTune ships one runtime per base model (`llama3_2`, `qwen2_5`), because its recipes and
configs *are* per model. `tune run` selects a recipe and a model-specific config file. So the
model family is fixed in the runtime.

TRL has no such axis. Its scripts are model-agnostic, and the model and dataset are ordinary
flags that the SDK sets per job. A copy of the TorchTune layout would therefore produce N
runtimes that differ only in a default flag value. The method is not an axis either. The
manifest's `sft` module is the raw-YAML default. An SDK-submitted `TrainJob` carries
`command: [torchrun, -m]`, with the trainer's module as the first entry of `args`
(`TRLConfig.to_args()`). So one runtime serves SFT, DPO and GRPO alike.

A runtime is still required, because the image must carry `trl` and its dependencies. The
default `torch-distributed` runtime cannot run TRL.

If a real axis appears later, the candidates are the *compute profile* (CPU, single-GPU,
multi-GPU/vLLM for GRPO rollouts), not the model family. That change would be additive: a second
runtime with the same framework label, selected with `train(runtime=...)`.

**Relationship to [#3718](https://github.com/kubeflow/trainer/pull/3718).** That PR proposes a
per-method GRPO runtime (`cmd/trainers/grpo/`, `manifests/base/runtimes/grpo/`) with its own
`train.py`. This KEP replaces that layout. One `trl-trainer` image and one `trl-distributed`
runtime cover GRPO through `trl.scripts.grpo`. There is no custom training script to maintain.
The GRPO work in #3718 lands as the `GRPOTrainer` and the GRPO E2E test, not as a separate image.

**The server side stops here.** It is an image and a manifest, with no control-plane code.
### Client Side (`kubeflow/sdk`)

#### Phases

| Phase | Trainer class | What ships |
|---|---|---|
| **Phase 1** (this KEP) | `BuiltinTrainer` | `FrameworkConfig`, the registry, and `TRLConfig` next to `TorchTuneConfig` |
| **Phase 2** (later, or a follow-up KEP) | `ConfigTrainer` | The `BaseTrainer` / `ConfigTrainer` / `FuncTrainer` hierarchy from sdk#285 |

Phase 1 gives users TRL now. It does not wait for the hierarchy refactor, and the refactor does
not block it. The rest of this section describes Phase 1.

**TRL is not supported in both `BuiltinTrainer` and `ConfigTrainer`.** When Phase 2 ships,
`TRLConfig` moves to `ConfigTrainer`, and `BuiltinTrainer` stops accepting it. The SDK does not
keep two code paths for one framework. Users who start with TRL in Phase 1 must change their
code in Phase 2. [Phase 2: the move to `ConfigTrainer`](#phase-2-the-move-to-configtrainer)
shows the change.

#### Current limitations

`BuiltinTrainer` has a single field, `config: TorchTuneConfig`. TorchTune is hardcoded at four
points:

| # | Coupling | Location |
|---|---|---|
| 1 | `BuiltinTrainer.config` annotated with the concrete `TorchTuneConfig` | `types.py:241` |
| 2 | Framework identifier derived by reflecting on that annotation (`BuiltinTrainer.__annotations__["config"].__name__`) | `types.py:245` |
| 3 | `trainer_type` and entrypoint selected by string-comparing the label against it | `utils.py:125-129`, `:152-159` |
| 4 | Config-to-argument translation guarded by `isinstance(..., TorchTuneConfig)` | `utils.py:465` |

Coupling #3 blocks everything else. `trainer_type` is not a field on the Runtime CR. Instead,
the SDK computes it as `BUILTIN_TRAINER if framework == TORCH_TUNE else
CUSTOM_TRAINER`. A runtime labelled `trl` therefore resolves to `CUSTOM_TRAINER` today. The SDK
would then try to build a training-function command for it. Until `get_runtime_trainer()`
changes, TorchTune is the only config-driven framework that can run.

#### `FrameworkConfig` and the registry

A config knows three things that nothing else knows. It knows the framework label it claims and
the entrypoint that runs it. It also knows how it renders itself as that entrypoint's arguments.
Together, these are all four couplings above, moved onto one object.

```python
@dataclass(kw_only=True)
class FrameworkConfig(abc.ABC):
    framework: ClassVar[str] = ""
    command: ClassVar[tuple[str, ...]] = ()

    @abstractmethod
    def to_args(self) -> list[str]:
        """Render this config's own fields as arguments for `command`."""

    def to_initializer_args(self, initializer: Optional["Initializer"]) -> list[str]:
        """Render the flags that point the framework at what the Initializer
        staged under /workspace. Default: none."""
        return []
```

There are two methods because the two inputs differ. `to_args()` is the config's own fields.
`to_initializer_args()` exists because the model and dataset flags still belong to the framework.
This is true even though the `Initializer` owns the download. TRL needs `--model_name_or_path`
and `--dataset_name`. TorchTune needs `dataset.data_files=`. Only the config knows its own
spelling. The staged paths are SDK constants (`constants.MODEL_PATH`, `DATASET_PATH`), so the
config never imports a backend.

The framework label resolves through a registry, not through a constant. The registry serves
exactly one lookup, in `get_runtime_trainer()`. It is also the out-of-tree extension path:

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

The registry uses an explicit decorator. This matches how the control plane registers its
plugins by hand in `plugins/registry.go`. [sdk#310](https://github.com/kubeflow/sdk/pull/310) is
the PoC for this shape. There is no automatic discovery. Code must import a config before it can
construct it, and the import registers it.

`TorchTuneConfig` keeps every field and its signature. It gains `framework = "torchtune"` and
`command = ("tune", "run")`. It also gains a `to_args()` that delegates to the existing emitter.
Last, it gains a `to_initializer_args()` that carries the `dataset.data_files=` / `data_dir=`
overrides it emits today.

#### `TRLConfig` and the trainer configs

`TRLConfig` holds the fields that all TRL trainers share. It also has one typed slot,
`trainer`, which holds the config for the TRL trainer to run. The class in that slot selects
the script module. The slot classes take their names from TRL's trainers: `SFTTrainer`,
`DPOTrainer` and `GRPOTrainer`. Users import them from `kubeflow.trainer.types`:

```python
from kubeflow.trainer.types import BuiltinTrainer, DPOTrainer, TRLConfig

BuiltinTrainer(config=TRLConfig(trainer=DPOTrainer(beta=0.1), learning_rate=5e-7))
```

```python
# kubeflow/trainer/types/trl.py

@dataclass(kw_only=True)
class TRLTrainerConfig(abc.ABC):
    """Kubeflow-owned config for one TRL trainer. The subclass selects trl.scripts.<module>."""
    module: ClassVar[str]

    @abstractmethod
    def to_args(self) -> list[str]:
        """Render the trainer-specific flags."""


@dataclass(kw_only=True)
class SFTTrainer(TRLTrainerConfig):
    module: ClassVar[str] = "sft"
    loss_type: Optional[Literal["nll", "dft", "chunked_nll"]] = None
    packing: Optional[bool] = None
    completion_only_loss: Optional[bool] = None


@dataclass(kw_only=True)
class DPOTrainer(TRLTrainerConfig):
    module: ClassVar[str] = "dpo"
    beta: Optional[float] = None                    # deviation from the reference model
    loss_type: Optional[list[str]] = None           # combinable; weighted by loss_weights
    loss_weights: Optional[list[float]] = None


@dataclass(kw_only=True)
class GRPOTrainer(TRLTrainerConfig):
    module: ClassVar[str] = "grpo"
    beta: Optional[float] = None                    # KL coefficient
    loss_type: Optional[Literal[
        "grpo", "dr_grpo", "dapo", "bnpo", "cispo", "sapo", "luspo", "vespo"]] = None
    num_generations: Optional[int] = None
    # Upstream reward_funcs lives on GRPOScriptArguments, not GRPOConfig;
    # merged here on purpose so one object covers the trainer's whole surface.
    reward_funcs: Optional[list[str]] = None


@register_framework
@dataclass(kw_only=True)
class TRLConfig(FrameworkConfig):
    framework: ClassVar[str] = "trl"
    # torchrun launches TRL's plain training script as a module; to_args()
    # supplies the module name as the first positional argument, so the full
    # argv is `torchrun -m trl.scripts.<module> --flag value ...`.
    command: ClassVar[tuple[str, ...]] = ("torchrun", "-m")

    trainer: TRLTrainerConfig                        # the typed slot

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
        (torchrun's -m positional), then shared flags, the trainer's own, then
        extra_args. Bools become store_true flags, lists expand after their
        flag. num_nodes and resources_per_node are never rendered."""
        return (
            [f"trl.scripts.{self.trainer.module}"]
            + self._shared_args()
            + self.trainer.to_args()
            + self._extra_args()
        )

    def to_initializer_args(self, initializer) -> list[str]:
        """--model_name_or_path=/workspace/model and
        --dataset_name=/workspace/dataset/<subpath>, using the same subpath
        helper as TorchTuneConfig. Absent initializer: raises ValueError."""
```

##### Why the shared fields sit on the outer config

The fields on `TRLConfig` are the ones that TRL itself shares. `SFTConfig`, `DPOConfig` and
`GRPOConfig` all inherit HF `TrainingArguments`. TRL's own `GRPOConfig` docstring says it
"includes only the parameters that are specific to GRPO training. For a full list of training
arguments, please refer to the `TrainingArguments` documentation." If we split along the same
seam, we declare each field once. The split then matches upstream, and we do not invent one.

The split is also lopsided, and that makes it worth making once. **161 flags are shared**
(`TrainingArguments` 133, `ModelConfig` 19, `ScriptArguments` 6, `DatasetMixtureConfig` 3).
Against this, 11 to 79 flags are method-specific, and GRPO has the 79. Under a per-method design,
those 161 would be re-frozen in every exported constructor.

The model and dataset are not fields at all. See
[Where the model and dataset come from](#where-the-model-and-dataset-come-from).

`num_nodes` and `resources_per_node` are not TRL flags at all. They are Kubeflow placement
knobs. They sit here because `TorchTuneConfig` already carries them and `BuiltinTrainer` has no
fields of its own. `to_args()` never renders them.

`extra_args` is the escape hatch for the shared flags that this class does not type. 161 flags
are too many to list. A user who needs `--gradient_accumulation_steps` should not have to wait
for an SDK release. The SDK renders it last, so it can also override anything above it.

##### Where the model and dataset come from

They come from `train(initializer=...)`, and from nowhere else. This is exactly as for
TorchTune, whose config has no model or dataset fields either. The SDK writes the `Initializer`
into `TrainJob.spec.initializer` as it does today. The runtime's initializer jobs download into
`/workspace`. `TRLConfig.to_initializer_args()` renders `--model_name_or_path=/workspace/model`
and `--dataset_name=/workspace/dataset/<subpath>`. The subpath derivation already exists for
TorchTune's `dataset.data_files=` override. It becomes a shared helper, not a second copy.

There are three reasons for having no string fields on the config:

- **One habit for both frameworks.** A TorchTune user already knows how to give TRL a model.
- **A string is ambiguous.** `Qwen/Qwen2.5-0.5B` is a hub id, `/mnt/data/model` is a folder,
  `s3://bucket/model` needs credentials. The SDK would have to guess. `Initializer` already knows,
  and already covers S3, the data cache, and secrets for gated models.
- **No tie-break rule.** Two ways to name a model need a rule for when both are set. One way
  does not.

If a user submits a `TRLConfig` without an `Initializer`, the SDK raises a `ValueError`.
Otherwise, the runtime's initializer jobs would start with no `STORAGE_URI` and fail late. A
shortcut such as `Initializer.from_hf(model=..., dataset=...)` can come later. It would help
TorchTune users equally. It is not part of this KEP.

##### Kubeflow-owned trainer configs

Kubeflow defines its own config classes. They hold only the TRL fields that Kubeflow supports.
We do not re-export TRL's config objects, and we do not copy all of their fields.

- **TRL field names.** Each field uses TRL's exact name, for example `beta`, `loss_type` and
  `num_generations`. A TRL user does not learn new names, and `to_args()` renders each field as
  the TRL flag with the same name.
- **A subset.** A field goes into a class only when Kubeflow tests it. Other flags go through
  `extra_args`.
- **No TRL import.** The SDK does not depend on the `trl` package. The TRL version is pinned in
  the runtime image, not in the SDK.
- **Adding a field.** Adding a field is an additive SDK change. It needs the field, a
  `to_args()` test, and a check against the pinned TRL version.

This keeps the public SDK API small and stable. When TRL renames or removes a field, only that
field in the Kubeflow class changes, not the whole surface.

#### Choosing the config shape

We considered four shapes. The one above is C. The full comparison, with the upstream field
counts behind it, is in the [design note](https://claude.ai/code/artifact/29468417-11ca-42d8-a8d2-b23c0939a14c).

| | **A. Flat + method enum** | **B. Per-method configs** | **C. Typed slot** (chosen) | **D. C + shared model/dataset specs** |
|---|---|---|---|---|
| Call | `TRLConfig(method=TRLMethod.GRPO, beta=0.04)` | `BuiltinTrainer(config=GRPOTrainer(...))` | `TRLConfig(trainer=GRPOTrainer(...), ...)` | four nested objects |
| Objects per job | 1 | 1 | 2 | 4 |
| Wrong-method parameter | accepted, then **silently dropped by TRL** | `TypeError` at construction | `TypeError` at construction | as C |
| Conflicting field types | **cannot be typed** | each declared once, correctly | each declared once, correctly | as C |
| The 161 shared flags | one signature | **re-frozen in every export** | one signature | one signature |
| Registry | one config per framework ✓ | **N configs claim one label** | one config per framework ✓ | one config per framework ✓ |
| Adding a method | one enum member + one map entry | one dataclass + a new export + registry change | one dataclass | one dataclass |

**Why A cannot be typed.** The same parameter name has a different type and different valid
values per method. We verified this against TRL's config sources:

| Field | `SFTConfig` | `DPOConfig` | `GRPOConfig` |
|---|---|---|---|
| `loss_type` | `str \| None`: `'nll'`, `'dft'`, `'chunked_nll'` | `list[str]`, default `["sigmoid"]`: 15 values, combinable | `str`, default `"dapo"`: `'grpo'`, `'dr_grpo'`, `'dapo'`, `'bnpo'`, `'cispo'`, … |
| `beta` | — | `float = 0.1`, deviation from the **reference model** | `float = 0.0`, **KL coefficient** |
| `shuffle_dataset` | `bool = False` | — | `bool \| None = True` |

A flat dataclass must pick one annotation for `loss_type`. So it is either wrong for two
methods or degraded to `Any`. Also, `beta` is one name for two different quantities with
different defaults. No annotation can disambiguate them.

**Why the type error matters here specifically.** TRL's scripts call
`parser.parse_args_and_config(fail_with_unknown_args=False)`. So when an argument does not
belong to the method that TRL runs, TRL **ignores** it. It does not reject it. A user who sets `num_generations` on an
SFT job gets a successful run that silently ignored it. Shape A pushes that detection nowhere.
The typed slot turns it into a `TypeError` before the TrainJob is created.

The current draft's `_METHOD_SCOPED_FIELDS` map was an attempt to recover this at runtime. We
have to maintain this information either way. If we express it as subclasses, we remove a
hand-maintained map, and we do not add one.

**Why not B.** B reads best at the call site, and it is closest to TRL's own shape. But the
registry is keyed one config per framework label. If three classes claim `trl`, the registry
needs a redesign to represent one framework's internal methods. B would also copy the 161 shared
flags into every public signature. So one new shared field would touch three frozen
constructors, then six, then eighteen. The other choice is to move them into a shared base. At
that point the base *is* the outer config, without the single entry point.

**Why not D.** It is tempting to group the shared fields further into model, dataset and
training specs. But the surface is shared only as a concept. TRL wants `--dataset_name`, and
LlamaFactory wants `--dataset`. TorchTune wants Hydra overrides like `model.lora_rank=16`, and it
could not use the block at all. D also costs four objects per job for one framework. And it is a
one-way door: moving fields into `config.model` later is a breaking change.

**Precedent.** The typed slot is the ordinary shape for "one of N variants, plus shared
settings". Keras `compile(optimizer=...)` takes an `Optimizer` instance with the shared
hyperparameters on the base class. `tf.data.Options` holds nested typed option objects
(`autotune`, `threading`, `experimental_optimization`). The scikit-learn meta-estimators take
estimator *objects* as parameters (`GridSearchCV(estimator=..., cv=...)`). In the Trainer's own
API, `MLPolicy` carries an `MLPolicySource` union whose member selects the behaviour. TRL itself
uses this shape, with one config class per trainer.

##### Naming the slot: `trainer`

The slot and its classes use TRL's trainer names. TRL has one trainer per technique
(`SFTTrainer`, `DPOTrainer`, `GRPOTrainer`), and its users already know those names. So the
slot is `trainer`, and the Kubeflow classes are `SFTTrainer`, `DPOTrainer` and `GRPOTrainer`.
This follows the guiding principle of this KEP: for now, stay consistent with TRL.

The classes are Kubeflow-owned. They are not TRL's classes and do not import TRL. See
[Kubeflow-owned trainer configs](#kubeflow-owned-trainer-configs).

Two name clashes are known:

- **TRL's own classes.** `trl.DPOTrainer` and `kubeflow.trainer.types.DPOTrainer` share a
  name. In a notebook, the last import wins. Users who import both must use the module
  namespace, for example `types.DPOTrainer`.
- **The SDK `*Trainer` suffix.** In the SDK today, a `*Trainer` class (`CustomTrainer`,
  `BuiltinTrainer`) is the object a user passes to `train(trainer=...)`. `DPOTrainer` goes one
  level deeper, inside `TRLConfig`. So `train(trainer=...)` and `TRLConfig(trainer=...)` use
  one word for two levels.

[Open Questions](#open-questions) records both clashes. The fallback is `*Config` names such as
`TRLDPOConfig`.

#### Wiring it up

In Phase 1, `BuiltinTrainer` stays where it is and keeps its construction signature. Only its
`config` field widens from `TorchTuneConfig` to `FrameworkConfig`. Both entry points then
accept any framework:

```python
BuiltinTrainer(config=TorchTuneConfig(...))   # today, still valid
BuiltinTrainer(config=TRLConfig(...))         # new, same machinery
```

Three call sites change, and each one *loses* a branch:

- **`get_runtime_trainer()` resolves the command through the registry.** This replaces the
  `framework == TORCH_TUNE` branch at `utils.py:152-159` and removes
  `constants.TORCH_TUNE_COMMAND`:

  ```python
  if config_cls := registry.get_framework(framework):
      trainer.set_command(config_cls.command)
  elif ml_policy.torch is not None:
      trainer.set_command(constants.TORCH_COMMAND)
  # ... mpi, default unchanged
  ```

- **`trainer_type` comes from the registry.** It is `BUILTIN_TRAINER` when `get_framework()`
  finds a registered config, and `CUSTOM_TRAINER` otherwise. This replaces `utils.py:125-129` and
  removes the `TORCH_TUNE` constant. The framework label stays the only discovery key.

- **The backend has no framework branch.** One line removes the `isinstance` check at
  `utils.py:465`, and it does not move the check to a new place. The line is
  `trainer_cr.args = config.to_args() + config.to_initializer_args(initializer)`. `command` still
  comes from the runtime, exactly as today.

`RuntimeTrainer.command` stays the field that consumers read. It is not config-specific.
`command[0] == "mpirun"` drives MPI detection. `get_runtime_packages` rewrites it for
single-process. The localprocess backend joins it into an entrypoint. If we moved it onto the
config, all of those would need a special case.

After this, a new framework adds no lines to `utils.py` and none to the backends. That is the
one-time change this KEP pays for.

Two details stay the same as on the TorchTune path:

- **Initializer-derived path arguments move onto the config.** Today, the backend computes
  TorchTune's `dataset.data_files=` / `data_dir=` overrides from the HF dataset initializer.
  These overrides become `TorchTuneConfig.to_initializer_args()`. TRL's `--model_name_or_path` /
  `--dataset_name` are `TRLConfig.to_initializer_args()`. Both use one shared subpath helper,
  and the backend stops knowing either spelling.
- **`TorchTuneConfig.to_args()` delegates to the existing emitter.** `get_args_from_peft_config`
  maps `LoraConfig` onto nested `model.*` keys. A flat `key=value` walk would emit `lora_rank=8`
  instead of `model.lora_rank=8`, and the job would fail. The emitters move verbatim to
  `types/torchtune.py`. They hold no Kubernetes logic. If they stayed in the backend, `types.py`
  would have to import a backend module.

`LoraConfig` is **not** reused for TRL. It is TorchTune-shaped. `apply_lora_to_output` and
`quantize_base` have no TRL analogue. TRL's PEFT surface is `--use_peft` / `--lora_r` /
`--lora_alpha` / `--lora_target_modules`. Sharing it would need a lossy translation.

#### Which framework, and why TRL

| Framework | Post-training methods | Maintenance | Argument shape | torchrun path |
|---|---|---|---|---|
| TRL | one plain script per method under `trl/scripts/` — `sft`, `dpo`, `grpo`, `kto`, `reward`, `rloo`. PPO was moved to `trl.experimental` ([trl#4466](https://github.com/huggingface/trl/issues/4466)) and has since been removed entirely | Active (Hugging Face) | flags | native: `torchrun -m trl.scripts.<method>` |
| TorchTune | SFT only, in the Kubeflow integration | Stopped 15 Jul 2025 ([#2883](https://github.com/meta-pytorch/torchtune/issues/2883)) | flags | via `tune run` (plugin rewrites the command) |
| LlamaFactory | Broad, but built on HF `Trainer` and PEFT | Active | config file | wrapped: needs `FORCE_TORCHRUN=1` and renamed env |
| Axolotl | Broad, but its GRPO is TRL's `GRPOTrainer` | Active | config file | native with `--launcher torchrun` (default is accelerate) |
| Unsloth | None of its own. It is an acceleration layer | Active | n/a | n/a — not a launcher |

We selected TRL on three grounds. First, its per-method scripts cover what the SDK cannot
express at all today: DPO and GRPO. Second, it is the engine, not a wrapper. LlamaFactory and
Axolotl are both built on the HF `Trainer` and PEFT. So an in-tree TRL trainer adds capability.
An in-tree wrapper would add a second surface over the same engine. Third, its scripts take
flags, so `TRLConfig` renders straight into `args`. A file-first framework would need the SDK to
stage a ConfigMap or volume first.

Choosing TRL first closes no doors. Every alternative reaches the SDK out of tree as a
`FrameworkConfig` subclass. That is why LlamaFactory is the reference out-of-tree framework. An
Unsloth-accelerated image is still labelled `framework: trl`. So it resolves to the same config,
and the user chooses it with `train(runtime=...)`. It needs no SDK field.

### Migration and Backward Compatibility

**Server side.** Additive. The change touches no existing code path.

| Aspect | Impact |
|---|---|
| CRDs | **No change.** No new or modified fields, no version bump. |
| Torch plugin | **No change.** The change touches neither `EnforceMLPolicy` nor `Validate`. TRL falls through the default path. A unit test asserts it stays that way. |
| Plugin registry | **No change.** TRL reuses the torch plugin via `mlPolicy.torch`. |
| Existing TorchTune runtimes | **No change** in behaviour. Same images and rendered `TrainJob`s. They get the deprecated label when TRL support lands. See [Deprecation](#deprecation). |
| Default manifests | **Additive.** A new runtime under a new name. The change renames or removes nothing. |

**Client side, Phase 1.** Additive.

| Aspect | Impact |
|---|---|
| `CustomTrainer`, `CustomTrainerContainer` | **No change.** All fields retained. |
| `BuiltinTrainer` | **Construction signature unchanged**. It produces byte-identical `TrainJob` arguments for TorchTune. `config` widens to `FrameworkConfig`. Not deprecated in Phase 1. |
| `TorchTuneConfig` | **No change** to fields or signature. It gains two `ClassVar`s, `to_args()` and `to_initializer_args()`. Deprecated with the TorchTune runtimes. See [Deprecation](#deprecation). |
| `TrainerClient.train()` | **No change** to the signature. |
| `TrainJobTemplate` | **No change.** |
| Python version | No new floor. `kw_only=True` needs 3.10, which is already the SDK's minimum. |
| Public exports | New names only: `FrameworkConfig`, `TRLConfig`, `TRLTrainerConfig`, `SFTTrainer`, `DPOTrainer`, `GRPOTrainer`. Nothing removed or renamed. |

#### Phase 2: the move to `ConfigTrainer`

Phase 2 is a **breaking change** for SDK users. `ConfigTrainer` replaces `BuiltinTrainer`. A
config that works with `BuiltinTrainer` in Phase 1 must move to `ConfigTrainer`:

```python
# Phase 1
TrainerClient().train(
    initializer=initializer,
    trainer=BuiltinTrainer(config=TRLConfig(...)),
)

# Phase 2
TrainerClient().train(
    initializer=initializer,
    trainer=ConfigTrainer(config=TRLConfig(...)),
)
```

What users must change:

- Replace `BuiltinTrainer` with `ConfigTrainer`. The config object stays the same.
- TorchTune users make the same change, as long as TorchTune is still supported (see
  [Deprecation](#deprecation)).

What does not change:

- `TRLConfig`, its `trainer` slot and its fields.
- The runtime, the framework label and the rendered `TrainJob`.

The SDK has no written policy for breaking API changes. `RELEASE.md` only defines version
numbers, and the SDK is at `0.x`. This KEP proposes this rule for Phase 2:

1. The release that adds `ConfigTrainer` also makes `BuiltinTrainer` show a deprecation
   warning.
2. `BuiltinTrainer` stays for at least two SDK minor releases after that release. This is the
   same window as the Trainer's runtime deprecation policy.
3. After that, a minor release can remove `BuiltinTrainer`.

The Phase 2 design (class names, fields, the `FuncTrainer` side) is decided in Phase 2, not
here.

### Test Plan

**Server-side Go unit tests** (`torch_test.go`, extending the table-driven cases). The tests
assert two cases side by side, so the two paths stay visibly distinct. In the first case, the
plugin leaves a `[torchrun, -m, trl.scripts.sft]` command alone. That command gets all five
`PET_*`, including the master vars that the TorchTune path suppresses. In the second case, the
plugin still rewrites a `[tune, run]` command. The first case is the regression guard for
[What changes in the plugin](#what-changes-in-the-plugin). It fails as soon as anyone adds a
TRL branch to the plugin.

**Manifest tests** — `manifests/base/runtimes/trl/` carries `framework: trl`,
`mlPolicy.torch: {}`, `command: [torchrun, -m, trl.scripts.sft]`, and mounts `initializer` at
`/workspace`.

**Launch-path test** — the proof script from the [PoC branch](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md),
moved into `examples/trl/` as part of S1. The test runs two `torchrun -m trl.scripts.sft`
launches with only the five `PET_*` variables set. The two launches must form world size 2. The
per-step epoch value (`0.0588` → `0.1111`) asserts this. The single-node control run guards the
assertion itself. Re-run the test on every `trl` version bump in `requirements.txt`.

#### Unit Tests

Client side:

- `to_args()` for each of `SFTTrainer`, `DPOTrainer`, `GRPOTrainer`: the module name is
  the first element. Bools render as store_true flags. Lists expand after their flag.
- Passing a parameter that belongs to another trainer raises `TypeError`.
- `TRLConfig.to_initializer_args()` raises `ValueError` without an `Initializer`. With one, it
  yields the `/workspace` model and dataset paths.
- `TorchTuneConfig.to_initializer_args()` matches the `dataset.data_files=` / `data_dir=` output
  of the current backend code byte for byte.
- `num_nodes` and `resources_per_node` never appear in `to_args()` output. `extra_args` renders
  last.
- `register_framework` rejects a config declaring no framework or no command.
- `get_runtime_trainer()` resolves `trainer_type` and `command` from the registry for both
  `torchtune` and `trl`, and falls through to torch/mpi/default for unregistered labels.
- `TorchTuneConfig.to_args()` emits nested `model.*` keys for `LoraConfig`, matching the
  current emitter byte for byte.

#### Integration tests

- With the `trl-distributed` runtime installed, `BuiltinTrainer(config=TRLConfig(...))` resolves
  it by label and yields `trainer_type == BUILTIN_TRAINER`. It emits `command: [torchrun, -m]`
  with `args == config.to_args() + config.to_initializer_args(initializer)` (module name first).
  This is the **cross-repo contract**: it fails if either side changes the label or the launch
  shape.
- `BuiltinTrainer(config=TorchTuneConfig(...))` produces byte-identical `TrainJob` arguments to
  the current implementation.

#### E2E tests

- A two-node TRL `TrainJob` reaches `Complete` **and** its logs show the two-node epoch
  increment. A check of completion alone would have passed for the accelerate failure in
  [Limitation: the TRL CLI and `accelerate launch`](#limitation-the-trl-cli-and-accelerate-launch).
  The world-size assertion is the part that catches that failure. The test runs on CPU. The
  launch path needs no GPU gate.
- `dpo` and `grpo` exercised end to end, not only `sft` (Beta).

**Backward compatibility** — every existing `CustomTrainer`, `BuiltinTrainer` and
`TrainJobTemplate` test passes without changes.

### Implementation Plan

The two sides are independent. The server side ships first, for two reasons. It is smaller.
Also, the client-side integration tests need a TRL runtime to run against.

**Server side**

| Phase | Contents |
|---|---|
| S1 | `cmd/trainers/trl/` with pinned versions and the launch-path proof script, both promoted from the [PoC branch](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md). Image publishing in the existing workflow |
| S2 | `manifests/base/runtimes/trl/` + kustomization entry, manifest tests, an `examples/trl/` TrainJob |
| S3 | The `torch_test.go` regression cases, then the two-node cluster E2E (CPU is enough, because the launch path is device-agnostic) |

S1 and S2 must ship together. A manifest that references an unpublished image is untestable.
An image with no manifest is unreachable.

**Client side**

| Phase | Contents |
|---|---|
| C1 | `FrameworkConfig`, the registry, `TorchTuneConfig` migrated onto it, `BuiltinTrainer.config` widened, the three call-site changes, and unit tests. No user-visible change yet. |
| C2 | `TRLConfig` and the three trainer configs. Public exports. Integration tests, including the cross-repo contract |
| C3 | Docs on sdk.kubeflow.org and an operator guide for building a runtime image for a new framework |

C1 is a pure refactor with no behaviour change, so it can merge on its own. C2 makes TRL
reachable from Python.

C1 to C3 are Phase 1. Phase 2 (`ConfigTrainer`) is planned after them, in this KEP or a
follow-up KEP. It does not block any C phase.

### Graduation Criteria

**Alpha**

- *Server:* the `trl-trainer` image is published and the `trl-distributed` runtime installs by
  default. A two-node TRL job completes with `world_size == numNodes × numProcPerNode`. E2E
  verifies this value, and does not rely on the job status. The torch plugin is unchanged, and a
  test asserts it.
- *Client:* `FrameworkConfig` and the registry are implemented. `TorchTuneConfig` runs through
  them with byte-identical output. `TRLConfig` with SFT is exported and works end to end. All
  existing tests pass.

**Beta**

- `dpo` and `grpo` exercised end to end, not only `sft`.
- An out-of-tree config, published **paired with an out-of-tree runtime image**. This proves both
  halves of the extension path. LlamaFactory is the reference: it is torchrun-wrapped rather
  than torchrun-native.
- The torchrun-first launch pattern documented on kubeflow.org as operator guidance for building
  a runtime image.
- SDK reference docs for `TRLConfig` and the trainer configs.

**GA**

- `TRLConfig` stable for a release, with the TRL version pin exercised by CI on every bump.
- At least one community `FrameworkConfig` subclass in the wild.
- A runtime can report its achieved world size to `TrainJob` status. Then a run that degrades
  silently is observable (see [Open Questions](#open-questions)).

#### Deprecation

**TorchTune is deprecated when TRL support lands**, because TorchTune is end-of-life. Upstream
stopped active development on 15 July 2025. It promised only critical bug fixes and security
patches during 2025, and no new features
([torchtune#2883](https://github.com/meta-pytorch/torchtune/issues/2883)).

"TRL support lands" means the TRL alpha criteria above are met. At that point, the
deprecation follows the
[Runtime Deprecation Policy](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/):

1. The TorchTune `ClusterTrainingRuntime`s under `manifests/base/runtimes/torchtune/` get the
   label `trainer.kubeflow.org/support: "deprecated"`.
2. The Trainer release notes list the deprecation as a breaking change.
3. The validation webhook warns when a deprecated runtime is deployed, and when a TrainJob
   uses one. This warning exists today.
4. The SDK warns when a deprecated runtime is listed or used. This warning does not exist
   yet. It is follow-up work for the deprecation, and it does not block TRL.
5. The TorchTune runtimes can be removed two Trainer minor releases after the deprecation, not
   earlier.

`TorchTuneConfig` stays in the SDK until its runtimes are removed. It then follows the same
SDK rule as `BuiltinTrainer` in [Phase 2](#phase-2-the-move-to-configtrainer): a deprecation
warning first, and removal no earlier than two SDK minor releases later.

## Open Questions

**Guiding principle.** For now, this KEP optimises for consistency with TRL. Where a choice is
open, we pick what TRL does: its trainer names, its field names and its scripts. We revisit
this when a second config-driven framework is in scope.

1. **Should a runtime report its achieved world size?** This is the largest gap left open, and
   it is not TRL-specific. The `trl-demo` failure exited 0 on both pods. So a two-node job that
   trained two isolated copies looked the same as one that trained jointly. torchrun-first
   removes that particular cause, but not the class. Any image whose launch degrades silently
   produces the same signature. A minimal mechanism: rank 0 reports the world size it joined
   with. The controller shows it as a status condition. The controller fails the job when it
   disagrees with `numNodes × numProcPerNode`. That is a control-plane change, and it is out of
   scope here. But without it, "the TrainJob succeeded" is not evidence that distributed
   training happened. [KEP-2779](../2779-trainjob-progress/README.md) already establishes a
   runtime→status channel that this mechanism could reuse.
2. **How far does dynamic registration go?** *Partly answered.* The SDK registry in this KEP
   is explicit: importing a config registers it. The server side shows that a new runtime needs
   no control-plane registration, only an image and a manifest. Open:
   - Should the SDK discover frameworks automatically, for example through Python entry
     points, instead of by import?
   - Should the Trainer *provision* runtimes for frameworks it does not ship, for example from
     a catalog CR?
3. **Should accelerate get its own plugin later?** The launcher-only options are in
   [What torchrun gives up](#what-torchrun-gives-up). They are an accelerate config file per
   job, the Megatron-LM plugin and TPU. These options are the reasons the question will return.
   On the SDK call, the position was that a dedicated accelerate plugin is not objectionable in
   principle. The ask was to understand the torchrun limitations first. This KEP now records
   them. This KEP deliberately keeps the question separate from shipping TRL, and does the same
   with dynamic registration (see #2). Both would need their own design discussion, and neither
   blocks TRL.
4. **Flat exports, or a `kubeflow.trainer.trl` submodule?** `SFTTrainer` in the existing flat
   `__all__` matches today's `__init__.py`. A submodule keeps the top level at one name per
   framework. The choice is frozen once it ships, so it needs a decision before the first
   release, not after.
5. **Do we expose experimental TRL methods?** PPO is the warning case. TRL moved it to
   `trl.experimental` ([trl#4466](https://github.com/huggingface/trl/issues/4466)), and later
   removed it from the package entirely. If we export a config for a method and upstream later
   drops that method, we still hold a public name. The proposal is to export only methods with a
   stable `trl/scripts/` module. If we ever want experimental methods, the proposal is to put
   them in a namespace that carries no stability promise.
6. **Does a compute-profile runtime axis appear for GRPO?** GRPO rollouts can use a vLLM server,
   which may justify a second TRL runtime. Deferred until there is a working GRPO E2E to measure.
7. **Which frameworks after TRL?** This KEP ships only TRL in tree. LlamaFactory, Axolotl and
   Unsloth are candidates (see [Which framework, and why TRL](#which-framework-and-why-trl)).
   Open: which of them, if any, Kubeflow ships in tree, and which stay out of tree as a
   `FrameworkConfig` subclass. An open point from the call: Unsloth may fit better as an option
   of the TRL runtime than as a separate framework.
8. **Which TRL trainers after SFT, DPO and GRPO?** TRL also has stable `RewardTrainer` and
   `RLOOTrainer`, each with a `trl/scripts/` module. `KTOTrainer` is exported at the top level,
   but a copy also exists under `trl/experimental/`, so its status needs a check. ORPO is
   experimental. Open: which of these Kubeflow adds, and when.
9. **Are the `*Trainer` slot names final?** The call chose TRL's trainer names (`DPOTrainer`).
   Two clashes are known (see [Naming the slot: `trainer`](#naming-the-slot-trainer)):
   `trl.DPOTrainer` in the same notebook, and the SDK's `*Trainer` suffix for objects passed to
   `train(trainer=...)`. The fallback is `*Config` names such as `TRLDPOConfig`. This needs a
   decision before the first release, because public names are frozen once shipped.
10. **Is the Phase 2 deprecation rule right?** The SDK has no written policy for breaking
    changes. This KEP proposes a warning first, and removal no earlier than two SDK minor
    releases later (see [Phase 2](#phase-2-the-move-to-configtrainer)). Maintainers should
    approve the rule or name a better one.
## Implementation History

- **2026-02-11**: KEP drafted in `kubeflow/sdk` as `docs/proposals/285-specialized-trainers`,
  scoped SDK-only.
- **2026-08-16**: transferred to `kubeflow/trainer` as KEP-2839 and extended with the server
  side ([#3930](https://github.com/kubeflow/trainer/pull/3930)). It supersedes
  [#3263](https://github.com/kubeflow/trainer/pull/3263).
- **2026-08-23**: proof of concept built on the `poc/trl-torch-plugin` branch (pinned at
  [`01b96f3`](https://github.com/YassinNouh21/trainer/tree/01b96f38608467d96acfc01c7500d49ebeca5d6f)).
  It confirmed torchrun as the launcher and recorded the `accelerate launch` route as a
  limitation.
- **2026-08-30**: config-shape options compared in a
  [design note](https://claude.ai/code/artifact/29468417-11ca-42d8-a8d2-b23c0939a14c). We chose
  the typed slot.
- **2026-09-09**: model and dataset come from `Initializer` only, as for TorchTune. `TRLConfig`
  has no string fields.
- **2026-09-02**: reviewed on the Kubeflow Trainer community call. The client-side design here
  supersedes the parallel SDK proposal in
  [sdk#627](https://github.com/kubeflow/sdk/pull/627), which covered the same ground. Agreed:
  restructure to the community KEP template, with the server and client designs separated. Drop
  the general SDK trainer hierarchy inherited from sdk#285. Give `TRLConfig` a typed per-method
  slot instead of a flat config with a method enum. Do **not** mirror TorchTune's
  per-model-family runtime layout, because the model family is not a TRL axis.
- **2026-10**: reviewed on the Kubeflow Trainer community call, with the goal to merge in
  November. Agreed: Phase 1 ships TRL through `BuiltinTrainer`, and the `ConfigTrainer`
  refactor is Phase 2 or a follow-up KEP. TRL is not supported in both trainers. The slot uses
  TRL's trainer names (`trainer=DPOTrainer(...)`). Kubeflow owns its config classes, with a
  subset of TRL's fields. TorchTune is deprecated once TRL lands. The server side is unchanged.

## Drawbacks

- **A second config-driven framework is a second thing to keep working.** The TRL version pin
  must track upstream. TRL moves fast: it moved PPO to a new location in a minor release. To
  reduce this risk, the pin lives in an image and in a proof script that CI runs, not in the
  control plane.
- **The SDK gains an indirection.** `BuiltinTrainer.config` is no longer one concrete type. To
  read the code, you must follow the registry to find what a label resolves to. The
  alternative is a branch per framework in shared code, which is what this replaces.
- **Two repositories must stay in step** on the framework label and the launch shape. One
  integration test asserts the contract, but it can only run where both are installed.
- **Phase 1 TRL users must change their code in Phase 2.** TRL is not supported in both
  `BuiltinTrainer` and `ConfigTrainer`, so these users must replace `BuiltinTrainer` with
  `ConfigTrainer`. We accept this cost because users get TRL now, without waiting for the
  refactor. The change is one class name, and the config stays the same.

## Alternatives

### A. A flat `TRLConfig` with a method enum

This is the shape in the previous draft: `TRLConfig(method=TRLMethod.GRPO, beta=0.04, ...)`,
with a `_METHOD_SCOPED_FIELDS` map that checks fields at construction. Rejected, because
`loss_type` and `beta` have conflicting types and meanings across methods. So the flat class
cannot be typed. Also, someone must maintain the runtime map by hand anyway. See
[Choosing the config shape](#choosing-the-config-shape).

### B. Top-level per-method configs

`BuiltinTrainer(config=GRPOTrainer(...))`, with no outer TRL config. This shape reads best at
the call site. But three classes would claim the `trl` label, in a registry keyed to one config
per framework. The 161 shared flags would then be frozen again in every public signature, or
moved into a base. That base then *is* the outer config. See
[Choosing the config shape](#choosing-the-config-shape).

### D. Shared model, dataset and training specs

This option groups the shared fields into nested `ModelSpec` / `DatasetSpec` objects. Rejected,
because the surface is shared only as a concept. TRL, LlamaFactory and TorchTune each name these
fields differently. TorchTune's Hydra overrides could not use the block at all. Also, it is a
one-way door: if a field moves into `config.model` later, that is a breaking change.

### One trainer class per framework (`TRLTrainer`, `TorchTuneTrainer`)

The trainer does nothing different per framework. It passes `command` and `to_args()` to the
backend. So a subclass per framework would put into the type hierarchy a distinction that exists
only in data. It also makes every framework a new exported class name and frozen public API. It
would also deprecate `BuiltinTrainer` for a `TorchTuneTrainer` successor, with no change in
behaviour. Phase 2 replaces `BuiltinTrainer` too. But it uses one `ConfigTrainer` for all
config-driven frameworks, not one class per framework. *What it gets right:* typed
per-framework fields and a flatter call. The accepted design keeps the typed fields, on the
config. The cost is one level of nesting.

### One runtime per base model, mirroring TorchTune

Rejected, because TRL has no equivalent of TorchTune's per-model runtimes. Those runtimes exist
to select per-model recipes and configs. TRL scripts are model-agnostic, and the model is a flag.
See [Why one runtime, not one per model family](#why-one-runtime-not-one-per-model-family).

### A dedicated TRL plugin in the control plane

This option adds a `pkg/runtime/framework/plugins/trl/` next to the torch plugin. Rejected. The
proof of concept shows that the torch plugin already injects everything torchrun needs. So a TRL
plugin would duplicate `EnforceMLPolicy` to produce identical output. It would also put the TRL
version in the controller's release cycle.

## References

**Kubeflow**

- [KEP-2170: Trainer V2 API](../2170-kubeflow-trainer-v2/README.md)
- [KEP-2401: LLM Trainer V2](../2401-llm-trainer-v2/README.md) — the TorchTune integration this
  generalizes. Its "Complement Torch Plugin" section originated the `EnforceMLPolicy` TorchTune
  branch
- [KEP-2779: TrainJob progress](../2779-trainjob-progress/README.md) — the runtime→status
  channel Open Question #1 would reuse
- Tracking issue [trainer#2839](https://github.com/kubeflow/trainer/issues/2839). GRPO via TRL
  is in progress in [#3508](https://github.com/kubeflow/trainer/issues/3508) /
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

- `torch.distributed.argparse_util` — the `env` action deriving `PET_{DEST}`. It shows why `PET_*`
  is torchrun's interface and nothing else's. `torch.distributed.run` — `auto` handling and the
  `-m` / `--module` invocation the runtime command uses.
- `trl/scripts/sft.py` — a plain script (`TrlParser` + `SFTTrainer`, `__main__` guard, no
  launcher logic). It has one sibling module per method. It parses with
  `fail_with_unknown_args=False`.
  `trl/cli/accelerate_launcher.py` — `launch_training_script` resolves
  `resources.files("trl.scripts")` and calls accelerate's `launch_command`, which is why the CLI
  is accelerate-bound.
- `trl/trainer/{sft,dpo,grpo}_config.py` — the per-method `loss_type` and `beta` declarations
  quoted in [Choosing the config shape](#choosing-the-config-shape). Each inherits HF
  `TrainingArguments` for the shared arguments.
- `accelerate.state.PartialState` — in-process env detection: `LOCAL_RANK != -1` → multi-GPU
  (nccl), `WORLD_SIZE > 1` on CPU → multi-CPU (gloo). This is what makes the script work under
  torchrun with no launcher flags.
- `accelerate.commands.launch` — the source behind the two *launcher* limitations recorded above.
  The line-level detail is in the
  [PoC write-up](https://github.com/YassinNouh21/trainer/blob/01b96f38608467d96acfc01c7500d49ebeca5d6f/examples/trl/README.md).
- [trl#4466](https://github.com/huggingface/trl/issues/4466) — PPO moved to experimental. It is
  absent from `trl/trainer/` and `trl/experimental/` at main.
  [torchtune#2883](https://github.com/meta-pytorch/torchtune/issues/2883) — development halted,
  15 July 2025.
