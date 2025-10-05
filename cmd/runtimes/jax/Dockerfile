FROM ghcr.io/nvidia/jax:jax as gpu-base
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        cmake \
        git \
        curl \
        libgoogle-glog-dev \
        libgflags-dev \
        libprotobuf-dev \
        protobuf-compiler \
        python3-dev \
        python3-pip \
        python3-setuptools && \
    rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir \
        numpy \
        jax \
        jaxlib \
        "jax[cuda12_pip]" -f https://storage.googleapis.com/jax-releases/jax_cuda_releases.html

FROM gpu-base as tpu-base

RUN pip install --no-cache-dir "jax[tpu]" -f https://storage.googleapis.com/jax-releases/libtpu_releases.html || \
    echo "TPU support not available" && \
    pip install --no-cache-dir libtpu-nightly || \
    echo "libtpu-nightly not available"

FROM tpu-base as gloo-base

RUN git clone https://github.com/facebookincubator/gloo.git \
    && cd gloo \
    && git checkout 43b7acbf372cdce14075f3526e39153b7e433b53 \
    && mkdir build \
    && cd build \
    && cmake ../ \
    && make \
    && make install

RUN pip install --no-cache-dir absl-py kubernetes

FROM gloo-base as production

WORKDIR /workspace

COPY version.sh /version.sh
RUN chmod +x /version.sh

ENTRYPOINT ["/bin/bash", "-c", "/version.sh && exec \"$@\"", "--"]
CMD ["/bin/bash"]
