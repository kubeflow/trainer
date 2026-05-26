# Releasing Kubeflow Trainer

## Table of Contents

- [Prerequisites](#prerequisites)
- [Versioning policy](#versioning-policy)
- [Release branches and tags](#release-branches-and-tags)
- [Prepare a release PR](#prepare-a-release-pr)
  - [New minor release (from master)](#new-minor-release-from-master)
  - [Patch release (from release branch)](#patch-release-from-release-branch)
- [Release automation after merge](#release-automation-after-merge-releaseyaml)
  - [Workflow DAG](#workflow-dag)
  - [Minor release flow (master push)](#minor-release-flow-master-push)
  - [Patch release flow (release branch push)](#patch-release-flow-release-branch-push)
- [PyPI OIDC trusted publishing setup](#pypi-oidc-trusted-publishing-setup)
- [Testing on a fork](#testing-on-a-fork)
  - [Fork setup](#fork-setup)
  - [Test the prepare and validation jobs](#test-the-prepare-and-validation-jobs)
  - [Test the full release flow](#test-the-full-release-flow)
  - [Cleanup after testing](#cleanup-after-testing)

---

## Prerequisites

- Docker available locally (required by `hack/release.sh` for changelog generation with
  [`git-cliff`](https://git-cliff.org/)).
- A [GitHub personal access token](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token)
  exported as `GITHUB_TOKEN` (recommended to avoid GitHub API rate limits during changelog
  generation):

  ```bash
  export GITHUB_TOKEN=<token>
  ```

- Maintainer access to [the Kubeflow Trainer API Python package](https://pypi.org/project/kubeflow-trainer-api/)
  (for production releases).

## Versioning policy

Kubeflow Trainer version format follows [Semantic Versioning](https://semver.org/).
Kubeflow Trainer versions are in the format of `vX.Y.Z`, where `X` is the major version, `Y` is
the minor version, and `Z` is the patch version.
The patch version contains only bug fixes.

Additionally, Kubeflow Trainer does pre-releases in this format: `vX.Y.Z-rc.N` where `N` is a number
of the `Nth` release candidate (RC) before an upcoming public release named `vX.Y.Z`.

> **Note:** The Python API package uses [PEP 440](https://peps.python.org/pep-0440/) versioning.
> Pre-release versions like `2.2.0-rc.1` are normalized to `2.2.0rc1` in
> `api/python_api/kubeflow_trainer_api/__init__.py`. The release workflow handles this conversion
> automatically.

## Release branches and tags

Kubeflow Trainer releases are tagged with tags like `vX.Y.Z`, for example `v2.0.0`.

Release branches are in the format of `release-X.Y`, where `X.Y` stands for
the minor release.

`vX.Y.Z` releases are released from the `release-X.Y` branch. For example,
`v2.0.0` release should be on `release-2.0` branch.

If you want to push changes to the `release-X.Y` release branch, you have to
cherry pick your changes from the `master` branch and submit a PR.

## Prepare a release PR

### New minor release (from master)

1. If you are working from a fork, ensure upstream tags are available locally:

   ```bash
   git remote add upstream https://github.com/kubeflow/trainer.git  # if missing
   git fetch upstream --tags
   git fetch origin --tags
   ```

2. Run the release script from your working branch:

   ```bash
   make release VERSION=X.Y.Z GITHUB_TOKEN=<token>
   # or for a release candidate:
   make release VERSION=X.Y.Z-rc.N GITHUB_TOKEN=<token>
   ```

   `make release` exports `GITHUB_TOKEN` and invokes `hack/release.sh`. The release script will:

   1. Validate the version format (`X.Y.Z` or `X.Y.Z-rc.N`).
   2. Verify the tag `vX.Y.Z` (or `vX.Y.Z-rc.N`) does not already exist.
   3. Update the following files:
      - `VERSION` — set to `vX.Y.Z`
      - `manifests/**/kustomization.yaml` — update all `newTag` values to `vX.Y.Z`
      - `manifests/overlays/manager/kustomization.yaml` — update `kubeflow_trainer_version` in
        `configMapGenerator`
      - `charts/kubeflow-trainer/Chart.yaml` — update `version` to `X.Y.Z`
      - `CHANGELOG/CHANGELOG-X.Y.md` — prepend unreleased section using `git-cliff`
   4. Run `make generate` to regenerate CRDs, OpenAPI specs, and Python API models.
   5. Create a signed-off commit: `Release vX.Y.Z`

3. Push the branch and open a PR to `master`:

   ```bash
   git push origin <your-branch>
   ```

4. Review the following files in the PR before merging:
   - `VERSION`
   - `api/python_api/kubeflow_trainer_api/__init__.py`
   - `CHANGELOG/CHANGELOG-X.Y.md`
   - `manifests/overlays/manager/kustomization.yaml` (check `kubeflow_trainer_version`)
   - `charts/kubeflow-trainer/Chart.yaml` (check `version`)

### Patch release (from release branch)

1. Cherry-pick the necessary fixes from `master` into the `release-X.Y` branch:

   ```bash
   git fetch upstream
   git checkout release-X.Y
   git rebase upstream/release-X.Y
   git cherry-pick <commit-sha>
   ```

2. Run the release script:

   ```bash
   make release VERSION=X.Y.Z GITHUB_TOKEN=<token>
   ```

3. Push the branch and open a PR to `release-X.Y`.

## Release automation after merge (`release.yaml`)

The release workflow is triggered when a push to `master` or a `release-*` branch modifies the
`VERSION` file. It performs the full release pipeline automatically.

### Workflow DAG

```text
prepare ──→ tests ──────────────┐
   │                            │
   └──→ build_python_api ──────┤
                                ↓
                          create_branch (master only)
                                ↓
                          publish_pypi (OIDC)
                                ↓
                          github_release (environment gate)
                                ↓
                          trigger_builds
```

### `prepare` — Validation

Runs on every trigger. Validates:

| Check | Details |
|-------|---------|
| Semver format | `VERSION` matches `vX.Y.Z` or `vX.Y.Z-rc.N` |
| Tag uniqueness | `vX.Y.Z` tag does not already exist |
| Manifest image tags | All `newTag` values in `manifests/` match the version tag |
| Helm chart version | `charts/kubeflow-trainer/Chart.yaml` `version` matches |
| Python API version | `__version__` in `__init__.py` matches (with PEP 440 RC normalization) |
| ConfigMap version | `kubeflow_trainer_version` in manager kustomization matches |

### `tests` — Go and Python unit tests

Runs `make test` (Go) and `make test-python` in parallel with `build_python_api`.
Both must pass before any publishing occurs.

### `build_python_api` — Build Python package

Builds the `kubeflow-trainer-api` wheel and sdist, runs `twine check`, and uploads
the artifacts for the `publish_pypi` job.

### Minor release flow (master push)

1. `prepare` validates all version checks.
2. `tests` and `build_python_api` run in parallel.
3. `create_branch` creates the `release-X.Y` branch from the merge commit.
4. `publish_pypi` publishes to PyPI using OIDC trusted publishing.
5. `github_release` creates the git tag and GitHub Release (requires `release` environment approval).
6. `trigger_builds` dispatches container image and Helm chart builds.

### Patch release flow (release branch push)

Same as above, except:

- `create_branch` is **skipped** (the branch already exists).
- All downstream jobs proceed normally — the `if: !cancelled() && !failure()` condition
  treats skipped dependencies as successful.

## PyPI OIDC trusted publishing setup

The release workflow uses [OIDC trusted publishing](https://docs.pypi.org/trusted-publishers/)
to publish to PyPI without long-lived API tokens.

**One-time setup by a PyPI maintainer:**

1. Go to [pypi.org/manage/project/kubeflow-trainer-api/settings/publishing/](https://pypi.org/manage/project/kubeflow-trainer-api/settings/publishing/).
2. Add a new trusted publisher with:

   | Field | Value |
   |-------|-------|
   | Owner | `kubeflow` |
   | Repository | `trainer` |
   | Workflow name | `release.yaml` |
   | Environment name | `release` |

3. The `publish_pypi` job uses `permissions: id-token: write` to request a short-lived
   OIDC token from GitHub, which PyPI verifies against the trusted publisher configuration.

**One-time setup in GitHub repository settings:**

1. Go to **Settings → Environments** in the `kubeflow/trainer` repository.
2. Create an environment named `release`.
3. (Optional) Add required reviewers for the `release` environment to gate deployments.

## Testing on a fork

Before merging release automation changes to upstream, you can validate the entire workflow
on your own fork.

### Fork setup

1. Fork `kubeflow/trainer` to your GitHub account.

2. Clone and set up remotes:

   ```bash
   git clone https://github.com/<your-user>/kf-trainer.git
   cd kf-trainer
   git remote add upstream https://github.com/kubeflow/trainer.git
   git fetch upstream --tags
   ```

3. Ensure GitHub Actions are enabled on your fork:
   - Go to **your fork → Actions** tab → click **"I understand my workflows, go ahead and enable them"**.

4. Create a `release` environment on your fork:
   - Go to **your fork → Settings → Environments → New environment**.
   - Name it `release`.
   - No required reviewers needed for testing.

5. (Optional) Set up OIDC for TestPyPI to validate the full publish flow:
   - Register the package on [test.pypi.org](https://test.pypi.org/).
   - Add a trusted publisher with your fork's owner, repo name, `release.yaml`, and `release`
     environment.
   - Add `repository-url: https://test.pypi.org/legacy/` to the `publish_pypi` step in your
     fork's copy of `release.yaml`.

### Test the prepare and validation jobs

This validates that `hack/release.sh` correctly updates all version files and that the
`prepare` job catches mismatches.

1. Create a test branch on your fork:

   ```bash
   git checkout -b test/release-flow
   ```

2. Run the release script locally:

   ```bash
   make release VERSION=99.0.0-rc.0 GITHUB_TOKEN=<token>
   ```

3. Push directly to `master` on your fork (this triggers the workflow):

   ```bash
   git push origin test/release-flow:master --force
   ```

4. Go to **your fork → Actions** and watch the `Release` workflow.
   - The `prepare` job should pass all validation checks.
   - The `tests` and `build_python_api` jobs should pass.
   - `create_branch` should create `release-99.0`.
   - `publish_pypi` will fail if OIDC is not configured on your fork (expected).

### Test the full release flow

To test including PyPI publish (against TestPyPI):

1. Temporarily modify `.github/workflows/release.yaml` on your test branch:

   ```yaml
   # In the publish_pypi job, add repository-url for TestPyPI:
   - name: Publish to PyPI
     uses: pypa/gh-action-pypi-publish@release/v1
     with:
       print-hash: true
       packages-dir: dist/
       repository-url: https://test.pypi.org/legacy/
   ```

2. Configure the trusted publisher on TestPyPI (see OIDC setup section above, but use
   `test.pypi.org` instead of `pypi.org`).

3. Push to master on your fork and verify the full pipeline runs to completion.

### Cleanup after testing

After validating on your fork, clean up test artifacts:

```bash
# Delete the test tag
git push origin :refs/tags/v99.0.0-rc.0

# Delete the test release branch
git push origin :refs/heads/release-99.0

# Reset your fork's master to upstream
git fetch upstream
git push origin upstream/master:master --force
```
