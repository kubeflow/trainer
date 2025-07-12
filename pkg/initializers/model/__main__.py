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


def main():
    logging.info("Starting pre-trained model initialization")

    try:
        storage_uri = os.environ[utils.STORAGE_URI_ENV]
    except Exception as e:
        logging.error("STORAGE_URI env variable must be set.")
        raise e

    match urlparse(storage_uri).scheme:
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
                f"STORAGE_URI must have the valid model provider. STORAGE_URI: {storage_uri}"
            )
            raise Exception


if __name__ == "__main__":
    main()
