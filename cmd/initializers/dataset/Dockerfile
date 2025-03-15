FROM python:3.11-alpine

WORKDIR /workspace

# Copy the required Python modules.
COPY cmd/initializers/dataset/requirements.txt .
COPY pkg/initializers pkg/initializers

# Install the needed packages.
RUN pip install -r requirements.txt

# Git is needed for the Kubeflow SDK to download JobSet Python models.
RUN apk update && apk add --no-cache git

# Copy and install the Kubeflow SDK for the configs.
COPY sdk sdk
COPY LICENSE LICENSE
COPY README.md README.md
RUN pip install ./sdk

ENTRYPOINT ["python", "-m", "pkg.initializers.dataset"]
