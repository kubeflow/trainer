FROM mpioperator/base:v0.6.0 AS mpi
FROM nvidia/cuda:12.4.1-devel-ubuntu22.04

# Disable interactive dialog from apt.
ENV DEBIAN_FRONTEND noninteractive

# Install libraries required for OpenMPI to work.
RUN apt-get update && apt install -y --no-install-recommends \
    cmake g++ gcc \
    wget vim \
    openssh-client openssh-server libcap2-bin \
    libopenmpi-dev openmpi-bin

# Add capability to run sshd as non-root.
RUN setcap CAP_NET_BIND_SERVICE=+eip /usr/sbin/sshd
RUN apt remove libcap2-bin -y

# Configure mpiuser and home directory.
RUN useradd -m mpiuser
WORKDIR /home/mpiuser

# Copy SSH configurations from the MPI image.
COPY --from=mpi /etc/ssh/ssh_config /etc/ssh/ssh_config
COPY --from=mpi /etc/ssh/sshd_config /etc/ssh/sshd_config
COPY --from=mpi /home/mpiuser/.sshd_config /home/mpiuser/.sshd_config

# Install the required Python packages.
RUN apt install -y python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python

ENV LD_LIBRARY_PATH=/usr/local/lib:/usr/local/mpi/lib:/usr/local/mpi/lib64:${LD_LIBRARY_PATH}
ENV HOME=/home/mpiuser
ENV PATH=$HOME/.local/bin:$PATH

COPY cmd/runtimes/deepspeed/requirements.txt .
RUN pip install --user -r requirements.txt

# Give mpiuser permission to download packages and HF models.
# .cache directory is used by ML frameworks to download models.
RUN chown -R mpiuser:mpiuser /home/mpiuser/.local
RUN mkdir -p /home/mpiuser/.cache && chown -R mpiuser:mpiuser /home/mpiuser/.cache
