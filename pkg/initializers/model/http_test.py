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
from pkg.initializers.model.http import HTTP


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with headers and timeout",
            {
                "storage_uri": "https://example.com/model.tar.gz",
                "headers": {"Authorization": "Bearer token123"},
                "timeout": 600,
            },
            {
                "storage_uri": "https://example.com/model.tar.gz",
                "headers": {"Authorization": "Bearer token123"},
                "timeout": 600,
                "ignore_patterns": ["*.msgpack", "*.h5"],
            },
        ),
        (
            "Minimal config with defaults",
            {"storage_uri": "https://example.com/model.bin"},
            {
                "storage_uri": "https://example.com/model.bin",
                "headers": None,
                "timeout": 300,
                "ignore_patterns": ["*.msgpack", "*.h5"],
            },
        ),
        (
            "Config with custom ignore patterns",
            {
                "storage_uri": "http://localhost/model.tar.gz",
                "ignore_patterns": ["*.log", "*.tmp"],
            },
            {
                "storage_uri": "http://localhost/model.tar.gz",
                "headers": None,
                "timeout": 300,
                "ignore_patterns": ["*.log", "*.tmp"],
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
            "Download tar.gz archive",
            {
                "config": {
                    "storage_uri": "https://example.com/model.tar.gz",
                    "headers": None,
                    "timeout": 300,
                },
                "filename": "model.tar.gz",
                "content_type": "application/gzip",
                "is_archive": True,
                "archive_type": "tar.gz",
            },
        ),
        (
            "Download zip archive",
            {
                "config": {
                    "storage_uri": "https://example.com/dataset.zip",
                    "headers": None,
                    "timeout": 300,
                },
                "filename": "dataset.zip",
                "content_type": "application/zip",
                "is_archive": True,
                "archive_type": "zip",
            },
        ),
        (
            "Download single file with headers",
            {
                "config": {
                    "storage_uri": "https://api.example.com/model.safetensors",
                    "headers": {"Authorization": "Bearer token123"},
                    "timeout": 600,
                },
                "filename": "model.safetensors",
                "content_type": "application/octet-stream",
                "is_archive": False,
                "archive_type": None,
            },
        ),
        (
            "Download from Content-Disposition header",
            {
                "config": {
                    "storage_uri": "https://example.com/download?id=123",
                    "headers": None,
                    "timeout": 300,
                },
                "filename": "bert-model.tar.gz",
                "content_type": "application/gzip",
                "content_disposition": 'attachment; filename="bert-model.tar.gz"',
                "is_archive": True,
                "archive_type": "tar.gz",
            },
        ),
    ],
)
def test_download_model(test_name, test_case):
    """Test model download with different file types"""

    print(f"Running test: {test_name}")

    http_model_instance = HTTP()
    http_model_instance.config = MagicMock(**test_case["config"])

    # Mock response
    mock_response = Mock()
    mock_response.status_code = 200
    mock_response.headers = {
        "content-type": test_case["content_type"],
        "content-length": "1024",
    }

    if "content_disposition" in test_case:
        mock_response.headers["content-disposition"] = test_case["content_disposition"]

    # Create mock file content
    if test_case["is_archive"]:
        if test_case["archive_type"] == "tar.gz":
            # Create in-memory tar.gz
            tar_buffer = io.BytesIO()
            with tarfile.open(fileobj=tar_buffer, mode="w:gz") as tar:
                # Add a dummy file
                dummy_data = b"model weights data"
                dummy_file = tarfile.TarInfo(name="model.bin")
                dummy_file.size = len(dummy_data)
                tar.addfile(dummy_file, io.BytesIO(dummy_data))
            mock_response.iter_content = lambda chunk_size: [tar_buffer.getvalue()]
        elif test_case["archive_type"] == "zip":
            # Create in-memory zip
            zip_buffer = io.BytesIO()
            with zipfile.ZipFile(zip_buffer, "w") as zf:
                zf.writestr("model.bin", b"model weights data")
            mock_response.iter_content = lambda chunk_size: [zip_buffer.getvalue()]
    else:
        # Single file
        mock_response.iter_content = lambda chunk_size: [b"model data chunk"]

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
        http_model_instance.download_model()

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
            # Archive file should be removed after extraction
            mock_remove.assert_called_once()

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, status_code, exception_type",
    [
        ("HTTP 404 Not Found", 404, requests.HTTPError),
        ("HTTP 500 Server Error", 500, requests.HTTPError),
        ("HTTP 403 Forbidden", 403, requests.HTTPError),
    ],
)
def test_download_model_http_errors(test_name, status_code, exception_type):
    """Test handling of HTTP errors"""

    print(f"Running test: {test_name}")

    http_model_instance = HTTP()
    http_model_instance.config = MagicMock(
        storage_uri="https://example.com/model.tar.gz",
        headers=None,
        timeout=300,
    )

    # Mock error response
    mock_response = Mock()
    mock_response.status_code = status_code
    mock_response.raise_for_status.side_effect = requests.HTTPError(
        f"HTTP {status_code}"
    )

    with patch("requests.get", return_value=mock_response):
        # Should raise HTTPError
        with pytest.raises(exception_type):
            http_model_instance.download_model()

    print("Test execution completed")


def test_download_model_timeout():
    """Test handling of request timeout"""

    print("Running test: Request timeout")

    http_model_instance = HTTP()
    http_model_instance.config = MagicMock(
        storage_uri="https://slow-server.com/model.tar.gz",
        headers=None,
        timeout=1,
    )

    # Mock timeout
    with patch("requests.get", side_effect=requests.Timeout("Connection timeout")):
        with pytest.raises(requests.Timeout):
            http_model_instance.download_model()

    print("Test execution completed")


def test_get_filename_from_url():
    """Test filename extraction from URL"""

    print("Running test: Filename extraction")

    http_model_instance = HTTP()

    # Test URL with clear filename
    mock_response = Mock()
    mock_response.headers = {}
    filename = http_model_instance._get_filename(
        "https://example.com/models/bert-base.tar.gz", mock_response
    )
    assert filename == "bert-base.tar.gz"

    # Test URL with Content-Disposition
    mock_response.headers = {
        "content-disposition": 'attachment; filename="custom-name.bin"'
    }
    filename = http_model_instance._get_filename(
        "https://example.com/download?id=123", mock_response
    )
    assert filename == "custom-name.bin"

    # Test URL without filename (fallback)
    mock_response.headers = {}
    filename = http_model_instance._get_filename("https://example.com/", mock_response)
    assert filename == "model"  # Default fallback

    print("Test execution completed")
