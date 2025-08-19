from unittest.mock import MagicMock, patch

import pytest
from kubernetes.client.rest import ApiException

import pkg.initializers.utils.utils as utils
from pkg.initializers.dataset.cache import CacheInitializer, create_cache_resources


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
                "namespace": "custom-namespace",
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
                "namespace": "custom-namespace",
            },
        ),
        (
            "Minimal config with only storage_uri",
            {
                "storage_uri": "cache://minimal-uri",
                "train_job_name": "minimal-job",
                "cache_image": "minimal-image:latest",
                "iam_role": "arn:aws:iam::123456789012:role/minimal-role",
                "namespace": "minimal-namespace",
            },
            {
                "storage_uri": "cache://minimal-uri",
                "train_job_name": "minimal-job",
                "cache_image": "minimal-image:latest",
                "cluster_size": "3",
                "metadata_loc": None,
                "table_name": None,
                "schema_name": None,
                "iam_role": "arn:aws:iam::123456789012:role/minimal-role",
                "head_cpu": "1",
                "head_mem": "1Gi",
                "worker_cpu": "2",
                "worker_mem": "2Gi",
                "namespace": "minimal-namespace",
            },
        ),
        (
            "Partial config with some values",
            {
                "storage_uri": "cache://partial-uri",
                "train_job_name": "partial-job",
                "cache_image": "partial-image:latest",
                "iam_role": "arn:aws:iam::123456789012:role/partial-role",
                "namespace": "partial-namespace",
                "head_cpu": "2",
                "worker_cpu": "4",
            },
            {
                "storage_uri": "cache://partial-uri",
                "train_job_name": "partial-job",
                "cache_image": "partial-image:latest",
                "cluster_size": "3",
                "metadata_loc": None,
                "table_name": None,
                "schema_name": None,
                "iam_role": "arn:aws:iam::123456789012:role/partial-role",
                "head_cpu": "2",
                "head_mem": "1Gi",
                "worker_cpu": "4",
                "worker_mem": "2Gi",
                "namespace": "partial-namespace",
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
        assert (
            cache_initializer_instance.config.train_job_name
            == expected["train_job_name"]
        )
        assert cache_initializer_instance.config.cache_image == expected["cache_image"]
        assert (
            cache_initializer_instance.config.cluster_size == expected["cluster_size"]
        )
        assert (
            cache_initializer_instance.config.metadata_loc == expected["metadata_loc"]
        )
        assert cache_initializer_instance.config.table_name == expected["table_name"]
        assert cache_initializer_instance.config.schema_name == expected["schema_name"]
        assert cache_initializer_instance.config.iam_role == expected["iam_role"]
        assert cache_initializer_instance.config.head_cpu == expected["head_cpu"]
        assert cache_initializer_instance.config.head_mem == expected["head_mem"]
        assert cache_initializer_instance.config.worker_cpu == expected["worker_cpu"]
        assert cache_initializer_instance.config.worker_mem == expected["worker_mem"]
        assert cache_initializer_instance.config.namespace == expected["namespace"]

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
                    "namespace": "test-namespace",
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
                    "train_job_name": "minimal-job",
                    "cache_image": "test-image:latest",
                    "iam_role": "arn:aws:iam::123456789012:role/test-role",
                    "namespace": "minimal-namespace",
                },
                "expected_substitutions": {
                    "NAME": "minimal-job",
                    "IAM_ROLE": "arn:aws:iam::123456789012:role/test-role",
                    "SIZE": "3",
                    "IMAGE": "test-image:latest",
                    "HEAD_CPU": "1",
                    "HEAD_MEM": "1Gi",
                    "WORKER_CPU": "2",
                    "WORKER_MEM": "2Gi",
                },
                "expected_train_job_name": "minimal-job",
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
                    "namespace": "mixed-namespace",
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

    # Use proper load_config instead of mocking config directly
    with patch.object(utils, "get_config_from_env", return_value=test_case["config"]):
        cache_initializer_instance.load_config()

    with patch("pkg.initializers.dataset.cache.create_cache_resources") as mock_create:

        # Execute download
        cache_initializer_instance.download_dataset()

        # Verify create_cache_resources was called with correct parameters
        mock_create.assert_called_once()
        call_args = mock_create.call_args

        # Check keyword arguments
        assert call_args[1]["train_job_name"] == test_case["expected_train_job_name"]
        assert call_args[1]["namespace"] == test_case["config"]["namespace"]
        assert (
            call_args[1]["cache_image"] == test_case["expected_substitutions"]["IMAGE"]
        )
        assert (
            call_args[1]["iam_role"] == test_case["expected_substitutions"]["IAM_ROLE"]
        )
        assert call_args[1]["cluster_size"] == int(
            test_case["expected_substitutions"]["SIZE"]
        )
        assert (
            call_args[1]["head_cpu"] == test_case["expected_substitutions"]["HEAD_CPU"]
        )
        assert (
            call_args[1]["head_mem"] == test_case["expected_substitutions"]["HEAD_MEM"]
        )
        assert (
            call_args[1]["worker_cpu"]
            == test_case["expected_substitutions"]["WORKER_CPU"]
        )
        assert (
            call_args[1]["worker_mem"]
            == test_case["expected_substitutions"]["WORKER_MEM"]
        )

        # Check optional parameters
        if "METADATA_LOC" in test_case["expected_substitutions"]:
            assert (
                call_args[1]["metadata_loc"]
                == test_case["expected_substitutions"]["METADATA_LOC"]
            )
        if "TABLE_NAME" in test_case["expected_substitutions"]:
            assert (
                call_args[1]["table_name"]
                == test_case["expected_substitutions"]["TABLE_NAME"]
            )
        if "SCHEMA_NAME" in test_case["expected_substitutions"]:
            assert (
                call_args[1]["schema_name"]
                == test_case["expected_substitutions"]["SCHEMA_NAME"]
            )

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, config_values, expected_defaults",
    [
        (
            "Minimal config uses defaults for optional fields",
            {
                "train_job_name": "required-job",
                "cache_image": "test-image:required",
                "iam_role": "arn:aws:iam::123456789012:role/required",
                "namespace": "required-namespace",
            },
            {
                "train_job_name": "required-job",
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

    # Use proper load_config instead of mocking config directly
    test_config = {"storage_uri": "cache://test"}
    test_config.update(config_values)

    with patch.object(utils, "get_config_from_env", return_value=test_config):
        cache_initializer_instance.load_config()

    with patch("pkg.initializers.dataset.cache.create_cache_resources") as mock_create:

        # Execute download
        cache_initializer_instance.download_dataset()

        # Get the call arguments
        call_args = mock_create.call_args

        # Verify defaults are applied
        assert call_args[1]["train_job_name"] == expected_defaults["train_job_name"]
        assert call_args[1]["cache_image"] == expected_defaults["cache_image"]
        assert call_args[1]["cluster_size"] == int(expected_defaults["cluster_size"])
        assert call_args[1]["iam_role"] == expected_defaults["iam_role"]
        assert call_args[1]["head_cpu"] == expected_defaults["head_cpu"]
        assert call_args[1]["head_mem"] == expected_defaults["head_mem"]
        assert call_args[1]["worker_cpu"] == expected_defaults["worker_cpu"]
        assert call_args[1]["worker_mem"] == expected_defaults["worker_mem"]

    print("Test execution completed")


@patch("pkg.initializers.dataset.cache.config")
@patch("pkg.initializers.dataset.cache.client")
def test_create_cache_resources_success(mock_client, mock_config):
    """Test successful creation of cache resources using Kubernetes client SDK"""

    # Setup mocks
    mock_api_client = MagicMock()
    mock_core_v1 = MagicMock()
    mock_custom_api = MagicMock()

    mock_client.ApiClient.return_value = mock_api_client
    mock_client.CoreV1Api.return_value = mock_core_v1
    mock_client.CustomObjectsApi.return_value = mock_custom_api

    # Mock training job response
    mock_training_job = {
        "apiVersion": "trainer.kubeflow.org/v1alpha1",
        "kind": "TrainingJob",
        "metadata": {"name": "test-job", "uid": "test-uid"},
    }
    mock_custom_api.get_namespaced_custom_object.return_value = mock_training_job

    # Mock LWS status to simulate ready state
    mock_lws_ready = {
        "status": {"conditions": [{"type": "Available", "status": "True"}]}
    }
    mock_custom_api.get_namespaced_custom_object.side_effect = [
        mock_training_job,  # First call for training job
        mock_lws_ready,  # Second call for LWS status check
    ]

    # Test the function
    result = create_cache_resources(
        train_job_name="test-job",
        iam_role="arn:aws:iam::123456789012:role/test-role",
        cluster_size=3,
        cache_image="test-image:latest",
        head_cpu="2",
        head_mem="4Gi",
        worker_cpu="4",
        worker_mem="8Gi",
        metadata_loc="s3://test-bucket/metadata",
        table_name="test_table",
        schema_name="test_schema",
        namespace="test-namespace",
    )

    # Verify the result
    assert result is True

    # Verify ServiceAccount creation was called
    mock_core_v1.create_namespaced_service_account.assert_called_once()

    # Verify LeaderWorkerSet creation was called
    mock_custom_api.create_namespaced_custom_object.assert_called()

    # Verify Service creation was called
    mock_core_v1.create_namespaced_service.assert_called_once()


@patch("pkg.initializers.dataset.cache.config")
@patch("pkg.initializers.dataset.cache.client")
def test_create_cache_resources_training_job_not_found(mock_client, mock_config):
    """Test handling when TrainingJob is not found"""

    # Setup mocks
    mock_api_client = MagicMock()
    mock_custom_api = MagicMock()

    mock_client.ApiClient.return_value = mock_api_client
    mock_client.CustomObjectsApi.return_value = mock_custom_api

    # Mock API exception when getting training job
    mock_custom_api.get_namespaced_custom_object.side_effect = ApiException(
        status=404, reason="Not Found"
    )

    # Test the function
    result = create_cache_resources(
        train_job_name="nonexistent-job",
        iam_role="arn:aws:iam::123456789012:role/test-role",
        cluster_size=3,
        cache_image="test-image:latest",
        head_cpu="2",
        head_mem="4Gi",
        worker_cpu="4",
        worker_mem="8Gi",
        namespace="test-namespace",
    )

    # Verify the result
    assert result is False
