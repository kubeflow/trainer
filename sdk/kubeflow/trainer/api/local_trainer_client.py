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
import os
from typing import List, Optional, Dict
import yaml

from kubeflow.trainer import models
from kubeflow.trainer.api.trainer_client_abc import TrainerClientABC
from kubeflow.trainer.constants import constants
from kubeflow.trainer.types import types
from kubeflow.trainer.utils import utils


class LocalTrainerClient(TrainerClientABC):
    def __init__(
        self,
        local_runtimes_path: str = constants.LOCAL_RUNTIMES_PATH,
    ):
        self.local_runtimes_path = local_runtimes_path

    def list_runtimes(self) -> List[types.Runtime]:
        runtimes = []
        for filename in os.listdir(self.local_runtimes_path):
            with open(os.path.join(self.local_runtimes_path, filename), "r") as f:
                content_str = f.read()
                content_dict = yaml.safe_load(content_str)
                runtime_cr = models.TrainerV1alpha1ClusterTrainingRuntime.from_dict(content_dict)
                runtimes.append(utils.get_runtime_from_crd(runtime_cr))
        return runtimes

    def get_runtime(self, name: str) -> types.Runtime | None:
        for r in self.list_runtimes():
            if r.name == name:
                return r
        return None

    def train(
            self,
            runtime: types.Runtime = types.DEFAULT_RUNTIME,
            initializer: Optional[types.Initializer] = None,
            trainer: Optional[types.CustomTrainer] = None,
    ) -> str:
        raise NotImplementedError()

    def list_jobs(
            self, runtime: Optional[types.Runtime] = None
    ) -> List[types.TrainJob]:
        raise NotImplementedError()

    def get_job(self, name: str) -> types.TrainJob:
        raise NotImplementedError()

    def get_job_logs(
            self,
            name: str,
            follow: Optional[bool] = False,
            step: str = constants.NODE,
            node_rank: int = 0,
    ) -> Dict[str, str]:
        raise NotImplementedError()

    def delete_job(self, name: str):
        raise NotImplementedError()
