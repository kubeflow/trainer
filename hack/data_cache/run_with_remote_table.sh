#!/bin/bash

# Check for required arguments
if [ $# -lt 5 ]; then
    echo "Usage: $0 <iam-role-arn> <metadata-loc> <table-name> <schema-name> <aws-profile> [environment]"
    exit 1
fi

IAM_ROLE_ARN="$1"
METADATA_LOC="$2"
TABLE_NAME="$3"
SCHEMA_NAME="$4"
AWS_PROFILE="$5"
ENVIRONMENT="${6:-LOCAL}"

echo "Using IAM Role: $IAM_ROLE_ARN"
echo "Metadata Location: $METADATA_LOC"
echo "Table Name: $TABLE_NAME"
echo "Schema Name: $SCHEMA_NAME"
echo "Environment: $ENVIRONMENT"
echo "AWS Profile: $AWS_PROFILE"

role_output=$(aws sts assume-role --role-arn "$IAM_ROLE_ARN" --role-session-name "RoleSession1" --profile "$AWS_PROFILE")

# Parse the JSON output using jq
export AWS_ACCESS_KEY_ID=$(echo $role_output | jq -r '.Credentials.AccessKeyId')
export AWS_SECRET_ACCESS_KEY=$(echo $role_output | jq -r '.Credentials.SecretAccessKey')
export AWS_SESSION_TOKEN=$(echo $role_output | jq -r '.Credentials.SessionToken')
export AWS_REGION=us-west-2
export AWS_EC2_METADATA_DISABLED=true

# Set required environment variables for testing
export METADATA_LOC="$METADATA_LOC"
export TABLE_NAME="$TABLE_NAME"
export SCHEMA_NAME="$SCHEMA_NAME"
export RUNTIME_ENV="$ENVIRONMENT"

# Function to cleanup processes on exit
cleanup() {
    echo ""
    echo "Stopping services..."
    kill -9 $WORKER1_PID $WORKER2_PID $HEAD_PID 2>/dev/null || true
    wait $WORKER1_PID $WORKER2_PID $HEAD_PID 2>/dev/null || true
    rm -rf /tmp/test_metadata
    exit 0
}

# Set up signal handlers for graceful shutdown
trap cleanup SIGINT SIGTERM

# Function to check if a service is ready
check_service_ready() {
    local host=$1
    local port=$2
    local service_name=$3

    echo "Waiting for $service_name to be ready on $host:$port..."
    while ! nc -z "$host" "$port" 2>/dev/null; do
        echo "  $service_name not ready yet, waiting 2 seconds..."
        sleep 2
    done
    echo "  $service_name is ready!"
}

echo "Starting worker node 1..."
cargo run --bin worker -- 0.0.0.0 50052 > worker1.log 2>&1 &
WORKER1_PID=$!

echo "Starting worker node 2..."
cargo run --bin worker -- 0.0.0.0 50053 > worker2.log 2>&1 &
WORKER2_PID=$!

# Wait for both workers to be ready
check_service_ready localhost 50052 "worker1"
check_service_ready localhost 50053 "worker2"

echo "Both workers are ready, starting head node..."
cargo run --bin head -- 0.0.0.0 50051 > head.log 2>&1 &
HEAD_PID=$!

check_service_ready localhost 50051 "head"

echo "All services are running. Press Ctrl+C to stop all services."
wait

#echo "Running client test..."
#cd src/client && cargo run 2>&1
#CLIENT_EXIT_CODE=$?
