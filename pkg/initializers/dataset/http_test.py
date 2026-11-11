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

import io
import tarfile
import zipfile
from unittest.mock import MagicMock, Mock, mock_open, patch

import pytest
import requests

import pkg.initializers.utils.utils as utils
from pkg.initializers.dataset.http import HTTP


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with headers and timeout",
            {
                "storage_uri": "https://example.com/dataset.tar.gz",
                "headers": {"Authorization": "Bearer token123"},
                "timeout": 600,
            },
            {
                "storage_uri": "https://example.com/dataset.tar.gz",
                "headers": {"Authorization": "Bearer token123"},
                "timeout": 600,
                "ignore_patterns": None,
            },
        ),
        (
            "Minimal config with defaults",
            {"storage_uri": "https://example.com/data.csv"},
            {
                "storage_uri": "https://example.com/data.csv",
                "headers": None,
                "timeout": 300,
                "ignore_patterns": None,
            },
        ),
        (
            "Config with custom ignore patterns",
            {
                "storage_uri": "http://localhost/dataset.zip",
                "ignore_patterns": ["*.tmp", "*.log"],
            },
            {
                "storage_uri": "http://localhost/dataset.zip",
                "headers": None,
                "timeout": 300,
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
            "Download tar.gz archive",
            {
                "config": {
                    "storage_uri": "https://example.com/imagenet.tar.gz",
                    "headers": None,
                    "timeout": 300,
                },
                "filename": "imagenet.tar.gz",
                "content_type": "application/gzip",
                "is_archive": True,
                "archive_type": "tar.gz",
            },
        ),
        (
            "Download zip archive",
            {
                "config": {
                    "storage_uri": "https://example.com/mnist.zip",
                    "headers": None,
                    "timeout": 300,
                },
                "filename": "mnist.zip",
                "content_type": "application/zip",
                "is_archive": True,
                "archive_type": "zip",
            },
        ),
        (
            "Download single CSV file",
            {
                "config": {
                    "storage_uri": "https://data.example.com/train.csv",
                    "headers": {"API-Key": "secret"},
                    "timeout": 600,
                },
                "filename": "train.csv",
                "content_type": "text/csv",
                "is_archive": False,
                "archive_type": None,
            },
        ),
    ],
)
def test_download_dataset(test_name, test_case):
    """Test dataset download with different file types"""

    print(f"Running test: {test_name}")

    http_dataset_instance = HTTP()
    http_dataset_instance.config = MagicMock(**test_case["config"])

    # Mock response
    mock_response = Mock()
    mock_response.status_code = 200
    mock_response.headers = {
        "content-type": test_case["content_type"],
        "content-length": "2048",
    }

    # Create mock file content
    if test_case["is_archive"]:
        if test_case["archive_type"] == "tar.gz":
            tar_buffer = io.BytesIO()
            with tarfile.open(fileobj=tar_buffer, mode="w:gz") as tar:
                dummy_data = b"dataset content"
                dummy_file = tarfile.TarInfo(name="data.csv")
                dummy_file.size = len(dummy_data)
                tar.addfile(dummy_file, io.BytesIO(dummy_data))
            mock_response.iter_content = lambda chunk_size: [tar_buffer.getvalue()]
        elif test_case["archive_type"] == "zip":
            zip_buffer = io.BytesIO()
            with zipfile.ZipFile(zip_buffer, "w") as zf:
                zf.writestr("data.csv", b"dataset content")
            mock_response.iter_content = lambda chunk_size: [zip_buffer.getvalue()]
    else:
        mock_response.iter_content = lambda chunk_size: [b"csv,data,here"]

    with patch("requests.get", return_value=mock_response) as mock_get, patch(
        "os.remove"
    ) as mock_remove, patch("tarfile.open") as mock_tar, patch(
        "zipfile.ZipFile"
    ) as mock_zip, patch(
        "builtins.open", mock_open()
    ), patch(
        "pathlib.Path.mkdir"
    ):

        # Execute download
        http_dataset_instance.download_dataset()

        # Verify HTTP GET was called correctly
        expected_headers = test_case["config"]["headers"] or {}
        mock_get.assert_called_once_with(
            test_case["config"]["storage_uri"],
            stream=True,
            headers=expected_headers,
            timeout=test_case["config"]["timeout"],
        )

        # Verify archive extraction if applicable
        if test_case["is_archive"]:
            if test_case["archive_type"] == "tar.gz":
                mock_tar.assert_called_once()
            elif test_case["archive_type"] == "zip":
                mock_zip.assert_called_once()
            mock_remove.assert_called_once()

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, status_code",
    [
        ("HTTP 404 Not Found", 404),
        ("HTTP 500 Server Error", 500),
        ("HTTP 403 Forbidden", 403),
    ],
)
def test_download_dataset_http_errors(test_name, status_code):
    """Test handling of HTTP errors"""

    print(f"Running test: {test_name}")

    http_dataset_instance = HTTP()
    http_dataset_instance.config = MagicMock(
        storage_uri="https://example.com/dataset.zip",
        headers=None,
        timeout=300,
    )

    mock_response = Mock()
    mock_response.status_code = status_code
    mock_response.raise_for_status.side_effect = requests.HTTPError(
        f"HTTP {status_code}"
    )

    with patch("requests.get", return_value=mock_response):
        with pytest.raises(requests.HTTPError):
            http_dataset_instance.download_dataset()

    print("Test execution completed")


def test_download_dataset_timeout():
    """Test handling of request timeout"""

    print("Running test: Dataset download timeout")

    http_dataset_instance = HTTP()
    http_dataset_instance.config = MagicMock(
        storage_uri="https://slow-server.com/large-dataset.tar.gz",
        headers=None,
        timeout=1,
    )

    with patch("requests.get", side_effect=requests.Timeout("Connection timeout")):
        with pytest.raises(requests.Timeout):
            http_dataset_instance.download_dataset()

    print("Test execution completed")


def test_get_filename_from_url():
    """Test filename extraction from URL"""

    print("Running test: Dataset filename extraction")

    http_dataset_instance = HTTP()

    # Test URL with clear filename
    mock_response = Mock()
    mock_response.headers = {}
    filename = http_dataset_instance._get_filename(
        "https://example.com/datasets/mnist-train.tar.gz", mock_response
    )
    assert filename == "mnist-train.tar.gz"

    # Test URL with Content-Disposition
    mock_response.headers = {
        "content-disposition": 'attachment; filename="imagenet-2024.zip"'
    }
    filename = http_dataset_instance._get_filename(
        "https://example.com/download?id=456", mock_response
    )
    assert filename == "imagenet-2024.zip"

    # Test URL without filename (fallback)
    mock_response.headers = {}
    filename = http_dataset_instance._get_filename(
        "https://example.com/", mock_response
    )
    assert filename == "dataset"  # Default fallback

    print("Test execution completed")
