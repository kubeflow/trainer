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
            },
        ),
        (
            "Successful download with S3 provider",
            {
                "storage_uri": "s3://dataset/path",
                "expected_error": None,
            },
        ),
        (
            "Missing storage URI environment variable",
            {
                "storage_uri": None,
                "access_token": None,
                "expected_error": Exception,
            },
        ),
        (
            "Invalid storage URI scheme",
            {
                "storage_uri": "invalid://dataset/path",
                "access_token": None,
                "expected_error": Exception,
            },
        ),
    ],
)
def test_dataset_main(test_name, test_case, mock_env_vars):
    """Test main script with different scenarios"""
    print(f"Running test: {test_name}")

    # Setup mock environment variables
    env_vars = {
        "STORAGE_URI": test_case["storage_uri"],
        "ACCESS_TOKEN": test_case.get("access_token", None),
    }
    mock_env_vars(**env_vars)

    # Setup mock instances
    mock_hf_instance = MagicMock()
    mock_s3_instance = MagicMock()

    with patch(
        "pkg.initializers.dataset.__main__.HuggingFace",
        return_value=mock_hf_instance,
    ) as mock_hf, patch(
        "pkg.initializers.dataset.__main__.S3",
        return_value=mock_s3_instance,
    ) as mock_s3:

        # Execute test
        if test_case["expected_error"]:
            with pytest.raises(test_case["expected_error"]):
                main()
        else:
            main()

            # Verify appropriate provider instance methods were called
            if test_case["storage_uri"] and test_case["storage_uri"].startswith(
                "hf://"
            ):
                mock_hf_instance.load_config.assert_called_once()
                mock_hf_instance.download_dataset.assert_called_once()
                mock_hf.assert_called_once()
            elif test_case["storage_uri"] and test_case["storage_uri"].startswith(
                "s3://"
            ):
                mock_s3_instance.load_config.assert_called_once()
                mock_s3_instance.download_dataset.assert_called_once()
                mock_s3.assert_called_once()

    print("Test execution completed")
