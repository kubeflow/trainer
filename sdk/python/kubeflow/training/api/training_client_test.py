import multiprocessing
import queue
from datetime import datetime, timedelta
from unittest.mock import Mock, patch

import pytest
from kubeflow.training import (
    KubeflowOrgV1JobCondition,
    KubeflowOrgV1JobStatus,
    KubeflowOrgV1PyTorchJob,
    KubeflowOrgV1PyTorchJobSpec,
    KubeflowOrgV1ReplicaSpec,
    KubeflowOrgV1RunPolicy,
    TrainingClient,
    constants,
)
from kubeflow.training.models import V1DeleteOptions
from kubernetes.client import (
    ApiClient,
    V1Container,
    V1EnvVar,
    V1ObjectMeta,
    V1PodSpec,
    V1PodTemplateSpec,
    V1Volume,
    V1VolumeMount,
)

TEST_NAME = "test"
TEST_IMAGE = "docker.io/test-training"

TIMEOUT = "timeout"
RUNTIME = "runtime"
MOCK_POD_OBJ = "mock_pod_obj"
NO_PODS = "no_pods"
DUMMY_POD_NAME = "Dummy V1PodList"
LIST_RESPONSE = [
    {"metadata": {"name": DUMMY_POD_NAME}},
]
SUCCESS = "success"
FAILED = "Failed"
CREATED = "Created"
RUNNING = "Running"
RESTARTING = "Restarting"
SUCCEEDED = "Succeeded"
INVALID = "invalid"

FAIL_LOGS = "fail_logs"
FAIL_EVENTS = "fail_events"
MULTI_PODS = "multi_pods"
PENDING_POD = "pending_pod"
NO_STATUS_POD = "no_status_pod"
QUEUE_TIMEOUT = "queue_timeout"
QUEUE_EMPTY = "queue_empty"
EVENT_CREATION_TIMESTAMP = datetime(2024, 1, 5, 22, 58, 20)


def conditional_error_handler(*args, **kwargs):
    if args[2] == TIMEOUT:
        raise multiprocessing.TimeoutError()
    elif args[2] == RUNTIME:
        raise RuntimeError()


def serialize_k8s_object(obj):
    api_client = ApiClient()
    return api_client.sanitize_for_serialization(obj)


def get_namespaced_custom_object_response(*args, **kwargs):
    if args[2] == TIMEOUT:
        raise multiprocessing.TimeoutError()
    elif args[2] == RUNTIME:
        raise RuntimeError()

    # Create a serialized Job
    serialized_job = serialize_k8s_object(generate_job_with_status(create_job()))

    # Mock the thread and set it's return value to the serialized Job
    mock_thread = Mock()
    mock_thread.get.return_value = serialized_job

    return mock_thread


def list_namespaced_custom_object_response(*args, **kwargs):
    if args[2] == TIMEOUT:
        raise multiprocessing.TimeoutError()
    elif args[2] == RUNTIME:
        raise RuntimeError()
    elif args[2] == "empty-namespace":
        mock_response = {"items": []}
    elif args[2] == "multi-jobs":
        mock_response = {
            "items": [
                serialize_k8s_object(generate_job_with_status(create_job())),
                serialize_k8s_object(generate_job_with_status(create_job())),
            ]
        }
    else:
        mock_response = {
            "items": [serialize_k8s_object(generate_job_with_status(create_job()))]
        }

    mock_thread = Mock()
    mock_thread.get.return_value = mock_response
    return mock_thread


def list_namespaced_pod_response(*args, **kwargs):
    class MockResponse:
        def get(self, timeout):
            """
            Simulates Kubernetes API response for listing namespaced pods,
            and pass timeout for verification

            :return:
                - If `args[0] == "timeout"`, raises `TimeoutError`.
                - If `args[0] == "runtime"`, raises `Exception`.
                - If `args[0] == "mock_pod_obj"`, returns a mock pod object
                  with `metadata.name = "Dummy V1PodList"`.
                - If `args[0] == "no_pods"`, returns an empty list of pods.
                - Otherwise, returns a default list of dicts representing pods,
                  with `timeout` included, for testing.
            """
            LIST_RESPONSE[0][TIMEOUT] = timeout
            if args[0] == TIMEOUT:
                raise multiprocessing.TimeoutError()
            if args[0] == RUNTIME:
                raise Exception()
            if args[0] == MOCK_POD_OBJ:
                pod_obj = Mock(metadata=Mock())
                pod_obj.metadata.name = DUMMY_POD_NAME
                return Mock(items=[pod_obj])
            if args[0] == NO_PODS:
                return Mock(items=[])
            return Mock(items=LIST_RESPONSE)

    return MockResponse()


def create_job(
    command=None,
    args=None,
    num_workers=2,
    labels=None,
    annotations=None,
    env_vars=None,
    volumes=None,
    volume_mounts=None,
):
    # Handle env_vars as either a dict or a list
    if env_vars:
        if isinstance(env_vars, dict):
            env_vars = [V1EnvVar(name=k, value=v) for k, v in env_vars.items()]
        elif isinstance(env_vars, list):
            env_vars = [
                v if isinstance(v, V1EnvVar) else V1EnvVar(**v) for v in env_vars
            ]

    container = V1Container(
        name=constants.PYTORCHJOB_CONTAINER,
        image=TEST_IMAGE,
        command=command,
        args=args,
        env=env_vars,
        volume_mounts=volume_mounts,
    )

    master = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                annotations={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(
                containers=[container],
                volumes=volumes,
            ),
        ),
    )

    pytorch_replica_specs = {"Master": master}

    # PyTorchJob always has 1 master and N-1 worker replicas.
    if num_workers > 1:
        pytorch_replica_specs["Worker"] = KubeflowOrgV1ReplicaSpec(
            replicas=num_workers - 1,
            template=V1PodTemplateSpec(
                metadata=V1ObjectMeta(
                    annotations={constants.ISTIO_SIDECAR_INJECTION: "false"}
                ),
                spec=V1PodSpec(
                    containers=[container],
                    volumes=volumes,
                ),
            ),
        )

    meta_kwargs = {"name": TEST_NAME, "namespace": TEST_NAME}
    if labels is not None:
        meta_kwargs["labels"] = labels
    if annotations is not None:
        meta_kwargs["annotations"] = annotations

    pytorchjob = KubeflowOrgV1PyTorchJob(
        api_version=constants.API_VERSION,
        kind=constants.PYTORCHJOB_KIND,
        metadata=V1ObjectMeta(**meta_kwargs),
        spec=KubeflowOrgV1PyTorchJobSpec(
            run_policy=KubeflowOrgV1RunPolicy(clean_pod_policy=None),
            pytorch_replica_specs=pytorch_replica_specs,
        ),
    )

    return pytorchjob


# Check if actual string contains all elements from the expected list.
class AnyStringWithElementsFromList:
    def __init__(self, expected):
        self.expected = expected

    def __eq__(self, actual):
        return all(e in str(actual) for e in self.expected)


def create_job_from_func(num_workers, packages_to_install=None, pip_index_url=None):

    command = constants.DEFAULT_COMMAND
    if num_workers > 1:
        args = [f'{constants.ENTRYPOINT_TORCH} "$program_path/ephemeral_script.py"']
    else:
        args = [f'{constants.ENTRYPOINT_PYTHON} "$program_path/ephemeral_script.py"']

    if pip_index_url and packages_to_install:
        args += [f"--index-url {pip_index_url} {packages_to_install[0]}"]

    job = create_job(command, AnyStringWithElementsFromList(args), num_workers)

    return job


def generate_job_with_status(
    job: constants.JOB_MODELS_TYPE,
    condition_type: str = constants.JOB_CONDITION_SUCCEEDED,
) -> constants.JOB_MODELS_TYPE:
    job.status = KubeflowOrgV1JobStatus(
        conditions=[
            KubeflowOrgV1JobCondition(
                type=condition_type,
                status=constants.CONDITION_STATUS_TRUE,
            )
        ]
    )
    return job


class DummyJobClass:
    def __init__(self, kind) -> None:
        self.kind = kind


def generate_pod(status, name=DUMMY_POD_NAME, timestamp=None):
    pod = Mock(metadata=Mock())
    pod.metadata.name = name
    pod.metadata.creation_timestamp = timestamp
    pod.status = status
    return pod


def mock_get_job_pods(*args, **kwargs):
    """Mock get_job_pods to return controlled pod objects"""
    namespace = kwargs.get("namespace")
    if namespace == f"pod {TIMEOUT}":
        raise TimeoutError()
    if namespace == f"pod {RUNTIME}":
        raise RuntimeError()
    if namespace == INVALID:
        raise ValueError()

    # Handle different test scenarios
    if namespace == MULTI_PODS:
        return [generate_pod(Mock(phase=RUNNING), f"pod-{i}") for i in range(3)]

    # To find relevant events, the pod's creation time must precede the event's creation time
    pod_creation_timestamp = EVENT_CREATION_TIMESTAMP - timedelta(seconds=1)
    pod = generate_pod(None, timestamp=pod_creation_timestamp)
    if namespace == PENDING_POD:
        pod.status = Mock(phase=constants.POD_PHASE_PENDING)
    elif namespace == NO_STATUS_POD:
        pod.status = None
    else:
        pod.status = Mock(phase=RUNNING)
    return [pod]


def mock_get_job(*args, **kwargs):
    """Mock get_job_pods to return controlled pod objects"""
    namespace = kwargs.get("namespace")
    if namespace == f"job {TIMEOUT}":
        raise TimeoutError()
    if namespace == f"job {RUNTIME}":
        raise RuntimeError()

    # Handle different test scenarios
    job = Mock()
    # To find relevant events, the job's creation time must precede the event's creation time
    job.metadata = Mock(
        creation_timestamp=EVENT_CREATION_TIMESTAMP - timedelta(seconds=1)
    )
    return job


def mock_read_namespaced_pod_log(*args, **kwargs):
    """Mock for reading pod logs"""
    if kwargs.get("namespace") == FAIL_LOGS:
        raise Exception("Failed to read logs")
    return "test log content"


def mock_watch(self, *args, **kwargs):
    namespace = kwargs.get("namespace")
    if namespace == FAIL_LOGS:
        raise Exception("Failed to read logs")
    if namespace == QUEUE_TIMEOUT:
        log_lines = [TIMEOUT]
    elif namespace == QUEUE_EMPTY:
        log_lines = [QUEUE_EMPTY]
    else:
        log_lines = ["line 1 of pod logs", "line 2 of pod logs", "line 3 of pod logs"]
    return iter(log_lines)


def mock_get_log_queue_pool(log_streams):
    mock_logs = []
    for stream in log_streams:
        # Convert iterator to list to preserve values
        log_lines = list(stream)
        mock_queue = Mock()
        # Use a list to maintain state between calls
        remaining_logs = log_lines.copy()  # Make a copy to avoid modifying original

        def get_next(timeout, logs=remaining_logs):
            if logs:
                log = logs.pop(0)
                if log == TIMEOUT:
                    raise TimeoutError
                if log == QUEUE_EMPTY:
                    raise queue.Empty
                return log
            return None

        mock_queue.get = Mock(side_effect=get_next)
        mock_queue.put = Mock()
        mock_logs.append(mock_queue)
    return mock_logs


def mock_list_namespaced_event(*args, **kwargs):
    """Mock for listing namespace events"""

    class MockEvent:
        def __init__(self, kind, name):
            self.involved_object = Mock(kind=kind)
            self.involved_object.name = name
            self.metadata = Mock(creation_timestamp=EVENT_CREATION_TIMESTAMP)
            self.message = f"{kind} Event 1"

    class MockEventList:
        def __init__(self):
            self.items = [
                MockEvent(constants.POD_KIND, DUMMY_POD_NAME),
                MockEvent(constants.PYTORCHJOB_KIND, TEST_NAME),
            ]

    if kwargs.get("namespace") == FAIL_EVENTS:
        raise Exception("Failed to read events")
    return MockEventList()


test_data_create_job = [
    (
        "valid flow",
        {"job": create_job(), "namespace": TEST_NAME},
        SUCCESS,
        create_job(),
    ),
    (
        "valid flow to create multi-node job with torchrun",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "train_func": lambda: print("Test Training Function"),
            "base_image": TEST_IMAGE,
            "num_workers": 3,
            "packages_to_install": ["boto3==1.34.14"],
            "pip_index_url": "https://pypi.custom.com/simple",
        },
        SUCCESS,
        create_job_from_func(
            num_workers=3,
            packages_to_install=["boto3==1.34.1"],
            pip_index_url="https://pypi.custom.com/simple",
        ),
    ),
    (
        "valid flow to create job with 1 worker",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "train_func": lambda: print("Test Training Function"),
            "base_image": TEST_IMAGE,
            "num_workers": 1,
        },
        SUCCESS,
        create_job_from_func(num_workers=1),
    ),
    (
        "valid flow to create job using image",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "base_image": TEST_IMAGE,
            "num_workers": 2,
        },
        SUCCESS,
        create_job(num_workers=2),
    ),
    (
        "invalid extra parameter",
        {
            "job": create_job(),
            "namespace": TEST_NAME,
            "base_image": "test_image",
        },
        ValueError,
        None,
    ),
    (
        "invalid job kind",
        {"job_kind": "invalid_job_kind"},
        ValueError,
        None,
    ),
    (
        "job name missing with train function",
        {"train_func": lambda: "test train function"},
        ValueError,
        None,
    ),
    (
        "job name missing with base image",
        {"base_image": "test_image"},
        ValueError,
        None,
    ),
    (
        "uncallable train function",
        {
            "name": TEST_NAME,
            "train_func": "uncallable train function",
        },
        ValueError,
        None,
    ),
    (
        "invalid number of workers",
        {
            "name": TEST_NAME,
            "num_workers": 0,
        },
        ValueError,
        None,
    ),
    (
        "num_procs_per_worker is set for TFJob",
        {
            "name": TEST_NAME,
            "job_kind": constants.TFJOB_KIND,
            "num_procs_per_worker": 5,
            "base_image": "test_image",
        },
        ValueError,
        None,
    ),
    (
        "num_chief_replicas and num_ps_replicas is set for PyTorchJov",
        {
            "name": TEST_NAME,
            "num_chief_replicas": 1,
            "num_ps_replicas": 1,
            "base_image": "test_image",
        },
        ValueError,
        None,
    ),
    (
        "paddle job can't be created using function",
        {
            "name": TEST_NAME,
            "train_func": lambda: "test train function",
            "job_kind": constants.PADDLEJOB_KIND,
        },
        ValueError,
        None,
    ),
    (
        "invalid job object",
        {"job": DummyJobClass(constants.TFJOB_KIND)},
        ValueError,
        None,
    ),
    (
        "create_namespaced_custom_object timeout error",
        {"job": create_job(), "namespace": TIMEOUT},
        TimeoutError,
        None,
    ),
    (
        "create_namespaced_custom_object runtime error",
        {"job": create_job(), "namespace": RUNTIME},
        RuntimeError,
        None,
    ),
    (
        "valid flow with env_vars as dict",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "env_vars": {"ENV_VAR": "env_value"},
            "base_image": TEST_IMAGE,
            "num_workers": 1,
        },
        SUCCESS,
        create_job(env_vars={"ENV_VAR": "env_value"}, num_workers=1),
    ),
    (
        "valid flow with env_vars as list",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "env_vars": [V1EnvVar(name="ENV_VAR", value="env_value")],
            "base_image": TEST_IMAGE,
            "num_workers": 2,
        },
        SUCCESS,
        create_job(
            env_vars=[V1EnvVar(name="ENV_VAR", value="env_value")], num_workers=2
        ),
    ),
    (
        "create job with a volume and a volume mount",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "base_image": TEST_IMAGE,
            "num_workers": 1,
            "volumes": [V1Volume(name="vol")],
            "volume_mounts": [V1VolumeMount(name="vol", mount_path="/mnt")],
        },
        SUCCESS,
        create_job(
            num_workers=1,
            volumes=[V1Volume(name="vol")],
            volume_mounts=[V1VolumeMount(name="vol", mount_path="/mnt")],
        ),
    ),
    (
        "invalid number of volume mount",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "base_image": TEST_IMAGE,
            "num_workers": 1,
            "volumes": [V1Volume(name="vol")],
        },
        ValueError,
        None,
    ),
    (
        "valid flow with labels",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "base_image": TEST_IMAGE,
            "num_workers": 1,
            "labels": {"upstream": "kubeflow", "component": "training"},
        },
        SUCCESS,
        create_job(
            num_workers=1,
            labels={"upstream": "kubeflow", "component": "training"},
            annotations=None,
        ),
    ),
    (
        "valid flow with annotations",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "base_image": TEST_IMAGE,
            "num_workers": 1,
            "annotations": {"purpose": "unit-test", "env": "test"},
        },
        SUCCESS,
        create_job(
            num_workers=1,
            labels=None,
            annotations={"purpose": "unit-test", "env": "test"},
        ),
    ),
]

test_data_get_job_pods = [
    (
        "valid flow with default namespace and default timeout",
        {
            "name": TEST_NAME,
        },
        f"{constants.JOB_NAME_LABEL}={TEST_NAME}",
        LIST_RESPONSE,
    ),
    (
        "invalid replica_type",
        {"name": TEST_NAME, "replica_type": "invalid_replica_type"},
        "Label not relevant",
        ValueError,
    ),
    (
        "invalid replica_type (uppercase)",
        {"name": TEST_NAME, "replica_type": constants.REPLICA_TYPE_WORKER},
        "Label not relevant",
        ValueError,
    ),
    (
        "valid flow with specific timeout, replica_index, replica_type and master role",
        {
            "name": TEST_NAME,
            "namespace": "test_namespace",
            "timeout": 60,
            "is_master": True,
            "replica_type": constants.REPLICA_TYPE_MASTER.lower(),
            "replica_index": 0,
        },
        f"{constants.JOB_NAME_LABEL}={TEST_NAME},"
        f"{constants.JOB_ROLE_LABEL}={constants.JOB_ROLE_MASTER},"
        f"{constants.REPLICA_TYPE_LABEL}={constants.REPLICA_TYPE_MASTER.lower()},"
        f"{constants.REPLICA_INDEX_LABEL}=0",
        LIST_RESPONSE,
    ),
    (
        "invalid flow with TimeoutError",
        {
            "name": TEST_NAME,
            "namespace": TIMEOUT,
        },
        "Label not relevant",
        TimeoutError,
    ),
    (
        "invalid flow with RuntimeError",
        {
            "name": TEST_NAME,
            "namespace": RUNTIME,
        },
        "Label not relevant",
        RuntimeError,
    ),
]

test_data_wait_for_job_conditions = [
    (
        "timeout waiting for succeeded condition",
        {
            "name": TEST_NAME,
            "namespace": TIMEOUT,
            "wait_timeout": 0,
        },
        TimeoutError,
    ),
    (
        "invalid expected condition",
        {
            "name": TEST_NAME,
            "namespace": "value",
            "expected_conditions": {"invalid"},
        },
        ValueError,
    ),
    (
        "invalid expected condition(lowercase)",
        {
            "name": TEST_NAME,
            "namespace": "value",
            "expected_conditions": {"succeeded"},
        },
        ValueError,
    ),
    (
        "job failed unexpectedly",
        {
            "name": TEST_NAME,
            "namespace": RUNTIME,
        },
        RuntimeError,
    ),
    (
        "valid case",
        {
            "name": TEST_NAME,
            "namespace": "test-namespace",
        },
        generate_job_with_status(create_job()),
    ),
    (
        "valid case with specified callback",
        {
            "name": TEST_NAME,
            "namespace": "test-namespace",
            "callback": lambda job: "test train function",
        },
        generate_job_with_status(create_job()),
    ),
]


test_data_get_job_pod_names = [
    (
        "valid flow",
        {
            "name": TEST_NAME,
            "namespace": MOCK_POD_OBJ,
        },
        [DUMMY_POD_NAME],
    ),
    (
        "valid flow with no pods available",
        {
            "name": TEST_NAME,
            "namespace": NO_PODS,
        },
        [],
    ),
]


test_data_update_job = [
    (
        "valid flow",
        {
            "name": TEST_NAME,
            "job": create_job(),
        },
        "No output",
    ),
    (
        "invalid job_kind",
        {
            "name": TEST_NAME,
            "job": create_job(),
            "job_kind": "invalid_job_kind",
        },
        ValueError,
    ),
    (
        "invalid flow with TimeoutError",
        {
            "name": TEST_NAME,
            "namespace": TIMEOUT,
            "job": create_job(),
        },
        TimeoutError,
    ),
    (
        "invalid flow with RuntimeError",
        {
            "name": TEST_NAME,
            "namespace": RUNTIME,
            "job": create_job(),
        },
        RuntimeError,
    ),
]

test_data_get_job = [
    (
        "valid flow with all parameters set",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        SUCCESS,
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"name": TEST_NAME, "job_kind": constants.TFJOB_KIND},
        RuntimeError,
    ),
    (
        "invalid flow incorrect parameter",
        {"name": TEST_NAME, "test": "example"},
        TypeError,
    ),
    (
        "invalid flow with incorrect value",
        {"name": TEST_NAME, "job_kind": INVALID},
        ValueError,
    ),
    (
        "runtime error case",
        {
            "name": TEST_NAME,
            "namespace": "runtime",
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        RuntimeError,
    ),
    (
        "invalid flow with timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
]

test_data_list_jobs = [
    (
        "valid flow with default namespace and default timeout",
        {},
        SUCCESS,
        [generate_job_with_status(create_job())],
    ),
    (
        "valid flow with all parameters set",
        {
            "namespace": TEST_NAME,
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        SUCCESS,
        [generate_job_with_status(create_job())],
    ),
    (
        "valid flow with empty job list",
        {
            "namespace": "empty-namespace",
        },
        SUCCESS,
        [],
    ),
    (
        "valid flow with multiple jobs",
        {
            "namespace": "multi-jobs",
        },
        SUCCESS,
        [
            generate_job_with_status(create_job()),
            generate_job_with_status(create_job()),
        ],
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"job_kind": constants.TFJOB_KIND},
        RuntimeError,
        None,
    ),
    (
        "invalid flow with incorrect parameter",
        {"test": "example"},
        TypeError,
        None,
    ),
    (
        "invalid flow with incorrect job_kind value",
        {"job_kind": "FailJob"},
        ValueError,
        None,
    ),
    (
        "runtime error case",
        {"namespace": RUNTIME},
        RuntimeError,
        None,
    ),
    (
        "timeout error case",
        {"namespace": TIMEOUT},
        TimeoutError,
        None,
    ),
]


test_data_delete_job = [
    (
        "valid flow with default namespace",
        {
            "name": TEST_NAME,
        },
        SUCCESS,
    ),
    (
        "invalid extra parameter",
        {"name": TEST_NAME, "namespace": TEST_NAME, "example": "test"},
        TypeError,
    ),
    (
        "invalid job kind",
        {"name": TEST_NAME, "job_kind": "invalid_job_kind"},
        RuntimeError,
    ),
    (
        "job name missing",
        {"namespace": TEST_NAME, "job_kind": constants.PYTORCHJOB_KIND},
        TypeError,
    ),
    (
        "delete_namespaced_custom_object timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
    (
        "delete_namespaced_custom_object runtime error",
        {"name": TEST_NAME, "namespace": RUNTIME},
        RuntimeError,
    ),
    (
        "valid flow",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        SUCCESS,
    ),
    (
        "valid flow with delete options",
        {
            "name": TEST_NAME,
            "delete_options": V1DeleteOptions(grace_period_seconds=30),
        },
        SUCCESS,
    ),
]


test_data_get_job_conditions = [
    (
        "valid flow with failed job condition",
        {"name": TEST_NAME, "namespace": FAILED},
        generate_job_with_status(
            create_job(), condition_type=constants.JOB_CONDITION_FAILED
        ),
    ),
    (
        "valid flow with restarting job condition",
        {"name": TEST_NAME, "namespace": RESTARTING},
        generate_job_with_status(
            create_job(), condition_type=constants.JOB_CONDITION_RESTARTING
        ),
    ),
    (
        "valid flow with running job condition",
        {"name": TEST_NAME, "namespace": RUNNING},
        generate_job_with_status(
            create_job(), condition_type=constants.JOB_CONDITION_RUNNING
        ),
    ),
    (
        "valid flow with created job condition",
        {"name": TEST_NAME, "namespace": CREATED},
        generate_job_with_status(
            create_job(), condition_type=constants.JOB_CONDITION_CREATED
        ),
    ),
    (
        "valid flow with all parameters set",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "job": create_job(),
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        generate_job_with_status(create_job()),
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"name": TEST_NAME, "job_kind": constants.TFJOB_KIND},
        RuntimeError,
    ),
    (
        "invalid flow incorrect parameter",
        {"name": TEST_NAME, "test": "example"},
        TypeError,
    ),
    (
        "invalid flow with incorrect value",
        {"name": TEST_NAME, "job_kind": INVALID},
        ValueError,
    ),
    (
        "runtime error case",
        {
            "name": TEST_NAME,
            "namespace": "runtime",
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        RuntimeError,
    ),
    (
        "invalid flow with timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
]


test_data_is_job_created = [
    (
        "valid flow with all parameters set",
        {
            "name": TEST_NAME,
            "namespace": CREATED,
            "job": create_job(),
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        True,
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"name": TEST_NAME, "job_kind": constants.TFJOB_KIND},
        RuntimeError,
    ),
    (
        "invalid flow incorrect parameter",
        {"name": TEST_NAME, "test": "example"},
        TypeError,
    ),
    (
        "invalid flow with incorrect value",
        {"name": TEST_NAME, "job_kind": INVALID},
        ValueError,
    ),
    (
        "runtime error case",
        {
            "name": TEST_NAME,
            "namespace": "runtime",
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        RuntimeError,
    ),
    (
        "invalid flow with timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
]


test_data_is_job_running = [
    (
        "valid flow with job that is running",
        {"name": TEST_NAME, "namespace": RUNNING},
        True,
    ),
    (
        "valid flow with all parameters set",
        {
            "name": TEST_NAME,
            "namespace": RUNNING,
            "job": create_job(),
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        True,
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"name": TEST_NAME, "job_kind": constants.TFJOB_KIND},
        RuntimeError,
    ),
    (
        "invalid flow incorrect parameter",
        {"name": TEST_NAME, "test": "example"},
        TypeError,
    ),
    (
        "invalid flow with incorrect value",
        {"name": TEST_NAME, "job_kind": INVALID},
        ValueError,
    ),
    (
        "runtime error case",
        {
            "name": TEST_NAME,
            "namespace": "runtime",
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        RuntimeError,
    ),
    (
        "invalid flow with timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
]


test_data_is_job_restarting = [
    (
        "valid flow with job that is restarting",
        {"name": TEST_NAME, "namespace": RESTARTING},
        True,
    ),
    (
        "valid flow with all parameters set",
        {
            "name": TEST_NAME,
            "namespace": RESTARTING,
            "job": create_job(),
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        True,
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"name": TEST_NAME, "job_kind": constants.TFJOB_KIND},
        RuntimeError,
    ),
    (
        "invalid flow incorrect parameter",
        {"name": TEST_NAME, "test": "example"},
        TypeError,
    ),
    (
        "invalid flow with incorrect value",
        {"name": TEST_NAME, "job_kind": INVALID},
        ValueError,
    ),
    (
        "runtime error case",
        {
            "name": TEST_NAME,
            "namespace": "runtime",
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        RuntimeError,
    ),
    (
        "invalid flow with timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
]


test_data_is_job_failed = [
    (
        "valid flow with job that is failed",
        {"name": TEST_NAME, "namespace": FAILED},
        True,
    ),
    (
        "valid flow with all parameters set",
        {
            "name": TEST_NAME,
            "namespace": FAILED,
            "job": create_job(),
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        True,
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"name": TEST_NAME, "job_kind": constants.TFJOB_KIND},
        RuntimeError,
    ),
    (
        "invalid flow incorrect parameter",
        {"name": TEST_NAME, "test": "example"},
        TypeError,
    ),
    (
        "invalid flow with incorrect value",
        {"name": TEST_NAME, "job_kind": INVALID},
        ValueError,
    ),
    (
        "runtime error case",
        {
            "name": TEST_NAME,
            "namespace": "runtime",
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        RuntimeError,
    ),
    (
        "invalid flow with timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
]


test_data_is_job_succeded = [
    (
        "valid flow with all parameters set",
        {
            "name": TEST_NAME,
            "namespace": SUCCEEDED,
            "job": create_job(),
            "job_kind": constants.PYTORCHJOB_KIND,
            "timeout": 120,
        },
        True,
    ),
    (
        "invalid flow with default namespace and a Job that doesn't exist",
        {"name": TEST_NAME, "job_kind": constants.TFJOB_KIND},
        RuntimeError,
    ),
    (
        "invalid flow incorrect parameter",
        {"name": TEST_NAME, "test": "example"},
        TypeError,
    ),
    (
        "invalid flow with incorrect value",
        {"name": TEST_NAME, "job_kind": INVALID},
        ValueError,
    ),
    (
        "runtime error case",
        {
            "name": TEST_NAME,
            "namespace": "runtime",
            "job_kind": constants.PYTORCHJOB_KIND,
        },
        RuntimeError,
    ),
    (
        "invalid flow with timeout error",
        {"name": TEST_NAME, "namespace": TIMEOUT},
        TimeoutError,
    ),
]

test_data_get_job_logs = [
    # Basic cases
    (
        "valid flow with default parameters",
        {
            "name": TEST_NAME,
        },
        {DUMMY_POD_NAME: "test log content"},
        {},
        SUCCESS,
    ),
    (
        "pod with pending status",
        {
            "name": TEST_NAME,
            "namespace": PENDING_POD,
        },
        {},  # No logs expected for pending pods
        {},
        SUCCESS,
    ),
    (
        "pod with pending status and follow",
        {
            "name": TEST_NAME,
            "namespace": PENDING_POD,
            "follow": True,
        },
        {},  # No logs expected for pending pods
        {},
        SUCCESS,
    ),
    (
        "pod with no status",
        {
            "name": TEST_NAME,
            "namespace": NO_STATUS_POD,
        },
        {},  # No logs expected
        {},
        SUCCESS,
    ),
    (
        "pod with no status and follow",
        {
            "name": TEST_NAME,
            "namespace": NO_STATUS_POD,
            "follow": True,
        },
        {},  # No logs expected
        {},
        SUCCESS,
    ),
    (
        "valid flow with logs and verbose",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "verbose": True,
        },
        {DUMMY_POD_NAME: "test log content"},
        {
            f"{constants.PYTORCHJOB_KIND.lower()}/{TEST_NAME}": [
                f"{EVENT_CREATION_TIMESTAMP.strftime('%Y-%m-%d %H:%M:%S')} PyTorchJob Event 1"
            ],
            f"{constants.POD_KIND.lower()}/{DUMMY_POD_NAME}": [
                f"{EVENT_CREATION_TIMESTAMP.strftime('%Y-%m-%d %H:%M:%S')} Pod Event 1"
            ],
        },
        SUCCESS,
    ),
    (
        "valid flow with worker logs",
        {
            "name": TEST_NAME,
            "namespace": TEST_NAME,
            "is_master": False,
            "replica_type": constants.REPLICA_TYPE_WORKER.lower(),
            "replica_index": 0,
        },
        {DUMMY_POD_NAME: "test log content"},
        {},
        SUCCESS,
    ),
    # Streaming cases
    (
        "valid flow with follow logs",
        {
            "name": TEST_NAME,
            "follow": True,
        },
        {
            DUMMY_POD_NAME: (
                "line 1 of pod logs" "line 2 of pod logs" "line 3 of pod logs"
            )
        },
        {},
        SUCCESS,
    ),
    (
        "valid flow with follow logs and multiple pods",
        {
            "name": TEST_NAME,
            "namespace": MULTI_PODS,
            "follow": True,
        },
        {
            "pod-0": ("line 1 of pod logs" "line 2 of pod logs" "line 3 of pod logs"),
            "pod-1": ("line 1 of pod logs" "line 2 of pod logs" "line 3 of pod logs"),
            "pod-2": ("line 1 of pod logs" "line 2 of pod logs" "line 3 of pod logs"),
        },
        {},
        SUCCESS,
    ),
    (
        "follow logs with queue empty",
        {
            "name": TEST_NAME,
            "namespace": QUEUE_EMPTY,
            "follow": True,
        },
        {},
        {},
        SUCCESS,
    ),
    # Error cases
    (
        "invalid replica type",
        {
            "name": TEST_NAME,
            "namespace": INVALID,
            "replica_type": "invalid_replica",
        },
        None,
        None,
        ValueError,
    ),
    (
        "timeout error when getting pods",
        {
            "name": TEST_NAME,
            "namespace": f"pod {TIMEOUT}",
        },
        None,
        None,
        TimeoutError,
    ),
    (
        "runtime error when getting pods",
        {
            "name": TEST_NAME,
            "namespace": f"pod {RUNTIME}",
        },
        None,
        None,
        RuntimeError,
    ),
    (
        "exception when reading logs with follow",
        {
            "name": TEST_NAME,
            "namespace": FAIL_LOGS,
            "follow": True,
        },
        None,
        None,
        Exception,
    ),
    (
        "runtime error when reading logs",
        {
            "name": TEST_NAME,
            "namespace": FAIL_LOGS,
        },
        None,
        None,
        RuntimeError,
    ),
    (
        "exception when reading events",
        {
            "name": TEST_NAME,
            "namespace": FAIL_EVENTS,
            "verbose": True,
        },
        None,
        None,
        Exception,
    ),
    (
        "timeout error when getting job",
        {
            "name": TEST_NAME,
            "namespace": f"job {TIMEOUT}",
            "verbose": True,
        },
        None,
        None,
        TimeoutError,
    ),
    (
        "runtime error when getting job",
        {
            "name": TEST_NAME,
            "namespace": f"job {RUNTIME}",
            "verbose": True,
        },
        None,
        None,
        RuntimeError,
    ),
    (
        "follow logs with queue timeout",
        {
            "name": TEST_NAME,
            "namespace": QUEUE_TIMEOUT,
            "follow": True,
        },
        {},  # Empty logs due to timeout
        {},
        TimeoutError,
    ),
]


@pytest.fixture
def training_client(request):
    mock_get_job_and_job_pods = (
        request.param.get("mock_get_job_and_job_pods", False)
        if hasattr(request, "param")
        else False
    )
    with patch(
        "kubernetes.client.CustomObjectsApi",
        return_value=Mock(
            create_namespaced_custom_object=Mock(side_effect=conditional_error_handler),
            patch_namespaced_custom_object=Mock(side_effect=conditional_error_handler),
            delete_namespaced_custom_object=Mock(side_effect=conditional_error_handler),
            get_namespaced_custom_object=Mock(
                side_effect=get_namespaced_custom_object_response
            ),
            list_namespaced_custom_object=Mock(
                side_effect=list_namespaced_custom_object_response
            ),
        ),
    ), patch(
        "kubernetes.client.CoreV1Api",
        return_value=Mock(
            list_namespaced_pod=Mock(side_effect=list_namespaced_pod_response),
            read_namespaced_pod_log=Mock(side_effect=mock_read_namespaced_pod_log),
            list_namespaced_event=Mock(side_effect=mock_list_namespaced_event),
        ),
    ), patch(
        "kubernetes.config.load_kube_config", return_value=Mock()
    ), patch(
        "kubernetes.watch.Watch",
        return_value=Mock(
            stream=Mock(side_effect=mock_watch),
        ),
    ), patch(
        "kubeflow.training.utils.utils.get_log_queue_pool",
        side_effect=mock_get_log_queue_pool,
    ):
        client = TrainingClient(job_kind=constants.PYTORCHJOB_KIND)
        if mock_get_job_and_job_pods:
            client.get_job_pods = Mock(side_effect=mock_get_job_pods)
            client.get_job = Mock(side_effect=mock_get_job)
        yield client


@pytest.mark.parametrize(
    "test_name,kwargs,expected_output,expected_job", test_data_create_job
)
def test_create_job(training_client, test_name, kwargs, expected_output, expected_job):
    """
    test create_job function of training client
    """
    print("Executing test:", test_name)
    try:
        training_client.create_job(**kwargs)

        assert expected_output == SUCCESS

        training_client.custom_api.create_namespaced_custom_object.assert_called_with(
            constants.GROUP,
            constants.VERSION,
            kwargs["namespace"],
            constants.JOB_PARAMETERS[constants.PYTORCHJOB_KIND]["plural"],
            expected_job,
        )
    except Exception as e:
        assert type(e) is expected_output
    print("test execution complete")


@pytest.mark.parametrize(
    "test_name,kwargs,expected_label_selector,expected_output",
    test_data_get_job_pods,
)
def test_get_job_pods(
    training_client, test_name, kwargs, expected_label_selector, expected_output
):
    """
    test get_job_pods function of training client
    """
    print("Executing test:", test_name)
    try:
        out = training_client.get_job_pods(**kwargs)
        # Verify that list_namespaced_pod called with specified arguments
        training_client.core_api.list_namespaced_pod.assert_called_with(
            kwargs.get("namespace", constants.DEFAULT_NAMESPACE),
            label_selector=expected_label_selector,
            async_req=True,
        )
        assert out[0].pop(TIMEOUT) == kwargs.get(TIMEOUT, constants.DEFAULT_TIMEOUT)
        assert out == expected_output
    except Exception as e:
        assert type(e) is expected_output
    print("test execution complete")


@pytest.mark.parametrize(
    "test_name,kwargs,expected_output",
    test_data_get_job_pod_names,
)
def test_get_job_pod_names(training_client, test_name, kwargs, expected_output):
    """
    test get_job_pod_names function of training client
    """
    print("Executing test:", test_name)
    out = training_client.get_job_pod_names(**kwargs)
    assert out == expected_output
    print("test execution complete")


@pytest.mark.parametrize("test_name,kwargs,expected_output", test_data_update_job)
def test_update_job(training_client, test_name, kwargs, expected_output):
    """
    test update_job function of training client
    """
    print("Executing test:", test_name)
    try:
        training_client.update_job(**kwargs)
        training_client.custom_api.patch_namespaced_custom_object.assert_called_with(
            constants.GROUP,
            constants.VERSION,
            kwargs.get("namespace", constants.DEFAULT_NAMESPACE),
            constants.JOB_PARAMETERS[kwargs.get("job_kind", training_client.job_kind)][
                "plural"
            ],
            kwargs.get("name"),
            kwargs.get("job"),
        )
    except Exception as e:
        assert type(e) is expected_output
    print("test execution complete")


@pytest.mark.parametrize(
    "test_name,kwargs,expected_output", test_data_wait_for_job_conditions
)
def test_wait_for_job_conditions(training_client, test_name, kwargs, expected_output):
    """
    test wait_for_job_conditions function of training client
    """
    print("Executing test:", test_name)
    try:
        out = training_client.wait_for_job_conditions(**kwargs)
        assert out == expected_output
    except Exception as e:
        assert type(e) is expected_output
    print("test execution complete")


@pytest.mark.parametrize("test_name,kwargs,expected_output", test_data_delete_job)
def test_delete_job(training_client, test_name, kwargs, expected_output):
    """
    test delete_job function of training client
    """
    print("Executing test: ", test_name)
    try:
        training_client.delete_job(**kwargs)
        assert expected_output == SUCCESS
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize("test_name,kwargs,expected_output", test_data_get_job)
def test_get_job(training_client, test_name, kwargs, expected_output):
    """
    test get_job function of training client
    """
    print("Executing test: ", test_name)

    try:
        training_client.get_job(**kwargs)
        assert expected_output == SUCCESS
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize(
    "test_name,kwargs,expected_output", test_data_get_job_conditions
)
def test_get_job_conditions(training_client, test_name, kwargs, expected_output):
    """
    test get_job_conditions function of training client
    """
    print("Executing test: ", test_name)

    try:
        training_client.get_job_conditions(**kwargs)
        if kwargs.get("namespace") is TEST_NAME:
            assert expected_output == generate_job_with_status(create_job())
        else:
            assert expected_output == generate_job_with_status(
                create_job(), condition_type=kwargs.get("namespace")
            )
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize("test_name,kwargs,expected_output", test_data_is_job_created)
def test_is_job_created(training_client, test_name, kwargs, expected_output):
    """
    test is_job_created function of training client
    """
    print("Executing test: ", test_name)

    try:
        training_client.is_job_created(**kwargs)
        if kwargs.get("namespace") is not (CREATED or RUNTIME or TIMEOUT):
            assert expected_output is False
        else:
            assert expected_output is True
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize("test_name,kwargs,expected_output", test_data_is_job_running)
def test_is_job_running(training_client, test_name, kwargs, expected_output):
    """
    test is_job_running function of training client
    """
    print("Executing test: ", test_name)

    try:
        training_client.is_job_running(**kwargs)
        if kwargs.get("namespace") is not (RUNNING or RUNTIME or TIMEOUT):
            assert expected_output is False
        else:
            assert expected_output is True
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize(
    "test_name,kwargs,expected_output", test_data_is_job_restarting
)
def test_is_job_restarting(training_client, test_name, kwargs, expected_output):
    """
    test is_is_job_restarting function of training client
    """
    print("Executing test: ", test_name)

    try:
        training_client.is_job_restarting(**kwargs)
        if kwargs.get("namespace") is not (RESTARTING or RUNTIME or TIMEOUT):
            assert expected_output is False
        else:
            assert expected_output is True
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize("test_name,kwargs,expected_output", test_data_is_job_failed)
def test_is_job_failed(training_client, test_name, kwargs, expected_output):
    """
    test is_is_job_failed function of training client
    """
    print("Executing test: ", test_name)

    try:
        training_client.is_job_failed(**kwargs)
        if kwargs.get("namespace") is not (FAILED or RUNTIME or TIMEOUT):
            assert expected_output is False
        else:
            assert expected_output is True
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize("test_name,kwargs,expected_output", test_data_is_job_succeded)
def test_is_job_succeeded(training_client, test_name, kwargs, expected_output):
    """
    test is_job_succeeded function of training client
    """
    print("Executing test: ", test_name)

    try:
        training_client.is_job_succeeded(**kwargs)
        if kwargs.get("namespace") is not (SUCCEEDED or RUNTIME or TIMEOUT):
            assert expected_output is False
        else:
            assert expected_output is True
    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")


@pytest.mark.parametrize(
    "test_name,kwargs,expected_status,expected_response", test_data_list_jobs
)
def test_list_jobs(
    training_client, test_name, kwargs, expected_status, expected_response
):
    """
    test list_jobs function of training client
    """
    print("Executing test: ", test_name)
    try:
        resp = training_client.list_jobs(**kwargs)
        assert expected_status == SUCCESS
        assert isinstance(resp, list)
        assert len(resp) == len(expected_response)
        for actual_job, expected_job in zip(resp, expected_response):
            assert actual_job.to_dict() == expected_job.to_dict()
    except Exception as e:
        assert type(e) is expected_status

    print("test execution complete")


@pytest.mark.parametrize(
    "training_client", [{"mock_get_job_and_job_pods": True}], indirect=True
)
@pytest.mark.parametrize(
    "test_name,kwargs,expected_logs,expected_events,expected_output",
    test_data_get_job_logs,
)
def test_get_job_logs(
    training_client,
    test_name,
    kwargs,
    expected_logs,
    expected_events,
    expected_output,
):
    """
    test get_job_logs function of training client
    """
    print("Executing test:", test_name)

    try:
        logs_dict, events_dict = training_client.get_job_logs(**kwargs)

        assert expected_output == SUCCESS
        assert logs_dict == expected_logs

        if kwargs.get("verbose", False):
            assert events_dict == expected_events
        else:
            assert events_dict == {}

        # Verify API calls
        training_client.get_job_pods.assert_called_with(
            name=kwargs["name"],
            namespace=kwargs.get("namespace", constants.DEFAULT_NAMESPACE),
            is_master=kwargs.get("is_master", True),
            replica_type=kwargs.get("replica_type"),
            replica_index=kwargs.get("replica_index"),
            timeout=kwargs.get("timeout", constants.DEFAULT_TIMEOUT),
        )

        if kwargs.get("verbose", False):
            training_client.get_job.assert_called_with(
                name=kwargs["name"],
                namespace=kwargs.get("namespace", constants.DEFAULT_NAMESPACE),
            )
            training_client.core_api.list_namespaced_event.assert_called_with(
                namespace=kwargs.get("namespace", constants.DEFAULT_NAMESPACE)
            )

    except Exception as e:
        assert type(e) is expected_output

    print("test execution complete")
