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
from pkg.initializers.dataset.http import HTTP


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "HTTPS config",
            {
                "storage_uri": "https://example.com/dataset.zip",
            },
            {
                "storage_uri": "https://example.com/dataset.zip",
                "ignore_patterns": None,
            },
        ),
        (
            "HTTP config with ignore patterns",
            {
                "storage_uri": "http://data.server.com/path/data.tar.gz",
                "ignore_patterns": ["*.tmp", "*.log"],
            },
            {
                "storage_uri": "http://data.server.com/path/data.tar.gz",
                "ignore_patterns": ["*.tmp", "*.log"],
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    http_dataset_instance = HTTP()

    with patch.object(utils, "get_config_from_env", return_value=test_config):
        http_dataset_instance.load_config()
        assert http_dataset_instance.config.__dict__ == expected

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "HTTPS download",
            {
                "config": {
                    "storage_uri": "https://example.com/models/dataset.zip",
                    "ignore_patterns": None,
                },
                "expected_base_url": "https://example.com",
                "expected_file_path": "models/dataset.zip",
            },
        ),
        (
            "HTTP download from subdirectory",
            {
                "config": {
                    "storage_uri": "http://data.server.com/path/to/data.tar.gz",
                    "ignore_patterns": None,
                },
                "expected_base_url": "http://data.server.com",
                "expected_file_path": "path/to/data.tar.gz",
            },
        ),
        (
            "HTTPS with port number",
            {
                "config": {
                    "storage_uri": "https://server.com:8080/data/file.zip",
                    "ignore_patterns": ["*.tmp"],
                },
                "expected_base_url": "https://server.com:8080",
                "expected_file_path": "data/file.zip",
            },
        ),
    ],
)
def test_download_dataset(test_name, test_case):
    """Test dataset download with different configurations"""

    print(f"Running test: {test_name}")

    http_dataset_instance = HTTP()
    http_dataset_instance.config = MagicMock(**test_case["config"])

    with patch(
        "pkg.initializers.dataset.http.opendal_utils.HTTPStorage"
    ) as mock_http_storage:
        mock_storage_instance = MagicMock()
        mock_http_storage.return_value = mock_storage_instance

        # Execute download
        http_dataset_instance.download_dataset()

        # Verify HTTPStorage was initialized with correct base URL
        mock_http_storage.assert_called_once_with(
            base_url=test_case["expected_base_url"]
        )

        # Verify download was called with correct parameters
        mock_storage_instance.download.assert_called_once_with(
            prefix=test_case["expected_file_path"],
            destination_path=utils.DATASET_PATH,
            ignore_patterns=test_case["config"]["ignore_patterns"],
        )

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, invalid_uri, expected_error",
    [
        (
            "Missing path",
            "https://example.com",
            "Invalid HTTP URI",
        ),
        (
            "Missing path with trailing slash",
            "https://example.com/",
            "Invalid HTTP URI",
        ),
    ],
)
def test_invalid_uri(test_name, invalid_uri, expected_error):
    """Test error handling for invalid URIs"""

    print(f"Running test: {test_name}")

    http_dataset_instance = HTTP()
    http_dataset_instance.config = MagicMock(
        storage_uri=invalid_uri, ignore_patterns=None
    )

    with pytest.raises(ValueError, match=expected_error):
        http_dataset_instance.download_dataset()

    print("Test execution completed")
