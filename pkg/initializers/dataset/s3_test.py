import os
import tempfile
from unittest.mock import MagicMock, patch

import boto3
import pytest
from moto import mock_aws

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
                "endpoint_url": "https://s3.amazonaws.com",
                "access_key_id": "test_access_key",
                "access_key_secret": "test_secret_key",
                "region": "us-east-1",
            },
            {
                "storage_uri": "s3://dataset/path",
                "endpoint_url": "https://s3.amazonaws.com",
                "access_key_id": "test_access_key",
                "access_key_secret": "test_secret_key",
                "region": "us-east-1",
            },
        ),
        (
            "Minimal config without credentials",
            {"storage_uri": "s3://dataset/path"},
            {
                "storage_uri": "s3://dataset/path",
                "endpoint_url": None,
                "access_key_id": None,
                "access_key_secret": None,
                "region": None,
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
        assert s3_dataset_instance.config.storage_uri == expected["storage_uri"]
        assert s3_dataset_instance.config.endpoint_url == expected["endpoint_url"]
        assert s3_dataset_instance.config.access_key_id == expected["access_key_id"]
        assert (
            s3_dataset_instance.config.access_key_secret
            == expected["access_key_secret"]
        )
        assert s3_dataset_instance.config.region == expected["region"]

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful download with credentials",
            {
                "config": {
                    "storage_uri": "s3://dataset/path/subpath",
                    "endpoint_url": "https://s3.amazonaws.com",
                    "access_key_id": "test_access_key",
                    "access_key_secret": "test_secret_key",
                    "region": "us-east-1",
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
                    "endpoint_url": None,
                    "access_key_id": None,
                    "access_key_secret": None,
                    "region": None,
                },
                "expected_bucket": "dataset",
                "expected_prefix": "path",
            },
        ),
    ],
)
@mock_aws
def test_download_dataset(test_name, test_case):
    """Test dataset download with different configurations"""

    print(f"Running test: {test_name}")

    # Create a mock S3 environment
    s3_client = boto3.client("s3")

    # Create the bucket
    bucket_name = test_case["expected_bucket"]
    s3_client.create_bucket(Bucket=bucket_name)

    # Upload test files to the mock S3 bucket
    prefix = test_case["expected_prefix"]
    test_files = [
        (f"{prefix}/file1.txt", "Test content 1"),
        (f"{prefix}/file2.txt", "Test content 2"),
        (f"{prefix}/foo/file3.txt", "Test content 3"),
    ]

    for key, content in test_files:
        s3_client.put_object(Bucket=bucket_name, Key=key, Body=content)

    # Verify files exist in the mock S3 bucket
    response = s3_client.list_objects_v2(Bucket=bucket_name, Prefix=prefix)
    assert "Contents" in response
    assert len(response["Contents"]) == 3

    # Setup the S3 dataset instance
    s3_dataset_instance = S3()
    s3_dataset_instance.config = MagicMock(**test_case["config"])

    # Create a temporary directory for downloads
    with tempfile.TemporaryDirectory() as temp_dir:
        dataset_path = os.path.join(temp_dir, "dataset")

        with patch.object(
            s3_dataset_instance, "_create_s3_client", return_value=s3_client
        ), patch.object(utils, "DATASET_PATH", dataset_path):

            # Execute download
            s3_dataset_instance.download_dataset()

            # Verify files were downloaded
            file1_path = os.path.join(dataset_path, "file1.txt")
            file2_path = os.path.join(dataset_path, "file2.txt")
            file3_path = os.path.join(dataset_path, "foo", "file3.txt")

            assert os.path.exists(file1_path), f"File {file1_path} should exist"
            assert os.path.exists(file2_path), f"File {file2_path} should exist"
            assert os.path.exists(file3_path), f"File {file3_path} should exist"

            # Verify file contents
            with open(file1_path, "r") as f:
                assert f.read() == "Test content 1"
            with open(file2_path, "r") as f:
                assert f.read() == "Test content 2"
            with open(file3_path, "r") as f:
                assert f.read() == "Test content 3"

    print("Test execution completed")
