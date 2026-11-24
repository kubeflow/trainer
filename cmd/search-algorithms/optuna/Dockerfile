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

FROM python:3.14-slim-bookworm

COPY --from=ghcr.io/astral-sh/uv:0.7 /uv /uvx /bin/

WORKDIR /app

# Copy the required Python modules.
COPY cmd/search-algorithms/optuna/pyproject.toml .
COPY cmd/search-algorithms/optuna/uv.lock .
COPY pkg/search_algorithm pkg/search_algorithm

# Generate the gRPC stubs, then drop grpcio-tools from the final environment.
RUN uv sync --locked && \
    uv run --no-sync python -m grpc_tools.protoc -I . --python_out=. --grpc_python_out=. \
        pkg/search_algorithm/optuna/proto/api.proto && \
    uv sync --locked --no-dev

ENV PATH="/app/.venv/bin:$PATH"

EXPOSE 6789

ENTRYPOINT ["python", "-m", "pkg.search_algorithm.optuna"]
