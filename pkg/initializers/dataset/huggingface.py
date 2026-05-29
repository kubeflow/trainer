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
from urllib.parse import urlparse

import huggingface_hub

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HuggingFace(utils.DatasetProvider):

    def load_config(self):
        config_dict = utils.get_config_from_env(types.HuggingFaceDatasetInitializer)
        self.config = types.HuggingFaceDatasetInitializer(**config_dict)

    def download_dataset(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)
        dataset_uri = (
            storage_uri_parsed.netloc + "/" + storage_uri_parsed.path.split("/")[1]
        )

        logging.info(f"Downloading dataset: {dataset_uri}")
        logging.info("-" * 40)

        if self.config.access_token:
            huggingface_hub.login(self.config.access_token)

        huggingface_hub.snapshot_download(
            repo_id=dataset_uri,
            repo_type="dataset",
            local_dir=utils.DATASET_PATH,
            ignore_patterns=self.config.ignore_patterns,
        )

        logging.info("Dataset has been downloaded")
