# Kubeflow Data Cache

## Prerequisites

- Rust and Cargo
- AWS CLI configured with appropriate credentials
- `jq` for JSON parsing
- `nc` (netcat) for service health checks


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

## Testing

### Run Client Test
```bash
cd test
cargo run --bin client -- --endpoint http://localhost:50051 --local-rank 2 --world-size 4
```

## Environment Configuration

The system supports two runtime environments:
- **Local Development**: Set `RUNTIME_ENV=LOCAL` to use localhost workers on ports 50052/50053
- **Kubernetes/LWS**: Uses `LWS_LEADER_ADDRESS` and `LWS_GROUP_SIZE` for service discovery


## Local Iceberg Table Support (Planned)

Currently, Kubeflow Data Cache integrates with static Iceberg tables backed by S3,
as described in the *Remote Table Testing* workflow above. While this works well
for production-like environments, it makes local development and end-to-end (e2e)
validation difficult without access to AWS infrastructure.

Issue #3174 tracks adding support for **local Iceberg tables backed by Parquet files**
on the local filesystem. The primary goals of this work are:

- Enable local validation of Data Cache behavior without S3
- Simplify developer workflows for testing and debugging
- Improve e2e test coverage in CI environments

This functionality is currently under development. Once available, this section
will be updated with instructions for setting up local Iceberg tables and running
local validation and e2e tests.
