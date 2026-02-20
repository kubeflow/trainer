FROM nvidia/cuda:12.4.0-runtime-ubuntu22.04

RUN apt update && apt install -y --no-install-recommends \
    python3-dev pip && rm -f /usr/bin/python && ln -s /usr/bin/python3 /usr/bin/python && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY cmd/runtimes/xgboost/requirements.txt .
RUN pip install --upgrade pip
RUN pip install -r requirements.txt

# .cache directory
RUN mkdir -p /root/.cache
