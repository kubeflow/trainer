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
from typing import Optional


# Representation for the Training Runtime.
@dataclass
class Runtime:
    name: str
    phase: str


# Representation for the TrainJob.
# TODO (andreyvelich): Discuss what fields users want to get.
@dataclass
class TrainJob:
    name: str
    runtime_ref: str
    creation_timestamp: str
    status: Optional[str] = None


# Representation for the Pod.
@dataclass
class Pod:
    name: str
    type: str
    status: Optional[str] = None


@dataclass
# Configuration for the HuggingFace dataset provider.
class HuggingFaceDatasetConfig:
    storage_uri: str
    access_token: Optional[str] = None


@dataclass
# Configuration for the HuggingFace model provider.
class HuggingFaceModelInputConfig:
    storage_uri: str
    access_token: Optional[str] = None
