---
name: add-new-crd
description: Scaffolds a new Kubernetes Custom Resource Definition (CRD) for the Kubeflow Trainer project. Covers Go type definitions with code generation markers, scheme registration, webhook and controller scaffolding, RBAC markers, testing wrappers, integration tests, and manifest generation. Use when asked to add a new CRD, create a new API resource, or define a new Kubernetes custom type for Trainer.
---

# Add a New CRD

Step-by-step workflow for adding a new Custom Resource Definition to
Kubeflow Trainer. This covers type definitions, scheme registration,
webhooks, controller, tests, and code generation.

Copy this checklist and track your progress:

```
CRD Progress:
- [ ] Step 1: Define API types
- [ ] Step 2: Register types with the scheme
- [ ] Step 3: Run code generation
- [ ] Step 4: Add webhooks
- [ ] Step 5: Add controller
- [ ] Step 6: Add testing wrappers
- [ ] Step 7: Add integration tests
- [ ] Step 8: Manual manifest and Helm updates
```

## Key files to read first

- **Existing type definitions**: [pkg/apis/trainer/v1alpha1/trainjob_types.go](../../../pkg/apis/trainer/v1alpha1/trainjob_types.go)
- **Scheme registration**: [pkg/apis/trainer/v1alpha1/groupversion_info.go](../../../pkg/apis/trainer/v1alpha1/groupversion_info.go)
- **Webhook setup**: [pkg/webhooks/setup.go](../../../pkg/webhooks/setup.go)
- **Controller setup**: [pkg/controller/setup.go](../../../pkg/controller/setup.go)
- **Package-level markers**: [pkg/apis/trainer/v1alpha1/doc.go](../../../pkg/apis/trainer/v1alpha1/doc.go)
- **API conventions**: [.agents/docs/api-conventions.md](../../docs/api-conventions.md) (SIG Architecture reference)
- **API changes guide**: [.agents/docs/api_changes.md](../../docs/api_changes.md) (SIG Architecture reference)

## Step 1: Define API types

Create `pkg/apis/trainer/v1alpha1/<newresource>_types.go`.

### Required markers on the root type

For a **namespaced** resource:

```go
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type NewResource struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              NewResourceSpec   `json:"spec,omitempty"`
    Status            NewResourceStatus `json:"status,omitempty"`
}
```

Notes on conditional markers:
- `+kubebuilder:subresource:status` - only add when the type has a Status field (e.g., `TrainingRuntime` omits it)
- `+kubebuilder:storageversion` - only needed when multiple API versions exist; omit while only `v1alpha1` exists
- `+kubebuilder:printcolumn` - add for fields useful in `kubectl get` output (State and Age are standard)

For a **cluster-scoped** resource, add these markers:

```go
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
```

### Required markers on the List type

```go
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type NewResourceList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []NewResource `json:"items"`
}
```

### Field-level validation

Always use [CEL validation](https://kubernetes.io/docs/reference/using-api/cel/)
when applicable. Common markers:

```go
// Immutable field:
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="field is immutable"

// Optional field with default:
// +optional
// +kubebuilder:default="value"

// List types for strategic merge patch:
// +listType=map
// +listMapKey=name

// Required field:
// +required
```

Reference existing types for the full set of markers used in this project:
- [trainjob_types.go](../../../pkg/apis/trainer/v1alpha1/trainjob_types.go) (namespaced, with printcolumns and CEL)
- [trainingruntime_types.go](../../../pkg/apis/trainer/v1alpha1/trainingruntime_types.go) (both namespaced and cluster-scoped)

## Step 2: Register types with the scheme

File: `pkg/apis/trainer/v1alpha1/groupversion_info.go`

Add both the root type and list type to `addKnownTypes()`:

```go
func addKnownTypes(scheme *runtime.Scheme) error {
    scheme.AddKnownTypes(GroupVersion,
        // ... existing types ...
        &NewResource{},
        &NewResourceList{},
    )
    metav1.AddToGroupVersion(scheme, GroupVersion)
    return nil
}
```

No changes needed in `cmd/trainer-controller-manager/main.go` - the
scheme is already registered globally via `trainer.AddToScheme(scheme)`.

## Step 3: Run code generation

Ensure Docker is running - code generation uses containerized tools.

```bash
make generate
```

This target also runs `manifests` as a prerequisite (CRD YAML, RBAC,
webhook config generation), so there is no need to run `make manifests`
separately.

It runs four steps:
1. `controller-gen object` - generates `zz_generated.deepcopy.go`
2. `hack/update-codegen.sh` - generates defaults, clients, informers, listers, apply configs under `pkg/client/`, and OpenAPI specs
3. `controller-gen object` for config API
4. Python API generation from swagger.json

Verify these generated files are created/updated:
- `pkg/apis/trainer/v1alpha1/zz_generated.deepcopy.go`
- `pkg/apis/trainer/v1alpha1/zz_generated.defaults.go`
- `pkg/apis/trainer/v1alpha1/zz_generated.openapi.go`
- `pkg/client/clientset/versioned/typed/trainer/v1alpha1/newresource.go`
- `pkg/client/listers/trainer/v1alpha1/newresource.go`
- `pkg/client/informers/externalversions/trainer/v1alpha1/newresource.go`
- `pkg/client/applyconfiguration/trainer/v1alpha1/newresource*.go`
- `api/openapi-spec/swagger.json`
- `api/python_api/` (Python models)

## Step 4: Add webhooks

Create `pkg/webhooks/<newresource>_webhook.go`.

### Defaulting webhook (if needed)

```go
// +kubebuilder:webhook:path=/mutate-trainer-kubeflow-org-v1alpha1-newresource,mutating=true,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=newresources,verbs=create;update,versions=v1alpha1,name=mnewresource.kb.io,admissionReviewVersions=v1
```

Implement `admission.Defaulter[*trainer.NewResource]`:

```go
type NewResourceDefaulter struct{ ... }

func (d *NewResourceDefaulter) Default(ctx context.Context, obj *trainer.NewResource) error { ... }
```

### Validation webhook

```go
// +kubebuilder:webhook:path=/validate-trainer-kubeflow-org-v1alpha1-newresource,mutating=false,failurePolicy=fail,sideEffects=None,groups=trainer.kubeflow.org,resources=newresources,verbs=create;update,versions=v1alpha1,name=vnewresource.kb.io,admissionReviewVersions=v1
```

Implement `admission.Validator[*trainer.NewResource]`:

```go
type NewResourceValidator struct{ ... }

func (v *NewResourceValidator) ValidateCreate(ctx context.Context, obj *trainer.NewResource) (admission.Warnings, error) { ... }
func (v *NewResourceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *trainer.NewResource) (admission.Warnings, error) { ... }
func (v *NewResourceValidator) ValidateDelete(ctx context.Context, obj *trainer.NewResource) (admission.Warnings, error) { ... }
```

The `admission.Validator` interface requires `error`, not `field.ErrorList`.
Build a `field.ErrorList` in a validation helper, then call `.ToAggregate()`
at the boundary to convert it to `error`.

### Register with manager

```go
func setupWebhookForNewResource(mgr ctrl.Manager) error {
    return ctrl.NewWebhookManagedBy(mgr, &trainer.NewResource{}).
        WithDefaulter(&NewResourceDefaulter{...}).
        WithValidator(&NewResourceValidator{...}).
        Complete()
}
```

Add the call in `pkg/webhooks/setup.go`:

```go
func Setup(mgr ctrl.Manager, runtimes map[string]runtime.Runtime) (string, error) {
    // ... existing calls ...
    if err := setupWebhookForNewResource(mgr); err != nil {
        return trainer.NewResourceKind, err
    }
    return "", nil
}
```

Reference: [pkg/webhooks/trainjob_webhook.go](../../../pkg/webhooks/trainjob_webhook.go) (full defaulter + validator)

## Step 5: Add controller

Create `pkg/controller/<newresource>_controller.go`.

### RBAC markers

Place directly above the `Reconcile` method:

```go
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=newresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=newresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=newresources/finalizers,verbs=update
```

### Reconciler struct

```go
type NewResourceReconciler struct {
    log      logr.Logger
    client   client.Client
    recorder events.EventRecorder
}

var _ reconcile.Reconciler = (*NewResourceReconciler)(nil)
```

### SetupWithManager

```go
func (r *NewResourceReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
    return builder.TypedControllerManagedBy[reconcile.Request](mgr).
        Named("newresource_controller").
        WithOptions(options).
        WatchesRawSource(source.TypedKind(mgr.GetCache(), &trainer.NewResource{}, ...)).
        Complete(r)
}
```

### Register in setup.go

File: `pkg/controller/setup.go`

Add the controller to `SetupControllers()`. This function returns
`(string, error)` - the failing Kind alongside the error. Dependencies
come from the manager:

```go
func SetupControllers(mgr ctrl.Manager, runtimes map[string]runtime.Runtime, options controller.Options) (string, error) {
    // ... existing calls ...
    newResourceRec := NewNewResourceReconciler(
        mgr.GetClient(),
        mgr.GetEventRecorder("trainer-newresource-controller"),
    )
    if err := newResourceRec.SetupWithManager(mgr, options); err != nil {
        return trainer.NewResourceKind, err
    }
    return "", nil
}
```

Reference: [pkg/controller/trainjob_controller.go](../../../pkg/controller/trainjob_controller.go) (full controller pattern)

## Step 6: Add testing wrappers

File: `pkg/util/testing/wrapper.go`

Add builder wrappers following the existing patterns (e.g.,
`MakeTrainJobWrapper`, `MakeTrainingRuntimeWrapper`). These are used by
both unit and integration tests.

## Step 7: Add integration tests

### Controller tests

File: `test/integration/controller/<newresource>_controller_test.go`

Pattern:
- Use `ginkgo.Ordered` container
- `BeforeAll`: call `fwk.Init()` + `fwk.RunManager(cfg, true)` which returns `(context.Context, client.Client)`
- `AfterAll`: call `fwk.Teardown()` to stop the envtest control plane
- `BeforeEach`: create a fresh namespace
- Use builder wrappers from `pkg/util/testing`
- Assert expected child resource creation and status conditions

```go
ginkgo.BeforeAll(func() {
    fwk = &framework.Framework{}
    cfg = fwk.Init()
    ctx, k8sClient = fwk.RunManager(cfg, true)
})
ginkgo.AfterAll(func() {
    fwk.Teardown()
})
```

### Webhook tests

File: `test/integration/webhooks/<newresource>_webhook_test.go`

Pattern:
- `BeforeAll`: call `fwk.Init()` + `fwk.RunManager(cfg, false)` + `AfterAll` with `fwk.Teardown()`
- The `false` argument only gates controller registration - `RunManager` always sets up webhooks
- Test validation rejection and defaulting

Reference:
- [test/integration/controller/trainjob_controller_test.go](../../../test/integration/controller/trainjob_controller_test.go)
- [test/integration/webhooks/trainjob_test.go](../../../test/integration/webhooks/trainjob_test.go)

The integration test framework (`test/integration/framework/framework.go`)
automatically loads CRDs from `manifests/base/crds/` and webhook configs
from `manifests/base/webhook/manifests.yaml`.

## Step 8: Manual manifest and Helm updates

After `make generate` (which already runs `make manifests`), two files
require manual edits:

1. Add the new CRD to `manifests/base/crds/kustomization.yaml`:

```yaml
resources:
  # ... existing CRDs ...
  - trainer.kubeflow.org_newresources.yaml
```

2. Add RBAC rules to `charts/kubeflow-trainer/templates/rbac/clusterrole.yaml`
   for the new resource (get, list, watch, create, update, patch, delete on
   the resource, its status subresource, and finalizers).

## Verification

After all steps, run the full CI-equivalent validation:

```bash
go mod tidy
make verify-boilerplate
make generate
make fmt
make golangci-lint
make test
make test-integration
make test-python
make helm-unittest
```

Confirm:
- `make generate` produces no uncommitted diff (this already includes `manifests`)
- All test and lint targets pass
- New CRD YAML exists in `manifests/base/crds/` and `charts/kubeflow-trainer/templates/crd/`
- CRD is listed in `manifests/base/crds/kustomization.yaml`
- RBAC rules are added in `charts/kubeflow-trainer/templates/rbac/clusterrole.yaml`
- No unrelated files were modified
