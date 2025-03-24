# Developer Guide

Kubeflow Training Operator is currently at v1.

## Requirements

- [Go](https://golang.org/) (1.23 or later)
- Docker:
    - Windows and Linux:
      - [Docker](https://docs.docker.com/) (23 or later)
      - [Lima](https://github.com/lima-vm/lima?tab=readme-ov-file#adopters) (an alternative to DockerDesktop) (0.21.0 or later)
    - MacOS:
      - [Lima](https://github.com/lima-vm/lima?tab=readme-ov-file#adopters) (0.21.0 or later)
      - [Colima](https://github.com/abiosoft/colima) (Lima specifically for MacOS) (0.6.8 or later)

- [Python](https://www.python.org/) (3.11 or later)
- [kustomize](https://kustomize.io/) (4.0.5 or later)
- [Kind](https://kind.sigs.k8s.io/) (0.22.0 or later)
- [pre-commit](https://pre-commit.com/)

Note for Lima the link is to the Adopters, which supports several different container environments.

## Building the operator

Create a symbolic link inside your GOPATH to the location you checked out the code

```sh
$ mkdir -p $(go env GOPATH)/src/github.com/kubeflow
$ ln -sf ${GIT_TRAINING} $(go env GOPATH)/src/github.com/kubeflow/training-operator
```

- GIT_TRAINING should be the location where you checked out https://github.com/kubeflow/training-operator

Install dependencies

Change directory to project root and then:
```sh
$ go mod tidy
```

Build the library

```sh
$ go install github.com/kubeflow/trainer/cmd/trainer-controller-manager
```

after installing check by using which
```sh
$ which trainer-controller-manager
```

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

## Load docker image

```sh
$ kind load docker-image ${IMG}
```

## Modify operator image with new one

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

## Go version

On ubuntu the default go package appears to be gccgo-go which has problems see [issue](https://github.com/golang/go/issues/15429) golang-go package is also really old so install from golang tarballs instead.

## Generate Python SDK

To generate Python SDK for the operator, run:

```
make generate
```

This command will re-generate the api and model files together with the documentation and model tests.
The following files/folders in `sdk/` are auto-generated and should not be modified directly:

```
sdk/kubeflow/trainer/models
sdk/kubeflow/trainer/*.py
```

The Training Operator client and public APIs are located here:

```
sdk/kubeflow/trainer/api
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
