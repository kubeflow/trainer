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

import logging
from concurrent import futures

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from pkg.search_algorithm.optuna.proto import api_pb2_grpc
from pkg.search_algorithm.optuna.service import OptunaService

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)

# Must match SearchAlgorithmServicePort and SearchAlgorithmServiceName in
# pkg/constants/constants.go. The readiness probe checks this service name.
PORT = 6789
HEALTH_SERVICE_NAME = "manager.v1beta1.Suggestion"


def main():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    api_pb2_grpc.add_SuggestionServicer_to_server(OptunaService(), server)

    health_servicer = health.HealthServicer()
    health_servicer.set(HEALTH_SERVICE_NAME, health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)

    server.add_insecure_port(f"[::]:{PORT}")
    server.start()
    logging.info("Optuna search algorithm listening on port %d", PORT)
    server.wait_for_termination()


if __name__ == "__main__":
    main()
