# XGBoost Runtime User Guide

This guide explains how to run distributed XGBoost jobs with Kubeflow Trainer.

## Overview

Kubeflow Trainer provides an XGBoost runtime for distributed training on Kubernetes. The runtime uses Rabit-based coordination and injects the environment variables required for worker discovery and collective communication.

Use this guide if you want to:

- run distributed XGBoost training with `TrainJob`
- use the Trainer SDK with the XGBoost runtime
- understand which environment variables are injected by the runtime
- start from the existing notebook example in this repository

## Prerequisites

Before you begin, make sure that:

1. Kubeflow Trainer is installed in your cluster.
2. The XGBoost runtime is installed.
3. You have access to a Kubernetes cluster with enough CPU or GPU resources for your workers.
4. You have the Kubeflow Python SDK installed.

```bash
pip install -U kubeflow
```

## Install the XGBoost Runtime

Apply the runtime manifest:

```bash
kubectl apply -f manifests/base/runtimes/xgboost_distributed.yaml
```

You can verify that the runtime is available with:

```bash
kubectl get clustertrainingruntime xgboost-distributed
```

## Submit a Distributed XGBoost Job

The repository already includes an example notebook:

- [`examples/xgboost/distributed-training/xgboost-distributed.ipynb`](../../../examples/xgboost/distributed-training/xgboost-distributed.ipynb)

At a high level, your training function should:

1. read the injected `DMLC_*` environment variables
2. start the tracker on rank 0
3. initialize the XGBoost collective communicator
4. build `DMatrix` objects inside the communicator context
5. train and save the model from rank 0

Example skeleton:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer


def xgboost_train(num_rounds: int = 100, max_depth: int = 6):
    import os
    import xgboost as xgb
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
        # Build DMatrix inside the collective context.
        # Load your data, train the model, and save from rank 0.
        pass

    if tracker is not None:
        tracker.wait_for()


client = TrainerClient()
client.train(
    trainer=CustomTrainer(func=xgboost_train, num_nodes=4),
    runtime=next(r for r in client.list_runtimes() if r.name == "xgboost-distributed"),
)
```

## Important Runtime Details

### Injected Environment Variables

The XGBoost runtime injects the environment variables needed for distributed coordination, including:

- `DMLC_TASK_ID`
- `DMLC_NUM_WORKER`
- `DMLC_TRACKER_URI`
- `DMLC_TRACKER_PORT`

### Rank 0 Starts the Tracker

Rank 0 is responsible for starting the Rabit tracker before the remaining workers connect.

### Build `DMatrix` Inside the Communicator Context

`DMatrix` construction should happen inside the communicator context because distributed setup may require synchronization across workers.

## CPU and GPU Workloads

The runtime supports both CPU and GPU jobs:

- CPU jobs use one worker per node.
- GPU jobs derive worker placement from the allocated GPU resources.

For GPU training, set the appropriate XGBoost training parameter, for example:

```python
params = {
    "objective": "binary:logistic",
    "max_depth": 6,
    "eta": 0.1,
    "device": "cuda",
}
```

## Related Resources

- [XGBoost runtime proposal](../../proposals/2598-XGboost-runtime-trainer-v2/README.md)
- [PyTorch examples](../../../examples/pytorch/README.md)
- [Distributed XGBoost notebook](../../../examples/xgboost/distributed-training/xgboost-distributed.ipynb)

## Troubleshooting

### Runtime not found

If `xgboost-distributed` is not available, confirm that the runtime manifest has been applied.

### Workers cannot connect

Check that:

- rank 0 starts the tracker successfully
- the injected `DMLC_*` variables are present in every worker
- your cluster networking allows worker-to-worker communication

### Training hangs during dataset setup

Make sure `DMatrix` is created inside the collective communication context.
