---
name: add-python-sdk-model
description: A step-by-step guide for adding a new Python SDK model to the Kubeflow Trainer API, including type definitions, code generation, initializer integration, and testing.
---

# Adding a Python SDK Model

This skill provides a step-by-step guide for adding a new Python SDK model to the Kubeflow Trainer API. SDK models are automatically generated from the Go API types. To add a new model, you will update the Go API, run code generation, and integrate the generated model into the Python initializers.

## 1. Type Definitions

Start by adding or updating the API types in the Go codebase.

The Go API definitions live in `pkg/apis/trainer/v1alpha1/` (for example, `trainjob_types.go` or `trainingruntime_types.go`). Add your new model or fields to the corresponding Go structs. Ensure that you include the necessary Kubernetes API machinery markers and JSON tags (e.g., `json:"myField,omitempty"`).

## 2. Generate SDK Models

Do **not** write or update the Python SDK models manually! They are generated from the OpenAPI specification, which in turn is generated from the Go API types.

Run the `generate` make target from the repository root:

```bash
make generate
```

This command will:
1. Update the OpenAPI specifications.
2. Run `hack/python-api/gen-api.sh` to generate the new Python SDK models using `openapi-generator-cli`.

The newly generated Python SDK models will be placed in `api/python_api/kubeflow_trainer_api/models/`. 

## 3. Initializer Integration

Once the Python SDK models are generated, you can integrate them into the Trainer's initialization logic. The Python initializers are responsible for preparing datasets and models before training.

The initializer code is located in:
* `pkg/initializers/dataset/` (for datasets)
* `pkg/initializers/model/` (for models)

Create or update the relevant Python module (e.g., `pkg/initializers/dataset/my_new_dataset.py`) to handle your new SDK model. Import the generated models from `kubeflow_trainer_api.models` and implement the logic needed to download, validate, or process your model/dataset.

## 4. Tests

After integrating your new model into the initializers, you must add tests to verify the behavior.

1. **Unit Tests**: Place your unit test file alongside your implementation (e.g., `pkg/initializers/dataset/my_new_dataset_test.py`). 
2. **Integration Tests**: If applicable, add integration tests in `test/integration/initializers/`.

Run the Python unit tests and integration tests using the provided `Makefile` targets:

```bash
# Run Python unit tests
make test-python

# Run Python integration tests
make test-python-integration
```
