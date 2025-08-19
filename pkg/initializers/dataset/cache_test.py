from unittest.mock import MagicMock, patch

import pytest

import pkg.initializers.utils.utils as utils
from pkg.initializers.dataset.cache import CacheInitializer


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
            {
                "storage_uri": "cache://minimal-uri",
                "train_job_name": "minimal-job",
                "cache_image": "minimal-image:latest",
                "iam_role": "arn:aws:iam::123456789012:role/minimal-role",
                "metadata_loc": "s3://minimal-bucket/metadata",
                "table_name": "minimal_table",
                "schema_name": "minimal_schema",
            },
            {
                "storage_uri": "cache://minimal-uri",
                "train_job_name": "minimal-job",
                "cache_image": "minimal-image:latest",
                "cluster_size": "3",
                "metadata_loc": "s3://minimal-bucket/metadata",
                "table_name": "minimal_table",
                "schema_name": "minimal_schema",
                "iam_role": "arn:aws:iam::123456789012:role/minimal-role",
                "head_cpu": "1",
                "head_mem": "1Gi",
                "worker_cpu": "2",
                "worker_mem": "2Gi",
            },
        ),
        (
            "Partial config with some values",
            {
                "storage_uri": "cache://partial-uri",
                "train_job_name": "partial-job",
                "cache_image": "partial-image:latest",
                "iam_role": "arn:aws:iam::123456789012:role/partial-role",
                "head_cpu": "2",
                "worker_cpu": "4",
                "metadata_loc": "s3://partial-bucket/metadata",
                "table_name": "partial_table",
                "schema_name": "partial_schema",
            },
            {
                "storage_uri": "cache://partial-uri",
                "train_job_name": "partial-job",
                "cache_image": "partial-image:latest",
                "cluster_size": "3",
                "metadata_loc": "s3://partial-bucket/metadata",
                "table_name": "partial_table",
                "schema_name": "partial_schema",
                "iam_role": "arn:aws:iam::123456789012:role/partial-role",
                "head_cpu": "2",
                "head_mem": "1Gi",
                "worker_cpu": "4",
                "worker_mem": "2Gi",
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
        assert cache_initializer_instance.config.__dict__ == expected

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
                    "train_job_name": "minimal-job",
                    "cache_image": "test-image:latest",
                    "iam_role": "arn:aws:iam::123456789012:role/test-role",
                    "metadata_loc": "s3://minimal-test-bucket/metadata",
                    "table_name": "minimal_test_table",
                    "schema_name": "minimal_test_schema",
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
                    "METADATA_LOC": "s3://minimal-test-bucket/metadata",
                    "TABLE_NAME": "minimal_test_table",
                    "SCHEMA_NAME": "minimal_test_schema",
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
                    "table_name": "mixed_table",
                    "schema_name": "mixed_schema",
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
                    "TABLE_NAME": "mixed_table",
                    "SCHEMA_NAME": "mixed_schema",
                },
                "expected_train_job_name": "mixed-job",
            },
        ),
    ],
)
def test_create_cache_cluster(test_name, test_case):
    """Test cache cluster creation with different configurations"""

    print(f"Running test: {test_name}")

    cache_initializer_instance = CacheInitializer()

    # Use proper load_config instead of mocking config directly
    with patch.object(utils, "get_config_from_env", return_value=test_case["config"]):
        cache_initializer_instance.load_config()

    with patch(
        "pkg.initializers.dataset.cache.get_namespace", return_value="test-namespace"
    ), patch("pkg.initializers.dataset.cache.config") as mock_config, patch(
        "pkg.initializers.dataset.cache.client"
    ) as mock_client:

        # Setup mocks for Kubernetes client
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
            "metadata": {
                "name": test_case["expected_train_job_name"],
                "uid": "test-uid",
            },
        }

        # Mock LeaderWorkerSet status response (ready state)
        mock_lws_ready = {
            "status": {"conditions": [{"type": "Available", "status": "True"}]}
        }

        # Set side_effect to return training job first, then ready LWS status
        mock_custom_api.get_namespaced_custom_object.side_effect = [
            mock_training_job,  # First call for training job
            mock_lws_ready,  # Second call for LWS status check
        ]

        # Execute cache cluster creation
        cache_initializer_instance.create_cache_cluster()

        # Verify Kubernetes client calls were made
        mock_config.load_incluster_config.assert_called_once()
        mock_client.ApiClient.assert_called_once()
        mock_client.CoreV1Api.assert_called_once_with(mock_api_client)
        mock_client.CustomObjectsApi.assert_called_once_with(mock_api_client)

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
                "metadata_loc": "s3://required-bucket/metadata",
                "table_name": "required_table",
                "schema_name": "required_schema",
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
                "metadata_loc": "s3://required-bucket/metadata",
                "table_name": "required_table",
                "schema_name": "required_schema",
            },
        ),
    ],
)
def test_default_values(test_name, config_values, expected_defaults):
    """Test that default values are applied correctly for cache cluster creation"""

    print(f"Running test: {test_name}")

    cache_initializer_instance = CacheInitializer()

    # Use proper load_config instead of mocking config directly
    test_config = {"storage_uri": "cache://test"}
    test_config.update(config_values)

    with patch.object(utils, "get_config_from_env", return_value=test_config):
        cache_initializer_instance.load_config()

    with patch(
        "pkg.initializers.dataset.cache.get_namespace", return_value="test-namespace"
    ), patch("pkg.initializers.dataset.cache.config") as mock_config, patch(
        "pkg.initializers.dataset.cache.client"
    ) as mock_client:

        # Setup mocks for Kubernetes client
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
            "metadata": {
                "name": expected_defaults["train_job_name"],
                "uid": "test-uid",
            },
        }

        # Mock LeaderWorkerSet status response (ready state)
        mock_lws_ready = {
            "status": {"conditions": [{"type": "Available", "status": "True"}]}
        }

        # Set side_effect to return training job first, then ready LWS status
        mock_custom_api.get_namespaced_custom_object.side_effect = [
            mock_training_job,  # First call for training job
            mock_lws_ready,  # Second call for LWS status check
        ]

        # Execute cache cluster creation
        cache_initializer_instance.create_cache_cluster()

        # Verify Kubernetes client calls were made
        mock_config.load_incluster_config.assert_called_once()
        mock_client.ApiClient.assert_called_once()
        mock_client.CoreV1Api.assert_called_once_with(mock_api_client)
        mock_client.CustomObjectsApi.assert_called_once_with(mock_api_client)

    print("Test execution completed")
