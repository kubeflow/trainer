# XGBoost Guide

This guide describes how to use TrainJob to run distributed
[XGBoost](https://xgboost.readthedocs.io/) training on Kubernetes.

## Prerequisites

Before exploring this guide, make sure to follow
[the Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.

## XGBoost Overview

XGBoost supports distributed training through the
[Collective](https://xgboost.readthedocs.io/en/latest/tutorials/kubernetes.html)
communication protocol (historically known as Rabit). In a distributed setting,
multiple worker processes each operate on a shard of the data and synchronize
histogram bin statistics via AllReduce to agree on the best tree splits.

Kubeflow Trainer integrates with XGBoost by:

- Deploying worker pods as a [JobSet](https://github.com/kubernetes-sigs/jobset).
- Automatically injecting the `DMLC_*` environment variables required by XGBoost's
 Collective communication layer (`DMLC_TRACKER_URI`, `DMLC_TRACKER_PORT`,
 `DMLC_TASK_ID`, `DMLC_NUM_WORKER`).
- Providing the rank-0 pod with the tracker address so user code can start a
 `RabitTracker` for worker coordination.
- Supporting both CPU and GPU training workloads.

The built-in runtime is called `xgboost-distributed` and uses the container image
`ghcr.io/kubeflow/trainer/xgboost-runtime:latest`, which includes XGBoost with
CUDA 12 support, NumPy, and scikit-learn.

### Worker Count

The total number of XGBoost workers is calculated as:

```text
DMLC_NUM_WORKER = numNodes × workersPerNode
```

- **CPU training**: 1 worker per node. Each worker uses OpenMP to parallelize
 across all available CPU cores.
- **GPU training**: 1 worker per GPU. The GPU count is derived from
 `resourcesPerNode` limits in the TrainJob.

## Distributed Training Function

Your training function runs on every XGBoost worker. The Kubeflow XGBoost runtime
injects the `DMLC_*` environment variables automatically.

```python
from kubeflow.trainer import TrainerClient, CustomTrainer


def xgboost_train_classification():
    import os

    import xgboost as xgb
    from sklearn.datasets import make_classification
    from sklearn.metrics import accuracy_score
    from sklearn.model_selection import train_test_split
    from xgboost import collective as coll
    from xgboost.tracker import RabitTracker

    rank = int(os.environ["DMLC_TASK_ID"])
    world_size = int(os.environ["DMLC_NUM_WORKER"])
    tracker_uri = os.environ["DMLC_TRACKER_URI"]
    tracker_port = int(os.environ["DMLC_TRACKER_PORT"])

    tracker = None
    if rank == 0:
        tracker = RabitTracker(host_ip="0.0.0.0", n_workers=world_size, port=tracker_port)
        tracker.start()

    with coll.CommunicatorContext(
        dmlc_tracker_uri=tracker_uri,
        dmlc_tracker_port=tracker_port,
        dmlc_task_id=str(rank),
    ):
        X, y = make_classification(
            n_samples=10000,
            n_features=20,
            n_informative=10,
            n_classes=2,
            random_state=42 + rank,
        )
        X_train, X_valid, y_train, y_valid = train_test_split(
            X, y, test_size=0.2, random_state=42
        )

        dtrain = xgb.QuantileDMatrix(X_train, label=y_train)
        dvalid = xgb.QuantileDMatrix(X_valid, label=y_valid, ref=dtrain)

        params = {
            "objective": "binary:logistic",
            "tree_method": "hist",
            "max_depth": 6,
            "eta": 0.1,
            "eval_metric": "logloss",
        }

        model = xgb.train(
            params,
            dtrain,
            num_boost_round=50,
            evals=[(dvalid, "validation")],
            verbose_eval=10,
        )

        preds = model.predict(dvalid)
        predictions = [1 if p > 0.5 else 0 for p in preds]
        accuracy = accuracy_score(y_valid, predictions)
        print(f"Worker {rank} - Validation Accuracy: {accuracy:.4f}")

        if coll.get_rank() == 0:
            model.save_model("/workspace/xgboost_model.json")

    if tracker is not None:
        tracker.wait_for()


client = TrainerClient()
job_id = client.train(
    runtime=client.get_runtime("xgboost-distributed"),
    trainer=CustomTrainer(func=xgboost_train_classification, num_nodes=2),
)
client.wait_for_job_status(job_id)
print("\n".join(client.get_job_logs(name=job_id)))
```

## Next Steps

- Check out the [XGBoost example](https://github.com/kubeflow/trainer/blob/master/examples/xgboost/distributed-training/xgboost-distributed.ipynb)
- Learn more about `TrainerClient()` APIs in the [Kubeflow SDK](https://github.com/kubeflow/sdk/blob/main/kubeflow/trainer/api/trainer_client.py)
- Explore **[XGBoost documentation](https://xgboost.readthedocs.io/en/latest/tutorials/kubernetes.html)** for advanced configuration options
