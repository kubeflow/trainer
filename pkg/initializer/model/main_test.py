from unittest.mock import MagicMock, patch

import pytest

from pkg.initializer.model.__main__ import main


@pytest.mark.parametrize(
    "test_name, test_case",
    [
        (
            "Successful download with HuggingFace provider",
            {
                "storage_uri": "hf://model/path",
                "access_token": "test_token",
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
                "storage_uri": "invalid://model/path",
                "access_token": None,
                "expected_error": Exception,
            },
        ),
    ],
)
def test_model_main(test_name, test_case, mock_env_vars):
    """Test main script with different scenarios"""
    print(f"Running test: {test_name}")

    # Setup mock environment variables
    env_vars = {
        "STORAGE_URI": test_case["storage_uri"],
        "ACCESS_TOKEN": test_case["access_token"],
    }
    mock_env_vars(**env_vars)

    # Setup mock HuggingFace instance
    mock_hf_instance = MagicMock()

    with patch(
        "pkg.initializer.model.__main__.HuggingFace",
        return_value=mock_hf_instance,
    ) as mock_hf:

        # Execute test
        if test_case["expected_error"]:
            with pytest.raises(test_case["expected_error"]):
                main()
        else:
            main()

            # Verify HuggingFace instance methods were called
            mock_hf_instance.load_config.assert_called_once()
            mock_hf_instance.download_model.assert_called_once()

        # Verify HuggingFace class instantiation
        if test_case["storage_uri"] and test_case["storage_uri"].startswith("hf://"):
            mock_hf.assert_called_once()

    print("Test execution completed")
