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

import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


def main():
    logging.info("Starting dataset initialization")

    storage_uri = os.getenv(utils.STORAGE_URI_ENV)
    if not storage_uri:
        raise ValueError("STORAGE_URI environment variable must be set")

    scheme = urlparse(storage_uri).scheme

    match scheme:
        # TODO (andreyvelich): Implement more dataset providers.
        case utils.HF_SCHEME:
            from pkg.initializers.dataset.huggingface import HuggingFace

            provider_cls = HuggingFace
        case utils.CACHE_SCHEME:
            from pkg.initializers.dataset.cache import CacheInitializer

            provider_cls = CacheInitializer
        case utils.S3_SCHEME:
            from pkg.initializers.dataset.s3 import S3

            provider_cls = S3
        case _:
            raise ValueError(
                f"Unsupported dataset storage URI scheme {scheme!r}: "
                f"expected one of {utils.HF_SCHEME!r}, "
                f"{utils.CACHE_SCHEME!r}, or {utils.S3_SCHEME!r}"
            )

    provider = provider_cls()
    provider.load_config()
    provider.download_dataset()


if __name__ == "__main__":
    main()
