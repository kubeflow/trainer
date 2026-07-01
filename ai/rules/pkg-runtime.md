---
paths:
  - "pkg/runtime/**"
---

# Runtime and plugin conventions

- Plugins follow factory pattern: `const Name` + `func New(ctx, client, indexer, cfg) (Plugin, error)`
- Use compile-time interface assertions: `var _ framework.XPlugin = (*PluginName)(nil)`
- Register in `plugins/registry.go` `NewRegistry()` - feature-gated plugins use `features.Enabled()`
- Only one `TrainJobStatusPlugin` allowed - framework errors at startup if multiple registered
- Plugin execution order within same interface type is non-deterministic - do not depend on ordering
- Use Info object methods (`FindPodSetByAncestor`, `FindContainerByPodSetName`) - not manual traversal
- Runtime registry (`core/registry.go`) has dependency resolution - declare dependencies in `RuntimeRegistrar`
