import pytest
from kubeflow.trainer import HuggingFaceDatasetConfig, HuggingFaceModelInputConfig

import pkg.initializer.utils.utils as utils


@pytest.mark.parametrize(
    "config_class,env_vars,expected",
    [
        (
            HuggingFaceModelInputConfig,
            {"STORAGE_URI": "hf://test", "ACCESS_TOKEN": "token"},
            {"storage_uri": "hf://test", "access_token": "token"},
        ),
        (
            HuggingFaceModelInputConfig,
            {"STORAGE_URI": "hf://test"},
            {"storage_uri": "hf://test", "access_token": None},
        ),
        (
            HuggingFaceDatasetConfig,
            {"STORAGE_URI": "hf://test", "ACCESS_TOKEN": "token"},
            {"storage_uri": "hf://test", "access_token": "token"},
        ),
        (
            HuggingFaceDatasetConfig,
            {"STORAGE_URI": "hf://test"},
            {"storage_uri": "hf://test", "access_token": None},
        ),
    ],
)
def test_get_config_from_env(mock_env_vars, config_class, env_vars, expected):
    mock_env_vars(**env_vars)
    result = utils.get_config_from_env(config_class)
    assert result == expected
