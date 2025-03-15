FROM python:3.11-alpine

WORKDIR /workspace

# Copy the required Python modules.
COPY cmd/initializers/dataset/requirements.txt .
COPY pkg/initializers pkg/initializers

# Install the needed packages.
RUN pip install -r requirements.txt

# Git is needed for the Kubeflow SDK to download JobSet Python models.
RUN apk update && apk add --no-cache git

ENTRYPOINT ["python", "-m", "pkg.initializers.dataset"]
