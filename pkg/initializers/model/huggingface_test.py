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
                "ignore_patterns": ["*.msgpack", "*.h5"],
                "access_token": "test_token",
            },
        ),
        (
            "Minimal config without token",
            {"storage_uri": "hf://model/path"},
            {
                "storage_uri": "hf://model/path",
                "ignore_patterns": ["*.msgpack", "*.h5"],
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
            "Successful download with safetensors (preferred path)",
            {
                "config": {
                    "storage_uri": "hf://username/model-name",
                    "ignore_patterns": ["*.msgpack", "*.h5"],
                    "access_token": "test_token",
                },
                "should_login": True,
                "expected_repo_id": "username/model-name",
                "has_safetensors": True,
                "has_pytorch": False,
            },
        ),
        (
            "Fallback to PyTorch weights when safetensors unavailable",
            {
                "config": {
                    "storage_uri": "hf://org/model-v1",
                    "ignore_patterns": ["*.msgpack", "*.h5"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "org/model-v1",
                "has_safetensors": False,
                "has_pytorch": True,
            },
        ),
    ],
)
def test_download_model(test_name, test_case):
    """Test model download with safetensors-first fallback logic"""

    print(f"Running test: {test_name}")

    huggingface_model_instance = HuggingFace()
    huggingface_model_instance.config = MagicMock(**test_case["config"])

    with patch("huggingface_hub.login") as mock_login, patch(
        "huggingface_hub.snapshot_download"
    ) as mock_download, patch("os.path.exists") as mock_exists, patch(
        "os.walk"
    ) as mock_walk:

        # Mock file system based on test case
        if test_case["has_safetensors"]:
            # Simulate safetensors files exist
            mock_walk.return_value = [
                (utils.MODEL_PATH, [], ["model.safetensors", "config.json"])
            ]
        elif test_case["has_pytorch"]:
            # Simulate only PyTorch files exist after fallback
            def walk_side_effect(path):
                # First call (safetensors check) returns no weights
                if mock_walk.call_count == 1:
                    return [(utils.MODEL_PATH, [], ["config.json"])]
                # Second call (pytorch check) returns weights
                return [(utils.MODEL_PATH, [], ["pytorch_model.bin", "config.json"])]

            mock_walk.side_effect = walk_side_effect

        mock_exists.return_value = True

        # Execute download
        huggingface_model_instance.download_model()

        # Verify login behavior
        if test_case["should_login"]:
            mock_login.assert_called_once_with(test_case["config"]["access_token"])
        else:
            mock_login.assert_not_called()

        # Verify download was called
        assert mock_download.called

        if test_case["has_safetensors"]:
            # Should only call snapshot_download once with safetensors patterns
            assert mock_download.call_count == 1
            call_args = mock_download.call_args
            assert call_args[1]["allow_patterns"] == [
                "*.json",
                "*.safetensors",
                "*.model",
                "*.txt",
                "*.gguf",
            ]
        elif test_case["has_pytorch"]:
            # Should call snapshot_download twice (safetensors fails, pytorch succeeds)
            assert mock_download.call_count == 2
            # Second call should use pytorch patterns
            call_args = mock_download.call_args_list[1]
            assert call_args[1]["allow_patterns"] == [
                "*.json",
                "*.bin",
                "*.pt",
                "*.pth",
                "*.model",
                "*.txt",
                "*.gguf",
            ]

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Detect safetensors weight files",
            {
                "file_list": ["model.safetensors", "config.json", "tokenizer.json"],
                "extensions": [".safetensors"],
                "expected": True,
            },
        ),
        (
            "Detect PyTorch weight files",
            {
                "file_list": ["pytorch_model.bin", "config.json"],
                "extensions": [".bin", ".pt", ".pth"],
                "expected": True,
            },
        ),
        (
            "No weight files found",
            {
                "file_list": ["config.json", "tokenizer.json", "README.md"],
                "extensions": [".safetensors"],
                "expected": False,
            },
        ),
        (
            "Multiple weight file types",
            {
                "file_list": ["model-00001.safetensors", "model-00002.safetensors"],
                "extensions": [".safetensors", ".bin"],
                "expected": True,
            },
        ),
    ],
)
def test_has_weight_files(test_name, test_case):
    """Test weight file detection helper method"""

    print(f"Running test: {test_name}")

    huggingface_model_instance = HuggingFace()

    with patch("os.walk") as mock_walk, patch("os.path.exists", return_value=True):
        mock_walk.return_value = [(utils.MODEL_PATH, [], test_case["file_list"])]

        result = huggingface_model_instance._has_weight_files(test_case["extensions"])
        assert result == test_case["expected"]

    print("Test execution completed")


def test_download_model_no_weights_found():
    """Test error handling when no weight files found after both attempts"""

    print("Running test: No weights found error")

    huggingface_model_instance = HuggingFace()
    huggingface_model_instance.config = MagicMock(
        storage_uri="hf://username/model-name",
        ignore_patterns=["*.msgpack", "*.h5"],
        access_token=None,
    )

    with patch("huggingface_hub.login"), patch(
        "huggingface_hub.snapshot_download"
    ) as mock_download, patch("os.walk") as mock_walk, patch(
        "os.path.exists", return_value=True
    ):

        # Simulate no weight files found in both attempts
        mock_walk.return_value = [(utils.MODEL_PATH, [], ["config.json", "README.md"])]

        with pytest.raises(
            RuntimeError, match="Failed to download model username/model-name"
        ):
            huggingface_model_instance.download_model()

        # Should have tried both download attempts
        assert mock_download.call_count == 2

    print("Test execution completed")


def test_download_model_complete_failure():
    """Test error handling when snapshot_download fails completely"""

    print("Running test: Complete download failure")

    huggingface_model_instance = HuggingFace()
    huggingface_model_instance.config = MagicMock(
        storage_uri="hf://username/nonexistent-model",
        ignore_patterns=["*.msgpack", "*.h5"],
        access_token=None,
    )

    with patch("huggingface_hub.login"), patch(
        "huggingface_hub.snapshot_download",
        side_effect=Exception("Repository not found"),
    ):

        with pytest.raises(Exception, match="Repository not found"):
            huggingface_model_instance.download_model()

    print("Test execution completed")
