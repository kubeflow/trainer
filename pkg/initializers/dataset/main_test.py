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

from pkg.initializers.dataset.__main__ import main


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful download with HuggingFace provider",
            {
                "storage_uri": "hf://dataset/path",
                "access_token": "test_token",
                "expected_error": None,
                "expected_error_match": None,
            },
        ),
        (
            "Successful download with S3 provider",
            {
                "storage_uri": "s3://dataset/path",
                "expected_error": None,
                "expected_error_match": None,
            },
        ),
        (
            "Successful download with Data Cache provider",
            {
                "storage_uri": "cache://dataset/path",
                "expected_error": None,
                "expected_error_match": None,
            },
        ),
        (
            "Missing storage URI environment variable",
            {
                "storage_uri": None,
                "access_token": None,
                "expected_error": ValueError,
                "expected_error_match": "STORAGE_URI environment variable must be set",
            },
        ),
        (
            "Empty storage URI environment variable",
            {
                "storage_uri": "",
                "access_token": None,
                "expected_error": ValueError,
                "expected_error_match": "STORAGE_URI environment variable must be set",
            },
        ),
        (
            "Invalid storage URI scheme",
            {
                "storage_uri": "invalid://dataset/path",
                "access_token": None,
                "expected_error": ValueError,
                "expected_error_match": (
                    "Unsupported dataset storage URI scheme 'invalid': expected one of "
                    "'hf', 'cache', or 's3'"
                ),
            },
        ),
    ],
)
def test_dataset_main(test_name, test_case, mock_env_vars):
    """Test main script with different scenarios"""
    print(f"Running test: {test_name}")

    env_vars = {
        "STORAGE_URI": test_case["storage_uri"],
        "ACCESS_TOKEN": test_case.get("access_token", None),
    }
    mock_env_vars(**env_vars)

    mock_hf_instance = MagicMock()
    mock_s3_instance = MagicMock()
    mock_cache_instance = MagicMock()

    with patch(
        "pkg.initializers.dataset.huggingface.HuggingFace",
        return_value=mock_hf_instance,
    ) as mock_hf, patch(
        "pkg.initializers.dataset.s3.S3",
        return_value=mock_s3_instance,
    ) as mock_s3, patch(
        "pkg.initializers.dataset.cache.CacheInitializer",
        return_value=mock_cache_instance,
    ) as mock_cache:

        if test_case["expected_error"]:
            with pytest.raises(
                test_case["expected_error"],
                match=test_case["expected_error_match"],
            ):
                main()
        else:
            main()

            storage_uri = test_case["storage_uri"]
            if storage_uri.startswith("hf://"):
                mock_hf.assert_called_once()
                mock_hf_instance.load_config.assert_called_once()
                mock_hf_instance.download_dataset.assert_called_once()
                mock_s3.assert_not_called()
                mock_cache.assert_not_called()
            elif storage_uri.startswith("s3://"):
                mock_s3.assert_called_once()
                mock_s3_instance.load_config.assert_called_once()
                mock_s3_instance.download_dataset.assert_called_once()
                mock_hf.assert_not_called()
                mock_cache.assert_not_called()
            elif storage_uri.startswith("cache://"):
                mock_cache.assert_called_once()
                mock_cache_instance.load_config.assert_called_once()
                mock_cache_instance.download_dataset.assert_called_once()
                mock_hf.assert_not_called()
                mock_s3.assert_not_called()

        if test_case["expected_error"]:
            mock_hf.assert_not_called()
            mock_s3.assert_not_called()
            mock_cache.assert_not_called()

    print("Test execution completed")
