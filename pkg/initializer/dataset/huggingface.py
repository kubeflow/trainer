import logging
from urllib.parse import urlparse

import huggingface_hub
from kubeflow.trainer import DATASET_PATH, HuggingFaceDatasetConfig

import pkg.initializer.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HuggingFace(utils.DatasetProvider):

    def load_config(self):
        config_dict = utils.get_config_from_env(HuggingFaceDatasetConfig)
        self.config = HuggingFaceDatasetConfig(**config_dict)

    def download_dataset(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)
        dataset_uri = storage_uri_parsed.netloc + storage_uri_parsed.path

        logging.info(f"Downloading dataset: {dataset_uri}")
        logging.info("-" * 40)

        if self.config.access_token:
            huggingface_hub.login(self.config.access_token)

        huggingface_hub.snapshot_download(
            repo_id=dataset_uri,
            repo_type="dataset",
            local_dir=DATASET_PATH,
        )

        logging.info("Dataset has been downloaded")
