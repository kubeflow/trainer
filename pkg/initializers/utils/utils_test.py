import pytest

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils


@pytest.mark.parametrize(
    "config_class,env_vars,expected",
    [
        (
            types.HuggingFaceModelInitializer,
            {
                "STORAGE_URI": "hf://test",
                "IGNORE_PATTERNS": "*.msgpack,*.h5",
                "ACCESS_TOKEN": "token",
            },
            {
                "storage_uri": "hf://test",
                "ignore_patterns": ["*.msgpack", "*.h5"],
                "access_token": "token",
            },
        ),
        (
            types.HuggingFaceModelInitializer,
            {"STORAGE_URI": "hf://test"},
            {
                "storage_uri": "hf://test",
                "ignore_patterns": None,
                "access_token": None,
            },
        ),
        (
            types.HuggingFaceDatasetInitializer,
            {
                "STORAGE_URI": "hf://test",
                "IGNORE_PATTERNS": "*.log,*.txt",
                "ACCESS_TOKEN": "token",
            },
            {
                "storage_uri": "hf://test",
                "ignore_patterns": ["*.log", "*.txt"],
                "access_token": "token",
            },
        ),
        (
            types.HuggingFaceDatasetInitializer,
            {"STORAGE_URI": "hf://test"},
            {"storage_uri": "hf://test", "ignore_patterns": None, "access_token": None},
        ),
        (
            types.S3DatasetInitializer,
            {
                "STORAGE_URI": "s3://bucket/path",
                "ENDPOINT": "https://s3.amazonaws.com",
                "ACCESS_KEY_ID": "test_key",
                "SECRET_ACCESS_KEY": "test_secret",
                "REGION": "us-east-1",
                "ROLE_ARN": "arn:aws:iam::123456789012:role/TestRole",
            },
            {
                "storage_uri": "s3://bucket/path",
                "ignore_patterns": None,
                "endpoint": "https://s3.amazonaws.com",
                "access_key_id": "test_key",
                "secret_access_key": "test_secret",
                "region": "us-east-1",
                "role_arn": "arn:aws:iam::123456789012:role/TestRole",
            },
        ),
        (
            types.S3DatasetInitializer,
            {"STORAGE_URI": "s3://bucket/path"},
            {
                "storage_uri": "s3://bucket/path",
                "ignore_patterns": None,
                "endpoint": None,
                "access_key_id": None,
                "secret_access_key": None,
                "region": None,
                "role_arn": None,
            },
        ),
        (
            types.S3DatasetInitializer,
            {
                "STORAGE_URI": "s3://bucket/path",
                "ACCESS_KEY_ID": "test_key",
                "SECRET_ACCESS_KEY": "test_secret",
            },
            {
                "storage_uri": "s3://bucket/path",
                "ignore_patterns": None,
                "endpoint": None,
                "access_key_id": "test_key",
                "secret_access_key": "test_secret",
                "region": None,
                "role_arn": None,
            },
        ),
        (
            types.S3ModelInitializer,
            {
                "STORAGE_URI": "s3://bucket/path",
                "IGNORE_PATTERNS": "*.log,*.txt",
                "ENDPOINT": "https://s3.amazonaws.com",
                "ACCESS_KEY_ID": "test_key",
                "SECRET_ACCESS_KEY": "test_secret",
                "REGION": "us-east-1",
                "ROLE_ARN": "arn:aws:iam::123456789012:role/TestRole",
            },
            {
                "storage_uri": "s3://bucket/path",
                "ignore_patterns": ["*.log", "*.txt"],
                "endpoint": "https://s3.amazonaws.com",
                "access_key_id": "test_key",
                "secret_access_key": "test_secret",
                "region": "us-east-1",
                "role_arn": "arn:aws:iam::123456789012:role/TestRole",
            },
        ),
        (
            types.S3ModelInitializer,
            {"STORAGE_URI": "s3://bucket/path"},
            {
                "storage_uri": "s3://bucket/path",
                "ignore_patterns": None,
                "endpoint": None,
                "access_key_id": None,
                "secret_access_key": None,
                "region": None,
                "role_arn": None,
            },
        ),
    ],
)
def test_get_config_from_env(mock_env_vars, config_class, env_vars, expected):
    mock_env_vars(**env_vars)
    result = utils.get_config_from_env(config_class)
    assert result == expected
