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

FROM pytorch/pytorch:2.9.1-cuda12.8-cudnn9-runtime

WORKDIR /opt/grpo

# Install build-essential for compiling any dependencies
RUN apt update && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy the required Python modules
COPY cmd/trainers/grpo/requirements.txt .

# Install the needed packages
RUN pip install -r requirements.txt

# Copy the GRPO training entrypoint outside /workspace to avoid PVC mount shadowing
COPY cmd/trainers/grpo/train.py .
