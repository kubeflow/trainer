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

from contextlib import contextmanager
from unittest.mock import MagicMock, patch

import pytest
from kubernetes.client.rest import ApiException

import pkg.initializers.utils.utils as utils
from pkg.initializers.dataset.cache import CacheInitializer, parse_cache_storage_uri


@pytest.mark.parametrize(
    ("storage_uri", "expected"),
    [
        ("cache://my_schema/my_table", ("my_schema", "my_table")),
        ("cache://test_schema/test_table", ("test_schema", "test_table")),
        ("cache://", None),
        ("cache://schema-only", None),
        ("cache:///table", None),
        ("cache://schema/", None),
        ("cache://schema/table/extra", None),
    ],
)
def test_parse_cache_storage_uri(storage_uri, expected):
    if expected is None:
        with pytest.raises(
            ValueError,
            match="expected format cache://<SCHEMA_NAME>/<TABLE_NAME>",
        ):
            parse_cache_storage_uri(storage_uri)
    else:
        assert parse_cache_storage_uri(storage_uri) == expected


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


def test_download_dataset_service_already_exists():
    """Service creation should be idempotent — a 409 AlreadyExists response
    must be treated as a no-op, not as a creation failure that triggers
    ServiceAccount cleanup."""

    cache_initializer_instance = CacheInitializer()

    config = {
        "storage_uri": "cache://test_schema/test_table",
        "train_job_name": "idempotent-job",
        "cache_image": "test-image:latest",
        "iam_role": "arn:aws:iam::123456789012:role/test-role",
        "metadata_loc": "s3://test-bucket/metadata",
    }

    with patch.object(utils, "get_config_from_env", return_value=config):
        cache_initializer_instance.load_config()

    with patch(
        "pkg.initializers.dataset.cache.get_namespace", return_value="test-namespace"
    ), patch("pkg.initializers.dataset.cache.config"), patch(
        "pkg.initializers.dataset.cache.client"
    ) as mock_client:

        mock_api_client = MagicMock()
        mock_core_v1 = MagicMock()
        mock_custom_api = MagicMock()

        mock_client.ApiClient.return_value = mock_api_client
        mock_client.CoreV1Api.return_value = mock_core_v1
        mock_client.CustomObjectsApi.return_value = mock_custom_api

        # Simulate Service already existing in the cluster (e.g., from a previous
        # reconcile that crashed after creating the Service).
        mock_core_v1.create_namespaced_service.side_effect = ApiException(
            status=409, reason="AlreadyExists"
        )

        mock_training_job = {
            "apiVersion": "trainer.kubeflow.org/v1alpha1",
            "kind": "TrainJob",
            "metadata": {"name": "idempotent-job", "uid": "test-uid"},
        }
        mock_lws_ready = {
            "status": {"conditions": [{"type": "Available", "status": "True"}]}
        }
        mock_custom_api.get_namespaced_custom_object.side_effect = [
            mock_training_job,
            mock_lws_ready,
        ]

        # Must not raise.
        cache_initializer_instance.download_dataset()

        # Must not trigger cleanup of the ServiceAccount.
        mock_core_v1.delete_namespaced_service_account.assert_not_called()


def _load_cache_initializer(config):
    cache_initializer_instance = CacheInitializer()
    with patch.object(utils, "get_config_from_env", return_value=config):
        cache_initializer_instance.load_config()
    return cache_initializer_instance


def _mock_training_job(train_job_name="test-job"):
    return {
        "apiVersion": "trainer.kubeflow.org/v1alpha1",
        "kind": "TrainJob",
        "metadata": {"name": train_job_name, "uid": "test-uid"},
    }


def _mock_lws_ready():
    return {"status": {"conditions": [{"type": "Available", "status": "True"}]}}


def _mock_kubernetes_clients(mock_client, train_job_name="test-job"):
    mock_api_client = MagicMock()
    mock_core_v1 = MagicMock()
    mock_custom_api = MagicMock()

    mock_client.ApiClient.return_value = mock_api_client
    mock_client.CoreV1Api.return_value = mock_core_v1
    mock_client.CustomObjectsApi.return_value = mock_custom_api

    mock_custom_api.get_namespaced_custom_object.side_effect = [
        _mock_training_job(train_job_name),
        _mock_lws_ready(),
    ]

    return mock_core_v1, mock_custom_api


@contextmanager
def _patch_cache_kubernetes(train_job_name="test-job"):
    with patch(
        "pkg.initializers.dataset.cache.get_namespace", return_value="test-namespace"
    ), patch("pkg.initializers.dataset.cache.config"), patch(
        "pkg.initializers.dataset.cache.client"
    ) as mock_client:
        yield _mock_kubernetes_clients(mock_client, train_job_name)


_DEFAULT_CONFIG = {
    "storage_uri": "cache://test_schema/test_table",
    "train_job_name": "test-job",
    "cache_image": "test-image:latest",
    "iam_role": "arn:aws:iam::123456789012:role/test-role",
    "metadata_loc": "s3://test-bucket/metadata",
}


def test_download_dataset_train_job_get_failure_raises():
    """TrainJob API errors must fail the initializer instead of returning."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_custom_api.get_namespaced_custom_object.side_effect = ApiException(
            status=500, reason="Internal Server Error"
        )

        with pytest.raises(ApiException) as exc_info:
            cache_initializer_instance.download_dataset()

        assert exc_info.value.status == 500
        mock_core_v1.create_namespaced_service_account.assert_not_called()
        mock_core_v1.delete_namespaced_service_account.assert_not_called()


def test_download_dataset_service_account_create_failure_raises():
    """ServiceAccount create errors must propagate without cleanup."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_core_v1.create_namespaced_service_account.side_effect = ApiException(
            status=403, reason="Forbidden"
        )

        with pytest.raises(ApiException) as exc_info:
            cache_initializer_instance.download_dataset()

        assert exc_info.value.status == 403
        mock_custom_api.create_namespaced_custom_object.assert_not_called()
        mock_core_v1.delete_namespaced_service_account.assert_not_called()


def test_download_dataset_leader_worker_set_already_exists_is_idempotent():
    """Pre-existing ServiceAccount and LeaderWorkerSet must not trigger cleanup."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_core_v1.create_namespaced_service_account.side_effect = ApiException(
            status=409, reason="AlreadyExists"
        )
        mock_custom_api.create_namespaced_custom_object.side_effect = ApiException(
            status=409, reason="AlreadyExists"
        )

        cache_initializer_instance.download_dataset()

        mock_core_v1.delete_namespaced_service_account.assert_not_called()
        mock_custom_api.delete_namespaced_custom_object.assert_not_called()
        mock_core_v1.delete_namespaced_service.assert_not_called()


def test_download_dataset_failure_cleans_up_only_created_resources():
    """Cleanup should delete only resources created in the current invocation."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_core_v1.create_namespaced_service_account.side_effect = ApiException(
            status=409, reason="AlreadyExists"
        )
        mock_custom_api.create_namespaced_custom_object.side_effect = ApiException(
            status=500, reason="Internal Server Error"
        )

        with pytest.raises(ApiException) as exc_info:
            cache_initializer_instance.download_dataset()

        assert exc_info.value.status == 500
        mock_core_v1.delete_namespaced_service_account.assert_not_called()
        mock_custom_api.delete_namespaced_custom_object.assert_not_called()
        mock_core_v1.delete_namespaced_service.assert_not_called()


def test_download_dataset_failure_cleans_up_newly_created_service_account():
    """LeaderWorkerSet failures must clean up a newly created ServiceAccount."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_custom_api.create_namespaced_custom_object.side_effect = ApiException(
            status=500, reason="Internal Server Error"
        )

        with pytest.raises(ApiException):
            cache_initializer_instance.download_dataset()

        mock_core_v1.delete_namespaced_service_account.assert_called_once_with(
            name="test-job-cache", namespace="test-namespace"
        )
        mock_custom_api.delete_namespaced_custom_object.assert_not_called()
        mock_core_v1.delete_namespaced_service.assert_not_called()


def test_download_dataset_failure_cleans_up_newly_created_leader_worker_set():
    """Service create failures must clean up a newly created LeaderWorkerSet."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_core_v1.create_namespaced_service.side_effect = ApiException(
            status=500, reason="Internal Server Error"
        )

        with pytest.raises(ApiException) as exc_info:
            cache_initializer_instance.download_dataset()

        assert exc_info.value.status == 500
        mock_custom_api.delete_namespaced_custom_object.assert_called_once_with(
            group="leaderworkerset.x-k8s.io",
            version="v1",
            namespace="test-namespace",
            plural="leaderworkersets",
            name="test-job-cache",
        )
        mock_core_v1.delete_namespaced_service_account.assert_called_once_with(
            name="test-job-cache", namespace="test-namespace"
        )
        mock_core_v1.delete_namespaced_service.assert_not_called()


def test_download_dataset_readiness_poll_failure_cleans_up_all_created_resources():
    """Readiness poll failures must clean up every resource created this run."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_custom_api.get_namespaced_custom_object.side_effect = [
            _mock_training_job(),
            ApiException(status=500, reason="Internal Server Error"),
        ]

        with pytest.raises(ApiException) as exc_info:
            cache_initializer_instance.download_dataset()

        assert exc_info.value.status == 500
        mock_core_v1.delete_namespaced_service.assert_called_once_with(
            name="test-job-cache-service", namespace="test-namespace"
        )
        mock_custom_api.delete_namespaced_custom_object.assert_called_once_with(
            group="leaderworkerset.x-k8s.io",
            version="v1",
            namespace="test-namespace",
            plural="leaderworkersets",
            name="test-job-cache",
        )
        mock_core_v1.delete_namespaced_service_account.assert_called_once_with(
            name="test-job-cache", namespace="test-namespace"
        )


def test_download_dataset_readiness_poll_failure_does_not_delete_preexisting_resources():
    """Readiness failures must not delete resources that already existed."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_core_v1.create_namespaced_service_account.side_effect = ApiException(
            status=409, reason="AlreadyExists"
        )
        mock_custom_api.create_namespaced_custom_object.side_effect = ApiException(
            status=409, reason="AlreadyExists"
        )
        mock_core_v1.create_namespaced_service.side_effect = ApiException(
            status=409, reason="AlreadyExists"
        )
        mock_custom_api.get_namespaced_custom_object.side_effect = [
            _mock_training_job(),
            ApiException(status=500, reason="Internal Server Error"),
        ]

        with pytest.raises(ApiException):
            cache_initializer_instance.download_dataset()

        mock_core_v1.delete_namespaced_service.assert_not_called()
        mock_custom_api.delete_namespaced_custom_object.assert_not_called()
        mock_core_v1.delete_namespaced_service_account.assert_not_called()


def test_download_dataset_cleanup_errors_do_not_mask_original_failure():
    """Cleanup failures must be logged without replacing the original error."""
    cache_initializer_instance = _load_cache_initializer(_DEFAULT_CONFIG)

    with _patch_cache_kubernetes() as (mock_core_v1, mock_custom_api):
        mock_custom_api.create_namespaced_custom_object.side_effect = ApiException(
            status=500, reason="Internal Server Error"
        )
        mock_core_v1.delete_namespaced_service_account.side_effect = RuntimeError(
            "cleanup failed"
        )

        with pytest.raises(ApiException) as exc_info:
            cache_initializer_instance.download_dataset()

        assert exc_info.value.status == 500
        mock_core_v1.delete_namespaced_service_account.assert_called_once()
