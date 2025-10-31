# KEP-2779: Track TrainJob progress and expose training metrics

Authors:

- Abhijeet Dhumal (Red Hat)
- Rob Bell (Red Hat)

## Summary

This document outlines a proposal to add real-time progress and training metrics updates to the TrainJob status api.

## Motivation

From a user perspective:

* As training jobs can often be long-running, it is useful for data scientists to have visibility on how training jobs are progressing, whether they are converging and when a job is likely to complete so they can proactively terminate or fix jobs that are not progressing as desired.
* In multi-tenant environments where GPUs may be shared across multiple users, data scientists want visibility on when existing jobs are likely to finish to help them decide when/whether to submit their jobs and how many GPUs to request for them.
* Currently, users need to inspect logs to determine progress which is cumbersome and makes it difficult to see an overview of progress across multiple train jobs. It is desirable to have an easier way to see the progress of their jobs.

From a programmatic perspective:

* API access to real-time metrics would allow other tools to integrate with trainer jobs, e.g. a dashboard (see [\#2648](https://github.com/kubeflow/trainer/issues/2648)) or integrating with Katib for hyperparameter optimization.
* Some hyperparameter optimization algorithms (e.g. [hyperband](https://www.kubeflow.org/docs/components/katib/user-guides/hp-tuning/configure-algorithm/#hyperband)) require real-time metrics to implement early-stopping as a strategy for optimizing resource allocation.

### Goals

1. **Expose real-time progress information and training metrics through the TrainJobs CR.** For example: percentage complete, estimated time remaining, current step/epoch, total steps/epochs, eval metrics.
2. **Provide built-in progress tracking support for selected ML frameworks (e.g. transformers, pytorch-lightning) in the kubeflow sdk.** Data scientists should be able to use the kubeflow sdk to create training jobs using these frameworks, and have progress tracking automatically instrumented.
3. **Provide a standard "protocol" for training runtimes to expose progress and training metrics.** It should be possible for custom trainer training jobs to use this contract to add progress tracking. It should be easy to enhance the kubeflow sdk with additional built-in frameworks that automatically instrument progress tracking.

### Non-Goals

1. **Replace tools like MLFlow or Tensorboard.** These tools can be used in addition to the proposed changes. They provide richer information about the training progress, but require additional infrastructure set up and do not easily expose their information in a format that is easy to consume via the Kubernetes api.
2. **Provide progress and metrics for dataset and model initialization.** The implementation could be easily extended for these phases of the TrainJob api, but we propose delaying this to a future iteration to limit scope.
3. **Automatically instrument custom trainer training jobs to have progress tracking.**
4. **Add progress tracking for the Kubeflow Trainer v1 api.** The v1 api is legacy.  This feature should only be added to the v2 api.
5. **Integrate with Katib’s hyperparameter optimiser.** Whilst the exposed training metrics should provide a route for this integration, the integration is out of scope of this proposal.

## Proposal

We propose the following high level design for exposing training progress information through the TrainJob custom resource:

1. Update the TrainJob `status` to include a new field `trainerStatus` to contain information about training progress and training metrics.
2. Define a "push"-based protocol for communicating the training status from the runtime to the trainer controller manager: a "primary" pod of the training job writes trainer status to the pod log; the controller manager streams these logs and updates the trainer status on the `TrainJob` CR.

The runtime pods for TrainJobs will need instrumenting to write trainer status messages to stdout and we propose making this as simple as possible for users by:

3. Adding new built-in trainers to the kubeflow-sdk that automatically register training callbacks in the runtime to output trainer status messages to the logs in the correct format. Initially we propose only adding a builtin trainer for the [*Transformers* *Trainer* API](https://huggingface.co/docs/transformers/en/main_classes/trainer).

## User Stories

### Story 1: Platform Engineer integrating with third parties

As a platform engineer, I want to be able to access information about training jobs so I can integrate Kubeflow Trainer with another application (e.g. a dashboard, a hyperparameter optimiser).

### Story 2: Data Scientist / ML Engineer monitoring training jobs

As a data scientist or ML Engineer, I want to see real-time information about my training jobs so I can make decisions on whether those jobs are likely to succeed or whether intervention is required.

I can use standard tools, like the Kubeflow Trainer Python SDK or `kubectl get trainjob` to access progress and performance metrics about my train jobs.

## Design Details

### TrainJob CRD changes

We propose adding a `trainerStatus` field to the TrainJob status API according to this schema:

```go
type TrainJobStatus struct {
    // ... existing fields

    // TrainerStatus provides a summary of the training part of the
    // TrainJob.
    // Empty if the status is unknown, e.g. the job has just started
    // or the job is not instrumented to report its status.
    TrainerStatus *TrainJobTrainerStatus `json:"trainerStatus,omitempty"`
}


type TrainJobTrainerStatus struct {

    // An estimate of how complete the TrainJob is as a percentage.
    // The value will be between 0 and 100, or empty if unknown.
    //
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Maximum=100
    ProgressPercentage *int32 `json:"progressPercentage,omitempty"`

    // The estimated remaining training time in seconds before the train 
    // job is completed.
    // The value will be empty if it is unknown.
    //
    // +kubebuilder:validation:Minimum=0
    EstimatedRemainingSeconds *int64 `json:"estimatedRemainingSeconds,omitempty"`

    // An approximate, human-readable version of the estimated remaining
    // training time before the train job is completed.
    // The value will be empty if it is unknown.
    // This message is intended for human audiences and its format should
    // not be relied on to be stable.
    // Consider using EstimatedRemainingSeconds instead.
    EstimatedRemainingTimeSummary *string `json:"estimatedRemainingTimeSummary,omitempty"`

    // The number of steps that have been completed so far.
    // The value will be empty if it is unknown or not applicable 
    // to the training algorithm.
    CurrentStep *int32 `json:"currentStep,omitempty"`

    // The total number of steps that have been requested.
    // The value will be empty if it is unknown or not applicable 
    // to the training algorithm.
    TotalSteps *int32 `json:"totalSteps,omitempty"`

    // The number of epochs that have been completed so far.
    // The value will be empty if it is unknown or not applicable 
    // to the training algorithm.
    CurrentEpoch *int32 `json:"currentEpoch,omitempty"`

    // The total number of epochs that have been requested.
    // The value will be empty if it is unknown or not applicable 
    // to the training algorithm.
    TotalEpochs *int32 `json:"totalEpochs,omitempty"`

    // The current metrics evaluated on the training data.
    // The metrics are key-values pairs, where the key is a user-defined name
    // for the metric and the value is the corresponding numeric value serialized
    // as a string.
    //
    // +mapType=atomic
    TrainMetrics map[string]string `json:"trainMetrics,omitempty"`

    // The current metrics evaluated on evaluation data.
    // The metrics are key-values pairs, where the key is a user-defined name
    // for the metric and the value is the corresponding numeric value serialized
    // as a string.
    //
    // +mapType=atomic
    EvalMetrics map[string]string `json:"evalMetrics,omitempty"`

    // The timestamp when these metrics were measured.
    LastUpdatedTime metav1.Time `json:"lastUpdatedTime"`
}
```

The trainerStatus field is optional as it can be unavailable, e.g. because the job is still initializing and status messages have not yet been emitted, or if the runtime has not been instrumented to expose the trainer status.

All fields (apart from lastUpdatedTime) are optional meaning that a runtime need only provide information that it has available or is relevant for that training algorithm (e.g., epochs are not relevant for XGBoost models).

```yaml
# Sample TrainJob example with TrainerStatus status implemented

apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
...
status:
  trainerStatus:
    # Overall progress
    progressPercentage: 45                           # 45% complete
    estimatedRemainingSeconds: 795649                # Precise duration
    estimatedRemainingTimeSummary: "9 days 5 hours"  # Human-readable

    # Training iterations
    currentStep: 4500                                # Completed 4500 steps
    totalSteps: 10000                                # Out of 10000 total
    currentEpoch: 2                                  # On epoch 2
    totalEpochs: 5                                   # Of 5 epochs
  
    # Training metrics (serialized as strings)
    trainMetrics:
      loss: "0.2347"                                 # Current training loss
      learning_rate: "0.0001"                        # Current LR
      grad_norm: "1.234"                             # Gradient norm
    
    # Evaluation metrics (from validation set)
    evalMetrics:
      eval_loss: "0.2451"                            # Validation loss
      eval_accuracy: "0.8912"                        # Validation accuracy
      eval_perplexity: "1.277"                       # Model perplexity
    
    # Timestamp of last progress update
    lastUpdatedTime: "2025-01-23T10:30:45Z"
```

We also propose adding the `Progress %` and `ETA` (estimated time of arrival) to the printer columns for the TrainJob custom resource:

```
$ kubectl get trainjob
NAME              STATE      PROGRESS %  ETA   AGE
an-example        Running    3           45m   13m
another-example   Complete   100               50m
```

The ETA column will be populated from the `status.trainerStatus.estimatedRemainingTimeSummary` field.

### Runtime: emitting trainer status messages

The "primary" training runtime pod will write trainer status messages to stdout in the following format:

* Messages will be written on a single new line.
* Messages may be interspersed between other (i.e. non status, application) log messages, but must be on their own line.
* The format of the messages will be a fixed tag `[trainer.kubeflow.org/v1alpha1/trainjob/trainerStatus]`, followed by whitespace, followed by a json payload containing the trainer status. E.g.

```
[trainer.kubeflow.org/v1alpha1/trainjob/trainerStatus] {"progressPercent":45,...}
```

* The tag will be used by the trainer controller manager to filter out application logs and select only the trainer status messages.
* The tag contains the TrainJob group/version/kind so it can be versioned with the TrainJob api. The trainer controller manager could, in principle, support reading multiple different versions of the trainer status messages.
* The schema of the json payload is as defined using the below Python dataclass. Note the schema is similar but not identical to the `TrainJobTrainerStatus` type on the CR, with minor differences for better Python compatibility, and the removal of `lastUpdatedTime` and `estimatedRemainingTimeSummary` which will be set by the controller manager.

```py
@dataclass.dataclass
class TrainJobTrainerStatus:
    progressPercentage: int | None
    estimatedRemainingSeconds: int | None
    currentStep: int | None
    totalSteps: int | None
    currentEpoch: int | None
    totalEpochs: int | None
    trainMetrics: dict[str, float] | None
    evalMetrics: dict[str, float] | None
```

When using distributed training, the runtime framework is responsible for collecting all the information in the trainer status messages onto the "primary" pod.

### Trainer Controller Manager: reading trainer status messages

The trainer controller manager will subscribe to the pod logs stream of the "primary" pod using the `pods/logs` api with "follow=true" and "timestamps=true" (for recovery, see below). The controller will filter out messages in memory using the fixed tag to select only the trainer status updates and parse the json payload. When trainer status messages are observed, the controller manager will update the status.trainerStatus field of the TrainJob custom resource.

Some additional details about the pod log streaming:

* The log streaming will only be created when the TrainJob is "active", defined as by when the "primary" pod has status "Running".
* The log streaming will be automatically terminated only when the "primary" pod is terminated. If the "primary" pod is terminated and rescheduled, the controller reconciliation loop will automatically subscribe to the logs of the new "primary" pod.
* The streaming will execute in a separate goroutine, with a separate goroutine per active TrainJob.
* The log stream will be filtered in memory (e.g. using bufio.Scanner). The default Scanner buffer size (64kB) should be large enough to avoid bufio.ErrTooLong errors when scanning the log output, whilst causing modest memory requirements even for large numbers of simultaneously active TrainJobs (e.g. 1000 active TrainJobs would require \~63MB of memory).
* The status.trainerStatus.lastUpdatedTime field will be set from the timestamp of the log message. This allows the controller manager to resume from the correct location in the log stream (using PodLogOptions.SinceTime) if it is restarted whilst a TrainJob is executing.
* If the log stream processing is interrupted (e.g. the GetLogs connection times out, or the log contains a line larger than the Scanner buffer size), the controller manager should resume the log stream from the timestamp of the last successfully read log line using (using PodLogOptions.SinceTime).
* Updating the status.trainerStatus field should be throttled to avoid unnecessary burden on the k8s API server (e.g. if the runtime emits many trainer status messages in quick succession).

The controller manager will populate `estimatedRemainingTimeSummary` by converting `estimatedRemainingSeconds` into a human-readable summary. The conversion can be lossy to improve readability. For example `estimatedRemainingSeconds=3610` (one hour and 10 seconds) may be converted to `estimatedRemainingSeconds=1 hour`.

By default, the controller manager will select the first pod of the JobSet as the "primary" pod. This may be overridden by adding an annotation `trainer.kubeflow.org/trainer-status-primary-pod: true` to the  primary pod (e.g. in the TrainingRuntimeSpec template). If the "primary" pod has multiple containers, the controller manager will by default stream logs of the first container. This may be overridden by adding the annotation `trainer.kubeflow.org/trainer-status-container: <container-name>` which will cause the controller manager to stream the logs of `<container-name>`.

For TrainJobs that contain multiple JobSets, by default all JobSets will be monitored using the above procedure. It’s assumed that only one of the JobSets will actually be instrumented with trainer status messages, in which case the correct status will be shown. If, however, multiple JobSets have been instrumented, the user can prevent a JobSet from being monitored adding an annotation `trainer.kubeflow.org/enable-trainer-status-monitoring: false` to the JobSet. Adding the same annotation to the overall TrainJob will disable all trainer status monitoring for that job.

The scalability of the maximum number of concurrently active train jobs will need testing.

### RBAC changes

The controller manager will need granting clusterwide permission to read pod logs.

```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubeflow-trainer-controller-manager
rules:
# ... existing rules
- apiGroups:
  - ""
  resources:
  - pods/log
  verbs:
  - get
```

### SDK Changes: instrumenting runtime

We’re proposing adding a new TransformersTrainer to the kubeflow-sdk. This will be a custom trainer with the same api as the CustomTrainer, but with the additional behaviour that it will automatically register a custom callback to output trainer status messages in the correct format.

Similar additional trainers could also be added in the future for other ML frameworks (e.g. PytorchLightningTrainer, XGBoostTrainer).

Example usage of the TransformersTrainer is:

```py
def train_fn():
    from transformers import Trainer
    
    trainer = Trainer(...)  # define the model and data...
    trainer.train()


from kubeflow.trainer import TrainerClient, TransformersBuiltinTrainer

client = TrainerClient()
client.train(
    runtime=client.get_runtime("transformers-distributed"),
    trainer=TransformersTrainer(
        func=train_fn,
        ...
    )
)
```

The sdk will generate the TrainJob resource using the same approach currently used by CustomTrainer (the python code is written inline in the pod `command`). A different template script will be used, however, which will inject a KubeflowTrainerStatusCallback which will emit status messages:

```py
# kubeflow/constants/constants.py
TRANSFORMERS_BUILTIN_TRAINER_FUNC_SCRIPT = textwrap.dedent(
    """
        read -r -d '' SCRIPT << EOM\n
        from transformers import TrainerCallback, trainer
        
        class KubeflowTrainerStatusCallback(transformers.TrainerCallback):
            ...  # will write training status messages to stdout

        # add the status callback to the default trainer callbacks.
        # this could also be achieved by monkeypatching the Trainer.__init__
        trainer.DEFAULT_CALLBACKS.append(KubeflowTrainerStatusCallback())
        
        {func_code}
        EOM
        printf "%s" \"$SCRIPT\" > \"{func_file}\"
        python \"{func_file}\"
    """
)

# kubeflow/utils/utils.py
def get_trainer_crd_from_transformers_builtin_trainer(
    runtime: types.Runtime,
    trainer: types.TransformerBuiltinTrainer,
) -> models.TrainerV1alpha1Trainer:
    trainer_crd = models.TrainerV1alpha1Trainer()
    ...  # as in the existing get_trainer_crd_from_custom_trainer function

    trainer_crd.command =  # as in the get_trainer_crd_from_custom_trainer
         # function, but using the TRANSFORMERS_BUILTIN_TRAINER_FUNC_SCRIPT
         # as the template

    ...  # as in the existing get_trainer_crd_from_custom_trainer function

    return trainer_crd
```

We are *not* proposing automatically instrumenting progress to the existing CustomTrainer: although this trainer can support any ML framework, it is harder to make it automatically instrument status updates for all (or at least a wide range of) frameworks. It may also be confusing for users and a bad onboarding experience if some frameworks automatically display progress, but others do not.

In contrast, defining separate custom trainers for each ML framework helps make the sdk more discoverable for users: they can easily see which ML frameworks are integrated with Kubeflow Trainer.

We can add documentation for how users can manually instrument trainer status updates for CustomTrainers, but we could consider encouraging users to think of the existing CustomTrainer as a lower-level API. Users can still use this lower-level API, but it will not automatically provide trainer status updates.

Additional changes to the sdk:

* Add a new trainerStatus field to the `TrainJob` response object.

```py
@dataclass
class TrainerStatus:
    progressPercentage: Optional[int]
    estimatedRemainingDurationSeconds: Optional[int]
    estimatedRemainingTimeSummary: Optional[str]
    currentStep: Optional[int]
    totalSteps: Optional[int]
    currentEpoch: Optional[int]
    totalEpochs: Optional[int]
    trainMetrics: Optional[dict[str, float]]
    evalMetrics: Optional[dict[str, float]]
    lastUpdatedTime: datetime


@dataclass
class TrainJob:
    # ... existing fields
    trainerStatus: Optional[TrainerStatus] = None
```

* Adding a new "transformers-distributed" ClusterRuntime which will be included in the default set of cluster runtimes included in the manifests.
* Publish new docker images for the "transformers-distributed"  runtime "ghcr.io/kubeflow/trainer/transformers-runtime". The docker image will include the transformers, accelerate and torch python packages.

## Considered alternatives

This section describes other approaches that were evaluated and the rationale for not selecting them.

### Alternatives to using pod logs to communicate status messages

We propose instrumenting the "primary" training pod so that it prints training status messages to stdout. The controller manager watches the pod logs and updates the TrainJob status when train status messages are observed. We outline below some alternative approaches.

#### Runtime pushes metrics to controller manager via web request

The Kubeflow Trainer controller manager exposes a web API for collecting trainer status; the trainer "primary" pod makes web requests on demand pushing status messages into the controller.  
Pros:

- More scalable: less network traffic as only status messages need transferring, instead of the entire "primary" pod logs.

Cons:

- Operational complexity: introduces an additional service (and possibly deployment) into the KFT control plane.
- Non-trivial amount of work to secure the service. In particular, role-based access control is required to secure the new endpoint and ensure that only a TrainJob is able to update its own status.

#### Serving trainer status messages via a webserver

Instrument the "primary" trainer pod to serve metrics via a small http server; the pod controller manager periodically pulls the metrics by making a web request to the "primary" pod.  
Pros:

- More secure. The controller manager does not need RBAC to read log messages from any pod in any namespace.
- More scalable: removes the risk that a pod outputs lots of log messages which overwhelms the api server; as it’s pull based, it can be best-effort based and the controller manager can adapt the poll frequency.

Cons:

- Non-trivial amount of work to secure the HTTP endpoint, e.g. with TLS and/or auth.
- Harder for cluster operators/platform maintainers to diagnose network misconfigurations, e.g. network policies blocking scraping.
- Pull based: status updates may be delayed.

#### Exposing metrics via prometheus

The runtime is instrumented with a Prometheus client which tracks and exposes the metrics; the Prometheus server is automatically configured to scrape the primary pod; the controller manager reads the metrics from Prometheus.  
Pros:

- Uses an existing standard and framework.
- Provides support for tracking the history of progress/metrics.

Cons:

- Introduces an external dependency to the deployment.
- Prometheus is not a typical part of the data science ecosystem. Data scientists are used to other tools that achieve the same things but with more familiar APIs (e.g. MLFlow).
- Harder for cluster operators/platform maintainers to diagnose network misconfigurations, e.g. network policies blocking scraping.
- Pull based: status updates may be delayed.

#### Controller manager reads status from a file in the "primary" pod

The "primary" pod writes training status to a file in its pod; the controller manager execs into the pod to read the status from a file.  
Pros:

- More scalable: less data being streamed to the controller manager; as it’s pull based, it can be best-effort based and the controller manager can adapt the poll frequency.

Cons:

- Significantly less secure: the controller manager must have `pods/exec` RBAC for all pods in the entire cluster.

## Alternatives to injecting code via pod command

We propose injecting code for instrumenting the progress callbacks via the pod command. We also considered this alternative.

### Adding progress callbacks as a library or in the Docker images

Rather than using the SDK to inject code, we could distribute the code (e.g. via a PyPI package) and/or embed it directly in the prebuilt trainer runtime Docker images .

Pros:

- Allows for larger amounts of code to be injected.
- More conventional, so less surprising for users/contributors trying to understand how the injection works.

Cons:

- Harder for users to create custom Docker images. Users need to be made aware that a library needs installing in their Docker image to enable support for progress tracking. We could get around this by automatically installing the library at runtime (similar to how the kubeflow-pipelines installs the kfp package at runtime), but this adds extra complication to support air-gapped clusters which do not have access to PyPI.
- May be harder to maintain version compatibility: the package would need to be compatible with a wide range of kubeflow-sdk versions and KFT control plane versions.
- May be harder for users to receive bug fixes to this runtime code. If the code is distributed in the trainer runtime docker images, platform engineers managing TrainingRuntimes may not be able to upgrade the base images frequently to pick up new package versions. The proposed approach of embedding the code in the kubeflow-sdk allows data scientists to pick up bug fixes by upgrading their sdk version.
