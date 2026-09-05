# Kubeflow Data Cache

## Prerequisites

- Rust and Cargo
- `nc` (netcat) for service health checks

Only needed to run against a table on S3 (see [Option 2](#option-2-remote-table-testing)):

- AWS CLI configured with appropriate credentials
- `jq` for JSON parsing


## Development Setup

### Build the project
```bash
cargo build
```

### Build in release mode
```bash
cargo build --release
```

## Docker Build Instructions

### Build the Docker image
```bash
docker build -f cmd/data_cache/Dockerfile -t kubeflow_data_cache .
```

### Run the head service
```bash
docker run -p 50051:50051 kubeflow_data_cache head
```

### Run the worker service
```bash
docker run -p 50052:50052 kubeflow_data_cache worker
```

## Running the System

### Option 1: Local Table Testing

Run the system against an Iceberg table on the local filesystem. No AWS account
or object store is required — the `iceberg` crate enables its `storage-fs`
backend by default, and the cache picks the backend from the scheme of
`METADATA_LOC`, so a local table loads through the same code path as one on S3.

```bash
../../hack/data_cache/run_with_local_table.sh [warehouse-dir] [rows-per-file ...]
```

**Example:**
```bash
../../hack/data_cache/run_with_local_table.sh /tmp/kubeflow-data-cache 3 2
```

**Parameters:**
- `warehouse-dir` (optional): Directory to generate the table in (defaults to `/tmp/kubeflow-data-cache`)
- `rows-per-file` (optional): Row count for each Parquet data file (defaults to `3 2`, i.e. two files)

This script will:
1. Generate an Iceberg table with Parquet data files under the warehouse directory
2. Export `METADATA_LOC`, `SCHEMA_NAME` and `TABLE_NAME` for the generated table
3. Start two worker nodes (ports 50052, 50053)
4. Start the head node (port 50051)
5. Wait for all services to be ready

Press `Ctrl+C` to stop all services.

To generate a table without starting a cluster:

```bash
cargo run --features local-fixtures --example create_local_table -- /tmp/warehouse 3 2
```

It prints the `export` lines for the three environment variables, so its output
can be sourced directly:

```bash
eval "$(cargo run -q --features local-fixtures --example create_local_table -- /tmp/warehouse)"
```

> **Note:** Iceberg records absolute paths in its metadata, so a generated table
> cannot be moved or copied to another machine. Regenerate it instead.

### Option 2: Remote Table Testing

Run the system with remote table configuration using IAM roles:

```bash
../../hack/data_cache/run_with_remote_table.sh <iam-role-arn> <metadata-loc> <table-name> <schema-name> <aws-profile> [environment]
```

**Example:**
```bash
../../hack/data_cache/run_with_remote_table.sh \
  arn:aws:iam::<account_id>:role/<role_name> \
  s3a://<metadata_file_path> \
  <table_name> \
  <schema_name> \
  <account_id> \
  LOCAL
```

**Parameters:**
- `iam-role-arn` (required): IAM role ARN for AWS access
- `metadata-loc` (required): S3 location of the metadata file
- `table-name` (required): Name of the table
- `schema-name` (required): Name of the schema
- `aws-profile` (required): AWS profile name
- `environment` (optional): Runtime environment (defaults to "LOCAL")

This script will:
1. Assume the specified IAM role
2. Set up AWS credentials and environment variables
3. Start two worker nodes (ports 50052, 50053)
4. Start the head node (port 50051)
5. Wait for all services to be ready

Press `Ctrl+C` to stop all services.

## Testing

### Run Unit Tests

```bash
make test-rust
```

Or directly from this directory:

```bash
cargo test --lib --bins
```

The tests that cover Iceberg table loading generate a local table with Parquet
data files into a temporary directory and drive the real head and worker code
paths against it. No fixture is checked into the repository, and no credentials
or network access are needed.

The helpers that build the table live in `src/testkit/local_table.rs`. They are
compiled for test builds and for builds that pass `--features local-fixtures`,
so they are not part of the `head` and `worker` binaries in the container image.

If a build runs out of memory, limit the number of parallel jobs:

```bash
cargo test --lib --bins -j 2
```

### Run Client Test
```bash
cd test
cargo run --bin client -- --endpoint http://localhost:50051 --local-rank 2 --world-size 4
```

## Environment Configuration

The system supports two runtime environments:
- **Local Development**: Set `RUNTIME_ENV=LOCAL` to use localhost workers on ports 50052/50053
- **Kubernetes/LWS**: Uses `LWS_LEADER_ADDRESS` and `LWS_GROUP_SIZE` for service discovery
