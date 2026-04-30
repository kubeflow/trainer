# Advanced YAML Examples

Production-oriented examples demonstrating advanced Kubeflow Trainer features.

### 1. Runtime Patches (`01-runtime-patches.yaml`)

Pod customization via the `runtimePatches` API — `nodeSelector`, `tolerations`, `serviceAccountName`, JobSet labels and annotations. Each patch is keyed by a `manager` so users, Kueue, and webhooks can each own one entry without conflicting.

```bash
kubectl apply -f 01-runtime-patches.yaml
kubectl describe trainjob runtime-patches-example
kubectl delete trainjob runtime-patches-example
```

### 2. Kueue Integration (`02-kueue-integration.yaml`)

Queue-based scheduling with [Kueue](https://kueue.sigs.k8s.io/). Requires Kueue and a configured LocalQueue/ClusterQueue. The TrainJob is created suspended; Kueue unsuspends it once admitted.

```bash
kubectl apply -f 02-kueue-integration.yaml
kubectl get trainjob kueue-example
kubectl get workload -l kueue.x-k8s.io/job-name=kueue-example
kubectl delete trainjob kueue-example
```

See the [Kueue scheduling guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/kueue/).

### 3. Volcano Gang Scheduling (`03-volcano-integration.yaml`)

Gang scheduling with [Volcano](https://volcano.sh/). Defines a ClusterTrainingRuntime with `podGroupPolicy.volcano` (it must be set on the Runtime, not the TrainJob), then a TrainJob that references it. Requires the Volcano scheduler.

```bash
kubectl apply -f 03-volcano-integration.yaml
kubectl get trainjob volcano-example
kubectl get podgroup -l jobset.sigs.k8s.io/jobset-name=volcano-example
kubectl delete trainjob volcano-example
kubectl delete clustertrainingruntime torch-distributed-volcano
```

See the [Volcano scheduling guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/volcano/).

### 4. Multi-Step Pipeline (`04-multi-step.yaml`)

A dataset-initializer step that downloads a HuggingFace dataset, then a trainer step that consumes it from the shared `/workspace/dataset` volume. The default `torch-distributed` runtime has no initializer step, so this example inlines a `TrainingRuntime` that adds one.

```bash
kubectl apply -f 04-multi-step.yaml
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=dataset-initializer -f
kubectl logs -l trainer.kubeflow.org/trainjob-ancestor-step=trainer -f
kubectl delete trainjob multi-step-example
kubectl delete trainingruntime torch-with-dataset-init
```

## See also

- [Runtime guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Job scheduling guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/)
