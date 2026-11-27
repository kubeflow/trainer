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

"""OpenDAL utilities."""

import fnmatch
import logging
import os
import stat
import tempfile
from abc import ABC, abstractmethod
from pathlib import Path
from typing import Optional

import opendal
from opendal.exceptions import NotFound

# Size of the chunks that an object is streamed to disk in, which bounds the memory
# used for file content regardless of the object size.
DOWNLOAD_CHUNK_SIZE = 8 * 1024 * 1024


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
        elif endpoint:
            # S3-compatible endpoints such as MinIO or Cloudflare R2 do not use AWS
            # regions; "auto" satisfies them without a real region.
            config["region"] = "auto"
        # Without an endpoint the operator talks to AWS S3, where the region selects
        # the endpoint and signs requests, so leave it to OpenDAL to resolve it from
        # AWS_REGION / AWS_DEFAULT_REGION rather than pin the bogus "auto".

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

            keys = self._resolve_keys(prefix)
            base = prefix.rstrip("/")

            downloaded = 0
            for key in keys:
                if ignore_patterns:
                    if any(fnmatch.fnmatch(key, p) for p in ignore_patterns):
                        logging.info(f"Skipping ignored file: {key}")
                        continue

                # Create relative path from the prefix
                relative_path = key[len(base) :].lstrip("/")
                if not relative_path:
                    # If prefix matches exactly, use the filename
                    relative_path = Path(key).name

                file_destination_path = destination / relative_path

                # Create directory if needed
                file_destination_path.parent.mkdir(parents=True, exist_ok=True)

                # Download the file via OpenDAL
                logging.info(f"Downloading {key} to {file_destination_path}")
                self._download_object(key, file_destination_path)
                downloaded += 1

            if downloaded == 0:
                logging.warning(
                    f"All {len(keys)} objects under s3://{self.bucket}/{prefix} "
                    "were skipped by the ignore patterns, nothing was downloaded"
                )

        except Exception as e:
            logging.error(f"Unexpected error downloading files: {e}")
            raise

        logging.info("Files have been downloaded")

    def _download_object(self, key: str, destination: Path):
        """Stream one object to the destination file.

        The object is written to a temporary file in the destination directory and
        moved to the destination only once it has been fully written, so a failed
        download never leaves a partial file at the destination or changes an
        existing one. The destination gets the same mode as a file written with
        open(), so the files stay readable from a shared volume.
        """
        mode = _destination_mode(destination)
        fd, temp_name = tempfile.mkstemp(
            dir=destination.parent, prefix=".s3dl-", suffix=".part"
        )
        try:
            with os.fdopen(fd, "wb") as dst, self.op.open(key, "rb") as src:
                # A read can return less than the chunk size before the end of the
                # object, only an empty read means the end.
                while chunk := src.read(DOWNLOAD_CHUNK_SIZE):
                    dst.write(chunk)
            os.chmod(temp_name, mode)
            os.replace(temp_name, destination)
        except BaseException:
            try:
                os.remove(temp_name)
            except FileNotFoundError:
                pass
            except OSError as e:
                logging.error(f"Failed to remove temporary file {temp_name}: {e}")
            raise

    def _resolve_keys(self, prefix: str) -> list[str]:
        """Return the object keys that the prefix refers to.

        OpenDAL lists by prefix, like S3 ListObjectsV2, so listing "models/llama"
        would also return "models/llama-2/...". Follow the OpenDAL convention
        instead: a path without a trailing "/" names an object, and a path with
        one names a directory. A path without a trailing "/" that is not an object
        falls back to the directory with the same name, and the exact object takes
        precedence when both exist.
        """
        base = prefix.rstrip("/")

        if base and not prefix.endswith("/") and self._is_object(base):
            return [base]

        directory = f"{base}/" if base else "/"
        keys = [
            entry.path
            for entry in self.op.list(directory, recursive=True)
            if not entry.metadata.is_dir
        ]
        if keys:
            return keys

        location = f"s3://{self.bucket}/{base}"
        if base and not prefix.endswith("/"):
            message = f"No object at {location} and no objects under {location}/"
        else:
            message = f"No objects under {location}/"
        raise FileNotFoundError(
            f"{message}. Objects are matched by their exact key or by directory, "
            "not by a partial name prefix."
        )

    def _is_object(self, key: str) -> bool:
        try:
            return bool(self.op.stat(key).is_file)
        except NotFound:
            return False


def _destination_mode(destination: Path) -> int:
    """Return the mode that open(destination, "wb") would leave the file with.

    An existing file keeps its mode, and a new file gets the default mode masked by
    the process umask.
    """
    try:
        return stat.S_IMODE(destination.stat().st_mode)
    except FileNotFoundError:
        umask = os.umask(0)
        os.umask(umask)
        return 0o666 & ~umask
