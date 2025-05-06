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
from kubeflow.training import KubeflowOrgV1RunPolicy
from kubeflow.training import KubeflowOrgV1TFJob
from kubeflow.training import KubeflowOrgV1TFJobSpec
from kubeflow.training import KubeflowOrgV1SchedulingPolicy
from kubeflow.training.constants import constants

import test.e2e.utils as utils
from test.e2e.constants import TEST_GANG_SCHEDULER_NAME_ENV_KEY
from test.e2e.constants import GANG_SCHEDULERS, NONE_GANG_SCHEDULERS

logging.basicConfig(format="%(message)s")
logging.getLogger("kubeflow.training.api.training_client").setLevel(logging.DEBUG)


TRAINING_CLIENT = TrainingClient(job_kind=constants.TFJOB_KIND)
JOB_NAME = "tfjob-mnist-ci-test"
CONTAINER_NAME = "tensorflow"
GANG_SCHEDULER_NAME = os.getenv(TEST_GANG_SCHEDULER_NAME_ENV_KEY, "")


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in NONE_GANG_SCHEDULERS,
    reason="For gang-scheduling",
)
def test_sdk_e2e_with_gang_scheduling(job_namespace):
    container = generate_container()

    worker = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="Never",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(
                containers=[container],
                scheduler_name=utils.get_pod_spec_scheduler_name(GANG_SCHEDULER_NAME),
            ),
        ),
    )

    unschedulable_tfjob = generate_tfjob(
        job_namespace, worker, KubeflowOrgV1SchedulingPolicy(min_available=10)
    )
    schedulable_tfjob = generate_tfjob(
        job_namespace, worker, KubeflowOrgV1SchedulingPolicy(min_available=1)
    )

    TRAINING_CLIENT.create_job(job=unschedulable_tfjob, namespace=job_namespace)
    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_unschedulable_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"TFJob E2E fails. Exception: {e}")

    TRAINING_CLIENT.update_job(schedulable_tfjob, JOB_NAME, job_namespace)
    logging.info(f"List of updated {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"TFJob E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


@pytest.mark.skipif(
    GANG_SCHEDULER_NAME in GANG_SCHEDULERS,
    reason="For plain scheduling",
)
def test_sdk_e2e(job_namespace):
    container = generate_container()

    worker = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        restart_policy="Never",
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                labels={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[container]),
        ),
    )

    tfjob = generate_tfjob(job_namespace, worker)

    TRAINING_CLIENT.create_job(job=tfjob, namespace=job_namespace)
    logging.info(f"List of created {TRAINING_CLIENT.job_kind}s")
    logging.info(TRAINING_CLIENT.list_jobs(job_namespace))

    try:
        utils.verify_job_e2e(TRAINING_CLIENT, JOB_NAME, job_namespace, wait_timeout=900)
    except Exception as e:
        utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
        TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)
        raise Exception(f"TFJob E2E fails. Exception: {e}")

    utils.print_job_results(TRAINING_CLIENT, JOB_NAME, job_namespace)
    TRAINING_CLIENT.delete_job(JOB_NAME, job_namespace)


def generate_tfjob(
    job_namespace: str,
    worker: KubeflowOrgV1ReplicaSpec,
    scheduling_policy: Optional[KubeflowOrgV1SchedulingPolicy] = None,
) -> KubeflowOrgV1TFJob:
    return KubeflowOrgV1TFJob(
        api_version=constants.API_VERSION,
        kind=constants.TFJOB_KIND,
        metadata=V1ObjectMeta(name=JOB_NAME, namespace=job_namespace),
        spec=KubeflowOrgV1TFJobSpec(
            run_policy=KubeflowOrgV1RunPolicy(
                clean_pod_policy="None",
                scheduling_policy=scheduling_policy,
            ),
            tf_replica_specs={"Worker": worker},
        ),
    )


def generate_container() -> V1Container:
    return V1Container(
        name=CONTAINER_NAME,
        image="gcr.io/kubeflow-ci/tf-mnist-with-summaries:1.0",
        command=[
            "python",
            "/var/tf_mnist/mnist_with_summaries.py",
            "--log_dir=/train/logs",
            "--learning_rate=0.01",
            "--batch_size=150",
        ],
        resources=V1ResourceRequirements(limits={"memory": "4Gi", "cpu": "1.6"}),
    )
