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

import pkg.initializers.utils.utils as utils
from pkg.initializers.model.kaggle import Kaggle


# Test cases for config loading
@pytest.mark.parametrize(
    "test_name, test_config, expected",
    [
        (
            "Full config with token",
            {
                "storage_uri": "kaggle://models/google/bert/tensorFlow2/answer-equivalence-bem",
                "access_token": "test_token",
            },
            {
                "storage_uri": "kaggle://models/google/bert/tensorFlow2/answer-equivalence-bem",
                "access_token": "test_token",
            },
        ),
        (
            "Minimal config without token",
            {
                "storage_uri": "kaggle://models/username/model/framework/variation",
            },
            {
                "storage_uri": "kaggle://models/username/model/framework/variation",
                "access_token": None,
            },
        ),
    ],
)
def test_load_config(test_name, test_config, expected):
    """Test config loading with different configurations"""
    print(f"Running test: {test_name}")

    kaggle_model_instance = Kaggle()
    with patch.object(utils, "get_config_from_env", return_value=test_config):
        kaggle_model_instance.load_config()
        assert kaggle_model_instance.config.__dict__ == expected

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful model download with token",
            {
                "config": {
                    "storage_uri": "kaggle://models/google/bert/tensorFlow2/answer-equivalence-bem",
                    "access_token": "test_token",
                },
                "should_set_token": True,
                "expected_handle": "google/bert/tensorFlow2/answer-equivalence-bem",
            },
        ),
        (
            "Successful model download without token",
            {
                "config": {
                    "storage_uri": "kaggle://models/username/my-model/pytorch/v1",
                    "access_token": None,
                },
                "should_set_token": False,
                "expected_handle": "username/my-model/pytorch/v1",
            },
        ),
        (
            "Model download with complex path",
            {
                "config": {
                    "storage_uri": "kaggle://models/organization/model-name/tensorflow/production",
                    "access_token": "test_key",
                },
                "should_set_token": True,
                "expected_handle": "organization/model-name/tensorflow/production",
            },
        ),
    ],
)
def test_download_model(test_name, test_case):
    """Test model download with different configurations"""

    print(f"Running test: {test_name}")

    kaggle_model_instance = Kaggle()
    kaggle_model_instance.config = MagicMock(**test_case["config"])

    with patch(
        "pkg.initializers.model.kaggle.kagglehub.model_download"
    ) as mock_download, patch(
        "pkg.initializers.model.kaggle.tempfile.mkdtemp"
    ) as mock_mkdtemp, patch(
        "pkg.initializers.model.kaggle.os.listdir"
    ) as mock_listdir, patch(
        "pkg.initializers.model.kaggle.shutil.move"
    ) as mock_move, patch(
        "pkg.initializers.model.kaggle.shutil.rmtree"
    ) as mock_rmtree, patch(
        "pkg.initializers.model.kaggle.os.path.exists"
    ) as mock_exists, patch.dict(
        "os.environ", {}, clear=True
    ):
        # Setup mocks for staging directory operations
        mock_mkdtemp.return_value = "/tmp/kaggle_model_test789"
        mock_listdir.return_value = ["model.pth", "config.json"]
        mock_exists.return_value = True

        # Execute download
        kaggle_model_instance.download_model()

        # Verify download was called with staging directory
        mock_download.assert_called_once()
        call_args = mock_download.call_args
        assert call_args.kwargs["handle"] == test_case["expected_handle"]
        assert call_args.kwargs["output_dir"] == "/tmp/kaggle_model_test789"
        # Verify move was called for each file
        assert mock_move.call_count == 2
        # Verify cleanup was called
        mock_rmtree.assert_called_once_with("/tmp/kaggle_model_test789")

    print("Test execution completed")


@pytest.mark.parametrize(
    "test_name, invalid_uri, expected_error",
    [
        (
            "Missing framework and variation",
            "kaggle://models/username/model",
            "Invalid Kaggle model URI",
        ),
        (
            "Missing variation",
            "kaggle://models/username/model/framework",
            "Invalid Kaggle model URI",
        ),
        (
            "Wrong resource type",
            "kaggle://datasets/username/dataset",
            "Invalid Kaggle model URI",
        ),
        (
            "Empty path",
            "kaggle://models",
            "Invalid Kaggle model URI",
        ),
    ],
)
def test_invalid_uri(test_name, invalid_uri, expected_error):
    """Test error handling for invalid URIs"""

    print(f"Running test: {test_name}")

    kaggle_model_instance = Kaggle()
    kaggle_model_instance.config = MagicMock(storage_uri=invalid_uri, access_token=None)

    with pytest.raises(ValueError, match=expected_error):
        kaggle_model_instance.download_model()

    print("Test execution completed")
