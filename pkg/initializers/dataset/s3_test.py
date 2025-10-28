import os
import tempfile
from unittest.mock import MagicMock, patch

import pytest

import pkg.initializers.utils.utils as utils
from pkg.initializers.dataset.s3 import S3


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with credentials",
            {
                "storage_uri": "s3://dataset/path",
                "role_arn": "arn:aws:iam::123456789012:role/TestRole",
                "endpoint": "https://s3.amazonaws.com",
                "access_key_id": "test_access_key",
                "secret_access_key": "test_secret_key",
                "region": "us-east-1",
            },
            {
                "storage_uri": "s3://dataset/path",
                "ignore_patterns": None,
                "role_arn": "arn:aws:iam::123456789012:role/TestRole",
                "endpoint": "https://s3.amazonaws.com",
                "access_key_id": "test_access_key",
                "secret_access_key": "test_secret_key",
                "region": "us-east-1",
            },
        ),
        (
            "Minimal config without credentials",
            {"storage_uri": "s3://dataset/path"},
            {
                "storage_uri": "s3://dataset/path",
                "ignore_patterns": None,
                "endpoint": None,
                "access_key_id": None,
                "secret_access_key": None,
                "region": None,
                "role_arn": None,
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    s3_dataset_instance = S3()

    with patch.object(utils, "get_config_from_env", return_value=test_config):
        s3_dataset_instance.load_config()
        assert s3_dataset_instance.config.__dict__ == expected

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful download with credentials",
            {
                "config": {
                    "storage_uri": "s3://dataset/path/subpath",
                    "endpoint": "https://s3.amazonaws.com",
                    "access_key_id": "test_access_key",
                    "secret_access_key": "test_secret_key",
                    "region": "us-east-1",
                    "role_arn": "arn:aws:iam::123456789012:role/TestRole",
                    "ignore_patterns": None,
                },
                "expected_bucket": "dataset",
                "expected_prefix": "path/subpath",
            },
        ),
        (
            "Successful download without credentials",
            {
                "config": {
                    "storage_uri": "s3://dataset/path",
                    "endpoint": None,
                    "access_key_id": None,
                    "secret_access_key": None,
                    "region": None,
                    "role_arn": None,
                    "ignore_patterns": None,
                },
                "expected_bucket": "dataset",
                "expected_prefix": "path",
            },
        ),
    ],
)
def test_download_dataset(test_name, test_case):
    """Test dataset download with different configurations"""

    print(f"Running test: {test_name}")

    # Setup the S3 dataset instance
    s3_dataset_instance = S3()
    s3_dataset_instance.config = MagicMock(**test_case["config"])

    # Create a temporary directory for downloads
    with tempfile.TemporaryDirectory() as temp_dir:
        dataset_path = os.path.join(temp_dir, "dataset")

        mock_storage = MagicMock()

        with (
            patch(
                "pkg.initializers.utils.opendal.S3Storage", return_value=mock_storage
            ),
            patch.object(utils, "DATASET_PATH", dataset_path),
        ):
            s3_dataset_instance.download_dataset()

            # Verify S3Storage was created with correct parameters
            from pkg.initializers.utils.opendal import S3Storage

            S3Storage.assert_called_once_with(
                bucket=test_case["expected_bucket"],
                endpoint=test_case["config"]["endpoint"],
                access_key_id=test_case["config"]["access_key_id"],
                secret_access_key=test_case["config"]["secret_access_key"],
                region=test_case["config"]["region"],
                role_arn=test_case["config"]["role_arn"],
            )

            # Verify download was called with correct parameters
            mock_storage.download.assert_called_once_with(
                prefix=test_case["expected_prefix"],
                destination_path=dataset_path,
                ignore_patterns=test_case["config"]["ignore_patterns"],
            )

    print("Test execution completed")
