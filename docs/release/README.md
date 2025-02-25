# Releasing the Kubeflow Trainer

TODO (andreyvelich): Update this doc for Kubeflow Trainer V2.

## Prerequisite

- [Write](https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories/repository-permission-levels-for-an-organization#permission-levels-for-repositories-owned-by-an-organization)
  permission for the Training Operator repository.

- Maintainer access to the [Training SDK](https://pypi.org/project/kubeflow-training/).

- Create a [GitHub Token](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token).

- Install `PyGithub` to generate the [Changelog](./../../CHANGELOG.md):

  ```
  pip install PyGithub==1.55
  ```

- Install `twine` to publish the SDK package:

  ```
  pip install twine==3.4.1
  ```

  - Create a [PyPI Token](https://pypi.org/help/#apitoken) to publish Training SDK.

  - Add the following config to your `~/.pypirc` file:

    ```
    [pypi]
       username = __token__
       password = <PYPI_TOKEN>
    ```

## Versioning policy

Training Operator version format follows [Semantic Versioning](https://semver.org/).
Training Operator versions are in the format of `vX.Y.Z`, where `X` is the major version, `Y` is
the minor version, and `Z` is the patch version.
The patch version contains only bug fixes.

Additionally, Training Operator does pre-releases in this format: `vX.Y.Z-rc.N` where `N` is a number
of the `Nth` release candidate (RC) before an upcoming public release named `vX.Y.Z`.

## Release branches and tags

Training Operator releases are tagged with tags like `vX.Y.Z`, for example `v0.11.0`.

Release branches are in the format of `vX.Y-branch`, where `X.Y` stands for
the minor release.

`vX.Y.Z` releases are released from the `vX.Y-branch` branch. For example,
`v1.8.0` release should be on `v1.8-branch` branch.

If you want to push changes to the `vX.Y-branch` release branch, you have to
cherry pick your changes from the `master` branch and submit a PR.

## Create a new Training Operator release

### Create release branch

1. Depends on what version you want to release,

   - Major or Minor version - Use the GitHub UI to create a release branch from `master` and name
     the release branch `vX.Y-branch`
   - Patch version - You don't need to create a new release branch.

1. Fetch the upstream changes into your local directory:

   ```
   git fetch upstream
   ```

1. Checkout into the release branch:

   ```
   git checkout vX.Y-branch
   git rebase upstream/vX.Y-branch
   ```

### Release Training SDK

1. Update the the VERSION in the [gen-sdk.sh script](../../hack/python-sdk/gen-sdk.sh#L27),
   packageVersion in the [swagger_config.json file](../../hack/python-sdk/swagger_config.json#L4),
   and the version in the [setup.py file](../../sdk/python/setup.py#L36).

   You need to follow this semantic `X.Y.Zrc.N` for the RC or `X.Y.Z` for the public release.

   For example:

   ```
   VERSION=1.8.0rc1

   packageVersion: 1.8.0rc1

   version=1.8.0rc1
   ```

1. Generate the Training SDK:

   ```
   make generate
   ```

1. Publish the Training SDK to PyPI:

   ```
   cd sdk/python
   python3 setup.py sdist bdist_wheel
   twine upload dist/*
   ```

1. Submit a PR to update SDK version on release branch similar to [this one](https://github.com/kubeflow/training-operator/pull/2151).

   ```
   git push origin vX.Y-branch
   ```

### Release Training Operator Image

1. Wait until the above PR will be merged on release branch and check the commit SHA.
   For example, [`4485b0a`](https://github.com/kubeflow/training-operator/commit/4485b0aa3fa23a8b762af92bc36d46bfb063d6f5)

1. Rebase your local release branch:

   ```
   git fetch upstream
   git rebase upstream/vX.Y-branch
   ```

1. Update the Docker image tag for [standalone](../../manifests/overlays/standalone/kustomization.yaml#L9)
   and [Kubeflow](../../manifests/overlays/kubeflow/kustomization.yaml#L9) overlays.

   For example:

   ```yaml
   newTag: "v1-4485b0a"
   ```

1. Submit a PR to update image version on release branch similar to
   [this one](https://github.com/kubeflow/training-operator/pull/2152).

### Create GitHub Tag

1. After the above PR is merged, rebase your release branch and push new tag to upstream

   ```bash
   git fetch upstream
   git rebase upstream/vX.Y-branch
   ```

   - For the RC tag as follows:

     ```
     git tag vX.Y.Z-rc.N
     git push upstream vX.Y.Z-rc.N
     ```

   - For the official release tag as follows:

     ```
     git tag vX.Y.Z
     git push upstream vX.Y.Z
     ```

## Update Changelog

1. Update the Changelog by running:

   ```
   python docs/release/changelog.py --token=<github-token> --range=<previous-release>..<current-release>
   ```

   If you are creating the **first minor pre-release** or the **minor** release (`X.Y`), your
   `previous-release` is equal to the latest release on the `vX.Y-branch` branch.
   For example: `--range=v1.7.1..v1.8.0`

   Otherwise, your `previous-release` is equal to the latest release on the `vX.Y-branch` branch.
   For example: `--range=v1.7.0..v1.8.0-rc.0`

   Group PRs in the Changelog into Features, Bug fixes, Documentation, etc.
   Check this example: [v1.7.0-rc.0](https://github.com/kubeflow/training-operator/blob/master/CHANGELOG.md#v170-rc0-2023-07-07)

   Finally, submit a PR with the updated Changelog.
