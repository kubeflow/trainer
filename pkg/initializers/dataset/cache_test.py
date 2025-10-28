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
                "storage_uri": "cache://test_schema/test_table",
                "train_job_name": "custom-job",
                "cache_image": "custom-image:latest",
                "cluster_size": "5",
                "metadata_loc": "s3://bucket/metadata",
                "iam_role": "arn:aws:iam::123456789012:role/custom-role",
                "head_cpu": "4",
                "head_mem": "8Gi",
                "worker_cpu": "8",
                "worker_mem": "16Gi",
            },
            {
                "storage_uri": "cache://test_schema/test_table",
                "train_job_name": "custom-job",
                "cache_image": "custom-image:latest",
                "cluster_size": "5",
                "metadata_loc": "s3://bucket/metadata",
                "iam_role": "arn:aws:iam::123456789012:role/custom-role",
                "head_cpu": "4",
                "head_mem": "8Gi",
                "worker_cpu": "8",
                "worker_mem": "16Gi",
                "readiness_initial_delay_seconds": "5",
                "readiness_period_seconds": "10",
                "readiness_timeout_seconds": "5",
                "readiness_failure_threshold": "3",
            },
        ),
        (
            "Minimal config with only storage_uri",
            {
                "storage_uri": "cache://minimal_schema/minimal_table",
                "train_job_name": "minimal-job",
                "cache_image": "minimal-image:latest",
                "iam_role": "arn:aws:iam::123456789012:role/minimal-role",
                "metadata_loc": "s3://minimal-bucket/metadata",
            },
            {
                "storage_uri": "cache://minimal_schema/minimal_table",
                "train_job_name": "minimal-job",
                "cache_image": "minimal-image:latest",
                "cluster_size": "3",
                "metadata_loc": "s3://minimal-bucket/metadata",
                "iam_role": "arn:aws:iam::123456789012:role/minimal-role",
                "head_cpu": "1",
                "head_mem": "1Gi",
                "worker_cpu": "2",
                "worker_mem": "2Gi",
                "readiness_initial_delay_seconds": "5",
                "readiness_period_seconds": "10",
                "readiness_timeout_seconds": "5",
                "readiness_failure_threshold": "3",
            },
        ),
        (
            "Partial config with some values",
            {
                "storage_uri": "cache://partial_schema/partial_table",
                "train_job_name": "partial-job",
                "cache_image": "partial-image:latest",
                "iam_role": "arn:aws:iam::123456789012:role/partial-role",
                "head_cpu": "2",
                "worker_cpu": "4",
                "metadata_loc": "s3://partial-bucket/metadata",
            },
            {
                "storage_uri": "cache://partial_schema/partial_table",
                "train_job_name": "partial-job",
                "cache_image": "partial-image:latest",
                "cluster_size": "3",
                "metadata_loc": "s3://partial-bucket/metadata",
                "iam_role": "arn:aws:iam::123456789012:role/partial-role",
                "head_cpu": "2",
                "head_mem": "1Gi",
                "worker_cpu": "4",
                "worker_mem": "2Gi",
                "readiness_initial_delay_seconds": "5",
                "readiness_period_seconds": "10",
                "readiness_timeout_seconds": "5",
                "readiness_failure_threshold": "3",
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
                    "storage_uri": "cache://test_schema/test_table",
                    "train_job_name": "full-job",
                    "cache_image": "custom-cache:v1.0",
                    "cluster_size": "5",
                    "metadata_loc": "s3://test-bucket/metadata",
                    "iam_role": "arn:aws:iam::123456789012:role/test-role",
                    "head_cpu": "4",
                    "head_mem": "8Gi",
                    "worker_cpu": "8",
                    "worker_mem": "16Gi",
                },
                "expected_train_job_name": "full-job",
            },
        ),
        (
            "Default values with minimal configuration",
            {
                "config": {
                    "storage_uri": "cache://minimal_test_schema/minimal_test_table",
                    "train_job_name": "minimal-job",
                    "cache_image": "test-image:latest",
                    "iam_role": "arn:aws:iam::123456789012:role/test-role",
                    "metadata_loc": "s3://minimal-test-bucket/metadata",
                },
                "expected_train_job_name": "minimal-job",
            },
        ),
        (
            "Mixed configuration with some defaults",
            {
                "config": {
                    "storage_uri": "cache://mixed_schema/mixed_table",
                    "train_job_name": "mixed-job",
                    "cache_image": "mixed-image:v2.0",
                    "iam_role": "arn:aws:iam::987654321098:role/mixed-role",
                    "head_cpu": "6",
                    "worker_mem": "32Gi",
                    "metadata_loc": "s3://mixed-bucket/data",
                },
                "expected_train_job_name": "mixed-job",
            },
        ),
        (
            "Minimal config uses defaults for optional fields",
            {
                "config": {
                    "storage_uri": "cache://required_schema/required_table",
                    "train_job_name": "required-job",
                    "cache_image": "test-image:required",
                    "iam_role": "arn:aws:iam::123456789012:role/required",
                    "metadata_loc": "s3://required-bucket/metadata",
                },
                "expected_train_job_name": "required-job",
            },
        ),
    ],
)
def test_download_dataset(test_name, test_case):
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
            "kind": "TrainJob",
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
        cache_initializer_instance.download_dataset()

        # Verify Kubernetes client calls were made
        mock_config.load_incluster_config.assert_called_once()
        mock_client.ApiClient.assert_called_once()
        mock_client.CoreV1Api.assert_called_once_with(mock_api_client)
        mock_client.CustomObjectsApi.assert_called_once_with(mock_api_client)

    print("Test execution completed")
