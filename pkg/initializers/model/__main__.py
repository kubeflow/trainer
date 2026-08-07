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
from pkg.initializers.model.huggingface import HuggingFace
from pkg.initializers.model.s3 import S3

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


def _get_storage_uri() -> str:
    storage_uri = os.environ.get(utils.STORAGE_URI_ENV)
    if not storage_uri:
        logging.error("STORAGE_URI env variable must be set.")
        raise ValueError("STORAGE_URI environment variable must be set")
    return storage_uri


def _unsupported_scheme_error(scheme: str) -> ValueError:
    return ValueError(
        f"Unsupported model storage URI scheme {scheme!r}: expected one of "
        f"{utils.HF_SCHEME!r} or {utils.S3_SCHEME!r}"
    )


def main():
    logging.info("Starting pre-trained model initialization")

    storage_uri = _get_storage_uri()
    scheme = urlparse(storage_uri).scheme

    match scheme:
        # TODO (andreyvelich): Implement more model providers.
        case utils.HF_SCHEME:
            hf = HuggingFace()
            hf.load_config()
            hf.download_model()
        case utils.S3_SCHEME:
            s3 = S3()
            s3.load_config()
            s3.download_model()
        case _:
            logging.error(
                "Unsupported model storage URI scheme %r: expected one of %r or %r",
                scheme,
                utils.HF_SCHEME,
                utils.S3_SCHEME,
            )
            raise _unsupported_scheme_error(scheme)


if __name__ == "__main__":
    main()
