# OptimizationJob Examples

Run hyperparameter searches over TrainJob trials. The
[OptimizationJob user guide](https://trainer.kubeflow.org/en/latest/user-guides/optimizationjob.html)
explains the API, metric reporting, and failure behavior.

## Prerequisites

- Kubeflow Trainer 2.4 or later with the OptimizationJob CRD and controller enabled.
- The `OptimizationJob` and `TrainJobStatus` feature gates enabled on the Trainer controller.
- The default `torch-distributed` ClusterTrainingRuntime installed.
- `kubectl` configured for the target cluster and namespace.
- Capacity for two concurrent CPU trials and the search algorithm Pod, and access to the
  container registries hosting their images.

## Random search

[`random-search.yaml`](random-search.yaml) fits a linear regression model to fixed, synthetic
training data and minimizes mean squared error on separate validation data. Four trials vary
learning rate and epoch count, with at most two trials running concurrently. Each trial uses one
CPU and Python's standard library; no GPUs, package installation, or dataset downloads are needed.
The example overrides the default runtime image and command, so it does not launch `torchrun`.

Run these commands from the repository root:

```bash
kubectl apply -f examples/optimizationjob/random-search.yaml
kubectl get trainjobs -l trainer.kubeflow.org/optimization-job=linear-regression-search
kubectl wait --for=condition=Complete optimizationjob/linear-regression-search --timeout=10m
kubectl get optimizationjob linear-regression-search -o jsonpath='{.status.result}'
```

If the wait times out, inspect `kubectl describe optimizationjob linear-regression-search` for
failure conditions; the wait command only watches for successful completion.

Delete the search and its owned trial resources when finished:

```bash
kubectl delete optimizationjob linear-regression-search
```
