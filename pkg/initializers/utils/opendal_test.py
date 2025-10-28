"""Tests for OpenDAL utilities."""

import os
import tempfile
from unittest.mock import MagicMock, Mock, patch

import pytest

from pkg.initializers.utils.opendal import S3Storage


class TestS3Storage:
    """Test suite for S3Storage class."""

    @pytest.mark.parametrize(
        "config,expected_region",
        [
            (
                {
                    "bucket": "test-bucket",
                    "endpoint": "https://s3.us-west-2.amazonaws.com",
                    "access_key_id": "test_key_id",
                    "secret_access_key": "test_secret_key",
                    "region": "us-west-2",
                    "role_arn": "arn:aws:iam::123456789012:role/TestRole",
                },
                "us-west-2",
            ),
            (
                {
                    "bucket": "prod-bucket",
                    "endpoint": "https://s3.eu-west-1.amazonaws.com",
                    "access_key_id": "prod_key",
                    "secret_access_key": "prod_secret",
                    "role_arn": "arn:aws:iam::123456789012:role/TestRole",
                },
                "auto",
            ),
        ],
    )
    def test_init(self, config, expected_region):
        """Test S3Storage initialization with various configurations."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator

            storage = S3Storage(**config)

            # Verify bucket is stored correctly
            assert storage.bucket == config["bucket"]

            # Verify Operator was called with correct parameters
            call_kwargs = mock_opendal.Operator.call_args[1]

            # Check required parameters
            assert call_kwargs["bucket"] == config["bucket"]
            assert call_kwargs["region"] == expected_region

            # Check optional parameters if provided
            for key in ["endpoint", "access_key_id", "secret_access_key"]:
                if key in config:
                    assert call_kwargs[key] == config[key]

    def _create_mock_entry(self, path: str, is_dir: bool = False) -> Mock:
        """Helper to create a mock S3 entry."""
        mock_entry = Mock()
        mock_entry.path = path
        mock_entry.metadata.is_dir = is_dir
        return mock_entry

    @pytest.mark.parametrize(
        "prefix,files,ignore_patterns",
        [
            (
                "dataset/",
                {
                    "dataset/train/data.csv": b"train data",
                    "dataset/test/data.csv": b"test data",
                    "dataset/README.md": b"readme content",
                },
                [],
            ),
            (
                "models/",
                {
                    "models/model.pth": b"model weights",
                    "models/config.json": b"config data",
                },
                [],
            ),
            (
                "data/",
                {
                    "data/file.txt": b"content",
                    "data/temp.log": b"log content",
                    "data/cache.tmp": b"cache content",
                },
                [".log", ".tmp"],
            ),
        ],
    )
    def test_download(self, prefix, files, ignore_patterns):
        """Test downloading files from S3."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator

            # Setup mock entries
            mock_entries = [self._create_mock_entry(path) for path in files.keys()]
            mock_operator.list.return_value = mock_entries

            # Setup read side effect
            mock_operator.read.side_effect = list(files.values())

            storage = S3Storage(bucket="test-bucket")

            with tempfile.TemporaryDirectory() as tmpdir:
                storage.download(
                    prefix=prefix,
                    destination_path=tmpdir,
                    ignore_patterns=ignore_patterns,
                )

                # Verify downloaded files
                for path, expected_content in files.items():
                    relative_path = path[len(prefix) :].lstrip("/")
                    full_path = os.path.join(tmpdir, relative_path)

                    # Check if file should be ignored
                    should_ignore = any(
                        path.endswith(pattern) for pattern in ignore_patterns
                    )

                    if should_ignore:
                        assert not os.path.exists(
                            full_path
                        ), f"File {relative_path} should have been ignored"
                    else:
                        assert os.path.exists(
                            full_path
                        ), f"Expected file {relative_path} not found"

                        with open(full_path, "rb") as f:
                            actual_content = f.read()
                            assert (
                                actual_content == expected_content
                            ), f"Content mismatch for {relative_path}"
