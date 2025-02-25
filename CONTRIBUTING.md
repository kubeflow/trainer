# Developer Guide

# TODO (andreyvelich): This doc needs to be updated for Kubeflow Trainer V2

Kubeflow Training Operator is currently at v1.

## Requirements

- [Go](https://golang.org/) (1.23 or later)
- [Docker](https://docs.docker.com/) (23 or later)
- [Python](https://www.python.org/) (3.11 or later)
- [kustomize](https://kustomize.io/) (4.0.5 or later)
- [Kind](https://kind.sigs.k8s.io/) (0.22.0 or later)
- [Lima](https://github.com/lima-vm/lima?tab=readme-ov-file#adopters) (an alternative to DockerDesktop) (0.21.0 or later)
  - [Colima](https://github.com/abiosoft/colima) (Lima specifically for MacOS) (0.6.8 or later)
- [pre-commit](https://pre-commit.com/)

Note for Lima the link is to the Adopters, which supports several different container environments.

## Building the operator

Create a symbolic link inside your GOPATH to the location you checked out the code

```sh
mkdir -p $(go env GOPATH)/src/github.com/kubeflow
ln -sf ${GIT_TRAINING} $(go env GOPATH)/src/github.com/kubeflow/training-operator
```

- GIT_TRAINING should be the location where you checked out https://github.com/kubeflow/training-operator

Install dependencies

```sh
go mod tidy
```

Build the library

```sh
go install github.com/kubeflow/training-operator/cmd/training-operator.v1
```

## Running the Operator Locally

Running the operator locally (as opposed to deploying it on a K8s cluster) is convenient for debugging/development.

### Run a Kubernetes cluster

First, you need to run a Kubernetes cluster locally. We recommend [Kind](https://kind.sigs.k8s.io).

You can create a `kind` cluster by running

```sh
kind create cluster
```

This will load your kubernetes config file with the new cluster.

After creating the cluster, you can check the nodes with the code below which should show you the kind-control-plane.

```sh
kubectl get nodes
```

The output should look something like below:

```
$ kubectl get nodes
NAME                 STATUS   ROLES           AGE   VERSION
kind-control-plane   Ready    control-plane   32s   v1.27.3
```

Note, that for the example job below, the PyTorchJob uses the `kubeflow` namespace.

From here we can apply the manifests to the cluster.

```sh
kubectl apply --server-side -k "github.com/kubeflow/training-operator/manifests/overlays/standalone"
```

Then we can patch it with the latest operator image.

```sh
kubectl patch -n kubeflow deployments training-operator --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "kubeflow/training-operator:latest"}]'
```

Then we can run the job with the following command.

```sh
kubectl apply -f https://raw.githubusercontent.com/kubeflow/training-operator/master/examples/pytorch/simple.yaml
```

And we can see the output of the job from the logs, which may take some time to produce but should look something like below.

```
$ kubectl logs -n kubeflow -l training.kubeflow.org/job-name=pytorch-simple --follow
Defaulted container "pytorch" out of: pytorch, init-pytorch (init)
2024-04-19T19:00:29Z INFO     Train Epoch: 1 [4480/60000 (7%)]	loss=2.2295
2024-04-19T19:00:32Z INFO     Train Epoch: 1 [5120/60000 (9%)]	loss=2.1790
2024-04-19T19:00:35Z INFO     Train Epoch: 1 [5760/60000 (10%)]	loss=2.1150
2024-04-19T19:00:38Z INFO     Train Epoch: 1 [6400/60000 (11%)]	loss=2.0294
2024-04-19T19:00:41Z INFO     Train Epoch: 1 [7040/60000 (12%)]	loss=1.9156
2024-04-19T19:00:44Z INFO     Train Epoch: 1 [7680/60000 (13%)]	loss=1.7949
2024-04-19T19:00:47Z INFO     Train Epoch: 1 [8320/60000 (14%)]	loss=1.5567
2024-04-19T19:00:50Z INFO     Train Epoch: 1 [8960/60000 (15%)]	loss=1.3715
2024-04-19T19:00:54Z INFO     Train Epoch: 1 [9600/60000 (16%)]	loss=1.3385
2024-04-19T19:00:57Z INFO     Train Epoch: 1 [10240/60000 (17%)]	loss=1.1650
2024-04-19T19:00:29Z INFO     Train Epoch: 1 [4480/60000 (7%)]	loss=2.2295
2024-04-19T19:00:32Z INFO     Train Epoch: 1 [5120/60000 (9%)]	loss=2.1790
2024-04-19T19:00:35Z INFO     Train Epoch: 1 [5760/60000 (10%)]	loss=2.1150
2024-04-19T19:00:38Z INFO     Train Epoch: 1 [6400/60000 (11%)]	loss=2.0294
2024-04-19T19:00:41Z INFO     Train Epoch: 1 [7040/60000 (12%)]	loss=1.9156
2024-04-19T19:00:44Z INFO     Train Epoch: 1 [7680/60000 (13%)]	loss=1.7949
2024-04-19T19:00:47Z INFO     Train Epoch: 1 [8320/60000 (14%)]	loss=1.5567
2024-04-19T19:00:50Z INFO     Train Epoch: 1 [8960/60000 (15%)]	loss=1.3715
2024-04-19T19:00:53Z INFO     Train Epoch: 1 [9600/60000 (16%)]	loss=1.3385
2024-04-19T19:00:57Z INFO     Train Epoch: 1 [10240/60000 (17%)]	loss=1.1650
```

## Testing changes locally

Now that you confirmed you can spin up an operator locally, you can try to test your local changes to the operator.
You do this by building a new operator image and loading it into your kind cluster.

### Build Operator Image

```sh
make docker-build IMG=my-username/training-operator:my-pr-01
```

You can swap `my-username/training-operator:my-pr-01` with whatever you would like.

## Load docker image

```sh
kind load docker-image my-username/training-operator:my-pr-01
```

## Modify operator image with new one

```sh
cd ./manifests/overlays/standalone
kustomize edit set image my-username/training-operator=my-username/training-operator:my-pr-01
```

Update the `newTag` key in `./manifests/overlayes/standalone/kustimization.yaml` with the new image.

Deploy the operator with:

```sh
kubectl apply -k ./manifests/overlays/standalone
```

And now we can submit jobs to the operator.

```sh
kubectl patch -n kubeflow deployments training-operator --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "my-username/training-operator:my-pr-01"}]'
kubectl apply -f https://raw.githubusercontent.com/kubeflow/training-operator/master/examples/pytorch/simple.yaml
```

You should be able to see a pod for your training operator running in your namespace using

```
kubectl logs -n kubeflow -l training.kubeflow.org/job-name=pytorch-simple
```

## Go version

On ubuntu the default go package appears to be gccgo-go which has problems see [issue](https://github.com/golang/go/issues/15429) golang-go package is also really old so install from golang tarballs instead.

## Generate Python SDK

To generate Python SDK for the operator, run:

```
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

## Code Style

### pre-commit

Make sure to install [pre-commit](https://pre-commit.com/) (`pip install
pre-commit`) and run `pre-commit install` from the root of the repository at
least once before creating git commits.

The pre-commit [hooks](../../.pre-commit-config.yaml) ensure code quality and
consistency. They are executed in CI. PRs that fail to comply with the hooks
will not be able to pass the corresponding CI gate. The hooks are only executed
against staged files unless you run `pre-commit run --all`, in which case,
they'll be executed against every file in the repository.

Specific programmatically generated files listed in the `exclude` field in
[.pre-commit-config.yaml](../../.pre-commit-config.yaml) are deliberately
excluded from the hooks.
