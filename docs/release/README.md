# Releasing the Kubeflow Trainer

## Prerequisite

- [Write](https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories/repository-permission-levels-for-an-organization#permission-levels-for-repositories-owned-by-an-organization)
  permission for the Kubeflow Trainer repository.

- Maintainer access to [the Kubeflow Trainer API Python modules](https://pypi.org/project/kubeflow-trainer-api/).

- Create a [GitHub Token](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token).

- Install `PyGithub` to generate the [Changelog](./../../CHANGELOG.md):

  ```
  pip install PyGithub>=1.55
  ```

- Install `twine` and `build` to publish the SDK package:

  ```
  pip install twine>=6.1.0
  pip install build>=1.3.0
  ```

  - Create a [PyPI Token](https://pypi.org/help/#apitoken) to publish Training SDK.

  - Add the following config to your `~/.pypirc` file:

    ```
    [pypi]
       username = __token__
       password = <PYPI_TOKEN>
    ```

## Versioning policy

Kubeflow Trainer version format follows [Semantic Versioning](https://semver.org/).
Kubeflow Trainer versions are in the format of `vX.Y.Z`, where `X` is the major version, `Y` is
the minor version, and `Z` is the patch version.
The patch version contains only bug fixes.

Additionally, Kubeflow Trainer does pre-releases in this format: `vX.Y.Z-rc.N` where `N` is a number
of the `Nth` release candidate (RC) before an upcoming public release named `vX.Y.Z`.

## Release branches and tags

Kubeflow Trainer releases are tagged with tags like `vX.Y.Z`, for example `v2.0.0`.

Release branches are in the format of `release-X.Y`, where `X.Y` stands for
the minor release.

`vX.Y.Z` releases are released from the `release-X.Y` branch. For example,
`v2.0.0` release should be on `release-2.0` branch.

If you want to push changes to the `release-X.Y` release branch, you have to
cherry pick your changes from the `master` branch and submit a PR.

## Create a new Kubeflow Trainer release

### Create release branch

1. Depends on what version you want to release,

   - Major or Minor version - Use the GitHub UI to create a release branch from `master` and name
     the release branch `release-X.Y`
   - Patch version - You don't need to create a new release branch.

1. Fetch the upstream changes into your local directory:

   ```
   git fetch upstream
   ```

1. Checkout into the release branch:

   ```
   git checkout release-X.Y
   git rebase upstream/release-X.Y
   ```

### Release Kubeflow Trainer API Modules

1. Update the `API_VERSION` in [the `gen-api.sh` file](../../hack/python-api/gen-api.sh).

   You must follow this semantic `X.Y.ZrcN` for the RC or `X.Y.Z` for the public release.

   For example:

   ```sh
   API_VERSION = "2.1.0rc0"
   ```

1. Generate and publish the Kubeflow Trainer Python API models:

   ```
   make generate
   cd api/python_api
   rm -rf dist
   python -m build
   twine upload dist/*
   ```

### Release Kubeflow Trainer images

1. Update the image tag in Kubeflow Trainer overlays:

   - [manager](../../manifests/overlays/manager/kustomization.yaml)
   - [runtimes](../../manifests/overlays/runtimes/kustomization.yaml)

   The image tags must be equal to the release version, for example: `newTag: v2.0.0-rc.1`

1. Commit your changes, tag the commit, and push it to upstream.

   - For the RC tag run the following:

   ```sh
   git add manifests
   git commit -s -m "Kubeflow Trainer Official Release vX.Y.Z-rc.N"
   git tag vX.Y.Z-rc.N
   git push upstream release-X.Y --tags
   ```

   - For the official release run the following:

   ```sh
   git add manifests
   git commit -s -m "Kubeflow Trainer Official Release vX.Y.Z"
   git tag vX.Y.Z
   git push upstream release-X.Y --tags
   ```

### Verify the image publish

Check that all GitHub actions on your release branch is complete and images are published to the
registry. In case of failure, manually restart the GitHub actions.

For example, you can see the
[completed GitHub actions on the `v2.0.0-rc.1` release](https://github.com/kubeflow/trainer/commit/7122fc1a0f02e3d97b1da2a8eb31148e10b286c9)

### Update the changelog

1. Update the changelog by running:

   ```
   python docs/release/changelog.py --token=<github-token> --range=<previous-release>..<current-release>
   ```

   If you are creating the **first minor pre-release** or the **minor** release (`X.Y`), your
   `previous-release` is equal to the latest release on the `release-X.Y-1` branch.
   For example: `--range=v2.0.1..v2.1.0`

   Otherwise, your `previous-release` is equal to the latest release on the `release-X.Y` branch.
   For example: `--range=v2.0.0-rc.0..v2.0.0-rc.1`

   Group PRs in the changelog into features, bug fixes, misc, etc.

   Check this example: [v2.0.0-rc.0](https://github.com/kubeflow/trainer/blob/master/CHANGELOG.md#v200-rc0-2025-06-10)

   Finally, submit a PR with the updated changelog.

### Create GitHub Release

After the changelog PR is merged, create the GitHub release using your tag.

Set as a pre-release for the release candidates (e.g. RC.1) or set as the latest release for the
official releases.

For the GitHub release description you can use the same PR list as in changelog. You can use this
script to remove links from the GitHub user names (GitHub releases render the user names without
links):

```sh
perl -pi -e 's/\[@([^\]]+)\]\(https:\/\/github\.com\/\1\)/@\1/g' changelog.md
```

### Announce the new release

Post the announcement for the new Kubeflow Trainer RC or official release in:

- [#kubeflow-trainer](https://www.kubeflow.org/docs/about/community/#slack-channels) Slack channel.
- [kubeflow-discuss](https://www.kubeflow.org/docs/about/community/#kubeflow-mailing-list) mailing list.
