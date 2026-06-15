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

WORKDIR /app

# Copy the required Python modules.
COPY cmd/initializers/dataset/pyproject.toml .
COPY cmd/initializers/dataset/uv.lock .
COPY pkg/initializers pkg/initializers

# Install uv and the needed packages.
RUN pip install --no-cache-dir uv && uv sync --frozen --no-dev --system

ENTRYPOINT ["python", "-m", "pkg.initializers.dataset"]
