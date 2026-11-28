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
from pkg.initializers.model.http import HTTP


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "HTTPS config with default ignore patterns",
            {
                "storage_uri": "https://example.com/model.safetensors",
            },
            {
                "storage_uri": "https://example.com/model.safetensors",
                "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
            },
        ),
        (
            "HTTP config with custom ignore patterns",
            {
                "storage_uri": "http://models.server.com/path/model.onnx",
                "ignore_patterns": ["*.tmp"],
            },
            {
                "storage_uri": "http://models.server.com/path/model.onnx",
                "ignore_patterns": ["*.tmp"],
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    http_model_instance = HTTP()

    with patch.object(utils, "get_config_from_env", return_value=test_config):
        http_model_instance.load_config()
        assert http_model_instance.config.__dict__ == expected

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "HTTPS model download",
            {
                "config": {
                    "storage_uri": "https://example.com/models/bert.safetensors",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                },
                "expected_base_url": "https://example.com",
                "expected_file_path": "models/bert.safetensors",
            },
        ),
        (
            "HTTP download from GitHub releases",
            {
                "config": {
                    "storage_uri": "https://github.com/org/repo/releases/download/v1/model.bin",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                },
                "expected_base_url": "https://github.com",
                "expected_file_path": "org/repo/releases/download/v1/model.bin",
            },
        ),
        (
            "HTTPS with custom port",
            {
                "config": {
                    "storage_uri": "https://server.local:9000/path/model.onnx",
                    "ignore_patterns": None,
                },
                "expected_base_url": "https://server.local:9000",
                "expected_file_path": "path/model.onnx",
            },
        ),
        (
            "URL with query parameters",
            {
                "config": {
                    "storage_uri": (
                        "https://models.example.com/bert.bin?" "version=2.0&format=onnx"
                    ),
                    "ignore_patterns": [
                        "*.msgpack",
                        "*.h5",
                        "*.bin",
                        "*.pt",
                        "*.pth",
                    ],
                },
                "expected_base_url": "https://models.example.com",
                "expected_file_path": "bert.bin?version=2.0&format=onnx",
            },
        ),
        (
            "URL with percent-encoded path",
            {
                "config": {
                    "storage_uri": "https://example.com/models%2Fbert%2Fweights.safetensors",
                    "ignore_patterns": ["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
                },
                "expected_base_url": "https://example.com",
                "expected_file_path": "models/bert/weights.safetensors",
            },
        ),
    ],
)
def test_download_model(test_name, test_case):
    """Test model download with different configurations"""

    print(f"Running test: {test_name}")

    http_model_instance = HTTP()
    http_model_instance.config = MagicMock(**test_case["config"])

    with patch(
        "pkg.initializers.model.http.opendal_utils.HTTPStorage"
    ) as mock_http_storage:
        mock_storage_instance = MagicMock()
        mock_http_storage.return_value = mock_storage_instance

        # Execute download
        http_model_instance.download_model()

        # Verify HTTPStorage was initialized with correct base URL
        mock_http_storage.assert_called_once_with(
            base_url=test_case["expected_base_url"]
        )

        # Verify download was called with correct parameters
        mock_storage_instance.download.assert_called_once_with(
            prefix=test_case["expected_file_path"],
            destination_path=utils.MODEL_PATH,
            ignore_patterns=test_case["config"]["ignore_patterns"],
        )

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, invalid_uri, expected_error",
    [
        (
            "Missing file path",
            "https://example.com",
            "Invalid HTTP URI",
        ),
        (
            "Only domain with trailing slash",
            "https://example.com/",
            "Invalid HTTP URI",
        ),
    ],
)
def test_invalid_uri(test_name, invalid_uri, expected_error):
    """Test error handling for invalid URIs"""

    print(f"Running test: {test_name}")

    http_model_instance = HTTP()
    http_model_instance.config = MagicMock(
        storage_uri=invalid_uri,
        ignore_patterns=["*.msgpack", "*.h5", "*.bin", "*.pt", "*.pth"],
    )

    with pytest.raises(ValueError, match=expected_error):
        http_model_instance.download_model()

    print("Test execution completed")
