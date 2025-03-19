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
from typing import Dict

from kubeflow.trainer.types import types

# How long to wait in seconds for requests to the Kubernetes API Server.
DEFAULT_TIMEOUT = 120

# Common constants.
GROUP = "trainer.kubeflow.org"
VERSION = "v1alpha1"
API_VERSION = f"{GROUP}/{VERSION}"

# The default Kubernetes namespace.
DEFAULT_NAMESPACE = "default"

# The Kind name for the ClusterTrainingRuntime.
CLUSTER_TRAINING_RUNTIME_KIND = "ClusterTrainingRuntime"

# The plural for the ClusterTrainingRuntime.
CLUSTER_TRAINING_RUNTIME_PLURAL = "clustertrainingruntimes"

# The Kind name for the TrainJob.
TRAINJOB_KIND = "TrainJob"

# The plural for the TrainJob.
TRAINJOB_PLURAL = "trainjobs"

# The label key to identify the relationship between TrainJob and Pod template in the runtime.
# For example, what PodTemplate must be overridden by TrainJob's .spec.trainer APIs.
TRAINJOB_ANCESTOR_LABEL = "trainer.kubeflow.org/trainjob-ancestor-step"

# The label key to identify training phase where TrainingRuntime should be used.
# For example, runtime for the pre-training or post-training.
# TODO (andreyvelich): Remove it.
PHASE_KEY = "trainer.kubeflow.org/phase"

# The value indicates that runtime can be used for the model pre-training.
PHASE_PRE_TRAINING = "pre-training"

# The value indicates that runtime can be used for the model post-training.
PHASE_POST_TRAINING = "post-training"

# The label key to identify the accelerator type for model training (e.g. GPU-Tesla-V100-16GB).
# TODO: Potentially, we should take this from the Node selectors.
ACCELERATOR_LABEL = "trainer.kubeflow.org/accelerator"

# Unknown indicates that the value can't be identified.
UNKNOWN = "Unknown"

# The default type for CPU device, and it indicates the label in the container resources.
CPU_DEVICE_TYPE = "cpu"

# The label for NVIDIA GPU in the container resources.
NVIDIA_GPU_LABEL = "nvidia.com/gpu"

# The default type for GPU device.
GPU_DEVICE_TYPE = "gpu"

# The label for TPU in the container resources.
TPU_LABEL = "google.com/tpu"

# The default type for TPU device.
TPU_DEVICE_TYPE = "tpu"

# The label key to identify the JobSet name of the Pod.
JOBSET_NAME_KEY = "jobset.sigs.k8s.io/jobset-name"

# The label key to identify the JobSet's ReplicatedJob of the Pod.
REPLICATED_JOB_KEY = "jobset.sigs.k8s.io/replicatedjob-name"

# The label key to identify the Job completion index of the Pod.
JOB_INDEX_KEY = "batch.kubernetes.io/job-completion-index"

# The name of the ReplicatedJob and container of the dataset initializer.
# Also, it represents the `trainjob-ancestor-step` label value for the dataset initializer step.
DATASET_INITIALIZER = "dataset-initializer"

# The name of the ReplicatedJob and container of the model initializer.
# Also, it represents the `trainjob-ancestor-step` label value for the model initializer step.
MODEL_INITIALIZER = "model-initializer"

# The default path to the users' workspace.
# TODO (andreyvelich): Discuss how to keep this path is sync with pkg.initializers.constants
WORKSPACE_PATH = "/workspace"

# The path where initializer downloads dataset.
DATASET_PATH = os.path.join(WORKSPACE_PATH, "dataset")

# The path where initializer downloads model.
MODEL_PATH = os.path.join(WORKSPACE_PATH, "model")

# The name of the ReplicatedJob and container of the node. The node usually represents single
# VM where distributed training code is executed.
# TODO: Change it to "node"
NODE = "trainer-node"

# The `trainjob-ancestor-step` label value for the trainer step.
TRAINER = "trainer"

# The Pod pending phase indicates that Pod has been accepted by the Kubernetes cluster,
# but one or more of the containers has not been made ready to run.
POD_PENDING = "Pending"

# The default PIP index URL to download Python packages.
DEFAULT_PIP_INDEX_URL = os.getenv("DEFAULT_PIP_INDEX_URL", "https://pypi.org/simple")

# The default command for the Trainer.
DEFAULT_COMMAND = ["bash", "-c"]

# The Torch env name for the number of procs per node (e.g. number of GPUs per Pod).
TORCH_ENV_NUM_PROC_PER_NODE = "PET_NPROC_PER_NODE"

# The container entrypoint for distributed PyTorch.
TORCH_ENTRYPOINT = "torchrun"

# The name of the ReplicatedJob to launch mpirun.
MPI_LAUNCHER = "launcher"

# The OpenMPI env name for the number of slots per nude (e.g. number of GPUs per Pod).
MPI_ENV_NUM_SLOTS_PER_NODE = "OMPI_MCA_orte_set_default_slots"

# The container entrypoint for distributed MPI
MPI_ENTRYPOINT = "mpirun"


# The dict where key is the container image and value its representation.
# Each Trainer representation defines trainer parameters (e.g. type, framework, entrypoint).
# TODO (andreyvelich): We should allow user to overrides the default image names.
ALL_TRAINERS: Dict[str, types.Trainer] = {
    # Custom Trainers.
    "pytorch/pytorch": types.Trainer(
        trainer_type=types.TrainerType.CUSTOM_TRAINER,
        framework=types.Framework.TORCH,
        entrypoint="torchrun",
    ),
    "ghcr.io/kubeflow/trainer/mlx-runtime": types.Trainer(
        trainer_type=types.TrainerType.CUSTOM_TRAINER,
        framework=types.Framework.MLX,
        entrypoint="mpirun --hostfile /etc/mpi/hostfile -x LD_LIBRARY_PATH=/usr/local/lib/ python3",
    ),
    "ghcr.io/kubeflow/trainer/deepspeed-runtime": types.Trainer(
        trainer_type=types.TrainerType.CUSTOM_TRAINER,
        framework=types.Framework.DEEPSPEED,
        entrypoint="mpirun --hostfile /etc/mpi/hostfile python3",
    ),
    # Builtin Trainers.
    "ghcr.io/kubeflow/trainer/torchtune-trainer": types.Trainer(
        trainer_type=types.TrainerType.BUILTIN_TRAINER,
        framework=types.Framework.TORCHTUNE,
        entrypoint="tune run",
    ),
}

# The default trainer configuration when runtime detection fails
DEFAULT_TRAINER = types.Trainer(
    trainer_type=types.TrainerType.CUSTOM_TRAINER,
    framework=types.Framework.TORCH,
    entrypoint="torchrun",
)

# The default runtime configuration for the train() API
DEFAULT_RUNTIME = types.Runtime(
    name="torch-distributed",
    trainer=DEFAULT_TRAINER,
)
