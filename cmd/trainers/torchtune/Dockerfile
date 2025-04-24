FROM pytorch/pytorch:2.5.0-cuda12.4-cudnn9-runtime

WORKDIR /workspace

# Copy the required Python modules
COPY cmd/trainers/torchtune/requirements.txt .

# Install the needed packages
RUN pip install -r requirements.txt
