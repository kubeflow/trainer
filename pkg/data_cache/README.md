# Kubeflow Data Cache

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
docker build -f cmd/data_cache/Dockerfile -t arrow_cache .
```

### Run the head service
```bash
docker run -p 50051:50051 arrow_cache head
```

### Run the worker service
```bash
docker run -p 50052:50052 arrow_cache worker
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
cd src/client
cargo run
```

## Environment Configuration

The system supports two runtime environments:
- **Local Development**: Set `RUNTIME_ENV=LOCAL` to use localhost workers on ports 50052/50053
- **Kubernetes/LWS**: Uses `LWS_LEADER_ADDRESS` and `LWS_GROUP_SIZE` for service discovery

## Prerequisites

- Rust and Cargo
- AWS CLI configured with appropriate credentials
- `jq` for JSON parsing
- `nc` (netcat) for service health checks
