# Releasing Kubeflow Trainer

## Prerequisites

- Docker available locally (required for changelog generation with
  [`git-cliff`](https://git-cliff.org/)).

- Create a [GitHub Token](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token).

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

For **the latest minor release**, run the following command from the `master` branch.

For **an older minor series patch** (for example, `v2.2.1` when `master` is on `v2.3.x`), checkout
to the corresponding `release-X.Y` branch and run the following command.

```bash
make release VERSION=vX.Y.Z GITHUB_TOKEN=<token>
# or for a release candidate:
make release VERSION=vX.Y.Z-rc.N GITHUB_TOKEN=<token>
```

This will:

1. Update `VERSION` to `vX.Y.Z`.
1. Update Python API models version to `X.Y.Z`
1. Update Helm Charts version to `X.Y.Z`
1. Generate `CHANGELOG/CHANGELOG-X.Y.md` using `git-cliff` (skipped for RC releases).

After reviewing the changes, create a signed commit and open a PR to the appropriate branch
(e.g. `master` or `release-X.Y`):

```bash
git add -A && git commit -s -m 'Prepare release vX.Y.Z'
```

### 2. Automated Release After Merge

When the `VERSION` change is merged, the
[release workflow](.github/workflows/release.yaml) runs automatically:

1. Ensures the `release-X.Y` branch exists and contains the version bump:
   cherry-picks the merged "Prepare release" commit (VERSION, Helm chart version,
   regenerated assets, and changelog) onto the branch, or creates the branch from
   `master` if it doesn't exist yet.
2. Pins the release-only image references on the release branch, then commits and pushes:
   - Image tags (`newTag`) in the manifest overlays.
   - `CACHE_IMAGE` in the data cache runtime.
   - `configMapGenerator` version in the manager overlay.
3. Builds and validates the Python package with `uv`.
4. Creates and pushes the git tag `vX.Y.Z` (skipped if it already exists).
5. Publishes the Python package to [PyPI](https://pypi.org/project/kubeflow-trainer-api/)
   using OIDC trusted publishing.
6. Creates a GitHub Release with the generated changelog and the built package artifacts.

## Announcement

Post the release announcement for the new Kubeflow Trainer release in:

- [#kubeflow-trainer](https://cloud-native.slack.com/archives/C0742LDFZ4K) Slack channel
- [`kubeflow-discuss`](https://groups.google.com/g/kubeflow-discuss) mailing list
