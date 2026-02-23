"""
progress.py

E2E test script for TrainJob progress tracking.

This script validates that the progress tracking endpoint can be called from within
a training container. It reads the progress server URL, CA certificate, and service
account token from environment variables injected by the Progress plugin, then sends
a single progress update with test metrics to verify the TrainJob status is updated.

Environment variables required:
- KUBEFLOW_TRAINER_STATUS_URL: HTTPS URL for the progress status endpoint
- KUBEFLOW_TRAINER_STATUS_CA_CERT: Path to CA certificate file for TLS verification
- KUBEFLOW_TRAINER_STATUS_TOKEN: Path to service account token file for authentication
"""

import json
import os
import ssl
from datetime import datetime, timezone
from urllib import error, request

url = os.environ["KUBEFLOW_TRAINER_STATUS_URL"]
ca_file = os.environ["KUBEFLOW_TRAINER_STATUS_CA_CERT"]
token = open(os.environ["KUBEFLOW_TRAINER_STATUS_TOKEN"]).read()
ssl_context = ssl.create_default_context(cafile=ca_file)

# Send a single progress update
payload = {
    "trainerStatus": {
        "progressPercentage": 42,
        "estimatedRemainingSeconds": 120,
        "metrics": [
            {"name": "loss", "value": "0.123"},
            {"name": "accuracy", "value": "0.95"},
        ],
        "lastUpdatedTime": datetime.now(timezone.utc).isoformat(),
    }
}
data = json.dumps(payload)
req = request.Request(
    url, data=data.encode("utf-8"), headers={"Authorization": f"Bearer {token}"}
)

try:
    resp = request.urlopen(req, context=ssl_context)
except error.HTTPError as ex:
    body = ex.read().decode("utf-8", errors="replace")
    print(f"Failed to update trainer status. {ex} {body}")
    raise
else:
    body = resp.read().decode("utf-8")
    print(f"Success updating trainer status: {resp.getcode()} {body}")
