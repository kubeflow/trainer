# Use the same multi-stage build pattern as the other MPI-based runtimes
FROM mpioperator/base:v0.6.0 AS mpi
FROM nvidia/cuda:12.8.1-devel-ubuntu22.04

# Install libraries required for OpenMPI and SSH
RUN apt update && apt install -y --no-install-recommends \
    openssh-server openssh-client libcap2-bin \
    g++ libopenmpi-dev \
    python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python && rm -rf /var/lib/apt/lists/*

# Add capability to run sshd as non-root
RUN setcap CAP_NET_BIND_SERVICE=+eip /usr/sbin/sshd
RUN apt remove libcap2-bin -y

# Configure the non-root mpiuser
RUN useradd -ms /bin/bash -u 1000 mpiuser
WORKDIR /home/mpiuser

# Copy SSH configurations from the base MPI image
COPY --from=mpi /etc/ssh/ssh_config /etc/ssh/ssh_config
COPY --from=mpi /etc/ssh/sshd_config /etc/ssh/sshd_config
COPY --from=mpi /home/mpiuser/.sshd_config /home/mpiuser/.sshd_config

# Set home directory for mpiuser
ENV HOME=/home/mpiuser
ENV PATH=$HOME/.local/bin:$PATH

# Copy and install Python requirements for XGBoost
COPY cmd/runtimes/xgboost/requirements.txt .
RUN pip install --user -r requirements.txt

# Create and set permissions for the training script directory
RUN mkdir -p /app && chown -R mpiuser:mpiuser /app
# Set permissions for package and model caches
RUN chown -R mpiuser:mpiuser /home/mpiuser/.local
RUN mkdir -p /home/mpiuser/.cache && chown -R mpiuser:mpiuser /home/mpiuser/.cache
