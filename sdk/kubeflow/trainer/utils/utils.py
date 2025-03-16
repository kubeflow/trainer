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

import inspect
import os
import queue
import textwrap
import threading
from typing import Any, Callable, Dict, List, Optional, Tuple, Union

import kubeflow.trainer.models as models
from kubeflow.trainer.constants import constants
from kubeflow.trainer.types import types
from kubernetes import config


def is_running_in_k8s() -> bool:
    return os.path.isdir("/var/run/secrets/kubernetes.io/")


def get_default_target_namespace() -> str:
    if not is_running_in_k8s():
        try:
            _, current_context = config.list_kube_config_contexts()
            return current_context["context"]["namespace"]
        except Exception:
            return constants.DEFAULT_NAMESPACE
    with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace", "r") as f:
        return f.readline()


def get_container_devices(
    resources: Optional[models.IoK8sApiCoreV1ResourceRequirements],
) -> Tuple[str, Union[str, float]]:
    """
    Get the device type and device count for the given container.
    """

    # TODO (andreyvelich): We should discuss how to get container device type.
    # Potentially, we can use the trainer.kubeflow.org/device label from the runtime or
    # node types.
    device = constants.UNKNOWN
    device_count = constants.UNKNOWN

    # If containers resource limits are empty, return Unknown.
    if resources is None or resources.limits is None:
        return device, device_count

    # TODO (andreyvelich): Support other resource labels (e.g. NPUs).
    if constants.NVIDIA_GPU_LABEL in resources.limits:
        device = constants.GPU_DEVICE_TYPE
        device_count = resources.limits[constants.NVIDIA_GPU_LABEL].actual_instance
    elif constants.TPU_LABEL in resources.limits:
        device = constants.TPU_DEVICE_TYPE
        device_count = resources.limits[constants.TPU_LABEL].actual_instance
    elif constants.CPU_LABEL in resources.limits:
        device = constants.CPU_DEVICE_TYPE
        device_count = resources.limits[constants.CPU_LABEL].actual_instance
    else:
        raise Exception(
            f"Unknown device type in the container resources: {resources.limits}"
        )
    if device_count is None:
        raise Exception(f"Failed to get device count for resources: {resources.limits}")

    return device, device_count


# TODO (andreyvelich): Discuss if we want to support V1ResourceRequirements resources as input.
def get_resources_per_node(
    resources_per_node: dict,
) -> models.IoK8sApiCoreV1ResourceRequirements:
    """
    Get the Trainer resources for the training node from the given dict.
    """

    # Convert all keys in resources to lowercase.
    resources = {
        k.lower(): models.IoK8sApimachineryPkgApiResourceQuantity(v)
        for k, v in resources_per_node.items()
    }
    if "gpu" in resources:
        resources["nvidia.com/gpu"] = resources.pop("gpu")

    resources = models.IoK8sApiCoreV1ResourceRequirements(
        requests=resources,
        limits=resources,
    )
    return resources


def get_args_using_train_func(
    train_func: Callable,
    train_func_parameters: Optional[Dict[str, Any]] = None,
    packages_to_install: Optional[List[str]] = None,
    pip_index_url: Optional[str] = constants.DEFAULT_PIP_INDEX_URL,
) -> List[str]:
    """
    Get the Trainer args from the given training function and parameters.
    """
    # Check if training function is callable.
    if not callable(train_func):
        raise ValueError(
            f"Training function must be callable, got function type: {type(train_func)}"
        )

    # Extract the function implementation.
    func_code = inspect.getsource(train_func)

    # Extract the file name where the function is defined.
    func_file = os.path.basename(inspect.getfile(train_func))

    # Function might be defined in some indented scope (e.g. in another function).
    # We need to dedent the function code.
    func_code = textwrap.dedent(func_code)

    # Wrap function code to execute it from the file. For example:
    # TODO (andreyvelich): Find a better way to run users' scripts.
    # def train(parameters):
    #     print('Start Training...')
    # train({'lr': 0.01})
    if train_func_parameters is None:
        func_code = f"{func_code}\n{train_func.__name__}()\n"
    else:
        func_code = f"{func_code}\n{train_func.__name__}({train_func_parameters})\n"

    # Prepare the template to execute script.
    # Currently, we override the file where the training function is defined.
    # That allows to execute the training script with the entrypoint.
    exec_script = textwrap.dedent(
        """
                program_path=$(mktemp -d)
                read -r -d '' SCRIPT << EOM\n
                {func_code}
                EOM
                printf "%s" \"$SCRIPT\" > \"{func_file}\"
                {entrypoint} \"{func_file}\""""
    )

    # Add function code to the execute script.
    # TODO (andreyvelich): Add support for other entrypoints.
    exec_script = exec_script.format(
        func_code=func_code,
        func_file=func_file,
        entrypoint=constants.ENTRYPOINT_TORCH,
    )

    # Install Python packages if that is required.
    if packages_to_install is not None and pip_index_url is not None:
        exec_script = (
            get_script_for_python_packages(packages_to_install, pip_index_url)
            + exec_script
        )

    # Return container command and args to execute training function.
    return [exec_script]


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


def get_dataset_initializer(
    dataset: Optional[types.HuggingFaceDatasetInitializer] = None,
) -> Optional[models.TrainerV1alpha1DatasetInitializer]:
    """
    Get the TrainJob dataset initializer from the given config.
    """
    if not isinstance(dataset, types.HuggingFaceDatasetInitializer):
        return None

    # TODO (andreyvelich): Support more parameters.
    dataset_initializer = models.TrainerV1alpha1DatasetInitializer(
        storageUri=(
            dataset.storage_uri
            if dataset.storage_uri.startswith("hf://")
            else "hf://" + dataset.storage_uri
        )
    )

    return dataset_initializer


def get_model_initializer(
    model: Optional[types.HuggingFaceModelInitializer] = None,
) -> Optional[models.TrainerV1alpha1ModelInitializer]:
    """
    Get the TrainJob model initializer from the given config.
    """
    if not isinstance(model, types.HuggingFaceModelInitializer):
        return None

    # TODO (andreyvelich): Support more parameters.
    model_initializer = models.TrainerV1alpha1ModelInitializer(
        storageUri=(
            model.storage_uri
            if model.storage_uri.startswith("hf://")
            else "hf://" + model.storage_uri
        )
    )

    return model_initializer


def wrap_log_stream(q: queue.Queue, log_stream: Any):
    while True:
        try:
            logline = next(log_stream)
            q.put(logline)
        except StopIteration:
            q.put(None)
            return


def get_log_queue_pool(log_streams: List[Any]) -> List[queue.Queue]:
    pool = []
    for log_stream in log_streams:
        q = queue.Queue(maxsize=100)
        pool.append(q)
        threading.Thread(target=wrap_log_stream, args=(q, log_stream)).start()
    return pool
