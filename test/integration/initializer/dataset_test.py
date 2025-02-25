import os
import runpy

import pytest
from conftest import verify_downloaded_files

import pkg.initializer.utils.utils as utils


class TestDatasetIntegration:
    """Integration tests for dataset initialization"""

    @pytest.fixture(autouse=True)
    def setup_teardown(self, setup_temp_path):
        self.temp_dir = setup_temp_path("DATASET_PATH")

    @pytest.mark.parametrize(
        "test_name, provider, test_case",
        [
            # Public HuggingFace dataset test
            (
                "HuggingFace - Public dataset",
                "huggingface",
                {
                    "storage_uri": "hf://karpathy/tiny_shakespeare",
                    "access_token": None,
                    "expected_files": ["tiny_shakespeare.py"],
                    "expected_error": None,
                },
            ),
            (
                "HuggingFace - Invalid dataset",
                "huggingface",
                {
                    "storage_uri": "hf://invalid/nonexistent-dataset",
                    "access_token": None,
                    "expected_files": None,
                    "expected_error": Exception,
                },
            ),
            (
                "HuggingFace - Login Failure",
                "huggingface",
                {
                    "storage_uri": "hf://karpathy/tiny_shakespeare",
                    "access_token": "invalid token",
                    "expected_files": None,
                    "expected_error": Exception,
                },
            ),
        ],
    )
    def test_dataset_download(self, test_name, provider, test_case):
        """Test end-to-end dataset download for different providers"""
        print(f"Running Integration test for {provider}: {test_name}")

        # Setup environment variables based on test case
        os.environ[utils.STORAGE_URI_ENV] = test_case["storage_uri"]
        expected_files = test_case.get("expected_files")

        if test_case.get("access_token"):
            os.environ["ACCESS_TOKEN"] = test_case["access_token"]

        # Run the main script
        if test_case["expected_error"]:
            with pytest.raises(test_case["expected_error"]):
                runpy.run_module(
                    "pkg.initializer.dataset.__main__", run_name="__main__"
                )
        else:
            runpy.run_module("pkg.initializer.dataset.__main__", run_name="__main__")
            verify_downloaded_files(self.temp_dir, expected_files)

        print("Test execution completed")
