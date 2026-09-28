# Prometheus monitoring overlay

This overlay adds a `ServiceMonitor` for an already-installed Trainer
controller. It expects the Prometheus Operator CRDs to be installed.

The endpoint uses HTTPS and validates the certificate generated for the
`kubeflow-trainer-controller-manager` Service with the existing
`kubeflow-trainer-webhook-cert` Secret.

Authentication is controlled by the controller configuration. This static
overlay assumes the metrics endpoint is served securely and does not add a
bearer token; platform-specific deployments should add the ServiceMonitor
authentication and RBAC required by their metrics configuration.
