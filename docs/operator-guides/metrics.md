# Trainer Controller Metrics

Kubeflow Trainer exposes controller-level Prometheus metrics for platform
administrators. The metrics describe installed runtimes, TrainJob activity,
and the workload capacity requested by TrainJobs.

Metrics are exposed by the controller metrics endpoint. Scraping and endpoint
security are configured by the deployment's metrics-server settings and
Prometheus integration.

## Metrics

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `kubeflow_trainer_runtime_info` | Gauge, value `1` | `namespace`, `name`, `api_group`, `kind`, `ml_policy`, `pod_group_policy` | Current `TrainingRuntime` and `ClusterTrainingRuntime` inventory. The namespace is empty for cluster-scoped runtimes. |
| `kubeflow_trainer_trainjob_created_total` | Counter | `namespace`, `runtime_ref` | TrainJobs observed after the controller's initial informer listing. |
| `kubeflow_trainer_trainjob_finished_total` | Counter | `result`, `reason`, `runtime_ref` | TrainJobs observed transitioning to a terminal condition. `result` is `succeeded` or `failed`; successful jobs use reason `none`. |
| `kubeflow_trainer_trainjob_lifetime_seconds` | Histogram | `result`, `runtime_ref` | Time from TrainJob creation to its terminal condition. |
| `kubeflow_trainer_trainjob_requested_accelerators` | Histogram | `accelerator_class`, `runtime_ref` | Requested accelerator capacity per TrainJob, including zero-accelerator jobs. This is requested capacity, not actual utilization. |
| `kubeflow_trainer_trainjob_requested_training_nodes_per_job` | Histogram | `runtime_ref` | Logical training nodes requested by each TrainJob after runtime defaults are applied. |

`runtime_ref` is a normalized identifier in the form
`api-group/kind/namespace/name`. The namespace component is empty for a
`ClusterTrainingRuntime`.

## Important limitations

- Creation, completion, and workload metrics are event-based and reset when
  the controller process restarts. Existing objects found during the initial
  informer listing are not counted as newly created.
- Runtime inventory is evaluated at scrape time, so it reflects the current
  runtime resources.
- Accelerator metrics describe requests configured on the TrainJob or its
  selected runtime. They do not measure allocation, utilization, or training
  framework runtime behavior.
- The metrics do not currently provide checkpoint, progress, suspend/resume,
  or per-replicated-job runtime instrumentation.
- Labels intentionally use bounded identifiers such as runtime references and
  resource classes. Avoid adding unbounded values such as arbitrary names or
  user-provided text to downstream aggregations.

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
system should be reviewed and configured downstream.
