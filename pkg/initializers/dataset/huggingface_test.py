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
from pkg.initializers.dataset.huggingface import HuggingFace


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with token",
            {"storage_uri": "hf://dataset/path", "access_token": "test_token"},
            {
                "storage_uri": "hf://dataset/path",
                "ignore_patterns": None,
                "access_token": "test_token",
            },
        ),
        (
            "Minimal config without token",
            {"storage_uri": "hf://dataset/path"},
            {
                "storage_uri": "hf://dataset/path",
                "ignore_patterns": None,
                "access_token": None,
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    huggingface_dataset_instance = HuggingFace()

    with patch.object(utils, "get_config_from_env", return_value=test_config):
        huggingface_dataset_instance.load_config()
        assert huggingface_dataset_instance.config.__dict__ == expected

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful download with token",
            {
                "config": {
                    "storage_uri": "hf://username/dataset-name",
                    "ignore_patterns": None,
                    "access_token": "test_token",
                },
                "should_login": True,
                "expected_repo_id": "username/dataset-name",
            },
        ),
        (
            "Successful download without token",
            {
                "config": {
                    "storage_uri": "hf://org/dataset-v1",
                    "ignore_patterns": None,
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "org/dataset-v1",
            },
        ),
    ],
)
def test_download_dataset(test_name, test_case):
    """Test dataset download with different configurations"""

    print(f"Running test: {test_name}")

    huggingface_dataset_instance = HuggingFace()
    huggingface_dataset_instance.config = MagicMock(**test_case["config"])

    with patch("huggingface_hub.login") as mock_login, patch(
        "huggingface_hub.snapshot_download"
    ) as mock_download:

        # Execute download
        huggingface_dataset_instance.download_dataset()

        # Verify login behavior
        if test_case["should_login"]:
            mock_login.assert_called_once_with(test_case["config"]["access_token"])
        else:
            mock_login.assert_not_called()

        # Verify download parameters
        mock_download.assert_called_once_with(
            repo_id=test_case["expected_repo_id"],
            local_dir=utils.DATASET_PATH,
            repo_type="dataset",
            ignore_patterns=test_case["config"]["ignore_patterns"],
        )
    print("Test execution completed")
