# Architecture Decisions and Constraints

Non-obvious design decisions, invariants, and gotchas that cannot be discovered by reading the code alone.

## RuntimeRef is Immutable

`TrainJob.Spec.RuntimeRef` is marked immutable via CEL validation. Once a TrainJob is created referencing a specific TrainingRuntime or ClusterTrainingRuntime, it cannot be changed. This prevents inconsistent state where a running job's topology no longer matches its runtime definition. If you add new TrainJob spec fields that affect runtime resolution, they must also be immutable.

## Plugin Execution is Interface-Based, Not Order-Based

Plugins are registered in a flat map (`plugins/registry.go`). The framework discovers which interfaces each plugin implements via type assertion at startup (`framework/core/framework.go`). A single plugin can implement multiple interfaces (e.g., JobSet implements ComponentBuilder, PodNetwork, WatchExtension, TrainJobStatus, and CustomValidation). The execution order of plugins within the same interface type is non-deterministic (map iteration). Plugins must not depend on execution order relative to other plugins of the same type.

The pipeline order across interface types is fixed: EnforceMLPolicy -> EnforcePodGroupPolicy -> PodNetwork -> ComponentBuilder. This ordering matters because later plugins consume the Info object that earlier plugins have mutated.

## Only One TrainJobStatusPlugin Allowed

The framework enforces at most one `TrainJobStatusPlugin` implementation. This is validated at startup and will error with `errorTooManyTrainJobStatusPlugin`. Unlike other plugin types that aggregate results, status is singular.

## Runtime Registry Has Dependency Resolution

`pkg/runtime/core/core.go` initializes runtimes with dependency ordering. ClusterTrainingRuntime depends on TrainingRuntime (they share internal framework state). If adding a new runtime kind, declare dependencies in the `RuntimeRegistrar` struct in `registry.go` or initialization will race.

## Code Generation is Not Optional After API Changes

After modifying anything in `pkg/apis/`, you must run `make generate`. This triggers: deepcopy generation, client/informer/lister generation, OpenAPI spec generation, CRD manifest generation, and Python API model generation. Forgetting this breaks the build in CI. The generation script is `hack/update-codegen.sh`.

## Server-Side Apply, Not Create/Update

The TrainJob controller uses Kubernetes server-side apply (SSA) for all child resource management. This means resources are described as apply configurations, not full objects. The `pkg/apply/` package provides helpers. Do not use `client.Create()` or `client.Update()` for controller-managed resources - use SSA patterns from existing code.

## Ancestor Labels Drive Component Identity

The label `trainer.kubeflow.org/ancestor` on PodSets identifies their role: `trainer`, `dataset-initializer`, or `model-initializer`. Plugins use `FindPodSetByAncestor()` to locate the correct PodSet. If you add a new component type, you must define a new ancestor constant in `pkg/constants/constants.go` and ensure plugins can locate it.

## Webhooks Delegate to the Runtime

TrainJob validation webhooks do not contain validation logic directly. They call `Runtime.ValidateObjects()`, which runs all registered `CustomValidationPlugin` implementations. To add validation for a new plugin, implement the `CustomValidationPlugin` interface - do not add logic to the webhook files.

## Feature Gates Guard Experimental Features

Experimental features (e.g., TrainJobStatus) are gated via `pkg/features/`. Feature-gated plugins are conditionally registered in `plugins/registry.go`. To add a feature-gated plugin, define the gate in `pkg/features/`, then conditionally add it in `NewRegistry()`.

## TrainingRuntime Finalizers Prevent Dangling References

TrainingRuntime and ClusterTrainingRuntime controllers add finalizers when a runtime is referenced by active TrainJobs. This prevents deletion of in-use runtimes. The controllers use an indexer (`pkg/runtime/indexer/`) to efficiently look up which TrainJobs reference a given runtime.

## Python and Rust Components Are Separate Build Targets

Python initializers (`cmd/initializers/`) and Rust data cache (`cmd/data_cache/`, `pkg/data_cache/`) are not part of the Go build. They have their own Dockerfiles, test commands (`make test-python`, `make test-rust`), and lint configurations. Changes to these components do not require `make generate`.

## Pre-commit Hooks and CI Linters

Pre-commit runs: isort + black + flake8 (Python), cargo fmt + cargo check (Rust), yaml/json checks. Go formatting is handled separately via `make fmt` (gofmt) and `make golangci-lint` (two configs: `.golangci.yaml` and `.golangci-kal.yml` for Kubernetes API linting). CI enforces all of these - run them locally before pushing.
