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

"""GRPO training entrypoint for Kubeflow Trainer.

This script launches TRL GRPOTrainer for reinforcement learning post-training
using Group Relative Policy Optimization. It reads configuration from environment
variables and command-line arguments, following the Kubeflow Trainer convention.

Environment Variables:
    STORAGE_URI:        URI for the pre-trained model (set by model initializer).
    DATASET_STORAGE_URI: URI for the training dataset (set by dataset initializer).
    MODEL_PATH:         Local path to the pre-trained model (default: /workspace/model).
    DATASET_PATH:       Local path to the training dataset (default: /workspace/dataset).
    OUTPUT_DIR:         Output directory for checkpoints (default: /workspace/output).
    NUM_GENERATIONS:    Number of completions per prompt (default: 4).
    MAX_PROMPT_LENGTH:  Maximum prompt token length (default: 512).
    MAX_COMPLETION_LENGTH: Maximum completion token length (default: 256).
    PER_DEVICE_TRAIN_BATCH_SIZE: Per-device batch size (default: 4).
    GRADIENT_ACCUMULATION_STEPS: Gradient accumulation steps (default: 4).
    NUM_TRAIN_EPOCHS:   Number of training epochs (default: 1).
    LEARNING_RATE:      Learning rate (default: 5e-7).
    LOGGING_STEPS:      Log every N steps (default: 1).
"""

import logging
import os
import sys
from typing import Union

from datasets import load_dataset, load_from_disk
from trl import GRPOConfig, GRPOTrainer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)-8s [%(name)s] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
)
logger = logging.getLogger(__name__)

WORKSPACE_MODEL = "/workspace/model"
WORKSPACE_DATASET = "/workspace/dataset"
WORKSPACE_OUTPUT = "/workspace/output"


def get_env(name: str, default: Union[str, None] = None) -> str:
    value = os.environ.get(name, default)
    if value is None:
        logger.error("Required environment variable %s is not set", name)
        sys.exit(1)
    return value


def get_env_int(name: str, default: int) -> int:
    return int(os.environ.get(name, str(default)))


def get_env_float(name: str, default: float) -> float:
    return float(os.environ.get(name, str(default)))


def length_reward(completions: list[str], **kwargs) -> list[float]:
    """Default reward function that rewards longer completions.

    Users should override REWARD_FUNCTION_MODULE or supply a custom
    training script for production reward functions.
    """
    return [float(len(c)) for c in completions]


def load_reward_function():
    """Load a custom reward function module if REWARD_FUNCTION_MODULE is set."""
    module_path = os.environ.get("REWARD_FUNCTION_MODULE")
    if module_path is None:
        logger.info("Using default length-based reward function")
        return length_reward

    import importlib.util

    spec = importlib.util.spec_from_file_location("reward_module", module_path)
    if spec is None or spec.loader is None:
        logger.error("Cannot load reward function module: %s", module_path)
        sys.exit(1)

    module = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(module)
    except FileNotFoundError:
        logger.error("Reward function module not found: %s", module_path)
        sys.exit(1)

    if not hasattr(module, "reward_function"):
        logger.error("Module %s must define a 'reward_function' callable", module_path)
        sys.exit(1)

    logger.info("Loaded custom reward function from %s", module_path)
    return module.reward_function


def main():
    logger.info("Starting GRPO training")

    model_path = get_env("MODEL_PATH", WORKSPACE_MODEL)
    dataset_path = get_env("DATASET_PATH", WORKSPACE_DATASET)
    output_dir = get_env("OUTPUT_DIR", WORKSPACE_OUTPUT)

    logger.info("Model path: %s", model_path)
    logger.info("Dataset path: %s", dataset_path)
    logger.info("Output directory: %s", output_dir)

    if os.path.isdir(dataset_path):
        logger.info("Loading dataset from disk: %s", dataset_path)
        dataset = load_from_disk(dataset_path)
    else:
        logger.info("Loading dataset from URI: %s", dataset_path)
        dataset = load_dataset(dataset_path, split="train")

    reward_fn = load_reward_function()

    training_args = GRPOConfig(
        output_dir=output_dir,
        num_generations=get_env_int("NUM_GENERATIONS", 4),
        max_prompt_length=get_env_int("MAX_PROMPT_LENGTH", 512),
        max_completion_length=get_env_int("MAX_COMPLETION_LENGTH", 256),
        per_device_train_batch_size=get_env_int("PER_DEVICE_TRAIN_BATCH_SIZE", 4),
        gradient_accumulation_steps=get_env_int("GRADIENT_ACCUMULATION_STEPS", 4),
        num_train_epochs=get_env_int("NUM_TRAIN_EPOCHS", 1),
        learning_rate=get_env_float("LEARNING_RATE", 5e-7),
        logging_steps=get_env_int("LOGGING_STEPS", 1),
        save_strategy="epoch",
        report_to="none",
    )

    logger.info("Initializing GRPOTrainer")
    trainer = GRPOTrainer(
        model=model_path,
        reward_funcs=reward_fn,
        args=training_args,
        train_dataset=dataset,
    )

    logger.info("Starting training")
    trainer.train()

    logger.info("Saving final model to %s", output_dir)
    trainer.save_model(output_dir)

    logger.info("GRPO training completed successfully")


if __name__ == "__main__":
    main()
