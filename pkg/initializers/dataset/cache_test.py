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


TRAIN_JOB = {
    "apiVersion": "trainer.kubeflow.org/v1alpha1",
    "kind": "TrainJob",
    "metadata": {"name": "cache-job", "uid": "test-uid"},
}

LWS_READY = {"status": {"conditions": [{"type": "Available", "status": "True"}]}}


def build_initializer() -> CacheInitializer:
    """Build a CacheInitializer with a minimal valid config."""
    initializer = CacheInitializer()
    config = {
        "storage_uri": "cache://test_schema/test_table",
        "train_job_name": "cache-job",
        "cache_image": "test-image:latest",
        "iam_role": "arn:aws:iam::123456789012:role/test-role",
        "metadata_loc": "s3://test-bucket/metadata",
    }
    with patch.object(utils, "get_config_from_env", return_value=config):
        initializer.load_config()
    return initializer


@pytest.fixture
def k8s_mocks():
    """Patch the Kubernetes clients the cache initializer builds."""
    with patch(
        "pkg.initializers.dataset.cache.get_namespace", return_value="test-namespace"
    ), patch("pkg.initializers.dataset.cache.config"), patch(
        "pkg.initializers.dataset.cache.client"
    ) as mock_client:
        core_v1 = MagicMock()
        custom_api = MagicMock()
        mock_client.ApiClient.return_value = MagicMock()
        mock_client.CoreV1Api.return_value = core_v1
        mock_client.CustomObjectsApi.return_value = custom_api
        # The real client echoes back the name given to V1ObjectMeta, and the
        # initializer reads it off the object to address deletes.
        mock_client.V1ServiceAccount.return_value.metadata.name = "cache-job-cache"
        mock_client.V1Service.return_value.metadata.name = "cache-job-cache-service"
        yield core_v1, custom_api


def test_download_dataset_raises_when_trainjob_lookup_fails(k8s_mocks):
    core_v1, custom_api = k8s_mocks
    custom_api.get_namespaced_custom_object.side_effect = ApiException(status=500)

    with pytest.raises(ApiException):
        build_initializer().download_dataset()

    core_v1.create_namespaced_service_account.assert_not_called()


def test_download_dataset_keeps_preexisting_service_account_on_failure(k8s_mocks):
    """A retry must not delete a ServiceAccount an earlier attempt left behind."""
    core_v1, custom_api = k8s_mocks
    core_v1.create_namespaced_service_account.side_effect = ApiException(
        status=409, reason="AlreadyExists"
    )
    # A single-element side_effect turns an unexpected entry into the readiness
    # loop into StopIteration rather than an endless hang.
    custom_api.get_namespaced_custom_object.side_effect = [TRAIN_JOB]
    custom_api.create_namespaced_custom_object.side_effect = ApiException(status=500)

    with pytest.raises(ApiException):
        build_initializer().download_dataset()

    core_v1.delete_namespaced_service_account.assert_not_called()


def test_download_dataset_leader_worker_set_already_exists(k8s_mocks):
    core_v1, custom_api = k8s_mocks
    custom_api.create_namespaced_custom_object.side_effect = ApiException(
        status=409, reason="AlreadyExists"
    )
    custom_api.get_namespaced_custom_object.side_effect = [TRAIN_JOB, LWS_READY]

    build_initializer().download_dataset()

    core_v1.delete_namespaced_service_account.assert_not_called()


def test_download_dataset_rolls_back_resources_it_created(k8s_mocks):
    core_v1, custom_api = k8s_mocks
    manager = MagicMock()
    manager.attach_mock(core_v1, "core")
    manager.attach_mock(custom_api, "custom")
    custom_api.get_namespaced_custom_object.side_effect = [TRAIN_JOB]
    core_v1.create_namespaced_service.side_effect = ApiException(status=500)

    with pytest.raises(ApiException):
        build_initializer().download_dataset()

    core_v1.delete_namespaced_service_account.assert_called_once_with(
        name="cache-job-cache", namespace="test-namespace"
    )
    custom_api.delete_namespaced_custom_object.assert_called_once_with(
        group="leaderworkerset.x-k8s.io",
        version="v1",
        namespace="test-namespace",
        plural="leaderworkersets",
        name="cache-job-cache",
    )
    # The Service is what failed, so it was never ours to delete.
    core_v1.delete_namespaced_service.assert_not_called()
    # Newest first, so nothing is deleted while something still depends on it.
    assert [name for name, _, _ in manager.mock_calls if ".delete_" in name] == [
        "custom.delete_namespaced_custom_object",
        "core.delete_namespaced_service_account",
    ]


def test_download_dataset_rolls_back_on_non_api_errors(k8s_mocks):
    """A connection error is as half-built as a 500, and must roll back too."""
    core_v1, custom_api = k8s_mocks
    custom_api.get_namespaced_custom_object.side_effect = [TRAIN_JOB]
    core_v1.create_namespaced_service.side_effect = ConnectionError("connection reset")

    with pytest.raises(ConnectionError):
        build_initializer().download_dataset()

    core_v1.delete_namespaced_service_account.assert_called_once()
    custom_api.delete_namespaced_custom_object.assert_called_once()


def test_download_dataset_cleanup_failure_does_not_mask_original_error(k8s_mocks):
    core_v1, custom_api = k8s_mocks
    custom_api.get_namespaced_custom_object.side_effect = [TRAIN_JOB]
    core_v1.create_namespaced_service.side_effect = ApiException(status=500)
    custom_api.delete_namespaced_custom_object.side_effect = ApiException(status=403)

    with pytest.raises(ApiException) as excinfo:
        build_initializer().download_dataset()

    assert excinfo.value.status == 500
    # Best effort: one failed delete must not strand the rest of the rollback.
    core_v1.delete_namespaced_service_account.assert_called_once()
