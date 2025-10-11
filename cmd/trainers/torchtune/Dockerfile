FROM pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime

WORKDIR /workspace

# Install build-essential for compiling any dependencies
RUN apt update && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy the required Python modules
COPY cmd/trainers/torchtune/requirements.txt .

# Install the needed packages
RUN pip install -r requirements.txt
