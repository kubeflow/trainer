import logging
import os
from urllib.parse import urlparse

import boto3

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class S3(utils.DatasetProvider):

    def _create_s3_client(self):
        """Create S3 client with optional credentials from config"""
        return boto3.client(
            "s3",
            endpoint_url=self.config.endpoint_url,
            aws_access_key_id=self.config.access_key_id,
            aws_secret_access_key=self.config.access_key_secret,
            region_name=self.config.region,
        )

    def load_config(self):
        config_dict = utils.get_config_from_env(types.S3DatasetInitializer)
        self.config = types.S3DatasetInitializer(**config_dict)

    def download_dataset(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)
        bucket_name = storage_uri_parsed.netloc
        prefix = storage_uri_parsed.path.lstrip("/")

        logging.info(
            f"Downloading dataset from S3 bucket: {bucket_name}, prefix: {prefix}"
        )
        logging.info("-" * 40)

        s3_client = self._create_s3_client()

        try:
            os.makedirs(utils.DATASET_PATH, exist_ok=True)

            paginator = s3_client.get_paginator("list_objects_v2")
            pages = paginator.paginate(Bucket=bucket_name, Prefix=prefix)

            for page in pages:
                if "Contents" in page:
                    for obj in page["Contents"]:
                        key = obj["Key"]
                        # Skip directories
                        if key.endswith("/"):
                            continue

                        # Create relative path from the prefix
                        relative_path = key[len(prefix) :].lstrip("/")
                        if not relative_path:
                            # If prefix matches exactly, use the filename
                            relative_path = os.path.basename(key)

                        destination_path = os.path.join(
                            utils.DATASET_PATH, relative_path
                        )

                        # Create directory if needed
                        destination_dir = os.path.dirname(destination_path)
                        if destination_dir:
                            os.makedirs(destination_dir, exist_ok=True)

                        # Download the file
                        logging.info(f"Downloading {key} to {destination_path}")
                        s3_client.download_file(bucket_name, key, destination_path)

        except Exception as e:
            logging.error(f"Unexpected error downloading dataset: {e}")
            raise

        logging.info("Dataset has been downloaded")
