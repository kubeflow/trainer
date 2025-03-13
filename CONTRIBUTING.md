# Developer Guide

This guide explains how to contribute to the Kubeflow Trainer V2 project.

## Requirements

- [Go](https://golang.org/) (1.23 or later)
- [Docker](https://docs.docker.com/) (23 or later)
- [Python](https://www.python.org/) (3.11 or later)
- [kustomize](https://kustomize.io/) (4.0.5 or later)
- [Kind](https://kind.sigs.k8s.io/) (0.27.0 or later)
- [Lima](https://github.com/lima-vm/lima?tab=readme-ov-file#adopters) (an alternative to DockerDesktop) (0.21.0 or later)
  - [Colima](https://github.com/abiosoft/colima) (Lima specifically for MacOS) (0.6.8 or later)
- [pre-commit](https://pre-commit.com/)

Note for Lima the link is to the Adopters, which supports several different container environments.

## Development

### Development Workflow

The Kubeflow Trainer project includes a Makefile with several helpful commands to streamline your development workflow:

```sh
# Generate manifests, APIs and SDK
make generate

# Download required tools for development
make ginkgo envtest controller-gen kind

# Format and verify code
make fmt vet golangci-lint

# Run tests (see Testing section below)
make test test-integration test-e2e

# Setup a cluster for e2e testing
make test-e2e-setup-cluster
```

You can see all available commands by running:
```sh
make help
```

### Setting Up Your Environment

#### Quick Setup

The simplest way to set up a local development environment is to use the provided Makefile command:

```sh
make test-e2e-setup-cluster
```

This command will:
1. Build the necessary Docker images
2. Create a Kind cluster if it doesn't exist
3. Load your images into the cluster
4. Deploy the operator

#### Manual Setup

Create a symbolic link inside your GOPATH to the location you checked out the code:

```sh
mkdir -p $(go env GOPATH)/src/github.com/kubeflow
ln -sf ${GIT_TRAINING} $(go env GOPATH)/src/github.com/kubeflow/training-operator
```

- GIT_TRAINING should be the location where you checked out https://github.com/kubeflow/training-operator

Install dependencies:

```sh
go mod tidy
```

Build the library:

```sh
go install github.com/kubeflow/training-operator/cmd/training-operator.v1
```

### Running the Operator Locally

Running the operator locally (as opposed to deploying it on a K8s cluster) is convenient for debugging/development.

You can create a `kind` cluster by running:

```sh
kind create cluster
```

This will load your kubernetes config file with the new cluster.

After creating the cluster, you can check the nodes with the code below which should show you the kind-control-plane:

```sh
kubectl get nodes
```

The output should look something like below:

```
$ kubectl get nodes
NAME                 STATUS   ROLES           AGE   VERSION
kind-control-plane   Ready    control-plane   32s   v1.27.3
```

From here we can apply the manifests to the cluster:

```sh
kubectl apply --server-side -k "github.com/kubeflow/training-operator/manifests/overlays/standalone"
```

Then we can patch it with the latest operator image:

```sh
kubectl patch -n kubeflow deployments training-operator --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "kubeflow/training-operator:latest"}]'
```

### Submit a Sample Job

After setting up the cluster, you can submit a sample job using a TrainJob:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-mnist-example
spec:
  runtimeRef:
    name: torch-distributed
    apiGroup: trainer.kubeflow.org
    kind: ClusterTrainingRuntime
```

Apply the job:
```sh
kubectl apply -f pytorch-job.yaml
```

Check the job status:
```sh
kubectl get trainjobs
kubectl describe trainjob pytorch-mnist-example
```

You can also run a traditional PyTorch job example:

```sh
kubectl apply -f https://raw.githubusercontent.com/kubeflow/training-operator/master/examples/pytorch/simple.yaml
```

And we can see the output of the job from the logs:

```sh
kubectl logs -n kubeflow -l training.kubeflow.org/job-name=pytorch-simple --follow
```

## Testing

The Kubeflow Trainer project includes several types of tests to ensure code quality and functionality.

### Unit Tests

Run the Go unit tests with:

```sh
make test
```

You can also run Python unit tests:

```sh
make test-python
```

### Integration Tests

Run the Go integration tests with:

```sh
make test-integration
```

For Python integration tests:

```sh
make test-python-integration
```

### E2E Tests

Run the end-to-end tests with:

```sh
make test-e2e
```

To set up a Kind cluster for e2e testing:

```sh
make test-e2e-setup-cluster
```

You can also run Jupyter notebook tests with Papermill:

```sh
make test-e2e-notebook
```

### Testing Local Changes

To test your local changes to the operator:

1. Build a custom operator image:
   ```sh
   make docker-build IMG=my-username/training-operator:my-pr-01
   ```

2. Load the image into your kind cluster:
   ```sh
   kind load docker-image my-username/training-operator:my-pr-01
   ```

3. Update the operator deployment to use your image:
   ```sh
   cd ./manifests/overlays/standalone
   kustomize edit set image my-username/training-operator=my-username/training-operator:my-pr-01
   ```

4. Deploy the updated operator:
   ```sh
   kubectl apply -k ./manifests/overlays/standalone
   ```

5. Submit a job to test:
   ```sh
   kubectl patch -n kubeflow deployments training-operator --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "my-username/training-operator:my-pr-01"}]'
   kubectl apply -f https://raw.githubusercontent.com/kubeflow/training-operator/master/examples/pytorch/simple.yaml
   ```

6. Check the job logs:
   ```sh
   kubectl logs -n kubeflow -l training.kubeflow.org/job-name=pytorch-simple
   ```

## SDK Development

### Generate Python SDK

To generate Python SDK for the operator, run:

```sh
./hack/python-sdk/gen-sdk.sh
```

This command will re-generate the api and model files together with the documentation and model tests.
The following files/folders in `sdk/python` are auto-generated and should not be modified directly:

```
sdk/python/docs
sdk/python/kubeflow/training/models
sdk/python/kubeflow/training/*.py
sdk/python/test/*.py
```

The Training Operator client and public APIs are located here:

```
sdk/python/kubeflow/training/api
```

## Best Practices

### Go Version

On ubuntu the default go package appears to be gccgo-go which has problems. It's recommended to install Go from official tarballs.

### Code Style

Make sure to install [pre-commit](https://pre-commit.com/) (`pip install pre-commit`) and run `pre-commit install` from the root of the repository at least once before creating git commits.

The pre-commit hooks ensure code quality and consistency. They are executed in CI. PRs that fail to comply with the hooks will not be able to pass the corresponding CI gate. The hooks are only executed against staged files unless you run `pre-commit run --all`, in which case, they'll be executed against every file in the repository.

Specific programmatically generated files listed in the `exclude` field in [.pre-commit-config.yaml](../../.pre-commit-config.yaml) are deliberately excluded from the hooks.
