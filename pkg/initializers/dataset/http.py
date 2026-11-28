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
from urllib.parse import unquote, urlparse

import pkg.initializers.types.types as types
import pkg.initializers.utils.opendal as opendal_utils
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HTTP(utils.DatasetProvider):
    """HTTP/HTTPS dataset provider using OpenDAL."""

    def load_config(self):
        config_dict = utils.get_config_from_env(types.HTTPDatasetInitializer)
        self.config = types.HTTPDatasetInitializer(**config_dict)

    def download_dataset(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)

        # Extract base URL and resource path with query string
        # For https://example.com/path/to/file.zip?version=1.0:
        #   base_url = https://example.com
        #   resource_path = path/to/file.zip?version=1.0 (includes query string)
        base_url = f"{storage_uri_parsed.scheme}://{storage_uri_parsed.netloc}"

        # Decode the path once to handle encoded characters like %2F
        # but preserve it as-is for the HTTP request
        decoded_path = unquote(storage_uri_parsed.path)
        file_path = decoded_path.lstrip("/")

        if not file_path:
            raise ValueError(
                f"Invalid HTTP URI: {self.config.storage_uri}. "
                f"Expected format: http(s)://domain/path/to/file"
            )

        # Preserve query string for the HTTP request
        query_string = storage_uri_parsed.query
        resource_path = f"{file_path}?{query_string}" if query_string else file_path

        http_storage = opendal_utils.HTTPStorage(base_url=base_url)

        http_storage.download(
            prefix=resource_path,
            destination_path=utils.DATASET_PATH,
            ignore_patterns=self.config.ignore_patterns,
        )
