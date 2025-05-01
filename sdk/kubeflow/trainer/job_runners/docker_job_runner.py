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

from datetime import datetime
from typing import Dict, List, Optional

import docker
from kubeflow.trainer.constants import constants
from kubeflow.trainer.job_runners.job_runner import JobRunner
from kubeflow.trainer.types import types
from kubeflow.trainer.utils import utils


class DockerJobRunner(JobRunner):
    """DockerJobRunner creates and manages training jobs using Docker.

    Args:
        docker_client: If provided, this client is used for Docker API calls.
            If not provided, a new client will be created from the user's environment.
    """

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
        framework: types.Framework,
        runtime_name: str,
    ) -> str:
        """Creates a training job.

        Args:
            image: The name of the container image to use for the job.
            entrypoint: The entrypoint for the container.
            command: The command to run in the container.
            num_nodes: The number of nodes to run the job on.
            framework: The framework being used.
            runtime_name: The name of the runtime being used.

        Returns:
            The name of the created job.

        Raises:
            RuntimeError: If the framework provided is not supported.
        """
        if framework != types.Framework.TORCH:
            raise RuntimeError(f"Framework '{framework}' is not currently supported.")

        train_job_name = (
            f"{constants.LOCAL_TRAIN_JOB_NAME_PREFIX}{utils.generate_train_job_name()}"
        )

        docker_network = self.docker_client.networks.create(
            name=train_job_name,
            driver="bridge",
            labels={
                constants.CONTAINER_TRAIN_JOB_NAME_LABEL: train_job_name,
                constants.CONTAINER_RUNTIME_LABEL: runtime_name,
            },
        )

        for i in range(num_nodes):
            self.docker_client.containers.run(
                name=f"{train_job_name}-{i}",
                network=docker_network.id,
                image=image,
                entrypoint=entrypoint,
                command=command,
                labels={
                    constants.CONTAINER_TRAIN_JOB_NAME_LABEL: train_job_name,
                    constants.LOCAL_NODE_RANK_LABEL: str(i),
                    constants.CONTAINER_RUNTIME_LABEL: runtime_name,
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

    def get_job(self, job_name: str) -> types.ContainerJob:
        """Get a specified container training job by its name.

        Args:
            job_name: The name of the training job to get.

        Returns:
            A container training job.
        """
        network = self.docker_client.networks.get(job_name)

        docker_containers = self.docker_client.containers.list(
            filters={
                "label": [f"{constants.CONTAINER_TRAIN_JOB_NAME_LABEL}={job_name}"]
            },
            all=True,
        )

        containers = []
        for container in docker_containers:
            containers.append(
                types.Container(
                    name=container.name,
                    status=container.status,
                ),
            )

        return types.ContainerJob(
            name=job_name,
            creation_timestamp=datetime.fromisoformat(network.attrs["Created"]),
            runtime_name=network.attrs["Labels"][constants.CONTAINER_RUNTIME_LABEL],
            containers=containers,
            status=self.__get_job_status(containers),
        )

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
            follow (bool): If true, follows job logs and prints them to standard out (default False)
            step (int): The training job step to target (default "node")
            node_rank (int): The node rank to retrieve logs from (default 0)

        Returns:
            Dict[str, str]: The logs of the training job, where the key is the
            step and node rank, and the value is the logs for that node.

        Raises:
            RuntimeError: If the job is not found.
        """
        # TODO (eoinfennessy): use "step" in query.
        containers = self.docker_client.containers.list(
            all=True,
            filters={
                "label": [
                    f"{constants.CONTAINER_TRAIN_JOB_NAME_LABEL}={job_name}",
                    f"{constants.LOCAL_NODE_RANK_LABEL}={node_rank}",
                ]
            },
        )
        if len(containers) == 0:
            raise RuntimeError(f"Could not find job '{job_name}'")

        logs: Dict[str, str] = {}
        if follow:
            for line in containers[0].logs(stream=True):
                decoded = line.decode("utf-8")
                print(decoded)
                logs[f"{step}-{node_rank}"] = (
                    logs.get(f"{step}-{node_rank}", "") + decoded + "\n"
                )
        else:
            logs[f"{step}-{node_rank}"] = containers[0].logs().decode()
        return logs

    def list_jobs(
        self,
        runtime_name: Optional[str] = None,
    ) -> List[types.ContainerJob]:
        """Lists container training jobs.

        Args:
            runtime_name: If provided, only return jobs that use the given runtime name.

        Returns:
            A list of container training jobs.
        """
        jobs = []
        for name in self.__list_job_names(runtime_name):
            jobs.append(self.get_job(name))
        return jobs

    def delete_job(self, job_name: str) -> None:
        """Deletes all resources associated with a Docker training job.
        Args:
            job_name (str): The name of the Docker training job.
        """
        containers = self.docker_client.containers.list(
            all=True,
            filters={"label": f"{constants.CONTAINER_TRAIN_JOB_NAME_LABEL}={job_name}"},
        )
        for c in containers:
            c.remove(force=True)
            print(f"Removed container: {c.name}")

        network = self.docker_client.networks.get(job_name)
        network.remove()
        print(f"Removed network: {network.name}")

    def __list_job_names(
        self,
        runtime_name: Optional[str] = None,
    ) -> List[str]:
        """Lists the names of all Docker training jobs.

        Args:
            runtime_name (Optional[str]): Filter by runtime name (default None)

        Returns:
            List[str]: A list of Docker training job names.
        """

        filters = {"label": [constants.CONTAINER_TRAIN_JOB_NAME_LABEL]}
        if runtime_name is not None:
            filters["label"].append(
                f"{constants.CONTAINER_RUNTIME_LABEL}={runtime_name}"
            )

        # Because a network is created for each job, we use network names to list all jobs.
        networks = self.docker_client.networks.list(filters=filters)

        job_names = []
        for n in networks:
            job_names.append(n.name)
        return job_names

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

    @staticmethod
    def __get_job_status(_: List[types.Container]) -> str:
        # TODO (eoinfennessy): Discuss how to report status
        return constants.UNKNOWN
