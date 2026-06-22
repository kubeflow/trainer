# Getting Started

This guide describes how to get started with Kubeflow Trainer and run distributed training
with PyTorch.

## Prerequisites

Ensure that you have access to a Kubernetes cluster with Kubeflow Trainer
control plane installed. If it is not set up yet, follow
[the installation guide](../operator-guides/installation.md) to quickly deploy
Kubeflow Trainer.

### Installing the Kubeflow Python SDK

Install the latest stable version of Kubeflow Python SDK:

```bash
pip install -U kubeflow
```

For the latest changes of Kubeflow Python SDK, install from the source repository directly:

```bash
pip install git+https://github.com/kubeflow/sdk.git@main
```

## Getting Started with PyTorch

Before creating a Kubeflow TrainJob, define the training function that handles end-to-end model
training. Each PyTorch node will execute this function within the configured distributed environment.
Typically, this function includes steps to download the dataset, initialize the model, and train it.

Kubeflow Trainer automatically sets up the distributed environment for PyTorch, enabling
[Distributed Data Parallel (DDP)](https://pytorch.org/tutorials/intermediate/ddp_tutorial.html).

```python
def train_pytorch():
    import os

    import torch
    from torch import nn
    import torch.nn.functional as F

    from torchvision import datasets, transforms
    import torch.distributed as dist
    from torch.utils.data import DataLoader, DistributedSampler

    # [1] Configure CPU/GPU device and distributed backend.
    # Kubeflow Trainer will automatically configure the distributed environment.
    device, backend = ("cuda", "nccl") if torch.cuda.is_available() else ("cpu", "gloo")
    dist.init_process_group(backend=backend)

    local_rank = int(os.getenv("LOCAL_RANK", 0))
    print(
        "Distributed Training with WORLD_SIZE: {}, RANK: {}, LOCAL_RANK: {}.".format(
            dist.get_world_size(),
            dist.get_rank(),
            local_rank,
        )
    )

    # [2] Define PyTorch CNN Model to be trained.
    class Net(nn.Module):
        def __init__(self):
            super(Net, self).__init__()
            self.conv1 = nn.Conv2d(1, 20, 5, 1)
            self.conv2 = nn.Conv2d(20, 50, 5, 1)
            self.fc1 = nn.Linear(4 * 4 * 50, 500)
            self.fc2 = nn.Linear(500, 10)

        def forward(self, x):
            x = F.relu(self.conv1(x))
            x = F.max_pool2d(x, 2, 2)
            x = F.relu(self.conv2(x))
            x = F.max_pool2d(x, 2, 2)
            x = x.view(-1, 4 * 4 * 50)
            x = F.relu(self.fc1(x))
            x = self.fc2(x)
            return F.log_softmax(x, dim=1)

    # [3] Attach model to the correct device.
    device = torch.device(f"{device}:{local_rank}")
    model = nn.parallel.DistributedDataParallel(Net().to(device))
    model.train()
    optimizer = torch.optim.SGD(model.parameters(), lr=0.1, momentum=0.9)

    # [4] Get the Fashion-MNIST dataset on local_rank=0 process.
    if local_rank == 0:
        dataset = datasets.FashionMNIST(
            "./data",
            train=True,
            download=True,
            transform=transforms.Compose([transforms.ToTensor()]),
        )
    dist.barrier()
    dataset = datasets.FashionMNIST(
        "./data",
        train=True,
        download=False,
        transform=transforms.Compose([transforms.ToTensor()]),
    )
    train_loader = DataLoader(
        dataset,
        batch_size=100,
        sampler=DistributedSampler(dataset),
    )

    # [5] Define the training loop.
    for epoch in range(3):
        for batch_idx, (inputs, labels) in enumerate(train_loader):
            # Attach tensors to the device.
            inputs, labels = inputs.to(device), labels.to(device)

            # Forward pass
            outputs = model(inputs)
            loss = F.nll_loss(outputs, labels)

            # Backward pass
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            if batch_idx % 10 == 0 and dist.get_rank() == 0:
                print(
                    "Train Epoch: {} [{}/{} ({:.0f}%)]\tLoss: {:.6f}".format(
                        epoch,
                        batch_idx * len(inputs),
                        len(train_loader.dataset),
                        100.0 * batch_idx / len(train_loader),
                        loss.item(),
                    )
                )

    # Wait for the training to complete and destroy to PyTorch distributed process group.
    dist.barrier()
    if dist.get_rank() == 0:
        print("Training is finished")
    dist.destroy_process_group()
```

After configuring the training function, check the available Kubeflow Training Runtimes:

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

for r in TrainerClient().list_runtimes():
    print(f"Runtime: {r.name}")
```

You should be able to see list of available Training Runtimes:

```python
Runtime: torch-distributed
```

Create a TrainJob using the `torch-distributed` Runtime, which scales your training function across
4 PyTorch nodes, every node has 1 GPU.

```python
job_id = TrainerClient().train(
    trainer=CustomTrainer(
        func=train_pytorch,
        num_nodes=4,
        resources_per_node={
            "cpu": 3,
            "memory": "16Gi",
            "gpu": 1, # Comment this line if you don't have GPUs.
        },
    )
)
```

You can check the steps of the TrainJob and the number of devices each PyTorch node is using:

```python
for s in TrainerClient().get_job(name=job_id).steps:
    print(f"Step: {s.name}, Status: {s.status}, Devices: {s.device} x {s.device_count}")
```

The output:

```python
Step: node-0, Status: Succeeded, Devices: gpu x 1
Step: node-1, Status: Succeeded, Devices: gpu x 1
Step: node-2, Status: Succeeded, Devices: gpu x 1
Step: node-3, Status: Succeeded, Devices: gpu x 1
```

Finally, you can check the training logs from the master node:

```python
for logline in TrainerClient().get_job_logs(job_id, follow=True):
    print(logline)
```

Since training was run on 4 GPUs, each PyTorch node processes 60,000 / 4 = 15,000 images
from the dataset:

```python

Distributed Training with WORLD_SIZE: 4, RANK: 0, LOCAL_RANK: 0.
Downloading http://fashion-mnist.s3-website.eu-central-1.amazonaws.com/train-images-idx3-ubyte.gz
...
Train Epoch: 0 [0/60000 (0%)]	Loss: 2.300872
Train Epoch: 0 [1000/60000 (7%)]	Loss: 1.641364
Train Epoch: 0 [2000/60000 (13%)]	Loss: 1.210384
Train Epoch: 0 [3000/60000 (20%)]	Loss: 0.994264
Train Epoch: 0 [4000/60000 (27%)]	Loss: 0.831711
Train Epoch: 0 [5000/60000 (33%)]	Loss: 0.613464
Train Epoch: 0 [6000/60000 (40%)]	Loss: 0.739163
Train Epoch: 0 [7000/60000 (47%)]	Loss: 0.843191
Train Epoch: 0 [8000/60000 (53%)]	Loss: 0.447067
Train Epoch: 0 [9000/60000 (60%)]	Loss: 0.538711
Train Epoch: 0 [10000/60000 (67%)]	Loss: 0.386125
Train Epoch: 0 [11000/60000 (73%)]	Loss: 0.545219
Train Epoch: 0 [12000/60000 (80%)]	Loss: 0.452070
Train Epoch: 0 [13000/60000 (87%)]	Loss: 0.551063
Train Epoch: 0 [14000/60000 (93%)]	Loss: 0.409985
Train Epoch: 1 [0/60000 (0%)]	Loss: 0.485615
Train Epoch: 1 [1000/60000 (7%)]	Loss: 0.414390
Train Epoch: 1 [2000/60000 (13%)]	Loss: 0.449027
Train Epoch: 1 [3000/60000 (20%)]	Loss: 0.262625
Train Epoch: 1 [4000/60000 (27%)]	Loss: 0.471265
Train Epoch: 1 [5000/60000 (33%)]	Loss: 0.353005
Train Epoch: 1 [6000/60000 (40%)]	Loss: 0.570305
Train Epoch: 1 [7000/60000 (47%)]	Loss: 0.574882
Train Epoch: 1 [8000/60000 (53%)]	Loss: 0.393912
Train Epoch: 1 [9000/60000 (60%)]	Loss: 0.346508
Train Epoch: 1 [10000/60000 (67%)]	Loss: 0.311427
Train Epoch: 1 [11000/60000 (73%)]	Loss: 0.336713
Train Epoch: 1 [12000/60000 (80%)]	Loss: 0.321332
Train Epoch: 1 [13000/60000 (87%)]	Loss: 0.348189
Train Epoch: 1 [14000/60000 (93%)]	Loss: 0.360835
Train Epoch: 2 [0/60000 (0%)]	Loss: 0.416435
Train Epoch: 2 [1000/60000 (7%)]	Loss: 0.364135
Train Epoch: 2 [2000/60000 (13%)]	Loss: 0.392644
Train Epoch: 2 [3000/60000 (20%)]	Loss: 0.265317
Train Epoch: 2 [4000/60000 (27%)]	Loss: 0.400089
Train Epoch: 2 [5000/60000 (33%)]	Loss: 0.333744
Train Epoch: 2 [6000/60000 (40%)]	Loss: 0.515001
Train Epoch: 2 [7000/60000 (47%)]	Loss: 0.489475
Train Epoch: 2 [8000/60000 (53%)]	Loss: 0.304395
Train Epoch: 2 [9000/60000 (60%)]	Loss: 0.274867
Train Epoch: 2 [10000/60000 (67%)]	Loss: 0.273643
Train Epoch: 2 [11000/60000 (73%)]	Loss: 0.303883
Train Epoch: 2 [12000/60000 (80%)]	Loss: 0.268735
Train Epoch: 2 [13000/60000 (87%)]	Loss: 0.277623
Train Epoch: 2 [14000/60000 (93%)]	Loss: 0.247948
Training is finished
```

## Next Steps

- Explore [the PyTorch guide for Kubeflow Trainer](../user-guides/pytorch.md).
