# Trainer Prometheus monitoring

This guide explains the monitoring and scraping flow introduced by PR #4102.
It covers the existing Trainer controller metrics endpoint and the Kubernetes
resources required to make that endpoint discoverable by Prometheus.

This feature does not add Trainer-specific application metrics. It exposes the
existing controller-runtime metrics endpoint through a Prometheus Operator
`ServiceMonitor`.

## End-to-end flow

The complete scrape path is:

```text
Trainer controller pod
        |
        | exposes /metrics on container port 8443
        v
Trainer Service
        |
        | named port: monitoring-port
        v
ServiceMonitor
        |
        | watched by Prometheus Operator
        v
Prometheus scrape target
        |
        v
PromQL and dashboards
```

The Trainer Service selects the controller pod. The ServiceMonitor selects the
Trainer Service. These are separate selectors and both must match for scraping
to work.

## Service and labels

The controller Service exposes the metrics endpoint through the named port
`monitoring-port`:

```yaml
ports:
  - name: monitoring-port
    port: 8443
    targetPort: metrics
```

The Service has stable labels used by the Kustomize ServiceMonitor:

```yaml
app.kubernetes.io/name: trainer
app.kubernetes.io/component: manager
app.kubernetes.io/part-of: kubeflow
```

The Service's own selector targets the Trainer manager pod. The
`app.kubernetes.io/part-of` label identifies the Service as part of Kubeflow;
it does not change Service routing.

## Helm installation

The Helm chart creates the ServiceMonitor only when explicitly enabled:

```yaml
serviceMonitor:
  enabled: true
  labels:
    release: prometheus
```

The additional labels are useful when the Prometheus resource selects
ServiceMonitors by label, as is common with kube-prometheus-stack.

The Helm ServiceMonitor selects the Trainer Service and uses the named metrics
port. With the default secure-serving configuration, it renders an HTTPS
endpoint and trusts the Trainer webhook certificate:

```yaml
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: kubeflow-trainer
      app.kubernetes.io/instance: kubeflow-trainer
      app.kubernetes.io/part-of: kubeflow
      app.kubernetes.io/component: manager
  endpoints:
    - port: monitoring-port
      scheme: https
      tlsConfig:
        ca:
          secret:
            name: kubeflow-trainer-webhook-cert
            key: ca.crt
```

The exact `app.kubernetes.io/name` and `app.kubernetes.io/instance` values
depend on the Helm release name.

## TLS behavior

When `manager.config.metrics.secureServing` is true, the ServiceMonitor uses
HTTPS and configures:

- the CA Secret used by the Trainer metrics server;
- the CA key, defaulting to `ca.crt`;
- the TLS server name, derived from the configured webhook Service name and
  release namespace.

Custom certificates can be supplied with:

```yaml
serviceMonitor:
  tlsConfig:
    ca:
      secret:
        name: custom-metrics-ca
        key: ca.crt
    serverName: trainer-metrics.example.test
```

If secure serving is disabled, the Helm ServiceMonitor uses HTTP and omits its
TLS configuration. Metrics authentication cannot be enabled with insecure
serving; the chart rejects that combination during rendering.

## Optional metrics authentication

Metrics authentication is opt-in:

```yaml
manager:
  config:
    metrics:
      secureServing: true
      auth:
        enabled: true

serviceMonitor:
  enabled: true
  authorization:
    credentials:
      name: prometheus-metrics-token
      key: token
```

With authentication enabled, the Trainer metrics server validates the incoming
bearer token through Kubernetes authentication and authorization APIs. The
Helm chart therefore grants the Trainer controller permission to create:

- `tokenreviews.authentication.k8s.io`;
- `subjectaccessreviews.authorization.k8s.io`.

The Prometheus ServiceAccount also needs permission to read the non-resource
URL `/metrics`. That permission is deployment-specific because the Trainer
chart does not own the Prometheus ServiceAccount:

```yaml
rules:
  - nonResourceURLs:
      - /metrics
    verbs:
      - get
```

This rule should be placed in a ClusterRole and bound to the ServiceAccount
used by Prometheus. The Secret referenced by
`serviceMonitor.authorization.credentials` must be available in the
ServiceMonitor namespace and readable by Prometheus.

## Kustomize installation

The static monitoring overlay is located at:

```text
manifests/overlays/monitoring/
```

It creates one HTTPS ServiceMonitor for the existing
`kubeflow-system/kubeflow-trainer-controller-manager` Service and uses the
existing webhook certificate. It does not create a bearer-token reference or
Prometheus RBAC resources because those details depend on the Prometheus
installation.

If metrics authentication is enabled for a Kustomize deployment, patch the
ServiceMonitor with an `authorization.credentials` Secret reference and grant
the Prometheus ServiceAccount the `/metrics` permission described above.

Prometheus installations that select ServiceMonitors by label may also require
a platform-specific label patch, for example a `release` label expected by the
Prometheus resource.

## What must be true for a healthy scrape

All of the following must be true:

1. The Prometheus Operator CRDs are installed.
2. The Trainer Service exists and has the `monitoring-port` port.
3. The ServiceMonitor selector matches the Trainer Service labels.
4. Prometheus is configured to watch the ServiceMonitor namespace and labels.
5. The TLS CA and server name match the certificate served by Trainer.
6. If authentication is enabled, the token Secret and both RBAC paths are
   configured.

## Validation checklist

Render and inspect the resources:

```bash
helm template kubeflow-trainer charts/kubeflow-trainer \
  --namespace kubeflow-system \
  --set serviceMonitor.enabled=true

kubectl kustomize manifests/overlays/monitoring
```

On a running cluster, verify:

```bash
kubectl get service -n kubeflow-system kubeflow-trainer-controller-manager
kubectl get servicemonitor -n kubeflow-system
kubectl get --raw /apis/monitoring.coreos.com/v1/namespaces/kubeflow-system/servicemonitors
```

Then check the Prometheus target page or API. The Trainer target should use
HTTPS on port 8443, report `up`, and have an empty scrape error.

## Scope and follow-up

PR #4102 provides secure metrics exposure and scrape configuration. It does not
define or instrument Trainer-specific lifecycle, runtime, or workload metrics.
Those metrics require separate controller instrumentation and tests.
