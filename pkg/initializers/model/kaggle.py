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

import kagglehub

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class Kaggle(utils.ModelProvider):
    """Kaggle model provider for downloading models from Kaggle."""

    def load_config(self):
        config_dict = utils.get_config_from_env(types.KaggleModelInitializer)
        self.config = types.KaggleModelInitializer(**config_dict)

    def download_model(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)

        # urlparse treats the part after :// as netloc, path comes after
        # For kaggle://models/google/bert/tensorFlow2/answer-equivalence-bem:
        #   netloc='models', path='/google/bert/tensorFlow2/answer-equivalence-bem'
        if storage_uri_parsed.netloc != "models":
            raise ValueError(
                f"Invalid Kaggle model URI: {self.config.storage_uri}. "
                f"Expected format: kaggle://models/<username>/<model>/<framework>/<variation>"
            )

        path = storage_uri_parsed.path.lstrip("/")
        if not path:
            raise ValueError(
                f"Invalid Kaggle model URI: {self.config.storage_uri}. "
                f"Expected format: kaggle://models/<username>/<model>/<framework>/<variation>"
            )

        path_parts = path.split("/")
        if len(path_parts) < 4:
            raise ValueError(
                f"Invalid Kaggle model URI: {self.config.storage_uri}. "
                f"Expected format: kaggle://models/<username>/<model>/<framework>/<variation>"
            )

        model_handle = path

        logging.info(f"Downloading Kaggle model: {model_handle}")
        logging.info("-" * 40)

        if self.config.access_token:
            os.environ["KAGGLE_API_TOKEN"] = self.config.access_token

        os.makedirs(utils.MODEL_PATH, exist_ok=True)

        try:
            kagglehub.model_download(
                handle=model_handle,
                output_dir=utils.MODEL_PATH,
            )
            logging.info("Model successfully downloaded")
        except Exception as e:
            logging.error(f"Model download failed: {e}")
            raise

        logging.info("Model has been downloaded")
