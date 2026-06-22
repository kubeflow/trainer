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


class S3(utils.ModelProvider):
    def load_config(self):
        config_dict = utils.get_config_from_env(types.S3ModelInitializer)
        self.config = types.S3ModelInitializer(**config_dict)

    def download_model(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)
        bucket = storage_uri_parsed.netloc
        prefix = storage_uri_parsed.path.lstrip("/")

        s3_storage = opendal_utils.S3Storage(
            bucket=bucket,
            endpoint=self.config.endpoint,
            access_key_id=self.config.access_key_id,
            secret_access_key=self.config.secret_access_key,
            region=self.config.region,
            role_arn=self.config.role_arn,
        )

        s3_storage.download(
            prefix=prefix,
            destination_path=utils.MODEL_PATH,
            ignore_patterns=self.config.ignore_patterns,
        )
