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
- **Concrete Type Architecture (OneOf)**: Implement a strongly-typed discriminated union pattern (e.g., `LogUniformSpace`, `TPEAlgorithm`) to simplify API validation and ensure canonical parameter definitions.
- **Single Canonical Provider (Optuna MVP):** Hard-scope the Phase 1 backend suggestion engine to Optuna to stabilize the orchestration loop before multi-tenant provider support is added. 
- **Stateless Suggestion Services**: Transition from Katib's 1-to-1 stateful sidecar model to a shared, stateless gRPC provider model where the controller passes the full trial history on demand.
- **Native CEL Validation**: Replace legacy validating webhooks with native Kubernetes Common Expression Language (CEL) rules to enforce mathematical domain constraints directly at the API server level.

### Phase 2: Stateful & Advanced Integrations

- **Advanced Pruning & Early Stopping:** Implement a separate `PruneAlgorithm` API block. This system will utilize the decoupled metric-reporting pipeline: the controller will run `should_prune()` logic asynchronously on accumulated history, and termination signals will be propagated to the `TrainJob` via the `KubeflowCallback` runtime integration.
- **Trial Suspension & Storage Checkpointing:** Introduce `OptimizationStorage` and `status.Suspended` to allow pausing and resuming trials mid-flight, pending integration with Early Stopping and Kueue.
- **Stateful Algorithms & Shared Initialization:** Implement One-Shot Jobs for Bayesian/TPE to persist mathematical state, and integrate the `SharedInitializer` plugin to share datasets across trials.

## 3. Non-Goals

- **Neural Architecture Search (NAS):** NAS requires a fundamentally different, graph-structured search space model and is out of scope.
- **Arbitrary CRD Support:** Supporting arbitrary K8s Custom Resources (e.g., standard K8s Jobs) is dropped to enforce `TrainJob` stability.
- **Pull-Based Metrics:** Legacy sidecar metric collectors (Prometheus, stdout parsers) are omitted.

## 4. Phase 1 API Design (v1alpha1)

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

## 5. Sample YAML (Phase 1)

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

## 6. Reconciliation & Architecture (Phase 1)

### Parameter Translation Flow
Algorithm settings differ fundamentally between backend frameworks (e.g., Optuna's `n_startup_trials` vs. Hyperopt's `n_startup_jobs`). To decouple the Go Controller from third-party library syntax while maintaining a clean, vendor-agnostic gRPC contract, parameter mapping will occur across a three-step pipeline:

1. **Kubernetes API (CRD):** Accepts strictly typed, canonical configurations defined by our OpenAPI schema (e.g., `tpe: { initialTrials: 10 }`).
2. **Go Reconciler:** Translates the canonical K8s types into provider-specific keys (e.g., mapping `initialTrials` to Optuna's `n_startup_trials`) and flattens them into a standard string map.
3. **gRPC API:** The `GetSuggestions` Protobuf message schema remains simple and provider-agnostic, utilizing a flat `map<string, string>` to transmit the parameters.
4. **Python Provider:** The backend suggestion microservice (Optuna) receives the flat map and injects it directly into the engine's initialization logic.

### Suggestion Service Integration
To eliminate the massive cluster resource overhead and startup latency of Katib's legacy 1-to-1 sidecar model, `OptimizationJob` utilizes a Stateless Shared Provider architecture via gRPC.

* Providers (like Optuna) are pre-deployed as long-running, shared microservices in the cluster.
* The `OptimizationJob` controller acts as the orchestrator. When evaluating a new trial, the controller gathers the history of completed TrainJobs by reading their annotations and final metrics, packages this history, and sends a single stateless gRPC request to the Provider.
* The Provider calculates the next parameters, returns them, and forgets the interaction, keeping mathematical execution stateless and independent of Kubernetes state.

## 7. Design Decisions & Open Discussions

### 7.1. Decision: Decoupling the gRPC Contract
**Status: Resolved in v1alpha1**
By resolving to push mathematical bounds and types into the Kubernetes Schema, we eliminate the need for the `ValidateAlgorithmSettings` gRPC call used in Katib. Furthermore, by translating the parameters to a flat map inside the Go controller, we prevent the gRPC protobuf schema from bloating into a massive structured file.

### 7.2. Decision: Parameter Propagation via Environment Variables & Annotations
**Status: Resolved in v1alpha1**
We have deprecated string templating (`{{.param}}`). To pass parameters to the training container reliably, `OptimizationJob` leverages native Kubernetes downward API mechanisms:

* **The Design:** The controller injects `KUBEFLOW_OPT_<PARAM_NAME>` as environment variables into the Pod. It simultaneously stores the raw JSON parameter assignment as an Annotation on the TrainJob metadata.
* **The "Why":** This aligns perfectly with the unified Kubeflow Python SDK (KEP-46). Data scientists can use SDK helper functions (e.g., `get_hyperparameters()`) to cleanly parse the environment variables inside their training scripts without modifying YAML command arguments. The metadata annotations allow the controller to reconstruct trial history purely from the Kubernetes API without requiring Katib DB.

### 7.3. Decision: Explicit Separation of Search vs. Pruning
**Status: Resolved (Phase 2 Roadmap)**

We explicitly rename the core API block to `searchAlgorithm` and define a separate, future `pruneAlgorithm` block.
Search algorithms (TPE/BO) and Pruning algorithms (ASHA/Hyperband) represent different mathematical domains—sampling vs. evaluation. Separate API blocks allow us to evolve these domains independently without polluting the schema with heterogeneous parameters.

### 7.4. Decision: Deprecating the Trial CRD
**Status: Resolved in v1alpha1**
With the new unified TrainJob API exposing metrics directly, the `OptimizationJob` controller bypasses the Trial CRD entirely. The `OptimizationJob` directly creates TrainJobs and reconstructs historical state by reading their labels and annotations.

### 7.5. Decision: Search Space Concrete Types (OneOf Pattern)
**Status: Resolved in v1alpha1**
Instead of employing a single flat struct with a generic type string, the `SearchSpace` utilizes a discriminated union. This establishes strong typing at the Kubernetes API layer, permitting mathematical CEL validations (`double()`, `int()`) and the easy addition of future mathematical domains without heavy Webhook validation logic.

### 7.6. Open Discussion: Decoupling Metric Reporting from Termination Logic
**Status: Pending**
Metric reporting from the TrainJob is strictly asynchronous and non-blocking. Pruning decisions are computed controller-side based on the monotonic metric history. A "Stop Signal" is propagated to the training runtime as a non-blocking annotation or status field, which the KubeflowCallback (SDK) periodically polls.

Synchronous "kill" calls during metric reporting create tight coupling and latency bottlenecks. By separating reporting from termination, we ensure the controller remains performant even under heavy trial loads.
