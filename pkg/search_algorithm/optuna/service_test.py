# Copyright The Kubeflow Authors.
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

from unittest.mock import MagicMock

import grpc
import pytest

from pkg.search_algorithm.optuna.proto import api_pb2
from pkg.search_algorithm.optuna.service import OptunaService


class Aborted(Exception):
    pass


def make_request(
    algorithm="random",
    seed="42",
    num_trials=0,
    count=3,
    lr_distribution=api_pb2.LOG_UNIFORM,
):
    parameters = [
        api_pb2.ParameterSpec(
            name="lr",
            parameter_type=api_pb2.DOUBLE,
            feasible_space=api_pb2.FeasibleSpace(
                min="0.0001", max="0.1", distribution=lr_distribution
            ),
        ),
        api_pb2.ParameterSpec(
            name="batch_size",
            parameter_type=api_pb2.INT,
            feasible_space=api_pb2.FeasibleSpace(
                min="16", max="128", distribution=api_pb2.UNIFORM
            ),
        ),
        api_pb2.ParameterSpec(
            name="optimizer",
            parameter_type=api_pb2.CATEGORICAL,
            feasible_space=api_pb2.FeasibleSpace(list=["adam", "sgd"]),
        ),
    ]
    settings = []
    if seed is not None:
        settings.append(api_pb2.AlgorithmSetting(name="random_state", value=seed))

    request = api_pb2.GetSuggestionsRequest(
        experiment=api_pb2.Experiment(
            spec=api_pb2.ExperimentSpec(
                algorithm=api_pb2.AlgorithmSpec(
                    algorithm_name=algorithm, algorithm_settings=settings
                ),
                parameter_specs=api_pb2.ExperimentSpec.ParameterSpecs(
                    parameters=parameters
                ),
            )
        ),
        current_request_number=count,
        total_request_number=num_trials,
    )
    for i in range(num_trials):
        request.trials.add(name=f"trial-{i}")
    return request


def get_suggestions(request):
    reply = OptunaService().GetSuggestions(request, MagicMock())
    return [
        {a.name: a.value for a in pa.assignments} for pa in reply.parameter_assignments
    ]


def test_suggestions_in_search_space():
    suggestions = get_suggestions(make_request(count=20))

    assert len(suggestions) == 20
    for s in suggestions:
        assert 0.0001 <= float(s["lr"]) <= 0.1
        assert 16 <= int(s["batch_size"]) <= 128
        assert s["optimizer"] in ("adam", "sgd")


def test_same_history_gives_same_suggestions():
    # A restarted service sees the same trials and must suggest the same values.
    first = get_suggestions(make_request(num_trials=4))
    second = get_suggestions(make_request(num_trials=4))

    assert first == second


def test_new_history_gives_new_suggestions():
    first = get_suggestions(make_request(num_trials=0))
    second = get_suggestions(make_request(num_trials=3))

    assert first != second


def test_log_uniform_samples_on_log_scale():
    suggestions = get_suggestions(make_request(count=200))
    lrs = sorted(float(s["lr"]) for s in suggestions)

    # The median of a log-uniform sample over [1e-4, 1e-1] is near 3e-3,
    # while a uniform sample would put it near 5e-2.
    assert lrs[len(lrs) // 2] < 0.01


def test_no_seed():
    suggestions = get_suggestions(make_request(seed=None))

    assert len(suggestions) == 3


@pytest.mark.parametrize(
    "test_case",
    [
        pytest.param(
            {
                "request": make_request(algorithm="tpe"),
                "match": "unsupported algorithm",
            },
            id="unsupported-algorithm",
        ),
        pytest.param(
            {
                "request": make_request(lr_distribution=api_pb2.NORMAL),
                "match": "unsupported distribution NORMAL",
            },
            id="unsupported-distribution",
        ),
    ],
)
def test_invalid_request(test_case):
    context = MagicMock()
    context.abort.side_effect = Aborted

    with pytest.raises(Aborted):
        OptunaService().GetSuggestions(test_case["request"], context)

    code, message = context.abort.call_args.args
    assert code == grpc.StatusCode.INVALID_ARGUMENT
    assert test_case["match"] in message
