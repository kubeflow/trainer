# Hyperparameter Optimization with OptimizationJob

Use `OptimizationJob` to tune hyperparameters across a set of TrainJob trials. Specify the
search space, objective metric, total trial budget, concurrency, and a TrainJob template in
one Kubernetes resource. The controller requests parameter suggestions, launches trials, and
records the best completed trial and its parameters in `.status.result`.

:::{warning}
**OptimizationJob is experimental, under active development, and not production-ready.**
It uses the alpha `trainer.kubeflow.org/v1alpha1` API.

Please note the following current limitations:
- **Search algorithms**: The controller currently supports **random search only** with a
  **single objective** (`Minimize` or `Maximize`). Other algorithms defined in the API
  (such as `grid`) are not yet executed.
- **Service resiliency**: The search algorithm suggestion service maintains state in memory.
  If its Pod restarts mid-search (e.g., due to a node eviction), it rejects previously completed
  trials and the search stalls without being marked `Failed`.
- **Strict trial failure handling**: The current controller does not tolerate trial failures;
  if any trial TrainJob fails, or if a completed trial is missing its objective metric, the
  entire OptimizationJob transitions to `Failed`.
- **Immutable specification**: The search space, objectives, and trial budgets cannot be
  modified after creation.
- **Metric reporting requirement**: Trials must explicitly push their objective metrics to
  the TrainJob status server before the training process exits. Metrics logged solely to
  stdout are not scraped.
:::

## Before you begin

Follow the [Getting Started guide](../getting-started/index.md) to install Trainer and its
runtimes. For the example below, you need:

- Kubeflow Trainer 2.4 or later to enable the OptimizationJob CRD and controller.
- The default `torch-distributed` ClusterTrainingRuntime.
- The `OptimizationJob` and `TrainJobStatus` alpha feature gates enabled on the Trainer controller:

  ```text
  --feature-gates=OptimizationJob=true,TrainJobStatus=true
  ```

  With Helm, set `manager.config.featureGates.OptimizationJob=true` and
  `manager.config.featureGates.TrainJobStatus=true`. See
  [TrainJob progress and training metrics](trainjob-progress.md) for status server prerequisites
  and configuration.
- Capacity for two concurrent trials requesting one CPU and 128 MiB of memory each, plus the
  search algorithm Pod. The cluster must be able to pull the example's Python image and the
  search algorithm image `ghcr.io/kubeflow/katib/suggestion-optuna:v0.19.0`.

Check the installed resources:

```bash
kubectl get crd optimizationjobs.trainer.kubeflow.org
kubectl get clustertrainingruntime torch-distributed
```

All commands below use your current `kubectl` namespace. The trial TrainJobs and search
algorithm resources are created in the same namespace as the OptimizationJob.

## How a search runs

1. The controller creates a Deployment and Service named `<optimizationjob-name>-search-algorithm`
   in the OptimizationJob namespace. The controller calls it over gRPC on port 6789, so allow this
   traffic if the namespace uses NetworkPolicies or a service mesh.
2. It supplies trial history to a Katib-compatible gRPC suggestion service and requests the
   next parameter assignments.
3. It creates TrainJobs from `spec.trainJobTemplate`, injecting each selected parameter as
   `KUBEFLOW_TRAINER_OPT_<parameter-name>` into the trainer's environment. Parameter names may
   contain only letters, digits, and underscores. Do not set your own `KUBEFLOW_TRAINER_OPT_*`
   env vars in the template; the controller treats them as search parameters.
4. Trials report objective metrics through the TrainJob status server. The controller compares
   successful completed trials and updates `.status.result` as the search progresses.

Trial parameters and reported metrics live in Kubernetes resources, so this workflow does not
require a separate HPO database or log-scraping metrics sidecar. The suggestion service lives
for the duration of the search and is cleaned up when the OptimizationJob succeeds or fails.
Kubernetes remains the persistent store; this is not an append-only archive of experiments.

:::{note}
The current Katib Optuna suggestion service keeps its state in memory. If its Pod restarts
mid-search (e.g., due to a node eviction), it rejects previously completed trials and the
OptimizationJob stops creating new trials without being marked `Failed`. Trial parameters and
metrics remain in the TrainJob resources.
:::

Trials reuse Trainer's [runtime configuration](../operator-guides/runtime.md). JobSet manages
the underlying training workloads; gang scheduling requires a compatible scheduler integration,
as described in the [job scheduling guide](../operator-guides/job-scheduling/overview.md).
`parallelTrials` controls trial concurrency, while `trainer.numNodes` controls nodes per trial.

## Run a CPU example

The following example fits a linear regression model to fixed synthetic data, then reports
mean squared error on separate validation inputs as `validation_loss`. It searches learning
rate and epoch count across four trials, with at most two running concurrently. It uses one
CPU per trial, requires no dataset downloads, and overrides the runtime's image and command
with a Python standard-library workload. It does not launch distributed worker processes.

From a local checkout of the repository, apply the manifest:

```bash
kubectl apply -f examples/optimizationjob/random-search.yaml
```

The complete manifest is shown below so it can also be saved and applied independently:

```{literalinclude} ../../examples/optimizationjob/random-search.yaml
:language: yaml
:start-at: apiVersion:
```

## Configure the search

| Field | Behavior |
| --- | --- |
| `objectives` | Exactly one metric name and direction: `Minimize` or `Maximize`. Direction defaults to `Minimize`. |
| `searchAlgorithm.random` | Random search. An optional `seed` is forwarded to the suggestion service; it does not seed your training code. |
| `parameters` | Between 1 and 100 named parameters, each with exactly one search space. |
| `numTrials` | Total trial budget, from 1 to 100. Defaults to 1. |
| `parallelTrials` | Concurrent trial limit, from 1 to `numTrials`. Defaults to 1. |
| `trainJobTemplate` | TrainJob metadata and spec, including the runtime and training command. Do not set template metadata `name` or `namespace`. |

Supported search spaces are:

- `uniform`: numeric `min` and `max` with `min < max`.
- `logUniform`: numeric `min` and `max` with `0 < min < max`.
- `categorical`: an explicit list of string `choices`.

Write numeric bounds as quoted decimal strings, such as `"0.01"` or `"1e-3"`. Numeric spaces
accept `type: Float` (the default) or `type: Int`; with `type: Int`, use integer bounds such as
`"1"` and `"64"`. Read selected values from the environment and convert them to the types expected
by your training code. Random search can suggest the same parameter combination more than once;
each suggestion still runs as a separate trial.

The OptimizationJob spec is immutable. To change the objective, search space, trial budget,
or workload template, create a new OptimizationJob.

## Report the objective metric

Each successful trial must report its objective metric to
`TrainJob.status.trainerStatus.metrics` **before its training process exits**. Logging a metric
to stdout alone is insufficient; the controller evaluates metrics stored in the TrainJob status.
The metric name must match `spec.objectives[0].metric`, and its value must be numeric.

Because each status update replaces the entire `metrics` list, **the objective metric must be
present in the trial's final status report**.

We recommend reporting metrics using one of the following approaches:

### Option 1: Kubeflow SDK `update_trainjob_status`

Use `update_trainjob_status` from `kubeflow.trainer` to report completion and metrics directly from
your Python script:

```python
from kubeflow.trainer import update_trainjob_status

# Report final objective metric before exiting
update_trainjob_status(
    progress_percent=100,
    metrics={"validation_loss": loss},
)
```

The SDK automatically handles authentication, TLS verification, retries, and rate limiting with
the injected status server.

### Option 2: HuggingFace Transformers (Zero Code Change)

When using `transformers >= 4.57.0`, the `KubeflowTrainerCallback` is registered automatically.
Evaluation metrics (such as `eval_loss` or custom metrics from `compute_metrics`) are pushed directly
to `TrainJob.status` without manual instrumentation.

For detailed configuration and prerequisites, see the
[TrainJob progress and training metrics](trainjob-progress.md) guide. For multi-node or distributed
trials, designate rank 0 to report the objective metric.

## Monitor trials and inspect the best result

```bash
kubectl get optimizationjob linear-regression-search -o yaml
kubectl get trainjobs -l trainer.kubeflow.org/optimization-job=linear-regression-search
kubectl wait --for=condition=Complete optimizationjob/linear-regression-search --timeout=10m
```

The wait command watches for successful completion only. If it times out, inspect the
OptimizationJob conditions and trial status before retrying:

```bash
kubectl describe optimizationjob linear-regression-search
```

Retrieve the selected trial and its parameter assignments:

```bash
kubectl get optimizationjob linear-regression-search -o jsonpath='{.status.result}'
BEST_TRIAL=$(kubectl get optimizationjob linear-regression-search -o jsonpath='{.status.result.trainJobName}')
kubectl get trainjob "$BEST_TRIAL" -o jsonpath='{.status.trainerStatus.metrics}'
kubectl logs -l "jobset.sigs.k8s.io/jobset-name=$BEST_TRIAL" --all-containers=true --tail=-1
```

Run the commands using `BEST_TRIAL` after at least one trial has completed successfully and a
result exists. `.status.result` contains `trainJobName` and `parameters`; read the selected
TrainJob's status for its metric values. The best result can improve while trials are running.
Equal objective values are resolved by the older creation timestamp, then by the
lexicographically smaller TrainJob name.

## Completion, failures, and cleanup

The job becomes `Complete` when its trial budget has been reached and no trials remain active.
The current controller marks the job `Failed` if a trial fails, or if a successful completed
trial has a missing or invalid objective metric. Inspect the condition's `reason` and `message`
for details, including `TrialFailed`, `ObjectiveMetricMissing`, `ObjectiveMetricInvalid`, or
`UnsupportedSearchAlgorithm` (for example, when `searchAlgorithm.grid` is set).

On completion or failure, the controller deletes the search algorithm Deployment and Service.
Trial TrainJobs are retained for inspection; active trials are not cancelled by this cleanup.
Do not delete trials while the search is running: deleted trials no longer count toward
`numTrials`, so the controller launches replacements, and `.status.result` is recomputed from the
remaining trials.

When finished, delete the OptimizationJob and let Kubernetes garbage-collect its owned trials:

```bash
kubectl delete optimizationjob linear-regression-search
```
