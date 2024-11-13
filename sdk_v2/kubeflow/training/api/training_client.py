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

import logging
import multiprocessing
import queue
import random
import string
import uuid
from typing import Callable, Dict, List, Optional

from kubeflow.training import models
from kubeflow.training.api_client import ApiClient
from kubeflow.training.constants import constants
from kubeflow.training.types import types
from kubeflow.training.utils import utils
from kubernetes import client, config, watch

logger = logging.getLogger(__name__)


class TrainingClient:
    def __init__(
        self,
        config_file: Optional[str] = None,
        context: Optional[str] = None,
        client_configuration: Optional[client.Configuration] = None,
        namespace: str = utils.get_default_target_namespace(),
    ):
        """TrainingClient constructor. Configure logging in your application
            as follows to see detailed information from the TrainingClient APIs:
            .. code-block:: python
                import logging
                logging.basicConfig()
                log = logging.getLogger("kubeflow.training.api.training_client")
                log.setLevel(logging.DEBUG)

        Args:
            config_file: Path to the kube-config file. Defaults to ~/.kube/config.
            context: Set the active context. Defaults to current_context from the kube-config.
            client_configuration: Client configuration for cluster authentication.
                You have to provide valid configuration with Bearer token or
                with username and password. You can find an example here:
                https://github.com/kubernetes-client/python/blob/67f9c7a97081b4526470cad53576bc3b71fa6fcc/examples/remote_cluster.py#L31
            namespace: Target Kubernetes namespace. If SDK runs outside of Kubernetes cluster it
                takes the namespace from the kube-config context. If SDK runs inside
                the Kubernetes cluster it takes namespace from the
                `/var/run/secrets/kubernetes.io/serviceaccount/namespace` file. By default it
                uses the `default` namespace.
        """

        # If client configuration is not set, use kube-config to access Kubernetes APIs.
        if client_configuration is None:
            # Load kube-config or in-cluster config.
            if config_file or not utils.is_running_in_k8s():
                config.load_kube_config(config_file=config_file, context=context)
            else:
                config.load_incluster_config()

        k8s_client = client.ApiClient(client_configuration)
        self.custom_api = client.CustomObjectsApi(k8s_client)
        self.core_api = client.CoreV1Api(k8s_client)
        self.api_client = ApiClient()

        self.namespace = namespace

    # TODO (andreyvelich): Currently, only Cluster Training Runtime is supported.
    def list_runtimes(self) -> List[types.Runtime]:
        """List of the available runtimes.

        Returns:
            List[Runtime]: List of available training runtimes. It returns an empty list if
                runtimes don't exist.

        Raises:
            TimeoutError: Timeout to list Runtimes.
            RuntimeError: Failed to list Runtimes.
        """

        result = []
        try:
            thread = self.custom_api.list_cluster_custom_object(
                constants.GROUP,
                constants.VERSION,
                constants.CLUSTER_TRAINING_RUNTIME_PLURAL,
                async_req=True,
            )
            # TODO (andreyvelich): We should de-serialize runtime into object.
            # For that, we need to import the JobSet models.
            response = thread.get(constants.DEFAULT_TIMEOUT)
            for item in response["items"]:
                # TODO (andreyvelich): Currently, the training phase label must be presented.
                if "labels" in item["metadata"]:
                    # Get the Trainer container resources.
                    resources = None
                    for job in item["spec"]["template"]["spec"]["replicatedJobs"]:
                        if job["name"] == constants.JOB_TRAINER_NODE:
                            pod_spec = job["template"]["spec"]["template"]["spec"]
                            for container in pod_spec["containers"]:
                                if container["name"] == constants.CONTAINER_TRAINER:
                                    if "resources" in container:
                                        resources = client.V1ResourceRequirements(
                                            **container["resources"]
                                        )

                    # TODO (andreyvelich): Currently, only Torch is supported for NumProcPerNode.
                    num_procs = None
                    if "torch" in item["spec"]["mlPolicy"]:
                        num_procs = item["spec"]["mlPolicy"]["torch"]["numProcPerNode"]

                    # Get the devices count.
                    device_count = utils.get_device_count(
                        item["spec"]["mlPolicy"]["numNodes"],
                        num_procs,
                        resources,
                    )
                    runtime = types.Runtime(
                        name=item["metadata"]["name"],  # type: ignore
                        phase=item["metadata"]["labels"][constants.PHASE_KEY],  # type: ignore
                        device=item["metadata"]["labels"][constants.DEVICE_KEY],  # type: ignore
                        device_count=device_count,
                    )

                    result.append(runtime)
        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to list {constants.CLUSTER_TRAINING_RUNTIME_KIND}s "
                f"in namespace: {self.namespace}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to list {constants.CLUSTER_TRAINING_RUNTIME_KIND}s "
                f"in namespace: {self.namespace}"
            )

        return result

    def train(
        self,
        runtime_ref: str,
        train_func: Optional[Callable] = None,
        num_nodes: Optional[int] = None,
        resources_per_node: Optional[dict] = None,
        packages_to_install: Optional[List[str]] = None,
        pip_index_url: str = constants.DEFAULT_PIP_INDEX_URL,
        # TODO (andreyvelich): Add num_nodes, func, resources to the Trainer or TrainerConfig ?
        trainer_config: Optional[types.TrainerConfig] = None,
        dataset_config: Optional[types.HuggingFaceDatasetConfig] = None,
        model_config: Optional[types.HuggingFaceModelInputConfig] = None,
    ) -> str:
        """Create the TrainJob. TODO (andreyvelich): Add description

        Returns:
            str: The unique name of the TrainJob that has been generated.

        Raises:
            ValueError: Input arguments are invalid.
            TimeoutError: Timeout to create TrainJobs.
            RuntimeError: Failed to create TrainJobs.
        """

        # Generate unique name for the TrainJob.
        # TODO (andreyvelich): Discuss this TrainJob name generation.
        train_job_name = random.choice(string.ascii_lowercase) + uuid.uuid4().hex[:11]

        # Build the Trainer.
        trainer = models.KubeflowOrgV2alpha1Trainer()

        # Add number of nodes to the Trainer.
        if num_nodes is not None:
            trainer.num_nodes = num_nodes

        # Add resources per node to the Trainer.
        if resources_per_node is not None:
            trainer.resources_per_node = utils.get_resources_per_node(
                resources_per_node
            )

        # Add command and args to the Trainer if training function is set.
        if train_func is not None:
            trainer.command = constants.DEFAULT_COMMAND
            # TODO: Support train function parameters.
            trainer.args = utils.get_args_using_train_func(
                train_func,
                None,
                packages_to_install,
                pip_index_url,
            )

        # Add the Lora config to the Trainer envs.
        if trainer_config and trainer_config.lora_config:
            trainer.env = utils.get_lora_config(trainer_config.lora_config)

        train_job = models.KubeflowOrgV2alpha1TrainJob(
            api_version=constants.API_VERSION,
            kind=constants.TRAINJOB_KIND,
            metadata=client.V1ObjectMeta(name=train_job_name),
            spec=models.KubeflowOrgV2alpha1TrainJobSpec(
                runtime_ref=models.KubeflowOrgV2alpha1RuntimeRef(name=runtime_ref),
                trainer=(
                    trainer if trainer != models.KubeflowOrgV2alpha1Trainer() else None
                ),
                dataset_config=utils.get_dataset_config(dataset_config),
                model_config=utils.get_model_config(model_config),
            ),
        )

        # Create the TrainJob.
        try:
            self.custom_api.create_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                self.namespace,
                constants.TRAINJOB_PLURAL,
                train_job,
            )
        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to create {constants.TRAINJOB_KIND}: {self.namespace}/{train_job_name}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to create {constants.TRAINJOB_KIND}: {self.namespace}/{train_job_name}"
            )

        logger.debug(
            f"{constants.TRAINJOB_KIND} {self.namespace}/{train_job_name} has been created"
        )

        return train_job_name

    def list_jobs(self) -> List[types.TrainJob]:
        """List of all TrainJobs.

        Returns:
            List[KubeflowOrgV2alpha1TrainJob]: List of created TrainJobs. It returns an empty list
                if TrainJobs don't exist.

        Raises:
            TimeoutError: Timeout to list TrainJobs.
            RuntimeError: Failed to list TrainJobs.
        """

        result = []
        try:
            thread = self.custom_api.list_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                self.namespace,
                constants.TRAINJOB_PLURAL,
                async_req=True,
            )
            response = thread.get(constants.DEFAULT_TIMEOUT)

            for item in response["items"]:
                item = self.api_client.deserialize(
                    utils.FakeResponse(item),
                    models.KubeflowOrgV2alpha1TrainJob,
                )

                train_job = types.TrainJob(
                    name=item.metadata.name,  # type: ignore
                    runtime_ref=item.spec.runtime_ref.name,  # type: ignore
                    creation_timestamp=item.metadata.creation_timestamp,  # type: ignore
                )

                # TODO (andreyvelich): This should be changed.
                if item.status:  # type: ignore
                    train_job.status = utils.get_trainjob_status(
                        item.status.conditions  # type: ignore
                    )

                result.append(train_job)

        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to list {constants.TRAINJOB_KIND}s in namespace: {self.namespace}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to list {constants.TRAINJOB_KIND}s in namespace: {self.namespace}"
            )

        return result

    # TODO (andreyvelich): Discuss whether we need this API.
    # Potentially, we can move this data to the TrainJob type.
    def get_job_pods(self, name: str) -> List[types.Pod]:
        """Get pod names for the TrainJob Job."""

        result = []
        try:
            thread = self.core_api.list_namespaced_pod(
                self.namespace,
                label_selector=f"{constants.JOBSET_NAME_KEY}={name}",
                async_req=True,
            )
            response = thread.get(constants.DEFAULT_TIMEOUT)

            for item in response.items:
                result.append(
                    types.Pod(
                        name=item.metadata.name,
                        component=utils.get_pod_type(item.metadata.labels),
                        status=item.status.phase if item.status else None,
                    )
                )

        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to list {constants.TRAINJOB_KIND}'s pods: {self.namespace}/{name}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to list {constants.TRAINJOB_KIND}'s pods: {self.namespace}/{name}"
            )

        return result

    def get_job_logs(
        self,
        name: str,
        follow: bool = False,
        component: str = constants.JOB_TRAINER_NODE,
        node_index: int = 0,
    ) -> Dict[str, str]:
        """Get the logs from TrainJob
        TODO (andreyvelich): Should we change node_index to node_rank ?
        TODO (andreyvelich): For the initializer, we can add the unit argument.
        """

        pod = None
        # Get Initializer or Trainer Pod.
        for p in self.get_job_pods(name):
            if p.status != constants.POD_PENDING:
                if p.component == component and component == constants.JOB_INITIALIZER:
                    pod = p
                elif p.component == component + "-" + str(node_index):
                    pod = p

        if pod is None:
            return {}

        # Dict where key is the Pod type and value is the Pod logs.
        logs_dict = {}

        # TODO (andreyvelich): Potentially, refactor this.
        # Support logging of multiple Pods.
        # TODO (andreyvelich): Currently, follow is supported only for Trainer.
        if follow and component == constants.JOB_TRAINER_NODE:
            log_streams = []
            log_streams.append(
                watch.Watch().stream(
                    self.core_api.read_namespaced_pod_log,
                    name=pod.name,
                    namespace=self.namespace,
                    container=constants.CONTAINER_TRAINER,
                )
            )
            finished = [False for _ in log_streams]

            # Create thread and queue per stream, for non-blocking iteration.
            log_queue_pool = utils.get_log_queue_pool(log_streams)

            # Iterate over every watching pods' log queue
            while True:
                for index, log_queue in enumerate(log_queue_pool):
                    if all(finished):
                        break
                    if finished[index]:
                        continue
                    # grouping the every 50 log lines of the same pod.
                    for _ in range(50):
                        try:
                            logline = log_queue.get(timeout=1)
                            if logline is None:
                                finished[index] = True
                                break
                            # Print logs to the StdOut
                            print(f"[{pod.component}]: {logline}")
                            # Add logs to the results dict.
                            if pod.component not in logs_dict:
                                logs_dict[pod.component] = logline + "\n"
                            else:
                                logs_dict[pod.component] += logline + "\n"
                        except queue.Empty:
                            break
                if all(finished):
                    return logs_dict

        try:
            if component == constants.JOB_INITIALIZER:
                logs_dict[constants.CONTAINER_DATASET_INITIALIZER] = (
                    self.core_api.read_namespaced_pod_log(
                        name=pod.name,
                        namespace=self.namespace,
                        container=constants.CONTAINER_DATASET_INITIALIZER,
                    )
                )
                logs_dict[constants.CONTAINER_MODEL_INITIALIZER] = (
                    self.core_api.read_namespaced_pod_log(
                        name=pod.name,
                        namespace=self.namespace,
                        container=constants.CONTAINER_MODEL_INITIALIZER,
                    )
                )
            else:
                logs_dict[component + "-" + str(node_index)] = (
                    self.core_api.read_namespaced_pod_log(
                        name=pod.name,
                        namespace=self.namespace,
                        container=constants.CONTAINER_TRAINER,
                    )
                )
        except Exception:
            raise RuntimeError(
                f"Failed to read logs for the pod {self.namespace}/{pod.name}"
            )

        return logs_dict

    def delete_job(self, name: str):
        """Delete the TrainJob.

        Args:
            name: Name of the TrainJob.

        Raises:
            TimeoutError: Timeout to delete TrainJob.
            RuntimeError: Failed to delete TrainJob.
        """

        try:
            self.custom_api.delete_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                self.namespace,
                constants.TRAINJOB_PLURAL,
                name=name,
            )
        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to delete {constants.TRAINJOB_KIND}: {self.namespace}/{name}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to delete {constants.TRAINJOB_KIND}: {self.namespace}/{name}"
            )

        logger.debug(
            f"{constants.TRAINJOB_KIND} {self.namespace}/{name} has been deleted"
        )
