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
# Ubuntu 26.04 is the first Ubuntu LTS that packages Open MPI 5 (5.0.10), 22.04 ships 4.1.2.
FROM nvidia/cuda:13.3.1-devel-ubuntu26.04

# Ubuntu 26.04 marks the system Python as externally managed (PEP 668).
# Dependencies are installed into the mpiuser site directory, as on previous releases.
ENV PIP_BREAK_SYSTEM_PACKAGES=1

# Debian ships the PRRTE help files, which hold the text of the messages mpirun prints, as
# libprrte-dev documentation, and Ubuntu images do not install documentation. Keep them.
RUN echo "path-include=/usr/share/doc/libprrte-dev/*" > /etc/dpkg/dpkg.cfg.d/prrte-help-files

# Install libraries required for OpenMPI to work. Ubuntu 26.04 packages Open MPI 5.0.10.
RUN apt update && apt install -y --no-install-recommends \
    openssh-server openssh-client \
    g++ libopenmpi-dev \
    python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python && rm -rf /var/lib/apt/lists/*

# Install the PRRTE help files where PRRTE looks for them.
RUN prte_share="$(prte_info --path pkgdatadir | awk '{print $NF}')" \
    && mkdir -p "${prte_share}" \
    && cp /usr/share/doc/libprrte-dev/help-*.txt* "${prte_share}" \
    && find "${prte_share}" -name 'help-*.txt.gz' -exec gunzip -f {} +

# Configure mpiuser and home directory.
# Ubuntu images ship an "ubuntu" user with UID 1000 since 24.04. The runtimes run as
# UID 1000, so remove it to let mpiuser take that UID.
RUN userdel -r ubuntu && useradd -m mpiuser
WORKDIR /home/mpiuser

# Copy SSH configurations from the MPI image.
COPY --from=mpi /etc/ssh/ssh_config /etc/ssh/ssh_config
COPY --from=mpi /etc/ssh/sshd_config /etc/ssh/sshd_config
COPY --from=mpi /home/mpiuser/.sshd_config /home/mpiuser/.sshd_config

# Set home directory for mpiuser.
ENV HOME=/home/mpiuser
ENV PATH=$HOME/.local/bin:$PATH

COPY cmd/runtimes/deepspeed/requirements.txt .
# Ubuntu 26.04 pip is a Debian package without a RECORD file, so pip cannot uninstall it.
RUN pip install --upgrade --ignore-installed pip
RUN pip install --user -r requirements.txt

# Give mpiuser permission to download packages and HF models.
# .cache directory is used by ML frameworks to download models.
RUN chown -R mpiuser:mpiuser /home/mpiuser/.local
RUN mkdir -p /home/mpiuser/.cache && chown -R mpiuser:mpiuser /home/mpiuser/.cache
