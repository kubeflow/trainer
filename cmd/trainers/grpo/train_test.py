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

import importlib
import os
import sys
from dataclasses import dataclass
from unittest.mock import MagicMock, patch

import pytest

sys.path.insert(0, os.path.dirname(__file__))
train = importlib.import_module("train")  # noqa: E402

get_env = train.get_env
get_env_int = train.get_env_int
get_env_float = train.get_env_float
length_reward = train.length_reward
load_reward_function = train.load_reward_function


@dataclass
class GRPOTestCase:
    name: str
    env_vars: dict
    expected: object = None
    expected_error: type = None


@pytest.mark.parametrize(
    "test_case",
    [
        GRPOTestCase(
            name="returns value when env var is set",
            env_vars={"MODEL_PATH": "/custom/model"},
            expected="/custom/model",
        ),
        GRPOTestCase(
            name="returns default when env var is not set",
            env_vars={},
            expected="/default/path",
        ),
    ],
)
def test_get_env(test_case, monkeypatch):
    for key, value in test_case.env_vars.items():
        monkeypatch.setenv(key, value)
    if "MODEL_PATH" not in test_case.env_vars:
        monkeypatch.delenv("MODEL_PATH", raising=False)

    result = get_env("MODEL_PATH", "/default/path")
    assert result == test_case.expected


@pytest.mark.parametrize(
    "test_case",
    [
        GRPOTestCase(
            name="exits when required env var is missing and no default",
            env_vars={},
            expected_error=SystemExit,
        ),
    ],
)
def test_get_env_missing_required(test_case, monkeypatch):
    monkeypatch.delenv("REQUIRED_VAR", raising=False)
    with pytest.raises(test_case.expected_error):
        get_env("REQUIRED_VAR")


@pytest.mark.parametrize(
    "test_case",
    [
        GRPOTestCase(
            name="parses integer from env",
            env_vars={"NUM_GENERATIONS": "8"},
            expected=8,
        ),
        GRPOTestCase(
            name="returns default when env var is not set",
            env_vars={},
            expected=4,
        ),
    ],
)
def test_get_env_int(test_case, monkeypatch):
    for key, value in test_case.env_vars.items():
        monkeypatch.setenv(key, value)
    if "NUM_GENERATIONS" not in test_case.env_vars:
        monkeypatch.delenv("NUM_GENERATIONS", raising=False)

    result = get_env_int("NUM_GENERATIONS", 4)
    assert result == test_case.expected


@pytest.mark.parametrize(
    "test_case",
    [
        GRPOTestCase(
            name="parses float from env",
            env_vars={"LEARNING_RATE": "1e-5"},
            expected=1e-5,
        ),
        GRPOTestCase(
            name="returns default when env var is not set",
            env_vars={},
            expected=5e-7,
        ),
    ],
)
def test_get_env_float(test_case, monkeypatch):
    for key, value in test_case.env_vars.items():
        monkeypatch.setenv(key, value)
    if "LEARNING_RATE" not in test_case.env_vars:
        monkeypatch.delenv("LEARNING_RATE", raising=False)

    result = get_env_float("LEARNING_RATE", 5e-7)
    assert result == test_case.expected


@pytest.mark.parametrize(
    "test_case",
    [
        GRPOTestCase(
            name="rewards based on completion length",
            env_vars={},
            expected=[5.0, 11.0, 0.0],
        ),
    ],
)
def test_length_reward(test_case):
    completions = ["hello", "hello world", ""]
    result = length_reward(completions)
    assert result == test_case.expected


@pytest.mark.parametrize(
    "test_case",
    [
        GRPOTestCase(
            name="returns default reward when no module specified",
            env_vars={},
        ),
        GRPOTestCase(
            name="exits when module file cannot be loaded",
            env_vars={"REWARD_FUNCTION_MODULE": "/nonexistent/module.py"},
            expected_error=SystemExit,
        ),
    ],
)
def test_load_reward_function(test_case, monkeypatch):
    for key, value in test_case.env_vars.items():
        monkeypatch.setenv(key, value)
    if "REWARD_FUNCTION_MODULE" not in test_case.env_vars:
        monkeypatch.delenv("REWARD_FUNCTION_MODULE", raising=False)

    if test_case.expected_error:
        with pytest.raises(test_case.expected_error):
            load_reward_function()
    else:
        fn = load_reward_function()
        assert callable(fn)
        assert fn == length_reward


@pytest.mark.parametrize(
    "test_case",
    [
        GRPOTestCase(
            name="main initializes GRPOTrainer with correct config",
            env_vars={
                "MODEL_PATH": "/workspace/model",
                "DATASET_PATH": "/workspace/dataset",
                "OUTPUT_DIR": "/workspace/output",
                "NUM_GENERATIONS": "4",
                "MAX_PROMPT_LENGTH": "512",
                "MAX_COMPLETION_LENGTH": "256",
                "PER_DEVICE_TRAIN_BATCH_SIZE": "2",
                "GRADIENT_ACCUMULATION_STEPS": "4",
                "NUM_TRAIN_EPOCHS": "1",
                "LEARNING_RATE": "5e-7",
                "LOGGING_STEPS": "1",
            },
        ),
    ],
)
def test_main(test_case, monkeypatch, tmp_path):
    for key, value in test_case.env_vars.items():
        monkeypatch.setenv(key, value)

    dataset_dir = tmp_path / "dataset"
    dataset_dir.mkdir()
    monkeypatch.setenv("DATASET_PATH", str(dataset_dir))

    mock_dataset = MagicMock()
    mock_trainer_instance = MagicMock()

    with (
        patch.object(train, "load_from_disk", return_value=mock_dataset) as mock_load,
        patch.object(train, "GRPOConfig") as mock_config_cls,
        patch.object(
            train, "GRPOTrainer", return_value=mock_trainer_instance
        ) as mock_trainer_cls,
    ):
        train.main()

        mock_load.assert_called_once_with(str(dataset_dir))
        mock_config_cls.assert_called_once()
        mock_trainer_cls.assert_called_once()
        mock_trainer_instance.train.assert_called_once()
        mock_trainer_instance.save_model.assert_called_once_with("/workspace/output")
