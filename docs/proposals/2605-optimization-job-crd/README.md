# KEP: OptimizationJob CRD for Hyperparameter Optimization

- **Authors:** Aniket Shaha (@aniket2405)
- **Mentors:** @akshaychitneni, @andreyvelich
- **Target Issue:** kubeflow/katib#2605

---

## Index

1. [Background & Motivation](#1-background--motivation)
2. [Goals by Phase](#2-goals-by-phase)
3. [Non-Goals](#3-non-goals)
4. [Phase 1 API Design (v1alpha1)](#4-phase-1-api-design-v1alpha1)
5. [Sample YAML (Phase 1)](#5-sample-yaml-phase-1)
6. [Reconciliation & Architecture (Phase 1)](#6-reconciliation--architecture-phase-1)
7. [Open Discussions](#7-open-discussions)

---

## 1. Background & Motivation

Historically, Katib has served as Kubeflow’s general-purpose hyperparameter tuning and Neural Architecture Search (NAS) engine. It uses the generic `Experiment` CRD to orchestrate trials, supporting arbitrary Kubernetes workloads via unstructured YAML templates. 

While highly flexible, its broad scope creates friction for standard ML workflows. It forces users to write verbose YAML and relies on brittle regex string substitution (e.g., `${searchSpace.lr}`) to inject parameters. With the introduction of the unified Kubeflow Python SDK (KEP-46), there is a strong need for a strongly-typed, iterative orchestration layer that integrates natively with `TrainJobs` and relies on push-based metrics.

## 2. Goals by Phase

To ensure a stable and reviewable implementation, the project is broken down into strict phases to manage scope.

### Phase 1: Core Orchestration (v1alpha1)

- **TrainJob Feature Flag** (Hard Dependency): The unified TrainJob feature flag MUST be enabled in the cluster/controller environment. The OptimizationJob orchestrator relies entirely on this API and will not function without it.
- **Tighter TrainJob Integration:** Introduce the `OptimizationJob` CRD focused exclusively on `TrainJobs`, using a structured `TrainJobTemplateSpec` to enable native Kubernetes API validation while allowing user-defined metadata.
- **Native Parameter Injection:** Replace legacy brittle regex YAML substitution with native Go text templating (e.g., `{{.parameter_name}}`), allowing safe injection of hyperparameters anywhere within the `TrainJob` spec.
- **Dependency Reduction (No Katib DB):** Rely strictly on the `TrainJob` Progress API (via `status.trainerStatus`) for evaluating objective metrics, completely removing the dependency on Katib DB for the core MVP.
- **In-Process Algorithm Execution:** Run stateless algorithms (Random, Grid) in-process within the controller to reduce pod startup latency and validate the core loop.
- **Precision-Safe Typing**: All hyperparameter values and search space boundaries (min/max) are strictly serialized as strings in the API to prevent standard JSON float precision loss and align with Trainer v2 patterns.
- **Native CEL Validation**: Replace legacy validating webhooks with native Kubernetes Common Expression Language (CEL) rules to enforce constraints (e.g., parallelTrials <= numTrials, search space requirements) directly at the API server level.

### Phase 2: Stateful & Advanced Integrations

- **Stateful Algorithms:** Implement One-Shot Jobs for Bayesian/TPE to persist mathematical state across iterations.
- **Shared Initialization:** Integrate the `SharedInitializer` plugin (once mature) to share datasets across trials.

### Phase 3: Advanced Scheduling & Custom Algorithms

- **Early Stopping & Schedulers:** Explore integrating Schedulers (Median Stopping Rule, Hyperband), either natively in Katib or deferred to the `TrainJob` API.
- **Metric Strategies:** Support extracting min/max from trial history (pending potential MLflow integration).

## 3. Non-Goals

- **Neural Architecture Search (NAS):** NAS requires a fundamentally different, graph-structured search space model and is out of scope.
- **Arbitrary CRD Support:** Supporting arbitrary K8s Custom Resources (e.g., standard K8s Jobs) is dropped to enforce `TrainJob` stability.
- **Pull-Based Metrics:** Legacy sidecar metric collectors (Prometheus, stdout parsers) are omitted.

## 4. Phase 1 API Design (v1alpha1)

The MVP API surface is strongly typed to ensure native API server validation via OpenAPI schemas and CEL rules, rejecting malformed requests before they reach the controller.

```go
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// OptimizationJobSpec defines the desired state of OptimizationJob.
type OptimizationJobSpec struct {
    // Objectives defines the metrics and directions (maximize/minimize).
    Objectives []Objective `json:"objectives"`

    Algorithm Algorithm `json:"algorithm"`

    // EarlyStopping separates the pruning logic from the search algorithm.
    // +optional
    EarlyStopping *EarlyStopping `json:"earlyStopping,omitempty"`

    // Parameters define the search space boundaries.
    Parameters []Parameter `json:"parameters"`

    TrialConfig TrialConfig `json:"trialConfig"`

    // TrainJobTemplate wraps the underlying TrainJob workload.
    // Parameter propagation is handled via native string rendering before creation.
    TrainJobTemplate TrainJobTemplateSpec `json:"trainJobTemplate"`
}

type Objective struct {
    Metric    string `json:"metric"`
    Direction string `json:"direction"` // maximize or minimize
}

type Algorithm struct {
    Name     string      `json:"name"`
    Provider *string     `json:"provider,omitempty"`
    Settings []SettingKV `json:"settings,omitempty"`
}

type EarlyStopping struct {
    Name     string      `json:"name"`
    Settings []SettingKV `json:"settings,omitempty"`
}

type SettingKV struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type Parameter struct {
    Name        string      `json:"name"`
    SearchSpace SearchSpace `json:"searchSpace"`
}

// SearchSpace defines the type and boundaries.
// Validated via CEL to ensure min/max exist for numbers, and lists exist for categoricals.
type SearchSpace struct {
    Type string   `json:"type"` // int, double, categorical
    Max  string   `json:"max,omitempty"`
    Min  string   `json:"min,omitempty"`
    List []string `json:"list,omitempty"`
}

type TrialConfig struct {
    NumTrials       *int32               `json:"numTrials,omitempty"`
    ParallelTrials  *int32               `json:"parallelTrials,omitempty"`
    MaxFailedTrials *int32               `json:"maxFailedTrials,omitempty"`
    Storage         *OptimizationStorage `json:"storage,omitempty"`
}

type OptimizationStorage struct {
    StorageUri *string `json:"storageUri,omitempty"`
    PvcName    *string `json:"pvcName,omitempty"`
}

type TrainJobTemplateSpec struct {
    // Standard object's metadata. System fields are blocked via CEL validation.
    // +optional
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // Specification of the desired behavior of the TrainJob.
    // Users place placeholders like {{.parameter_name}} anywhere in this spec.
    Spec TrainJobSpec `json:"spec"`
}

type OptimizationJobStatus struct {
    Phase      string             `json:"phase,omitempty"`
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    Active    int32 `json:"active,omitempty"`
    Suspended int32 `json:"suspended,omitempty"`
    Succeeded int32 `json:"succeeded,omitempty"`
    Failed    int32 `json:"failed,omitempty"`

    // BestTrial caches the highest performing trial based on the Objective.
    BestTrial *BestTrial `json:"bestTrial,omitempty"`
}

type BestTrial struct {
    Name       string                `json:"name"`
    Value      string                `json:"value"`
    Parameters []ParameterAssignment `json:"parameters,omitempty"`
}

type ParameterAssignment struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}
```

## 5. Sample YAML (Phase 1)

The `TrainJobTemplate` utilizes a structured approach. Hyperparameters are injected natively using text placeholders (`{{.parameter_name}}`). The Validating Admission Webhook ensures that any parameter declared in `spec.parameters` actually exists as a placeholder inside the template prior to admitting the resource.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: OptimizationJob
metadata:
  name: tune-bert
spec:
  objectives:
    - metric: accuracy
      direction: maximize
  algorithm:
    name: random
    provider: optuna
  parameters:
    - name: learning_rate
      searchSpace:
        type: double
        min: "0.001"
        max: "0.1"
    - name: batch_size
      searchSpace:
        type: categorical
        list: ["16", "32", "64"]
  trialConfig:
    numTrials: 10
    parallelTrials: 2
  trainJobTemplate:
    metadata:
      labels:
        hpo-experiment: tune-bert
    spec:
      runtimeRef:
        name: pytorch-distributed
        apiGroup: trainer.kubeflow.org
        kind: ClusterTrainingRuntime
      trainer:
        # Hyperparameters are injected securely via string templating
        image: docker.io/my-org/bert-tuner:latest
        command:
          - "python"
          - "train.py"
          - "--lr={{.learning_rate}}"
          - "--batch-size={{.batch_size}}"
```

## 6. Reconciliation & Architecture (Phase 1)

### Suggestion Service Integration

To optimize resource utilization and minimize trial startup latency, Phase 1 adopts a split execution strategy for algorithms:

- **Stateless Algorithms (Random, Grid):** The controller executes these in-process.
  - By avoiding the deployment of separate, always-on gRPC pods, we eliminate unnecessary startup latency and cluster overhead.
  - The controller calls internal generation functions directly during the reconciliation loop.
- **(Deferred to Phase 2) Stateful Algorithms:** These will be executed via transient One-Shot Jobs to ensure mathematical state is persisted without requiring always-on resources.

### Controller Flow

The reconciliation loop follows a strictly defined lifecycle to manage trial execution without external database dependencies:

1. **Suggestion Phase:** The controller evaluates current cluster capacity against `trialConfig.parallelTrials` and invokes the in-process Suggestion Service to generate new parameter assignments.
2. **Trial Injection:** The controller constructs `TrainJob` manifests from the provided `TrainJobTemplate`, dynamically resolving hyperparameter values using native string templating (e.g., replacing `{{.learning_rate}}` with `0.01`) before submitting the object to the API server.
3. **Monitoring (No Katib DB):** The controller monitors the `TrainJobStatus` via the Progress API.
  - It relies on `status.trainerStatus` to track real-time success or failure of active trials.
4. **Completion Phase:** Upon trial completion, the controller evaluates the final metrics surfaced in the `TrainJob` status, While the schema natively supports an array of `Objectives` for future extensibility, the Phase 1 controller will evaluate the primary (first) objective to identify the `BestTrial` and update the `OptimizationJobStatus` accordingly.

## 7. Design Decisions & Open Discussions

### 7.1. Decision: Parameter Propagation via String Templating
**Status: Resolved in v1alpha1**
To inject parameters into the training container without forcing users to adopt a custom SDK or injecting heavyweight gRPC sidecars, `OptimizationJob` utilizes native string templating.
* **The Design:** Users write their `TrainJobTemplateSpec` normally but place text placeholders (e.g., `{{.learning_rate}}`) wherever they need the value (CLI args, env vars, labels).
* **The "Why":** Before applying the `TrainJob`, the controller performs an in-memory string replacement. This ensures the pod boots with the exact configuration required, keeps the API schema concise, and requires zero code changes from the user's ML script. A Validating Admission Webhook ensures that all declared parameters exist as placeholders in the template prior to admitting the resource.

### 7.2. Decision: Deprecating the Trial CRD
**Status: Resolved in v1alpha1**
In legacy Katib, the `Trial` CRD was necessary because worker jobs lacked native observability; the Trial acted as an intermediary metrics scraper. With the new unified `TrainJob` API exposing `TrainerStatus.Metrics` directly, this adapter layer is obsolete.
The `OptimizationJob` controller bypasses the `Trial` CRD entirely, creating `TrainJobs` directly and watching their status for convergence. This eliminates an entire controller reconciliation loop and keeps the hierarchy clean (`OptimizationJob` -> `TrainJob`).

### 7.3. Open Discussion: Handling Dynamic Algorithms (Ray Tune, PBT, Hyperband)
Population-Based Training (PBT) and Hyperband algorithms are stateful and dynamic. Frameworks like Ray Tune handle this by directly mutating Python worker memory mid-flight. Because Kubernetes pod specifications are immutable once running, we evaluated two paths for OptimizationJob:

* **Approach 1: The Kubernetes-Native Path (Suspend & Patch) [Recommended]**
    Leverage native K8s job suspension. When a trial hits a bracket, the controller flips `Suspend: true` on the `TrainJob`. The pods spin down, and the ML framework checkpoints to persistent storage. The controller appends a `RuntimePatch` to overwrite the hyperparameters, then flips `Suspend: false`. The pods reschedule, load the new parameters/checkpoint, and resume.
    *Tradeoffs:* Introduces scheduling latency, but keeps cluster state completely declarative and requires no custom user SDKs. We have introduced a `Suspended` counter in `OptimizationJobStatus` to support this logic safely.
* **Approach 2: The Zero-Restart Path (gRPC Sidecar)**
    Inject a lightweight gRPC sidecar into the `TrainJob` pod. The controller sends parameter updates to the sidecar, and the user's Python script polls a local socket to update its memory.
    *Tradeoffs:* Extremely fast, but breaks the declarative truth of K8s (Pod spec won't match running memory state) and creates high integration friction by forcing users to rewrite their ML code.

### 7.4. Open Discussion: Pluggable Suggestion Architecture
If a user requests "Bayesian Optimization," we cannot hardcode the execution to a single backend library (e.g., Optuna uses TPE, while Google OSS Vizier uses gRPC-native scaling).
To avoid vendor lock-in, Phase 2 will treat mathematical execution as an external dependency. We have introduced a `Provider` field into the `Algorithm` struct. The `OptimizationJob` controller will act strictly as a router, maintaining a standard gRPC contract with backend suggestion engines deployed as independent microservices. The controller will read `TrainJob` metrics, check the `Provider`, send the history to the respective microservice, and receive the next hyperparameters to apply.
