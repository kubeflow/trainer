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
- **Native Parameter Injection:** Replace legacy brittle regex YAML substitution with standard Kubernetes mechanisms: prefixed environment variables (e.g., `KUBEFLOW_OPT_LR`) and Pod annotations, allowing the SDK to easily parse configurations.
- **Dependency Reduction (No Katib DB or Trial CRD):** Rely strictly on the `TrainJob` annotations for historical parameters and the Progress API (via `status.trainerStatus`) for evaluating objective metrics.
- **Concrete Type Architecture (OneOf)**: Implement a strongly-typed discriminated union pattern (`IntSpace`, `DoubleSpace`, `CategoricalSpace`) for the SearchSpace, simplifying API validation and ensuring extensibility.
- **Stateless Suggestion Services**: Transition from Katib's 1-to-1 stateful sidecar model to a shared, stateless gRPC provider model where the controller passes the full trial history on demand.
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

type SearchSpace struct {
    Int *IntSpace `json:"int,omitempty"`
    Double *DoubleSpace `json:"double,omitempty"`
    Categorical *CategoricalSpace `json:"categorical,omitempty"`
}

type IntSpace struct {
    Min string `json:"min"`
    Max string `json:"max"`
    Step *string `json:"step,omitempty"`
}

type DoubleSpace struct {
    Min string `json:"min"`
    Max string `json:"max"`
    Scale *string `json:"scale,omitempty"`
}

type CategoricalSpace struct {
    List []string `json:"list"`
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
        double:
          min: "0.001"
          max: "0.1"
          scale: "log"
    - name: batch_size
      searchSpace:
        categorical:
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
        image: docker.io/my-org/bert-tuner:latest
        command:
          - "python"
          - "train.py"
        # The ML script reads KUBEFLOW_OPT_LEARNING_RATE and KUBEFLOW_OPT_BATCH_SIZE 
        # either manually or via the Kubeflow Python SDK helper functions.
```

## 6. Reconciliation & Architecture (Phase 1)

### Suggestion Service Integration

To eliminate the massive cluster resource overhead and startup latency of Katib's legacy 1-to-1 sidecar model, `OptimizationJob` utilizes a Stateless Shared Provider architecture via gRPC.

- Providers like Optuna or Vizier are pre-deployed as long-running, shared microservices in the cluster.

- The OptimizationJob controller acts as the orchestrator. When evaluating a new trial, the controller gathers the history of completed TrainJobs by reading their annotations and final metrics, packages this history, and sends a single stateless gRPC request to the Provider.

- The Provider calculates the next parameters, returns them, and forgets the interaction, keeping mathematical execution stateless and independent of Kubernetes state.

### Controller Flow

The reconciliation loop follows a strictly defined lifecycle to manage trial execution without external database dependencies:

1. **State Gathering:** The controller queries existing TrainJobs via label selectors, reconstructing the trial history using `TrainJob` annotations (for parameters) and the `TrainerStatus` (for metrics).
2. **Suggestion Phase:** The controller invokes the Suggestion Service (either via gRPC or in-process for Random/Grid) passing the history and the count of required parameters.
3. **Trial Injection:** The controller dynamically injects the generated hyperparameters into the `TrainJob` template as prefixed environment variables (e.g., `KUBEFLOW_OPT_<NAME>`) and appends a tracking annotation before creating the `TrainJob`.
4. **Monitoring (No Katib DB):** The controller relies strictly on the `TrainJob` status to track success/failure.
5. **Completion Phase:** Upon trial completion, the `BestTrial` is evaluated and cached in `OptimizationJobStatus`. No database or Trial CRD is required.

## 7. Design Decisions & Open Discussions

### 7.1. Decision: Parameter Propagation via Environment Variables & Annotations
**Status: Resolved in v1alpha1**
We have deprecated string templating ({{.param}}). To pass parameters to the training container reliably, `OptimizationJob` leverages native Kubernetes downward API mechanisms:
* **The Design:** The controller injects `KUBEFLOW_OPT_<PARAM_NAME>` as environment variables into the Pod. It simultaneously stores the raw JSON parameter assignment as an Annotation on the TrainJob metadata.
* **The "Why":** This aligns perfectly with the unified Kubeflow Python SDK (KEP-46). Data scientists can use SDK helper functions (e.g., `get_hyperparameters()`) to cleanly parse the environment variables inside their training scripts without modifying YAML command arguments. The metadata annotations allow the controller to reconstruct trial history purely from the Kubernetes API without requiring Katib DB.

### 7.2. Decision: Deprecating the Trial CRD
**Status: Resolved in v1alpha1**
With the new unified TrainJob API exposing metrics directly, the `OptimizationJob` controller bypasses the Trial CRD entirely. The `OptimizationJob` directly creates TrainJobs and reconstructs historical state by reading their labels and annotations.

### 7.3. Decision: Search Space Concrete Types (OneOf Pattern)
**Status: Resolved in v1alpha1**
Instead of employing a single flat struct with a generic `type` string, the `SearchSpace` utilizes a discriminated union (Concrete Sub-types like `IntSpace` or `DoubleSpace`). This establishes strong typing at the Kubernetes API layer, eliminating the need for heavy custom validating webhooks and permitting the easy addition of future mathematical domains.

### 7.4. Open Discussion: Handling Dynamic Algorithms (Ray Tune, PBT, Hyperband)
Because K8s pod environment variables and template specs are strictly immutable once created, we cannot "pause, mutate, and resume" a single `TrainJob` for stateful algorithms like PBT.
To handle mid-flight hyperparameter mutation safely within Kubernetes, we evaluate the following patterns for Phase 2:

* **Approach 1: The Kubernetes-Native Path (Checkpoint & Recreate) [Recommended]**
    When a trial hits a bracket, the `TrainJob` saves a checkpoint to a persistent volume (PVC) and completes. The controller evaluates the population, mutates the parameters, and spins up a  new TrainJob. This new job receives the mutated environment variables and an extra injected RESTORE_PATH variable pointing to the previous checkpoint, resuming the trial.
    *Tradeoffs:* Introduces some scheduling latency (image pulls, pod scheduling)
