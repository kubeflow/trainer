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

import huggingface_hub

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class HuggingFace(utils.ModelProvider):

    def load_config(self):
        config_dict = utils.get_config_from_env(types.HuggingFaceModelInitializer)
        self.config = types.HuggingFaceModelInitializer(**config_dict)

    def download_model(self):
        storage_uri_parsed = urlparse(self.config.storage_uri)
        model_uri = storage_uri_parsed.netloc + storage_uri_parsed.path

        logging.info(f"Downloading model: {model_uri}")
        logging.info("-" * 40)

        if self.config.access_token:
            huggingface_hub.login(self.config.access_token)

        allow_patterns = ["*.json", "*.safetensors", "*.model", "*.txt", "*.gguf"]
        ignore_patterns = self.config.ignore_patterns
        repo_files = huggingface_hub.list_repo_files(
            repo_id=model_uri, token=self.config.access_token
        )
        if not any(path.endswith(".safetensors") for path in repo_files):
            logging.info("No safetensors found; falling back to PyTorch weights")
            pytorch_patterns = ["*.bin", "*.pt", "*.pth"]
            allow_patterns.extend(pytorch_patterns)
            ignore_patterns = [
                p for p in (ignore_patterns or []) if p not in pytorch_patterns
            ]

        huggingface_hub.snapshot_download(
            repo_id=model_uri,
            local_dir=utils.MODEL_PATH,
            allow_patterns=allow_patterns,
            ignore_patterns=ignore_patterns,
        )

        logging.info("Model has been downloaded")
