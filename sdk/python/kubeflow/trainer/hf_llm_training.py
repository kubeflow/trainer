import argparse
import json
import logging
import os
from urllib.parse import urlparse

import transformers
from datasets import Dataset, load_from_disk
from datasets.distributed import split_dataset_by_node
from peft import LoraConfig, get_peft_model
from transformers import (
    AutoModelForCausalLM,
    AutoModelForImageClassification,
    AutoTokenizer,
    DataCollatorForLanguageModeling,
    DataCollatorWithPadding,
    Trainer,
    TrainingArguments,
)

# Configure logger.
log_formatter = logging.Formatter(
    "%(asctime)s %(levelname)-8s %(message)s", "%Y-%m-%dT%H:%M:%SZ"
)
logger = logging.getLogger(__file__)
console_handler = logging.StreamHandler()
console_handler.setFormatter(log_formatter)
logger.addHandler(console_handler)
logger.setLevel(logging.INFO)


def setup_model_and_tokenizer(model_uri, transformer_type, model_dir, num_labels):
    # Set up the model and tokenizer
    parsed_uri = urlparse(model_uri)
    model_name = parsed_uri.netloc + parsed_uri.path

    if num_labels != "None":
        model = transformer_type.from_pretrained(
            pretrained_model_name_or_path=model_name,
            cache_dir=model_dir,
            local_files_only=True,
            trust_remote_code=True,
            num_labels=int(num_labels),
        )
    else:
        model = transformer_type.from_pretrained(
            pretrained_model_name_or_path=model_name,
            cache_dir=model_dir,
            local_files_only=True,
            trust_remote_code=True,
        )

    tokenizer = AutoTokenizer.from_pretrained(
        pretrained_model_name_or_path=model_name,
        cache_dir=model_dir,
        local_files_only=True,
    )

    # Freeze model parameters
    for param in model.parameters():
        param.requires_grad = False

    if not tokenizer.pad_token:
        tokenizer.pad_token = tokenizer.eos_token

    return model, tokenizer


def load_and_preprocess_data(dataset_dir, transformer_type, tokenizer):
    # Load and preprocess the dataset
    logger.info("Load and preprocess dataset")

    if transformer_type != AutoModelForImageClassification:
        dataset = load_from_disk(dataset_dir)

        logger.info(f"Dataset specification: {dataset}")
        logger.info("-" * 40)

        logger.info("Tokenize dataset")
        # TODO (andreyvelich): Discuss how user should set the tokenizer function.
        dataset = dataset.map(
            lambda x: tokenizer(x["text"], padding="max_length", truncation=True),
            batched=True,
        )
    else:
        dataset = load_from_disk(dataset_dir)

    # Check if dataset contains `train` key. Otherwise, load full dataset to train_data.
    if "train" in dataset:
        train_data = dataset["train"]
    else:
        train_data = dataset

    try:
        eval_data = dataset["eval"]
    except Exception:
        eval_data = None
        logger.info("Evaluation dataset is not found")

    # Distribute dataset across PyTorchJob workers.
    RANK = int(os.environ["RANK"])
    WORLD_SIZE = int(os.environ["WORLD_SIZE"])
    logger.info(
        f"Distributed dataset across PyTorchJob workers. WORLD_SIZE: {WORLD_SIZE}, RANK: {RANK}"
    )
    if isinstance(train_data, Dataset):
        train_data = split_dataset_by_node(
            train_data,
            rank=RANK,
            world_size=WORLD_SIZE,
        )
    if isinstance(eval_data, Dataset):
        eval_data = split_dataset_by_node(
            eval_data,
            rank=RANK,
            world_size=WORLD_SIZE,
        )

    return train_data, eval_data


def setup_peft_model(model, lora_config):
    # Set up the PEFT model
    lora_config = LoraConfig(**json.loads(lora_config))
    reference_lora_config = LoraConfig()
    for key, val in lora_config.__dict__.items():
        old_attr = getattr(reference_lora_config, key, None)
        if old_attr is not None:
            val = type(old_attr)(val)
        setattr(lora_config, key, val)

    model.enable_input_require_grads()
    model = get_peft_model(model, lora_config)
    return model


def train_model(model, transformer_type, train_data, eval_data, tokenizer, train_args):
    # Setup the Trainer.
    trainer = Trainer(
        model=model,
        train_dataset=train_data,
        eval_dataset=eval_data,
        args=train_args,
    )

    # TODO (andreyvelich): Currently, data collator is supported only for casual LM Transformer.
    if transformer_type == AutoModelForCausalLM:
        logger.info("Add data collector for language modeling")
        logger.info("-" * 40)
        trainer.data_collator = DataCollatorForLanguageModeling(
            tokenizer,
            pad_to_multiple_of=8,
            mlm=False,
        )
    else:
        logger.info("Add general data collator with padding")
        logger.info("-" * 40)
        trainer.data_collator = DataCollatorWithPadding(
            tokenizer,
            pad_to_multiple_of=8,
        )

    # Train the model.
    trainer.train()


def parse_arguments():
    parser = argparse.ArgumentParser(
        description="Script for training a model with PEFT configuration."
    )

    parser.add_argument("--model_uri", help="model uri")
    parser.add_argument("--transformer_type", help="model transformer type")
    parser.add_argument("--num_labels", default="None", help="number of classes")
    parser.add_argument("--model_dir", help="directory containing model")
    parser.add_argument("--dataset_dir", help="directory containing dataset")
    parser.add_argument("--lora_config", help="lora_config")
    parser.add_argument(
        "--training_parameters", help="hugging face training parameters"
    )

    return parser.parse_args()


if __name__ == "__main__":
    logger.info("Starting HuggingFace LLM Trainer")
    args = parse_arguments()
    train_args = TrainingArguments(**json.loads(args.training_parameters))
    reference_train_args = transformers.TrainingArguments(
        output_dir=train_args.output_dir
    )
    for key, val in train_args.to_dict().items():
        old_attr = getattr(reference_train_args, key, None)
        if old_attr is not None:
            val = type(old_attr)(val)
        setattr(train_args, key, val)

    transformer_type = getattr(transformers, args.transformer_type)

    logger.info("Setup model and tokenizer")
    model, tokenizer = setup_model_and_tokenizer(
        args.model_uri, transformer_type, args.model_dir, args.num_labels
    )

    logger.info("Preprocess dataset")
    train_data, eval_data = load_and_preprocess_data(
        args.dataset_dir, transformer_type, tokenizer
    )

    logger.info("Setup LoRA config for model")
    model = setup_peft_model(model, args.lora_config)

    logger.info("Start model training")
    train_model(model, transformer_type, train_data, eval_data, tokenizer, train_args)

    logger.info("Training is complete")
