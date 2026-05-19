# Changelog

## [v8.9.1](https://github.com/kubeflow/trainer/releases/tag/v8.9.1) (2026-05-19)

This is Kubeflow Trainer v8.9.1 release.

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=v8.9.1"
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=v8.9.1"
```

You can now install controller manager with Helm charts 🚀

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer --version 8.9.1
```

For more information, please see [the Kubeflow Trainer docs](https://www.kubeflow.org/docs/components/trainer/overview/)
