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

import grpc
import optuna

from pkg.search_algorithm.optuna.proto import api_pb2, api_pb2_grpc

SUPPORTED_ALGORITHM = "random"
SEED_SETTING = "random_state"

optuna.logging.set_verbosity(optuna.logging.WARNING)


def build_search_space(parameters):
    space = {}
    for p in parameters:
        fs = p.feasible_space
        if p.parameter_type == api_pb2.CATEGORICAL:
            space[p.name] = optuna.distributions.CategoricalDistribution(list(fs.list))
            continue

        if fs.distribution not in (
            api_pb2.DISTRIBUTION_UNSPECIFIED,
            api_pb2.UNIFORM,
            api_pb2.LOG_UNIFORM,
        ):
            raise ValueError(
                f"parameter {p.name!r}: unsupported distribution "
                f"{api_pb2.Distribution.Name(fs.distribution)}"
            )
        log = fs.distribution == api_pb2.LOG_UNIFORM

        if p.parameter_type == api_pb2.DOUBLE:
            space[p.name] = optuna.distributions.FloatDistribution(
                float(fs.min), float(fs.max), log=log
            )
        elif p.parameter_type == api_pb2.INT:
            space[p.name] = optuna.distributions.IntDistribution(
                int(fs.min), int(fs.max), log=log
            )
        else:
            raise ValueError(
                f"parameter {p.name!r}: unsupported type "
                f"{api_pb2.ParameterType.Name(p.parameter_type)}"
            )
    return space


def parse_seed(settings):
    for s in settings:
        if s.name == SEED_SETTING:
            return int(s.value)
    return None


class OptunaService(api_pb2_grpc.SuggestionServicer):
    def GetSuggestions(self, request, context):
        spec = request.experiment.spec
        try:
            if spec.algorithm.algorithm_name != SUPPORTED_ALGORITHM:
                raise ValueError(
                    f"unsupported algorithm {spec.algorithm.algorithm_name!r}"
                )
            space = build_search_space(spec.parameter_specs.parameters)
            seed = parse_seed(spec.algorithm.algorithm_settings)
        except ValueError as e:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))

        # The random sampler never looks at past trials, so a fresh study per call
        # is enough. Offsetting the seed by the trial count keeps suggestions
        # reproducible across restarts without storing any state.
        if seed is not None:
            seed += len(request.trials)
        study = optuna.create_study(sampler=optuna.samplers.RandomSampler(seed=seed))

        reply = api_pb2.GetSuggestionsReply()
        for _ in range(request.current_request_number):
            trial = study.ask(fixed_distributions=space)
            reply.parameter_assignments.add(
                assignments=[
                    api_pb2.ParameterAssignment(name=k, value=str(v))
                    for k, v in trial.params.items()
                ]
            )
        return reply
