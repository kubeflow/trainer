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
    pip_index_url: Optional[str] = constants.DEFAULT_PIP_INDEX_URL
    num_nodes: Optional[int] = None
    resources_per_node: Optional[Dict] = None


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
