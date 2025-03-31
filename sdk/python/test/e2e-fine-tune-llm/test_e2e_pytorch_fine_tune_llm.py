# Copyright 2024 kubeflow.org.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import logging

import transformers
from kubeflow.storage_initializer.hugging_face import (
    HuggingFaceDatasetParams,
    HuggingFaceModelParams,
    HuggingFaceTrainerParams,
)
from kubeflow.training import TrainingClient, constants
from peft import LoraConfig

import test.e2e.utils as utils

logging.basicConfig(format="%(message)s")
logging.getLogger("kubeflow.training.api.training_client").setLevel(logging.DEBUG)

TRAINING_CLIENT = TrainingClient(job_kind=constants.PYTORCHJOB_KIND)


def test_sdk_e2e_create_from_train_api(job_namespace="default"):
    JOB_NAME = "pytorchjob-from-train-api"

    # Use test case from fine-tuning API tutorial.
    # https://www.kubeflow.org/docs/components/training/user-guides/fine-tuning/
    TRAINING_CLIENT.train(
        name=JOB_NAME,
        namespace=job_namespace,
        # BERT model URI and type of Transformer to train it.
        model_provider_parameters=HuggingFaceModelParams(
            model_uri="hf://google-bert/bert-base-cased",
            transformer_type=transformers.AutoModelForSequenceClassification,
            num_labels=5,
        ),
        # In order to save test time, use 8 samples from Yelp dataset.
        dataset_provider_parameters=HuggingFaceDatasetParams(
            repo_id="yelp_review_full",
            split="train[:8]",
        ),
        # Specify HuggingFace Trainer parameters.
        trainer_parameters=HuggingFaceTrainerParams(
            training_parameters=transformers.TrainingArguments(
                output_dir="test_trainer",
                save_strategy="no",
                eval_strategy="no",
                do_eval=False,
                disable_tqdm=True,
                log_level="info",
                num_train_epochs=1,
            ),
            # Set LoRA config to reduce number of trainable parameters.
            lora_config=LoraConfig(
                r=8,
                lora_alpha=8,
                lora_dropout=0.1,
                bias="none",
            ),
        ),
        num_workers=1,
        num_procs_per_worker=1,
        resources_per_worker={
            "gpu": 0,
            "cpu": 2,
            "memory": "10G",
        },
        storage_config={
            "size": "10Gi",
            "access_modes": ["ReadWriteOnce"],
        },
    )

    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"PyTorchJob create from API E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
