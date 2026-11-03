# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
optimizationjob_trial.py

E2E trial script for OptimizationJob that trains a model with the hyperparameters suggested
by the search algorithm and reports the objective metric using the status server endpoint.

The workload mirrors examples/optimizationjob/random-search.yaml: it fits y = 2x + 1 with
gradient descent on fixed training data and reports the mean squared error on separate
validation data as validation_loss. The status update is sent with the standard library
instead of the Kubeflow SDK, so the trial does not need to install packages.

The e2e test recomputes validation_loss from the trial parameters, so the training loop
must stay in sync with linearRegressionValidationLoss in e2e_test.go.

Environment variables required:
- KUBEFLOW_TRAINER_OPT_learning_rate: Learning rate suggested by the search algorithm
- KUBEFLOW_TRAINER_OPT_epochs: Number of epochs suggested by the search algorithm
- KUBEFLOW_TRAINER_SERVER_URL: HTTPS URL for the status server endpoint
- KUBEFLOW_TRAINER_SERVER_CA_CERT: Path to CA certificate file for TLS verification
- KUBEFLOW_TRAINER_SERVER_TOKEN: Path to service account token file for authentication
"""

import json
import os
import ssl
from datetime import datetime, timezone
from urllib import error, request

learning_rate = float(os.environ["KUBEFLOW_TRAINER_OPT_learning_rate"])
epochs = int(os.environ["KUBEFLOW_TRAINER_OPT_epochs"])

# Fit y = 2x + 1 with gradient descent on fixed training data.
train_x = [-1.0, -0.5, 0.0, 0.5, 1.0]
weight, bias = 0.0, 0.0
for _ in range(epochs):
    errors = [weight * x + bias - (2 * x + 1) for x in train_x]
    weight_grad = sum(error * x for error, x in zip(errors, train_x))
    weight -= learning_rate * 2 * weight_grad / len(train_x)
    bias -= learning_rate * 2 * sum(errors) / len(train_x)

validation_x = [-0.75, -0.25, 0.25, 0.75]
squared_errors = [(weight * x + bias - (2 * x + 1)) ** 2 for x in validation_x]
loss = sum(squared_errors) / len(squared_errors)
print(f"learning_rate={learning_rate} epochs={epochs} validation_loss={loss}")

url = os.environ["KUBEFLOW_TRAINER_SERVER_URL"]
ca_file = os.environ["KUBEFLOW_TRAINER_SERVER_CA_CERT"]
token = open(os.environ["KUBEFLOW_TRAINER_SERVER_TOKEN"]).read().strip()
ssl_context = ssl.create_default_context(cafile=ca_file)

# Report the objective metric before exiting.
payload = {
    "trainerStatus": {
        "progressPercentage": 100,
        "estimatedRemainingSeconds": 0,
        "metrics": [{"name": "validation_loss", "value": str(loss)}],
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
    print(f"Failed to report validation_loss to TrainJob status server. {ex} {body}")
    raise
else:
    body = resp.read().decode("utf-8")
    print(
        f"Success reporting validation_loss to TrainJob status server. {resp.getcode()} {body}"
    )
