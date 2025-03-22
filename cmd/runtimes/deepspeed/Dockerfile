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

ARG PORT=2222

# Disable StrictHostKeyChecking to Allow OpenSSH to talk to containers without asking for it.
# TrainJob controller mounts the .ssh folder from a Secret.
# Disable UserKnownHostsFile to avoid write permissions on .ssh folder.
# Disabling StrictModes avoids directory and files read permission checks.
RUN sed -i "s/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g" /etc/ssh/ssh_config
RUN echo "    UserKnownHostsFile /dev/null" >>/etc/ssh/ssh_config
RUN sed -i "s/[ #]\(.*Port \).*/ \1${PORT}/g" /etc/ssh/ssh_config
RUN sed -i "s/#\(StrictModes \).*/\1no/g" /etc/ssh/sshd_config
RUN sed -i "s/#\(Port \).*/\1${PORT}/g" /etc/ssh/sshd_config

# Configure mpiuser and home directory.
RUN useradd -m mpiuser
WORKDIR /home/mpiuser

# Configurations for running sshd as non-root.
COPY --chown=mpiuser cmd/runtimes/deepspeed/sshd_config .sshd_config
RUN echo "Port ${PORT}" >>/home/mpiuser/.sshd_config

ENV LD_LIBRARY_PATH=/usr/local/lib:/usr/local/mpi/lib:/usr/local/mpi/lib64:${LD_LIBRARY_PATH}

# Install the required Python packages.
RUN apt install -y python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python
COPY cmd/runtimes/deepspeed/requirements.txt .
RUN pip install -r requirements.txt
