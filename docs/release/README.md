# Releasing Kubeflow Trainer

## Prerequisites

- Docker available locally (required by `hack/release.sh` for changelog generation with `git-cliff`).
- `GITHUB_TOKEN` exported locally (recommended to avoid GitHub API rate limits while generating changelog):

```bash
export GITHUB_TOKEN=<token>
```

- If you are working from a fork, ensure upstream tags are available locally before running release:

```bash
git remote add upstream https://github.com/kubeflow/trainer.git  # if missing
git fetch upstream --tags
git fetch origin --tags
```

## Prepare a release PR

Run the release target from your working branch:

```bash
make release VERSION=X.Y.Z GITHUB_TOKEN=<token>
# or
make release VERSION=X.Y.Z-rc.N GITHUB_TOKEN=<token>
```

`make release` exports `GITHUB_TOKEN` and invokes `hack/release.sh`. The release script will:

1. Validate the version format.
2. Verify the tag `vX.Y.Z` (or `vX.Y.Z-rc.N`) does not already exist.
3. Update:
   - `VERSION`
   - image tags in `manifests/*.yaml` (`newTag` and pinned `ghcr.io/...:latest`)
   - `charts/kubeflow-trainer/Chart.yaml` (`version`)
   - `CHANGELOG.md` (prepends unreleased section using `git-cliff`)
4. Run `make generate`.
5. Create a signed-off commit:

```text
Release vX.Y.Z
```

Push the branch and open a PR to `master`.

## PR validation (`check-release.yaml`)

When a PR to `master` changes `VERSION`, CI validates:

1. `VERSION` matches semver format.
2. The tag does not already exist.
3. Every `manifests` `newTag` equals `v<VERSION-without-leading-v>`.
4. `charts/kubeflow-trainer/Chart.yaml` version equals `VERSION` without leading `v`.
5. `api/python_api/kubeflow_trainer_api/__init__.py` `__version__` equals `VERSION` without leading `v`.

## Release automation after merge (`release.yaml`)

When the `VERSION` change is merged into `master`, the workflow:

1. Re-validates version and manifest tags.
2. Builds and validates Python package artifacts.
3. Publishes the package to PyPI (`kubeflow-trainer-api`).
4. Creates release branch `release-<version-without-v>` if it does not exist.
5. Creates and pushes git tag `v<version-without-v>`.
6. Creates GitHub Release using generated changelog.
7. Dispatches:
   - `build-and-push-images.yaml` for container image publishing
   - `publish-helm-charts.yaml` for Helm chart publishing
