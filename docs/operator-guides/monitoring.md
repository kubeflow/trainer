# Prometheus Monitoring

This guide describes how to monitor the Kubeflow Trainer controller using Prometheus metrics
and the optional Grafana dashboard shipped with the Helm chart.

## Prerequisites

- Kubeflow Trainer installed via [Helm chart](installation.md#install-with-helm-charts) or kustomize manifests
- Prometheus configured to scrape the controller's metrics endpoint

:::{note}
The Trainer controller serves metrics over HTTPS on port 8443 with
[controller-runtime secure serving](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/metrics/server).
The Helm chart does not include a ServiceMonitor or PodMonitor, so you must configure
Prometheus scraping separately. If you use the
[Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), create a
ServiceMonitor that targets port 8443 with TLS settings matching your cluster's CA.
:::

## Controller Metrics

The Trainer controller exposes standard
[controller-runtime metrics](https://book.kubebuilder.io/reference/metrics-reference)
at the `/metrics` endpoint. These include Go process metrics, reconciliation counters, and
workqueue statistics.

Key metrics used by the Grafana dashboard:

| Metric | Type | Description |
| --- | --- | --- |
| `up` | gauge | Whether Prometheus can scrape the controller target |
| `go_goroutines` | gauge | Number of active goroutines |
| `process_resident_memory_bytes` | gauge | Resident memory size in bytes |
| `controller_runtime_reconcile_total` | counter | Total reconciliation attempts (labels: `controller`, `result`) |
| `controller_runtime_reconcile_time_seconds` | histogram | Reconciliation duration in seconds (labels: `controller`) |
| `workqueue_depth` | gauge | Current depth of the work queue (label: `name`) |
| `workqueue_retries_total` | counter | Total number of workqueue retries (label: `name`) |

You can verify that metrics are being scraped by port-forwarding to the controller pod:

```bash
kubectl port-forward -n kubeflow-system deployment/kubeflow-trainer-controller-manager 8443:8443
```

Then query `https://localhost:8443/metrics` (the endpoint uses a self-signed certificate, so you
may need to skip TLS verification for local debugging).

## Enabling the Grafana Dashboard

The Helm chart includes a pre-built Grafana dashboard that provides out-of-the-box visibility into
controller health and TrainJob lifecycle. It is disabled by default.

To enable it, set `grafanaDashboard.enabled` in your Helm values:

```yaml
grafanaDashboard:
  enabled: true
```

This creates a ConfigMap labeled with `grafana_dashboard: "1"` for
[Grafana sidecar](https://github.com/grafana/helm-charts/tree/main/charts/grafana#sidecar-for-dashboards)
auto-discovery. If your Grafana sidecar uses a different label selector, override it with
`grafanaDashboard.labels`.

To place the dashboard in a specific Grafana folder, use annotations:

```yaml
grafanaDashboard:
  enabled: true
  annotations:
    grafana_folder: "Kubeflow"
```

## Dashboard Panels

The dashboard is organized into three rows.

### Controller Health

- **Controller scrape up** - whether Prometheus can reach the controller target
- **Goroutines** - number of active goroutines over time
- **Memory (RSS)** - resident memory usage of the controller process
- **Reconciles / sec** - reconciliation rate broken down by controller and result (success/error)
- **Reconcile duration p95** - 95th percentile reconciliation latency by controller

### Queue & Backlog

- **Workqueue depth** - current number of items waiting in each work queue
- **Workqueue retries / sec** - rate of retried items per queue

### TrainJob Lifecycle

- **TrainJob reconciles / sec** - reconciliation rate for the TrainJob controller by result
- **TrainJob reconcile errors / sec** - error rate for TrainJob reconciliations
- **TrainJob reconcile duration p95** - 95th percentile latency for TrainJob reconciliations

All panels use template variables for datasource, namespace, and job, so you can filter by
the Prometheus datasource and the namespace where the Trainer controller is running.
