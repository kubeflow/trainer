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

import logging
import os
from urllib.parse import urlparse

import huggingface_hub

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HuggingFace(utils.ModelProvider):

    def load_config(self):
        config_dict = utils.get_config_from_env(types.HuggingFaceModelInitializer)
        self.config = types.HuggingFaceModelInitializer(**config_dict)

    def download_model(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)
        model_uri = storage_uri_parsed.netloc + storage_uri_parsed.path

        logging.info(f"Downloading model: {model_uri}")
        logging.info("-" * 40)

        if self.config.access_token:
            huggingface_hub.login(self.config.access_token)

        # Define pattern sets for different weight formats.
        # Following vLLM approach: try safetensors first (preferred), fallback to PyTorch formats.
        # Ref: https://github.com/kubeflow/trainer/pull/2303#discussion_r1815913663
        safetensors_patterns = [
            "*.json",
            "*.safetensors",
            "*.model",
            "*.txt",
            "*.gguf",
        ]

        pytorch_patterns = [
            "*.json",
            "*.bin",
            "*.pt",
            "*.pth",
            "*.model",
            "*.txt",
            "*.gguf",
        ]

        # Try safetensors first (preferred format: faster and safer).
        logging.info("Attempting to download model with safetensors format...")
        try:
            huggingface_hub.snapshot_download(
                repo_id=model_uri,
                local_dir=utils.MODEL_PATH,
                allow_patterns=safetensors_patterns,
                ignore_patterns=self.config.ignore_patterns,
            )

            # Check if we actually got weight files.
            if self._has_weight_files([".safetensors"]):
                logging.info("Model downloaded successfully with safetensors format")
                return
            else:
                logging.warning(
                    "No safetensors files found in downloaded model. "
                    "Falling back to PyTorch formats..."
                )
        except Exception as e:
            logging.warning(
                f"Failed to download with safetensors format: {e}. "
                "Attempting fallback to PyTorch formats..."
            )

        # Fallback: Try PyTorch formats (.bin, .pt, .pth).
        logging.info(
            "Attempting to download model with PyTorch formats (.bin, .pt, .pth)..."
        )
        try:
            huggingface_hub.snapshot_download(
                repo_id=model_uri,
                local_dir=utils.MODEL_PATH,
                allow_patterns=pytorch_patterns,
                ignore_patterns=self.config.ignore_patterns,
            )

            # Verify we got weight files.
            if self._has_weight_files([".bin", ".pt", ".pth"]):
                logging.info("Model downloaded successfully with PyTorch format")
                return
            else:
                raise ValueError(
                    f"No model weight files found for {model_uri}. "
                    "Checked for .safetensors, .bin, .pt, and .pth files. "
                    "The model repository may be incomplete or corrupted."
                )
        except Exception as e:
            raise RuntimeError(
                f"Failed to download model {model_uri}. "
                f"Tried both safetensors and PyTorch formats. "
                f"Error: {e}"
            ) from e

    def _has_weight_files(self, extensions: list[str]) -> bool:
        """
        Check if downloaded model contains weight files with given extensions.

        Args:
            extensions: List of file extensions to check (e.g., ['.safetensors', '.bin'])

        Returns:
            True if at least one weight file with the given extensions exists
        """
        if not os.path.exists(utils.MODEL_PATH):
            return False

        try:
            for root, _, files in os.walk(utils.MODEL_PATH):
                for file in files:
                    if any(file.endswith(ext) for ext in extensions):
                        logging.debug(f"Found weight file: {file}")
                        return True
        except Exception as e:
            logging.error(f"Error checking for weight files: {e}")
            return False

        return False
