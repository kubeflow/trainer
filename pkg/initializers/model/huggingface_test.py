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
from pkg.initializers.model.huggingface import HuggingFace


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with token",
            {"storage_uri": "hf://model/path", "access_token": "test_token"},
            {
                "storage_uri": "hf://model/path",
                "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                "access_token": "test_token",
            },
        ),
        (
            "Minimal config without token",
            {"storage_uri": "hf://model/path"},
            {
                "storage_uri": "hf://model/path",
                "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                "access_token": None,
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    huggingface_model_instance = HuggingFace()
    with patch.object(utils, "get_config_from_env", return_value=test_config):
        huggingface_model_instance.load_config()
        assert huggingface_model_instance.config.__dict__ == expected

    print("Test execution completed")


DEFAULT_ALLOW_PATTERNS = ["*.json", "*.safetensors", "*.model", "*.txt"]
DEFAULT_IGNORE_PATTERNS = ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"]


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful download with token, repo has safetensors",
            {
                "config": {
                    "storage_uri": "hf://username/model-name",
                    "ignore_patterns": DEFAULT_IGNORE_PATTERNS,
                    "access_token": "test_token",
                },
                "should_login": True,
                "expected_repo_id": "username/model-name",
                "repo_files": ["config.json", "model.safetensors"],
                "expect_list_repo_files": True,
                "expected_allow_patterns": DEFAULT_ALLOW_PATTERNS,
                "expected_ignore_patterns": DEFAULT_IGNORE_PATTERNS,
            },
        ),
        (
            "Successful download without token, repo has safetensors",
            {
                "config": {
                    "storage_uri": "hf://org/model-v1",
                    "ignore_patterns": DEFAULT_IGNORE_PATTERNS,
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "org/model-v1",
                "repo_files": ["config.json", "model.safetensors"],
                "expect_list_repo_files": True,
                "expected_allow_patterns": DEFAULT_ALLOW_PATTERNS,
                "expected_ignore_patterns": DEFAULT_IGNORE_PATTERNS,
            },
        ),
        (
            "Repo has no safetensors, falls back to legacy weights",
            {
                "config": {
                    "storage_uri": "hf://org/legacy-model",
                    "ignore_patterns": DEFAULT_IGNORE_PATTERNS,
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "org/legacy-model",
                "repo_files": ["config.json", "pytorch_model.bin"],
                "expect_list_repo_files": True,
                "expected_allow_patterns": DEFAULT_ALLOW_PATTERNS
                + ["*.bin", "*.pt", "*.pth"],
                "expected_ignore_patterns": ["*.msgpack", "*.h5"],
            },
        ),
        (
            "Custom ignore_patterns without legacy formats skips repo listing",
            {
                "config": {
                    "storage_uri": "hf://org/custom-model",
                    "ignore_patterns": ["*.msgpack"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "org/custom-model",
                "repo_files": [],
                "expect_list_repo_files": False,
                "expected_allow_patterns": DEFAULT_ALLOW_PATTERNS,
                "expected_ignore_patterns": ["*.msgpack"],
            },
        ),
    ],
)
def test_download_model(test_name, test_case):
    """Test model download with different configurations"""

    print(f"Running test: {test_name}")

    huggingface_model_instance = HuggingFace()
    huggingface_model_instance.config = MagicMock(**test_case["config"])

    with patch("huggingface_hub.login") as mock_login, patch(
        "huggingface_hub.snapshot_download"
    ) as mock_download, patch(
        "huggingface_hub.list_repo_files", return_value=test_case["repo_files"]
    ) as mock_list_files:

        # Execute download
        huggingface_model_instance.download_model()

        # Verify login behavior
        if test_case["should_login"]:
            mock_login.assert_called_once_with(test_case["config"]["access_token"])
        else:
            mock_login.assert_not_called()

        # Verify we only pay for the repo listing call when we actually need
        # to decide on legacy weight formats
        if test_case["expect_list_repo_files"]:
            mock_list_files.assert_called_once_with(test_case["expected_repo_id"])
        else:
            mock_list_files.assert_not_called()

        # Verify download parameters
        mock_download.assert_called_once_with(
            repo_id=test_case["expected_repo_id"],
            local_dir=utils.MODEL_PATH,
            allow_patterns=test_case["expected_allow_patterns"],
            ignore_patterns=test_case["expected_ignore_patterns"],
        )
    print("Test execution completed")
