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


# Configuration for the custom trainer.
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


# Configuration for the Builtin Trainer.
@dataclass
class BuiltinTrainer:
    pass


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


# Representation for a container used in a local job.
@dataclass
class Container:
    name: str
    status: str

    def to_step(self) -> Step:
        return Step(name=self.name, status=self.status, pod_name=self.name)


# Representation for a local container job.
@dataclass
class ContainerJob:
    name: str
    creation_timestamp: datetime
    runtime_name: str
    containers: List[Container]
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
            "-x",
            "LD_LIBRARY_PATH=/usr/local/lib/",
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
