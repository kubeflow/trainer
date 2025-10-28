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
                "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", ".pt", ".pth"],
                "access_token": "test_token",
            },
        ),
        (
            "Minimal config without token",
            {"storage_uri": "hf://model/path"},
            {
                "storage_uri": "hf://model/path",
                "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", ".pt", ".pth"],
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
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", ".pt", ".pth"],
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
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", ".pt", ".pth"],
                    "access_token": None,
                },
                "should_login": False,
                "expected_repo_id": "org/model-v1",
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
            allow_patterns=["*.json", "*.safetensors", "*.model", "*.txt"],
            ignore_patterns=test_case["config"]["ignore_patterns"],
        )
    print("Test execution completed")
