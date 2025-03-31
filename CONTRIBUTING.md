# Developer Guide

This guide explains how to contribute to the Kubeflow Trainer V2 project.
For the Kubeflow Trainer documentation, please check [the official Kubeflow documentation](https://www.kubeflow.org/docs/components/trainer/overview/).

## Requirements

- [Go](https://golang.org/) (1.23 or later)
- [Docker](https://docs.docker.com/) (23 or later)
- [Lima](https://github.com/lima-vm/lima?tab=readme-ov-file#adopters) (an alternative to DockerDesktop) (0.21.0 or later)
  - [Colima](https://github.com/abiosoft/colima) (Lima specifically for MacOS) (0.6.8 or later)
- [Python](https://www.python.org/) (3.11 or later)
- [kustomize](https://kustomize.io/) (4.0.5 or later)
- [Kind](https://kind.sigs.k8s.io/) (0.27.0 or later)
- [pre-commit](https://pre-commit.com/)

Note for Lima the link is to the Adopters, which supports several different container environments.

## Development

The Kubeflow Trainer project includes a Makefile with several helpful commands to streamline your development workflow:

```sh
# Generate manifests, APIs and SDK
make generate
```

You can see all available commands by running:
```sh
make help
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

To set up a Kind cluster for e2e testing:

```sh
make test-e2e-setup-cluster
```

Run the end-to-end tests with:

```sh
make test-e2e
```


You can also run Jupyter notebook tests with Papermill:

```sh
make test-e2e-notebook
```
## Best Practices

### Go Development
When coding:

Follow the [effective go](https://go.dev/doc/effective_go) guidelines.
Run [`make generate`](https://github.com/kubeflow/trainer/blob/4e6199c9486d861655a712d7017b8f23f9f2e48e/Makefile#L87) locally to verify if changes follow best practices before submitting PRs.

When writing tests:

Use [cmp.Diff](https://pkg.go.dev/github.com/google/go-cmp/cmp#Diff) instead of reflect.Equal, to provide useful comparisons.
Define test cases as maps instead of slices to avoid dependencies on the running order. Map key should be equal to the test case name.

On ubuntu the default go package appears to be gccgo-go which has problems. It's recommended to install Go from official tarballs.

## Code Style


### pre-commit

Make sure to install [pre-commit](https://pre-commit.com/) (`pip install pre-commit`) and run `pre-commit install` from the root of the repository at least once before creating git commits.

The pre-commit hooks ensure code quality and consistency. They are executed in CI. PRs that fail to comply with the hooks will not be able to pass the corresponding CI gate. The hooks are only executed against staged files unless you run `pre-commit run --all`, in which case, they'll be executed against every file in the repository.

Specific programmatically generated files listed in the `exclude` field in [.pre-commit-config.yaml](../../.pre-commit-config.yaml) are deliberately excluded from the hooks.


## Running the Operator Locally

Running the operator locally (as opposed to deploying it on a K8s cluster) is convenient for debugging/development.

### Run a Kubernetes cluster

First, you need to run a Kubernetes cluster locally. We recommend [Kind](https://kind.sigs.k8s.io).

You can create a `kind` cluster by running

```sh
$ kind create cluster
```

This will load your kubernetes config file with the new cluster.

After creating the cluster, you can check the nodes with the code below which should show you the kind-control-plane.

```sh
$ kubectl get nodes
```

The output should look something like below:

```
$ kubectl get nodes
NAME                 STATUS   ROLES           AGE   VERSION
kind-control-plane   Ready    control-plane   32s   v1.27.3
```

Note, that for the example job below, the TrainJob uses the `kubeflow-system` namespace.

From here we can apply the manifests to the cluster.

```sh
$ kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=master"
```
```sh
$ kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=master"
```

Ensure that the JobSet and Trainer controller manager pods are running:
```
$ kubectl get pods -n kubeflow-system

NAME                                                   READY   STATUS    RESTARTS   AGE
jobset-controller-manager-694f54749-tx9t8              1/1     Running   0          2m19s
kubeflow-trainer-controller-manager-74c685f689-td8ms   1/1     Running   0          2m19s

```



Then we can patch it with the latest operator image.


```sh
$ kubectl patch -n kubeflow-system deployments kubeflow-trainer-controller-manager --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "kubeflow/training-operator:latest"}]'

deployment.apps/kubeflow-trainer-controller-manager patched
```

Then we can run the job with the following command.


`trainjob.yaml file:`
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

```sh
$ kubectl apply -f trainjob.yaml
```

We can see the output of the job from the logs like below. But before checking logs, first check if our trainjob has completed.
```sh
$ kubectl get trainjobs --all-namespaces
NAMESPACE   NAME                    STATE      AGE
default     pytorch-mnist-example   Complete   15m
```

to check which node executed the job:
```sh
$ kubectl get pods -n default
NAME                                           READY   STATUS      RESTARTS   AGE
pytorch-mnist-example-trainer-node-0-0-2t9bl   0/1     Completed   0          16m
```

check the logs (you should see a list of python packages)
```sh
$ kubectl logs -f pytorch-mnist-example-trainer-node-0-0-2t9bl -n default -c trainer
Torch Distributed Runtime
--------------------------------------
Torch Default Runtime Env
Package                   Version
------------------------- ------------
archspec                  0.2.3
asttokens                 2.4.1
astunparse                1.6.3
attrs                     24.2.0
beautifulsoup4            4.12.3
{--truncated for readability--}
```

## Testing changes locally

Now that you confirmed you can spin up an operator locally, you can try to test your local changes to the operator.
You do this by building a new operator image and loading it into your kind cluster.

### Build Operator Image

```sh
$ export IMG=my-username/training-operator:my-pr-01
```
```sh
$ docker build -t ${IMG} -f cmd/trainer-controller-manager/Dockerfile .
```
You can swap `my-username/training-operator:my-pr-01` with whatever you would like.

### Load docker image

```sh
$ kind load docker-image ${IMG}
```

### Modify operator image with new one

```sh
$ cd ./manifests/overlays/manager

$ kustomize edit set image my-username/training-operator=my-username/training-operator:my-pr-01

```

Update the `newTag` key in `./manifests/overlayes/standalone/kustimization.yaml` with the new image.

Deploy the operator with (after changing directory back to project root):

```sh
$ kubectl apply --server-side -k ./manifests/overlays/manager
```

And now we can submit jobs to the operator.

```sh
$ kubectl patch -n kubeflow-system deployments kubeflow-trainer-controller-manager --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "my-username/training-operator:my-pr-01"}]'

deployment.apps/kubeflow-trainer-controller-manager patched
```

Again apply the trainjob using the steps similar as done in "Running Your Operator Locally" section. You may need to delete the trainjob if it exists and recreate it. After the trainjob has been created, you can verify that your operator image was used

```
$ kubectl get deployment -n kubeflow-system kubeflow-trainer-controller-manager -o=jsonpath='{.spec.template.spec.containers[0].image}'

my-username/training-operator:my-pr-01
```


## Legacy Setup Instructions

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

### Running Sample Jobs

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

### SDK Development

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
