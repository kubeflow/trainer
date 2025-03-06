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
import json
import math
import os
import queue
import textwrap
import threading
from typing import Any, Callable, Dict, List, Optional, Tuple

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
) -> Tuple[str, str]:
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
        device_count = resources.limits[constants.NVIDIA_GPU_LABEL]
    elif constants.TPU_LABEL in resources.limits:
        device = constants.TPU_DEVICE_TYPE
        device_count = resources.limits[constants.TPU_LABEL]
    elif constants.CPU_LABEL in resources.limits:
        device = constants.CPU_DEVICE_TYPE
        device_count = resources.limits[constants.CPU_LABEL]
    else:
        raise Exception(
            f"Unknown device type in the container resources: {resources.limits}"
        )

    return device, device_count


# TODO (andreyvelich): Discuss how to make this validation easier for users.
def validate_trainer(trainer: types.Trainer):
    """
    Validate that trainer has the correct configuration.
    """

    if (
        trainer.func or trainer.func_args or trainer.packages_to_install
    ) and trainer.fine_tuning_config:
        raise ValueError(
            "Trainer function parameters and fine tuning config can't be set together"
        )


# TODO (andreyvelich): Discuss if we want to support V1ResourceRequirements resources as input.
def get_resources_per_node(
    resources_per_node: dict,
) -> models.IoK8sApiCoreV1ResourceRequirements:
    """
    Get the Trainer resources for the training node from the given dict.
    """

    # Convert all keys in resources to lowercase.
    resources = {k.lower(): str(v) for k, v in resources_per_node.items()}
    if "gpu" in resources:
        resources["nvidia.com/gpu"] = resources.pop("gpu")

    resources = models.IoK8sApiCoreV1ResourceRequirements(
        requests=resources,
        limits=resources,
    )
    return resources


# TODO (andreyvelich): Move this part to the Kubeflow Trainer torch plugins.
# Ref issue: https://github.com/kubeflow/trainer/issues/2407
def get_num_proc_per_node(resources_per_node: dict) -> str:
    """
    Get the Trainer numProcPerNode from the given resources.
    """

    resources = {k.lower(): v for k, v in resources_per_node.items()}
    # NumProcPerNode is equal to number of GPUs or CPUs, otherwise set it to `auto`
    for key, value in resources.items():
        if "gpu" in key:
            return str(value)

    for key, value in resources.items():
        if "cpu" in key:
            # For now, we can't convert milliCPUs to the numProcPerNode.
            try:
                value = math.ceil(int(value))
                return str(value)
            except Exception:
                pass

    return "auto"


def get_args_using_train_func(
    train_func: Callable,
    train_func_parameters: Optional[Dict[str, Any]] = None,
    packages_to_install: Optional[List[str]] = None,
    pip_index_url: str = constants.DEFAULT_PIP_INDEX_URL,
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
    if packages_to_install is not None:
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


def get_lora_config(lora_config: types.LoraConfig) -> List[models.IoK8sApiCoreV1EnvVar]:
    """
    Get the TrainJob env from the given Lora config.
    """

    env = models.IoK8sApiCoreV1EnvVar(
        name=constants.ENV_LORA_CONFIG, value=json.dumps(lora_config.__dict__)
    )
    return [env]


def get_dataset_config(
    dataset_config: Optional[types.HuggingFaceDatasetConfig] = None,
) -> Optional[models.TrainerV1alpha1DatasetConfig]:
    """
    Get the TrainJob DatasetConfig from the given config.
    """
    if dataset_config is None:
        return None

    # TODO (andreyvelich): Support more parameters.
    ds_config = models.TrainerV1alpha1DatasetConfig(
        storageUri=(
            dataset_config.storage_uri
            if dataset_config.storage_uri.startswith("hf://")
            else "hf://" + dataset_config.storage_uri
        )
    )

    return ds_config


def get_model_config(
    model_config: Optional[types.HuggingFaceModelInputConfig] = None,
) -> Optional[models.TrainerV1alpha1ModelConfig]:
    """
    Get the TrainJob ModelConfig from the given config.
    """
    if model_config is None:
        return None

    # TODO (andreyvelich): Support more parameters.
    m_config = models.TrainerV1alpha1ModelConfig(
        input=models.TrainerV1alpha1InputModel(storageUri=model_config.storage_uri)
    )

    return m_config


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
