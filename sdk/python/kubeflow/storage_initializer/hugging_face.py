import json
import logging
from dataclasses import dataclass, field
from typing import Optional, Union
from urllib.parse import urlparse

import transformers
from peft import LoraConfig

from .abstract_dataset_provider import datasetProvider
from .abstract_model_provider import modelProvider
from .constants import VOLUME_PATH_DATASET, VOLUME_PATH_MODEL

TRANSFORMER_TYPES = Union[
    transformers.AutoModelForSequenceClassification,
    transformers.AutoModelForTokenClassification,
    transformers.AutoModelForQuestionAnswering,
    transformers.AutoModelForCausalLM,
    transformers.AutoModelForMaskedLM,
    transformers.AutoModelForImageClassification,
]


# Configure logger.
log_formatter = logging.Formatter(
    "%(asctime)s %(levelname)-8s %(message)s", "%Y-%m-%dT%H:%M:%SZ"
)
logger = logging.getLogger(__file__)
console_handler = logging.StreamHandler()
console_handler.setFormatter(log_formatter)
logger.addHandler(console_handler)
logger.setLevel(logging.INFO)


@dataclass
class HuggingFaceModelParams:
    model_uri: str
    transformer_type: TRANSFORMER_TYPES
    access_token: str = None
    num_labels: Optional[int] = None

    def __post_init__(self):
        # Custom checks or validations can be added here
        if self.model_uri == "" or self.model_uri is None:
            raise ValueError("model_uri cannot be empty.")


@dataclass
class HuggingFaceTrainerParams:
    training_parameters: transformers.TrainingArguments = field(
        default_factory=transformers.TrainingArguments
    )
    lora_config: LoraConfig = field(default_factory=LoraConfig)


class HuggingFace(modelProvider):
    def load_config(self, serialised_args):
        # implementation for loading the config
        self.config = HuggingFaceModelParams(**json.loads(serialised_args))

    def download_model_and_tokenizer(self):
        # implementation for downloading the model
        logger.info("Downloading model")
        logger.info("-" * 40)
        transformer_type_class = getattr(transformers, self.config.transformer_type)
        parsed_uri = urlparse(self.config.model_uri)
        self.model = parsed_uri.netloc + parsed_uri.path
        transformer_type_class.from_pretrained(
            self.model,
            token=self.config.access_token,
            cache_dir=VOLUME_PATH_MODEL,
            trust_remote_code=True,
        )
        transformers.AutoTokenizer.from_pretrained(
            self.model,
            token=self.config.access_token,
            cache_dir=VOLUME_PATH_MODEL,
        )


@dataclass
class HuggingFaceDatasetParams:
    repo_id: str
    access_token: Optional[str] = None
    # TODO (andreyvelich): Discuss where we should specify dataset preprocess parameters.
    split: Optional[str] = None

    def __post_init__(self):
        # Custom checks or validations can be added here
        if self.repo_id == "" or self.repo_id is None:
            raise ValueError("repo_id is None")


class HuggingFaceDataset(datasetProvider):
    def load_config(self, serialised_args):
        self.config = HuggingFaceDatasetParams(**json.loads(serialised_args))

    def download_dataset(self):
        logger.info("Downloading dataset")
        logger.info("-" * 40)
        import huggingface_hub
        from datasets import load_dataset

        if self.config.access_token:
            huggingface_hub.login(self.config.access_token)

        # Load dataset and save to disk.
        dataset = load_dataset(self.config.repo_id, split=self.config.split)
        dataset.save_to_disk(VOLUME_PATH_DATASET)
