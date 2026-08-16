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

        allow_patterns = ["*.json", "*.safetensors", "*.model", "*.txt"]
        ignore_patterns = self.config.ignore_patterns

        # By default we skip the legacy, pickle-based PyTorch weight formats
        # in favor of safetensors, since safetensors is the safer format and
        # downloading both is wasteful. But some repos only ship the legacy
        # formats, so if that's the case here, allow them through instead of
        # silently downloading nothing usable.
        # Ref: https://github.com/kubeflow/trainer/pull/2303#discussion_r1815913663
        # Ref: https://github.com/kubeflow/trainer/issues/3909
        legacy_weight_patterns = ["*.bin", "*.pt", "*.pth"]
        if ignore_patterns and set(legacy_weight_patterns) & set(ignore_patterns):
            repo_files = huggingface_hub.list_repo_files(model_uri)
            has_safetensors = any(f.endswith(".safetensors") for f in repo_files)
            if not has_safetensors:
                allow_patterns += legacy_weight_patterns
                ignore_patterns = [
                    p for p in ignore_patterns if p not in legacy_weight_patterns
                ]

        huggingface_hub.snapshot_download(
            repo_id=model_uri,
            local_dir=utils.MODEL_PATH,
            allow_patterns=allow_patterns,
            ignore_patterns=ignore_patterns,
        )

        logging.info("Model has been downloaded")
