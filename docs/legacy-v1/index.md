# Legacy v1 Documentation

:::{warning}
**Kubeflow Training Operator v1 is deprecated.**

This documentation is for the legacy Kubeflow Training Operator v1, which has been superseded by Kubeflow Trainer v2.

**Migration recommended**: See the [Migration Guide](../operator-guides/migration.md) to upgrade from v1 to v2.
:::

## What Changed in v2?

Kubeflow Trainer v2 introduces:

- **Unified API**: Single `TrainJob` CRD replaces framework-specific CRDs (PyTorchJob, TFJob, etc.)
- **Extensible Runtime System**: Plugin-based architecture for custom ML policies and schedulers
- **Local Execution**: Run training jobs locally with Docker, Podman, or process backends
- **Improved Developer Experience**: Python SDK, auto-generated configs, builtin trainers

## Legacy v1 Documentation

```{toctree}
:maxdepth: 1
:caption: Legacy v1 Guides

installation
getting-started
```

```{toctree}
:maxdepth: 1
:caption: Legacy v1 User Guides

user-guides/pytorch
user-guides/tensorflow
user-guides/paddlepaddle
user-guides/xgboost
user-guides/jax
user-guides/mpi
user-guides/llm-fine-tuning
```

## Support Policy

Kubeflow Training Operator v1 is in maintenance mode:

- **Security fixes**: Critical security issues will be patched
- **Bug fixes**: No new bug fixes for v1
- **New features**: All new development is on v2
- **End of life**: To be determined

For new deployments, use Kubeflow Trainer v2.
