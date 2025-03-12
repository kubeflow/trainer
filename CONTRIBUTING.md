# Developer Guide

# TODO (Trainer V2 Update):  The testing and local Docker image building instructions need to be updated for Kubeflow Trainer V2. The current instructions in the repository are based on the Training Operator V1 and may not be directly applicable to the Trainer V2 project..

## Requirements

- [Go](https://golang.org/) (1.23 or later)
- [Docker](https://docs.docker.com/) (23 or later)
- [Python](https://www.python.org/) (3.11 or later)
- [kustomize](https://kustomize.io/) (4.0.5 or later)
- [Kind](https://kind.sigs.k8s.io/) (0.22.0 or later)
- [Lima](https://github.com/lima-vm/lima?tab=readme-ov-file#adopters) (an alternative to DockerDesktop) (0.21.0 or later)
  - [Colima](https://github.com/abiosoft/colima) (Lima specifically for MacOS) (0.6.8 or later)
- [pre-commit](https://pre-commit.com/)

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

From here we can apply the manifests to the cluster.

```sh
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=master"
```

### Submit a Sample Job

You can submit a sample job using a TrainJob:

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

## Building the Operator

### Create Symbolic Link

Create a symbolic link inside your GOPATH to the location you checked out the code:

```sh
mkdir -p $(go env GOPATH)/src/github.com/kubeflow
ln -sf ${GIT_TRAINER} $(go env GOPATH)/src/github.com/kubeflow/trainer
```

- `GIT_TRAINER` should be the location where you checked out the Kubeflow Trainer repository

### Install Dependencies

```sh
go mod tidy
```

### Build the Controller Manager

```sh
go build -o bin/manager cmd/trainer-controller-manager/main.go
```

## Testing Changes Locally

**TODO**: The following section needs to be updated for Kubeflow Trainer V2:

### Build Operator Image

```sh
make docker-build IMG=my-username/training-operator:my-pr-01
```

### Load Docker Image

```sh
kind load docker-image my-username/training-operator:my-pr-01
```

### Modify Operator Image

```sh
cd ./manifests/overlays/standalone
kustomize edit set image my-username/training-operator=my-username/training-operator:my-pr-01
```

### Deploy Operator

```sh
kubectl apply -k ./manifests/overlays/standalone
```

### Submit Jobs

```sh
kubectl patch -n kubeflow deployments training-operator --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "my-username/training-operator:my-pr-01"}]'
kubectl apply -f https://raw.githubusercontent.com/kubeflow/training-operator/master/examples/pytorch/simple.yaml
```

## Go Version

On ubuntu the default go package appears to be gccgo-go which has problems. It's recommended to install Go from official tarballs.

## Generate Python SDK

To generate Python SDK for the operator, run:

```sh
./hack/python-sdk/gen-sdk.sh
```

## Code Style

### pre-commit

Make sure to install [pre-commit](https://pre-commit.com/) (`pip install pre-commit`) and run `pre-commit install` from the root of the repository at least once before creating git commits.

The pre-commit hooks ensure code quality and consistency. They are executed in CI. PRs that fail to comply with the hooks will not be able to pass the corresponding CI gate.
