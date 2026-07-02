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

FROM mpioperator/base:v0.8.0 AS mpi
FROM nvidia/cuda:13.3.0-devel-ubuntu22.04

# Install libraries required for OpenMPI to work. Image installs OpenMPI 5.0.7
RUN apt update && apt install -y --no-install-recommends \
    openssh-server openssh-client \
    g++ libopenmpi-dev \
    python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python && rm -rf /var/lib/apt/lists/*

# Configure mpiuser and home directory.
RUN useradd -m mpiuser
WORKDIR /home/mpiuser

# Copy SSH configurations from the MPI image.
COPY --from=mpi /etc/ssh/ssh_config /etc/ssh/ssh_config
COPY --from=mpi /etc/ssh/sshd_config /etc/ssh/sshd_config
COPY --from=mpi /home/mpiuser/.sshd_config /home/mpiuser/.sshd_config

# Set home directory for mpiuser.
ENV HOME=/home/mpiuser
ENV PATH=$HOME/.local/bin:$PATH

COPY cmd/runtimes/deepspeed/requirements.txt .
RUN pip install --upgrade pip
RUN python -m pip install --retries 5 --timeout 120 --user -r requirements.txt

# Give mpiuser permission to download packages and HF models.
# .cache directory is used by ML frameworks to download models.
RUN chown -R mpiuser:mpiuser /home/mpiuser/.local
RUN mkdir -p /home/mpiuser/.cache && chown -R mpiuser:mpiuser /home/mpiuser/.cache
