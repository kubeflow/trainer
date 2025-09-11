from unittest.mock import MagicMock, patch

import pytest

import pkg.initializers.utils.utils as utils
from pkg.initializers.dataset.cache_initalizer import CacheInitializer


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with all values",
            {
                "storage_uri": "cache://test-uri",
                "train_job_name": "custom-job",
                "cache_image": "custom-image:latest",
                "cluster_size": "5",
                "metadata_loc": "s3://bucket/metadata",
                "table_name": "test_table",
                "schema_name": "test_schema",
                "iam_role": "arn:aws:iam::123456789012:role/custom-role",
                "head_cpu": "4",
                "head_mem": "8Gi",
                "worker_cpu": "8",
                "worker_mem": "16Gi",
            },
            {
                "storage_uri": "cache://test-uri",
                "train_job_name": "custom-job",
                "cache_image": "custom-image:latest",
                "cluster_size": "5",
                "metadata_loc": "s3://bucket/metadata",
                "table_name": "test_table",
                "schema_name": "test_schema",
                "iam_role": "arn:aws:iam::123456789012:role/custom-role",
                "head_cpu": "4",
                "head_mem": "8Gi",
                "worker_cpu": "8",
                "worker_mem": "16Gi",
            },
        ),
        (
            "Minimal config with only storage_uri",
            {"storage_uri": "cache://minimal-uri"},
            {
                "storage_uri": "cache://minimal-uri",
                "train_job_name": None,
                "cache_image": None,
                "cluster_size": None,
                "metadata_loc": None,
                "table_name": None,
                "schema_name": None,
                "iam_role": None,
                "head_cpu": None,
                "head_mem": None,
                "worker_cpu": None,
                "worker_mem": None,
            },
        ),
        (
            "Partial config with some values",
            {
                "storage_uri": "cache://partial-uri",
                "train_job_name": "partial-job",
                "head_cpu": "2",
                "worker_cpu": "4",
            },
            {
                "storage_uri": "cache://partial-uri",
                "train_job_name": "partial-job",
                "cache_image": None,
                "cluster_size": None,
                "metadata_loc": None,
                "table_name": None,
                "schema_name": None,
                "iam_role": None,
                "head_cpu": "2",
                "head_mem": None,
                "worker_cpu": "4",
                "worker_mem": None,
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    cache_initializer_instance = CacheInitializer()

    with patch.object(utils, "get_config_from_env", return_value=test_config):
        cache_initializer_instance.load_config()
        assert cache_initializer_instance.config.storage_uri == expected["storage_uri"]
        assert cache_initializer_instance.config.train_job_name == expected["train_job_name"]
        assert cache_initializer_instance.config.cache_image == expected["cache_image"]
        assert cache_initializer_instance.config.cluster_size == expected["cluster_size"]
        assert cache_initializer_instance.config.metadata_loc == expected["metadata_loc"]
        assert cache_initializer_instance.config.table_name == expected["table_name"]
        assert cache_initializer_instance.config.schema_name == expected["schema_name"]
        assert cache_initializer_instance.config.iam_role == expected["iam_role"]
        assert cache_initializer_instance.config.head_cpu == expected["head_cpu"]
        assert cache_initializer_instance.config.head_mem == expected["head_mem"]
        assert cache_initializer_instance.config.worker_cpu == expected["worker_cpu"]
        assert cache_initializer_instance.config.worker_mem == expected["worker_mem"]

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Full configuration with all substitutions",
            {
                "config": {
                    "storage_uri": "cache://full-test",
                    "train_job_name": "full-job",
                    "cache_image": "custom-cache:v1.0",
                    "cluster_size": "5",
                    "metadata_loc": "s3://test-bucket/metadata",
                    "table_name": "test_table",
                    "schema_name": "test_schema",
                    "iam_role": "arn:aws:iam::123456789012:role/test-role",
                    "head_cpu": "4",
                    "head_mem": "8Gi",
                    "worker_cpu": "8",
                    "worker_mem": "16Gi",
                },
                "expected_substitutions": {
                    "NAME": "full-job",
                    "IAM_ROLE": "arn:aws:iam::123456789012:role/test-role",
                    "SIZE": "5",
                    "IMAGE": "custom-cache:v1.0",
                    "HEAD_CPU": "4",
                    "HEAD_MEM": "8Gi",
                    "WORKER_CPU": "8",
                    "WORKER_MEM": "16Gi",
                    "METADATA_LOC": "s3://test-bucket/metadata",
                    "TABLE_NAME": "test_table",
                    "SCHEMA_NAME": "test_schema",
                },
                "expected_train_job_name": "full-job",
            },
        ),
        (
            "Default values with minimal configuration",
            {
                "config": {
                    "storage_uri": "cache://minimal-test",
                    "cache_image": "test-image:latest",
                    "iam_role": "arn:aws:iam::123456789012:role/test-role",
                },
                "expected_substitutions": {
                    "NAME": "cache-test",
                    "IAM_ROLE": "arn:aws:iam::123456789012:role/test-role",
                    "SIZE": "3",
                    "IMAGE": "test-image:latest",
                    "HEAD_CPU": "1",
                    "HEAD_MEM": "1Gi",
                    "WORKER_CPU": "2",
                    "WORKER_MEM": "2Gi",
                },
                "expected_train_job_name": "cache-test",
            },
        ),
        (
            "Mixed configuration with some defaults",
            {
                "config": {
                    "storage_uri": "cache://mixed-test",
                    "train_job_name": "mixed-job",
                    "cache_image": "mixed-image:v2.0",
                    "iam_role": "arn:aws:iam::987654321098:role/mixed-role",
                    "head_cpu": "6",
                    "worker_mem": "32Gi",
                    "metadata_loc": "s3://mixed-bucket/data",
                },
                "expected_substitutions": {
                    "NAME": "mixed-job",
                    "IAM_ROLE": "arn:aws:iam::987654321098:role/mixed-role",
                    "SIZE": "3",
                    "IMAGE": "mixed-image:v2.0",
                    "HEAD_CPU": "6",
                    "HEAD_MEM": "1Gi",
                    "WORKER_CPU": "2",
                    "WORKER_MEM": "32Gi",
                    "METADATA_LOC": "s3://mixed-bucket/data",
                },
                "expected_train_job_name": "mixed-job",
            },
        ),
    ],
)
def test_download_dataset(test_name, test_case):
    """Test dataset download with different configurations"""

    print(f"Running test: {test_name}")

    cache_initializer_instance = CacheInitializer()
    
    # Create a proper mock config with all attributes set to None by default
    mock_config = MagicMock()
    mock_config.storage_uri = test_case["config"]["storage_uri"]
    mock_config.train_job_name = test_case["config"].get("train_job_name")
    mock_config.cache_image = test_case["config"].get("cache_image")
    mock_config.cluster_size = test_case["config"].get("cluster_size")
    mock_config.metadata_loc = test_case["config"].get("metadata_loc")
    mock_config.table_name = test_case["config"].get("table_name")
    mock_config.schema_name = test_case["config"].get("schema_name")
    mock_config.iam_role = test_case["config"].get("iam_role")
    mock_config.head_cpu = test_case["config"].get("head_cpu")
    mock_config.head_mem = test_case["config"].get("head_mem")
    mock_config.worker_cpu = test_case["config"].get("worker_cpu")
    mock_config.worker_mem = test_case["config"].get("worker_mem")
    
    cache_initializer_instance.config = mock_config

    with patch("pkg.initializers.dataset.cache_initalizer.deploy_lws_with_substitution") as mock_deploy:

        # Execute download
        cache_initializer_instance.download_dataset()

        # Verify deploy_lws_with_substitution was called with correct parameters
        mock_deploy.assert_called_once()
        call_args = mock_deploy.call_args

        # Check positional arguments
        assert call_args[0][0] == test_case["expected_train_job_name"]  # train_job_name
        assert call_args[0][1] == 'cache-initializer-template.yaml'  # yaml_path

        # Check keyword arguments
        assert call_args[1]["namespace"] == 'cache-test'
        assert call_args[1]["timeout"] == 600

        # Verify substitutions
        actual_substitutions = call_args[1]["substitutions"]
        expected_substitutions = test_case["expected_substitutions"]
        
        # Check all expected substitutions are present and correct
        for key, expected_value in expected_substitutions.items():
            assert key in actual_substitutions, f"Missing substitution key: {key}"
            assert actual_substitutions[key] == expected_value, f"Substitution {key}: expected '{expected_value}', got '{actual_substitutions[key]}'"

        # Ensure no unexpected substitutions are present (only check required ones)
        required_keys = {"NAME", "IAM_ROLE", "SIZE", "IMAGE", "HEAD_CPU", "HEAD_MEM", "WORKER_CPU", "WORKER_MEM"}
        for key in required_keys:
            assert key in actual_substitutions, f"Required substitution key missing: {key}"

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, config_values, expected_defaults",
    [
        (
            "All None values should use defaults (except required fields)",
            {
                "cache_image": "test-image:required",
                "iam_role": "arn:aws:iam::123456789012:role/required",
                "train_job_name": None,
                "cluster_size": None,
                "head_cpu": None,
                "head_mem": None,
                "worker_cpu": None,
                "worker_mem": None,
            },
            {
                "train_job_name": "cache-test",
                "cache_image": "test-image:required",
                "cluster_size": "3",
                "iam_role": "arn:aws:iam::123456789012:role/required",
                "head_cpu": "1",
                "head_mem": "1Gi",
                "worker_cpu": "2",
                "worker_mem": "2Gi",
            },
        ),
        (
            "Empty string values should use defaults (except required fields)",
            {
                "cache_image": "test-image:required",
                "iam_role": "arn:aws:iam::123456789012:role/required",
                "train_job_name": "",
                "cluster_size": "",
                "head_cpu": "",
                "head_mem": "",
                "worker_cpu": "",
                "worker_mem": "",
            },
            {
                "train_job_name": "cache-test",
                "cache_image": "test-image:required",
                "cluster_size": "3",
                "iam_role": "arn:aws:iam::123456789012:role/required",
                "head_cpu": "1",
                "head_mem": "1Gi",
                "worker_cpu": "2",
                "worker_mem": "2Gi",
            },
        ),
    ],
)
def test_default_values(test_name, config_values, expected_defaults):
    """Test that default values are applied correctly"""

    print(f"Running test: {test_name}")

    cache_initializer_instance = CacheInitializer()
    cache_initializer_instance.config = MagicMock(
        storage_uri="cache://test",
        **config_values
    )

    with patch("pkg.initializers.dataset.cache_initalizer.deploy_lws_with_substitution") as mock_deploy:

        # Execute download
        cache_initializer_instance.download_dataset()

        # Get the substitutions that were passed
        call_args = mock_deploy.call_args
        actual_substitutions = call_args[1]["substitutions"]

        # Verify defaults are applied
        assert actual_substitutions["NAME"] == expected_defaults["train_job_name"]
        assert actual_substitutions["IMAGE"] == expected_defaults["cache_image"]
        assert actual_substitutions["SIZE"] == expected_defaults["cluster_size"]
        assert actual_substitutions["IAM_ROLE"] == expected_defaults["iam_role"]
        assert actual_substitutions["HEAD_CPU"] == expected_defaults["head_cpu"]
        assert actual_substitutions["HEAD_MEM"] == expected_defaults["head_mem"]
        assert actual_substitutions["WORKER_CPU"] == expected_defaults["worker_cpu"]
        assert actual_substitutions["WORKER_MEM"] == expected_defaults["worker_mem"]

    print("Test execution completed")


def test_missing_cache_image_raises_error():
    """Test that missing cache_image raises ValueError"""
    cache_initializer_instance = CacheInitializer()
    
    mock_config = MagicMock()
    mock_config.storage_uri = "cache://test"
    mock_config.cache_image = None
    mock_config.iam_role = "arn:aws:iam::123456789012:role/test"
    
    cache_initializer_instance.config = mock_config

    with pytest.raises(ValueError, match="CACHE_IMAGE environment variable is required"):
        cache_initializer_instance.download_dataset()


def test_missing_iam_role_raises_error():
    """Test that missing iam_role raises ValueError"""
    cache_initializer_instance = CacheInitializer()
    
    mock_config = MagicMock()
    mock_config.storage_uri = "cache://test"
    mock_config.cache_image = "test-image:latest"
    mock_config.iam_role = None
    
    cache_initializer_instance.config = mock_config

    with pytest.raises(ValueError, match="IAM_ROLE environment variable is required"):
        cache_initializer_instance.download_dataset()