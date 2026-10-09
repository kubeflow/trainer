# Prometheus monitoring overlay

This overlay adds a `ServiceMonitor` for an already-installed Trainer
controller. It expects the Prometheus Operator CRDs to be installed.

The endpoint uses HTTPS and validates the certificate generated for the
`kubeflow-trainer-controller-manager` Service with the existing
`kubeflow-trainer-webhook-cert` Secret.

Authentication is controlled by the controller configuration. This static
overlay assumes the metrics endpoint is served securely and does not add a
bearer token. If metrics authentication is enabled, patch the ServiceMonitor
with an authorization.credentials Secret reference and configure the
Prometheus ServiceAccount with permission to read the non-resource URL
`/metrics`.

The required Prometheus RBAC rule is equivalent to:

```yaml
rules:
  - nonResourceURLs:
      - /metrics
    verbs:
      - get
```

Bind this rule through a ClusterRoleBinding to the ServiceAccount used by
Prometheus. The token Secret referenced by authorization.credentials must be
available in the namespace containing the ServiceMonitor and readable by
Prometheus.
