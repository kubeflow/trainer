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


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful download with token",
            {
                "config": {
                    "storage_uri": "hf://username/model-name",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": "test_token",
                },
                "should_login": True,
                "expected_repo_id": "username/model-name",
            },
        ),
        (
            "Successful download without token",
            {
                "config": {
                    "storage_uri": "hf://org/model-v1",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "org/model-v1",
            },
        ),
        (
            "Model id without a namespace",
            {
                "config": {
                    "storage_uri": "hf://gpt2",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "gpt2",
            },
        ),
        (
            "Model id without a namespace and a trailing slash",
            {
                "config": {
                    "storage_uri": "hf://gpt2/",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "gpt2",
            },
        ),
        (
            "Trailing slash is ignored",
            {
                "config": {
                    "storage_uri": "hf://meta-llama/Llama-3.2-1B/",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "meta-llama/Llama-3.2-1B",
            },
        ),
        (
            "Extra path segments are ignored",
            {
                "config": {
                    "storage_uri": "hf://Qwen/Qwen2.5-1.5B-Instruct/tree/main",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "Qwen/Qwen2.5-1.5B-Instruct",
            },
        ),
        (
            "Empty model id is rejected",
            {
                "config": {
                    "storage_uri": "hf://",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": "test_token",
                },
                "expected_error": ValueError,
            },
        ),
        (
            "Empty model id with extra slashes is rejected",
            {
                "config": {
                    "storage_uri": "hf:///",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                    "access_token": None,
                },
                "expected_error": ValueError,
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
    ) as mock_download:

        if "expected_error" in test_case:
            with pytest.raises(
                test_case["expected_error"], match="Invalid HuggingFace model URI"
            ):
                huggingface_model_instance.download_model()
            mock_login.assert_not_called()
            mock_download.assert_not_called()
            return

        # Execute download
        huggingface_model_instance.download_model()

        # Verify login behavior
        if test_case["should_login"]:
            mock_login.assert_called_once_with(test_case["config"]["access_token"])
        else:
            mock_login.assert_not_called()

        # Verify download parameters
        mock_download.assert_called_once_with(
            repo_id=test_case["expected_repo_id"],
            local_dir=utils.MODEL_PATH,
            allow_patterns=["*.json", "*.safetensors", "*.model", "*.txt", "*.gguf"],
            ignore_patterns=test_case["config"]["ignore_patterns"],
        )
    print("Test execution completed")
