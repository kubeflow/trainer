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

import os
import logging
import pytest
from typing import Tuple, Optional

from kubernetes.client import V1PodTemplateSpec
from kubernetes.client import V1ObjectMeta
from kubernetes.client import V1PodSpec
from kubernetes.client import V1Container
from kubernetes.client import V1ResourceRequirements

from kubeflow.training import TrainingClient
from kubeflow.training import KubeflowOrgV1ReplicaSpec
from kubeflow.training import KubeflowOrgV1MPIJob
from kubeflow.training import KubeflowOrgV1MPIJobSpec
from kubeflow.training import KubeflowOrgV1RunPolicy
from kubeflow.training import KubeflowOrgV1SchedulingPolicy
from kubeflow.training.constants import constants

import test.e2e.utils as utils
from test.e2e.constants import TEST_GANG_SCHEDULER_NAME_ENV_KEY
from test.e2e.constants import GANG_SCHEDULERS, NONE_GANG_SCHEDULERS

logging.basicConfig(format="%(message)s")
logging.getLogger("kubeflow.training.api.training_client").setLevel(logging.DEBUG)

TRAINING_CLIENT = TrainingClient(job_kind=constants.MPIJOB_KIND)
JOB_NAME = "mpijob-pytorch-ci-test"
CONTAINER_NAME = "mpi"
GANG_SCHEDULER_NAME = os.getenv(TEST_GANG_SCHEDULER_NAME_ENV_KEY, "")


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in NONE_GANG_SCHEDULERS,
    reason="For gang-scheduling",
)
def test_sdk_e2e_with_gang_scheduling(job_namespace):
    launcher_container, worker_container = generate_containers()

    launcher = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="Never",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(
                containers=[launcher_container],
                scheduler_name=utils.get_pod_spec_scheduler_name(GANG_SCHEDULER_NAME),
            ),
        ),
    )

    worker = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="Never",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(
                containers=[worker_container],
                scheduler_name=utils.get_pod_spec_scheduler_name(GANG_SCHEDULER_NAME),
            ),
        ),
    )

    mpijob = generate_mpijob(
        job_namespace, launcher, worker, KubeflowOrgV1SchedulingPolicy(min_available=10)
    )
    patched_mpijob = generate_mpijob(
        job_namespace, launcher, worker, KubeflowOrgV1SchedulingPolicy(min_available=2)
    )

    TRAINING_CLIENT.create_job(job=mpijob, namespace=job_namespace)
    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_unschedulable_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"MPIJob E2E fails. Exception: {e}")

    TRAINING_CLIENT.update_job(patched_mpijob, JOB_NAME, job_namespace)
    logging.info(f"List of updated {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"MPIJob E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in GANG_SCHEDULERS,
    reason="For plain scheduling",
)
def test_sdk_e2e(job_namespace):
    launcher_container, worker_container = generate_containers()

    launcher = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="Never",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[launcher_container]),
        ),
    )

    worker = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="Never",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[worker_container]),
        ),
    )

    mpijob = generate_mpijob(job_namespace, launcher, worker)

    TRAINING_CLIENT.create_job(job=mpijob, namespace=job_namespace)
    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"MPIJob E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


def generate_mpijob(
    job_namespace: str,
    launcher: KubeflowOrgV1ReplicaSpec,
    worker: KubeflowOrgV1ReplicaSpec,
    scheduling_policy: Optional[KubeflowOrgV1SchedulingPolicy] = None,
) -> KubeflowOrgV1MPIJob:
    return KubeflowOrgV1MPIJob(
        api_version=constants.API_VERSION,
        kind=constants.MPIJOB_KIND,
        metadata=V1ObjectMeta(name=JOB_NAME, namespace=job_namespace),
        spec=KubeflowOrgV1MPIJobSpec(
            slots_per_worker=1,
            run_policy=KubeflowOrgV1RunPolicy(
                clean_pod_policy="None",
                scheduling_policy=scheduling_policy,
            ),
            mpi_replica_specs={"Launcher": launcher, "Worker": worker},
        ),
    )


def generate_containers() -> Tuple[V1Container, V1Container]:
    launcher_container = V1Container(
        name=CONTAINER_NAME,
        image="horovod/horovod:0.28.1",
        command=["mpirun"],
        args=[
            "-np",
            "1",
            "--allow-run-as-root",
            "-bind-to",
            "none",
            "-map-by",
            "slot",
            "-x",
            "LD_LIBRARY_PATH",
            "-x",
            "PATH",
            "-mca",
            "pml",
            "ob1",
            "-mca",
            "btl",
            "^openib",
            "python",
            "/horovod/examples/pytorch/pytorch_mnist.py",
            "--epochs",
            "1",
        ],
        resources=V1ResourceRequirements(limits={"memory": "1Gi", "cpu": "0.4"}),
    )

    worker_container = V1Container(
        name=CONTAINER_NAME,
        image="horovod/horovod:0.28.1",
        resources=V1ResourceRequirements(limits={"memory": "3Gi", "cpu": "1.2"}),
    )

    return launcher_container, worker_container
