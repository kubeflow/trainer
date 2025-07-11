# Developer Guide

This guide explains how to contribute to the Kubeflow Trainer V2 project.
For the Kubeflow Trainer documentation, please check [the official Kubeflow documentation](https://www.kubeflow.org/docs/components/trainer/overview/).

## Requirements

- [Go](https://golang.org/) (1.24 or later)
- [Docker](https://docs.docker.com/) (23 or later)
- [Lima](https://github.com/lima-vm/lima?tab=readme-ov-file#adopters) (an alternative to DockerDesktop) (0.21.0 or later)
  - [Colima](https://github.com/abiosoft/colima) (Lima specifically for MacOS) (0.6.8 or later)
- [Python](https://www.python.org/) (3.11 or later)
- [kustomize](https://kustomize.io/) (4.0.5 or later)
- [Kind](https://kind.sigs.k8s.io/) (0.27.0 or later)
- [pre-commit](https://pre-commit.com/)

Note for Lima the link is to the Adopters, which supports several different container environments.

## Development

The Kubeflow Trainer project includes a Makefile with several helpful commands to streamline your development workflow:

```sh
# Generate manifests and APIs.
make generate
```

You can see all available commands by running:

```sh
make help
```

## Testing

The Kubeflow Trainer project includes several types of tests to ensure code quality and functionality.

### Unit Tests

Run the Go unit tests with:

```sh
make test
```

You can also run Python unit tests:

```sh
make test-python
```

### Integration Tests

Run the Go integration tests with:

```sh
make test-integration
```

For Python integration tests:

```sh
make test-python-integration
```

### E2E Tests

To set up a Kind cluster for e2e testing:

```sh
make test-e2e-setup-cluster
```

Run the end-to-end tests with:

```sh
make test-e2e
```

You can also run Jupyter notebook tests with Papermill:

```sh
make test-e2e-notebook
```

## Coding Style

Make sure to install [pre-commit](https://pre-commit.com/) (`pip install pre-commit`) and run `pre-commit install` from the root of the repository at least once before creating git commits.

The pre-commit hooks ensure code quality and consistency. They are executed in CI. PRs that fail to comply with the hooks will not be able to pass the corresponding CI gate. The hooks are only executed against staged files unless you run `pre-commit run --all`, in which case, they'll be executed against every file in the repository.

Specific programmatically generated files listed in the `exclude` field in [.pre-commit-config.yaml](../../.pre-commit-config.yaml) are deliberately excluded from the hooks.

## Best Practices

### Pull Request Title Conventions

We enforce a pull request (PR) title convention to quickly indicate the type and scope of a PR.
The PR titles are used to generated changelog for releases.

PR titles must:

- Follow the [Conventional Commits specification](https://www.conventionalcommits.org/en/v1.0.0/).
- Have an appropriate [type and scope](./.github/workflows/check-pr-title.yaml)

Examples:

- fix(operator): Check empty value for registry
- chore(ci): Remove unused scripts
- feat(docs): Create guide for LLM Fine-Tuning

### Kubeflow Enhancement Proposal (KEP)

For any significant features or enhancement for Kubeflow Trainer project we follow the
[Kubeflow Enhancement Proposal process](https://github.com/kubeflow/community/tree/master/proposals).

If you want to submit a significant change to the Kubeflow Trainer, please submit a new KEP under
[./docs/proposals](./docs/proposals/) directory.

### Go Development

When coding:

Follow the [effective go](https://go.dev/doc/effective_go) guidelines.
Run [`make generate`](https://github.com/kubeflow/trainer/blob/4e6199c9486d861655a712d7017b8f23f9f2e48e/Makefile#L87) locally to verify if changes follow best practices before submitting PRs.

When writing tests:

Use [cmp.Diff](https://pkg.go.dev/github.com/google/go-cmp/cmp#Diff) instead of reflect.Equal, to provide useful comparisons.
Define test cases as maps instead of slices to avoid dependencies on the running order. Map key should be equal to the test case name.

On ubuntu the default go package appears to be gccgo-go which has problems. It's recommended to install Go from official tarballs.

### SDK Development

Changes to the Kubeflow Trainer Python SDK can be made in the https://github.com/kubeflow/sdk repo.

The Trainer SDK can be found at https://github.com/kubeflow/sdk/tree/main/python/kubeflow/trainer.
