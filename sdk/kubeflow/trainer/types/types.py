# Copyright 2024 The Kubeflow Authors.
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


from dataclasses import dataclass
from datetime import datetime
from enum import Enum
from typing import Callable, Dict, List, Optional, Union

from kubeflow.trainer.constants import constants


# Configuration for the Custom Trainer.
@dataclass
class CustomTrainer:
    """Custom Trainer configuration. Configure the self-contained function
        that encapsulates the entire model training process.

    Args:
        func (`Callable`): The function that encapsulates the entire model training process.
        func_args (`Optional[Dict]`): The arguments to pass to the function.
        packages_to_install (`Optional[List[str]]`):
            A list of Python packages to install before running the function.
        pip_index_url (`Optional[str]`): The PyPI URL from which to install Python packages.
        num_nodes (`Optional[int]`): The number of nodes to use for training.
        resources_per_node (`Optional[Dict]`): The computing resources to allocate per node.
    """

    func: Callable
    func_args: Optional[Dict] = None
    packages_to_install: Optional[List[str]] = None
    pip_index_url: str = constants.DEFAULT_PIP_INDEX_URL
    num_nodes: Optional[int] = None
    resources_per_node: Optional[Dict] = None


# TODO(Electronic-Waste): Add more loss functions.
# Loss function for the TorchTune LLM Trainer.
class Loss(Enum):
    """Loss function for the TorchTune LLM Trainer."""

    CEWithChunkedOutputLoss = "torchtune.modules.loss.CEWithChunkedOutputLoss"


# Data type for the TorchTune LLM Trainer.
class DataType(Enum):
    """Data type for the TorchTune LLM Trainer."""

    BF16 = "bf16"
    FP32 = "fp32"


# Data file type for the TorchTune LLM Trainer.
class DataFileType(Enum):
    """Data file type for the TorchTune LLM Trainer."""

    JSON = "json"
    CSV = "csv"
    PARQUET = "parquet"
    ARROW = "arrow"
    TEXT = "text"
    XML = "xml"


# Configuration for the TorchTune Instruct dataset.
@dataclass
class InstructDataset:
    """
    Configuration for the custom dataset with user instruction prompts and model responses.
    REF: https://pytorch.org/torchtune/main/generated/torchtune.datasets.instruct_dataset.html

    Args:
        source (`Optional[DataFileType]`): Data file type.
        data_dir (`Optional[str]`):
            The path to the data directory. For HF dataset, the value should be `<repo>/<dir_path>`.
            The behavior is equal to passing all data files under `data_dir` to `data_files`.
            all the files in a directory. But if `data_files` is specified, `data_dir` is ignored.
        data_files (`Optional[List[str]]`):
            The path to the data files. For HF dataset, the value should be `<repo>/<file_path>`.
        split (`Optional[str]`):
            The split of the dataset to use.  You can use this argument to load a subset of
            a given split, e.g. split="train[:10%]". Default is `train`.
        train_on_input (`Optional[bool]`):
            Whether the model is trained on the user prompt or not. Default is False.
        new_system_prompt (`Optional[str]`):
            The new system prompt to use. If specified, prepend a system message.
            This can serve as instructions to guide the model response. Default is None.
        column_map (`Optional[Dict[str, str]]`):
            A mapping to change the expected "input" and "output" column names to the actual
            column names in the dataset. Keys should be "input" and "output" and values should
            be the actual column names. Default is None, keeping the default "input" and
            "output" column names.
    """

    source: Optional[DataFileType] = None
    data_dir: Optional[str] = None
    data_files: Optional[List[str]] = None
    split: Optional[str] = None
    train_on_input: Optional[bool] = None
    new_system_prompt: Optional[str] = None
    column_map: Optional[Dict[str, str]] = None


# Configuration for the TorchTune LLM Trainer.
@dataclass
class TorchTuneConfig:
    """TorchTune LLM Trainer configuration. Configure the parameters in
        the TorchTune LLM Trainer that already includes the fine-tuning logic.

    Args:
        dtype (`Optional[Dtype]`):
            The underlying data type used to represent the model and optimizer parameters.
            Currently, we only support `bf16` and `fp32`.
        batch_size (`Optional[int]`):
            The number of samples processed before updating model weights.
        epochs (`Optional[int]`):
            The number of samples processed before updating model weights.
        loss (`Optional[Loss]`): The loss algorithm we use to fine-tune the LLM,
            e.g. `torchtune.modules.loss.CEWithChunkedOutputLoss`.
        num_nodes (`Optional[int]`): The number of nodes to use for training.
        dataset_preprocess_config (`Optional[InstructDataset]`):
            Configuration for the dataset preprocessing.
        resources_per_node (`Optional[Dict]`): The computing resources to allocate per node.
    """

    dtype: Optional[DataType] = None
    batch_size: Optional[int] = None
    epochs: Optional[int] = None
    loss: Optional[Loss] = None
    num_nodes: Optional[int] = None
    dataset_preprocess_config: Optional[InstructDataset] = None
    resources_per_node: Optional[Dict] = None


# Configuration for the Builtin Trainer.
@dataclass
class BuiltinTrainer:
    """
    Builtin Trainer configuration. Configure the builtin trainer that already includes
        the fine-tuning logic, requiring only parameter adjustments.

    Args:
        config (`TorchTuneConfig`): The configuration for the builtin trainer.
    """

    config: TorchTuneConfig


class TrainerType(Enum):
    CUSTOM_TRAINER = CustomTrainer.__name__
    BUILTIN_TRAINER = BuiltinTrainer.__name__


class Framework(Enum):
    TORCH = "torch"
    DEEPSPEED = "deepspeed"
    MLX = "mlx"
    TORCHTUNE = "torchtune"


# Representation for the Trainer of the runtime.
@dataclass
class Trainer:
    trainer_type: TrainerType
    framework: Framework
    entrypoint: Optional[List[str]] = None
    accelerator: str = constants.UNKNOWN
    accelerator_count: Union[str, float, int] = constants.UNKNOWN


# Representation for the Training Runtime.
@dataclass
class Runtime:
    name: str
    trainer: Trainer
    pretrained_model: Optional[str] = None


# Representation for the TrainJob steps.
@dataclass
class Step:
    name: str
    status: Optional[str]
    pod_name: str
    device: str = constants.UNKNOWN
    device_count: Union[str, int] = constants.UNKNOWN


# Representation for the TrainJob.
# TODO (andreyvelich): Discuss what fields users want to get.
@dataclass
class TrainJob:
    name: str
    creation_timestamp: datetime
    runtime: Runtime
    steps: List[Step]
    status: Optional[str] = constants.UNKNOWN


# Configuration for the HuggingFace dataset initializer.
# TODO (andreyvelich): Discuss how to keep these configurations is sync with pkg.initializers.types
@dataclass
class HuggingFaceDatasetInitializer:
    storage_uri: str
    access_token: Optional[str] = None


# Configuration for the HuggingFace model initializer.
@dataclass
class HuggingFaceModelInitializer:
    storage_uri: str
    access_token: Optional[str] = None


@dataclass
class Initializer:
    """Initializer defines configurations for dataset and pre-trained model initialization

    Args:
        dataset (`Optional[HuggingFaceDatasetInitializer]`): The configuration for one of the
            supported dataset initializers.
        model (`Optional[HuggingFaceModelInitializer]`): The configuration for one of the
            supported model initializers.
    """

    dataset: Optional[HuggingFaceDatasetInitializer] = None
    model: Optional[HuggingFaceModelInitializer] = None


# The dict where key is the container image and value its representation.
# Each Trainer representation defines trainer parameters (e.g. type, framework, entrypoint).
# TODO (andreyvelich): We should allow user to overrides the default image names.
ALL_TRAINERS: Dict[str, Trainer] = {
    # Custom Trainers.
    "pytorch/pytorch": Trainer(
        trainer_type=TrainerType.CUSTOM_TRAINER,
        framework=Framework.TORCH,
        entrypoint=[constants.TORCH_ENTRYPOINT],
    ),
    "ghcr.io/kubeflow/trainer/mlx-runtime": Trainer(
        trainer_type=TrainerType.CUSTOM_TRAINER,
        framework=Framework.MLX,
        entrypoint=[
            constants.MPI_ENTRYPOINT,
            "--hostfile",
            constants.MPI_HOSTFILE,
            "bash",
            "-c",
        ],
    ),
    "ghcr.io/kubeflow/trainer/deepspeed-runtime": Trainer(
        trainer_type=TrainerType.CUSTOM_TRAINER,
        framework=Framework.DEEPSPEED,
        entrypoint=[
            constants.MPI_ENTRYPOINT,
            "--hostfile",
            constants.MPI_HOSTFILE,
            "bash",
            "-c",
        ],
    ),
    # Builtin Trainers.
    "ghcr.io/kubeflow/trainer/torchtune-trainer": Trainer(
        trainer_type=TrainerType.BUILTIN_TRAINER,
        framework=Framework.TORCHTUNE,
        entrypoint=constants.DEFAULT_TORCHTUNE_COMMAND,
    ),
}

# The default trainer configuration when runtime detection fails
DEFAULT_TRAINER = Trainer(
    trainer_type=TrainerType.CUSTOM_TRAINER,
    framework=Framework.TORCH,
    entrypoint=[constants.TORCH_ENTRYPOINT],
)

# The default runtime configuration for the train() API
DEFAULT_RUNTIME = Runtime(
    name="torch-distributed",
    trainer=DEFAULT_TRAINER,
)
