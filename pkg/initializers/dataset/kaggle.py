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


class Kaggle(utils.DatasetProvider):
    """Kaggle dataset provider for downloading datasets from Kaggle."""

    def load_config(self):
        config_dict = utils.get_config_from_env(types.KaggleDatasetInitializer)
        self.config = types.KaggleDatasetInitializer(**config_dict)

    def download_dataset(self):
        # Parse kaggle://datasets/username/dataset-name or kaggle://competitions/competition-name
        storage_uri_parsed = urlparse(self.config.storage_uri)

        # urlparse treats the part after :// as netloc, so we need to combine netloc and path
        # For kaggle://competitions/titanic: netloc='competitions', path='/titanic'
        # For kaggle://datasets/user/data: netloc='datasets', path='/user/data'
        resource_type = storage_uri_parsed.netloc
        path = storage_uri_parsed.path.lstrip("/")

        if not resource_type:
            raise ValueError(
                f"Invalid Kaggle URI: {self.config.storage_uri}. "
                f"Expected format: kaggle://datasets/<username>/<dataset> or "
                f"kaggle://competitions/<competition>"
            )

        if resource_type == "datasets":
            # path should be username/dataset-name
            if not path or "/" not in path:
                raise ValueError(
                    f"Invalid Kaggle dataset URI: {self.config.storage_uri}. "
                    f"Expected format: kaggle://datasets/<username>/<dataset>"
                )
            # Use path as the dataset handle: username/dataset-name
            dataset_handle = path

            logging.info(f"Downloading Kaggle dataset: {dataset_handle}")
            logging.info("-" * 40)

            # Set authentication if provided
            if self.config.access_token:
                os.environ["KAGGLE_API_TOKEN"] = self.config.access_token

            # Download dataset using kagglehub
            kagglehub.dataset_download(
                handle=dataset_handle,
                output_dir=utils.DATASET_PATH,
            )

        elif resource_type == "competitions":
            # path should be competition-name
            if not path:
                raise ValueError(
                    f"Invalid Kaggle competition URI: {self.config.storage_uri}. "
                    f"Expected format: kaggle://competitions/<competition>"
                )
            # Use path as the competition name
            competition_name = path

            logging.info(f"Downloading Kaggle competition: {competition_name}")
            logging.info("-" * 40)

            # Set authentication if provided
            if self.config.access_token:
                os.environ["KAGGLE_API_TOKEN"] = self.config.access_token

            # Download competition data using kagglehub
            kagglehub.competition_download(
                handle=competition_name,
                output_dir=utils.DATASET_PATH,
            )

        else:
            raise ValueError(
                f"Invalid Kaggle resource type: {resource_type}. "
                f"Expected 'datasets' or 'competitions'"
            )

        logging.info("Dataset has been downloaded")
