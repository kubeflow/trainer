# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from unittest.mock import MagicMock, patch

import pytest

import pkg.initializers.utils.utils as utils
from pkg.initializers.dataset.kaggle import Kaggle


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with token",
            {
                "storage_uri": "kaggle://datasets/username/dataset-name",
                "access_token": "test_token",
            },
            {
                "storage_uri": "kaggle://datasets/username/dataset-name",
                "access_token": "test_token",
            },
        ),
        (
            "Minimal config without token",
            {
                "storage_uri": "kaggle://datasets/user/data",
            },
            {
                "storage_uri": "kaggle://datasets/user/data",
                "access_token": None,
            },
        ),
        (
            "Competition config",
            {
                "storage_uri": "kaggle://competitions/digit-recognizer",
                "access_token": "comp_token",
            },
            {
                "storage_uri": "kaggle://competitions/digit-recognizer",
                "access_token": "comp_token",
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    kaggle_dataset_instance = Kaggle()
    with patch.object(utils, "get_config_from_env", return_value=test_config):
        kaggle_dataset_instance.load_config()
        assert kaggle_dataset_instance.config.__dict__ == expected

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful dataset download with token",
            {
                "config": {
                    "storage_uri": "kaggle://datasets/username/spotify-recommendation",
                    "access_token": "test_token",
                },
                "resource_type": "datasets",
                "should_set_token": True,
                "expected_handle": "username/spotify-recommendation",
            },
        ),
        (
            "Successful dataset download without token",
            {
                "config": {
                    "storage_uri": "kaggle://datasets/org/public-data",
                    "access_token": None,
                },
                "resource_type": "datasets",
                "should_set_token": False,
                "expected_handle": "org/public-data",
            },
        ),
        (
            "Dataset with version",
            {
                "config": {
                    "storage_uri": "kaggle://datasets/user/dataset/versions/1",
                    "access_token": "key123",
                },
                "resource_type": "datasets",
                "should_set_token": True,
                "expected_handle": "user/dataset/versions/1",
            },
        ),
    ],
)
def test_download_dataset(test_name, test_case):
    """Test dataset download with different configurations"""

    print(f"Running test: {test_name}")

    kaggle_dataset_instance = Kaggle()
    kaggle_dataset_instance.config = MagicMock(**test_case["config"])

    with patch(
        "pkg.initializers.dataset.kaggle.kagglehub.dataset_download"
    ) as mock_dataset_download, patch(
        "pkg.initializers.dataset.kaggle.kagglehub.competition_download"
    ) as mock_competition_download, patch.dict(
        "os.environ", {}, clear=True
    ):

        # Execute download
        kaggle_dataset_instance.download_dataset()

        # Verify correct download function was called
        if test_case["resource_type"] == "datasets":
            mock_dataset_download.assert_called_once_with(
                handle=test_case["expected_handle"],
                output_dir=utils.DATASET_PATH,
            )
            mock_competition_download.assert_not_called()

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful competition download with token",
            {
                "config": {
                    "storage_uri": "kaggle://competitions/digit-recognizer",
                    "access_token": "comp_token",
                },
                "resource_type": "competitions",
                "should_set_token": True,
                "expected_competition": "digit-recognizer",
            },
        ),
        (
            "Successful competition download without token",
            {
                "config": {
                    "storage_uri": "kaggle://competitions/titanic",
                    "access_token": None,
                },
                "resource_type": "competitions",
                "should_set_token": False,
                "expected_competition": "titanic",
            },
        ),
    ],
)
def test_download_competition(test_name, test_case):
    """Test competition download with different configurations"""

    print(f"Running test: {test_name}")

    kaggle_dataset_instance = Kaggle()
    kaggle_dataset_instance.config = MagicMock(**test_case["config"])

    with patch(
        "pkg.initializers.dataset.kaggle.kagglehub.dataset_download"
    ) as mock_dataset_download, patch(
        "pkg.initializers.dataset.kaggle.kagglehub.competition_download"
    ) as mock_competition_download, patch.dict(
        "os.environ", {}, clear=True
    ):

        # Execute download
        kaggle_dataset_instance.download_dataset()

        # Verify correct download function was called
        if test_case["resource_type"] == "competitions":
            mock_competition_download.assert_called_once_with(
                handle=test_case["expected_competition"],
                output_dir=utils.DATASET_PATH,
            )
            mock_dataset_download.assert_not_called()

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, invalid_uri, expected_error",
    [
        (
            "Missing dataset name",
            "kaggle://datasets/username",
            "Invalid Kaggle dataset URI",
        ),
        (
            "Invalid resource type",
            "kaggle://models/username/model",
            "Invalid Kaggle resource type",
        ),
        (
            "Empty path",
            "kaggle://datasets",
            "Invalid Kaggle dataset URI",
        ),
        (
            "Unknown resource type",
            "kaggle://notebooks/username/notebook",
            "Invalid Kaggle resource type",
        ),
    ],
)
def test_invalid_uri(test_name, invalid_uri, expected_error):
    """Test error handling for invalid URIs"""

    print(f"Running test: {test_name}")

    kaggle_dataset_instance = Kaggle()
    kaggle_dataset_instance.config = MagicMock(
        storage_uri=invalid_uri, access_token=None
    )

    with pytest.raises(ValueError, match=expected_error):
        kaggle_dataset_instance.download_dataset()

    print("Test execution completed")
