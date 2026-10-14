# Trainer Controller Metrics

Kubeflow Trainer exposes controller-level Prometheus metrics for platform
administrators. The metrics describe installed runtimes, TrainJob activity,
and the workload capacity requested by TrainJobs.

Metrics are exposed by the controller metrics endpoint. This document covers
the five metric families implemented by PR #4190. TLS, authentication,
ServiceMonitor resources, and scrape configuration are provided by PR #4102
or by the platform deployment; PR #4190 does not create a Prometheus scrape
target by itself.

## Metrics

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `kubeflow_trainer_runtime_info` | Gauge, value `1` | `namespace`, `name`, `api_group`, `kind`, `ml_policy`, `pod_group_policy` | Current `TrainingRuntime` and `ClusterTrainingRuntime` inventory. The namespace is empty for cluster-scoped runtimes. |
| `kubeflow_trainer_trainjob_created_total` | Counter | `namespace`, `runtime_ref` | TrainJobs observed after the controller's initial informer listing. |
| `kubeflow_trainer_trainjob_finished_total` | Counter | `result`, `reason`, `runtime_ref` | TrainJobs observed transitioning to a terminal condition. `result` is `succeeded` or `failed`; successful jobs use reason `none`. |
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
- The metrics do not currently provide durable history, checkpoint usage,
  progress, suspend/resume, accelerator utilization, or per-replicated-job
  runtime instrumentation.
- Labels use normalized identifiers and bounded categories, but total
  cardinality still grows with the number of runtimes and namespaces. Avoid
  adding job names, UIDs, raw errors, or other unbounded values.

## Example PromQL

Count completed TrainJobs by runtime and result:

```promql
sum by (runtime_ref, result) (
  rate(kubeflow_trainer_trainjob_finished_total[15m])
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
