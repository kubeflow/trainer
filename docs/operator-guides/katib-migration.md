# Migrating from Katib to OptimizationJob

The Katib project is deprecated. Hyperparameter optimization in Kubeflow continues with the
`OptimizationJob` API in Kubeflow Trainer. This guide shows how to move a Katib `Experiment`
to an `OptimizationJob` and lists the Katib features that do not have an equivalent yet.

:::{warning}
OptimizationJob uses the alpha `trainer.kubeflow.org/v1alpha1` API and currently runs
**random search only**, with a single objective. Read the limitations in the
[OptimizationJob guide](../user-guides/optimizationjob.md) before migrating, and check
[Features without an equivalent](#features-without-an-equivalent) to see whether your
Experiments depend on something that is not available yet.
:::

## Before you begin

You need Kubeflow Trainer 2.4 or later with the `OptimizationJob` and `TrainJobStatus` feature
gates enabled. The [OptimizationJob guide](../user-guides/optimizationjob.md#before-you-begin) lists the full
prerequisites.

Katib and Kubeflow Trainer can run in the same cluster, so you can move Experiments one at a
time.

## What changes

| Katib | OptimizationJob |
| --- | --- |
| `Experiment` (`kubeflow.org/v1beta1`) | `OptimizationJob` (`trainer.kubeflow.org/v1alpha1`) |
| `Trial`, which wraps any Kubernetes resource (`Job`, `PyTorchJob`, and so on) | A `TrainJob` created from `spec.trainJobTemplate` |
| `Suggestion` resource and algorithm service | A search algorithm Deployment and Service owned by the OptimizationJob |
| `${trialParameters.<name>}` placeholders in the trial template | `KUBEFLOW_TRAINER_OPT_<parameter-name>` environment variables on the trainer |
| Metrics collector sidecar (StdOut, File, TensorFlowEvent, and others) | The trial pushes metrics to the TrainJob status server. Log scraping is not supported. |
| Katib DB Manager and its database | Trial parameters and metrics are stored on the TrainJob resources |
| `status.currentOptimalTrial` | `status.result` |

## Map the Experiment spec

| Katib `Experiment` field | `OptimizationJob` field | Notes |
| --- | --- | --- |
| `objective.objectiveMetricName` | `objectives[0].metric` | Exactly one objective. `objective.additionalMetricNames` has no equivalent: extra metrics you push appear in the TrainJob status but are not used by the search. |
| `objective.type` (`minimize`, `maximize`) | `objectives[0].direction` (`Minimize`, `Maximize`) | Values are capitalized. Defaults to `Minimize`. |
| `algorithm.algorithmName: random` | `searchAlgorithm.random` | |
| `algorithm.algorithmSettings` `random_state` | `searchAlgorithm.random.seed` | |
| `parameters[].parameterType: double` | `parameters[].searchSpace.uniform` or `logUniform` | `type` defaults to `Float`. |
| `parameters[].parameterType: int` | `parameters[].searchSpace.uniform` or `logUniform` with `type: Int` | Bounds must be integers, such as `"1"` and `"64"`. |
| `parameters[].parameterType: categorical` | `parameters[].searchSpace.categorical.choices` | |
| `parameters[].parameterType: discrete` | `parameters[].searchSpace.categorical.choices` | Choices are strings; convert them in your code. Choices must be unique, at most 64 characters each, and at most 100 in total. |
| `parameters[].feasibleSpace.distribution: logUniform` | `parameters[].searchSpace.logUniform` | `min` must be greater than 0. |
| `parameters[].feasibleSpace.min`, `max` | `searchSpace.<space>.min`, `max` | Quoted decimal strings, with `min < max`. |
| `maxTrialCount` | `numTrials` | 1 to 100. Defaults to 1. |
| `parallelTrialCount` | `parallelTrials` | 1 to `numTrials`. Defaults to 1. Katib defaults to 3. |
| `trialTemplate.trialSpec` | `trainJobTemplate.spec` | Always a TrainJob that references a training runtime. |
| `trialTemplate.trialParameters` | Not needed | Every parameter is injected as an environment variable. |
| `trialTemplate.primaryContainerName`, `successCondition`, `failureCondition` | Not needed | Trial status comes from the TrainJob. |
| `metricsCollectorSpec` | Not needed | Trials push metrics to the TrainJob status server. |

Parameter names may contain only letters, digits, and underscores, and cannot start with a
digit. Katib names such as `num-layers` must be renamed, for example to `num_layers`.

## Update the training code

Two changes are needed in the code that runs each trial.

**Read parameters from the environment.** Katib replaces `${trialParameters.<name>}` in the
trial template, usually in the container arguments. OptimizationJob sets one environment variable
per parameter on the trainer instead, named after the parameter itself, not a trial parameter
alias:

```python
import os

learning_rate = float(os.environ["KUBEFLOW_TRAINER_OPT_learning_rate"])
epochs = int(os.environ["KUBEFLOW_TRAINER_OPT_epochs"])
```

**Push the objective metric.** Katib's default StdOut collector parses lines such as
`validation_loss=0.12` from the logs. OptimizationJob does not read logs. The trial must report
the metric to the TrainJob status server before it exits:

```python
from kubeflow.trainer import update_trainjob_status

if not update_trainjob_status(progress_percent=100, metrics={"validation_loss": loss}):
    raise RuntimeError("Failed to report validation_loss")
```

`update_trainjob_status` sends at most one update every 5 seconds and returns `False` for calls
it drops. Each update replaces the whole `metrics` list, so the last update a trial sends must
include the objective metric. If your Katib code printed the metric after every epoch, make sure
the final value is sent and check the return value.

With `transformers >= 5.4.0`, metrics logged during `trainer.train()`, including evaluation
metrics, are pushed automatically. Metrics from a `trainer.evaluate()` call made after
`trainer.train()` returns are not sent, so report those with `update_trainjob_status`. See
[Report the objective metric](../user-guides/optimizationjob.md#report-the-objective-metric).

:::{note}
For each trial, Katib's default metric strategy keeps the best value the trial reported: the
minimum when minimizing, the maximum when maximizing. OptimizationJob uses the last value the
trial reported for the objective metric. If your trial reports per-epoch values, report the value
you want compared, such as the best validation loss, last.
:::

## Example: random search

The following Katib Experiment tunes the learning rate and epoch count of a small linear
regression workload. The trial prints `validation_loss` to stdout for the StdOut collector.

```yaml
apiVersion: kubeflow.org/v1beta1
kind: Experiment
metadata:
  name: linear-regression-search
spec:
  objective:
    type: minimize
    objectiveMetricName: validation_loss
  algorithm:
    algorithmName: random
  parallelTrialCount: 2
  maxTrialCount: 4
  maxFailedTrialCount: 1
  parameters:
    - name: learning_rate
      parameterType: double
      feasibleSpace:
        min: "0.01"
        max: "0.2"
        distribution: logUniform
    - name: epochs
      parameterType: int
      feasibleSpace:
        min: "10"
        max: "40"
  trialTemplate:
    primaryContainerName: training-container
    trialParameters:
      - name: learningRate
        reference: learning_rate
      - name: epochs
        reference: epochs
    trialSpec:
      apiVersion: batch/v1
      kind: Job
      spec:
        template:
          spec:
            restartPolicy: Never
            containers:
              - name: training-container
                image: python:3.11-slim-bookworm
                command:
                  - sh
                  - -c
                  - |-
                    python3 -c '
                    import sys
                    import time
                    learning_rate, epochs = float(sys.argv[1]), int(sys.argv[2])
                    train_x = [-1.0, -0.5, 0.0, 0.5, 1.0]
                    weight, bias = 0.0, 0.0
                    for _ in range(epochs):
                        errors = [weight * x + bias - (2 * x + 1) for x in train_x]
                        weight -= learning_rate * 2 * sum(e * x for e, x in zip(errors, train_x)) / 5
                        bias -= learning_rate * 2 * sum(errors) / 5
                    validation_x = [-0.75, -0.25, 0.25, 0.75]
                    loss = sum((weight * x + bias - (2 * x + 1)) ** 2 for x in validation_x) / 4
                    # The Katib metrics collector fails if the trial exits before it attaches.
                    time.sleep(10)
                    print(f"validation_loss={loss}")
                    ' "${trialParameters.learningRate}" "${trialParameters.epochs}"
```

The equivalent OptimizationJob reads the parameters from the environment and pushes the metric.
It runs each trial as a TrainJob on the `torch-distributed` runtime:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: OptimizationJob
metadata:
  name: linear-regression-search
spec:
  objectives:
    - metric: validation_loss
      direction: Minimize
  searchAlgorithm:
    random: {}
  parallelTrials: 2
  numTrials: 4
  parameters:
    - name: learning_rate
      searchSpace:
        logUniform:
          min: "0.01"
          max: "0.2"
    - name: epochs
      searchSpace:
        uniform:
          min: "10"
          max: "40"
          type: Int
  trainJobTemplate:
    spec:
      runtimeRef:
        apiGroup: trainer.kubeflow.org
        kind: ClusterTrainingRuntime
        name: torch-distributed
      trainer:
        numNodes: 1
        image: python:3.11-slim-bookworm
        command:
          - /bin/sh
          - -c
          - |
            pip install --quiet kubeflow
            python3 - << 'EOF'
            import os
            from kubeflow.trainer import update_trainjob_status

            learning_rate = float(os.environ["KUBEFLOW_TRAINER_OPT_learning_rate"])
            epochs = int(os.environ["KUBEFLOW_TRAINER_OPT_epochs"])
            train_x = [-1.0, -0.5, 0.0, 0.5, 1.0]
            weight, bias = 0.0, 0.0
            for _ in range(epochs):
                errors = [weight * x + bias - (2 * x + 1) for x in train_x]
                weight -= learning_rate * 2 * sum(e * x for e, x in zip(errors, train_x)) / 5
                bias -= learning_rate * 2 * sum(errors) / 5
            validation_x = [-0.75, -0.25, 0.25, 0.75]
            loss = sum((weight * x + bias - (2 * x + 1)) ** 2 for x in validation_x) / 4
            print(f"validation_loss={loss}")
            if not update_trainjob_status(progress_percent=100, metrics={"validation_loss": loss}):
                raise RuntimeError("Failed to report validation_loss")
            EOF
```

Once the trials finish, read the result from the OptimizationJob instead of
`status.currentOptimalTrial`:

```bash
kubectl get optimizationjob linear-regression-search -o jsonpath='{.status.result}'
```

`status.result` holds the best trial's TrainJob name and parameters. Its metric values are on
that TrainJob, under `status.trainerStatus.metrics`. Katib's `Succeeded` condition corresponds to
`Complete` on an OptimizationJob.

## Behavior differences

- **Trial failures.** Katib keeps going until `maxFailedTrialCount` trials fail. The current
  OptimizationJob controller marks the job `Failed` as soon as one trial fails, or when a
  successful trial is missing its objective metric. Trials that are still running are not
  cancelled and keep using resources until they finish.
- **Objective goal.** Katib stops early once `objective.goal` is reached. OptimizationJob always
  runs `numTrials` trials.
- **Trial count.** Katib's `maxTrialCount` counts finished trials. `numTrials` counts the
  TrainJobs the controller creates.
- **Spec changes.** The OptimizationJob spec is immutable and there is no `resumePolicy`. To
  change the search or continue it, create a new OptimizationJob.
- **Trial cleanup.** Trial TrainJobs are kept until the OptimizationJob is deleted, similar to
  `trialTemplate.retain: true`. The search algorithm Deployment and Service are removed when
  the job completes or fails.

## Features without an equivalent

The following Katib features are not available in OptimizationJob yet. Keep using Katib for
Experiments that depend on them. If you rely on one of these features, start a thread in the
[`#kubeflow-trainer` Slack channel](https://www.kubeflow.org/docs/about/community/#kubeflow-slack)
so it can be prioritized.

| Katib feature | Status |
| --- | --- |
| Algorithms other than `random`: `grid`, `tpe`, `multivariate-tpe`, `cmaes`, `bayesianoptimization`, `hyperband`, `sobol`, `pbt` | `searchAlgorithm.grid` is defined in the API but fails with `UnsupportedSearchAlgorithm`. Hyperband and PBT are tracked in [#3802](https://github.com/kubeflow/trainer/issues/3802). |
| Early stopping (`earlyStopping`, median stop) | Tracked in [#3802](https://github.com/kubeflow/trainer/issues/3802). |
| Neural architecture search (`nasConfig`, ENAS, DARTS) | Not supported. The [OptimizationJob KEP](https://github.com/kubeflow/trainer/tree/master/proposals/2605-optimization-job-crd) drops NAS. |
| `feasibleSpace.step` | No equivalent. List the values in a `categorical` search space instead. |
| `normal` and `logNormal` distributions | Tracked in [#3795](https://github.com/kubeflow/trainer/issues/3795). |
| `maxFailedTrialCount` | Not supported. Any trial failure fails the job. Tracked in [#3743](https://github.com/kubeflow/trainer/issues/3743). |
| Trial templates from a ConfigMap (`trialTemplate.configMap`) | Not supported. The template is inline. |
| Trials that are not TrainJobs, such as a plain `Job` | Not supported. The KEP limits trials to TrainJobs. |
| Katib UI | Not available. Use `kubectl` to inspect the OptimizationJob, and `kubectl` or the Kubeflow SDK `TrainerClient` to inspect its TrainJobs. |

## Python SDK

The Katib SDK `KatibClient.tune()` API and the Kubeflow SDK `OptimizerClient` create Katib
Experiments. To use OptimizationJob today, create the resource from YAML, as shown above.
Python SDK support for OptimizationJob is tracked in
[#3794](https://github.com/kubeflow/trainer/issues/3794). The design in
[#3881](https://github.com/kubeflow/trainer/pull/3881) proposes an `optimize()` API on
`TrainerClient`.
