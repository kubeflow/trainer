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
from typing import Optional

from kubernetes.client import V1PodTemplateSpec
from kubernetes.client import V1ObjectMeta
from kubernetes.client import V1PodSpec
from kubernetes.client import V1Container
from kubernetes.client import V1ResourceRequirements

from kubeflow.training import TrainingClient
from kubeflow.training import KubeflowOrgV1ReplicaSpec
from kubeflow.training import KubeflowOrgV1PyTorchJob
from kubeflow.training import KubeflowOrgV1PyTorchJobSpec
from kubeflow.training import KubeflowOrgV1RunPolicy
from kubeflow.training import KubeflowOrgV1SchedulingPolicy
from kubeflow.training import constants

import test.e2e.utils as utils
from test.e2e.constants import TEST_GANG_SCHEDULER_NAME_ENV_KEY
from test.e2e.constants import GANG_SCHEDULERS, NONE_GANG_SCHEDULERS

logging.basicConfig(format="%(message)s")
logging.getLogger("kubeflow.training.api.training_client").setLevel(logging.DEBUG)

TRAINING_CLIENT = TrainingClient(job_kind=constants.PYTORCHJOB_KIND)
CONTAINER_NAME = "pytorch"
GANG_SCHEDULER_NAME = os.getenv(TEST_GANG_SCHEDULER_NAME_ENV_KEY, "")


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in NONE_GANG_SCHEDULERS,
    reason="For gang-scheduling",
)
def test_sdk_e2e_with_gang_scheduling(job_namespace):
    JOB_NAME = "pytorchjob-gang-scheduling"
    container = generate_container()

    master = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="OnFailure",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(
                scheduler_name=utils.get_pod_spec_scheduler_name(GANG_SCHEDULER_NAME),
                containers=[container],
            ),
        ),
    )

    worker = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="OnFailure",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(
                scheduler_name=utils.get_pod_spec_scheduler_name(GANG_SCHEDULER_NAME),
                containers=[container],
            ),
        ),
    )

    unschedulable_pytorchjob = generate_pytorchjob(
        job_namespace,
        JOB_NAME,
        master,
        worker,
        KubeflowOrgV1SchedulingPolicy(min_available=10),
    )
    schedulable_pytorchjob = generate_pytorchjob(
        job_namespace,
        JOB_NAME,
        master,
        worker,
        KubeflowOrgV1SchedulingPolicy(min_available=2),
    )

    TRAINING_CLIENT.create_job(job=unschedulable_pytorchjob, namespace=job_namespace)
    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_unschedulable_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"PyTorchJob E2E fails. Exception: {e}")

    TRAINING_CLIENT.update_job(schedulable_pytorchjob, JOB_NAME, job_namespace)
    logging.info(f"List of updated {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"PyTorchJob E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in GANG_SCHEDULERS,
    reason="For plain scheduling",
)
def test_sdk_e2e(job_namespace):
    JOB_NAME = "pytorchjob-e2e"
    container = generate_container()

    master = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="OnFailure",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[container]),
        ),
    )

    worker = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="OnFailure",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[container]),
        ),
    )

    pytorchjob = generate_pytorchjob(job_namespace, JOB_NAME, master, worker)

    TRAINING_CLIENT.create_job(job=pytorchjob, namespace=job_namespace)
    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"PyTorchJob E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in GANG_SCHEDULERS,
    reason="For plain scheduling",
)
def test_sdk_e2e_managed_by(job_namespace):
    JOB_NAME = "pytorchjob-e2e"
    container = generate_container()

    master = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="OnFailure",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[container]),
        ),
    )

    worker = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="OnFailure",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[container]),
        ),
    )

    #1. Job created with default value: 'kubeflow.org/training-operator' - job created and status updated
    #2. Job created with kueue value: 'kueue.x-k8s.io/multikueue' - job created but status not updated
    #3. Job created with invalid value (not acceptable by the webhook) - job not created
    controllers = {
        JOB_NAME+"-default-controller": 'kubeflow.org/training-operator',
        JOB_NAME+"-multikueue-controller": 'kueue.x-k8s.io/multikueue', 
        JOB_NAME+"-invalid-controller": 'kueue.x-k8s.io/other-controller',
    }
    for job_name, managed_by in controllers.items():
        pytorchjob = generate_pytorchjob(job_namespace, job_name, master, worker, managed_by=managed_by)
        try:
            TRAINING_CLIENT.create_job(job=pytorchjob, namespace=job_namespace)
        except Exception as e:
            if "invalid" in str(job_name):
                error_message = f"Failed to create PyTorchJob: {job_namespace}/{job_name}"
                assert error_message in str(e), f"Unexpected error: {e}"
            else:
                raise Exception(f"PyTorchJob E2E fails. Exception: {e}")
    
    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    jobs = TRAINING_CLIENT.list_jobs(job_namespace)
    logging.info(jobs)

    try:
        #Only jobs with valid controllers should be created, 2 out of 3 satisfy this condition: 'kubeflow.org/training-operator' and 'kueue.x-k8s.io/multikueue'
        if len(jobs) != 2:
            raise Exception(f"Too many PyTorchJobs created {jobs}")
    
        for job in jobs:
            if job._metadata.name == 'kubeflow.org/training-operator':
                utils.verify_job_e2e(TRAINING_CLIENT, job._metadata.name, job_namespace, wait_timeout=900)
            if job._metadata.name == 'kueue.x-k8s.io/multikueue':
                conditions = TRAINING_CLIENT.get_job_conditions(job._metadata.name, job_namespace, TRAINING_CLIENT.job_kind, job)
                if len(conditions) != 0:
                    raise Exception(f"{TRAINING_CLIENT.job_kind} conditions {conditions} should not be updated, externally managed by {managed_by}")


    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"PyTorchJob E2E fails. Exception: {e}")

    for job in jobs:
        utils.print_job_results(TRAINING_CLIENT, job._metadata.name, job_namespace)
        TRAINING_CLIENT.delete_job(job._metadata.name, job_namespace)

@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in GANG_SCHEDULERS,
    reason="For plain scheduling",
)
def test_sdk_e2e_create_from_func(job_namespace):
    JOB_NAME = "pytorchjob-from-func"

    def train_func():
        import time

        for i in range(10):
            print(f"Start training for Epoch {i}")
            time.sleep(1)

    num_workers = 3

    TRAINING_CLIENT.create_job(
        name=JOB_NAME,
        namespace=job_namespace,
        train_func=train_func,
        num_workers=num_workers,
    )

    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"PyTorchJob create from function E2E fails. Exception: {e}")

    # Verify that PyTorchJob has correct pods.
    pod_names = TRAINING_CLIENT.get_job_pod_names(
        name=JOB_NAME, namespace=job_namespace
    )

    if len(pod_names) != num_workers or f"{JOB_NAME}-worker-0" not in pod_names:
        raise Exception(f"PyTorchJob has incorrect pods: {pod_names}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in GANG_SCHEDULERS,
    reason="For plain scheduling",
)
def test_sdk_e2e_create_from_image(job_namespace):
    JOB_NAME = "pytorchjob-from-image"

    TRAINING_CLIENT.create_job(
        name=JOB_NAME,
        namespace=job_namespace,
        base_image="docker.io/hello-world",
        num_workers=1,
    )

    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"PyTorchJob create from function E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


def generate_pytorchjob(
    job_namespace: str,
    job_name: str,
    master: KubeflowOrgV1ReplicaSpec,
    worker: KubeflowOrgV1ReplicaSpec,
    scheduling_policy: Optional[KubeflowOrgV1SchedulingPolicy] = None,
    managed_by: Optional[str] = None,
) -> KubeflowOrgV1PyTorchJob:
    return KubeflowOrgV1PyTorchJob(
        api_version=constants.API_VERSION,
        kind=constants.PYTORCHJOB_KIND,
        metadata=V1ObjectMeta(name=job_name, namespace=job_namespace),
        spec=KubeflowOrgV1PyTorchJobSpec(
            run_policy=KubeflowOrgV1RunPolicy(
                clean_pod_policy="None",
                scheduling_policy=scheduling_policy,
                managed_by=managed_by,
            ),
            pytorch_replica_specs={"Master": master, "Worker": worker},
        ),
    )


def generate_container() -> V1Container:
    return V1Container(
        name=CONTAINER_NAME,
        image="kubeflow/pytorch-dist-mnist:latest",
        args=["--backend", "gloo", "--epochs", "1"],
        resources=V1ResourceRequirements(limits={"memory": "2Gi", "cpu": "0.8"}),
    )
