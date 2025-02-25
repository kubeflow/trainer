import logging
from urllib.parse import urlparse

import huggingface_hub
from kubeflow.trainer import MODEL_PATH, HuggingFaceModelInputConfig

import pkg.initializer.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HuggingFace(utils.ModelProvider):

    def load_config(self):
        config_dict = utils.get_config_from_env(HuggingFaceModelInputConfig)
        self.config = HuggingFaceModelInputConfig(**config_dict)

    def download_model(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)
        model_uri = storage_uri_parsed.netloc + storage_uri_parsed.path

        logging.info(f"Downloading model: {model_uri}")
        logging.info("-" * 40)

        if self.config.access_token:
            huggingface_hub.login(self.config.access_token)

        # TODO (andreyvelich): We should consider to follow vLLM approach with allow patterns.
        # Ref: https://github.com/kubeflow/trainer/pull/2303#discussion_r1815913663
        # TODO (andreyvelich): We should update patterns for Mistral model
        # Ref: https://github.com/kubeflow/trainer/pull/2303#discussion_r1815914270
        huggingface_hub.snapshot_download(
            repo_id=model_uri,
            local_dir=MODEL_PATH,
            allow_patterns=["*.json", "*.safetensors", "*.model"],
            ignore_patterns=["*.msgpack", "*.h5", "*.bin", ".pt", ".pth"],
        )

        logging.info("Model has been downloaded")
