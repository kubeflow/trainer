# Advanced YAML Examples

Production-ready examples demonstrating advanced Kubeflow Trainer features.

## Examples

### 1. PodSpec Overrides (`01-podspec-overrides.yaml`)

Customizing pod specifications with `podTemplateOverrides` — resource limits, env vars, node selectors, tolerations, and annotations.

```bash
kubectl apply -f 01-podspec-overrides.yaml
kubectl describe trainjob podspec-example
kubectl delete trainjob podspec-example
```

### 2. Kueue Integration (`02-kueue-integration.yaml`)

Queue-based job scheduling with [Kueue](https://kueue.sigs.k8s.io/). Requires Kueue and a configured LocalQueue/ClusterQueue.

```bash
kubectl apply -f 02-kueue-integration.yaml
kubectl get trainjobs kueue-example
kubectl delete trainjob kueue-example
```

See: [Kubeflow Trainer Kueue Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/kueue/)

### 3. Volcano Gang Scheduling (`03-volcano-integration.yaml`)

Gang scheduling for distributed training with [Volcano](https://volcano.sh/). Creates a ClusterTrainingRuntime with `podGroupPolicy` enabled (note: `podGroupPolicy` must be set in the Runtime, not the TrainJob). Requires Volcano scheduler.

```bash
kubectl apply -f 03-volcano-integration.yaml
kubectl get trainjobs volcano-example
kubectl get podgroup -l trainer.kubeflow.org/job-name=volcano-example
kubectl delete trainjob volcano-example && kubectl delete clustertrainingruntime torch-distributed-volcano
```

See: [Kubeflow Trainer Volcano Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/volcano/)

### 4. Multi-Step Pipeline (`04-multi-step.yaml`)

Training pipeline with dataset initialization — downloads a dataset before training begins.

```bash
kubectl apply -f 04-multi-step.yaml
kubectl logs -l trainer.kubeflow.org/job-name=multi-step-example -f
kubectl delete trainjob multi-step-example
```

## Additional Resources

- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)
- [Runtime Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/)
- [Job Scheduling Guide](https://www.kubeflow.org/docs/components/trainer/operator-guides/job-scheduling/)
