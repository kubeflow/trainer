import multiprocessing
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
    V1ObjectMeta,
    V1PodSpec,
    V1PodTemplateSpec,
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


def conditional_error_handler(*args, **kwargs):
    if args[2] == TIMEOUT:
        raise multiprocessing.TimeoutError()
    elif args[2] == RUNTIME:
        raise RuntimeError()


def serialize_k8s_object(obj):
    api_client = ApiClient()
    return api_client.sanitize_for_serialization(obj)


def get_namespaced_custom_object_response(*args, **kwargs):
    if args[2] == "timeout":
        raise multiprocessing.TimeoutError()
    elif args[2] == "runtime":
        raise RuntimeError()

    # Create a serialized Job
    serialized_job = serialize_k8s_object(generate_job_with_status(create_job()))

    # Mock the thread and set it's return value to the serialized Job
    mock_thread = Mock()
    mock_thread.get.return_value = serialized_job

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
):
    container = V1Container(
        name=constants.PYTORCHJOB_CONTAINER,
        image=TEST_IMAGE,
        command=command,
        args=args,
    )

    master = KubeflowOrgV1ReplicaSpec(
        replicas=1,
        template=V1PodTemplateSpec(
            metadata=V1ObjectMeta(
                annotations={constants.ISTIO_SIDECAR_INJECTION: "false"}
            ),
            spec=V1PodSpec(containers=[container]),
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
                spec=V1PodSpec(containers=[container]),
            ),
        )

    pytorchjob = KubeflowOrgV1PyTorchJob(
        api_version=constants.API_VERSION,
        kind=constants.PYTORCHJOB_KIND,
        metadata=V1ObjectMeta(name=TEST_NAME, namespace=TEST_NAME),
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
            "name": "test job",
            "train_func": "uncallable train function",
        },
        ValueError,
        None,
    ),
    (
        "invalid number of workers",
        {
            "name": "test job",
            "num_workers": 0,
        },
        ValueError,
        None,
    ),
    (
        "paddle job can't be created using function",
        {
            "name": "test job",
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


@pytest.fixture
def training_client():
    with patch(
        "kubernetes.client.CustomObjectsApi",
        return_value=Mock(
            create_namespaced_custom_object=Mock(side_effect=conditional_error_handler),
            patch_namespaced_custom_object=Mock(side_effect=conditional_error_handler),
            delete_namespaced_custom_object=Mock(side_effect=conditional_error_handler),
            get_namespaced_custom_object=Mock(
                side_effect=get_namespaced_custom_object_response
            ),
        ),
    ), patch(
        "kubernetes.client.CoreV1Api",
        return_value=Mock(
            list_namespaced_pod=Mock(side_effect=list_namespaced_pod_response)
        ),
    ), patch(
        "kubernetes.config.load_kube_config", return_value=Mock()
    ):
        client = TrainingClient(job_kind=constants.PYTORCHJOB_KIND)
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
