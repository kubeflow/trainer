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

FROM mpioperator/base:v0.8.2 AS mpi

# Ubuntu 22.04 packages Open MPI 4.1.2, so build Open MPI 5 from source. The image stays on
# Ubuntu 22.04 because mlx-data does not publish wheels for Python 3.14, the Python of
# Ubuntu 26.04, the first Ubuntu LTS that packages Open MPI 5.
FROM nvidia/cuda:13.3.1-devel-ubuntu22.04 AS openmpi
ARG OPENMPI_VERSION=5.0.10
ARG OPENMPI_SHA256=0acecc4fc218e5debdbcb8a41d182c6b0f1d29393015ed763b2a91d5d7374cc6
RUN apt update && apt install -y --no-install-recommends \
    ca-certificates curl g++ make perl bzip2 zlib1g-dev && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL -o /tmp/openmpi.tar.bz2 \
        https://download.open-mpi.org/release/open-mpi/v${OPENMPI_VERSION%.*}/openmpi-${OPENMPI_VERSION}.tar.bz2 \
    && echo "${OPENMPI_SHA256}  /tmp/openmpi.tar.bz2" | sha256sum -c - \
    && tar -xjf /tmp/openmpi.tar.bz2 -C /tmp \
    && cd /tmp/openmpi-${OPENMPI_VERSION} \
    && ./configure --prefix=/usr/local \
    && make -j"$(nproc)" \
    && make install DESTDIR=/openmpi

FROM nvidia/cuda:13.3.1-devel-ubuntu22.04

# Install libraries required for OpenMPI to work.
RUN apt update && apt install -y --no-install-recommends \
    openssh-server openssh-client \
    g++ libblas-dev liblapack-dev liblapacke-dev \
    python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python && rm -rf /var/lib/apt/lists/*

# Install Open MPI 5 under /usr/local, which is on the PATH of the SSH sessions that
# mpirun uses to start its daemons on the worker nodes.
COPY --from=openmpi /openmpi/usr/local /usr/local
RUN ldconfig

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

COPY cmd/runtimes/mlx/requirements.txt .
RUN pip install --upgrade pip
RUN pip install --user -r requirements.txt

# Give mpiuser permission to download packages and HF models.
# .cache directory is used by ML frameworks to download models.
RUN chown -R mpiuser:mpiuser /home/mpiuser/.local
RUN mkdir -p /home/mpiuser/.cache && chown -R mpiuser:mpiuser /home/mpiuser/.cache
