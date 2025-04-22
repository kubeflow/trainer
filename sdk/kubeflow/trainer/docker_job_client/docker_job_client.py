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

from typing import List, Dict, Optional

import docker

from kubeflow.trainer.types import types
from kubeflow.trainer.constants import constants
from kubeflow.trainer.utils import utils


class DockerJobClient:
    def __init__(self, docker_client: Optional[docker.DockerClient] = None):
        if docker_client is None:
            self.docker_client = docker.from_env()
        else:
            self.docker_client = docker_client

    def create_job(
        self,
        image: str,
        entrypoint: List[str],
        command: List[str],
        num_nodes: int,
        framework: types.Framework
    ) -> str:
        if framework != types.Framework.TORCH:
            raise RuntimeError(f"Framework '{framework}' is not currently supported.")

        train_job_name = f"{constants.LOCAL_TRAIN_JOB_NAME_PREFIX}{utils.generate_train_job_name()}"

        docker_network = self.docker_client.networks.create(
            name=train_job_name,
            driver="bridge",
            labels={
                constants.DOCKER_TRAIN_JOB_NAME_LABEL: train_job_name,
            },
        )

        for i in range(num_nodes):
            c = self.docker_client.containers.run(
                name=f"{train_job_name}-{i}",
                network=docker_network.id,
                image=image,
                entrypoint=entrypoint,
                command=command,
                labels={
                    constants.DOCKER_TRAIN_JOB_NAME_LABEL: train_job_name,
                    constants.LOCAL_NODE_RANK_LABEL: str(i),
                },
                environment=self.__get_container_environment(
                    framework=framework,
                    head_node_address=f"{train_job_name}-0",
                    num_nodes=num_nodes,
                    node_rank=i,
                ),
                detach=True,
            )

        return train_job_name

    def get_job(self, job_name: str):
        raise NotImplementedError()

    def get_job_logs(
        self,
        job_name: str,
        follow: bool = False,
        step: str = constants.NODE,
        node_rank: int = 0,
    ) -> Dict[str, str]:
        """Gets container logs for the training job

        Args:
            job_name (str): The name of the training job
            follow (bool): If true, follows the job logs and prints them to standard out (default False)
            step (int): The training job step to target (default "node")
            node_rank (int): The node rank to retrieve logs from (default 0)

        Returns:
            Dict[str, str]: The logs of the training job, where the key is the
            step and node rank, and the value is the logs for that node.
        """
        # TODO (eoinfennessy): use "step" in query.
        containers = self.docker_client.containers.list(
            all=True,
            filters={
                "label": [
                    f"{constants.DOCKER_TRAIN_JOB_NAME_LABEL}={job_name}",
                    f"{constants.LOCAL_NODE_RANK_LABEL}={node_rank}",
                ]
            },
        )
        if len(containers) == 0:
            raise RuntimeError(f"Could not find job '{job_name}'")

        logs: Dict[str, str] = {}
        if follow:
            for l in containers[0].logs(stream=True):
                decoded = l.decode("utf-8")
                print(decoded)
                logs[f"{step}-{node_rank}"] = (
                    logs.get(f"{step}-{node_rank}", "")
                    + decoded
                    + "\n"
                )
        else:
            logs[f"{step}-{node_rank}"] = containers[0].logs().decode()
        return logs

    def list_jobs(self) -> List[str]:
        """Lists the names of all Docker training jobs.

        Returns:
            List[str]: A list of Docker training job names.
        """

        # Because a network is created for each job, we use network names to list all jobs.
        networks = self.docker_client.networks.list(
            filters={"label": constants.DOCKER_TRAIN_JOB_NAME_LABEL},
        )

        job_names = []
        for n in networks:
            job_names.append(n.name)
        return job_names

    def delete_job(self, job_name: str) -> None:
        """Deletes all resources associated with a Docker training job.
        Args:
            job_name (str): The name of the Docker training job.
        """
        containers = self.docker_client.containers.list(
            all=True,
            filters={"label": f"{constants.DOCKER_TRAIN_JOB_NAME_LABEL}={job_name}"}
        )
        for c in containers:
            c.remove(force=True)
            print(f"Removed container: {c.name}")

        network = self.docker_client.networks.get(job_name)
        network.remove()
        print(f"Removed network: {network.name}")

    @staticmethod
    def __get_container_environment(
        framework: types.Framework,
        head_node_address: str,
        num_nodes: int,
        node_rank: int,
    ) -> Dict[str, str]:
        if framework != types.Framework.TORCH:
            raise RuntimeError(f"Framework '{framework}' is not currently supported.")

        return {
            "PET_NNODES": str(num_nodes),
            "PET_NPROC_PER_NODE": "1",
            "PET_NODE_RANK": str(node_rank),
            "PET_MASTER_ADDR": head_node_address,
            "PET_MASTER_PORT": str(constants.TORCH_HEAD_NODE_PORT),
        }
