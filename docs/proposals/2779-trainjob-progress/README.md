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
2. **Have zero dependencies.** The user should not need to install any additional components into their cluster for the feature to work. It should work "out-of-the-box".
3. **Optional.** Users can choose to opt in to providing progress tracking, but are not required to use this feature.
4. **Introduce callbacks to third party libraries (e.g. transformers) so users can easily instrumentation their TrainJobs.** Data scientists should be able to use these callbacks to add progress tracking to their jobs. For Transformers this would follow similar integrations, e.g. for [MLFlow](https://github.com/huggingface/transformers/blob/v4.57.3/src/transformers/integrations/integration_utils.py#L1361).

### Non-Goals

1. **Store the history of how metrics changed throughout training.** This feature is not trying to replicate or replace functionality provided by tools like MLFLow and Tensorboard. These tools can be used alongside this feature to record richer information about the training process. However, unlike MLFlow and Tensorboard, this feature has zero dependencies (e.g. no additional infrastructure) and will work out "of-the-box" without additional set up.
2. **Provide progress and metrics for dataset and model initialization.** The implementation could be easily extended for these phases of the TrainJob api, but we propose delaying this to a future iteration to limit scope.
3. **Automatically instrument custom trainer training jobs to have progress tracking.**
4. **Add progress tracking for the Kubeflow Trainer v1 api.** The v1 api is legacy.  This feature should only be added to the v2 api.
5. **Integrate with Katib’s hyperparameter optimiser.** Whilst the exposed training metrics should provide a route for this integration, the integration is out of scope of this proposal.

## User Stories

### Story 1: Platform Engineer integrating with third parties

As a platform engineer, I want to be able to access information about training jobs so I can integrate Kubeflow Trainer with another application (e.g. a dashboard, a hyperparameter optimiser).

### Story 2: Data Scientist / ML Engineer monitoring training jobs

As a data scientist or ML Engineer, I want to see real-time information about my training jobs so I can make decisions on whether those jobs are likely to succeed or whether intervention is required.

I can use standard tools, like the Kubeflow Trainer Python SDK or `kubectl get trainjob` to access progress and performance metrics about my train jobs.

### Story 3: Data Scientist / ML Engineer opting out of monitoring training jobs

As a data scientist or ML Engineer, I want to create a TrainJob but I do not want to have to work out how to integrate training monitoring.

## Design Details

We propose an approach with the following high-level **push-based** design:

1. The TrainJob custom resource exposes the current training progress and metrics via a new optional field `status.trainerStatus`.
2. The trainer control plane exposes a new http service which can receive the trainer status from the trainer runtime pods.
3. The user instruments their trainer runtime pod(s) to periodically send the current trainer status to this endpoint which updates the status of the TrainJob.

Users can choose not to instrument their runtime, in which case no progress and metrics will be available on the TrainJob. The feature is therefore optional and opt-in.

The feature will have an associated feature gate `TrainJobProgress`, defaulting to "disabled". Disabling the gate will disable the http service.

### CRD changes

The TrainJob API would be updated to include a new optional `status.trainerStatus` field with this schema:

```go
type TrainJobStatus struct {
    // ... existing fields

    // trainerStatus provides a summary of the status of the training
	// part of the TrainJob.
    // Empty if the status is unknown, e.g. the job has just started
    // or the job is not instrumented to report its status.
    // +optional
    TrainerStatus *TrainJobTrainerStatus `json:"trainerStatus,omitempty"`

    // Future extension (out of scope):
    // DataInitializerStatus *TrainJobDataInitializerStatus `json:"dataInitializerStatus,omitempty"`
    // ModelInitializerStatus *TrainJobModelInitializerStatus `json:"modelInitializerStatus,omitempty"`
}


type TrainJobTrainerStatus struct {

    // progressPercentage gives an estimate of how complete the TrainJob is as a percentage.
    // The value will be between 0 and 100, or empty if unknown.
    //
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Maximum=100
    // +optional
    ProgressPercentage *int32 `json:"progressPercentage,omitempty"`

    // estimatedRemainingSeconds gives the estimated remaining training time in seconds
    // before the train job is completed.
    // The value will be empty if it is unknown.
    //
    // +kubebuilder:validation:Minimum=0
    // +optional
    EstimatedRemainingSeconds *int32 `json:"estimatedRemainingSeconds,omitempty"`

    // metrics contains the current metrics for the model.
    //
    // +listType=atomic
    // +optional
    Metrics []Metric `json:"metrics,omitempty"`

    // lastUpdatedTime is the timestamp when these metrics were observed.
    // +optional
    LastUpdatedTime metav1.Time `json:"lastUpdatedTime,omitempty"`
}

type Metric struct {
    // name is a user-defined label for the metric, e.g. "loss", "eval_accuracy".
    // +kubebuilder:validation:MinLength=1
	// +required
    Name string `json:"name,omitempty"`

    // value of the metric. Values must be serialized as a string.
	// +kubebuilder:validation:MinLength=1
    // +required
    Value string `json:"value,omitempty"`
}
```

The trainerStatus field is optional as it can be unavailable, e.g. because the job is still initializing and status messages have not yet been emitted, or if the runtime has not been instrumented to expose the trainer status.

All fields (apart from lastUpdatedTime) are optional meaning that a runtime need only provide information that it has available or is relevant for that training algorithm (e.g., epochs are not relevant for XGBoost models).

The design deliberately does not make any changes to the `TrainJobSpec`: the control plane does not require any configuration. Users opt in to the training status by instrumenting their runtime pods to send the training status to the control plane.

The design may be extended in future to add equivalent progress and metric statuses for the model and data initializer components.

```yaml
# Sample TrainJob example with TrainerStatus implemented

apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
...
status:
  trainerStatus:
    # Overall progress
    progressPercentage: 45                           # 45% complete
    estimatedRemainingSeconds: 795649                # Precise duration

    # The most recent training metrics that were reported
    metrics:
      - name: loss                                   # Current training loss
        value: "0.2347"
      - name: eval_loss                              # Current validation loss
        value: "0.2451"
      - name: accuracy                               # Current training accuracy
        value: "0.9876"
      - name: currentEpoch                           # Current training epoch that is being evaluated
        value: "2"
      - name: totalEpochs                            # The total number of epochs that may be performed
        value: "5"

    # Timestamp of last progress update
    lastUpdatedTime: "2025-01-23T10:30:45Z"
```

We also propose adding the `Progress %` to the printer columns for the TrainJob custom resource:

```
$ kubectl get trainjob
NAME              STATE      PROGRESS %  AGE
an-example        Running    3           13m
another-example   Complete   100         50m
```

### Control plane endpoint

If the feature gate is enabled, the control plane will expose a new http server endpoint where trainer pods can submit the trainer status. The http server will be added as a new port in the existing `kubeflow-trainer-controller-manager` service.

The endpoint will be `POST: /apis/trainer.kubeflow.org/v1alpha1/namespaces/{namespace}/trainjobs/{name}/status`, where `{namespace}` and `{name}` are the namespace and name of the TrainJob.

The payload for the endpoint will have the following schema:
```go
type ProgressStatus {
    // trainerStatus provides a summary of the status of the training
    // part of the TrainJob.
    // Empty if the status is unknown, e.g. the job has just started
    // or the job is not instrumented to report its status.
    // +optional
    TrainerStatus *TrainJobTrainerStatus `json:"trainerStatus,omitempty"`

	// Future extension (out of scope):
	// DataInitializerStatus *TrainJobDataInitializerStatus `json:"dataInitializerStatus,omitempty"`
	// ModelInitializerStatus *TrainJobModelInitializerStatus `json:"modelInitializerStatus,omitempty"`
}
```

The schema uses a nested structure for future extensibility (e.g. the same endpoint could be used to receiver progress updates from a data initializer or model initializer).

On receiving requests to this endpoint, the control plane will validate the source of the request (see [Security considerations](#security-considerations)) and then directly update the `status.trainerStatus` field.

The control plane does not need to be highly available: the runtime can retry the status update request with some delay whilst continuing the training, or skip the update entirely.

An example payload is:
```
POST /apis/trainer.kubeflow.org/v1alpha1/namespaces/{namespace}/trainjobs/{name}/status HTTP/1.1
Host: kubeflow-trainer-controller-manager.kubeflow:8082
Authorization: Bearer {jwt}
Content-Type: application/json

{
  "trainerStatus": {
    "progressPercentage": 45,
    "estimatedRemainingSeconds": 795649,
    "metrics": [
      {
        "name": "loss",
        "value": "0.2347"
      },
      {
        "name": "eval_loss",
        "value": "0.2451"
      },
      {
        "name": "accuracy",
        "value": "0.9876"
      },
      {
        "name": "currentEpoch",
        "value": "2"
      },
      {
        "name": "totalEpochs",
        "value": "5"
      }
    ],
    "lastUpdatedTime": "2025-01-23T10:30:45Z"
  }
}
```

### Security considerations

The control plane endpoint will be secured with TLS and token-based authentication.

TLS will use the same certificate used by the webhook:
- for each TrainJob, the control plane will copy the webhook certificate authority cert into a configmap in the train job namespace. The control plane will need to watch the source secret and synchronize any changes (e.g. CA certificate rotated).
- the control plane will inject the `ca.crt` file into all containers of all pods of the TrainJob using a configmap volume.
- the runtime will need to use the custom `ca.crt` file when it makes requests to the control plane.

Token-based authentication using projected service account tokens will be used to validate that the train job status can only be updated by pods that belong to that train job:
- if the feature gate is enabled the control plane will inject a serviceAccountToken projected volume with custom audience `trainer.kubeflow.org` into all containers of all pods of the train job.
- the runtime will need to send this token as a bearer token in all status update requests.
- on receiving a status update request, the control plane will authorise the token by:
  1. validating the provided token using a TokenReview request. The audience must be `trainer.kubeflow.org`.
  2. validating the pod is part of the train job by:
    - getting the pod name/namespace from the `kubernetes.io` claim of the decoded jwt token
    - looking up all the pods for the train job
    - verifying that the source pod is in the list of pods for the train job

Note that the token does _not_ have permissions for the kubernetes API server due to the custom audience. The control plane also does _not_ require the service account that the token is associated with to be granted any RBAC permissions. It only requires that the token is a valid, comes from the API server, and was issued to a pod that belongs to the TrainJob.

The new endpoint must rate-limit requests and cache TokenReview responses to avoid a misconfigured TrainJob (examples: a job with many nodes which all send status updates simultaneously; a job that progresses very quickly) from triggering API server rate-limiting which may cause denial-of-service for other TrainJobs or for the reconciler itself. The rate-limiting should be keyed on the jwt subject claim (the service account name and namespace).

Deliberate malicious attacks can be mitigated by validating the jwt token against the API server public key before performing the TokenReview request. Tokens that are definitely invalid would not cause any API server requests.

The below summarises the volumes the control plane will inject if the feature gate is enabled:
```yaml
# mutated PodSpec
spec:
  containers:
  - name: worker
    volumeMounts:
    - name: kubeflow-trainer-token
      mountPath: "/var/run/secrets/kubeflow/trainer"
      readOnly: true
  volumes:
  - name: kubeflow-trainer-token
    projected:
      sources:
      - serviceAccountToken:
        audience: trainer.kubeflow.org
        expirationSeconds: 3600
        path: token
      - configmap:
        name: <train-job-name>-tls-config
        items:
          - key: ca.crt
            path: ca.crt
```

### RBAC changes

The control plane service account requires these additional RBAC permissions:

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
  - pods
  verbs:
  - get
  - list
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
```

### Runtime instrumentation

Users will need to instrument their train jobs so that they periodically send training status to the control plane. Some frameworks make this easy by letting users add "hooks" or "callbacks" to the training loop, and where possible we'll seek to add integrations to those libraries. For example, for the `Transformers` framework we'll look to add a custom `KubeflowTrainerCallback` following the [existing integrations approach](https://github.com/huggingface/transformers/blob/v4.57.0/src/transformers/integrations/integration_utils.py#L1361).

To make it easier for training pods to update the training status, if the feature gate is enabled the control plane will inject the following environment variables into all containers of all pods of the training job:
```shell
KUBEFLOW_TRAINER_STATUS_URL=https://kubeflow-trainer-controller-manager.kubeflow:8082/apis/trainer.kubeflow.org/v1alpha1/namespaces/{namespace}/trainjobs/{name}/status
KUBEFLOW_TRAINER_STATUS_CA_CERT=/var/run/secrets/kubeflow/trainer/ca.crt
KUBEFLOW_TRAINER_STATUS_TOKEN=/var/run/secrets/kubeflow/trainer/token
```

These environment variables make it easy for any pod to report the runtime code for submitting status updates, e.g.:
```python
from urllib import request
import os
import ssl

def update_training_status(payload):
    try:
        url = os.environ["KUBEFLOW_TRAINER_STATUS_URL"]
        ca_file = os.environ["KUBEFLOW_TRAINER_STATUS_CA_CERT"]
        token = open(os.environ["KUBEFLOW_TRAINER_STATUS_TOKEN"]).read()
        ssl_context = ssl.create_default_context(cafile=ca_file)
        req = request.Request(url, data=payload, headers={"Authorization": f"Bearer {token}"})
        request.urlopen(req, ssl_context=ssl_context)
    except Exception as ex:
        print(f"[Kubeflow] Unable to send trainer status. {e}")
```

### Kubeflow SDK Changes

The Kubeflow SDK will be updated to add a new trainerStatus field to the `TrainJob` response object.

```py
@dataclass
class Metric:
    name: str
    value: str


@dataclass
class TrainerStatus:
    progressPercentage: Optional[int]
    estimatedRemainingDurationSeconds: Optional[int]
    metrics: Optional[list[Metric]]
    lastUpdatedTime: datetime


@dataclass
class TrainJob:
    # ... existing fields
    trainerStatus: Optional[TrainerStatus] = None
```

## Other considered alternatives

This section describes other approaches that were evaluated and the rationale for not selecting them.

### Pull-based: control plane scrapes the metrics from the

We also considered in detail an alternative **pull-based** approach with the following high-level design:

1. The TrainJob custom resource exposes the current training progress and metrics via a new optional field `status.trainerStatus`.
2. The user configures their (Cluster)TrainerRuntime so that **one** of the ReplicatedJobs has annotations that enable monitoring.
3. The user instruments their trainer runtime code so that the current trainer status is written to a local file in a specific format.
4. The control plane injects a sidecar container into one of the runtime pods of the configured ReplicatedJob. The sidecar has access to the local file through a shared volume, and contains an http server that serves the training progress and metrics from the shared file.
5. The trainer control plane periodically scrapes the http server to fetch the progress and metrics and then updates the TrainJob custom resource.
6. When training is completed, the sidecar container exposes the final contents of the shared file through its termination message which is read by the control plane. This ensures the final train status is collected.

As in the push-based design, the feature is optional but available for all TrainJobs. Users opt in to the functionality by adding configuration to their TrainingRuntime and instrumenting their runtime to write the metrics.

#### Design Details

##### CRD changes

The same changes will be made to the TrainJob as in the [push-based design](#crd-changes).

##### Sidecar container injection

To enable monitoring and sidecar injection, users must add the annotation `trainer.kubeflow.org/trainjob-monitoring-step: trainer` to one of the replicated jobs in their (Cluster)TrainingRuntime. This will cause the control plane to:
- inject a [sidecar container](https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/) into **one** of the trainer pods
- inject a shared `emptyDir` volume between the sidecar and all other containers in that pod.
- create a new service `<train-job-name>-trainer-monitoring` pointing at the sidecar http server. The control plane will add the label `trainer.kubeflow.org/trainjob-monitoring-pod: <train-job-name>` to the pod which will be used in the service selector.

The sidecar will contain a lightweight http server packaged as a new image published with the Kubeflow Trainer release. The server reads the current metrics directly from a file in the shared volume when handling each request.

Users can optionally add the annotations `trainer.kubeflow.org/monitoring-port: <port-number>` (defaults to `28080`) and `trainer.kubeflow.org/monitoring-interval: <duration>` (defaults to `30s`) to replicated job in their (Cluster)TrainingRuntime to configure the sidecar container port and scrape interval.

Additional details:
- The control plane will set the sidecar `terminationMessagePath` to the location of the shared file to allow the final metrics to be collected. See [Scraping the metrics](#scraping-the-metrics).
- The control plane will inject the sidecar in only one pod to minimise resource usage. This will be achieved by duplicating the target replicated job (after merging with PodTemplateSpecOverrides), setting the replicas to 1, injecting sidecar and volume, and updating other replicate job dependencies as necessary.
- Resource requests for the sidecar can be configured globally via a control plane config setting (i.e. the same resource requests will be used for all TrainJobs in the cluster).

The below gives an example ClusterTrainingRuntime with monitoring enabled:
```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
spec:
  # ...
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              annotations:
                trainer.kubeflow.org/trainjob-monitoring-step: trainer
                trainer.kubeflow.org/trainjob-monitoring-port: 28080
                trainer.kubeflow.org/trainjob-monitoring-interval: 30s
            spec:
              template:
                ...
```

The below summarises the configuration the control plane will inject:
```yaml
# mutated PodSpec
spec:
  initContainers:
  - name: trainer-metrics-server
    image: ghcr.io/kubeflow/trainer/trainer-metrics-server
    restartPolicy: Always  # configure as sidecar
    terminationMessagePath: /var/kubeflow/trainer/status/status.json
    volumeMounts:
    - mountPath: /var/kubeflow/trainer/status
      name: kubeflow-trainer-status
      readonly: true
  containers:
  - name: node
    ...
    volumeMounts:
      - mountPath: /var/kubeflow/trainer/status
        name: kubeflow-trainer-status
  volumes:
    - name: kubeflow-trainer-status
      emptyDir: {}
```

##### Scraping the metrics

While the train job is active, the control plane will scrape the metrics server as part of its reconciliation loop using the `<train-job-name>-trainer-monitoring` service and update the TrainJob `trainingStatus`. The reconciliation will be re-queued based on the interval in the `trainer.kubeflow.org/monitoring-interval` label which will trigger the next scrape.

To ensure the final training status is collected after training has completed successfully, the control plane will configure the sidecar `terminationMessagePath` to point to the shared metrics file. When the job is terminated, the control plane will read the contents of the sidecar termination message from the Pod config.

##### Runtime instrumentation

The user must instrument their runtime code so the main runtime container(s) will periodically write the current training status to a file in the shared volume. The file must contain a single json entry payload, with schema the same as `TrainJobTrainerStatus`.

The control plane will inject the following environment variable into all containers of all pods of the training job:
```shell
KUBEFLOW_TRAINER_STATUS_FILE=/var/kubeflow/trainer/status/status.json
```

When updating the training status, the main container must replace the file contents so the file only ever contains a single status. This replacement should be done atomically using a temporary file followed by a rename to avoid the race condition of the http server reading a partially updated file.

##### Security considerations

Securing the http server with auth and TLS can be achieved as follows:
- the control plane creates a secret in the train job namespace containing an API key and a self-signed certificate. The control plane could periodically rotate these secrets.
- the secret is accessed by the sidecar container through a volume mount.
- before scraping the sidecar container, the control plane looks up the API key and certificate authority from the secret.

However, as the data exposed by the http server may not be considered particularly sensitive, it may be acceptable to expose the metrics server without auth or TLS which would avoid a lot of complexity.

##### RBAC changes

No RBAC changes are required.

#### Comparison of the  approaches

|                                | Pull-based web request (the proposed approach)                                                                                           | Push-based http server sidecar (this alternative)                                                                                                                           | Preferred approach                                      |
|--------------------------------|------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------|
| **User experience**            | Simple user experience, equivalent to MLFlow. Always available and users opt-in entirely through runtime code. Easier to debug problems. | Enabling the monitoring requires configuring the (Cluster)TrainingRuntime and instrumenting the runtime code. Errors are harder to debug (users must inspect sidecar logs). | Pull-based (web request) is much simpler UX.            |
| **Security**                   | New external endpoint creates an additional threat route for the control plane, e.g. for accidental/malicious denial-of-service.         | Securing the server with TLS requires putting secrets into user namespace. The blast radius from compromised TLS/auth secrets is small though.                              | Both approaches introduce security concerns.            |
| **Robustness**                 | Users must ensure failed web requests are handled and do not terminate terminate the training loop.                                      | Errors are less likely to terminate the training loop.                                                                                                                      | Push-based (http server) is marginally more robust.     |
| **Complexity and maintenance** | Significant complexity in the endpoint auth mechanism, but complexity is managed once in the control plane.                              | Some complexity in selecting the pods to instrument with sidecar. Requires a new image to be maintained as part of the release.                                             | Push-based (http server) adds slightly less complexity. |
| **Compatibility**              | Compatible with any k8s version.                                                                                                         | Relies on sidecar containers which is k8s 1.33+ (beta in 1.29+).                                                                                                            | Pull-based (web request) has no compatibility problems. |
| **Flexibility**                | Highly flexible. Can support any framework that supports runtime instrumentation (e.g. using callbacks).                                 | Highly flexible. Can support any framework that supports runtime instrumentation (e.g. using callbacks).                                                                    | Both approaches are equally flexible.                   |
| **Scalability**                | Should be highly scalable to thousands of simultaneous train jobs assuming train jobs update the status relatively infrequently.         | Highly scalable. The control plane can scrape the training status on best-effort.                                                                                           | Both approaches should be sufficiently scalable.        |

### Trainer Pods updating the TrainJob status directly

The trainer pods could directly interact with the API server to update the TrainJob status.

Pros:

- No need for new control plane service.

Cons:

- The trainer pods require privileged access to the API server. Given these pods are running arbitrary user code, this would warrant additional security sandboxing.
- The trainer pods cannot use the default service account. The control plane would need to automatically create a service account with the required permissions for a train job, or users would need to provide a service account and ensure it has the necessary permissions.
- The trainer runtime require a kubernetes client to be available, meaning it must either be pre-installed in the runtime or installed/injected at runtime.

### Exposing metrics via MLFlow or Prometheus

The runtime is instrumented with an MLFlow or Prometheus client which tracks and exposes the metrics. The controller manager reads the metrics from MLFlow or Prometheus and updates the custom resource.

Pros:

- No need for new control plane service.
- Uses an existing standard and framework.
- Provides support for tracking the history of progress/metrics.

Cons:

- Introduces an external dependency to the deployment.

### Communicating the training status through the pod logs

The runtime pods write the training status to the pod logs via stdout in an agreed format; the Kubeflow Trainer controller manager subscribes to the pod logs and updates the TrainJob status when log messages are observed.

Pros:

- No need for new control plane service.
- Push based: the training status is updated immediately.
- Simple security model: cluster RBAC can be used to restrict access to the cluster
- Less susceptible to hard-to-diagnose network misconfigurations, e.g. network policies blocking http requests.

Cons:

- Less scalable: runtime pods may output a high volume of log messages that need transferring to the controller; the approach may not scale to many simultaneously running train jobs. It also may put non-trivial burden on the kubernetes API server.
- Reconciliation is no longer stateless: the controller must keep a long-lived request open for each active train job. Resuming broken requests is non-trivial.
- Less secure: the controller must be granted clusterwide permission to read pod logs for any pod in the cluster.
