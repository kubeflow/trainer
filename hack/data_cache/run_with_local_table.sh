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

#!/bin/bash
# Start data-cache head and workers against a local on-disk Iceberg table (file://).
#
# Usage:
#   ./hack/data_cache/run_with_local_table.sh [environment]
#
# If the fixture is missing, runs hack/data_cache/generate_local_iceberg_fixture.py first.
# Requires: Rust/cargo, nc, curl, python3 + pyiceberg/pyarrow/sqlalchemy for generation.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
FIXTURE_DIR="${REPO_ROOT}/pkg/data_cache/testdata/local_iceberg"
METADATA_DIR="${FIXTURE_DIR}/warehouse/local/demo/metadata"
ENVIRONMENT="${1:-LOCAL}"

latest_metadata_file() {
  ls -1 "${METADATA_DIR}"/*.metadata.json 2>/dev/null | sort | tail -1
}

ensure_fixture() {
  if [[ -z "$(latest_metadata_file || true)" ]]; then
    echo "Local Iceberg fixture not found; generating..."
    python3 "${SCRIPT_DIR}/generate_local_iceberg_fixture.py"
  fi
  local meta
  meta="$(latest_metadata_file)"
  if [[ -z "${meta}" ]]; then
    echo "ERROR: failed to find metadata file under ${METADATA_DIR}" >&2
    exit 1
  fi
  python3 -c "import pathlib; print(pathlib.Path('${meta}').resolve().as_uri())"
}

METADATA_LOC="$(ensure_fixture)"
TABLE_NAME="demo"
SCHEMA_NAME="local"

echo "Metadata Location: ${METADATA_LOC}"
echo "Table Name: ${TABLE_NAME}"
echo "Schema Name: ${SCHEMA_NAME}"
echo "Environment: ${ENVIRONMENT}"

export METADATA_LOC
export TABLE_NAME
export SCHEMA_NAME
export RUNTIME_ENV="${ENVIRONMENT}"

cleanup() {
  echo ""
  echo "Stopping services..."
  kill -9 "${WORKER1_PID:-}" "${WORKER2_PID:-}" "${HEAD_PID:-}" 2>/dev/null || true
  wait "${WORKER1_PID:-}" "${WORKER2_PID:-}" "${HEAD_PID:-}" 2>/dev/null || true
  for port in 8080 8081 8082 50051 50052 50053; do
    pid=$(lsof -ti :"${port}" 2>/dev/null || true)
    if [[ -n "${pid}" ]]; then
      kill -9 "${pid}" 2>/dev/null || true
    fi
  done
  exit 0
}

trap cleanup SIGINT SIGTERM

cd "${REPO_ROOT}/pkg/data_cache"

for port in 8080 8081 8082 50051 50052 50053; do
  pid=$(lsof -ti :"${port}" 2>/dev/null || true)
  if [[ -n "${pid}" ]]; then
    kill -9 "${pid}" 2>/dev/null || true
    sleep 1
  fi
done

check_service_port() {
  local host=$1 port=$2 name=$3
  echo "Waiting for ${name} on ${host}:${port}..."
  while ! nc -z "${host}" "${port}" 2>/dev/null; do
    sleep 2
  done
}

check_service_ready() {
  local host=$1 health_port=$2 name=$3
  echo "Checking ${name} readiness on :${health_port}..."
  local count=0
  while [[ ${count} -lt 60 ]]; do
    http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "http://${host}:${health_port}/ready" 2>/dev/null || echo "000")
    if [[ "${http_code}" == "200" ]]; then
      echo "  ${name} is ready"
      return 0
    fi
    count=$((count + 1))
    sleep 2
  done
  echo "ERROR: ${name} not ready" >&2
  return 1
}

echo "Starting worker 1..."
HEALTH_PORT=8081 cargo run --bin worker -- 0.0.0.0 50052 >"${REPO_ROOT}/worker1.log" 2>&1 &
WORKER1_PID=$!

echo "Starting worker 2..."
HEALTH_PORT=8082 cargo run --bin worker -- 0.0.0.0 50053 >"${REPO_ROOT}/worker2.log" 2>&1 &
WORKER2_PID=$!

check_service_port localhost 50052 worker1
check_service_port localhost 50053 worker2

echo "Starting head..."
HEALTH_PORT=8080 cargo run --bin head -- 0.0.0.0 50051 >"${REPO_ROOT}/head.log" 2>&1 &
HEAD_PID=$!

check_service_port localhost 50051 head
check_service_ready localhost 8081 worker1
check_service_ready localhost 8082 worker2
check_service_ready localhost 8080 head

echo ""
echo "All services ready. Example client:"
echo "  cd pkg/data_cache/test && cargo run -- --endpoint http://localhost:50051 --local-rank 0 --world-size 2"
echo "Press Ctrl+C to stop."
wait
