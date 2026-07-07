# KEP: OptimizationJob CRD for Hyperparameter Optimization

- **Authors:** Aniket Shaha (@aniket2405)
- **Mentors:** @akshaychitneni, @andreyvelich
- **Target Issue:** kubeflow/katib#2605

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

---

## 1. Background & Motivation

Historically, Katib has served as Kubeflow’s general-purpose hyperparameter tuning and Neural Architecture Search (NAS) engine. It uses the generic `Experiment` CRD to orchestrate trials, supporting arbitrary Kubernetes workloads via unstructured YAML templates. 

While highly flexible, its broad scope creates friction for standard ML workflows. It forces users to write verbose YAML and relies on brittle regex string substitution (e.g., `${searchSpace.lr}`) to inject parameters. With the introduction of the unified Kubeflow Python SDK (KEP-46), there is a strong need for a strongly-typed, iterative orchestration layer that integrates natively with `TrainJobs` and relies on push-based metrics.

## 2. User Stories

**Story 1: The ML Engineer (Simplified Orchestration)**
* **As an ML Engineer**, I want to define my hyperparameter tuning configurations directly alongside my `TrainJob` template.
* **Motivation:** To avoid managing two separate, loosely-coupled CRDs (Experiment and Trial) and ensure my training infrastructure and tuning parameters are version-controlled in a single file.

**Story 2: The Data Scientist (Immediate Observability)**
* **As a Data Scientist**, I want to see the "best trial" results directly in the `OptimizationJob` status.
* **Motivation:** To avoid executing manual `kubectl` queries across dozens of individual pods to figure out which combination of learning rate and batch size actually performed the best.

**Story 3: The Platform Operator (Stateless Infrastructure)**
* **As a Platform Operator**, I want the HPO orchestration service to be stateless and avoid deploying dedicated sidecars or persistent databases.
* **Motivation:** To eliminate the heavy cluster resource overhead required by legacy sidecar models and reduce the operational complexity of maintaining a persistent storage layer strictly for HPO experiments.

**Story 4: The ML Researcher (Native SDK Integration)**
* **As an ML Researcher**, I want to consume hyperparameter suggestions via standard environment variables rather than brittle YAML regex string substitution.
* **Motivation:** Using the `KUBEFLOW_OPT_<NAME>` pattern allows me to cleanly parse tuning suggestions inside my Python scripts using existing SDK helper functions without modifying my container's CLI argument parsing logic.

## 3. Goals by Phase

To ensure a stable and reviewable implementation, the project is broken down into strict phases to manage scope.

### Phase 1: Core Orchestration (v1alpha1)

- **TrainJob Feature Flag** (Hard Dependency): The unified TrainJob feature flag MUST be enabled in the cluster/controller environment. The OptimizationJob orchestrator relies entirely on this API and will not function without it.
- **Tighter TrainJob Integration:** Introduce the `OptimizationJob` CRD focused exclusively on `TrainJobs`, using a structured `TrainJobTemplateSpec` to enable native Kubernetes API validation while allowing user-defined metadata.
- **Native Parameter Injection:** Replace legacy brittle regex YAML substitution with standard Kubernetes mechanisms: prefixed environment variables (e.g., `KUBEFLOW_OPT_LR`) and Pod annotations, allowing the SDK to easily parse configurations.
- **Dependency Reduction (No Katib DB or Trial CRD):** Rely strictly on the `TrainJob` annotations for historical parameters and the Progress API (via `status.trainerStatus`) for evaluating objective metrics.
- **Concrete Type Architecture (OneOf)**: Implement a strongly-typed discriminated union pattern (e.g., `LogUniformSpace`, `TPEAlgorithm`) to simplify API validation and ensure canonical parameter definitions.
- **Single Canonical Provider (Optuna MVP):** Hard-scope the Phase 1 backend suggestion engine to Optuna to stabilize the orchestration loop before multi-tenant provider support is added. 
- **Stateless Suggestion Services**: Transition from Katib's 1-to-1 stateful sidecar model to a shared, stateless gRPC provider model where the controller passes the full trial history on demand.
- **Native CEL Validation**: Replace legacy validating webhooks with native Kubernetes Common Expression Language (CEL) rules to enforce mathematical domain constraints directly at the API server level.

### Phase 2: Stateful & Advanced Integrations

- **Advanced Pruning & Early Stopping:** Implement a separate `PruneAlgorithm` API block. This system will utilize the decoupled metric-reporting pipeline: the controller will run `should_prune()` logic asynchronously on accumulated history, and termination signals will be propagated to the `TrainJob` via the `KubeflowCallback` runtime integration.
- **Trial Suspension & Storage Checkpointing:** Introduce `OptimizationStorage` and `status.Suspended` to allow pausing and resuming trials mid-flight, pending integration with Early Stopping and Kueue.
- **Stateful Algorithms & Shared Initialization:** Implement One-Shot Jobs for Bayesian/TPE to persist mathematical state, and integrate the `SharedInitializer` plugin to share datasets across trials.

## 4. Non-Goals

- **Neural Architecture Search (NAS):** NAS requires a fundamentally different, graph-structured search space model and is out of scope.
- **Arbitrary CRD Support:** Supporting arbitrary K8s Custom Resources (e.g., standard K8s Jobs) is dropped to enforce `TrainJob` stability.
- **Pull-Based Metrics:** Legacy sidecar metric collectors (Prometheus, stdout parsers) are omitted.

## 5. Phase 1 API Design (v1alpha1)

The MVP API surface is strongly typed to ensure native API server validation via OpenAPI schemas and CEL rules. Mathematical parameters like standard deviations and interval boundaries utilize `string` types to prevent float precision rounding, protected by K8s CEL type-casting.

```go
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// OptimizationJobSpec defines the desired state of OptimizationJob.
type OptimizationJobSpec struct {
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	Objectives []Objective `json:"objectives"`

	SearchAlgorithm SearchAlgorithm `json:"searchAlgorithm"`

	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Parameters []Parameter `json:"parameters"`

	TrialConfig TrialConfig `json:"trialConfig"`

	TrainJobTemplate TrainJobTemplateSpec `json:"trainJobTemplate"`
}

type Objective struct {
	// +kubebuilder:validation:MinLength=1
	Metric string `json:"metric"`

	// +kubebuilder:validation:Enum=maximize;minimize
	Direction string `json:"direction"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.random) ? 1 : 0) + (has(self.grid) ? 1 : 0) + (has(self.tpe) ? 1 : 0) + (has(self.bayesian) ? 1 : 0) + (has(self.custom) ? 1 : 0) == 1",message="Exactly one search algorithm configuration must be provided"
type SearchAlgorithm struct {
	// Provider specifies the backend suggestion engine. Defaults to "optuna".
	// +optional
	Provider *string `json:"provider,omitempty"`

	// +optional
	Random *RandomAlgorithm `json:"random,omitempty"`
	// +optional
	Grid *GridAlgorithm `json:"grid,omitempty"`
	// +optional
	TPE *TPEAlgorithm `json:"tpe,omitempty"`
	// +optional
	Bayesian *BayesianAlgorithm `json:"bayesian,omitempty"`
	// +optional
	Custom *CustomAlgorithm `json:"custom,omitempty"`

	// ProviderSettings acts as an escape hatch for arbitrary or proprietary engine kwargs.
	// +listType=map
	// +listMapKey=name
	// +optional
	ProviderSettings []SettingKV `json:"providerSettings,omitempty"`
}

type RandomAlgorithm struct {
	// +optional
	Seed *int64 `json:"seed,omitempty"`
}

// GridAlgorithm is intentionally empty; step-intervals are derived from SearchSpace.Int.Step.
type GridAlgorithm struct{}

type TPEAlgorithm struct {
	// +kubebuilder:validation:Minimum=1
	// +optional
	InitialTrials *int32 `json:"initialTrials,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +optional
	EICandidates *int32 `json:"eiCandidates,omitempty"`

	// +optional
	Seed *int64 `json:"seed,omitempty"`
}

type BayesianAlgorithm struct {
	// +kubebuilder:validation:Minimum=1
	// +optional
	InitialTrials *int32 `json:"initialTrials,omitempty"`

	// +kubebuilder:validation:Enum=ucb;ei;pi
	// +optional
	AcquisitionFunction *string `json:"acquisitionFunction,omitempty"`
}

type CustomAlgorithm struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +listType=map
	// +listMapKey=name
	// +optional
	Settings []SettingKV `json:"settings,omitempty"`
}

type SettingKV struct {
	// +kubebuilder:validation:MinLength=1
	Name  string `json:"name"`
	Value string `json:"value"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.uniform) ? 1 : 0) + (has(self.logUniform) ? 1 : 0) + (has(self.normal) ? 1 : 0) + (has(self.logNormal) ? 1 : 0) + (has(self.int) ? 1 : 0) + (has(self.categorical) ? 1 : 0) == 1",message="Exactly one search space distribution configuration must be provided"
type SearchSpace struct {
	// +optional
	Uniform *UniformSpace `json:"uniform,omitempty"`
	// +optional
	LogUniform *LogUniformSpace `json:"logUniform,omitempty"`
	// +optional
	Normal *NormalSpace `json:"normal,omitempty"`
	// +optional
	LogNormal *LogNormalSpace `json:"logNormal,omitempty"`
	// +optional
	Int *IntSpace `json:"int,omitempty"`
	// +optional
	Categorical *CategoricalSpace `json:"categorical,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type UniformSpace struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

// +kubebuilder:validation:XValidation:rule="double(self.min) > 0.0",message="min must be strictly greater than 0 for a log-uniform distribution"
// +kubebuilder:validation:XValidation:rule="double(self.min) < double(self.max)",message="min must be strictly less than max"
type LogUniformSpace struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

// +kubebuilder:validation:XValidation:rule="double(self.stdDev) > 0.0",message="stdDev must be strictly greater than 0"
type NormalSpace struct {
	Mean   string `json:"mean"`
	StdDev string `json:"stdDev"`
}

// +kubebuilder:validation:XValidation:rule="double(self.stdDev) > 0.0",message="stdDev must be strictly greater than 0"
type LogNormalSpace struct {
	Mean   string `json:"mean"`
	StdDev string `json:"stdDev"`
}

// +kubebuilder:validation:XValidation:rule="int(self.min) < int(self.max)",message="min must be strictly less than max"
type IntSpace struct {
	Min string `json:"min"`
	Max string `json:"max"`
	// +optional
	Step *string `json:"step,omitempty"`
}

type CategoricalSpace struct {
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	List []string `json:"list"`
}

type Parameter struct {
	// +kubebuilder:validation:MinLength=1
	Name        string      `json:"name"`
	SearchSpace SearchSpace `json:"searchSpace"`
}

// +kubebuilder:validation:XValidation:rule="!has(self.parallelTrials) || !has(self.numTrials) || self.parallelTrials <= self.numTrials",message="parallelTrials cannot exceed numTrials"
type TrialConfig struct {
	// +kubebuilder:validation:Minimum=1
	NumTrials *int32 `json:"numTrials,omitempty"`

	// +kubebuilder:validation:Minimum=1
	ParallelTrials *int32 `json:"parallelTrials,omitempty"`

	// +kubebuilder:validation:Minimum=0
	MaxFailedTrials *int32 `json:"maxFailedTrials,omitempty"`
}

type TrainJobTemplateSpec struct {
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.name) && !has(self.namespace)", message="name and namespace cannot be set in a template."
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TrainJobSpec `json:"spec"`
}

type OptimizationJobStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`

	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Active int32 `json:"active,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Succeeded int32 `json:"succeeded,omitempty"`

	// +kubebuilder:validation:Minimum=0
	Failed int32 `json:"failed,omitempty"`

	BestTrial *BestTrial `json:"bestTrial,omitempty"`
}

type BestTrial struct {
	Name  string `json:"name"`
	Value string `json:"value"`

	// +listType=atomic
	// +optional
	Parameters []ParameterAssignment `json:"parameters,omitempty"`
}

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
```

## 6. Sample YAML (Phase 1)

The `TrainJobTemplate` utilizes a structured approach. Legacy string templating has been entirely removed. Hyperparameters are dynamically injected by the controller directly into the Pod as prefixed environment variables (e.g., `KUBEFLOW_OPT_<PARAM_NAME>`) and appended as annotations on the `TrainJob` metadata, allowing the Kubeflow Python SDK to parse them natively."

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: OptimizationJob
metadata:
  name: tpe-tuning-mvp
spec:
  objectives:
    - metric: "val_loss"
      direction: "minimize"

  # Strictly typed mathematical intent
  searchAlgorithm:
    provider: "optuna"
    tpe:
      initialTrials: 10
      eiCandidates: 24
    providerSettings:
      - name: "OPTUNA_EXPERIMENTAL_FLAG"
        value: "true"

  # Strictly typed statistical distributions
  parameters:
    - name: "learning_rate"
      searchSpace:
        logUniform:
          min: "0.0001"
          max: "0.1"
    - name: "batch_size"
      searchSpace:
        categorical:
          list: ["16", "32", "64"]

  trialConfig:
    numTrials: 20
    parallelTrials: 4

  trainJobTemplate:
    metadata:
      labels:
        hpo-experiment: tpe-tuning-mvp
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

## 7. Reconciliation & Architecture (Phase 1)

### 7.1. gRPC API Strategy & Adapter Pattern

To accelerate the MVP and reduce risk, the evolution of the gRPC contract between the Go controller and the Python suggestion engines is divided into two phases:

**Phase 1: Legacy API Adapter (Initial Release)**
For the initial v1alpha1 release, we will use the **existing Katib gRPC API design** (`api.v1.beta1`). 
* The controller will act as a translation adapter. It will map the new, strictly typed `OptimizationJob` structs (e.g., `SearchSpace`, `RandomAlgorithm`) into the legacy `Experiment` and `Trial` protobuf messages.
* This allows us to natively reuse the existing, Python suggestion images (e.g., `ghcr.io/kubeflow/katib/suggestion-optuna:latest`) without requiring any immediate modifications to the Python microservices.
* The controller remains stateless: it reconstructs the trial history by reading `TrainJob` annotations and passes the full history via the `GetSuggestionsRequest` on demand.

**Phase 2: gRPC Contract Refactoring (Post-Release)**
After the core orchestration loop is stabilized in the first release, the gRPC contract will be refactored. The legacy `Experiment` protobuf dependency will be removed. The KEP will be updated at that time to align with the new structure.

### 7.2 The Suggestion Service Architecture

**Legacy Statefulness (Katib Today)**
Katib currently operates on a 1-to-1 mapping where every `Experiment` triggers a dedicated, stateful `Suggestion` sidecar. This model forces each experiment to maintain a local database connection and internal state, creating significant resource overhead and operational complexity for sidecar lifecycle management.

**The Stateless Evolution (OptimizationJob):**
Our model evolves this architecture into a stateless, provider-agnostic system:

**Deployment Pattern**
For Phase 1, we maintain isolation by deploying one dedicated `Suggestion` service container per `OptimizationJob`.

**Stateless Orchestration**
Unlike Katib, our controller treats the service as an ephemeral provider. The controller orchestrates the experiment by gathering history from completed `TrainJob` annotations and passing this full, point-in-time snapshot to the `GetSuggestions` gRPC method.

**Independence**
The Provider calculates the next parameters and returns them, "forgetting" the interaction immediately. This keeps mathematical execution stateless and entirely independent of the Kubernetes cluster state, removing the need for a persistent database or stateful sidecars.

## 8. Design Decisions & Open Discussions

### 8.1. Decision: Decoupling the gRPC Contract
**Status: Deferred to Phase 2**
Initially, we considered creating a new, provider-agnostic gRPC protobuf schema for Phase 1 to prevent the schema from bloating. However, to ensure a faster and more stable initial release, we have decided to use the existing Katib `api.v1.beta1` protobufs via an adapter pattern in the Go controller. Once the first release is complete, this decision will be revisited, and the gRPC contract will be decoupled and refactored.

### 8.2. Decision: Parameter Propagation via Environment Variables & Annotations
**Status: Resolved in v1alpha1**
We have deprecated string templating (`{{.param}}`). To pass parameters to the training container reliably, `OptimizationJob` leverages native Kubernetes downward API mechanisms:

* **The Design:** The controller injects `KUBEFLOW_OPT_<PARAM_NAME>` as environment variables into the Pod. It simultaneously stores the raw JSON parameter assignment as an Annotation on the TrainJob metadata.
* **The "Why":** This aligns perfectly with the unified Kubeflow Python SDK (KEP-46). Data scientists can use SDK helper functions (e.g., `get_hyperparameters()`) to cleanly parse the environment variables inside their training scripts without modifying YAML command arguments. The metadata annotations allow the controller to reconstruct trial history purely from the Kubernetes API without requiring Katib DB.

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
**Status: Pending**
Metric reporting from the TrainJob is strictly asynchronous and non-blocking. Pruning decisions are computed controller-side based on the monotonic metric history. A "Stop Signal" is propagated to the training runtime as a non-blocking annotation or status field, which the KubeflowCallback (SDK) periodically polls.

Synchronous "kill" calls during metric reporting create tight coupling and latency bottlenecks. By separating reporting from termination, we ensure the controller remains performant even under heavy trial loads.
