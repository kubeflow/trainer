---
paths:
  - "pkg/apis/**"
---

# API type conventions

- Pure data types only - no business logic, no imports from `pkg/controller/`, `pkg/webhooks/`, or `pkg/runtime/`
- Use CEL validation (`+kubebuilder:validation:XValidation`) over webhook validation when possible
- Mark optional fields with `+optional` and pointer types with `omitempty` json tag
- Add `+kubebuilder:object:root=true` and `+k8s:deepcopy-gen:interfaces=...` markers on root types
- Register new types in `groupversion_info.go` `addKnownTypes()`
- Run `make generate` after any change - generated files (`zz_generated.*.go`) must never be edited manually
