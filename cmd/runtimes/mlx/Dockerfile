FROM mpioperator/base:v0.6.0 AS mpi
FROM debian:trixie

# Install libraries required for OpenMPI and MLX. This image installs OpenMPI 5.0.7
RUN apt update && apt install -y --no-install-recommends \
    openssh-server openssh-client libcap2-bin \
    libopenmpi-dev \
    git g++ libblas-dev liblapack-dev liblapacke-dev

# Add capability to run sshd as non-root.
RUN setcap CAP_NET_BIND_SERVICE=+eip /usr/sbin/sshd

# Configure mpiuser and home directory.
RUN useradd -m mpiuser
WORKDIR /home/mpiuser

# Copy SSH configurations from the MPI image.
COPY --from=mpi /etc/ssh/ssh_config /etc/ssh/ssh_config
COPY --from=mpi /etc/ssh/sshd_config /etc/ssh/sshd_config
COPY --from=mpi /home/mpiuser/.sshd_config /home/mpiuser/.sshd_config

# Install the required Python packages. This image has Python 3.13
RUN apt update && apt install -y python3 python3-pip && ln -s /usr/bin/python3 /usr/bin/python && apt clean

# We have to build MLX and MLX Data from source.
RUN git clone https://github.com/ml-explore/mlx.git
RUN cd mlx && git checkout f018e248cd75dbb65668f418d6afb67842ea28b7 && CMAKE_BUILD_PARALLEL_LEVEL=8 pip install -v --break-system-packages .

RUN git clone https://github.com/ml-explore/mlx-data.git
RUN cd mlx-data && git checkout 79516daa75aa3e9fd72fc5e3fb5e9e629912feac && CMAKE_BUILD_PARALLEL_LEVEL=8 pip install -v --break-system-packages .
