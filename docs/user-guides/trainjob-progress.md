# TrainJob Progress and Training Metrics

How to surface real-time training progress, ETA, and custom metrics directly in TrainJob status using the Kubeflow SDK or HuggingFace Transformers.

The TrainJob progress and training metrics feature allows training scripts to push structured
progress data (completion percentage, estimated time remaining, and custom key-value metric
pairs) directly into `TrainJob.status.trainerStatus` in real time. This eliminates the need
to manually parse or scrape container logs to monitor training health and performance.

:::{important}
The TrainJob progress feature was introduced in Kubeflow Trainer v2.2.0 and requires the `TrainJobStatus` alpha
feature gate to be enabled on the controller.
:::

:::{note}
Make sure to follow the [Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.
:::

## Prerequisites

- Kubeflow Trainer v2.2.0 or later installed on your cluster.
- The `TrainJobStatus` alpha feature gate enabled on the Trainer controller. To enable
  it, pass the flag to the controller at startup:

  ```bash
  --feature-gates=TrainJobStatus=true
  ```

  Or, if deploying via Helm, set `manager.config.featureGates.TrainJobStatus=true`.

  The command-line flag takes precedence over any value set in the controller config file.

- For SDK-based progress reporting: Kubeflow SDK `>= 0.5.0`.

## View TrainJob progress and metrics

Once progress reporting is active, inspect `status.trainerStatus` using standard tooling.

### Using kubectl

```bash
kubectl get trainjob <trainjob-name> -o jsonpath='{.status.trainerStatus}'
```

To view the full status including conditions alongside trainer status:

```bash
kubectl describe trainjob <trainjob-name>
```

Example output showing `status.trainerStatus`:

```text
Status:
  Trainer Status:
    Progress Percentage:          45
    Estimated Remaining Seconds:  3600
    Metrics:
      Name:   loss
      Value:  0.2347
      Name:   accuracy
      Value:  0.9876
    Last Updated Time:  2026-06-12T03:00:00Z
```

:::{note}
The field names match the Go struct fields in
[`TrainerStatus`](https://pkg.go.dev/github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1#TrainerStatus).
:::

### Using the Kubeflow SDK

```python
from kubeflow.trainer import TrainerClient

client = TrainerClient()

status = client.get_job("my-trainjob")

print(f"Progress:  {status.trainer_status.progress_percentage}%")
print(f"ETA:       {status.trainer_status.estimated_remaining_seconds}s")
print(f"Metrics:   {status.trainer_status.metrics}")
```

## Report progress from your training script

### Option A: HuggingFace Transformers (zero code change)

If you're using the HuggingFace Transformers API, you don't need to make any code changes. The integration is done automatically, and the trainer progress and metrics will automatically get written to the `TrainJob.status`.

**Requirements:** You need to use `transformers >= 4.57.0`.

It's implemented using a `KubeflowTrainerCallback` that's automatically registered when transformers detects it's running using Kubeflow Trainer.

You can add additional metrics using the `Trainer` `compute_metrics` argument. See the [transformers docs](https://huggingface.co/docs/transformers/main_classes/trainer#transformers.Trainer.compute_metrics) for more info.

```python
from transformers import Trainer, TrainingArguments
import evaluate
import numpy as np

# Load a metric
metric = evaluate.load("accuracy")

def compute_metrics(eval_pred):
    logits, labels = eval_pred
    predictions = np.argmax(logits, axis=-1)
    return metric.compute(predictions=predictions, references=labels)

# model and train_dataset initialization omitted for brevity
training_args = TrainingArguments(
    output_dir="./results",
    num_train_epochs=3,
    logging_steps=10,
    eval_strategy="epoch", # compute_metrics is run during evaluation
)

trainer = Trainer(
    model=model,
    args=training_args,
    train_dataset=train_dataset,
    eval_dataset=eval_dataset,
    compute_metrics=compute_metrics,
)

# When running inside a Kubeflow TrainJob with TrainJobStatus enabled,
# KubeflowTrainerCallback is auto-registered and reports the following
# at each logging step: loss, learning_rate, epoch, completion percentage.
# Any custom metrics returned by compute_metrics (like 'accuracy') are also automatically reported.
trainer.train()
```

### Option B: Kubeflow SDK (custom training loop)

Use `update_trainjob_status` from `kubeflow.trainer.utils` when writing a custom training
loop or using a framework other than HuggingFace Transformers. The SDK throttles updates
to at most once every 5 seconds to avoid overloading the controller. Use `force=True` to
bypass the throttle — recommended at the very start and end of training so those
transitions are always captured.

For detailed API usage of `update_trainjob_status`, refer to the [Kubeflow SDK documentation](https://www.kubeflow.org/docs/components/trainer/sdk/).

```python
from kubeflow.trainer.utils import update_trainjob_status

total_epochs = 10

# Signal the start of training (force=True bypasses the 5-second throttle)
update_trainjob_status(progress_percent=0, force=True)

for epoch in range(total_epochs):
    # --- your training logic here ---

    # Replace these with your actual computed values
    progress    = int((epoch + 1) / total_epochs * 100)
    eta_seconds = compute_eta(epoch, total_epochs)  # your own ETA helper
    metrics     = {
        "loss":     train_loss,   # replace with actual metric values
        "accuracy": train_acc,    # replace with actual metric values
    }

    # The SDK handles throttling automatically — safe to call at every step.
    update_trainjob_status(
        progress_percent=progress,
        estimated_remaining_seconds=eta_seconds,
        metrics=metrics,
    )

# Signal completion (force=True ensures the final state is never throttled)
update_trainjob_status(progress_percent=100, force=True)
```

### Option C: Raw HTTP (any language or framework)

If you are not using Python, or want to integrate progress reporting into a framework
without an existing callback, read the injected environment variables and POST directly
to the controller endpoint.

#### Technical Details

- **Environment Variables**: The controller injects `KUBEFLOW_TRAINER_SERVER_URL` (the HTTPS endpoint), `KUBEFLOW_TRAINER_SERVER_CA_CERT` (path to the CA cert file), and `KUBEFLOW_TRAINER_SERVER_TOKEN` (path to the bearer token file).
- **Authentication**: Each request is authenticated using a projected service account token (OIDC-verified by the controller). The projected service account token is issued with a TrainJob-specific audience so the controller can verify that update requests target the correct TrainJob.
- **TLS Configuration**: The endpoint reuses the same webhook TLS certificates as the controller, with automatic cert rotation.

#### Implementation Guidance

When building a raw HTTP client:
- **Client-side rate limiting**: Ensure your client throttles requests (e.g. at most once every 5 seconds) to avoid overloading the Kubernetes API server and controller.
- **Token rotation**: The bearer token injected into the container is a JWT that periodically rotates. You should check the expiry of the JWT or ensure you re-read the token from the filesystem file (`KUBEFLOW_TRAINER_SERVER_TOKEN`) rather than caching it indefinitely.
- **Error handling**: Ensure there's sufficient error handling (e.g., catching timeouts and transient network errors) to avoid breaking or impacting your main training loops.

```python
import os
from datetime import datetime, timezone
import requests
import time

server_url = os.environ.get("KUBEFLOW_TRAINER_SERVER_URL")
ca_cert_path = os.environ.get("KUBEFLOW_TRAINER_SERVER_CA_CERT")
token_path = os.environ.get("KUBEFLOW_TRAINER_SERVER_TOKEN")

# Only attempt reporting when running inside a Kubeflow TrainJob
if server_url and ca_cert_path and token_path:
    # Read the token. In production, re-read this file periodically as the JWT token rotates
    with open(token_path, "r") as f:
        token = f.read().strip()

    headers = {"Authorization": f"Bearer {token}"}
    payload = {
        "trainerStatus": {
            "progressPercentage": 45,  # replace with actual value (0–100)
            "estimatedRemainingSeconds": 120,  # replace with actual seconds remaining
            "metrics": [
                {"name": "loss", "value": "0.15"},
                {"name": "accuracy", "value": "0.92"},
            ],
            "lastUpdatedTime": datetime.now(timezone.utc).isoformat(),
        }
    }

    try:
        response = requests.post(
            server_url,
            headers=headers,
            json=payload,
            verify=ca_cert_path,
            timeout=5,
        )
        response.raise_for_status()
    except Exception as e:
        # Log but never raise reporting must not interrupt training
        print(f"Failed to update TrainJob progress: {e}")
```

The server returns HTTP 200 with the parsed payload on success, and a
`metav1.Status`-style JSON object on error, consistent with the Kubernetes API server
conventions.

## Future plans

The following capabilities are planned as follow-ons to this feature, as described in the
[TrainJob progress proposal](https://github.com/kubeflow/trainer/tree/master/docs/proposals/2779-trainjob-progress):

- Periodic, transparent checkpointing triggered automatically based on ETA.
- Integration with `OptimizationJob` for hyperparameter tuning jobs (Katib).

## Next steps

- [Getting Started with Kubeflow Trainer](../getting-started/index)
- [Configure TrainJob Lifecycle](trainjob-lifecycle.md)
- [TrainJob Progress feature proposal](https://github.com/kubeflow/trainer/tree/master/docs/proposals/2779-trainjob-progress)
- [Kubeflow Trainer v2.2 release blog post](https://blog.kubeflow.org/kubeflow-trainer-v2.2-release/)
