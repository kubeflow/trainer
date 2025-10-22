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

    # Kill any remaining processes on the ports
    echo "Cleaning up ports..."
    for port in 8080 8081 8082 50051 50052 50053; do
        pid=$(lsof -ti :$port 2>/dev/null)
        if [ ! -z "$pid" ]; then
            echo "  Killing process on port $port (PID: $pid)"
            kill -9 $pid 2>/dev/null || true
        fi
    done

    exit 0
}

# Set up signal handlers for graceful shutdown
trap cleanup SIGINT SIGTERM

# Kill any existing processes on the ports we need
echo "Checking for existing processes on required ports..."
for port in 8080 8081 8082 50051 50052 50053; do
    pid=$(lsof -ti :$port 2>/dev/null)
    if [ ! -z "$pid" ]; then
        echo "  Killing existing process on port $port (PID: $pid)"
        kill -9 $pid 2>/dev/null || true
        sleep 1
    fi
done
echo "Port cleanup complete."
echo ""

# Function to check if a service port is open
check_service_port() {
    local host=$1
    local port=$2
    local service_name=$3

    echo "Waiting for $service_name to be available on $host:$port..."
    while ! nc -z "$host" "$port" 2>/dev/null; do
        echo "  $service_name not available yet, waiting 2 seconds..."
        sleep 2
    done
    echo "  $service_name port is open!"
}

# Function to check if a service is ready using readiness probe
check_service_ready() {
    local grpc_host=$1
    local grpc_port=$2
    local health_port=$3
    local service_name=$4

    echo "Checking $service_name readiness (Health port: $health_port)..."

    # First wait for the health check port to be open
    local max_port_wait=30
    local port_wait_count=0
    while ! nc -z "$grpc_host" "$health_port" 2>/dev/null; do
        port_wait_count=$((port_wait_count + 1))
        if [ $port_wait_count -ge $max_port_wait ]; then
            echo "  ERROR: $service_name health endpoint not available after ${max_port_wait} attempts"
            return 1
        fi
        sleep 2
    done

    # Then check the readiness probe endpoint
    local max_ready_wait=60
    local ready_wait_count=0
    while true; do
        ready_wait_count=$((ready_wait_count + 1))
        if [ $ready_wait_count -ge $max_ready_wait ]; then
            echo "  ERROR: $service_name readiness probe never returned 200 after ${max_ready_wait} attempts"
            return 1
        fi

        # Use curl with timeout and capture the response
        http_code=$(curl -s -o /dev/null -w "%{http_code}" --http1.1 --max-time 5 "http://$grpc_host:$health_port/ready" 2>/dev/null || echo "000")

        if [ "$http_code" = "200" ]; then
            echo "  $service_name is ready!"
            break
        elif [ "$http_code" = "503" ]; then
            # Service not ready yet, keep waiting
            sleep 2
        elif [ "$http_code" = "000" ]; then
            echo "  Connection failed, retrying..."
            sleep 2
        else
            echo "  Unexpected HTTP code: $http_code, retrying..."
            sleep 2
        fi
    done
}

echo "Starting worker node 1..."
HEALTH_PORT=8081 cargo run --bin worker -- 0.0.0.0 50052 > worker1.log 2>&1 &
WORKER1_PID=$!

echo "Starting worker node 2..."
HEALTH_PORT=8082 cargo run --bin worker -- 0.0.0.0 50053 > worker2.log 2>&1 &
WORKER2_PID=$!

# Wait for both workers' gRPC ports to be open (simple port check for bootstrap)
check_service_port localhost 50052 "worker1"
check_service_port localhost 50053 "worker2"

echo "Both workers are available, starting head node..."
HEALTH_PORT=8080 cargo run --bin head -- 0.0.0.0 50051 > head.log 2>&1 &
HEAD_PID=$!

check_service_port localhost 50051 "head"

echo ""
echo "Cluster bootstrap complete. Verifying all services are ready..."
echo ""

# Final readiness check after cluster bootstrap
echo "Performing final readiness checks..."
check_service_ready localhost 50052 8081 "worker1"
check_service_ready localhost 50053 8082 "worker2"
check_service_ready localhost 50051 8080 "head"

echo ""
echo "All services are running and ready. Press Ctrl+C to stop all services."
wait

#echo "Running client test..."
#cd src/client && cargo run 2>&1
#CLIENT_EXIT_CODE=$?
