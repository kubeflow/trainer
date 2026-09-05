#!/bin/bash

# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Runs a data cache cluster against an Iceberg table on the local filesystem.
#
# This is the counterpart of run_with_remote_table.sh and needs no AWS account:
# the Iceberg table is generated locally and loaded through the same storage-fs
# backend the cache already supports.
#
# Usage: $0 [warehouse-dir] [rows-per-file ...]
# Example: $0 /tmp/kubeflow-data-cache 3 2

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRATE_DIR="$(cd "${SCRIPT_DIR}/../../pkg/data_cache" && pwd)"

WAREHOUSE_DIR="${1:-/tmp/kubeflow-data-cache}"
shift || true
ROWS_PER_FILE=("$@")
if [ ${#ROWS_PER_FILE[@]} -eq 0 ]; then
    ROWS_PER_FILE=(3 2)
fi

cd "${CRATE_DIR}"

echo "Warehouse directory: ${WAREHOUSE_DIR}"
echo "Rows per data file: ${ROWS_PER_FILE[*]}"
echo ""

# Start from a clean warehouse: Iceberg metadata records absolute paths, so a
# stale table from a previous run would still reference its old data files.
rm -rf "${WAREHOUSE_DIR}"
mkdir -p "${WAREHOUSE_DIR}"

echo "Generating local Iceberg table..."
TABLE_ENV=$(cargo run -q --features local-fixtures --example create_local_table -- \
    "${WAREHOUSE_DIR}" "${ROWS_PER_FILE[@]}")
eval "${TABLE_ENV}"

# RUNTIME_ENV=LOCAL makes the head node look for workers on localhost instead of
# resolving LeaderWorkerSet pod addresses.
export RUNTIME_ENV="${RUNTIME_ENV:-LOCAL}"

echo "METADATA_LOC: ${METADATA_LOC}"
echo "SCHEMA_NAME:  ${SCHEMA_NAME}"
echo "TABLE_NAME:   ${TABLE_NAME}"
echo ""

# Function to cleanup processes on exit
cleanup() {
    echo ""
    echo "Stopping services..."
    kill -9 $WORKER1_PID $WORKER2_PID $HEAD_PID 2>/dev/null || true
    wait $WORKER1_PID $WORKER2_PID $HEAD_PID 2>/dev/null || true

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
    local health_port=$2
    local service_name=$3

    echo "Checking $service_name readiness (Health port: $health_port)..."

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

    local max_ready_wait=60
    local ready_wait_count=0
    while true; do
        ready_wait_count=$((ready_wait_count + 1))
        if [ $ready_wait_count -ge $max_ready_wait ]; then
            echo "  ERROR: $service_name readiness probe never returned 200 after ${max_ready_wait} attempts"
            return 1
        fi

        http_code=$(curl -s -o /dev/null -w "%{http_code}" --http1.1 --max-time 5 "http://$grpc_host:$health_port/ready" 2>/dev/null || echo "000")

        if [ "$http_code" = "200" ]; then
            echo "  $service_name is ready!"
            break
        elif [ "$http_code" = "503" ]; then
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

check_service_port localhost 50052 "worker1"
check_service_port localhost 50053 "worker2"

echo "Both workers are available, starting head node..."
HEALTH_PORT=8080 cargo run --bin head -- 0.0.0.0 50051 > head.log 2>&1 &
HEAD_PID=$!

check_service_port localhost 50051 "head"

echo ""
echo "Cluster bootstrap complete. Verifying all services are ready..."
echo ""

check_service_ready localhost 8081 "worker1"
check_service_ready localhost 8082 "worker2"
check_service_ready localhost 8080 "head"

echo ""
echo "All services are running and ready."
echo "Stream the cached rows with:"
echo "  cd ${CRATE_DIR}/test && cargo run --bin client -- --endpoint http://localhost:50051 --local-rank 1 --world-size 2"
echo ""
echo "Press Ctrl+C to stop all services."
wait
