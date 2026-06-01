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
- **Tighter TrainJob Integration:** Introduce the `OptimizationJob` CRD focused exclusively on `TrainJobs`, using `runtime.RawExtension` for the template to allow user-defined metadata/labels.
- **Native Parameter Injection:** Replace legacy regex YAML substitution with native Kubernetes Environment Variable injection (e.g., `OPT_LEARNING_RATE`).
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

The MVP API surface is intentionally concise. Metric strategies, target goals, and complex statuses are deferred to keep the initial PR minimal.

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
)

type OptimizationJobSpec struct {
    // Objectives defines the metrics and directions (minimize/maximize).
    Objectives []Objective `json:"objectives"`

    // Algorithm defines the HPO algorithm (Random/Grid for Phase 1).
    Algorithm Algorithm `json:"algorithm"`

    // Parameters define the search space boundaries.
    Parameters []Parameter `json:"parameters"`

    // TrialConfig controls parallelism and max trials.
    TrialConfig TrialConfig `json:"trialConfig"`

    // TrialTemplate embeds the TrainJob template.
    // Parameters are injected via native Kubernetes Environment Variables.
    // +kubebuilder:pruning:PreserveUnknownFields
    TrialTemplate runtime.RawExtension `json:"trialTemplate"`
}

type Objective struct {
    Metric    string             `json:"metric"`
    Direction string             `json:"direction"`
}

type Algorithm struct {
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
    Type string   `json:"type"` // int, double, categorical
    Max  string   `json:"max,omitempty"`
    Min  string   `json:"min,omitempty"`
    List []string `json:"list,omitempty"`
}

type TrialConfig struct {
    NumTrials       *int32 `json:"numTrials,omitempty"`
    ParallelTrials  int32  `json:"parallelTrials,omitempty"`
    MaxFailedTrials int32  `json:"maxFailedTrials,omitempty"`
}

type OptimizationJobStatus struct {
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    Active    int32 `json:"active,omitempty"`
    Succeeded int32 `json:"succeeded,omitempty"`
    Failed    int32 `json:"failed,omitempty"`

    // BestTrial caches the highest performing trial and its parameters.
    BestTrial *BestTrial `json:"bestTrial,omitempty"`
}

type BestTrial struct {
    Name              string                `json:"name"`
    OptimalParameters []ParameterAssignment `json:"optimalParameters,omitempty"`
}

type ParameterAssignment struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}
```

## 5. Sample YAML (Phase 1)

Because `TrialTemplate` utilizes `runtime.RawExtension`, the controller does not enforce a rigid structure on the embedded object. This allows users to freely inject metadata, such as labels or annotations, for both the underlying Trial and the resulting `TrainJob`.

```yaml
apiVersion: optimizer.kubeflow.org/v1alpha1
kind: OptimizationJob
metadata:
  name: tune-bert
spec:
  objectives:
    - metric: accuracy
      direction: maximize
  algorithm:
    name: random
  parameters:
    - name: lr
      searchSpace:
        type: double
        min: "0.001"
        max: "0.1"
  trialConfig:
    numTrials: 10
    parallelTrials: 2
  trialTemplate:
    apiVersion: trainer.kubeflow.org/v1alpha1
    kind: TrainJob
    metadata:
      labels:
        hpo-experiment: tune-bert
    spec:
      trainer:
        image: docker.io/pytorch
        command:
          - "python"
          - "train.py"
          # Hyperparameters injected safely as Env Vars via $(VAR) expansion
          - "--lr=$(OPT_LR)"
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
2. **Trial Injection:** The controller constructs `TrainJob` manifests from the provided `TrialTemplate`, dynamically appending hyperparameter values as native `corev1.EnvVar` entries (e.g., `{"name": "OPT_LR", "value": "0.01"}`).
3. **Monitoring (No Katib DB):** The controller monitors the `TrainJobStatus` via the Progress API.
  - It relies on `status.trainerStatus` to track real-time success or failure of active trials.
4. **Completion Phase:** Upon trial completion, the controller evaluates the final metrics surfaced in the `TrainJob` status, While the schema natively supports an array of `Objectives` for future extensibility, the Phase 1 controller will evaluate the primary (first) objective to identify the `BestTrial` and update the `OptimizationJobStatus` accordingly.

## 7. Open Discussions

### Discussion 1: Checkpoint/Weight Handoff Contract

Advanced schedulers, such as Population-Based Training (PBT) planned for Phase 3, require the ability to copy model weights between trials.

In distributed environments, localized suggestion pods lack the necessary access to network-attached PVCs to perform these copies.

We propose discussing the introduction of a `checkpointRef` field within the `TrainJob` specification.

This would allow the Trainer controller to natively handle PVC-to-PVC data transfers, ensuring the OptimizationJob orchestrator remains decoupled from lower-level RBAC and storage complexities.