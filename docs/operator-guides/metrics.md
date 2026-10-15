# Trainer Controller Metrics

Kubeflow Trainer exposes controller-level Prometheus metrics for platform
administrators. The metrics describe installed runtimes, TrainJob activity,
and the workload capacity requested by TrainJobs.

Metrics are exposed by the controller metrics endpoint. TLS, authentication,
ServiceMonitor resources, and scrape configuration are provided by the platform
deployment; the controller does not create a Prometheus scrape target by
itself.

## Metrics

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `kubeflow_trainer_runtime_info` | Gauge, value `1` | `namespace`, `name`, `api_group`, `kind`, `ml_policy`, `pod_group_policy` | Current `TrainingRuntime` and `ClusterTrainingRuntime` inventory. The namespace is empty for cluster-scoped runtimes. |
| `kubeflow_trainer_trainjob_created_total` | Counter | `namespace`, `runtime_ref` | TrainJobs observed after the controller's initial informer listing. |
| `kubeflow_trainer_trainjob_complete_total` | Counter | `runtime_ref` | TrainJobs observed completing successfully. |
| `kubeflow_trainer_trainjob_failed_total` | Counter | `reason`, `runtime_ref` | TrainJobs observed failing. Known failure reasons are preserved; unknown reasons are reported as `other`. |
| `kubeflow_trainer_trainjob_lifetime_seconds` | Histogram | `result`, `runtime_ref` | Time from TrainJob creation to its terminal condition. |
| `kubeflow_trainer_trainjob_requested_training_nodes_per_job` | Histogram | `runtime_ref` | Logical training nodes requested by each TrainJob after runtime defaults are applied. |

`runtime_ref` is a normalized identifier in the form
`api-group/kind/namespace/name`. The namespace component is empty for a
`ClusterTrainingRuntime`.

## Important limitations

- Creation, completion, and workload metrics are event-based and reset when
  the controller process restarts or leadership changes. Existing objects
  found during the initial informer listing are not counted as newly created.
- Runtime inventory is evaluated at scrape time, so it reflects the current
  runtime resources.
- All Trainer metric families are exposed only by the elected leader replica;
  follower replicas expose none of them.
- The metrics do not currently provide durable history, checkpoint usage,
  progress, suspend/resume, accelerator utilization, or per-replicated-job
  runtime instrumentation.
- Labels use normalized identifiers and bounded categories, but total
  cardinality still grows with the number of runtimes and namespaces. Avoid
  adding job names, UIDs, raw errors, or other unbounded values.

## Example PromQL

Count successfully completed TrainJobs by runtime:

```promql
sum by (runtime_ref) (
  rate(kubeflow_trainer_trainjob_complete_total[15m])
)
```

Count failed TrainJobs by reason and runtime:

```text
sum by (reason, runtime_ref) (
  increase(kubeflow_trainer_trainjob_failed_total[$__range])
)
```

Calculate the observed failure percentage:

```text
100 *
sum(increase(kubeflow_trainer_trainjob_failed_total[$__range]))
/
(
  sum(increase(kubeflow_trainer_trainjob_complete_total[$__range]))
  +
  sum(increase(kubeflow_trainer_trainjob_failed_total[$__range]))
)
```

Inspect the distribution of requested training nodes:

```promql
histogram_quantile(
  0.95,
  sum by (le, runtime_ref) (
    rate(kubeflow_trainer_trainjob_requested_training_nodes_per_job_bucket[15m])
  )
)
```

The controller metrics are intended for cluster-local observability. Any
export, retention, filtering, or aggregation into an external telemetry
system should be reviewed and configured by the platform deployment.

## Metric stability

Metric names, labels, and semantics are part of the public observability
contract. Changes will follow the [Kubernetes metrics deprecation
policy](https://kubernetes.io/docs/reference/deprecation-policy/).
