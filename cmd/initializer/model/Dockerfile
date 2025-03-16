FROM python:3.11-slim-bookworm

WORKDIR /workspace

# Copy the required Python modules.
COPY cmd/initializer/model/requirements.txt .
COPY pkg/initializer pkg/initializer

# Install the needed packages.
RUN pip install -r requirements.txt

# Git is needed for the Kubeflow SDK to download JobSet Python models.
RUN apt update && apt install -y git

# Copy and install the Kubeflow SDK for the configs.
COPY sdk sdk
COPY LICENSE LICENSE
COPY README.md README.md
RUN pip install ./sdk

ENTRYPOINT ["python", "-m", "pkg.initializer.model"]
