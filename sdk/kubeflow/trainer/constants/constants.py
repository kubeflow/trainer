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

# The label key to identify training phase where TrainingRuntime should be used.
# For example, runtime for the pre-training or post-training.
PHASE_KEY = "trainer.kubeflow.org/phase"

# The value indicates that runtime can be used for the model pre-training.
PHASE_PRE_TRAINING = "pre-training"

# The value indicates that runtime can be used for the model post-training.
PHASE_POST_TRAINING = "post-training"

# The label key to identify the accelerator type for model training (e.g. GPU-Tesla-V100-16GB).
# TODO: Potentially, we should take this from the Node selectors.
ACCELERATOR_KEY = "trainer.kubeflow.org/accelerator"

# Unknown indicates that the value can't be identified.
UNKNOWN = "Unknown"

# The label for CPU in the container resources.
CPU_LABEL = "cpu"

# The default type for CPU device.
CPU_DEVICE_TYPE = "cpu"

# The label for NVIDIA GPU in the container resources.
NVIDIA_GPU_LABEL = "nvidia.com/gpu"

# The default type for GPU device.
GPU_DEVICE_TYPE = "gpu"

# The label for TPU in the container resources.
TPU_LABEL = "google.com/tpu"

# The default type for TPU device.
TPU_DEVICE_TYPE = "tpu"

# The Kind name for the TrainJob.
TRAINJOB_KIND = "TrainJob"

# The plural for the TrainJob.
TRAINJOB_PLURAL = "trainjobs"

# The default PIP index URL to download Python packages.
DEFAULT_PIP_INDEX_URL = os.getenv("DEFAULT_PIP_INDEX_URL", "https://pypi.org/simple")

# The default command for the Trainer.
DEFAULT_COMMAND = ["bash", "-c"]

# Distributed PyTorch entrypoint.
ENTRYPOINT_TORCH = "torchrun"

# The label key to identify the JobSet name of the Pod.
JOBSET_NAME_KEY = "jobset.sigs.k8s.io/jobset-name"

# The label key to identify the JobSet's ReplicatedJob of the Pod.
REPLICATED_JOB_KEY = "jobset.sigs.k8s.io/replicatedjob-name"

# The label key to identify the Job completion index of the Pod.
JOB_INDEX_KEY = "batch.kubernetes.io/job-completion-index"

# The Job name for the initializer.
JOB_INITIALIZER = "initializer"

# The container name for the dataset initializer.
CONTAINER_DATASET_INITIALIZER = "dataset-initializer"

# The container name for the model initializer.
CONTAINER_MODEL_INITIALIZER = "model-initializer"

# The default path to the users' workspace.
WORKSPACE_PATH = "/workspace"

# The path where initializer downloads dataset.
DATASET_PATH = os.path.join(WORKSPACE_PATH, "dataset")

# The path where initializer downloads model.
MODEL_PATH = os.path.join(WORKSPACE_PATH, "model")

# The Job name for the launcher (e.g. mpirun launcher).
JOB_LAUNCHER = "launcher"

# The container name for the launcher
CONTAINER_LAUNCHER = "launcher"

# The Job name for the trainer nodes.
JOB_TRAINER_NODE = "trainer-node"

# The Pod type for the master node.
MASTER_NODE = f"{JOB_TRAINER_NODE}-0"

# The Pod pending phase indicates that Pod has been accepted by the Kubernetes cluster,
# but one or more of the containers has not been made ready to run.
POD_PENDING = "Pending"

# The container name for the Trainer.
CONTAINER_TRAINER = "trainer"

# The Torch env name for the number of procs per node (e.g. number of GPUs per Pod).
TORCH_ENV_NUM_PROC_PER_NODE = "PET_NPROC_PER_NODE"

# Env variable for the Lora config
ENV_LORA_CONFIG = "LORA_CONFIG"
