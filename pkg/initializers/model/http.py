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

import pkg.initializers.types.types as types
import pkg.initializers.utils.opendal as opendal_utils
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HTTP(utils.ModelProvider):
    """HTTP/HTTPS model provider using OpenDAL."""

    def load_config(self):
        config_dict = utils.get_config_from_env(types.HTTPModelInitializer)
        self.config = types.HTTPModelInitializer(**config_dict)

    def download_model(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)

        # Extract base URL and file path
        # For https://example.com/path/to/model.bin:
        #   base_url = https://example.com
        #   file_path = path/to/model.bin
        base_url = f"{storage_uri_parsed.scheme}://{storage_uri_parsed.netloc}"
        file_path = storage_uri_parsed.path.lstrip("/")

        if not file_path:
            raise ValueError(
                f"Invalid HTTP URI: {self.config.storage_uri}. "
                f"Expected format: http(s)://domain/path/to/file"
            )

        http_storage = opendal_utils.HTTPStorage(base_url=base_url)

        http_storage.download(
            prefix=file_path,
            destination_path=utils.MODEL_PATH,
            ignore_patterns=self.config.ignore_patterns,
        )
