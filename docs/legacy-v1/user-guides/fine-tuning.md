# How to Fine-Tune LLMs with Kubeflow

Overview of the LLM fine-tuning API in the Training Operator

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This page describes how to use a [train API from the Training Python SDK](https://github.com/kubeflow/training-operator/blob/release-1.9/sdk/python/kubeflow/training/api/training_client.py#L95) that simplifies the ability to fine-tune LLMs with distributed PyTorchJob workers.

If you want to learn more about how the fine-tuning API fits in the Kubeflow ecosystem, head to the [explanation guide](../explanation/fine-tuning.md).

## Prerequisites

You need to install the Training Python SDK [with fine-tuning support](../installation.md#install-the-python-sdk-with-fine-tuning-capabilities) to run this API.

## How to use the Fine-Tuning API?

You need to provide the following parameters to use the `train` API:

- Number of PyTorch workers and resources per workers.
- Trainer parameters.
- Dataset parameters.
- Pre-trained model parameters.

For example, you can use the `train` API to fine-tune the BERT model using the Yelp Review dataset from HuggingFace Hub with the code below:

```python
import transformers
from peft import LoraConfig

from kubeflow.training import TrainingClient
from kubeflow.storage_initializer.hugging_face import (
    HuggingFaceModelParams,
    HuggingFaceTrainerParams,
    HuggingFaceDatasetParams,
)

TrainingClient().train(
    name="fine-tune-bert",
    # BERT model URI and type of Transformer to train it.
    model_provider_parameters=HuggingFaceModelParams(
        model_uri="hf://google-bert/bert-base-cased",
        transformer_type=transformers.AutoModelForSequenceClassification,
    ),
    # Use 3000 samples from Yelp dataset.
    dataset_provider_parameters=HuggingFaceDatasetParams(
        repo_id="yelp_review_full",
        split="train[:3000]",
    ),
    # Specify HuggingFace Trainer parameters. In this example, we will skip evaluation and model checkpoints.
    trainer_parameters=HuggingFaceTrainerParams(
        training_parameters=transformers.TrainingArguments(
            output_dir="test_trainer",
            save_strategy="no",
            eval_strategy="no",
            do_eval=False,
            disable_tqdm=True,
            log_level="info",
        ),
        # Set LoRA config to reduce number of trainable model parameters.
        lora_config=LoraConfig(
            r=8,
            lora_alpha=8,
            lora_dropout=0.1,
            bias="none",
        ),
    ),
    num_workers=4,  # nnodes parameter for torchrun command.
    num_procs_per_worker=2,  # nproc-per-node parameter for torchrun command.
    resources_per_worker={
        "gpu": 2,
        "cpu": 5,
        "memory": "10G",
    },
)
```

After you execute `train`, the Training Operator will orchestrate the appropriate PyTorchJob resources to fine-tune the LLM.

## Dataset and Model Parameter Classes

### HuggingFaceModelParams

#### Description

The `HuggingFaceModelParams` dataclass holds configuration parameters for initializing Hugging Face models with validation checks.

| Attribute | Type | Description |
| --- | --- | --- |
| `model_uri` | `str` | URI or path to the Hugging Face model (must not be empty). |
| `transformer_type` | `TRANSFORMER_TYPES` | Specifies the model type for various NLP/ML tasks. |
| `access_token` | `Optional[str]` (default: `None`) | Token for accessing private models on Hugging Face. |
| `num_labels` | `Optional[int]` (default: `None`) | Number of output labels (used for classification tasks). |

##### Supported Transformer Types (TRANSFORMER_TYPES)

| Model Type | Task |
| --- | --- |
| `AutoModelForSequenceClassification` | Text classification |
| `AutoModelForTokenClassification` | Named entity recognition |
| `AutoModelForQuestionAnswering` | Question answering |
| `AutoModelForCausalLM` | Text generation (causal) |
| `AutoModelForMaskedLM` | Masked language modeling |
| `AutoModelForImageClassification` | Image classification |

#### Example Usage

```python
from transformers import AutoModelForSequenceClassification
from kubeflow.storage_initializer.hugging_face import HuggingFaceModelParams

params = HuggingFaceModelParams(
    model_uri="bert-base-uncased",
    transformer_type=AutoModelForSequenceClassification,
    access_token="huggingface_access_token",
    num_labels=2  # For binary classification
)
```

### HuggingFaceDatasetParams

#### Description

The `HuggingFaceDatasetParams` class holds configuration parameters for loading datasets from Hugging Face with validation checks.

| Attribute | Type | Description |
| --- | --- | --- |
| `repo_id` | `str` | Identifier of the dataset repository on Hugging Face (must not be empty). |
| `access_token` | `Optional[str]` (default: `None`) | Token for accessing private datasets on Hugging Face. |
| `split` | `Optional[str]` (default: `None`) | Dataset split to load (e.g., `"train"`, `"test"`). |

#### Example Usage

```python
from kubeflow.storage_initializer.hugging_face import HuggingFaceDatasetParams

dataset_params = HuggingFaceDatasetParams(
    repo_id="imdb",            # Public dataset repository ID on Hugging Face
    split="train",             # Dataset split to load
    access_token=None          # Not needed for public datasets
)
```

### HuggingFaceTrainerParams

#### Description

The `HuggingFaceTrainerParams` class is used to define parameters for the training process in the Hugging Face framework. It includes the training arguments and LoRA configuration to optimize model training.

| Parameter | Type | Description |
| --- | --- | --- |
| `training_parameters` | `transformers.TrainingArguments` | Contains the training arguments like learning rate, epochs, batch size, etc. |
| `lora_config` | `LoraConfig` | LoRA configuration to reduce the number of trainable parameters in the model. |

#### Example Usage

```python
from transformers import TrainingArguments
from peft import LoraConfig
from kubeflow.storage_initializer.hugging_face import HuggingFaceTrainerParams

trainer_params = HuggingFaceTrainerParams(
    training_parameters=TrainingArguments(
        output_dir="results",
        learning_rate=2e-5,
        num_train_epochs=3,
        per_device_train_batch_size=8,
    ),
    lora_config=LoraConfig(
        r=8,
        lora_alpha=16,
        lora_dropout=0.1,
        bias="none",
    ),
)
```

### S3DatasetParams

#### Description

The `S3DatasetParams` class is used for loading datasets from S3-compatible object storage. It includes validation checks to ensure proper configuration.

| Parameter | Type | Description |
| --- | --- | --- |
| `endpoint_url` | `str` | URL of the S3-compatible storage service. |
| `bucket_name` | `str` | Name of the S3 bucket containing the dataset. |
| `file_key` | `str` | Key (path) to the dataset file within the bucket. |
| `region_name` | `str`, optional | The AWS region of the S3 bucket (optional). |
| `access_key` | `str`, optional | The access key for authentication with S3 (optional). |
| `secret_key` | `str`, optional | The secret key for authentication with S3 (optional). |

#### Implementation Details

The `S3DatasetParams` class includes validation checks to ensure required parameters are provided and the endpoint URL is valid. The actual dataset download is handled by the `S3` class which uses boto3 to interact with the S3-compatible storage.

#### Example Usage

```python
from kubeflow.storage_initializer.s3 import S3DatasetParams

s3_params = S3DatasetParams(
    endpoint_url="https://s3.amazonaws.com",
    bucket_name="my-dataset-bucket",
    file_key="datasets/train.csv",
    region_name="us-west-2",
    access_key="YOUR_ACCESS_KEY",
    secret_key="YOUR_SECRET_KEY"
)
```

## Using custom images with Fine-Tuning API

Platform engineers can customize the storage initializer and trainer images by setting the `STORAGE_INITIALIZER_IMAGE` and `TRAINER_TRANSFORMER_IMAGE` environment variables before executing the `train` command.

For example: In your python code, set the env vars before executing `train`:

```python
...
os.environ['STORAGE_INITIALIZER_IMAGE'] = 'docker.io/<username>/<custom-storage-initiailizer_image>'
os.environ['TRAINER_TRANSFORMER_IMAGE'] = 'docker.io/<username>/<custom-trainer_transformer_image>'

TrainingClient().train(...)
```

## Next Steps

Run the example to [fine-tune the TinyLlama LLM](https://github.com/kubeflow/training-operator/blob/release-1.9/examples/pytorch/language-modeling/train_api_hf_dataset.ipynb)

Check this example to compare the `create_job` and the `train` Python API for [fine-tuning BERT LLM](https://github.com/kubeflow/training-operator/blob/release-1.9/examples/pytorch/text-classification/Fine-Tune-BERT-LLM.ipynb).

Understand [the architecture behind train API](../reference/fine-tuning.md).
