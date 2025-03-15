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
from typing import Callable, Dict, List, Optional

from kubeflow.trainer.constants import constants


# Representation for the Training Runtime.
@dataclass
class Runtime:
    name: str
    phase: str
    accelerator: str
    accelerator_count: str


# Representation for the TrainJob component.
@dataclass
class Component:
    name: str
    status: Optional[str]
    device: str
    device_count: str
    pod_name: str


# Representation for the TrainJob.
# TODO (andreyvelich): Discuss what fields users want to get.
@dataclass
class TrainJob:
    name: str
    runtime_ref: str
    creation_timestamp: datetime
    components: List[Component]
    status: Optional[str] = "Unknown"


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
    pip_index_url: Optional[str] = constants.DEFAULT_PIP_INDEX_URL
    num_nodes: Optional[int] = None
    resources_per_node: Optional[Dict] = None


# Configuration for the TorchTune LLM Trainer.
@dataclass
class TorchTuneConfig:
    """TorchTune LLM Trainer configuration. Configure the parameters in
        the TorchTune LLM Trainer that already includes the fine-tuning logic.

    Args:
        dtype (`Optional[str]`):
            The underlying data type used to represent the model and optimizer parameters.
            Currently, we only support `bf16` and `fp32`.
        batch_size (`Optional[int]`):
            The number of samples processed before updating model weights.
        epochs (`Optional[int]`):
            The number of samples processed before updating model weights.
        loss (`Optional[str]`): The loss algorithm we use to fine-tune the LLM,
            e.g. `torchtune.modules.loss.CEWithChunkedOutputLoss`.
        num_nodes (`Optional[int]`): The number of nodes to use for training.
        resources_per_node (`Optional[Dict]`): The computing resources to allocate per node.
    """

    dtype: Optional[str] = None
    batch_size: Optional[int] = None
    epochs: Optional[int] = None
    loss: Optional[str] = None
    num_nodes: Optional[int] = None
    resources_per_node: Optional[Dict] = None


# Configuration for the HuggingFace dataset provider.
@dataclass
class HuggingFaceDatasetConfig:
    storage_uri: str
    access_token: Optional[str] = None


@dataclass
# Configuration for the HuggingFace model provider.
class HuggingFaceModelInputConfig:
    storage_uri: str
    access_token: Optional[str] = None
