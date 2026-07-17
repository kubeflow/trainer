# KEP-3562: OptimizationJob CRD for Hyperparameter Optimization

- **Authors:** Aniket Shaha (@aniket2405)

---

## Index

1. [Background & Motivation](#1-background--motivation)
2. [User Stories](#2-user-stories)
3. [Goals by Phase](#3-goals-by-phase)
4. [Non-Goals](#4-non-goals)
5. [Phase 1 API Design (v1alpha1)](#5-phase-1-api-design-v1alpha1)
6. [Sample YAML (Phase 1)](#6-sample-yaml-phase-1)
7. [Reconciliation & Architecture (Phase 1)](#7-reconciliation--architecture-phase-1)
8. [Open Discussions](#8-open-discussions)
9. [Implementation History](#9-implementation-history)
10. [Alternatives](#10-alternatives)

---

## 1. Background & Motivation

Historically, Katib has served as Kubeflow’s general-purpose hyperparameter tuning and Neural Architecture Search (NAS) engine. It uses the generic `Experiment` CRD to orchestrate trials, supporting arbitrary Kubernetes workloads via unstructured YAML templates. 

While highly flexible, its broad scope creates friction for standard ML workflows. It forces users to write verbose YAML and relies on brittle regex string substitution (e.g., `${searchSpace.lr}`) to inject parameters. With the introduction of the unified Kubeflow Python SDK [KEP-46](https://github.com/kramaranya/sdk/blob/a8d248d13019d9bab0af047770b9bf8e81ed7358/docs/proposals/46-hyperparameter-optimization/README.md), there is a strong need for a strongly-typed, iterative orchestration layer that integrates natively with `TrainJobs` and relies on push-based metrics.

## 2. User Stories

**Story 1: The ML Engineer (Simplified Orchestration)**
* **As an ML Engineer**, I want to define my hyperparameter tuning configurations directly alongside my `TrainJob` template.
* **Motivation:** To avoid managing two separate, loosely-coupled CRDs (Experiment and Trial) and ensure my training infrastructure and tuning parameters are version-controlled in a single file.

**Example input (OptimizationJob):**
```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: OptimizationJob
metadata:
  name: tuning-job
spec:
  searchAlgorithm:
    random:
      randomState: 42
  parameters:
    - name: "lr"
      searchSpace:
        uniform:
          min: "0.01"
          max: "0.1"
  # ... trial config ...
  trainJobTemplate:
    spec:
	    runtimeRef:
        apiGroup: trainer.kubeflow.org
        kind: ClusterTrainingRuntime
        name: torch-distributed

      trainer:
        image: docker.io/my-org/model:latest
```

**Story 2: The Data Scientist (Immediate Observability)**
* **As a Data Scientist**, I want to see the "best trial" results directly in the `OptimizationJob` status.
* **Motivation:** To avoid executing manual `kubectl` queries across dozens of individual pods to figure out which combination of learning rate and batch size actually performed the best.

**Story 3: The Platform Operator (Stateless Infrastructure)**
* **As a Platform Operator**, I want the HPO orchestration service to be stateless and avoid deploying dedicated sidecars or persistent databases.
* **Motivation:** To eliminate the heavy cluster resource overhead required by legacy sidecar models and reduce the operational complexity of maintaining a persistent storage layer strictly for HPO experiments.

**Story 4: The ML Researcher (Native SDK Integration)**
* **As an ML Researcher**, I want to consume hyperparameter suggestions via standard environment variables rather than brittle YAML regex string substitution.
* **Motivation:** Using the `KUBEFLOW_TRAINER_OPT_<NAME>` pattern allows me to cleanly parse tuning suggestions inside my Python scripts using existing SDK helper functions without modifying my container's CLI argument parsing logic. [Separate KEP for this integration].

## 3. Goals

- **Tighter TrainJob Integration:** Introduce the `OptimizationJob` CRD focused exclusively on `TrainJobs`, using a structured `TrainJobTemplateSpec`.
- **Native Parameter Injection:** Replace legacy brittle regex YAML substitution with standard Kubernetes mechanisms: prefixed environment variables (e.g., `KUBEFLOW_TRAINER_OPT_LR`) and Pod annotations.
- **Dependency Reduction (No Katib DB or Trial CRD):** Rely strictly on the `TrainJob` annotations for historical parameters and the Progress API for evaluating metrics.
- **Comprehensive Search Space API**: Define a robust API capable of supporting various parameter distributions to accurately model hyperparameter search spaces.
- **Single Canonical Provider (Optuna MVP):** Hard-scope the backend suggestion engine to Optuna to stabilize the orchestration loop before multi-tenant provider support is added in future interations.
- **Stateless Algorithm Execution**: Compute suggestions dynamically using a stateless gRPC provider model that reconstructs trial history directly from the Kubernetes API rather than relying on a Katib DB.
- **Native CEL Validation**: Replace legacy validating webhooks with native Kubernetes Common Expression Language (CEL) rule.

- **Phase 2 / Phase 3 Goals**:
  - Kubeflow SDK integrations
  - Advanced Search Spaces: Normal and LogNormal distributions
  - Refactor Katib gRPC service
  - Refactor Optuna algorithm service in Trainer repository
  - Secure Auth for gRPC service
  - Support for Multi-Objective Optimization
  - Integration w/ Kueue with `suspend` and `managedBy` APIs in OptimizationJob

## 4. Non-Goals / Future Iterations
To ensure a stable and reviewable initial release (Phase 1), the following features are explicitly out of scope for now and will be addressed in future iterations:

**Advanced/Custom Algorithms (Phase 2):**
- Custom algorithms, TPE (Tree-structured Parzen Estimator), Gaussian Process (GP) Bayesian optimization, and advanced trial pruning algorithms (e.g., ASHA, Hyperband) are deferred. Phase 1 supports only Random and Grid search.

**State & Storage (Phase 2):**
- **Trial Suspension & Storage Checkpointing:** `OptimizationStorage` and `status.Suspended` to allow pausing/resuming trials mid-flight.
- **Stateful Algorithms & Shared Initialization:** One-Shot Jobs for algorithms that persist mathematical state, and the `SharedInitializer` plugin.

**Deprecated Katib Features (Not Supported):**
- **Neural Architecture Search (NAS):** Requires a fundamentally different search space model.
- **Arbitrary CRD Support:** Dropped to enforce `TrainJob` stability.
- **Pull-Based Metrics:** Legacy sidecar metric collectors (Prometheus, stdout parsers) are omitted.
- Pause and Resume Experiments.
- Support for complex metric strategies.
- Support for multiple providers for the same algorithm.
- Integration with the legacy Katib UI.

## 5. Phase 1 API Design (v1alpha1)

The MVP API surface is strongly typed to ensure native API server validation via OpenAPI schemas and CEL rules. Mathematical parameters like standard deviations and interval boundaries utilize `string` types to prevent float precision rounding, protected by K8s CEL type-casting.

```go
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:validation:Enum=Maximize;Minimize
type ObjectiveDirection string

const (
    ObjectiveDirectionMaximize ObjectiveDirection = "Maximize"
    ObjectiveDirectionMinimize ObjectiveDirection = "Minimize"
)

type Objective struct {
	// Metric specifies the name of the objective metric to track. Defaults to "loss".
  // +kubebuilder:default=loss
  // +kubebuilder:validation:MinLength=1
  // +kubebuilder:validation:MaxLength=64
  // +optional
	Metric *string `json:"metric,omitempty"`

	// Direction specifies the optimization goal. Defaults to "Minimize".
	// +kubebuilder:default=Minimize
	// +optional
	Direction *ObjectiveDirection `json:"direction,omitempty"`
}

// OptimizationJobSpec defines the desired state of OptimizationJob.
type OptimizationJobSpec struct {
	// +listType=map
  // +listMapKey=metric
  // +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1
  // +required
	Objectives []Objective `json:"objectives"`

  // +optional
	SearchAlgorithm *SearchAlgorithm `json:"searchAlgorithm,omitempty"`

	// +listType=map
  // +listMapKey=name
  // +kubebuilder:validation:MinItems=1
  // +kubebuilder:validation:MaxItems=1
  // +required
	Parameters []Parameter `json:"parameters"`

  // +optional
	TrialPolicy *TrialPolicy `json:"trialPolicy,omitempty"`

  // +required
	TrainJobTemplate TrainJobTemplateSpec `json:"trainJobTemplate"`
}

// +kubebuilder:validation:XValidation:rule="[has(self.random), has(self.grid)].filter(x, x).size() == 1",message="Exactly one search algorithm configuration must be provided"
type SearchAlgorithm struct {
	// +optional
	Random *RandomAlgorithm `json:"random,omitempty"`
	// +optional
	Grid *GridAlgorithm `json:"grid,omitempty"`
}

type RandomAlgorithm struct {
	// +optional
	Seed *int64 `json:"seed,omitempty"`
}

// GridAlgorithm is intentionally empty; step-intervals are derived from SearchSpace.Int.Step.
type GridAlgorithm struct{}

// +kubebuilder:validation:Enum=Int;Float
type ParameterType string

const (
    ParameterTypeInt   ParameterType = "Int"
    ParameterTypeFloat ParameterType = "Float"
)

// SearchSpace acts as a Discriminated Union (OneOf) supporting flexible statistical distributions.
// +kubebuilder:validation:XValidation:rule="[has(self.uniform), has(self.logUniform), has(self.categorical)].filter(x, x).size() == 1",message="Exactly one search space distribution configuration must be provided"
type SearchSpace struct {
	// +optional
	Uniform *UniformSpace `json:"uniform,omitempty"`

	// +optional
	LogUniform *LogUniformSpace `json:"logUniform,omitempty"`

	// +optional
	Categorical *CategoricalSpace `json:"categorical,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.matches('^-?(0|[1-9][0-9]*)(\\.[0-9]+)?([eE][+-]?[0-9]+)?$')",message="value must be a valid numeric value"
// +kubebuilder:validation:MaxLength=64
type Double string

// UniformSpace defines a continuous uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type UniformSpace struct {
  // +required
	Min Double `json:"min"`

  // +required
	Max Double `json:"max"`

	// Type specifies the underlying data type. Defaults to "Float".
  // +kubebuilder:default=Float
  // +required
  Type ParameterType `json:"type"`
}

// LogUniformSpace defines a continuous log-uniform distribution over [Min, Max].
// +kubebuilder:validation:XValidation:rule="double(self.min) > 0.0",message="min must be strictly greater than 0"
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type LogUniformSpace struct {
  // +required
	Min Double `json:"min"`

  // +required
	Max Double `json:"max"`

	// Type specifies the underlying data type. Defaults to "Float".
	// +kubebuilder:default=Float
  // +required
	Type ParameterType `json:"type"`
}

// CategoricalSpace defines a search space over a discrete set of unordered strings.
type CategoricalSpace struct {
	// Choices is the set of strings to sample from.
  // +kubebuilder:validation:MinItems=1
  // +kubebuilder:validation:MaxItems=1
  // +listType=set
  // +required
	Choices []string `json:"choices"`
}

type Parameter struct {
  // Name is the name of the hyperparameter.
	// +kubebuilder:validation:MinLength=1
  // +kubebuilder:validation:MaxLength=64
  // +required
	Name        string      `json:"name"`

  // +required
	SearchSpace SearchSpace `json:"searchSpace"`
}

// ParameterAssignment represents a single hyperparameter and its assigned value.
type ParameterAssignment struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Name string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +required
	Value string `json:"value"`
}

// +kubebuilder:validation:XValidation:rule="self.parallelTrials <= self.numTrials",message="parallelTrials cannot exceed numTrials"
type TrialPolicy struct {
	// +kubebuilder:validation:Minimum=1
  // +optional
	NumTrials *int32 `json:"numTrials,omitempty"`

  // +kubebuilder:default=1
  // +kubebuilder:validation:Minimum=1
  // +kubebuilder:validation:Maximum=100
  // +optional
	ParallelTrials int32 `json:"parallelTrials"`
}

type TrainJobTemplateSpec struct {
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.name) && !has(self.namespace)", message="name and namespace cannot be set in a template."
	metav1.ObjectMeta `json:"metadata,omitempty"`

  // +required
	Spec TrainJobSpec `json:"spec,omitzero"`
}

type OptimizationJobStatus struct {
	// +listType=map
	// +listMapKey=type
  // +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

  // +optional
	Result *Result `json:"result,omitempty"`
}

// Result tracks the parameters of the highest performing trial.
type Result struct {
	// TrainJobName is the name of the underlying TrainJob that achieved this result.
  // +kubebuilder:validation:MinLength=1
  // +required
  TrainJobName string `json:"trainJobName"`
	// +listType=map
	// +listMapKey=name
  // +kubebuilder:validation:MaxItems=100
	// +optional
	Parameters []ParameterAssignment `json:"parameters,omitempty"`
}
```

## 6. Sample YAML (Phase 1)

The `TrainJobTemplate` utilizes a structured approach. Hyperparameters are dynamically injected by the controller directly into the Pod as prefixed environment variables (e.g., `KUBEFLOW_TRAINER_OPT_<PARAM_NAME>`) and appended as annotations on the `TrainJob` metadata.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: OptimizationJob
metadata:
  name: bayesian-tuning-mvp
spec:
  objectives:
    - metric: "val_loss"
      direction: "minimize"

  searchAlgorithm:
    bayesian:
      initialTrials: 10
      acquisitionFunction: "ei"

  parameters:
    - name: "learning_rate"
      searchSpace:
        logUniform:
          min: "0.0001"
          max: "0.1"
    - name: "batch_size"
      searchSpace:
        categorical:
          choices: ["16", "32", "64"]

  trialPolicy:
    numTrials: 20
    parallelTrials: 4

  trainJobTemplate:
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
        # The ML script reads KUBEFLOW_TRAINER_OPT_LEARNING_RATE and KUBEFLOW_TRAINER_OPT_BATCH_SIZE 
        # either manually or via the Kubeflow Python SDK helper functions.

status:
  conditions:
    - type: "Complete"
      status: "True"
      reason: "MaxTrialsReached"
  result:
    trainJobName: "bayesian-tuning-mvp-trial-ab12c"
    parameters:
      - name: "learning_rate"
        value: "0.0021"
      - name: "batch_size"
        value: "32"
```

## 7. Reconciliation & Architecture (Phase 1)

### 7.1 Prerequisites

* TrainJob Feature Flag (Hard Dependency): The unified `TrainJob` API feature flag MUST be enabled in the cluster/controller environment.

### 7.2. gRPC API Strategy & Adapter Pattern

To accelerate the MVP and reduce risk, the evolution of the gRPC contract between the Go controller and the Python suggestion engines is divided into two phases:

**Phase 1: Legacy API Adapter (Initial Release)**
For the initial v1alpha1 release, we will use the **existing Katib gRPC API design** (`api.v1.beta1`). 
* The controller will act as a translation adapter. It will map the new, strictly typed `OptimizationJob` structs (e.g., `SearchSpace`, `RandomAlgorithm`) into the legacy `Experiment` and `Trial` protobuf messages.
* This allows us to natively reuse the existing, Python suggestion images (e.g., `ghcr.io/kubeflow/katib/suggestion-optuna:latest`) without requiring any immediate modifications to the Python microservices.
* The controller remains stateless: it reconstructs the trial history by reading `TrainJob` annotations and passes the full history via the `GetSuggestionsRequest` on demand.

**Phase 2: gRPC Contract Refactoring (Post-Release)**
After the core orchestration loop is stabilized in the first release, the gRPC contract will be refactored. The legacy `Experiment` protobuf dependency will be removed. The KEP will be updated at that time to align with the new structure.

### 7.3 The Suggestion Service Architecture

**Legacy Statefulness (Katib Today)**
Katib currently operates on a 1-to-1 mapping where every `Experiment` triggers a dedicated, stateful `Suggestion` sidecar. This model forces each experiment to maintain a local database connection and internal state, creating significant resource overhead and operational complexity for sidecar lifecycle management.

**The Stateless Evolution (OptimizationJob):**
Our model evolves this architecture into a stateless, provider-agnostic system:

**Deployment Pattern**
For Phase 1, we maintain isolation by deploying one dedicated `Suggestion` service container per `OptimizationJob`. This pod runs continuously for the duration of the job, but holds no persistent state or database volume.

**Stateless Orchestration**
Unlike Katib, our controller treats the service as an ephemeral provider. The controller orchestrates the experiment by gathering history from completed `TrainJob` annotations and passing this full, point-in-time snapshot to the `GetSuggestions` gRPC method.

**Independence**
The Provider calculates the next parameters and returns them, "forgetting" the interaction immediately. This keeps mathematical execution stateless and entirely independent of the Kubernetes cluster state, removing the need for a persistent database or stateful sidecars.

### 7.4. State Transition & Conditions

In this section, we define the `OptimizationJob` state transition (`.status.conditions`). The basic `OptimizationJob` state machine tracks the lifecycle of the hyperparameter tuning experiment. The terminal condition (`Failed` or `Complete`) is decided based on the aggregate status of the underlying `TrainJob` objects and the rules defined in the `TrialPolicy`.

In the state transition, a `Created=False` condition will occur in the following situations, identified by the condition reasons (`.status.conditions.[type="Created"].reason`):
* **AlgorithmServiceCreationFailed**: When the controller fails to construct or deploy the gRPC provider service required for generating hyperparameters.
* **TrainJobsCreationFailed**: When the controller successfully generates suggestions but fails to deploy the resulting `TrainJob` objects to the cluster.

The core successful conditions for the Phase 1 MVP are:
* **Created**: The `OptimizationJob` has been accepted, the suggestion service is running, and the controller is actively provisioning trial `TrainJobs`.
* **Complete**: The conditions of the `TrialPolicy` have been satisfied (e.g., the desired `NumTrials` have successfully finished) and the best result has been recorded.
* **Failed**: The `OptimizationJob` encountered a terminal error preventing further execution (e.g., the backend suggestion gRPC service crashed).

## 8. Design Decisions & Open Discussions

### 8.1. Decision: Decoupling the gRPC Contract
**Status: Deferred to Phase 2**
Initially, we considered creating a new, provider-agnostic gRPC protobuf schema for Phase 1 to prevent the schema from bloating. However, to ensure a faster and more stable initial release, we have decided to use the existing Katib `api.v1.beta1` protobufs via an adapter pattern in the Go controller. Once the first release is complete, this decision will be revisited, and the gRPC contract will be decoupled and refactored.

### 8.2. Decision: Parameter Propagation via Environment Variables & Annotations
**Status: Resolved in v1alpha1**
We have deprecated string templating (`{{.param}}`). To pass parameters to the training container reliably, `OptimizationJob` leverages native Kubernetes downward API mechanisms:

* **The Design:** The controller injects `KUBEFLOW_TRAINER_OPT_<PARAM_NAME>` as environment variables directly into the `trainJob.spec.trainer.env` array. It simultaneously stores the raw JSON parameter assignment as an Annotation on the TrainJob metadata.
* **The "Why":** This aligns well with the unified Kubeflow Python SDK (KEP-46). Data scientists can use SDK helper functions (e.g., `get_hyperparameters()`) to cleanly parse the environment variables inside their training scripts without modifying YAML command arguments. The metadata annotations allow the controller to reconstruct trial history purely from the Kubernetes API without requiring Katib DB.

### 8.3. Decision: Explicit Separation of Search vs. Pruning
**Status: Resolved (Phase 2 Roadmap)**

We explicitly rename the core API block to `searchAlgorithm` and define a separate, future `pruneAlgorithm` block.
Search algorithms (TPE/BO) and Pruning algorithms (ASHA/Hyperband) represent different mathematical domains—sampling vs. evaluation. Separate API blocks allow us to evolve these domains independently without polluting the schema with heterogeneous parameters.

### 8.4. Decision: Deprecating the Trial CRD
**Status: Resolved in v1alpha1**
With the new unified TrainJob API exposing metrics directly, the `OptimizationJob` controller bypasses the Trial CRD entirely. The `OptimizationJob` directly creates TrainJobs and reconstructs historical state by reading their labels and annotations.

### 8.5. Decision: Search Space Concrete Types (OneOf Pattern)
**Status: Resolved in v1alpha1**
Instead of employing a single flat struct with a generic type string, the `SearchSpace` utilizes a discriminated union. This establishes strong typing at the Kubernetes API layer, permitting mathematical CEL validations (`double()`, `int()`) and the easy addition of future mathematical domains without heavy Webhook validation logic.

### 8.6. Open Discussion: Decoupling Metric Reporting from Termination Logic
**Status: Resolved (Phase 2 Roadmap)**
Metric reporting from the `TrainJob` is strictly asynchronous and relies entirely on the **TrainJobStatus** feature gate. As defined in [KEP-2779](https://github.com/kubeflow/trainer/tree/master/proposals/2779-trainjob-progress), the Optimization controller will consume metrics directly from the standardized `TrainJob` status fields rather than deploying custom sidecars. 

Pruning decisions are computed controller-side based on this monotonic metric history. A "Stop Signal" is then propagated to the training runtime as a non-blocking annotation or status field, which the Kubeflow SDK periodically polls. Synchronous "kill" calls during metric reporting create tight coupling and latency bottlenecks; by separating reporting from termination, we ensure the controller remains performant under heavy trial loads.

## Implementation History

* **2026-06-01:** Initial KEP draft creation for the `OptimizationJob` CRD.

## 10. Alternatives

### Extend Existing Katib Experiment and Trial CRDs
Instead of introducing the `OptimizationJob` CRD, we could have updated the existing Katib `Experiment` and `Trial` CRDs to support `TrainJob` references.
Katib's current architecture is fundamentally built around unstructured YAML templates and arbitrary CRD support, relying heavily on brittle regex string substitution (e.g., `${searchSpace.lr}`). Fitting this legacy structure to support the strictly typed `TrainJob` v2 API would require massive breaking changes to Katib or result in a disjointed user experience. Introducing a purpose-built `OptimizationJob` ensures tight coupling with `TrainJob` and native Kubernetes validation.

### Stateful Sidecars with Persistent Storage
Katib currently deploys a stateful `Suggestion` sidecar and a persistent DB layer for every experiment. We could have replicated this architecture for `OptimizationJob`.
Deploying dedicated databases for every hyperparameter tuning job introduces severe cluster resource bloat and operational complexity. By reconstructing the trial history directly from completed `TrainJob` annotations and passing it statelessly to the gRPC provider, we eliminate the need for persistent storage and sidecar lifecycle management.