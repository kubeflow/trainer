FROM mpioperator/base:v0.6.0 AS mpi
FROM quay.io/slopezpa/fedora-vgpu

# Install libraries required for OpenMPI and MLX.
RUN dnf install -y cmake gcc gcc-c++ clang \
    wget git \
    openblas-devel lapack-devel \
    openssh-server openssh-clients libcap

# Add capability to run sshd as non-root.
RUN setcap CAP_NET_BIND_SERVICE=+eip /usr/sbin/sshd

ARG PORT=2222

# Configure mpiuser and home directory.
RUN useradd -m mpiuser
WORKDIR /home/mpiuser

# Configurations for running sshd as non-root.
COPY --from=mpi /home/mpiuser/.sshd_config /home/mpiuser/.sshd_config
RUN echo "Port ${PORT}" >>/home/mpiuser/.sshd_config

# Install OpenMPI 5.0.7 version. MLX requires OpenMPI >= 5.0 version.
ARG MPI_VERSION=5.0.7
RUN wget https://download.open-mpi.org/release/open-mpi/v5.0/openmpi-${MPI_VERSION}.tar.gz
RUN tar -xvf openmpi-${MPI_VERSION}.tar.gz && rm -rf openmpi-${MPI_VERSION}.tar.gz
RUN cd openmpi-${MPI_VERSION} && ./configure --prefix=/usr/local && make install
RUN rm -rf openmpi-${MPI_VERSION}

ENV LD_LIBRARY_PATH=/usr/local/lib

# Install the required Python packages. This image has Python 3.12
RUN dnf install -y python3.12-devel pip && ln -s /usr/bin/python3.12 /usr/bin/python

# We have to build MLX and MLX Data from source.
RUN git clone https://github.com/ml-explore/mlx.git
RUN cd mlx && git checkout f018e248cd75dbb65668f418d6afb67842ea28b7 && CMAKE_BUILD_PARALLEL_LEVEL=8 pip install -v .

# RUN git clone https://github.com/ml-explore/mlx-data.git
# RUN cd mlx-data && pip install -v .
RUN git clone -b fix-load-mnist https://github.com/andreyvelich/mlx-data.git && cd mlx-data && CMAKE_BUILD_PARALLEL_LEVEL=8 pip install -v .
