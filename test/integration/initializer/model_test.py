import os
import runpy

import pytest
from conftest import verify_downloaded_files

import pkg.initializer.utils.utils as utils


class TestModelIntegration:
    """Integration tests for model initialization"""

    @pytest.fixture(autouse=True)
    def setup_teardown(self, setup_temp_path):
        self.temp_dir = setup_temp_path("MODEL_PATH")

    @pytest.mark.parametrize(
        "test_name, provider, test_case",
        [
            # Public HuggingFace model test
            (
                "HuggingFace - Public model",
                "huggingface",
                {
                    "storage_uri": "hf://hf-internal-testing/tiny-random-bert",
                    "access_token": None,
                    "expected_files": [
                        "config.json",
                        "model.safetensors",
                        "tokenizer.json",
                        "tokenizer_config.json",
                    ],
                    "expected_error": None,
                },
            ),
            (
                "HuggingFace - Invalid model",
                "huggingface",
                {
                    "storage_uri": "hf://invalid/nonexistent-model",
                    "access_token": None,
                    "expected_files": None,
                    "expected_error": Exception,
                },
            ),
            (
                "HuggingFace - Login failure",
                "huggingface",
                {
                    "storage_uri": "hf://hf-internal-testing/tiny-random-bert",
                    "access_token": "invalid token",
                    "expected_files": None,
                    "expected_error": Exception,
                },
            ),
        ],
    )
    def test_model_download(self, test_name, provider, test_case):
        """Test end-to-end model download for different providers"""
        print(f"Running Integration test for {provider}: {test_name}")

        # Setup environment variables based on test case
        os.environ[utils.STORAGE_URI_ENV] = test_case["storage_uri"]
        expected_files = test_case.get("expected_files")

        # Handle token/credentials
        if test_case.get("access_token"):
            os.environ["ACCESS_TOKEN"] = test_case["access_token"]

        # Run the main script
        if test_case["expected_error"]:
            with pytest.raises(test_case["expected_error"]):
                runpy.run_module("pkg.initializer.model.__main__", run_name="__main__")
        else:
            runpy.run_module("pkg.initializer.model.__main__", run_name="__main__")
            verify_downloaded_files(self.temp_dir, expected_files)

        print("Test execution completed")
