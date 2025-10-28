import os
from abc import ABC, abstractmethod
from dataclasses import fields
from typing import Dict

STORAGE_URI_ENV = "STORAGE_URI"
HF_SCHEME = "hf"
CACHE_SCHEME = "cache"
S3_SCHEME = "s3"

# The default path to the users' workspace.
# TODO (andreyvelich): Discuss how to keep this path is sync with Kubeflow SDK constants.
WORKSPACE_PATH = "/workspace"

# The path where initializer downloads dataset.
DATASET_PATH = os.path.join(WORKSPACE_PATH, "dataset")

# The path where initializer downloads model.
MODEL_PATH = os.path.join(WORKSPACE_PATH, "model")


class ModelProvider(ABC):
    @abstractmethod
    def load_config(self):
        raise NotImplementedError()

    @abstractmethod
    def download_model(self):
        raise NotImplementedError()


class DatasetProvider(ABC):
    @abstractmethod
    def load_config(self):
        raise NotImplementedError()

    @abstractmethod
    def download_dataset(self):
        raise NotImplementedError()


# Get DataClass config from the environment variables.
# Env names must be equal to the DataClass parameters.
def get_config_from_env(config) -> Dict:
    config_from_env = {}

    for field in fields(config):
        env_value = os.getenv(field.name.upper())

        if field.name == "ignore_patterns":
            config_from_env[field.name] = (
                [item.strip() for item in env_value.split(",") if item.strip()]
                if env_value
                else None
            )
        else:
            config_from_env[field.name] = env_value if env_value else None

    return config_from_env
