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

from importlib import resources
from pathlib import Path
from typing import Dict, List, Optional

import yaml
from kubeflow.trainer import models
from kubeflow.trainer.api.abstract_trainer_client import AbstractTrainerClient
from kubeflow.trainer.constants import constants
from kubeflow.trainer.job_runners import DockerJobRunner, JobRunner
from kubeflow.trainer.types import types
from kubeflow.trainer.utils import utils


class LocalTrainerClient(AbstractTrainerClient):
    def __init__(
        self,
        local_runtimes_path: Optional[Path] = None,
        job_runner: Optional[JobRunner] = None,
    ):
        print(
            "Warning: LocalTrainerClient is an alpha feature for Kubeflow Trainer. "
            "Some features may be unstable or unimplemented."
        )

        if local_runtimes_path is None:
            self.local_runtimes_path = (
                resources.files(constants.PACKAGE_NAME) / constants.LOCAL_RUNTIMES_PATH
            )
        else:
            self.local_runtimes_path = local_runtimes_path

        if job_runner is None:
            self.job_runner = DockerJobRunner()
        else:
            self.job_runner = job_runner

    def list_runtimes(self) -> List[types.Runtime]:
        runtimes = []
        for cr in self.__list_runtime_crs():
            runtimes.append(utils.get_runtime_from_crd(cr))
        return runtimes

    def get_runtime(self, name: str) -> types.Runtime:
        for r in self.list_runtimes():
            if r.name == name:
                return r
        raise RuntimeError(f"No runtime found with name '{name}'")

    def train(
        self,
        runtime: types.Runtime = types.DEFAULT_RUNTIME,
        initializer: Optional[types.Initializer] = None,
        trainer: Optional[types.CustomTrainer] = None,
    ) -> str:
        runtime_cr = self.__get_runtime_cr(runtime.name)
        if runtime_cr is None:
            raise RuntimeError(f"No runtime found with name '{runtime.name}'")

        runtime_container = utils.get_runtime_trainer_container(
            runtime_cr.spec.template.spec.replicated_jobs
        )
        if runtime_container is None:
            raise RuntimeError("No runtime container found")

        image = runtime_container.image
        if image is None:
            raise RuntimeError("No runtime container image specified")

        if trainer and trainer.func:
            entrypoint, command = utils.get_entrypoint_using_train_func(
                runtime,
                trainer.func,
                trainer.func_args,
                trainer.pip_index_url,
                trainer.packages_to_install,
            )
        else:
            entrypoint = runtime_container.command
            command = runtime_container.args

        if trainer and trainer.num_nodes:
            num_nodes = trainer.num_nodes
        else:
            num_nodes = 1

        train_job_name = self.job_runner.create_job(
            image=image,
            entrypoint=entrypoint,
            command=command,
            num_nodes=num_nodes,
            framework=runtime.trainer.framework,
            runtime_name=runtime.name,
        )
        return train_job_name

    def list_jobs(
        self, runtime: Optional[types.Runtime] = None
    ) -> List[types.TrainJob]:
        runtime_name = runtime.name if runtime else None
        container_jobs = self.job_runner.list_jobs(runtime_name)

        train_jobs = []
        for container_job in container_jobs:
            train_jobs.append(self.__container_job_to_train_job(container_job))
        return train_jobs

    def get_job(self, name: str) -> types.TrainJob:
        container_job = self.job_runner.get_job(name)
        return self.__container_job_to_train_job(container_job)

    def get_job_logs(
        self,
        name: str,
        follow: Optional[bool] = False,
        step: str = constants.NODE,
        node_rank: int = 0,
    ) -> Dict[str, str]:
        """Gets logs for the specified training job
        Args:
            name (str): The name of the training job
            follow (bool): If true, follows job logs and prints them to standard out (default False)
            step (int): The training job step to target (default "node")
            node_rank (int): The node rank to retrieve logs from (default 0)

        Returns:
            Dict[str, str]: The logs of the training job, where the key is the
            step and node rank, and the value is the logs for that node.
        """
        return self.job_runner.get_job_logs(
            job_name=name, follow=follow, step=step, node_rank=node_rank
        )

    def delete_job(self, name: str):
        self.job_runner.delete_job(job_name=name)

    def __list_runtime_crs(self) -> List[models.TrainerV1alpha1ClusterTrainingRuntime]:
        runtime_crs = []
        for filename in self.local_runtimes_path.iterdir():
            with open(filename, "r") as f:
                cr_str = f.read()
                cr_dict = yaml.safe_load(cr_str)
                cr = models.TrainerV1alpha1ClusterTrainingRuntime.from_dict(cr_dict)
                if cr is not None:
                    runtime_crs.append(cr)
        return runtime_crs

    def __get_runtime_cr(
        self,
        name: str,
    ) -> Optional[models.TrainerV1alpha1ClusterTrainingRuntime]:
        for cr in self.__list_runtime_crs():
            if cr.metadata.name == name:
                return cr
        return None

    def __container_job_to_train_job(
        self, container_job: types.ContainerJob
    ) -> types.TrainJob:
        return types.TrainJob(
            name=container_job.name,
            creation_timestamp=container_job.creation_timestamp,
            steps=[container.to_step() for container in container_job.containers],
            runtime=self.get_runtime(container_job.runtime_name),
            status=container_job.status,
        )
