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
import tarfile
import zipfile
from pathlib import Path
from urllib.parse import urlparse

import requests

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HTTP(utils.ModelProvider):
    """HTTP/HTTPS model provider for downloading models from web URLs."""

    def load_config(self):
        config_dict = utils.get_config_from_env(types.HTTPModelInitializer)
        self.config = types.HTTPModelInitializer(**config_dict)

    def download_model(self):
        url = self.config.storage_uri

        logging.info(f"Downloading model from: {url}")
        logging.info("-" * 40)

        # Prepare headers
        headers = self.config.headers if self.config.headers else {}

        # Download with streaming
        response = requests.get(
            url, stream=True, headers=headers, timeout=self.config.timeout
        )
        response.raise_for_status()

        # Determine filename
        filename = self._get_filename(url, response)
        temp_path = Path(utils.MODEL_PATH) / filename

        # Download file
        total_size = int(response.headers.get("content-length", 0))
        if total_size > 0:
            logging.info(f"Downloading {filename} ({total_size / 1024 / 1024:.2f} MB)")
        else:
            logging.info(f"Downloading {filename}")

        with open(temp_path, "wb") as f:
            for chunk in response.iter_content(chunk_size=8192):
                if chunk:
                    f.write(chunk)

        logging.info(f"Downloaded to: {temp_path}")

        # Extract if archive
        if filename.endswith((".tar.gz", ".tgz")):
            logging.info("Extracting tar.gz archive...")
            with tarfile.open(temp_path, "r:gz") as tar:
                tar.extractall(utils.MODEL_PATH)
            os.remove(temp_path)
            logging.info("Archive extracted and removed")
        elif filename.endswith(".tar"):
            logging.info("Extracting tar archive...")
            with tarfile.open(temp_path, "r:") as tar:
                tar.extractall(utils.MODEL_PATH)
            os.remove(temp_path)
            logging.info("Archive extracted and removed")
        elif filename.endswith(".zip"):
            logging.info("Extracting zip archive...")
            with zipfile.ZipFile(temp_path, "r") as zip_ref:
                zip_ref.extractall(utils.MODEL_PATH)
            os.remove(temp_path)
            logging.info("Archive extracted and removed")
        else:
            logging.info(f"Single file downloaded: {filename}")

        logging.info("Model has been downloaded")

    def _get_filename(self, url: str, response: requests.Response) -> str:
        """Extract filename from URL or Content-Disposition header."""
        # Try Content-Disposition header first
        if "content-disposition" in response.headers:
            content_disp = response.headers["content-disposition"]
            if "filename=" in content_disp:
                filename = content_disp.split("filename=")[1].strip('"').strip("'")
                return filename

        # Fall back to URL path
        parsed = urlparse(url)
        filename = os.path.basename(parsed.path)
        return filename if filename else "model"
