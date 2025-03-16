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
from typing import Dict, List, Optional

import kubeflow.trainer.models as models
from kubeflow.trainer.constants import constants
from kubeflow.trainer.types import types
from kubeflow.trainer.utils import utils
from kubernetes import client, config, watch

logger = logging.getLogger(__name__)


class TrainerClient:
    def __init__(
        self,
        config_file: Optional[str] = None,
        context: Optional[str] = None,
        client_configuration: Optional[client.Configuration] = None,
        namespace: str = utils.get_default_target_namespace(),
    ):
        """TrainerClient constructor. Configure logging in your application
            as follows to see detailed information from the TrainerClient APIs:
            .. code-block:: python
                import logging
                logging.basicConfig()
                log = logging.getLogger("kubeflow.trainer.api.trainer_client")
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

            response = thread.get(constants.DEFAULT_TIMEOUT)
            for item in response["items"]:

                runtime = models.TrainerV1alpha1ClusterTrainingRuntime.from_dict(item)

                if not (
                    runtime
                    and runtime.metadata
                    and runtime.spec
                    and runtime.spec.ml_policy
                ):
                    raise Exception(f"Runtime object is invalid: {runtime}")

                ml_policy = runtime.spec.ml_policy
                metadata = runtime.metadata

                # TODO (andreyvelich): Currently, the labels must be presented.
                if metadata.name and metadata.labels and ml_policy.num_nodes:
                    if not (
                        runtime.spec.template.spec
                        and runtime.spec.template.spec.replicated_jobs
                    ):
                        raise Exception(
                            f"Runtime Job template is invalid: {runtime.spec.template.spec}"
                        )
                    # Get the Trainer container resources.
                    resources = None
                    for job in runtime.spec.template.spec.replicated_jobs:
                        if job.name == constants.JOB_TRAINER_NODE:
                            if not (
                                job.template.spec and job.template.spec.template.spec
                            ):
                                raise Exception(
                                    f"JobSet template is invalid: {runtime.spec.template.spec}"
                                )
                            for container in job.template.spec.template.spec.containers:
                                if container.name == constants.CONTAINER_TRAINER:
                                    resources = container.resources

                    # Get the accelerator for the Trainer nodes.
                    # TODO (andreyvelich): Currently, we get the accelerator type from
                    # the runtime labels.
                    _, accelerator_count = utils.get_container_devices(resources)

                    # NumProcPerNode from Torch or MPI overrides accelerator count.
                    if (
                        ml_policy.torch
                        and ml_policy.torch.num_proc_per_node
                        and isinstance(
                            ml_policy.torch.num_proc_per_node.actual_instance, int
                        )
                    ):
                        accelerator_count = (
                            ml_policy.torch.num_proc_per_node.actual_instance
                        )
                    elif ml_policy.mpi and ml_policy.mpi.num_proc_per_node:
                        accelerator_count = ml_policy.mpi.num_proc_per_node

                    if isinstance(accelerator_count, (float, int)):
                        accelerator_count = accelerator_count * ml_policy.num_nodes

                    result.append(
                        types.Runtime(
                            name=metadata.name,
                            phase=(
                                metadata.labels[constants.PHASE_KEY]
                                if constants.PHASE_KEY in metadata.labels
                                else constants.UNKNOWN
                            ),
                            accelerator=(
                                metadata.labels[constants.ACCELERATOR_KEY]
                                if constants.ACCELERATOR_KEY in metadata.labels
                                else constants.UNKNOWN
                            ),
                            accelerator_count=str(accelerator_count),
                        )
                    )

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
        initializer: Optional[types.Initializer] = None,
        trainer: Optional[types.CustomTrainer] = None,
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
        trainer_crd = models.TrainerV1alpha1Trainer()

        # Add number of nodes to the Trainer.
        if trainer and trainer.num_nodes:
            trainer_crd.num_nodes = trainer.num_nodes

        # Add resources per node to the Trainer.
        if trainer and trainer.resources_per_node:
            trainer_crd.resources_per_node = utils.get_resources_per_node(
                trainer.resources_per_node
            )

        # Add command and args to the Trainer if training function is set.
        if trainer and trainer.func:
            trainer_crd.command = constants.DEFAULT_COMMAND
            # TODO: Support train function parameters.
            trainer_crd.args = utils.get_args_using_train_func(
                trainer.func,
                trainer.func_args,
                trainer.packages_to_install,
                trainer.pip_index_url,
            )

        train_job = models.TrainerV1alpha1TrainJob(
            apiVersion=constants.API_VERSION,
            kind=constants.TRAINJOB_KIND,
            metadata=models.IoK8sApimachineryPkgApisMetaV1ObjectMeta(
                name=train_job_name
            ),
            spec=models.TrainerV1alpha1TrainJobSpec(
                runtimeRef=models.TrainerV1alpha1RuntimeRef(name=runtime_ref),
                trainer=(
                    trainer_crd
                    if trainer_crd != models.TrainerV1alpha1Trainer()
                    else None
                ),
                initializer=(
                    models.TrainerV1alpha1Initializer(
                        dataset=utils.get_dataset_initializer(initializer.dataset),
                        model=utils.get_model_initializer(initializer.model),
                    )
                    if isinstance(initializer, types.Initializer)
                    else None
                ),
            ),
        )

        # Create the TrainJob.
        try:
            self.custom_api.create_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                self.namespace,
                constants.TRAINJOB_PLURAL,
                train_job.to_dict(),
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

    def list_jobs(self, runtime_ref: Optional[str] = None) -> List[types.TrainJob]:
        """List of all TrainJobs.

        Returns:
            List[TrainerV1alpha1TrainJob]: List of created TrainJobs.
                It returns an empty list if TrainJobs don't exist.

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

            trainjob_list = models.TrainerV1alpha1TrainJobList.from_dict(response)
            if not trainjob_list:
                return result

            for trainjob in trainjob_list.items:
                # If runtime ref is set, we check the TrainJob's runtime.
                if (
                    runtime_ref is not None
                    and trainjob.spec
                    and trainjob.spec.runtime_ref
                    and trainjob.spec.runtime_ref.name != runtime_ref
                ):
                    continue

                result.append(self.__get_trainjob_from_crd(trainjob))

        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to list {constants.TRAINJOB_KIND}s in namespace: {self.namespace}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to list {constants.TRAINJOB_KIND}s in namespace: {self.namespace}"
            )

        return result

    def get_job(self, name: str) -> types.TrainJob:
        """Get the TrainJob information"""

        try:
            thread = self.custom_api.get_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                self.namespace,
                constants.TRAINJOB_PLURAL,
                name,
                async_req=True,
            )

            trainjob = models.TrainerV1alpha1TrainJob.from_dict(
                thread.get(constants.DEFAULT_TIMEOUT)  # type: ignore
            )

        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to get {constants.TRAINJOB_KIND}: {self.namespace}/{name}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to get {constants.TRAINJOB_KIND}: {self.namespace}/{name}"
            )

        return self.__get_trainjob_from_crd(trainjob)  # type: ignore

    def get_job_logs(
        self,
        name: str,
        follow: bool = False,
        component: str = constants.JOB_TRAINER_NODE,
        node_index: int = 0,
    ) -> Dict[str, str]:
        """Get the logs from TrainJob
        TODO (andreyvelich): Should we change node_index to node_rank ?
        """

        pod_name = None
        # Get Initializer or Trainer Pod name.
        for c in self.get_job(name).components:
            if c.status != constants.POD_PENDING:
                if c.name == component and component == constants.DATASET_INITIALIZER:
                    pod_name = c.pod_name
                elif c.name == component + "-" + str(node_index):
                    pod_name = c.pod_name

        if pod_name is None:
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
                    name=pod_name,
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
                            print(f"[{component}]: {logline}")
                            # Add logs to the results dict.
                            if component not in logs_dict:
                                logs_dict[component] = logline + "\n"
                            else:
                                logs_dict[component] += logline + "\n"
                        except queue.Empty:
                            break
                if all(finished):
                    return logs_dict

        try:
            if component == constants.DATASET_INITIALIZER:
                logs_dict[constants.DATASET_INITIALIZER] = (
                    self.core_api.read_namespaced_pod_log(
                        name=pod_name,
                        namespace=self.namespace,
                        container=constants.DATASET_INITIALIZER,
                    )
                )
                logs_dict[constants.MODEL_INITIALIZER] = (
                    self.core_api.read_namespaced_pod_log(
                        name=pod_name,
                        namespace=self.namespace,
                        container=constants.MODEL_INITIALIZER,
                    )
                )
            else:
                logs_dict[component + "-" + str(node_index)] = (
                    self.core_api.read_namespaced_pod_log(
                        name=pod_name,
                        namespace=self.namespace,
                        container=constants.CONTAINER_TRAINER,
                    )
                )
        except Exception:
            raise RuntimeError(
                f"Failed to read logs for the pod {self.namespace}/{pod_name}"
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

    def __get_trainjob_from_crd(
        self,
        trainjob_crd: models.TrainerV1alpha1TrainJob,
    ) -> types.TrainJob:

        if not (
            trainjob_crd.metadata
            and trainjob_crd.metadata.name
            and trainjob_crd.metadata.namespace
            and trainjob_crd.spec
            and trainjob_crd.metadata.creation_timestamp
        ):
            raise Exception(f"TrainJob CRD is invalid: {trainjob_crd}")

        name = trainjob_crd.metadata.name
        namespace = trainjob_crd.metadata.namespace

        # Construct the TrainJob from the CRD.
        train_job = types.TrainJob(
            name=name,
            runtime_ref=trainjob_crd.spec.runtime_ref.name,
            creation_timestamp=trainjob_crd.metadata.creation_timestamp,
            components=[],
        )

        # Add the TrainJob status.
        # TODO (andreyvelich): Discuss how we should show TrainJob status to SDK users.
        if trainjob_crd.status and trainjob_crd.status.conditions:
            for c in trainjob_crd.status.conditions:
                if c.type == "Created" and c.status == "True":
                    status = "Created"
                elif c.type == "Complete" and c.status == "True":
                    status = "Succeeded"
                elif c.type == "Failed" and c.status == "True":
                    status = "Failed"
            train_job.status = status

        # Select Pods created by appropriate JobSet, and Initializer, Launcher, or Trainer Job.
        # TODO (andreyvelich): Refactor it to check this label:
        # `trainer.kubeflow.org/trainjob-ancestor-step`.
        label_selector = "{}={},{} in ({}, {}, {}, {})".format(
            constants.JOBSET_NAME_KEY,
            name,
            constants.REPLICATED_JOB_KEY,
            constants.DATASET_INITIALIZER,
            constants.MODEL_INITIALIZER,
            constants.JOB_LAUNCHER,
            constants.JOB_TRAINER_NODE,
        )

        # Add the TrainJob components, e.g. trainer nodes and initializer.
        try:
            response = self.core_api.list_namespaced_pod(
                namespace,
                label_selector=label_selector,
                async_req=True,
            ).get(constants.DEFAULT_TIMEOUT)

            # Convert Pod to the correct format.
            pod_list = models.IoK8sApiCoreV1PodList.from_dict(response.to_dict())
            if not pod_list:
                return train_job

            for pod in pod_list.items:
                if not (pod.metadata and pod.metadata.name and pod.spec):
                    raise Exception(f"TrainJob Pod is invalid: {pod}")
                # The TrainJob Pods might have these containers:
                # dataset-initializer, model-initializer, launcher, and trainer
                for c in pod.spec.containers:
                    device, device_count = utils.get_container_devices(c.resources)
                    if c.name == constants.CONTAINER_TRAINER:
                        # For Trainer numProcPerNode overrides container resources.
                        if c.env:
                            for env in c.env:
                                if (
                                    env.name == constants.TORCH_ENV_NUM_PROC_PER_NODE
                                    and env.value
                                    and env.value.isdigit()
                                ):
                                    device_count = env.value
                        # For Trainer node Job, component name is equal to <Job_Name>-<Index>
                        if not pod.metadata.labels:
                            raise Exception(
                                f"TrainJob Pod labels are invalid: {pod.metadata.labels}"
                            )
                        component_name = "{}-{}".format(
                            constants.JOB_TRAINER_NODE,
                            pod.metadata.labels[constants.JOB_INDEX_KEY],
                        )
                    elif (
                        c.name == constants.DATASET_INITIALIZER
                        or c.name == constants.MODEL_INITIALIZER
                        or c.name == constants.CONTAINER_LAUNCHER
                    ):
                        # For dataset, model initializers, or launcher ReplicatedJob,
                        # the component is equal to the container name.
                        component_name = c.name

                component = types.Component(
                    name=component_name,
                    status=pod.status.phase if pod.status else None,
                    device=device,
                    device_count=str(device_count),
                    pod_name=pod.metadata.name,
                )

                train_job.components.append(component)
        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to list {constants.TRAINJOB_KIND}'s components: {namespace}/{name}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to list {constants.TRAINJOB_KIND}'s components: {namespace}/{name}"
            )

        return train_job
