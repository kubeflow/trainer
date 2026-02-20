# Testing the Release Process on a Fork

This guide walks through testing the full Kubeflow Trainer release pipeline
using a personal GitHub fork **without** publishing anything to the upstream
registries or PyPI.

---

## Prerequisites

| Tool | Purpose |
|------|---------|
| Git | Branch and tag management |
| Docker | Changelog generation via git-cliff (local `hack/release.sh`) |
| GNU Make | Running `make release` |
| Python 3.11+ | Chart.yaml patching inside `release.sh` |
| GitHub CLI (`gh`) | Optional — convenient for creating PRs |

You also need a **GitHub personal access token** (classic) with `repo` and
`workflow` scopes, referred to as `$GITHUB_TOKEN` below.

---

## 1. Fork and Clone

```bash
# Fork kubeflow/trainer on GitHub, then:
git clone https://github.com/<you>/trainer.git
cd trainer
git remote add upstream https://github.com/kubeflow/trainer.git
git fetch upstream
git checkout -b test-release upstream/master
git push origin test-release
```

---

## 2. Remove the Repository Guard

The `build-and-push-images.yaml` workflow skips on forks because of:

```yaml
if: github.repository == 'kubeflow/trainer'
```

For fork testing, create an override commit **on your fork only**:

```bash
# .github/workflows/build-and-push-images.yaml
# Change line 16:
#   if: github.repository == 'kubeflow/trainer'
# To:
#   if: true
```

> **Do not include this change in any PR back to upstream.**

---

## 3. Set Up the `release` Environment

The `publish_pypi` and `github_release` jobs require a GitHub Environment
named **`release`**.

1. Go to your fork → **Settings → Environments → New environment**
2. Name it `release`
3. No protection rules are needed for testing

---

## 4. Configure PyPI (Optional — Test PyPI)

The `publish_pypi` job uses **OIDC trusted publishing** (no API token).
To test actual publishing without touching the real PyPI:

1. Register on [Test PyPI](https://test.pypi.org)
2. Create a trusted publisher for your fork:
   - Owner: `<your-github-username>`
   - Repository: `trainer`
   - Workflow: `release.yaml`
   - Environment: `release`
3. Temporarily edit `release.yaml` to point at Test PyPI:
   ```yaml
   # In the publish_pypi job, add:
   - name: Publish to PyPI
     uses: pypa/gh-action-pypi-publish@release/v1
     with:
       print-hash: true
       packages-dir: dist/
       repository-url: https://test.pypi.org/legacy/   # ← add this line
   ```

If you just want to test everything **except** the actual PyPI upload,
skip this step — the job will fail at the publish step, but all prior
jobs will still run and validate correctly.

---

## 5. Prepare the Release Commit Locally

Pick a test version that does not collide with existing tags:

```bash
export GITHUB_TOKEN=ghp_your_token_here

# Standard release:
make release VERSION=99.0.0

# Or release candidate:
make release VERSION=99.0.0-rc.1
```

This runs `hack/release.sh` which:
- Writes `v99.0.0` to `VERSION`
- Updates all `newTag:` values in `manifests/` to `v99.0.0`
- Pins `ghcr.io/kubeflow/trainer/*:latest` images to `v99.0.0`
- Updates the configmap version in `manifests/overlays/manager/`
- Sets `Chart.yaml` version to `99.0.0`
- Sets `api/python_api/kubeflow_trainer_api/__init__.py` to `99.0.0`
- Generates CHANGELOG.md via git-cliff (requires Docker)
- Runs `make generate`
- Creates a signed commit: `Release v99.0.0`

---

## 6. Push and Open a PR

```bash
git push origin test-release

# Open PR against your fork's master branch
gh pr create --base master --title "chore(release): Release v99.0.0" \
  --body "Test release" --repo <you>/trainer
```

### What to verify on the PR

The **Check Release** workflow (`check-release.yaml`) runs and validates:
- VERSION matches semver pattern
- Tag `v99.0.0` does not already exist
- All `newTag:` values in manifests match `v99.0.0`
- `Chart.yaml` version matches `99.0.0`
- Python API `__version__` matches `99.0.0`

All checks must pass before merging.

---

## 7. Merge and Watch the Release Workflow

Merge the PR into your fork's `master`. This triggers `release.yaml`
(on push to `master` when `VERSION` changes).

### Job execution order

```
prepare
  ├─→ build_python_api
  │     ├─→ create_branch_and_tag
  │     │     ├─→ trigger_builds  (dispatches image + helm workflows)
  │     │     └─→ publish_pypi    (OIDC → PyPI)
  │     │           └─→ github_release (changelog + GitHub Release)
```

### What each job does

| Job | What to verify |
|-----|---------------|
| `prepare` | Version parsed, tag/branch outputs set correctly |
| `build_python_api` | Package builds, twine check passes, artifact uploaded |
| `create_branch_and_tag` | Branch `release-99.0` created, tag `v99.0.0` pushed |
| `trigger_builds` | `build-and-push-images` and `publish-helm-charts` workflows dispatched |
| `publish_pypi` | OIDC auth works, package published (or fails gracefully on fork) |
| `github_release` | GitHub Release created with git-cliff changelog |

---

## 8. Verify Dispatched Workflows

After `trigger_builds` runs, check the **Actions** tab for two additional
workflow runs:

### build-and-push-images
- Triggered via `workflow_dispatch` with `ref: v99.0.0`
- Builds all 7 container images
- On forks (with the guard removed): pushes to `ghcr.io/<you>/trainer/*`
- Verify the `template-publish-image` action tags images with `v99.0.0`

### publish-helm-charts
- Triggered via `workflow_dispatch` with `ref: v99.0.0`
- Reads `Chart.yaml` version (should be `99.0.0` since ref is the tag)
- Packages `kubeflow-trainer-99.0.0.tgz`
- Pushes to `oci://ghcr.io/<you>/charts`

---

## 9. Validate the GitHub Release

Go to your fork's **Releases** page and confirm:
- Release named `Kubeflow Trainer v99.0.0`
- Tag: `v99.0.0`
- Body contains the git-cliff changelog for **only** the latest release
- `prerelease` is `false` for stable, `true` for `-rc.N`

---

## 10. Cleanup

```bash
# Delete the test tag and release branch from your fork
git push origin --delete v99.0.0
git push origin --delete release-99.0

# Delete the GitHub Release via the UI or:
gh release delete v99.0.0 --repo <you>/trainer --yes

# Reset your master branch
git checkout master
git reset --hard upstream/master
git push origin master --force
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `build-and-push-images` skipped | Repository guard `github.repository == 'kubeflow/trainer'` | See Step 2 |
| `publish_pypi` fails with 403 | OIDC trusted publisher not configured for your fork | See Step 4, or ignore — prior jobs still validate |
| `trigger_builds` fails with 403 | `actions: write` permission missing | Ensure `GITHUB_TOKEN` has `workflow` scope in fork settings |
| `github_release` body is empty | git-cliff found no conventional commits | Ensure commits use `feat:`, `fix:`, `chore:` prefixes |
| `release.sh` crashes on `GITHUB_TOKEN` | Unset token with `set -o nounset` | Export: `export GITHUB_TOKEN=ghp_...` |
| `check-release` fails on Chart version | `release.sh` didn't run or was run with wrong version | Re-run `make release VERSION=X.Y.Z` |

---

## Notes

- The `release.yaml` workflow uses **OIDC trusted publishing** for PyPI —
  no `PYPI_API_TOKEN` secret is needed. The GitHub environment `release`
  must match what is configured as a trusted publisher on PyPI.
- The `--latest` flag on git-cliff ensures only the current release's
  changelog appears in the GitHub Release body, not the full history.
- On forks, image pushes go to `ghcr.io/<you>/trainer/*` (GHCR
  auto-scopes to the repository owner).
