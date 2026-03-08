# Distributed Data Cache

Enable efficient data streaming for distributed training workloads with Kubeflow's distributed Arrow cache cluster.

## Overview

Kubeflow's distributed data cache provides efficient data preprocessing and streaming for distributed training workloads. By leveraging Apache Arrow and Apache DataFusion, the cache cluster stores data in-memory with zero-copy GPU transfer capabilities, eliminating redundant preprocessing and enabling scalable data access across multiple training jobs.

Key features:
- **Scalable access**: Multiple training jobs can simultaneously stream data from the cache via Apache Arrow Flight protocol
- **Reduced redundancy**: Data preprocessing occurs once in the cache cluster rather than on each training node
- **Distributed architecture**: Automatically partitions datasets across data nodes for parallel access
- **Zero-copy GPU transfer**: Direct data transfer to GPU memory using Apache Arrow
- **Iceberg table format**: Standard table format with S3 storage support

## How It Works

The distributed data cache uses a two-stage workflow:

### Stage 1: Dataset Initializer Phase

A distributed cache cluster is created that:
1. Loads data from S3-based Iceberg tables
2. Preprocesses and caches data in memory using Apache Arrow
3. Serves data to training nodes via Arrow Flight protocol
4. Automatically partitions dataset across cache nodes

### Stage 2: Training Phase

Training nodes:
1. Stream data from the cache cluster instead of loading raw data
2. Receive preprocessed, sharded data ready for training
3. Achieve faster iteration with reduced I/O overhead
4. Focus computational resources on model training

```{mermaid}
graph LR
    S3[(S3 Iceberg Tables)] --> DC[Data Cache Cluster]
    DC --> TN1[Training Node 1]
    DC --> TN2[Training Node 2]
    DC --> TN3[Training Node 3]
    DC --> TN4[Training Node 4]
```

## Prerequisites

Before using the distributed data cache, ensure you have:

- Completed the [Getting Started](../getting-started/index) guide
- Kubeflow Trainer controller manager installed
- LeaderWorkerSet controller manager installed
- Access to S3-compatible storage with Iceberg tables
- AWS IAM credentials for S3 access (if using AWS)

## Installation

The distributed data cache requires additional components beyond the base Kubeflow Trainer installation.

### Using kubectl

Install data cache components with server-side apply:

```bash
export VERSION=v2.1.0
kubectl apply --server-side -k \
  "https://github.com/kubeflow/trainer.git/manifests/overlays/data-cache?ref=${VERSION}"
```

This installs:
- `ClusterTrainingRuntime` with `torch-distributed-with-cache` support
- RBAC resources for initializer bootstrap
- Service accounts and role bindings

### Using Helm Charts

Enable data cache during Helm installation:

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --set dataCache.enabled=true \
    --namespace kubeflow-system
```

Or upgrade existing installation:

```bash
helm upgrade kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer \
    --set dataCache.enabled=true \
    --namespace kubeflow-system \
    --reuse-values
```

### Verify Installation

Check that the runtime is available:

```bash
kubectl get clustertrainingruntime torch-distributed-with-cache
```

Verify RBAC resources in your namespace:

```bash
kubectl get sa,rolebinding -n default | grep cache-initializer
```

Expected output:
```
serviceaccount/cache-initializer-bootstrap
rolebinding.rbac.authorization.k8s.io/cache-initializer-bootstrap
```

## Data Format Requirements

The distributed data cache requires datasets in Iceberg table format stored in S3-compatible storage.

### Iceberg Table Structure

- **Metadata**: JSON file describing table schema, partitions, and snapshots
- **Data files**: Parquet files containing actual data
- **Manifest files**: Track data files and statistics

### Preparing Data with PyIceberg

```python
from pyiceberg.catalog import load_catalog
from pyiceberg.schema import Schema
from pyiceberg.types import NestedField, StringType, IntegerType, FloatType
import pyarrow as pa

# Define schema
schema = Schema(
    NestedField(1, "id", IntegerType(), required=False),
    NestedField(2, "text", StringType(), required=False),
    NestedField(3, "label", IntegerType(), required=False),
)

# Create catalog
catalog = load_catalog("default", **{
    "type": "glue",
    "warehouse": "s3://my-bucket/warehouse",
})

# Create table
table = catalog.create_table(
    "my_namespace.my_table",
    schema=schema,
)

# Prepare data
data = pa.table({
    "id": [1, 2, 3],
    "text": ["sample text 1", "sample text 2", "sample text 3"],
    "label": [0, 1, 0],
})

# Write data to table
table.append(data)

print(f"Metadata location: {table.metadata_location}")
```

### Preparing Data with Apache Spark

```python
from pyspark.sql import SparkSession

# Create Spark session with Iceberg support
spark = SparkSession.builder \
    .appName("IcebergWriter") \
    .config("spark.jars.packages", "org.apache.iceberg:iceberg-spark-runtime-3.3_2.12:1.4.0") \
    .config("spark.sql.extensions", "org.apache.iceberg.spark.extensions.IcebergSparkSessionExtensions") \
    .config("spark.sql.catalog.spark_catalog", "org.apache.iceberg.spark.SparkCatalog") \
    .config("spark.sql.catalog.spark_catalog.type", "hadoop") \
    .config("spark.sql.catalog.spark_catalog.warehouse", "s3://my-bucket/warehouse") \
    .getOrCreate()

# Create DataFrame
df = spark.createDataFrame([
    (1, "sample text 1", 0),
    (2, "sample text 2", 1),
    (3, "sample text 3", 0),
], ["id", "text", "label"])

# Write as Iceberg table
df.writeTo("spark_catalog.my_namespace.my_table") \
    .using("iceberg") \
    .create()

print("Iceberg table created successfully")
```

## Configuration

Configure the distributed data cache using `DataCacheInitializer` in your TrainJob.

### DataCacheInitializer Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `storage_uri` | str | Base path for cache storage (local or S3) | Yes |
| `metadata_loc` | str | S3 path to Iceberg metadata.json file | Yes |
| `iam_role` | str | AWS IAM role ARN for S3 access | Yes (for AWS) |
| `num_data_nodes` | int | Number of cache nodes (default: 1) | No |

### Basic Configuration Example

```python
from kubeflow.trainer import (
    TrainerClient,
    CustomTrainer,
    Initializer,
    DataCacheInitializer,
)

client = TrainerClient()

job_id = client.train(
    trainer=CustomTrainer(
        func=train_with_cache,
        num_nodes=4,
        resources_per_node={
            "cpu": 4,
            "memory": "16Gi",
            "gpu": 1,
        },
    ),
    initializer=Initializer(
        data_cache=DataCacheInitializer(
            storage_uri="s3://my-bucket/cache-storage",
            metadata_loc="s3://my-bucket/warehouse/my_namespace/my_table/metadata/metadata.json",
            iam_role="arn:aws:iam::123456789012:role/KubeflowTrainerRole",
            num_data_nodes=2,  # 2 cache nodes for parallel access
        )
    ),
    runtime="torch-distributed-with-cache"
)

print(f"Training job with data cache: {job_id}")
```

### Advanced Configuration

```python
# Multi-node training with distributed cache
job_id = client.train(
    trainer=CustomTrainer(
        func=train_with_cache,
        num_nodes=8,  # 8 training nodes
        resources_per_node={
            "cpu": 8,
            "memory": "32Gi",
            "gpu": 2,
        },
    ),
    initializer=Initializer(
        data_cache=DataCacheInitializer(
            storage_uri="s3://my-bucket/cache-storage",
            metadata_loc="s3://my-bucket/warehouse/my_namespace/my_table/metadata/metadata.json",
            iam_role="arn:aws:iam::123456789012:role/KubeflowTrainerRole",
            num_data_nodes=4,  # 4 cache nodes for higher throughput
        )
    ),
    runtime="torch-distributed-with-cache"
)
```

## PyTorch Integration

Kubeflow provides `DataCacheDataset`, a PyTorch `IterableDataset` subclass that streams data from the cache cluster.

### Basic Usage

```python
def train_with_cache():
    """Train PyTorch model with distributed data cache."""
    import torch
    import torch.distributed as dist
    from torch.nn.parallel import DistributedDataParallel as DDP
    from kubeflow.trainer.dataset.data_cache import DataCacheDataset

    # Initialize distributed training
    dist.init_process_group(backend="nccl")
    rank = dist.get_rank()
    world_size = dist.get_world_size()

    # Create dataset from cache
    dataset = DataCacheDataset(
        cache_addr="data-cache-service:50051",  # Automatic service discovery
        batch_size=32,
    )

    # Create dataloader
    dataloader = torch.utils.data.DataLoader(
        dataset,
        batch_size=None,  # Dataset returns batches
        num_workers=2,
        pin_memory=True,
    )

    # Model setup
    model = MyModel().cuda()
    model = DDP(model)
    optimizer = torch.optim.Adam(model.parameters(), lr=0.001)

    # Training loop
    for epoch in range(10):
        for batch in dataloader:
            # batch is already a tensor from DataCacheDataset
            inputs = batch["input"].cuda()
            labels = batch["label"].cuda()

            optimizer.zero_grad()
            outputs = model(inputs)
            loss = torch.nn.functional.cross_entropy(outputs, labels)
            loss.backward()
            optimizer.step()

            if rank == 0 and step % 100 == 0:
                print(f"Epoch {epoch}, Loss: {loss.item():.4f}")

    dist.destroy_process_group()
```

### Custom Preprocessing

Override `DataCacheDataset` methods for custom preprocessing:

```python
from kubeflow.trainer.dataset.data_cache import DataCacheDataset
import torch

class CustomDataCacheDataset(DataCacheDataset):
    """Custom dataset with preprocessing."""

    def preprocess_batch(self, record_batch):
        """Custom preprocessing for each batch."""
        # Convert Arrow RecordBatch to tensors
        inputs = torch.tensor(record_batch["input"].to_numpy())
        labels = torch.tensor(record_batch["label"].to_numpy())

        # Apply custom transformations
        inputs = inputs.float() / 255.0  # Normalize
        labels = labels.long()

        return {"input": inputs, "label": labels}

# Use in training
def train_with_custom_preprocessing():
    import torch.distributed as dist

    dist.init_process_group(backend="nccl")

    dataset = CustomDataCacheDataset(
        cache_addr="data-cache-service:50051",
        batch_size=64,
    )

    dataloader = torch.utils.data.DataLoader(
        dataset,
        batch_size=None,
        num_workers=4,
    )

    # Training loop...
```

### Sharding Across Workers

`DataCacheDataset` automatically distributes data shards across distributed workers:

```python
def train_with_sharding():
    """Training with automatic data sharding."""
    import torch.distributed as dist
    from kubeflow.trainer.dataset.data_cache import DataCacheDataset

    # Initialize distributed
    dist.init_process_group(backend="nccl")
    rank = dist.get_rank()
    world_size = dist.get_world_size()

    # Dataset automatically shards data across workers
    dataset = DataCacheDataset(
        cache_addr="data-cache-service:50051",
        batch_size=32,
        # Sharding happens automatically based on rank and world_size
    )

    print(f"Rank {rank}/{world_size}: "
          f"Receiving shard {rank} of {world_size}")

    # Each worker receives unique data shard
    for batch in dataset:
        # Process batch...
        pass
```

## Complete Training Example

Here's a complete example using the distributed data cache:

```python
def train_resnet_with_cache():
    """Train ResNet on ImageNet with distributed data cache."""
    import torch
    import torch.nn as nn
    import torch.distributed as dist
    from torch.nn.parallel import DistributedDataParallel as DDP
    from torchvision.models import resnet50
    from kubeflow.trainer.dataset.data_cache import DataCacheDataset

    # Initialize distributed training
    dist.init_process_group(backend="nccl")
    rank = dist.get_rank()
    world_size = dist.get_world_size()

    if rank == 0:
        print(f"Starting distributed training with {world_size} workers")

    # Create dataset from cache
    train_dataset = DataCacheDataset(
        cache_addr="data-cache-service:50051",
        batch_size=256,
    )

    # Create dataloader
    train_loader = torch.utils.data.DataLoader(
        train_dataset,
        batch_size=None,  # Dataset returns batches
        num_workers=4,
        pin_memory=True,
        prefetch_factor=2,
    )

    # Model setup
    model = resnet50(pretrained=False, num_classes=1000)
    model = model.cuda()
    model = DDP(model)

    # Optimizer and loss
    optimizer = torch.optim.SGD(
        model.parameters(),
        lr=0.1,
        momentum=0.9,
        weight_decay=1e-4
    )
    criterion = nn.CrossEntropyLoss().cuda()

    # Learning rate scheduler
    scheduler = torch.optim.lr_scheduler.StepLR(
        optimizer,
        step_size=30,
        gamma=0.1
    )

    # Training loop
    num_epochs = 90

    if rank == 0:
        print("Starting training...")

    for epoch in range(num_epochs):
        model.train()
        epoch_loss = 0.0
        num_batches = 0

        for batch_idx, batch in enumerate(train_loader):
            # Get data from cache
            images = batch["image"].cuda()
            labels = batch["label"].cuda()

            # Forward pass
            optimizer.zero_grad()
            outputs = model(images)
            loss = criterion(outputs, labels)

            # Backward pass
            loss.backward()
            optimizer.step()

            # Accumulate loss
            epoch_loss += loss.item()
            num_batches += 1

            # Log progress
            if batch_idx % 100 == 0 and rank == 0:
                print(f"Epoch {epoch}, Batch {batch_idx}, Loss: {loss.item():.4f}")

        # Update learning rate
        scheduler.step()

        # Print epoch summary
        if rank == 0:
            avg_loss = epoch_loss / num_batches
            current_lr = scheduler.get_last_lr()[0]
            print(f"Epoch {epoch} completed. "
                  f"Average Loss: {avg_loss:.4f}, LR: {current_lr:.6f}")

    # Save model
    if rank == 0:
        torch.save(model.state_dict(), "resnet50_imagenet.pth")
        print("Training completed. Model saved.")

    dist.destroy_process_group()
```

## Performance Considerations

### Cache Node Sizing

Choose `num_data_nodes` based on:

- **Dataset size**: Larger datasets benefit from more nodes
- **Training parallelism**: More training nodes need more cache nodes for throughput
- **Memory per node**: Distribute dataset across nodes to fit in memory

**General guidelines:**
- Small datasets (<10GB): 1-2 cache nodes
- Medium datasets (10-100GB): 2-4 cache nodes
- Large datasets (>100GB): 4-8+ cache nodes

### Memory Configuration

Ensure cache nodes have sufficient memory:

```python
# Configure cache node resources (modify runtime YAML)
# Example: 64GB memory per cache node
num_data_nodes = 4
dataset_size_gb = 200
memory_per_node_gb = dataset_size_gb / num_data_nodes * 1.5  # 75GB with overhead
```

### Network Bandwidth

- Use high-bandwidth network (10Gbps+) between cache and training nodes
- Collocate cache and training nodes in same availability zone
- Consider data compression for bandwidth-constrained environments

### Batch Size Tuning

Larger batches reduce network overhead:

```python
dataset = DataCacheDataset(
    cache_addr="data-cache-service:50051",
    batch_size=512,  # Larger batches = fewer network requests
)
```

## Monitoring and Debugging

### Check Cache Cluster Status

```bash
# List cache initializer pods
kubectl get pods -l app=data-cache-initializer

# Check cache cluster logs
kubectl logs -l app=data-cache-initializer -f

# Verify service is available
kubectl get svc data-cache-service
```

### Monitor Cache Performance

```python
def train_with_monitoring():
    """Training with cache performance monitoring."""
    import time
    from kubeflow.trainer.dataset.data_cache import DataCacheDataset

    dataset = DataCacheDataset(
        cache_addr="data-cache-service:50051",
        batch_size=256,
    )

    start_time = time.time()
    batch_count = 0

    for batch in dataset:
        batch_count += 1

        if batch_count % 100 == 0:
            elapsed = time.time() - start_time
            throughput = batch_count / elapsed
            print(f"Batch {batch_count}, Throughput: {throughput:.2f} batches/sec")

        # Training code...
```

### Troubleshooting

**Cache connection errors:**

```bash
# Verify service DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup data-cache-service

# Test cache connectivity
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- \
  nc -zv data-cache-service 50051
```

**Slow data loading:**

- Increase `num_data_nodes` for better parallelism
- Increase `num_workers` in DataLoader
- Use larger batch sizes to reduce network overhead
- Ensure cache nodes have sufficient memory

## Examples and Resources

### Complete Examples

- **ImageNet Training**: Large-scale image classification with data cache (coming soon)
- **LLM Pre-training**: Language model training with cached datasets (coming soon)

### API Documentation

- TrainerClient API Reference: Complete SDK documentation (coming soon)
- DataCacheDataset API: Dataset class documentation (coming soon)

### Related Guides

- [Getting Started](../getting-started/index): Initial setup
- [PyTorch Distributed](pytorch): PyTorch distributed training
- [DeepSpeed](deepspeed): Large-scale training with DeepSpeed

## Limitations

Current limitations of the distributed data cache:

- **Iceberg format only**: Datasets must be in Iceberg table format
- **S3 storage**: Currently supports S3-compatible storage only
- **Read-only**: Cache is read-only during training (no writes)
- **Memory-bound**: Dataset must fit in cache cluster memory
- **No dynamic scaling**: Cache nodes don't auto-scale during training
