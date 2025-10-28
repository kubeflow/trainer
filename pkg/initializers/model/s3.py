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
