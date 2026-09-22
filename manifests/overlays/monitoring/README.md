# Prometheus monitoring overlay

This overlay adds a `ServiceMonitor` for the Trainer controller metrics
endpoint. It expects the Prometheus Operator CRDs to be installed.

The endpoint uses HTTPS and validates the certificate generated for the
`kubeflow-trainer-controller-manager` Service with the existing
`kubeflow-trainer-webhook-cert` Secret.

Authentication is controlled by the controller configuration. If metrics
authentication is enabled, the Prometheus service account must be granted
permission to access `/metrics` by the platform-specific deployment.
