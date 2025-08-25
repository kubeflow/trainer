FROM mpioperator/base:v0.6.0 AS mpi
FROM nvidia/cuda:12.8.1-devel-ubuntu22.04

# Install libraries required for OpenMPI to work. Image installs OpenMPI 5.0.7
RUN apt update && apt install -y --no-install-recommends \
    openssh-server openssh-client libcap2-bin \
    g++ libopenmpi-dev libblas-dev liblapack-dev liblapacke-dev \
    python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python && rm -rf /var/lib/apt/lists/*

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

# Set home directory for mpiuser.
ENV HOME=/home/mpiuser
ENV PATH=$HOME/.local/bin:$PATH

COPY cmd/runtimes/mlx/requirements.txt .
RUN pip install --user -r requirements.txt

# Give mpiuser permission to download packages and HF models.
# .cache directory is used by ML frameworks to download models.
RUN chown -R mpiuser:mpiuser /home/mpiuser/.local
RUN mkdir -p /home/mpiuser/.cache && chown -R mpiuser:mpiuser /home/mpiuser/.cache
