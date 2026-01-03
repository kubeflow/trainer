import argparse
import os
import sys
from kubeflow.trainer import CustomTrainer, TrainerClient

def train_fn():
    import torch
    import torch.distributed as dist
    import torch.nn.functional as F
    from torch import nn
    from torch.utils.data import DataLoader, DistributedSampler
    from torchvision import datasets, transforms

    class Net(nn.Module):
        def __init__(self):
            super().__init__()
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
            return F.log_softmax(self.fc2(x), dim=1)

    # Distributed setup
    # If we're just testing locally, we'll bypass the DDP init if WORLD_SIZE isn't set
    world_size = int(os.environ.get("WORLD_SIZE", 1))
    if world_size > 1:
        backend = "nccl" if torch.cuda.is_available() else "gloo"
        dist.init_process_group(backend=backend)
        local_rank = int(os.getenv("LOCAL_RANK", 0))
        device = torch.device(f"cuda:{local_rank}" if torch.cuda.is_available() else "cpu")
        model = nn.parallel.DistributedDataParallel(Net().to(device))
        sampler = DistributedSampler(datasets.FashionMNIST("./data", train=True, download=True, transform=transforms.ToTensor()))
    else:
        device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        model = Net().to(device)
        sampler = None

    optimizer = torch.optim.SGD(model.parameters(), lr=0.1, momentum=0.9)

    # Load data
    dataset = datasets.FashionMNIST("./data", train=True, download=True, transform=transforms.ToTensor())
    loader = DataLoader(dataset, batch_size=128, sampler=sampler, shuffle=(sampler is None))

    print(f"Starting training on {device}...")
    for epoch in range(1):
        model.train()
        for batch_idx, (data, target) in enumerate(loader):
            data, target = data.to(device), target.to(device)
            optimizer.zero_grad()
            output = model(data)
            loss = F.nll_loss(output, target)
            loss.backward()
            optimizer.step()

            if batch_idx % 20 == 0:
                print(f"Epoch: {epoch} | Batch: {batch_idx} | Loss: {loss.item():.4f}")
            
            # Short circuit for test mode if we want, but let's run a full epoch
            if os.environ.get("KUBEFLOW_TRAINER_TEST"):
                if batch_idx >= 10: break

    if world_size > 1:
        dist.destroy_process_group()

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--nodes", type=int, default=1, help="Number of nodes")
    parser.add_argument("--test", action="store_true", help="Run a quick local test without Kubeflow")
    args = parser.parse_args()

    if args.test:
        print("Running quick local test...")
        os.environ["KUBEFLOW_TRAINER_TEST"] = "1"
        train_fn()
        sys.exit(0)

    client = TrainerClient()
    
    job_name = client.train(
        trainer=CustomTrainer(
            func=train_fn,
            num_nodes=args.nodes,
            packages_to_install=["torch", "torchvision"]
        )
    )

    print(f"Submitted TrainJob: {job_name}")
