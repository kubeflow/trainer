<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

**Table of Contents** _generated with [DocToc](https://github.com/thlorenz/doctoc)_

- [TF-Operator Design (v1alpha2)](#tf-operator-design-v1alpha2)
  - [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [UI or API](#ui-or-api)
  - [Design](#design)
    - [TFController](#tfcontroller)
    - [Distributed TensorFlow Configuration](#distributed-tensorflow-configuration)
    - [Event-Driven](#event-driven)
    - [Reconciler](#reconciler)
    - [Error Handling](#error-handling)
    - [Test](#test)
  - [Alternatives Considered](#alternatives-considered)
    - [Future Works](#future-works)
    - [Related Issues](#related-issues)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

_Authors:_

- @ScorpioCPH - Penghao Cen &lt;cenph@caicloud.io&gt;

_Status_

- 2018-03-18 - Accepted
- 2018-04-10 - Implementation Started
- 2018-07-02 - v1alpha2 released in 0.2

# KEP-30: TF-Operator Design (v1alpha2)

## Motivation

Kubeflow community currently have [tf-operator](https://github.com/kubeflow/tf-operator) (v1alpha1) for running TensorFlow jobs on Kubernetes. And we have received some refactoring [requests](https://github.com/kubeflow/tf-operator/issues?q=is%3Aissue+is%3Aopen+label%3Akind%2Fapi-change) for API changes.
Open this file to summarize the design details and move the version of API to `v1alpha2`.

## Goals

- Define the structure of API `v1alpha2`.
  - Cover most of the refactoring requests we have discussed.
  - Simplify the API definition.
- Define an `event-driven` mechanism for TFJob life-cycle management.
  - And use `reconciler` mechanism as a double check.
- Clarify the `error handing` logic.
- Provide a `test` mechanism to verify the design and implementation.

## Non-Goals

- **Notes:** As we make a big change in API v1alpha2, compatibility with v1alpha1 is **NOT** be taken into consideration in this proposal.

## UI or API

The `TFJob` API v1alpha2 object will have the following structure:

**TFJob**:

```go
// TFJob represents the configuration of signal TFJob
type TFJob struct {
    metav1.TypeMeta `json:",inline"`

    // Standard object's metadata.
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // Specification of the desired behavior of the TFJob.
    Spec TFJobSpec `json:"spec,omitempty"`

    // Most recently observed status of the TFJob.
    // This data may not be up to date.
    // Populated by the system.
    // Read-only.
    Status TFJobStatus `json:"status,omitempty"`
}
```

**TFJobSpec**:

```go
// TFJobSpec is a desired state description of the TFJob.
type TFJobSpec struct {
    // TFReplicaSpecs is map of TFReplicaType and TFReplicaSpec
    // specifies the TF replicas to run.
    // For example,
    //   {
    //     "PS": TFReplicaSpec,
    //     "Worker": TFReplicaSpec,
    //   }
    TFReplicaSpecs map[TFReplicaType]*TFReplicaSpec `json:"tfReplicaSpecs"`
}
```

**TFReplicaSpec**:

```go
// TFReplicaSpec is a description of the TFReplica
type TFReplicaSpec struct {
    // Replicas is the desired number of replicas of the given template.
    // If unspecified, defaults to 1.
    Replicas *int32 `json:"replicas,omitempty"`

    // Template is the object that describes the pod that
    // will be created for this TFReplica.
    // We use RestartPolicy in PodTemplateSpec
    // to describe how the containers within the pod should be restarted.
    // Please set this restart policy carefully according to your code.
    Template *v1.PodTemplateSpec `json:"template,omitempty"`

    // Restart policy for all TFReplicas within the TFJob.
    // One of Always, OnFailure, Never and ExitCode.
    // Default to Always.
    RestartPolicy RestartPolicy `json:"restartPolicy,omitempty"`
}

// RestartPolicy describes how the TFReplicas should be restarted.
// Only one of the following restart policies may be specified.
// If none of the following policies is specified, the default one
// is RestartPolicyAlways.
type RestartPolicy string

const (
    RestartPolicyAlways    RestartPolicy = "Always"
    RestartPolicyOnFailure RestartPolicy = "OnFailure"
    RestartPolicyNever     RestartPolicy = "Never"
    RestartPolicyExitCode  RestartPolicy = "ExitCode"
)
```

**TFReplicaType**:

```go
// TFReplicaType is the type for TFReplica.
type TFReplicaType string

const (
    // TFReplicaTypePS is the type for parameter servers of distributed TensorFlow.
    TFReplicaTypePS TFReplicaType = "PS"

    // TFReplicaTypeWorker is the type for workers of distributed TensorFlow.
    TFReplicaTypeWorker TFReplicaType = "Worker"

    // TFReplicaTypeChief is the type for chief worker of distributed TensorFlow.
    // If there is "chief" replica type, it's the "chief worker".
    // Else, worker:0 is the chief worker.
    TFReplicaTypeChief TFReplicaType = "Chief"

    // TFReplicaTypeEval is the type for evaluation replica in TensorFlow.
    TFReplicaTypeEval TFReplicaType = "Eval"
)
```

**TFJobStatus**:

```go
// TFJobStatus represents the current observed state of the TFJob.
type TFJobStatus struct {
    // TFReplicaStatuses is map of TFReplicaType and TFReplicaStatus,
    // specifies the status of each TFReplica.
    TFReplicaStatuses map[TFReplicaType]*TFReplicaStatus `json:"tfReplicaStatuses"`

    // Represents time when the TFJob was acknowledged by the TFJob controller.
    // It is not guaranteed to be set in happens-before order across separate operations.
    // It is represented in RFC3339 form and is in UTC.
    StartTime *metav1.Time `json:"startTime,omitempty"`

    // Represents time when the TFJob was completed. It is not guaranteed to
    // be set in happens-before order across separate operations.
    // It is represented in RFC3339 form and is in UTC.
    CompletionTime *metav1.Time `json:"completionTime,omitempty"`

    // Represents last time when the TFJob was reconciled. It is not guaranteed to
    // be set in happens-before order across separate operations.
    // It is represented in RFC3339 form and is in UTC.
    LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

    // Represents is an array of current observed TFJob conditions.
    Conditions []TFJobCondition `json:"conditions"`
}
```

**TFReplicaStatus**:

```go
// TFReplicaStatus represents the current observed state of the TFReplica.
type TFReplicaStatus struct {
    // The number of actively running pods.
    Active int32 `json:"active,omitempty""`

    // The number of pods which reached phase Succeeded.
    Succeeded int32 `json:"succeeded,omitempty"`

    // The number of pods which reached phase Failed.
    Failed int32 `json:"failed,omitempty"`
}
```

**TFJobCondition**:

```go
// TFJobCondition describes the state of the TFJob at a certain point.
type TFJobCondition struct {
    // Type of TFJob condition.
    Type TFJobConditionType `json:"type"`

    // Status of the condition, one of True, False, Unknown.
    Status v1.ConditionStatus `json:"status"`

    // The reason for the condition's last transition.
    Reason string `json:"reason,omitempty"`

    // A human readable message indicating details about the transition.
    Message string `json:"message,omitempty"`

    // The last time this condition was updated.
    LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

    // Last time the condition transitioned from one status to another.
    LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}
```

**TFJobConditionType**:

```go
// TFJobConditionType defines all kinds of types of TFJobStatus.
type TFJobConditionType string

const (
    // TFJobCreated means all sub-resources (e.g. services/pods) of this TFJob
    // have been successfully created.
    // But they are waiting to be scheduled and launched.
    TFJobCreated TFJobConditionType = "Created"

    // TFJobRunning means all sub-resources (e.g. services/pods) of this TFJob
    // have been successfully scheduled and launched.
    // The training is running without error.
    TFJobRunning TFJobConditionType = "Running"

    // TFJobRestarting means one or more sub-resources (e.g. services/pods) of this TFJob
    // reached phase failed but maybe restarted according to it's restart policy
    // which specified by user in v1.PodTemplateSpec.
    // The training is freezing/pending.
    TFJobRestarting TFJobConditionType = "Restarting"

    // TFJobSucceeded means all sub-resources (e.g. services/pods) of this TFJob
    // reached phase have terminated in success.
    // The training is complete without error.
    TFJobSucceeded TFJobConditionType = "Succeeded"

    // TFJobFailed means one or more sub-resources (e.g. services/pods) of this TFJob
    // reached phase failed with no restarting.
    // The training has failed its execution.
    TFJobFailed TFJobConditionType = "Failed"
)
```

## Design

### TFController

The TFJob controller `tf-operator` will process TFJobs and CRUD services/pods according to the spec of TFJob. It is responsible for synchronizing TFJob objects stored in the system with actual running services and pods, continuously strive to make the observed state match the desired state.

Here is the definition of `TFController`:

```go
type TFController struct {
    // kubeClientset is a standard kubernetes clientset
    kubeClientset kubernetes.Interface

    // tfJobClientset is a clientset for CRD TFJob
    tfJobClientset tfjobclient.Interface

    tfJobLister listers.TFJobLister
    tfJobSynced cache.InformerSynced

    // for pod/service CRUD
    podLister     kubelisters.PodLister
    podControl    controller.PodControlInterface
    serviceLister kubelisters.ServiceLister

    // workQueue is a rate limited work queue. This is used to queue work to be
    // processed instead of performing it as soon as a change happens. This
    // means we can ensure we only process a fixed amount of resources at a
    // time, and makes it easy to ensure we are never processing the same item
    // simultaneously in two different workers.
    workQueue workqueue.RateLimitingInterface

    // recorder is an event recorder for recording Event resources to the
    // Kubernetes API.
    recorder record.EventRecorder

    // A TTLCache of pod creates/deletes each TFReplica expects to see
    expectations controller.ControllerExpectationsInterface
}
```

### Distributed TensorFlow Configuration

**Auto-Generated TF_CONFIG:**

To make distributed TensorFlow work, user **should** get the distributed TensorFlow configurations `TF_CONFIG` which generated by `tf-operator`.
This config looks like this:

```json
{
    "cluster": {
        "ps": ["ps1:2222", "ps2:2222"],
        "worker": ["worker1:2222", "worker2:2222", "worker3:2222"]
    },
    "task": {
        "type": "ps",
        "index": 1
        },
    }
}
```

`tf-operator` will append these auto-generated environment variables into `Env` field.
Check more details from [here](https://cloud.google.com/ml-engine/docs/trainer-considerations#use_tf_config).

**User-Defined Arguments:**

Other user-defined arguments can also be passed into container by `Args` field in `Container` struct.

### Event-Driven

First, we should follow the `Event-Driven` pattern as other resource controller in kubernetes (e.g. Deployment/Job):

- Start `tfJobInformer` to listen on CRUD events of TFJob.
  - `tfJobInformer` was automatically generated from API definition by `informer-gen` script.
- Create one pair pod/service for each specify TFReplicaType + replica index in TFJob CreateHandler.
  - For example, as a given TFReplicaSpec:
    ```
    {
      "PS": {
          Replicas: 2,
      },
      "Worker": {
          Replicas: 3,
      },
    }
    ```
    We will create:
    - `two` pair pods/services for PSs:
      - tf-job-name-ps-1-uid
      - tf-job-name-ps-2-uid
    - `three` pair pods/services for Workers:
      - tf-job-name-worker-1-uid
      - tf-job-name-worker-2-uid
      - tf-job-name-worker-3-uid
  - We use a postfix `uid` to make each object name unique.
  - Then set these objects' `OwnerReferences` to this TFJob object.
- Listen on pods/services via `podInformer` and `serviceInformer`.
  - On pod created/updated/deleted, get TFJob object by parsing `OwnerReferences`, set the `TFJob.Status` as defined above according to the whole TF cluster state.
  - Update the `TFJob.Status.Condition` if needed.
- Terminate/Delete the TFJob object if every pod is completed (or leave pod phase as `Succeeded`).
  - This maybe be lead to logs and model checkpoint files unreachable.

### Reconciler

More than that, we should provide a `Reconciler` mechanism to reconcile observed and desired states and repair discrepancies as a double check for `Event-Driven` mechanism.

Here is `configuration` of tf-operator:

```go
// TFControllerConfiguration contains configuration of tf-operator.
// DefaultTimerConfig is the suggested tf-operator configuration for production.
type TFControllerConfiguration struct {
    // ReconcilerSyncLoopPeriod is the amount of time the reconciler sync states loop
    // wait between two reconciler sync.
    // It is set to 15 sec by default.
    // TODO(cph): maybe we can let it grows by multiple in the future
    // and up to 5 minutes to reduce idle loop.
    // e.g. 15s, 30s, 60s, 120s...
    ReconcilerSyncLoopPeriod metav1.Duration
}

// DefaultTFControllerConfiguration is the suggested tf-operator configuration for production.
var DefaultTFControllerConfiguration TFControllerConfiguration = TFControllerConfiguration{
    ReconcilerSyncLoopPeriod: 15 * time.Second,
}
```

Reconciler use a `ReconcilerSyncLoopPeriod` to determine whether we should call this reconciler or ignore it. We should leave a record `LastReconcileTime` in TFJob object of course:

```go
    // Represents last time when the TFJob was reconciled. It is not guaranteed to
    // be set in happens-before order across separate operations.
    // It is represented in RFC3339 form and is in UTC.
    LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
```

As `tfJobImformer` provides a forcing resync mechanism by calling `UpdateFunc` which defined in `ResourceEventHandlerFuncs` periodically. We can call the reconciler in this function:

- UpdateFunc return a TFJob object periodically.
- Check `LastReconcileTime` to determine whether we should trigger a reconciler call.
- `tf-operator` will list all pods/services which related to this TFJob.
  - Compare the current state to the spec of this TFJob.
  - Try to recovery the failed pod/service to make the training healthy.
    - Error handing is described below.
- Update the status of this TFJob.
- TODO: we should call this reconciler with an exponential back-off delay (15s, 30s, 60s …) capped at 5 minutes.

### Error Handling

To make the system robust, the tf-operator should be able to locally and automatically recover from errors.

We extend kubernetes built-in `RestartPolicy` by adding new policy `ExitCode`:

```go
    RestartPolicyAlways    RestartPolicy = "Always"
    RestartPolicyOnFailure RestartPolicy = "OnFailure"
    RestartPolicyNever     RestartPolicy = "Never"
    RestartPolicyExitCode  RestartPolicy = "ExitCode"
```

We let users set this field according to their model code.

- If set RestartPolicy to `OnFailure`/`Always`, user should add reloading checkpoint code by themselves.
- Otherwise restarting will take no effect.

`ExitCode` policy means that user should add exit code by themselves, `tf-operator` will check these exit codes to determine the behavior when a error occurs:

- 1-127: permanent error, do not restart.
- 128-255: retryable error, will restart the pod.

### Test

**Unit Test**

TBD

**E2E Test**

We can use this model from TensorFlow [repo](https://github.com/tensorflow/tensorflow/tree/master/tensorflow/tools/dist_test) for e2e test.

## Alternatives Considered

### Future Works

Apart from the above, we should add these abilities in the future:

- Provide a properly mechanism to store training logs and checkpoint files.
  - [FYI](https://github.com/kubeflow/tf-operator/issues/128)

### Related Issues

- [Refactor TFJobStatus in CRD API](https://github.com/kubeflow/tf-operator/issues/333)
- [Deprecate the TfImage field](https://github.com/kubeflow/tf-operator/issues/330)
- [[discussion] Differences between tensorflow/k8s and caicloud/kubeflow-controller](https://github.com/kubeflow/tf-operator/issues/283)
- [API: some comments about API changes from PR #215 review](https://github.com/kubeflow/tf-operator/issues/249)
- [Use conditions instead of phase](https://github.com/kubeflow/tf-operator/issues/223)
- [API Review](https://github.com/kubeflow/tf-operator/issues/64)
- [Phase is wrong unexpected TfJob phase: Done](https://github.com/kubeflow/tf-operator/issues/110)
