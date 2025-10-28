"""OpenDAL utilities."""

import logging
from abc import ABC, abstractmethod
from pathlib import Path
from typing import Optional

import opendal


class OpenDALStorage(ABC):
    @abstractmethod
    def download(
        self,
        prefix: str,
        destination_path: str,
        ignore_patterns: Optional[list[str]] = None,
    ):
        raise NotImplementedError()


class S3Storage(OpenDALStorage):
    def __init__(
        self,
        bucket: str,
        endpoint: Optional[str] = None,
        access_key_id: Optional[str] = None,
        secret_access_key: Optional[str] = None,
        region: Optional[str] = None,
        role_arn: Optional[str] = None,
    ):
        config = {
            "root": "/",
            "bucket": bucket,
        }

        if endpoint:
            config["endpoint"] = endpoint

        if access_key_id:
            config["access_key_id"] = access_key_id

        if secret_access_key:
            config["secret_access_key"] = secret_access_key

        if region:
            config["region"] = region
        else:
            config["region"] = "auto"

        if role_arn:
            config["role_arn"] = role_arn

        retry_layer = opendal.layers.RetryLayer(max_times=3, factor=2.0, jitter=True)
        self.op = opendal.Operator("s3", **config).layer(retry_layer)

        self.bucket = bucket

    def download(
        self,
        prefix: str,
        destination_path: str,
        ignore_patterns: Optional[list[str]] = None,
    ):
        logging.info(
            f"Downloading files from S3 bucket: {self.bucket}, prefix: {prefix}"
        )
        logging.info("-" * 40)

        try:
            destination = Path(destination_path)
            destination.mkdir(parents=True, exist_ok=True)

            # List all objects with the given prefix
            entries = self.op.list(prefix, recursive=True)

            for entry in entries:
                if entry.metadata.is_dir:
                    continue

                key = entry.path
                if ignore_patterns:
                    if key.endswith(tuple(ignore_patterns)):
                        logging.info(f"Skipping ignored file: {key}")
                        continue

                # Create relative path from the prefix
                relative_path = key[len(prefix) :].lstrip("/")
                if not relative_path:
                    # If prefix matches exactly, use the filename
                    relative_path = Path(key).name

                file_destination_path = destination / relative_path

                # Create directory if needed
                file_destination_path.parent.mkdir(parents=True, exist_ok=True)

                # Download the file via OpenDAL
                logging.info(f"Downloading {key} to {file_destination_path}")
                data = self.op.read(key)
                with open(file_destination_path, "wb") as f:
                    f.write(data)

        except Exception as e:
            logging.error(f"Unexpected error downloading files: {e}")
            raise

        logging.info("Files have been downloaded")
