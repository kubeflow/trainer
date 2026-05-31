# Kubeflow Data Cache

## Prerequisites

- Rust and Cargo
- For **remote (S3) tables**: AWS CLI configured with appropriate credentials and `jq`
- For **local tables**: Python 3 with `pyiceberg`, `pyarrow`, and `sqlalchemy` (fixture generation only)
- `nc` (netcat) and `curl` for service health checks


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

### Option 1: Remote Table Testing

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

### Option 2: Local Iceberg table (file://)

Use an on-disk Iceberg table with Parquet files for local validation and CI (no AWS credentials).

**Generate the test fixture once** (from repository root):

```bash
python3 hack/data_cache/generate_local_iceberg_fixture.py
```

**Run head and workers** (generates the fixture automatically if missing):

```bash
./hack/data_cache/run_with_local_table.sh
```

Default table identifiers: `SCHEMA_NAME=local`, `TABLE_NAME=demo`. The script sets `METADATA_LOC` to the latest `*.metadata.json` under `pkg/data_cache/testdata/local_iceberg/`.

`METADATA_LOC` must be an absolute URI (`file://`, `s3://`, or `s3a://`).

## Testing

### Run unit and integration tests

From repository root:

```bash
make test-rust
```

This includes a local Iceberg fixture integration test (`tests/local_iceberg_fixture.rs`), which regenerates the fixture via Python when needed.

### Run Client Test

With services running (remote or local script):

```bash
cd test
cargo run --bin client -- --endpoint http://localhost:50051 --local-rank 2 --world-size 4
```

## Environment Configuration

The system supports two runtime environments:
- **Local Development**: Set `RUNTIME_ENV=LOCAL` to use localhost workers on ports 50052/50053
- **Kubernetes/LWS**: Uses `LWS_LEADER_ADDRESS` and `LWS_GROUP_SIZE` for service discovery
