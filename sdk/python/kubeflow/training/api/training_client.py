# Copyright 2023 The Kubeflow Authors.
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

import json
import logging
import multiprocessing
import queue
import time
from typing import Any, Callable, Dict, List, Optional, Set, Tuple, Union

from kubeflow.storage_initializer.constants import (
    VOLUME_PATH_DATASET,
    VOLUME_PATH_MODEL,
)
from kubeflow.training import models
from kubeflow.training.api_client import ApiClient
from kubeflow.training.constants import constants
from kubeflow.training.utils import utils
from kubernetes import client, config, watch

logger = logging.getLogger(__name__)

status_logger = utils.StatusLogger(
    header="{:<30.30} {:<20.20} {}".format("NAME", "STATE", "TIME"),
    column_format="{:<30.30} {:<20.20} {}",
)


class TrainingClient(object):
    def __init__(
        self,
        config_file: Optional[str] = None,
        context: Optional[str] = None,
        client_configuration: Optional[client.Configuration] = None,
        namespace: str = utils.get_default_target_namespace(),
        job_kind: str = constants.PYTORCHJOB_KIND,
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
            namespace: Target Kubernetes namespace. By default it takes namespace
                from `/var/run/secrets/kubernetes.io/serviceaccount/namespace` location
                or set as `default`. Namespace can be overridden during method invocations.
            job_kind: Target Training Job kind (e.g. `TFJob`, `PyTorchJob`, `MPIJob`).
                Job kind can be overridden during method invocations.
                The default Job kind is `PyTorchJob`.

        Raises:
            ValueError: Job kind is invalid.
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
        if job_kind not in constants.JOB_PARAMETERS:
            raise ValueError(
                f"Job kind must be one of these: {list(constants.JOB_PARAMETERS.keys())}"
            )
        self.job_kind = job_kind

    def train(
        self,
        name: str,
        namespace: Optional[str] = None,
        labels: Optional[Dict[str, str]] = None,
        annotations: Optional[Dict[str, str]] = None,
        num_workers: int = 1,
        num_procs_per_worker: int = 1,
        resources_per_worker: Union[dict, client.V1ResourceRequirements, None] = None,
        model_provider_parameters=None,
        dataset_provider_parameters=None,
        trainer_parameters=None,
        init_env_vars: Optional[
            Union[Dict[str, str], List[Union[models.V1EnvVar, models.V1EnvVar]]]
        ] = None,
        env_vars: Optional[
            Union[Dict[str, str], List[Union[models.V1EnvVar, models.V1EnvVar]]]
        ] = None,
        storage_config: Dict[str, Optional[Union[str, List[str]]]] = {
            "size": constants.PVC_DEFAULT_SIZE,
            "storage_class": None,
            "access_modes": constants.PVC_DEFAULT_ACCESS_MODES,
        },
    ):
        """High level API to fine-tune LLMs with distributed PyTorchJob. Follow this guide
        for more information about this feature: TODO (andreyvelich): Add link.

        It uses the pre-created Storage Initializer to download pre-trained model and dataset, and
        Trainer to fine-tune LLM. Your cluster should support PVC with ReadOnlyMany access mode
        to distribute data across PyTorchJob workers.

        It uses `torchrun` CLI to fine-tune model in distributed mode with multiple PyTorchJob
        workers. Follow this guide to know more about `torchrun` CLI:
        https://pytorch.org/docs/stable/elastic/run.html

        This feature is in alpha stage and Kubeflow community is looking for your feedback.
        Please use #kubeflow-training Slack channel or Kubeflow Training Operator GitHub
        for your questions or suggestions.

        Args:
            name: Name of the PyTorchJob.
            namespace: Namespace for the PyTorchJob. By default namespace is taken from
                `TrainingClient` object.
            labels: Labels for the PyTorchJob. You can specify a dictionary as a mapping object
                representing the labels.
            annotations: Annotations for the PyTorchJob. You can specify a dictionary as a
                mapping object representing the annotations.
            num_workers: Number of PyTorchJob workers.
            num_procs_per_worker: Number of processes per PyTorchJob worker for `torchrun` CLI. You
                should use this parameter if you want to use more than 1 GPU per PyTorchJob worker.
            resources_per_worker: A parameter that lets you specify how much
                resources each PyTorchJob worker container should have. You can either specify a
                kubernetes.client.V1ResourceRequirements object (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ResourceRequirements.md)
                or a dictionary that includes one or more of the following keys:
                `cpu`, `memory`, or `gpu` (other keys will be ignored). Appropriate
                values for these keys are documented here:
                https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/.
                For example:
                ```
                {
                    "cpu": "1",
                    "memory": "2Gi",
                    "gpu": "1",
                }
                ```
                Please note, `gpu` specifies a resource request with a key of
                `nvidia.com/gpu`, i.e. an NVIDIA GPU. If you need a different type
                of GPU, pass in a V1ResourceRequirement instance instead, since it's
                more flexible. This parameter is optional and defaults to None.
            model_provider_parameters: Parameters for the model provider in the Storage Initializer.
                For example, HuggingFace model name and Transformer type for that model, like:
                AutoModelForSequenceClassification. This argument must be the type of
                `kubeflow.storage_initializer.hugging_face.HuggingFaceModelParams`
            dataset_provider_parameters: Parameters for the dataset provider in the
                Storage Initializer. For example, name of the HuggingFace dataset or
                AWS S3 configuration. This argument must be the type of
                `kubeflow.storage_initializer.hugging_face.HuggingFaceDatasetParams` or
                `kubeflow.storage_initializer.s3.S3DatasetParams`
            trainer_parameters: Parameters for LLM Trainer that will fine-tune pre-trained model
                with the given dataset. For example, LoRA config for parameter-efficient fine-tuning
                and HuggingFace training arguments like optimizer or number of training epochs.
                This argument must be the type of
                `kubeflow.storage_initializer.HuggingFaceTrainerParams`
            init_env_vars: Environment variable(s) to be attached to init container.
                You can specify a dictionary as a mapping object representing the environment
                variables. Otherwise, you can specify a list, in which the element can either
                be a kubernetes.client.models.V1EnvVar (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1EnvVar.md)
                or a kubernetes.client.models.V1EnvFromSource (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1EnvFromSource.md)
            env_vars: Environment variable(s) to be attached to training container.
                You can specify a dictionary as a mapping object representing the environment
                variables. Otherwise, you can specify a list, in which the element can either
                be a kubernetes.client.models.V1EnvVar (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1EnvVar.md)
                or a kubernetes.client.models.V1EnvFromSource (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1EnvFromSource.md)
            storage_config: Configuration for Storage Initializer PVC to download pre-trained model
                and dataset. You can configure PVC size and storage class name in this argument.
        """
        try:
            import peft  # noqa: F401
            import transformers  # noqa: F401
        except ImportError:
            raise ImportError(
                "Train API dependencies not installed. "
                + "Run: pip install -U 'kubeflow-training[huggingface]' "
            )

        # fmt: off

        from kubeflow.storage_initializer.hugging_face import (
            HuggingFaceDatasetParams,
            HuggingFaceModelParams,
        )
        from kubeflow.storage_initializer.s3 import S3DatasetParams

        # fmt: on

        print(
            "Thank you for using `train` API for LLMs fine-tuning. This feature is in alpha stage "
            "Kubeflow community is looking for your feedback. Please share your experience "
            "via #kubeflow-training Slack channel or Kubeflow Training Operator GitHub."
        )

        if (
            not name
            or not model_provider_parameters
            or not dataset_provider_parameters
            or not trainer_parameters
        ):
            raise ValueError("One of the required parameters is None")

        namespace = namespace or self.namespace

        # TODO (andreyvelich): PVC Creation should be part of Training Operator Controller.
        # Ref issue: https://github.com/kubeflow/training-operator/issues/1971
        try:
            self.core_api.create_namespaced_persistent_volume_claim(
                namespace=namespace,
                body=utils.get_pvc_spec(
                    pvc_name=name,
                    namespace=namespace,
                    storage_config=storage_config,
                ),
            )
        except Exception as e:
            pvc_list = self.core_api.list_namespaced_persistent_volume_claim(namespace)
            # Check if the PVC with the specified name exists
            for pvc in pvc_list.items:
                if pvc.metadata.name == name:
                    print(f"PVC '{name}' already exists in namespace " f"{namespace}.")
                    break
            else:
                raise RuntimeError(f"failed to create PVC. Error: {e}")

        if isinstance(model_provider_parameters, HuggingFaceModelParams):
            mp = "hf"
        else:
            raise ValueError(
                f"Invalid model provider parameters {model_provider_parameters}"
            )

        if isinstance(dataset_provider_parameters, S3DatasetParams):
            dp = "s3"
        elif isinstance(dataset_provider_parameters, HuggingFaceDatasetParams):
            dp = "hf"
        else:
            raise ValueError(
                f"Invalid dataset provider parameters {dataset_provider_parameters}"
            )

        # create init container spec
        init_container_spec = utils.get_container_spec(
            name=constants.STORAGE_INITIALIZER,
            base_image=constants.STORAGE_INITIALIZER_IMAGE,
            args=[
                "--model_provider",
                mp,
                "--model_provider_parameters",
                json.dumps(model_provider_parameters.__dict__, cls=utils.SetEncoder),
                "--dataset_provider",
                dp,
                "--dataset_provider_parameters",
                json.dumps(dataset_provider_parameters.__dict__),
            ],
            volume_mounts=[constants.STORAGE_INITIALIZER_VOLUME_MOUNT],
            env_vars=init_env_vars,
        )

        # create app container spec
        container_spec = utils.get_container_spec(
            name=constants.JOB_PARAMETERS[constants.PYTORCHJOB_KIND]["container"],
            base_image=constants.TRAINER_TRANSFORMER_IMAGE,
            args=[
                "--model_uri",
                model_provider_parameters.model_uri,
                "--transformer_type",
                model_provider_parameters.transformer_type.__name__,
                "--num_labels",
                str(model_provider_parameters.num_labels),
                "--model_dir",
                VOLUME_PATH_MODEL,
                "--dataset_dir",
                VOLUME_PATH_DATASET,
                "--lora_config",
                json.dumps(
                    trainer_parameters.lora_config.__dict__, cls=utils.SetEncoder
                ),
                "--training_parameters",
                json.dumps(trainer_parameters.training_parameters.to_dict()),
            ],
            volume_mounts=[constants.STORAGE_INITIALIZER_VOLUME_MOUNT],
            resources=resources_per_worker,
            env_vars=env_vars,
        )

        storage_initializer_volume = models.V1Volume(
            name=constants.STORAGE_INITIALIZER,
            persistent_volume_claim=models.V1PersistentVolumeClaimVolumeSource(
                claim_name=name
            ),
        )

        # create worker pod spec
        worker_pod_template_spec = utils.get_pod_template_spec(
            containers=[container_spec],
            volumes=[storage_initializer_volume],
        )

        # create master pod spec
        master_pod_template_spec = utils.get_pod_template_spec(
            containers=[container_spec],
            init_containers=[init_container_spec],
            volumes=[storage_initializer_volume],
        )

        job = utils.get_pytorchjob_template(
            name=name,
            namespace=namespace,
            labels=labels,
            annotations=annotations,
            master_pod_template_spec=master_pod_template_spec,
            worker_pod_template_spec=worker_pod_template_spec,
            num_workers=num_workers,
            num_procs_per_worker=num_procs_per_worker,
        )

        self.create_job(job, namespace=namespace)

    def create_job(
        self,
        job: Optional[constants.JOB_MODELS_TYPE] = None,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        labels: Optional[Dict[str, str]] = None,
        annotations: Optional[Dict[str, str]] = None,
        job_kind: Optional[str] = None,
        base_image: Optional[str] = None,
        train_func: Optional[Callable] = None,
        parameters: Optional[Dict[str, Any]] = None,
        num_workers: Optional[int] = 1,
        num_procs_per_worker: Optional[Union[int, str]] = None,
        resources_per_worker: Union[dict, models.V1ResourceRequirements, None] = None,
        num_chief_replicas: Optional[int] = None,
        num_ps_replicas: Optional[int] = None,
        packages_to_install: Optional[List[str]] = None,
        pip_index_url: str = constants.DEFAULT_PIP_INDEX_URL,
        env_vars: Optional[
            Union[Dict[str, str], List[Union[models.V1EnvVar, models.V1EnvVar]]]
        ] = None,
        volumes: Optional[List[models.V1Volume]] = None,
        volume_mounts: Optional[List[models.V1VolumeMount]] = None,
    ):
        """Create the Training Job.
        Job can be created using one of the following options:

        - Define custom resource object in `job` parameter (e.g. TFJob or PyTorchJob).
        - Define training function in `train_func` parameter and number of workers.
        - Define Docker image in `base_image` parameter and number of workers.

        Args:
            job: Job object. Object must be one of these types: KubeflowOrgV1TFJob,
                KubeflowOrgV1PyTorchJob, etc.
            name: Name for the Job. It must be set if `job` parameter is omitted.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            labels: Labels for the Job. You can specify a dictionary as a mapping
                object representing the labels.
            annotations: Annotations for the Job. You can specify a dictionary as
                a mapping object representing the annotations.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). It must be set if
                `job` parameter is omitted. By default Job kind is taken from
                `TrainingClient` object.
            base_image: Image that Job uses to train the model on each training replica.
                If `train_func` parameter is set, this image is used to execute the training
                function. The `constants` module contains some base images, the default image
                is `docker.io/pytorch/pytorch:1.12.1-cuda11.3-cudnn8-runtime`
            train_func: Function that Job uses to train the model on each training replica.
                This function must be Callable. Optionally, this function might have one dict
                argument to define input parameters for the function. If `train_func` is
                set, Base Image must support `bash` CLI to execute the training script.
            parameters: Dict of input parameters that training function might receive.
            num_workers: Number of Worker replicas for the Job.
            num_procs_per_worker: Number of processes per PyTorchJob worker for `torchrun` CLI. You
                should use this parameter if you want to use more than 1 GPU per PyTorchJob worker.
                Set to "auto" to automatically use available GPU/CPU PyTorch resources.
            resources_per_worker: A parameter that lets you specify how much
                resources each Worker container should have. You can either specify a
                kubernetes.client.V1ResourceRequirements object (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ResourceRequirements.md)
                or a dictionary that includes one or more of the following keys:
                `cpu`, `memory`, or `gpu` (other keys will be ignored). Appropriate
                values for these keys are documented here:
                https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/.
                For example:
                ```
                {
                    "cpu": "1",
                    "memory": "2Gi",
                    "gpu": "1",
                }
                ```
                Please note, `gpu` specifies a resource request with a key of
                `nvidia.com/gpu`, i.e. an NVIDIA GPU. If you need a different type
                of GPU, pass in a V1ResourceRequirement instance instead, since it's
                more flexible. This parameter is optional and defaults to None.
            num_chief_replicas: Number of Chief replicas for the TFJob. Number
                of Chief replicas can't be more than 1.
            num_ps_replicas: Number of Parameter Server replicas for the TFJob.
            packages_to_install: List of Python packages to install in addition
                to the base image packages if `train_func` parameter is set.
                These packages are installed before executing the objective function.
            pip_index_url: The PyPI url from which to install Python packages.
            env_vars: Environment variable(s) to be attached to training container.
                You can specify a dictionary as a mapping object representing the environment
                variables. Otherwise, you can specify a list, in which the element can either
                be a kubernetes.client.models.V1EnvVar (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1EnvVar.md)
                or a kubernetes.client.models.V1EnvFromSource (documented here:
                https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1EnvFromSource.md)
            volumes: Volume(s) to be attached to the replicas.
            volume_mounts: VolumeMount(s) specifying where to mount the volume(s) into the replicas.

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to create Job.
            RuntimeError: Failed to create Job.
        """

        # When Job is set, only namespace, labels, and annotations args are allowed.
        if job is not None:
            for key, value in locals().items():
                if (
                    key
                    not in [
                        "self",
                        "job",
                        "namespace",
                        "pip_index_url",
                        "num_workers",
                        "labels",
                        "annotations",
                    ]
                    and value is not None
                ):
                    raise ValueError(
                        "If `job` is set only `namespace`, `labels`, and `annotations` "
                        "arguments are allowed. "
                        f"Argument `{key}` must be None."
                    )

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind
        if job is not None:
            job_kind = str(job.kind)

        if job_kind not in constants.JOB_PARAMETERS:
            raise ValueError(
                f"Job kind must be one of these: {constants.JOB_PARAMETERS.keys()}"
            )

        if len(volumes or []) != len(volume_mounts or []):
            raise ValueError(
                "Volumes and VolumeMounts must be the same length: "
                f"{len(volumes or [])} vs. {len(volume_mounts or [])}"
            )

        # If Training function or base image is set, configure Job template.
        if job is None and (train_func is not None or base_image is not None):
            # Job name must be set to configure Job template.
            if name is None:
                raise ValueError(
                    "Job name must be set to configure Job from function or image"
                )

            # Check if at least one Worker is set.
            # TODO (andreyvelich): Remove this check once we have CEL validation.
            # Ref: https://github.com/kubeflow/training-operator/issues/1708
            if num_workers is None or num_workers < 1:
                raise ValueError(f"At least one Worker for {job_kind} must be set")

            # Assign the default base image.
            # TODO (andreyvelich): Add base image for other Job kinds.
            if base_image is None:
                base_image = constants.JOB_PARAMETERS[job_kind]["base_image"]

            # By default we don't set command and args for the training container.
            command, args = None, None

            # If training function is set get the command and args.
            if train_func is not None:
                # Use `torchrun` for distributed PyTorch training, otherwise use `python`
                if job_kind == constants.PYTORCHJOB_KIND and (
                    num_workers > 1 or num_procs_per_worker is not None
                ):
                    entrypoint = constants.ENTRYPOINT_TORCH
                else:
                    entrypoint = constants.ENTRYPOINT_PYTHON

                command, args = utils.get_command_using_train_func(
                    train_func=train_func,
                    entrypoint=entrypoint,
                    train_func_parameters=parameters,
                    packages_to_install=packages_to_install,
                    pip_index_url=pip_index_url,
                )

            # Get Training Container template.
            container_spec = utils.get_container_spec(
                name=constants.JOB_PARAMETERS[job_kind]["container"],
                base_image=base_image,
                command=command,
                args=args,
                resources=resources_per_worker,
                env_vars=env_vars,
                volume_mounts=volume_mounts,
            )

            # Get Pod template spec using the above container.
            pod_template_spec = utils.get_pod_template_spec(
                containers=[container_spec],
                volumes=volumes,
            )

            # Configure template for different Jobs.
            # TODO (andreyvelich): Add support for other kinds (e.g. MPIJob).
            if job_kind == constants.TFJOB_KIND:
                if num_procs_per_worker is not None:
                    raise ValueError(
                        f"num_procs_per_worker can't be set for {constants.TFJOB_KIND}"
                    )
                job = utils.get_tfjob_template(
                    name=name,
                    namespace=namespace,
                    labels=labels,
                    annotations=annotations,
                    pod_template_spec=pod_template_spec,
                    num_workers=num_workers,
                    num_chief_replicas=num_chief_replicas,
                    num_ps_replicas=num_ps_replicas,
                )
            elif job_kind == constants.PYTORCHJOB_KIND:
                if num_chief_replicas is not None or num_ps_replicas is not None:
                    raise ValueError(
                        "num_chief_replicas and num_ps_replicas can't be set for "
                        f"{constants.PYTORCHJOB_KIND}"
                    )
                job = utils.get_pytorchjob_template(
                    name=name,
                    namespace=namespace,
                    labels=labels,
                    annotations=annotations,
                    worker_pod_template_spec=pod_template_spec,
                    num_workers=num_workers,
                    num_procs_per_worker=num_procs_per_worker,
                )
            else:
                raise ValueError(
                    f"Job kind {job_kind} can't be created using function or image. "
                    + "Number of Workers must be set."
                )

        # Verify Job object type.
        if not isinstance(
            job,
            getattr(models, constants.JOB_PARAMETERS[job_kind]["model"]),
        ):
            raise ValueError(
                f"Job must be one of these types: {constants.JOB_MODELS}, but Job is: {type(job)}"
            )

        # Create the Training Job.
        try:
            self.custom_api.create_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                namespace,
                constants.JOB_PARAMETERS[job_kind]["plural"],
                job,
            )
        except multiprocessing.TimeoutError:
            raise TimeoutError(
                f"Timeout to create {job_kind}: {namespace}/{job.metadata.name}"
            )
        except Exception:
            raise RuntimeError(
                f"Failed to create {job_kind}: {namespace}/{job.metadata.name}"
            )

        logger.debug(f"{job_kind} {namespace}/{job.metadata.name} has been created")

    def get_job(
        self,
        name: str,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> constants.JOB_MODELS_TYPE:
        """Get the Training Job.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            object: Job object. For example: KubeflowOrgV1PyTorchJob

        Raises:
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job.
        """

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind

        if job_kind not in constants.JOB_PARAMETERS:
            raise ValueError(
                f"Job kind must be one of these: {constants.JOB_PARAMETERS.keys()}"
            )

        try:
            thread = self.custom_api.get_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                namespace,
                constants.JOB_PARAMETERS[job_kind]["plural"],
                name,
                async_req=True,
            )
            response = utils.FakeResponse(thread.get(timeout))
            job = self.api_client.deserialize(
                response, constants.JOB_PARAMETERS[job_kind]["model"]
            )

        except multiprocessing.TimeoutError:
            raise TimeoutError(f"Timeout to get {job_kind}: {namespace}/{name}")
        except Exception:
            raise RuntimeError(f"Failed to get {job_kind}: {namespace}/{name}")

        return job

    def list_jobs(
        self,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> List[constants.JOB_MODELS_TYPE]:
        """List of all Training Jobs with specific kind in namespace.

        Args:
            namespace: Namespace to list the Jobs. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            list[object]: List of Job objects.
                For example: list of KubeflowOrgV1PyTorchJob objects. It returns empty list
                if Jobs can't be found.

        Raises:
            TimeoutError: Timeout to list Jobs
            RuntimeError: Failed to list Jobs
        """

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind

        if job_kind not in constants.JOB_PARAMETERS:
            raise ValueError(
                f"Job kind must be one of these: {constants.JOB_PARAMETERS.keys()}"
            )

        result = []
        try:
            thread = self.custom_api.list_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                namespace,
                constants.JOB_PARAMETERS[job_kind]["plural"],
                async_req=True,
            )
            response = thread.get(timeout)
            result = [
                self.api_client.deserialize(
                    utils.FakeResponse(item),
                    constants.JOB_PARAMETERS[job_kind]["model"],
                )
                for item in response.get("items")
            ]
        except multiprocessing.TimeoutError:
            raise TimeoutError(f"Timeout to list {job_kind}s in namespace: {namespace}")
        except Exception:
            raise RuntimeError(f"Failed to list {job_kind}s in namespace: {namespace}")

        return result

    def get_job_conditions(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        job: Optional[constants.JOB_MODELS_TYPE] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> List[models.V1JobCondition]:
        """Get the Training Job conditions. Training Job is in the condition when
        `status=True` for the appropriate condition `type`. For example,
        Training Job is Succeeded when `status=True` and `type=Succeeded`.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            job: Job object can be set to get the conditions. Object must be one of
                these types: KubeflowOrgV1TFJob, KubeflowOrgV1PyTorchJob, etc.
                If this parameter is omitted, it gets Job with the given name and kind.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            list[V1JobCondition]: List of Job conditions with
                last transition time, last update time, message, reason, type, and
                status. It returns empty list if Job does not have any
                conditions yet.

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job.
        """

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind

        if job_kind not in constants.JOB_PARAMETERS:
            raise ValueError(
                f"Job kind must be one of these: {constants.JOB_PARAMETERS.keys()}"
            )

        if job is not None and not isinstance(
            job, getattr(models, constants.JOB_PARAMETERS[job_kind]["model"])
        ):
            raise ValueError(f"Job must be one of these types: {constants.JOB_MODELS}")

        # If Job is not set, get the Training Job.
        if job is None:
            # Job name must be set when Job object is not set.
            if name is None:
                raise ValueError(
                    "Job name must be set to configure Job from function or image"
                )

            job = self.get_job(
                name=name,
                namespace=namespace,
                job_kind=job_kind,
                timeout=timeout,
            )
        if job.status and job.status.conditions and len(job.status.conditions) > 0:
            return job.status.conditions
        return []

    def is_job_created(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        job: Optional[constants.JOB_MODELS_TYPE] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> bool:
        """Check if Training Job is Created.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            job: Job object can be set to get the conditions. Object must be one of
                these types: KubeflowOrgV1TFJob, KubeflowOrgV1PyTorchJob, etc.
                If this parameter is omitted, it gets Job with the given name and kind.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            bool: True if Job is Created, else False.

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job.
        """

        return utils.has_condition(
            self.get_job_conditions(name, namespace, job_kind, job, timeout),
            constants.JOB_CONDITION_CREATED,
        )

    def is_job_running(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        job: Optional[constants.JOB_MODELS_TYPE] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> bool:
        """Check if Training Job is Running.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            job: Job object can be set to get the conditions. Object must be one of
                these types: KubeflowOrgV1TFJob, KubeflowOrgV1PyTorchJob, etc.
                If this parameter is omitted, it gets Job with the given name and kind.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            bool: True if Job is Running, else False.

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job.
        """

        return utils.has_condition(
            self.get_job_conditions(name, namespace, job_kind, job, timeout),
            constants.JOB_CONDITION_RUNNING,
        )

    def is_job_restarting(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        job: Optional[constants.JOB_MODELS_TYPE] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> bool:
        """Check if Training Job is Restarting.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            job: Job object can be set to get the conditions. Object must be one of
                these types: KubeflowOrgV1TFJob, KubeflowOrgV1PyTorchJob, etc.
                If this parameter is omitted, it gets Job with the given name and kind.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            bool: True if Job is Restarting, else False.

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job.
        """

        return utils.has_condition(
            self.get_job_conditions(name, namespace, job_kind, job, timeout),
            constants.JOB_CONDITION_RESTARTING,
        )

    def is_job_succeeded(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        job: Optional[constants.JOB_MODELS_TYPE] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> bool:
        """Check if Training Job is Succeeded.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            job: Job object can be set to get the conditions. Object must be one of
                these types: KubeflowOrgV1TFJob, KubeflowOrgV1PyTorchJob, etc.
                If this parameter is omitted, it gets Job with the given name and kind.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            bool: True if Job is Succeeded, else False.

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job.
        """

        return utils.has_condition(
            self.get_job_conditions(name, namespace, job_kind, job, timeout),
            constants.JOB_CONDITION_SUCCEEDED,
        )

    def is_job_failed(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        job: Optional[constants.JOB_MODELS_TYPE] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> bool:
        """Check if Training Job is Failed.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            job: Job object can be set to get the conditions. Object must be one of
                these types: KubeflowOrgV1TFJob, KubeflowOrgV1PyTorchJob, etc.
                If this parameter is omitted, it gets Job with the given name and kind.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            bool: True if Job is Failed, else False.

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job.
        """

        return utils.has_condition(
            self.get_job_conditions(name, namespace, job_kind, job, timeout),
            constants.JOB_CONDITION_FAILED,
        )

    def wait_for_job_conditions(
        self,
        name: str,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        expected_conditions: Set = {constants.JOB_CONDITION_SUCCEEDED},
        wait_timeout: int = 600,
        polling_interval: int = 15,
        callback: Optional[Callable] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> constants.JOB_MODELS_TYPE:
        """Wait until Training Job reaches any of the specified conditions.
        By default it waits for the Succeeded condition.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            expected_conditions: Set of expected conditions. It must be subset of this:
                `{"Created", "Running", "Restarting", "Succeeded", "Failed"}`
            wait_timeout: How many seconds to wait until Job reaches one of
                the expected conditions.
            polling_interval: The polling interval in seconds to get Job status.
            callback: Callback function that is invoked after Job
                status is polled. This function takes a single argument which
                is current Job object.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            object: Job object. For example: KubeflowOrgV1PyTorchJob

        Raises:
            ValueError: Invalid input parameters.
            TimeoutError: Timeout to get Job.
            RuntimeError: Failed to get Job, or Job reaches Failed condition and
                Failed is not in `expected_conditions` set.
        """

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind

        if not expected_conditions.issubset(constants.JOB_CONDITIONS):
            raise ValueError(
                f"Expected conditions: {expected_conditions} must be subset of \
                    {constants.JOB_CONDITIONS}"
            )
        for _ in range(round(wait_timeout / polling_interval)):
            # We should get Job only once per cycle and check the statuses.
            job = self.get_job(
                name=name,
                namespace=namespace,
                job_kind=job_kind,
                timeout=timeout,
            )

            # Get Job conditions.
            conditions = self.get_job_conditions(
                job=job, timeout=timeout, job_kind=job_kind
            )
            if len(conditions) > 0:
                status_logger(
                    name,
                    conditions[-1].type,
                    conditions[-1].last_transition_time,
                )

            # Execute callback function is it is set.
            if callback:
                callback(job)

            # Raise an exception if Job is Failed and Failed is not the expected condition.
            if (
                constants.JOB_CONDITION_FAILED not in expected_conditions
                and utils.has_condition(conditions, constants.JOB_CONDITION_FAILED)
            ):
                raise RuntimeError(
                    f"{job_kind} {namespace}/{name} is Failed. "
                    f"{job_kind} conditions: {job.status.conditions}"
                )

            # Return Job when it reaches expected condition.
            for expected_condition in expected_conditions:
                if utils.has_condition(conditions, expected_condition):
                    return job

            time.sleep(polling_interval)

        raise TimeoutError(
            f"Timeout waiting for {job_kind}: {namespace}/{name} to reach expected conditions: \
                {expected_conditions}"
        )

    def get_job_pods(
        self,
        name: str,
        namespace: Optional[str] = None,
        is_master: bool = False,
        replica_type: Optional[str] = None,
        replica_index: Optional[int] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> List[models.V1Pod]:
        """Get pods for the Training Job.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            is_master: Whether to get pods only with the label
                `training.kubeflow.org/job-role: master`.
            replica_type: Type of the Job replica.
                For TFJob one of `Chief`, `PS`, or `worker`.

                For PyTorchJob one of `master` or `worker`.

                For XGBoostJob one of `master` or `worker`.

                For MPIJob one of `launcher` or `worker`.

                For PaddleJob one of `master` or `worker`.

                For JAXJob `worker`.

            replica_index: Index for the Job replica.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            list[V1Pod]: List of the Job pods.

        Raises:
            ValueError: Job replica type is invalid.
            TimeoutError: Timeout to get Job pods.
            RuntimeError: Failed to get Job pods.
        """

        namespace = namespace or self.namespace

        if (
            replica_type is not None
            and replica_type not in constants.TFJOB_REPLICA_TYPES
            and replica_type not in constants.PYTORCHJOB_REPLICA_TYPES
            and replica_type not in constants.XGBOOSTJOB_REPLICA_TYPES
            and replica_type not in constants.MPIJOB_REPLICA_TYPES
            and replica_type not in constants.PADDLEJOB_REPLICA_TYPES
            and replica_type not in constants.JAXJOB_REPLICA_TYPES
        ):
            raise ValueError(
                f"TFJob replica type must be one of {constants.TFJOB_REPLICA_TYPES}\n"
                f"PyTorchJob replica type must be one of {constants.PYTORCHJOB_REPLICA_TYPES}\n"
                f"XGBoostJob replica type must be one of {constants.XGBOOSTJOB_REPLICA_TYPES}\n"
                f"MPIJob replica type must be one of {constants.MPIJOB_REPLICA_TYPES}\n"
                f"PaddleJob replica type must be one of {constants.PADDLEJOB_REPLICA_TYPES}"
                f"JAXJob replica type must be one of {constants.PADDLEJOB_REPLICA_TYPES}"
            )

        label_selector = f"{constants.JOB_NAME_LABEL}={name}"

        # Add Job role label if that is required.
        if is_master:
            label_selector += f",{constants.JOB_ROLE_LABEL}={constants.JOB_ROLE_MASTER}"

        # Add Replica type label if that is required.
        if replica_type:
            label_selector += (
                f",{constants.REPLICA_TYPE_LABEL}={str.lower(replica_type)}"
            )

        # Add Replica index label if that is required.
        if replica_index is not None:
            label_selector += f",{constants.REPLICA_INDEX_LABEL}={replica_index}"

        # Return list of Training Job pods.
        try:
            thread = self.core_api.list_namespaced_pod(
                namespace,
                label_selector=label_selector,
                async_req=True,
            )
            return thread.get(timeout).items
        except multiprocessing.TimeoutError:
            raise TimeoutError(f"Timeout to list pods for Job: {namespace}/{name}")
        except Exception:
            raise RuntimeError(f"Failed to list pods for Job: {namespace}/{name}")

    def get_job_pod_names(
        self,
        name: str,
        namespace: Optional[str] = None,
        is_master: bool = False,
        replica_type: Optional[str] = None,
        replica_index: Optional[int] = None,
        timeout: int = constants.DEFAULT_TIMEOUT,
    ) -> List[str]:
        """Get pod names for the Training Job.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            is_master: Whether to get pods only with the label
                `training.kubeflow.org/job-role: master`.
            replica_type: Type of the Job replica.
                For TFJob one of `Chief`, `PS`, or `worker`.

                For PyTorchJob one of `master` or `worker`.

                For XGBoostJob one of `master` or `worker`.

                For MPIJob one of `launcher` or `worker`.

                For PaddleJob one of `master` or `worker`.

                For JAXJob `worker`.

            replica_index: Index for the Job replica.
            timeout: Kubernetes API server timeout in seconds to execute the request.

        Returns:
            list[str]: List of the Job pod names.

        Raises:
            ValueError: Job replica type is invalid.
            TimeoutError: Timeout to get Job pods.
            RuntimeError: Failed to get Job pods.
        """

        namespace = namespace or self.namespace

        pods = self.get_job_pods(
            name=name,
            namespace=namespace,
            is_master=is_master,
            replica_type=replica_type,
            replica_index=replica_index,
            timeout=timeout,
        )
        pod_names = []
        for pod in pods:
            pod_names.append(pod.metadata.name)
        return pod_names

    def get_job_logs(
        self,
        name: str,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        is_master: bool = True,
        replica_type: Optional[str] = None,
        replica_index: Optional[int] = None,
        follow: bool = False,
        timeout: int = constants.DEFAULT_TIMEOUT,
        verbose: bool = False,
    ) -> Tuple[Dict[str, str], Dict[str, List[str]]]:
        """Get the logs for every Training Job pod. By default it returns logs from
        the `master` pod. Logs are returned in this format: { "pod-name": "Log data" }.

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            is_master: Whether to get logs for the pod with the label
                `training.kubeflow.org/job-role: master`.
            replica_type: Optional, type of the Job replica.
                For TFJob one of `chief`, `ps`, or `worker`.

                For PyTorchJob one of `master` or `worker`.

                For XGBoostJob one of `master` or `worker`.

                For MPIJob one of `launcher` or `worker`.

                For PaddleJob one of `master` or `worker`.

                For JAXJob `worker`.
            replica_index: Optional, index for the Job replica.
            container: Pod container to get the logs.
            follow: Whether to follow the log stream of the pod and print logs to StdOut.
            timeout: Optional, Kubernetes API server timeout in seconds
                to execute the request.
            verbose: Whether to get Kubernetes events for Job and corresponding pods.
                If you need to get events from all PyTorchJob's Pods, set `isMaster = False`.

        Returns:
            Dict[str, str]: A dictionary in which the keys are pod names and the
            values are the corresponding logs.
            Dict[str, str]: A dictionary in which the keys are object kind and name, and the
            values are list of the corresponding Kubernetes events with their timestamps. This
            value is returned only if `verbose = True`. For example:
            ```json
            {
              "PyTorchJob train-mnist": [
                "2024-01-05 22:58:20 Created pod: train-mnist-worker-0"
              ],
              "Pod train-mnist-worker-0": [
                "2024-01-05 22:58:20 Created container init-pytorch"
              ]
            }
            ```

        Raises:
            ValueError: Job replica type is invalid.
            TimeoutError: Timeout to get Job or Job's pods
            RuntimeError: Failed to get Job or Job's pods.
        """

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind

        pods = self.get_job_pods(
            name=name,
            namespace=namespace,
            is_master=is_master,
            replica_type=replica_type,
            replica_index=replica_index,
            timeout=timeout,
        )

        logs_dict = {}
        events_dict = {}
        if pods and follow:
            log_streams = []
            for pod in pods:
                if (
                    pod.status is not None
                    and pod.status.phase != constants.POD_PHASE_PENDING
                ):
                    log_streams.append(
                        watch.Watch().stream(
                            self.core_api.read_namespaced_pod_log,
                            name=pod.metadata.name,
                            namespace=namespace,
                            container=constants.JOB_PARAMETERS[job_kind]["container"],
                        )
                    )
            finished = [False for _ in log_streams]

            # Create thread and queue per stream, for non-blocking iteration
            log_queue_pool = utils.get_log_queue_pool(log_streams)

            # Iterate over every watching pods' log queue
            while True:
                for index, log_queue in enumerate(log_queue_pool):
                    if all(finished):
                        break
                    if finished[index]:
                        continue
                    # grouping the every 50 log lines of the same pod
                    for _ in range(50):
                        try:
                            logline = log_queue.get(timeout=1)
                            if logline is None:
                                finished[index] = True
                                break

                            # Print logs to the StdOut
                            print(f"[Pod {pods[index].metadata.name}]: {logline}")
                            # Add logs to the results dict.
                            if pods[index].metadata.name not in logs_dict:
                                logs_dict[pods[index].metadata.name] = logline
                            else:
                                logs_dict[pods[index].metadata.name] += logline
                        except queue.Empty:
                            break
                if all(finished):
                    break
        elif pods:
            for pod in pods:
                if (
                    pod.status is not None
                    and pod.status.phase != constants.POD_PHASE_PENDING
                ):
                    try:
                        pod_logs = self.core_api.read_namespaced_pod_log(
                            name=pod.metadata.name,
                            namespace=namespace,
                            container=constants.JOB_PARAMETERS[job_kind]["container"],
                        )
                        logs_dict[pod.metadata.name] = pod_logs
                    except Exception:
                        raise RuntimeError(
                            f"Failed to read logs for pod {namespace}/{pod.metadata.name}"
                        )
        # If verbose is set, return Kubernetes events for Job and pods.
        if verbose:
            job = self.get_job(name=name, namespace=namespace)
            events = self.core_api.list_namespaced_event(namespace=namespace)

            # Get events for the Job and Job's pods.
            for event in events.items:
                utils.add_event_to_dict(
                    events_dict=events_dict,
                    event=event,
                    object_kind=job_kind,
                    object_name=name,
                    object_creation_timestamp=job.metadata.creation_timestamp,
                )
                if pods:
                    for pod in pods:
                        utils.add_event_to_dict(
                            events_dict=events_dict,
                            event=event,
                            object_kind=constants.POD_KIND,
                            object_name=pod.metadata.name,
                            object_creation_timestamp=pod.metadata.creation_timestamp,
                        )

        return logs_dict, events_dict

    def update_job(
        self,
        job: constants.JOB_MODELS_TYPE,
        name: str,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
    ):
        """Update the Training Job by using patch Kubernetes API.

        Args:
            job: Job object. For example, object with type
                KubeflowOrgV1TFJob or KubeflowOrgV1PyTorchJob.
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
        Raises:
            TimeoutError: Timeout to update Job
            RuntimeError: Failed to update Job
        """

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind

        if job_kind not in constants.JOB_PARAMETERS:
            raise ValueError(
                f"Job kind must be one of these: {constants.JOB_PARAMETERS.keys()}"
            )

        try:
            self.custom_api.patch_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                namespace,
                constants.JOB_PARAMETERS[job_kind]["plural"],
                name,
                job,
            )
        except multiprocessing.TimeoutError:
            raise TimeoutError(f"Timeout to update {job_kind}: {namespace}/{name}")
        except Exception:
            raise RuntimeError(f"Failed to update {job_kind}: {namespace}/{name}")

        logger.debug(f"{job_kind} {namespace}/{name} has been updated")

    def delete_job(
        self,
        name: str,
        namespace: Optional[str] = None,
        job_kind: Optional[str] = None,
        delete_options: Optional[models.V1DeleteOptions] = None,
    ):
        """Delete the Training Job

        Args:
            name: Name for the Job.
            namespace: Namespace for the Job. By default namespace is taken from
                `TrainingClient` object.
            job_kind: Kind for the Job (e.g. `TFJob` or `PyTorchJob`). By default Job kind
                is taken from `TrainingClient` object.
            delete_options: Optional, V1DeleteOptions to set while deleting
                the Job. For example, grace period seconds.

        Raises:
            TimeoutError: Timeout to delete Job.
            RuntimeError: Failed to delete Job.
        """

        namespace = namespace or self.namespace
        job_kind = job_kind or self.job_kind

        try:
            self.custom_api.delete_namespaced_custom_object(
                constants.GROUP,
                constants.VERSION,
                namespace,
                constants.JOB_PARAMETERS[job_kind]["plural"],
                name=name,
                body=delete_options,
            )
        except multiprocessing.TimeoutError:
            raise TimeoutError(f"Timeout to delete {job_kind}: {namespace}/{name}")
        except Exception:
            raise RuntimeError(f"Failed to delete {job_kind}: {namespace}/{name}")

        logger.debug(f"{job_kind} {namespace}/{name} has been deleted")
