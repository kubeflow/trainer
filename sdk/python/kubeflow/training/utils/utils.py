# Copyright 2021 kubeflow.org.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import inspect
import json
import logging
import os
import queue
import textwrap
import threading
from datetime import datetime
from typing import Any, Callable, Dict, List, Optional, Tuple, Union

from kubeflow.training import models
from kubeflow.training.constants import constants
from kubernetes import config

logger = logging.getLogger(__name__)


class StatusLogger:
    """Logger to print Training Job statuses."""

    def __init__(self, header, column_format):
        self.header = header
        self.column_format = column_format
        self.first_call = True

    def __call__(self, *values):
        if self.first_call:
            logger.debug(self.header)
            self.first_call = False
        logger.debug(self.column_format.format(*values))


class FakeResponse:
    """Fake object of RESTResponse to deserialize
    Ref) https://github.com/kubeflow/katib/pull/1630#discussion_r697877815
    Ref) https://github.com/kubernetes-client/python/issues/977#issuecomment-592030030
    """

    def __init__(self, obj):
        self.data = json.dumps(obj)


class SetEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, set):
            return list(obj)
        if isinstance(obj, type):
            return obj.__name__
        return json.JSONEncoder.default(self, obj)


def is_running_in_k8s():
    return os.path.isdir("/var/run/secrets/kubernetes.io/")


def get_default_target_namespace():
    if not is_running_in_k8s():
        try:
            _, current_context = config.list_kube_config_contexts()
            return current_context["context"]["namespace"]
        except Exception:
            return constants.DEFAULT_NAMESPACE
    with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace", "r") as f:
        return f.readline()


def wrap_log_stream(q, stream):
    while True:
        try:
            logline = next(stream)
            q.put(logline)
        except StopIteration:
            q.put(None)
            return


def get_log_queue_pool(streams):
    pool = []
    for stream in streams:
        q = queue.Queue(maxsize=100)
        pool.append(q)
        threading.Thread(target=wrap_log_stream, args=(q, stream)).start()
    return pool


def has_condition(conditions: List[models.V1JobCondition], condition_type: str) -> bool:
    """
    Verify if the condition list has the required condition.
    Condition should be valid object with `type` and `status`.
    """

    for c in conditions:
        if c.type == condition_type and c.status == constants.CONDITION_STATUS_TRUE:
            return True
    return False


def get_script_for_python_packages(
    packages_to_install: List[str], pip_index_url: str
) -> str:
    """
    Get init script to install Python packages from the given pip index URL.
    """
    packages_str = " ".join([str(package) for package in packages_to_install])

    script_for_python_packages = textwrap.dedent(
        f"""
        if ! [ -x "$(command -v pip)" ]; then
            python -m ensurepip || python -m ensurepip --user || apt-get install python-pip
        fi

        PIP_DISABLE_PIP_VERSION_CHECK=1 python -m pip install --quiet \
        --no-warn-script-location --index-url {pip_index_url} {packages_str}
        """
    )

    return script_for_python_packages


def get_command_using_train_func(
    train_func: Callable,
    entrypoint: str,
    train_func_parameters: Optional[Dict[str, Any]] = None,
    packages_to_install: Optional[List[str]] = None,
    pip_index_url: str = constants.DEFAULT_PIP_INDEX_URL,
) -> Tuple[List[str], List[str]]:
    """
    Get container args and command from the given training function and parameters.
    """
    # Check if function is callable.
    if not callable(train_func):
        raise ValueError(
            f"Training function must be callable, got function type: {type(train_func)}"
        )

    # Extract function implementation.
    func_code = inspect.getsource(train_func)

    # Function might be defined in some indented scope (e.g. in another function).
    # We need to dedent the function code.
    func_code = textwrap.dedent(func_code)

    # Wrap function code to execute it from the file. For example:
    # def train(parameters):
    #     print('Start Training...')
    # train({'lr': 0.01})
    if train_func_parameters is None:
        func_code = f"{func_code}\n{train_func.__name__}()\n"
    else:
        func_code = f"{func_code}\n{train_func.__name__}({train_func_parameters})\n"

    # Prepare execute script template.
    exec_script = textwrap.dedent(
        """
                program_path=$(mktemp -d)
                read -r -d '' SCRIPT << EOM\n
                {func_code}
                EOM
                printf "%s" \"$SCRIPT\" > \"$program_path/ephemeral_script.py\"
                {entrypoint} \"$program_path/ephemeral_script.py\""""
    )

    # Add function code to the execute script.
    exec_script = exec_script.format(func_code=func_code, entrypoint=entrypoint)

    # Install Python packages if that is required.
    if packages_to_install is not None:
        exec_script = (
            get_script_for_python_packages(packages_to_install, pip_index_url)
            + exec_script
        )

    # Return container command and args to execute training function.
    return constants.DEFAULT_COMMAND, [exec_script]


def get_container_spec(
    name: str,
    base_image: str,
    command: Optional[List[str]] = None,
    args: Optional[List[str]] = None,
    resources: Union[dict, models.V1ResourceRequirements, None] = None,
    volume_mounts: Optional[List[models.V1VolumeMount]] = None,
    env_vars: Optional[
        Union[Dict[str, str], List[Union[models.V1EnvVar, models.V1EnvVar]]]
    ] = None,
) -> models.V1Container:
    """
    Get container spec for the given parameters.
    """

    if name is None or base_image is None:
        raise ValueError("Container name or base image cannot be none")

    # Handle env_vars as either a dict or a list
    if env_vars:
        if isinstance(env_vars, dict):
            env_vars = [models.V1EnvVar(name=k, value=v) for k, v in env_vars.items()]
        elif isinstance(env_vars, list):
            env_vars = [
                v if isinstance(v, models.V1EnvVar) else models.V1EnvVar(**v)
                for v in env_vars
            ]

    # Create initial container spec.
    container_spec = models.V1Container(
        name=name,
        image=base_image,
        command=command,
        args=args,
        volume_mounts=volume_mounts,
        env=env_vars,
    )

    # Convert dict to the Kubernetes container resources if that is required.
    if isinstance(resources, dict):
        # Convert all keys in resources to lowercase.
        resources = {k.lower(): v for k, v in resources.items()}
        if "gpu" in resources:
            resources["nvidia.com/gpu"] = resources.pop("gpu")

        resources = models.V1ResourceRequirements(
            requests=resources,
            limits=resources,
        )

    # Add resources to the container spec.
    container_spec.resources = resources

    return container_spec


def get_pod_template_spec(
    containers: List[models.V1Container],
    init_containers: Optional[List[models.V1Container]] = None,
    volumes: Optional[List[models.V1Volume]] = None,
) -> models.V1PodTemplateSpec:
    """
    Get Pod template spec for the given parameters.
    """

    # Create Pod template spec. If the value is None, Pod doesn't have that parameter
    pod_template_spec = models.V1PodTemplateSpec(
        metadata=models.V1ObjectMeta(
            annotations={constants.ISTIO_SIDECAR_INJECTION: "false"}
        ),
        spec=models.V1PodSpec(
            init_containers=init_containers,
            containers=containers,
            volumes=volumes,
        ),
    )

    return pod_template_spec


def get_tfjob_template(
    name: str,
    namespace: str,
    pod_template_spec: models.V1PodTemplateSpec,
    num_workers: int,
    labels: Optional[Dict[str, str]] = None,
    annotations: Optional[Dict[str, str]] = None,
    num_chief_replicas: Optional[int] = None,
    num_ps_replicas: Optional[int] = None,
):

    # Create TFJob template.
    tfjob = models.KubeflowOrgV1TFJob(
        api_version=constants.API_VERSION,
        kind=constants.TFJOB_KIND,
        metadata=models.V1ObjectMeta(name=name, namespace=namespace),
        spec=models.KubeflowOrgV1TFJobSpec(
            run_policy=models.KubeflowOrgV1RunPolicy(clean_pod_policy=None),
            tf_replica_specs={},
        ),
    )

    # Add Chief, PS, and Worker replicas to the TFJob.
    if num_chief_replicas is not None:
        tfjob.spec.tf_replica_specs[constants.REPLICA_TYPE_CHIEF] = (
            models.KubeflowOrgV1ReplicaSpec(
                replicas=num_chief_replicas,
                template=pod_template_spec,
            )
        )

    if labels is not None:
        tfjob.metadata.labels = labels

    if annotations is not None:
        tfjob.metadata.annotations = annotations

    if num_ps_replicas is not None:
        tfjob.spec.tf_replica_specs[constants.REPLICA_TYPE_PS] = (
            models.KubeflowOrgV1ReplicaSpec(
                replicas=num_ps_replicas,
                template=pod_template_spec,
            )
        )

    if num_workers is not None:
        tfjob.spec.tf_replica_specs[constants.REPLICA_TYPE_WORKER] = (
            models.KubeflowOrgV1ReplicaSpec(
                replicas=num_workers,
                template=pod_template_spec,
            )
        )

    return tfjob


def get_pytorchjob_template(
    name: str,
    namespace: str,
    num_workers: int,
    worker_pod_template_spec: Optional[models.V1PodTemplateSpec],
    labels: Optional[Dict[str, str]] = None,
    annotations: Optional[Dict[str, str]] = None,
    master_pod_template_spec: Optional[models.V1PodTemplateSpec] = None,
    num_procs_per_worker: Optional[Union[int, str]] = None,
):

    # Create PyTorchJob template.
    pytorchjob = models.KubeflowOrgV1PyTorchJob(
        api_version=constants.API_VERSION,
        kind=constants.PYTORCHJOB_KIND,
        metadata=models.V1ObjectMeta(name=name, namespace=namespace),
        spec=models.KubeflowOrgV1PyTorchJobSpec(
            run_policy=models.KubeflowOrgV1RunPolicy(clean_pod_policy=None),
            pytorch_replica_specs={},
        ),
    )

    if labels is not None:
        pytorchjob.metadata.labels = labels

    if annotations is not None:
        pytorchjob.metadata.annotations = annotations

    if num_procs_per_worker is not None:
        pytorchjob.spec.nproc_per_node = str(num_procs_per_worker)

    # Create Master replica if that is set.
    if master_pod_template_spec:
        pytorchjob.spec.pytorch_replica_specs[constants.REPLICA_TYPE_MASTER] = (
            models.KubeflowOrgV1ReplicaSpec(
                replicas=1,
                template=master_pod_template_spec,
            )
        )
    # If we don't define Master template, use the Worker template.
    else:
        pytorchjob.spec.pytorch_replica_specs[constants.REPLICA_TYPE_MASTER] = (
            models.KubeflowOrgV1ReplicaSpec(
                replicas=1,
                template=worker_pod_template_spec,
            )
        )

    # Create Worker with num_workers - 1 replicas.
    # TODO (andreyvelich): Investigate if we can run PyTorchJob without the Master
    # Currently, if Master is not set, Training Operator controller
    # doesn't set RANK and WORLD_SIZE for PyTorchJob.
    # Ref issue: https://github.com/kubeflow/training-operator/issues/1991
    if num_workers > 1:
        pytorchjob.spec.pytorch_replica_specs[constants.REPLICA_TYPE_WORKER] = (
            models.KubeflowOrgV1ReplicaSpec(
                replicas=num_workers - 1,
                template=worker_pod_template_spec,
            )
        )

    return pytorchjob


def get_pvc_spec(
    pvc_name: str,
    namespace: str,
    storage_config: Dict[str, Optional[Union[str, List[str]]]],
):
    if pvc_name is None or namespace is None:
        raise ValueError("One of the required storage config argument is None")

    if "size" not in storage_config:
        storage_config["size"] = constants.PVC_DEFAULT_SIZE

    if "access_modes" not in storage_config:
        storage_config["access_modes"] = constants.PVC_DEFAULT_ACCESS_MODES

    pvc_spec = models.V1PersistentVolumeClaim(
        api_version="v1",
        kind="PersistentVolumeClaim",
        metadata={"name": pvc_name, "namespace": namespace},
        spec=models.V1PersistentVolumeClaimSpec(
            access_modes=storage_config["access_modes"],
            resources=models.V1ResourceRequirements(
                requests={"storage": storage_config["size"]}
            ),
        ),
    )

    if "storage_class" in storage_config:
        pvc_spec.spec.storage_class_name = storage_config["storage_class"]

    return pvc_spec


def add_event_to_dict(
    events_dict: Dict[str, List[str]],
    event: models.CoreV1Event,
    object_kind: str,
    object_name: str,
    object_creation_timestamp: datetime,
):
    """Add Kubernetes event to the dict with this format:
    ```
    {"<Object Kind>/<Object Name>": "<Event Timestamp> <Event Message>"}
    ```
    """
    if (
        event.involved_object.kind == object_kind
        and event.involved_object.name == object_name
        and event.metadata.creation_timestamp >= object_creation_timestamp
    ):
        event_key = f"{object_kind.lower()}/{object_name}"
        event_time = event.metadata.creation_timestamp.strftime("%Y-%m-%d %H:%M:%S")
        event_msg = f"{event_time} {event.message}"
        if event_key not in events_dict:
            events_dict[event_key] = [event_msg]
        else:
            events_dict[event_key] += [event_msg]
