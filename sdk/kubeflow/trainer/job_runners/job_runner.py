# Copyright 2025 The Kubeflow Authors.
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

from abc import ABC, abstractmethod
from typing import List, Dict

from kubeflow.trainer.constants import constants
from kubeflow.trainer.types import types


class JobRunner(ABC):
    @abstractmethod
    def create_job(
            self,
            image: str,
            entrypoint: List[str],
            command: List[str],
            num_nodes: int,
            framework: types.Framework,
    ) -> str:
        pass

    @abstractmethod
    def get_job(self, job_name: str):
        pass

    @abstractmethod
    def get_job_logs(
            self,
            job_name: str,
            follow: bool = False,
            step: str = constants.NODE,
            node_rank: int = 0,
    ) -> Dict[str, str]:
        pass

    @abstractmethod
    def list_jobs(self) -> List[str]:
        pass

    @abstractmethod
    def delete_job(self, job_name: str) -> None:
        pass
