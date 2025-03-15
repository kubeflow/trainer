FROM python:3.11-alpine

WORKDIR /workspace

# Copy the required Python modules.
COPY cmd/initializers/model/requirements.txt .
COPY pkg/initializers pkg/initializers

# Install the needed packages.
RUN pip install -r requirements.txt

ENTRYPOINT ["python", "-m", "pkg.initializers.model"]
