FROM python:3.12

RUN apt-get update && apt install -y --no-install-recommends \
    cmake \ 
    build-essential \
    git \
    libstdc++6

RUN git clone https://github.com/pytorch/gloo.git
RUN cd gloo \
    && mkdir build \
    && cd build \
    && cmake ../ \
    && make \
    && make install

RUN ls -lha /usr/local/lib | grep gloo
RUN ls -lha /usr/local/include/gloo

RUN cd gloo && git rev-parse HEAD

WORKDIR /home/jax-runtime

# TODO(mahdi): it should be 'COPY cmd/runtimes/jax/requirements.txt .' when finalized
COPY requirements.txt .
RUN pip install -r requirements.txt

CMD ["sh", "-c", "ls -lha /usr/local/lib | grep gloo && pip list | grep jax"]
