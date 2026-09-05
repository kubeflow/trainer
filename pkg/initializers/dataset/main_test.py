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

import sys
from unittest.mock import MagicMock, patch

import pytest
from kubernetes.client.rest import ApiException

from pkg.initializers.dataset.__main__ import main


@pytest.mark.parametrize(
    "test_case",
    [
        pytest.param(
            {
                "storage_uri": "hf://dataset/path",
                "expected_provider": "hf",
                "expected_error": None,
                "expected_error_match": None,
            },
            id="huggingface-provider",
        ),
        pytest.param(
            {
                "storage_uri": "cache://schema/table",
                "expected_provider": "cache",
                "expected_error": None,
                "expected_error_match": None,
            },
            id="cache-provider",
        ),
        pytest.param(
            {
                "storage_uri": "s3://dataset/path",
                "expected_provider": "s3",
                "expected_error": None,
                "expected_error_match": None,
            },
            id="s3-provider",
        ),
        pytest.param(
            {
                "storage_uri": None,
                "expected_provider": None,
                "expected_error": ValueError,
                "expected_error_match": (
                    "STORAGE_URI environment variable must be set"
                ),
            },
            id="missing-storage-uri",
        ),
        pytest.param(
            {
                "storage_uri": "",
                "expected_provider": None,
                "expected_error": ValueError,
                "expected_error_match": (
                    "STORAGE_URI environment variable must be set"
                ),
            },
            id="empty-storage-uri",
        ),
        pytest.param(
            {
                "storage_uri": "invalid://dataset/path",
                "expected_provider": None,
                "expected_error": ValueError,
                "expected_error_match": (
                    "Unsupported dataset storage URI scheme 'invalid'"
                ),
            },
            id="unsupported-provider",
        ),
        pytest.param(
            {
                "storage_uri": "cache://test_schema/test_table",
                "expected_provider": "cache",
                "expected_error": ApiException,
                "expected_error_match": None,
                "provider_raises_on_download": "cache",
            },
            id="cache-provider-api-failure",
        ),
    ],
)
def test_dataset_main(test_case, mock_env_vars):
    """Test dataset provider dispatch and invalid storage URI handling."""
    mock_env_vars(STORAGE_URI=test_case["storage_uri"])

    mock_hf_instance = MagicMock()
    mock_cache_instance = MagicMock()
    mock_s3_instance = MagicMock()

    mock_hf = MagicMock(return_value=mock_hf_instance)
    mock_cache = MagicMock(return_value=mock_cache_instance)
    mock_s3 = MagicMock(return_value=mock_s3_instance)

    if test_case.get("provider_raises_on_download") == "cache":
        mock_cache_instance.download_dataset.side_effect = ApiException(
            status=500, reason="Internal Server Error"
        )

    mock_provider_modules = {
        "pkg.initializers.dataset.huggingface": MagicMock(
            HuggingFace=mock_hf,
        ),
        "pkg.initializers.dataset.cache": MagicMock(
            CacheInitializer=mock_cache,
        ),
        "pkg.initializers.dataset.s3": MagicMock(
            S3=mock_s3,
        ),
    }

    provider_mocks = {
        "hf": (mock_hf, mock_hf_instance),
        "cache": (mock_cache, mock_cache_instance),
        "s3": (mock_s3, mock_s3_instance),
    }

    with patch.dict(sys.modules, mock_provider_modules):
        if test_case["expected_error"] is not None:
            with pytest.raises(
                test_case["expected_error"],
                match=test_case["expected_error_match"],
            ):
                main()

            if test_case.get("provider_raises_on_download"):
                expected_provider = test_case["expected_provider"]
                expected_constructor, expected_instance = provider_mocks[
                    expected_provider
                ]
                expected_constructor.assert_called_once_with()
                expected_instance.load_config.assert_called_once_with()
                expected_instance.download_dataset.assert_called_once_with()

                for provider_name, (
                    provider_constructor,
                    provider_instance,
                ) in provider_mocks.items():
                    if provider_name == expected_provider:
                        continue
                    provider_constructor.assert_not_called()
                    provider_instance.load_config.assert_not_called()
                    provider_instance.download_dataset.assert_not_called()
            else:
                for provider_constructor, provider_instance in provider_mocks.values():
                    provider_constructor.assert_not_called()
                    provider_instance.load_config.assert_not_called()
                    provider_instance.download_dataset.assert_not_called()

            return

        main()

        expected_provider = test_case["expected_provider"]
        expected_constructor, expected_instance = provider_mocks[expected_provider]

        expected_constructor.assert_called_once_with()
        expected_instance.load_config.assert_called_once_with()
        expected_instance.download_dataset.assert_called_once_with()

        for provider_name, (
            provider_constructor,
            provider_instance,
        ) in provider_mocks.items():
            if provider_name == expected_provider:
                continue

            provider_constructor.assert_not_called()
            provider_instance.load_config.assert_not_called()
            provider_instance.download_dataset.assert_not_called()
