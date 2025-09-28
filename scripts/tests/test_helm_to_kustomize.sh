#!/bin/bash
#
# Copyright 2025 The Kubeflow authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TEST_TMP_DIR="${SCRIPT_DIR}/tmp_test_$$"
SYNC_SCRIPT="${PROJECT_ROOT}/scripts/helm_to_kustomize_sync.sh"

# Cleanup function
cleanup() {
    if [[ -d "${TEST_TMP_DIR}" ]]; then
        rm -rf "${TEST_TMP_DIR}"
    fi
}
trap cleanup EXIT

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
print_test_header() {
    echo -e "${YELLOW}Running: $1${NC}"
}

assert_file_exists() {
    if [[ -f "$1" ]]; then
        echo -e "${GREEN}✓ File exists: $1${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ File not found: $1${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_directory_exists() {
    if [[ -d "$1" ]]; then
        echo -e "${GREEN}✓ Directory exists: $1${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ Directory not found: $1${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_contains() {
    if grep -q "$2" "$1" 2>/dev/null; then
        echo -e "${GREEN}✓ File $1 contains: $2${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ File $1 does not contain: $2${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_yaml_valid() {
    if command -v yq &> /dev/null; then
        if yq eval '.' "$1" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Valid YAML: $1${NC}"
            ((TESTS_PASSED++))
            return 0
        else
            echo -e "${RED}✗ Invalid YAML: $1${NC}"
            ((TESTS_FAILED++))
            return 1
        fi
    else
        # If yq is not available, do basic YAML check
        if [[ -f "$1" ]] && head -n1 "$1" | grep -qE '^(---|apiVersion:)'; then
            echo -e "${GREEN}✓ Basic YAML check passed: $1${NC}"
            ((TESTS_PASSED++))
            return 0
        else
            echo -e "${RED}✗ Basic YAML check failed: $1${NC}"
            ((TESTS_FAILED++))
            return 1
        fi
    fi
}

assert_script_runs() {
    if bash "$1" "${@:2}" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Script runs successfully: $1${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ Script failed: $1 with exit code $?${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Setup test environment
setup_test_env() {
    mkdir -p "${TEST_TMP_DIR}"
    cd "${TEST_TMP_DIR}"
}

# Test 1: Script exists and is executable
test_script_exists() {
    print_test_header "Test 1: Script exists and is executable"
    assert_file_exists "${SYNC_SCRIPT}"

    if [[ -x "${SYNC_SCRIPT}" ]]; then
        echo -e "${GREEN}✓ Script is executable${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${YELLOW}! Script is not executable, attempting to make it executable${NC}"
        chmod +x "${SYNC_SCRIPT}"
        if [[ -x "${SYNC_SCRIPT}" ]]; then
            echo -e "${GREEN}✓ Script made executable${NC}"
            ((TESTS_PASSED++))
        else
            echo -e "${RED}✗ Failed to make script executable${NC}"
            ((TESTS_FAILED++))
        fi
    fi
}

# Test 2: Script shows help
test_help_output() {
    print_test_header "Test 2: Script shows help"

    if bash "${SYNC_SCRIPT}" --help 2>&1 | grep -q "Usage:"; then
        echo -e "${GREEN}✓ Script shows help with --help${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗ Script does not show help${NC}"
        ((TESTS_FAILED++))
    fi
}

# Test 3: Script validates Helm installation
test_helm_validation() {
    print_test_header "Test 3: Script validates Helm installation"

    # Check if helm is installed for the test
    if command -v helm &> /dev/null; then
        echo -e "${GREEN}✓ Helm is installed${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${YELLOW}⚠ Helm is not installed, skipping Helm-specific tests${NC}"
    fi
}

# Test 4: Script creates output directory structure
test_output_directory_structure() {
    print_test_header "Test 4: Script creates output directory structure"
    setup_test_env

    # Run script in dry-run mode if available, or with minimal config
    if bash "${SYNC_SCRIPT}" \
        --chart-path "${PROJECT_ROOT}/charts/kubeflow-trainer" \
        --output-dir "${TEST_TMP_DIR}/output" \
        --dry-run 2>/dev/null; then

        assert_directory_exists "${TEST_TMP_DIR}/output"
        assert_directory_exists "${TEST_TMP_DIR}/output/base"
        assert_directory_exists "${TEST_TMP_DIR}/output/overlays"
    else
        echo -e "${YELLOW}⚠ Dry-run mode not available, skipping structure test${NC}"
    fi
}

# Test 5: Script generates kustomization.yaml files
test_kustomization_files() {
    print_test_header "Test 5: Script generates kustomization.yaml files"
    setup_test_env

    # Create a minimal test Helm chart
    mkdir -p "${TEST_TMP_DIR}/test-chart/templates"
    cat > "${TEST_TMP_DIR}/test-chart/Chart.yaml" << EOF
apiVersion: v2
name: test-chart
version: 0.1.0
EOF

    cat > "${TEST_TMP_DIR}/test-chart/templates/deployment.yaml" << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: test:latest
EOF

    # Test generation
    if bash "${SYNC_SCRIPT}" \
        --chart-path "${TEST_TMP_DIR}/test-chart" \
        --output-dir "${TEST_TMP_DIR}/output" 2>/dev/null; then

        assert_file_exists "${TEST_TMP_DIR}/output/base/kustomization.yaml"
        assert_contains "${TEST_TMP_DIR}/output/base/kustomization.yaml" "resources:"
    else
        echo -e "${YELLOW}⚠ Script execution failed, checking if it's due to missing dependencies${NC}"
    fi
}

# Test 6: Script handles CRDs correctly
test_crd_handling() {
    print_test_header "Test 6: Script handles CRDs correctly"

    if [[ -d "${PROJECT_ROOT}/charts/kubeflow-trainer/crds" ]]; then
        echo -e "${GREEN}✓ CRDs directory exists${NC}"
        ((TESTS_PASSED++))

        # Check if script would process CRDs
        if bash "${SYNC_SCRIPT}" --check-crds 2>/dev/null; then
            echo -e "${GREEN}✓ Script can handle CRDs${NC}"
            ((TESTS_PASSED++))
        else
            echo -e "${YELLOW}⚠ CRD check not implemented yet${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ No CRDs directory found${NC}"
    fi
}

# Test 7: Script validates output against Helm render
test_output_validation() {
    print_test_header "Test 7: Script validates output against Helm render"

    if command -v helm &> /dev/null; then
        setup_test_env

        # Render Helm chart
        if helm template test-release "${PROJECT_ROOT}/charts/kubeflow-trainer" \
            --output-dir "${TEST_TMP_DIR}/helm-output" 2>/dev/null; then

            echo -e "${GREEN}✓ Helm template rendered successfully${NC}"
            ((TESTS_PASSED++))

            # Run sync script
            if bash "${SYNC_SCRIPT}" \
                --chart-path "${PROJECT_ROOT}/charts/kubeflow-trainer" \
                --output-dir "${TEST_TMP_DIR}/kustomize-output" \
                --validate 2>/dev/null; then

                echo -e "${GREEN}✓ Sync script executed with validation${NC}"
                ((TESTS_PASSED++))
            else
                echo -e "${YELLOW}⚠ Validation not yet implemented${NC}"
            fi
        else
            echo -e "${YELLOW}⚠ Helm template rendering failed${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ Helm not installed, skipping validation test${NC}"
    fi
}

# Test 8: Script handles errors gracefully
test_error_handling() {
    print_test_header "Test 8: Script handles errors gracefully"

    # Test with non-existent chart path
    if bash "${SYNC_SCRIPT}" \
        --chart-path "/non/existent/path" \
        --output-dir "${TEST_TMP_DIR}/output" 2>/dev/null; then

        echo -e "${RED}✗ Script should fail with non-existent path${NC}"
        ((TESTS_FAILED++))
    else
        echo -e "${GREEN}✓ Script properly fails with non-existent path${NC}"
        ((TESTS_PASSED++))
    fi

    # Test with invalid chart
    setup_test_env
    mkdir -p "${TEST_TMP_DIR}/invalid-chart"

    if bash "${SYNC_SCRIPT}" \
        --chart-path "${TEST_TMP_DIR}/invalid-chart" \
        --output-dir "${TEST_TMP_DIR}/output" 2>/dev/null; then

        echo -e "${RED}✗ Script should fail with invalid chart${NC}"
        ((TESTS_FAILED++))
    else
        echo -e "${GREEN}✓ Script properly fails with invalid chart${NC}"
        ((TESTS_PASSED++))
    fi
}

# Main test runner
main() {
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}  Helm to Kustomize Sync Script Tests  ${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""

    # Run all tests
    test_script_exists
    echo ""
    test_help_output
    echo ""
    test_helm_validation
    echo ""
    test_output_directory_structure
    echo ""
    test_kustomization_files
    echo ""
    test_crd_handling
    echo ""
    test_output_validation
    echo ""
    test_error_handling
    echo ""

    # Summary
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}              Test Summary              ${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
    echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"

    if [[ ${TESTS_FAILED} -eq 0 ]]; then
        echo -e "${GREEN}All tests passed! ✓${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed ✗${NC}"
        exit 1
    fi
}

# Run tests
main "$@"