# Releasing Kubeflow Trainer

## Prerequisites

- [Write](https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories/repository-permission-levels-for-an-organization#permission-levels-for-repositories-owned-by-an-organization)
  permission for the Kubeflow Trainer repository.

- Docker available locally (required for changelog generation with
  [`git-cliff`](https://git-cliff.org/)).

- Create a [GitHub Token](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token)
  and set it as `GITHUB_TOKEN` environment variable.

## Versioning Policy

Kubeflow Trainer version format follows [Semantic Versioning](https://semver.org/).
Kubeflow Trainer versions are in the format of `vX.Y.Z`, where `X` is the major version, `Y` is
the minor version, and `Z` is the patch version.
The patch version contains only bug fixes.

Additionally, Kubeflow Trainer does pre-releases in this format: `vX.Y.Z-rc.N` where `N` is a number
of the `Nth` release candidate (RC) before an upcoming public release named `vX.Y.Z`.

## Release Branches and Tags

Kubeflow Trainer releases are tagged with tags like `vX.Y.Z`, for example `v2.0.0`.

Release branches are in the format of `release-X.Y`, where `X.Y` stands for
the minor release.

`vX.Y.Z` releases are released from the `release-X.Y` branch. For example,
`v2.0.0` release should be on `release-2.0` branch.

If you want to push changes to the `release-X.Y` release branch, you have to
cherry pick your changes from the `master` branch and submit a PR.

## Changelog Structure

Kubeflow Trainer uses a directory-based changelog structure under `CHANGELOG/`:

```text
CHANGELOG/
├── CHANGELOG-1.x.md
├── CHANGELOG-2.0.md
├── CHANGELOG-2.1.md
└── CHANGELOG-2.2.md
```

Each file contains releases for that minor series. The `make release` target
prepends new entries automatically using `git-cliff`.

## Step-by-Step Release Process

### 1. Update Version and Changelog

Run the release target from your working branch:

```bash
make release VERSION=X.Y.Z GITHUB_TOKEN=<token>
# or for a release candidate:
make release VERSION=X.Y.Z-rc.N GITHUB_TOKEN=<token>
```

This will:

1. Update `VERSION` to `vX.Y.Z`.
2. Generate `CHANGELOG/CHANGELOG-X.Y.md` using `git-cliff` (skipped for RC releases).

After reviewing the changes, create a signed commit:

```bash
git add -A && git commit -s -m 'Release vX.Y.Z'
```

### 2. Submit a Release PR

- **Latest minor release** (including patches on the latest minor series):
  open a PR to `master`.

- **Old minor series patch** (e.g. `v2.1.2` when `master` is at `v2.2.x`):
  cherry-pick the necessary fixes from `master` via PRs to the `release-X.Y` branch,
  then open the release PR to `release-X.Y`.

### 3. Automated Release After Merge

When the `VERSION` change is merged, the
[release workflow](.github/workflows/release.yaml) runs automatically:

1. Validates version format and ensures the tag doesn't already exist.
2. Runs Go and Python unit tests.
3. Builds the Python package.
4. Creates the `release-X.Y` branch (if it doesn't exist).
5. Updates release assets on the release branch:
   - Helm chart version in `Chart.yaml`.
   - Python API `__version__`.
   - Image tags and `configMapGenerator` version in manifests.
6. Publishes the Python package to [PyPI](https://pypi.org/project/kubeflow-trainer-api/)
   using OIDC trusted publishing.
7. Creates and pushes the git tag `vX.Y.Z`.
8. Creates a GitHub Release with the generated changelog.
9. Triggers container image and Helm chart publishing.

> **Note**: Helm chart version, Python API version, and manifest image tags are only updated
> on the release branch, not on `master`. This ensures users deploying from `master` always
> get the latest images.
