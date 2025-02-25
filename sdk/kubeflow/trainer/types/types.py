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


from dataclasses import dataclass, field
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
    status: str
    device: str
    device_count: str
    pod_name: str


# Representation for the TrainJob.
# TODO (andreyvelich): Discuss what fields users want to get.
@dataclass
class TrainJob:
    name: str
    runtime_ref: str
    creation_timestamp: str
    components: List[Component]
    status: Optional[str] = "Unknown"


# Configuration for the Lora to configure parameter efficient fine-tuning.
@dataclass
class LoraConfig:
    r: Optional[int] = field(
        default=None, metadata={"help": "Lora attention dimension"}
    )
    lora_alpha: Optional[int] = field(default=None, metadata={"help": "Lora alpha"})
    lora_dropout: Optional[float] = field(
        default=None, metadata={"help": "Lora dropout"}
    )


@dataclass
class FineTuningConfig:
    # TODO (andreyvelich): Add more configs once we support them, e.g. QLoRA.
    peft_config: Optional[LoraConfig] = None


# Configuration for the Trainer.
# TODO (andreyvelich): Discuss what values should be on the Trainer.
@dataclass
class Trainer:
    """Trainer configuration.
    TODO: Add the description
    """

    func: Optional[Callable] = None
    func_args: Optional[Dict] = None
    packages_to_install: Optional[List[str]] = None
    pip_index_url: str = constants.DEFAULT_PIP_INDEX_URL
    fine_tuning_config: Optional[FineTuningConfig] = None
    num_nodes: Optional[int] = None
    resources_per_node: Optional[dict] = None


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
