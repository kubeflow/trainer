---
paths:
  - "pkg/controller/**"
---

# Controller conventions

- Reconcilers implement `reconcile.Reconciler` - register in `setup.go` via `SetupControllers()`
- Use `ctrl.LoggerFrom(ctx)` for structured logging - never create loggers manually
- Use `mgr.GetEventRecorder()` for events - record both success and failure
- Use server-side apply (SSA) for child resources via `pkg/apply/` - never `client.Create()` or `client.Update()`
- Use finalizers via `ctrlutil.AddFinalizer` / `ctrlutil.RemoveFinalizer` for deletion cleanup
- Tests use table-driven `map[string]struct` pattern with `utiltesting.NewClientBuilder()`
