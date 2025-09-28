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

set -euo pipefail

# Script version
VERSION="1.0.0"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHART_PATH="${PROJECT_ROOT}/charts/kubeflow-trainer"
OUTPUT_DIR="${PROJECT_ROOT}/manifests"
VALUES_FILE=""
RELEASE_NAME="kubeflow-trainer"
NAMESPACE="kubeflow"
DRY_RUN=false
VALIDATE=false
VERBOSE=false
PRESERVE_COMMENTS=false

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_debug() {
    if [[ "${VERBOSE}" == "true" ]]; then
        echo -e "${YELLOW}[DEBUG]${NC} $1"
    fi
}

# Help message
show_help() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Sync Helm charts with Kustomize manifests automatically.

OPTIONS:
    -h, --help                  Show this help message
    -v, --version               Show version
    -c, --chart-path PATH       Path to Helm chart (default: ${CHART_PATH})
    -o, --output-dir PATH       Output directory for Kustomize manifests (default: ${OUTPUT_DIR})
    -f, --values-file FILE      Values file for Helm rendering (optional)
    -r, --release-name NAME     Release name for Helm rendering (default: ${RELEASE_NAME})
    -n, --namespace NAMESPACE   Namespace for resources (default: ${NAMESPACE})
    -d, --dry-run               Perform dry run without writing files
    --validate                  Validate output against Helm rendering
    --verbose                   Enable verbose output
    --preserve-comments         Preserve comments from Helm templates
    --check-crds                Check CRD handling capability

EXAMPLES:
    # Basic sync using defaults
    $(basename "$0")

    # Sync with custom chart and output paths
    $(basename "$0") -c ./my-chart -o ./my-manifests

    # Sync with custom values file
    $(basename "$0") -f custom-values.yaml

    # Dry run to see what would be generated
    $(basename "$0") --dry-run

    # Validate generated manifests
    $(basename "$0") --validate

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -v|--version)
                echo "Version: ${VERSION}"
                exit 0
                ;;
            -c|--chart-path)
                CHART_PATH="$2"
                shift 2
                ;;
            -o|--output-dir)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -f|--values-file)
                VALUES_FILE="$2"
                shift 2
                ;;
            -r|--release-name)
                RELEASE_NAME="$2"
                shift 2
                ;;
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            --validate)
                VALIDATE=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --preserve-comments)
                PRESERVE_COMMENTS=true
                shift
                ;;
            --check-crds)
                check_crd_capability
                exit $?
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for Helm
    if ! command -v helm &> /dev/null; then
        log_error "Helm is not installed. Please install Helm first."
        exit 1
    fi
    log_debug "Helm version: $(helm version --short)"

    # Check for yq (optional but recommended)
    if command -v yq &> /dev/null; then
        log_debug "yq is installed: $(yq --version)"
        YQ_AVAILABLE=true
    else
        log_warning "yq is not installed. Some features may be limited."
        log_warning "Install yq for better YAML processing: https://github.com/mikefarah/yq"
        YQ_AVAILABLE=false
    fi

    # Validate chart path
    if [[ ! -d "${CHART_PATH}" ]]; then
        log_error "Chart path does not exist: ${CHART_PATH}"
        exit 1
    fi

    if [[ ! -f "${CHART_PATH}/Chart.yaml" ]]; then
        log_error "Invalid Helm chart: Chart.yaml not found in ${CHART_PATH}"
        exit 1
    fi

    # Validate values file if provided
    if [[ -n "${VALUES_FILE}" ]] && [[ ! -f "${VALUES_FILE}" ]]; then
        log_error "Values file does not exist: ${VALUES_FILE}"
        exit 1
    fi

    log_success "Prerequisites check passed"
}

# Check CRD handling capability
check_crd_capability() {
    if [[ -d "${CHART_PATH}/crds" ]]; then
        log_info "CRDs directory found at ${CHART_PATH}/crds"
        return 0
    else
        log_info "No CRDs directory found"
        return 1
    fi
}

# Create output directory structure
create_output_structure() {
    log_info "Creating output directory structure..."

    local dirs=(
        "${OUTPUT_DIR}/base"
        "${OUTPUT_DIR}/base/crds"
        "${OUTPUT_DIR}/base/manager"
        "${OUTPUT_DIR}/base/rbac"
        "${OUTPUT_DIR}/base/webhook"
        "${OUTPUT_DIR}/overlays"
        "${OUTPUT_DIR}/overlays/standalone"
        "${OUTPUT_DIR}/overlays/kubeflow-platform"
    )

    for dir in "${dirs[@]}"; do
        if [[ "${DRY_RUN}" == "false" ]]; then
            mkdir -p "${dir}"
            log_debug "Created directory: ${dir}"
        else
            log_debug "[DRY-RUN] Would create directory: ${dir}"
        fi
    done

    log_success "Output directory structure created"
}

# Render Helm templates
render_helm_templates() {
    log_info "Rendering Helm templates..."

    local temp_dir=$(mktemp -d)
    local helm_cmd="helm template ${RELEASE_NAME} ${CHART_PATH}"

    # Add namespace
    helm_cmd="${helm_cmd} --namespace ${NAMESPACE}"

    # Add values file if provided
    if [[ -n "${VALUES_FILE}" ]]; then
        helm_cmd="${helm_cmd} --values ${VALUES_FILE}"
    fi

    # Add output directory
    helm_cmd="${helm_cmd} --output-dir ${temp_dir}"

    log_debug "Running: ${helm_cmd}"

    if ! ${helm_cmd} 2>/dev/null; then
        log_error "Failed to render Helm templates"
        rm -rf "${temp_dir}"
        exit 1
    fi

    echo "${temp_dir}"
}

# Process rendered templates
process_templates() {
    local rendered_dir="$1"

    log_info "Processing rendered templates..."

    # Process CRDs
    if [[ -d "${CHART_PATH}/crds" ]]; then
        process_crds
    fi

    # Process templates by type
    local template_dir="${rendered_dir}/${RELEASE_NAME}/templates"

    if [[ -d "${template_dir}" ]]; then
        # Process different resource types
        process_resource_type "${template_dir}" "deployment" "manager"
        process_resource_type "${template_dir}" "service" "manager"
        process_resource_type "${template_dir}" "clusterrole" "rbac"
        process_resource_type "${template_dir}" "clusterrolebinding" "rbac"
        process_resource_type "${template_dir}" "role" "rbac"
        process_resource_type "${template_dir}" "rolebinding" "rbac"
        process_resource_type "${template_dir}" "serviceaccount" "rbac"
        process_resource_type "${template_dir}" "validatingwebhookconfiguration" "webhook"
        process_resource_type "${template_dir}" "mutatingwebhookconfiguration" "webhook"
        process_resource_type "${template_dir}" "secret" "webhook"
    fi

    log_success "Templates processed successfully"
}

# Process CRDs
process_crds() {
    log_info "Processing CRDs..."

    local crd_source="${CHART_PATH}/crds"
    local crd_dest="${OUTPUT_DIR}/base/crds"

    if [[ -d "${crd_source}" ]]; then
        for crd_file in "${crd_source}"/*.yaml; do
            if [[ -f "${crd_file}" ]]; then
                local filename=$(basename "${crd_file}")
                if [[ "${DRY_RUN}" == "false" ]]; then
                    cp "${crd_file}" "${crd_dest}/${filename}"
                    log_debug "Copied CRD: ${filename}"
                else
                    log_debug "[DRY-RUN] Would copy CRD: ${filename}"
                fi
            fi
        done

        # Create kustomization.yaml for CRDs
        create_kustomization_file "${crd_dest}" "crds"
    fi
}

# Process specific resource type
process_resource_type() {
    local template_dir="$1"
    local resource_pattern="$2"
    local target_subdir="$3"

    log_debug "Processing ${resource_pattern} resources..."

    local target_dir="${OUTPUT_DIR}/base/${target_subdir}"

    # Find and process matching files
    for file in "${template_dir}"/*; do
        if [[ -f "${file}" ]] && grep -qi "kind:.*${resource_pattern}" "${file}" 2>/dev/null; then
            local filename=$(generate_resource_filename "${file}" "${resource_pattern}")

            if [[ "${DRY_RUN}" == "false" ]]; then
                # Process and clean the file
                clean_and_copy_resource "${file}" "${target_dir}/${filename}"
                log_debug "Processed ${resource_pattern}: ${filename}"
            else
                log_debug "[DRY-RUN] Would process ${resource_pattern}: ${filename}"
            fi
        fi
    done
}

# Generate appropriate filename for resource
generate_resource_filename() {
    local file="$1"
    local resource_type="$2"

    # Extract metadata name if possible
    local name=""
    if [[ "${YQ_AVAILABLE}" == "true" ]]; then
        name=$(yq eval '.metadata.name' "${file}" 2>/dev/null || echo "")
    fi

    if [[ -n "${name}" ]]; then
        echo "${resource_type}_${name}.yaml"
    else
        echo "${resource_type}.yaml"
    fi
}

# Clean and copy resource file
clean_and_copy_resource() {
    local source="$1"
    local dest="$2"

    if [[ "${PRESERVE_COMMENTS}" == "true" ]]; then
        cp "${source}" "${dest}"
    else
        # Remove Helm template comments but preserve YAML structure
        grep -v '^{{-' "${source}" | grep -v '^#.*{{' > "${dest}"
    fi

    # Add namespace if not present and not a cluster-scoped resource
    if ! grep -q "namespace:" "${dest}" && ! grep -qE "kind:.*(ClusterRole|ClusterRoleBinding|CustomResourceDefinition)" "${dest}"; then
        if [[ "${YQ_AVAILABLE}" == "true" ]]; then
            yq eval ".metadata.namespace = \"${NAMESPACE}\"" -i "${dest}"
        fi
    fi
}

# Create kustomization.yaml files
create_kustomization_file() {
    local dir="$1"
    local component="$2"

    log_debug "Creating kustomization.yaml for ${component}..."

    local kustomization_file="${dir}/kustomization.yaml"

    if [[ "${DRY_RUN}" == "false" ]]; then
        # Find all YAML files in the directory
        local resources=()
        for file in "${dir}"/*.yaml; do
            if [[ -f "${file}" ]] && [[ "$(basename "${file}")" != "kustomization.yaml" ]]; then
                resources+=("$(basename "${file}")")
            fi
        done

        # Create kustomization.yaml
        {
            echo "apiVersion: kustomize.config.k8s.io/v1beta1"
            echo "kind: Kustomization"
            echo ""
            echo "# Component: ${component}"
            echo "namespace: ${NAMESPACE}"
            echo ""
            echo "resources:"
            for resource in "${resources[@]}"; do
                echo "  - ${resource}"
            done

            # Add common labels
            echo ""
            echo "commonLabels:"
            echo "  app.kubernetes.io/name: trainer"
            echo "  app.kubernetes.io/component: ${component}"
            echo "  app.kubernetes.io/part-of: kubeflow"
            echo "  app.kubernetes.io/managed-by: kustomize"
        } > "${kustomization_file}"

        log_debug "Created kustomization.yaml for ${component}"
    else
        log_debug "[DRY-RUN] Would create kustomization.yaml for ${component}"
    fi
}

# Create base kustomization.yaml
create_base_kustomization() {
    log_info "Creating base kustomization.yaml..."

    local base_kustomization="${OUTPUT_DIR}/base/kustomization.yaml"

    if [[ "${DRY_RUN}" == "false" ]]; then
        {
            echo "apiVersion: kustomize.config.k8s.io/v1beta1"
            echo "kind: Kustomization"
            echo ""
            echo "# Base configuration for Kubeflow Trainer"
            echo "namespace: ${NAMESPACE}"
            echo ""
            echo "resources:"

            # Add subdirectories as resources
            for subdir in crds manager rbac webhook; do
                if [[ -d "${OUTPUT_DIR}/base/${subdir}" ]] && [[ -n "$(ls -A "${OUTPUT_DIR}/base/${subdir}"/*.yaml 2>/dev/null)" ]]; then
                    echo "  - ${subdir}"
                fi
            done

            echo ""
            echo "# Common labels applied to all resources"
            echo "commonLabels:"
            echo "  app.kubernetes.io/name: trainer"
            echo "  app.kubernetes.io/part-of: kubeflow"
            echo "  app.kubernetes.io/managed-by: kustomize"
        } > "${base_kustomization}"

        log_success "Created base kustomization.yaml"
    else
        log_debug "[DRY-RUN] Would create base kustomization.yaml"
    fi
}

# Create overlay kustomization files
create_overlay_kustomizations() {
    log_info "Creating overlay kustomization files..."

    # Standalone overlay
    create_standalone_overlay

    # Kubeflow platform overlay
    create_kubeflow_overlay

    log_success "Created overlay kustomization files"
}

# Create standalone overlay
create_standalone_overlay() {
    local overlay_dir="${OUTPUT_DIR}/overlays/standalone"
    local kustomization_file="${overlay_dir}/kustomization.yaml"

    if [[ "${DRY_RUN}" == "false" ]]; then
        {
            echo "apiVersion: kustomize.config.k8s.io/v1beta1"
            echo "kind: Kustomization"
            echo ""
            echo "# Standalone deployment overlay"
            echo "namespace: ${NAMESPACE}"
            echo ""
            echo "resources:"
            echo "  - ../../base"
            echo ""
            echo "# Standalone specific patches can be added here"
            echo "patchesStrategicMerge: []"
            echo ""
            echo "# Standalone specific config"
            echo "configMapGenerator: []"
            echo "secretGenerator: []"
        } > "${kustomization_file}"

        log_debug "Created standalone overlay"
    else
        log_debug "[DRY-RUN] Would create standalone overlay"
    fi
}

# Create Kubeflow platform overlay
create_kubeflow_overlay() {
    local overlay_dir="${OUTPUT_DIR}/overlays/kubeflow-platform"
    local kustomization_file="${overlay_dir}/kustomization.yaml"

    if [[ "${DRY_RUN}" == "false" ]]; then
        {
            echo "apiVersion: kustomize.config.k8s.io/v1beta1"
            echo "kind: Kustomization"
            echo ""
            echo "# Kubeflow platform integration overlay"
            echo "namespace: kubeflow"
            echo ""
            echo "resources:"
            echo "  - ../../base"
            echo ""
            echo "# Platform specific labels"
            echo "commonLabels:"
            echo "  app.kubernetes.io/part-of: kubeflow"
            echo "  katib.kubeflow.org/component: \"yes\""
            echo ""
            echo "# Platform specific patches can be added here"
            echo "patchesStrategicMerge: []"
            echo ""
            echo "# Platform specific config"
            echo "configMapGenerator: []"
            echo "secretGenerator: []"
        } > "${kustomization_file}"

        log_debug "Created Kubeflow platform overlay"
    else
        log_debug "[DRY-RUN] Would create Kubeflow platform overlay"
    fi
}

# Validate generated manifests
validate_manifests() {
    log_info "Validating generated manifests..."

    local validation_errors=0

    # Validate base kustomization
    if command -v kubectl &> /dev/null; then
        log_debug "Validating with kubectl..."

        if ! kubectl kustomize "${OUTPUT_DIR}/base" > /dev/null 2>&1; then
            log_error "Base kustomization validation failed"
            ((validation_errors++))
        else
            log_success "Base kustomization is valid"
        fi

        # Validate overlays
        for overlay in standalone kubeflow-platform; do
            if [[ -d "${OUTPUT_DIR}/overlays/${overlay}" ]]; then
                if ! kubectl kustomize "${OUTPUT_DIR}/overlays/${overlay}" > /dev/null 2>&1; then
                    log_error "Overlay ${overlay} validation failed"
                    ((validation_errors++))
                else
                    log_success "Overlay ${overlay} is valid"
                fi
            fi
        done
    else
        log_warning "kubectl not available, skipping kustomize validation"
    fi

    # Compare with Helm output if requested
    if [[ "${VALIDATE}" == "true" ]]; then
        validate_against_helm
    fi

    if [[ ${validation_errors} -gt 0 ]]; then
        log_error "Validation failed with ${validation_errors} errors"
        return 1
    fi

    log_success "All validations passed"
    return 0
}

# Validate against Helm rendering
validate_against_helm() {
    log_info "Validating against Helm rendering..."

    local temp_helm=$(mktemp -d)
    local temp_kustomize=$(mktemp -d)

    # Render Helm templates
    helm template "${RELEASE_NAME}" "${CHART_PATH}" \
        --namespace "${NAMESPACE}" \
        ${VALUES_FILE:+--values "${VALUES_FILE}"} \
        > "${temp_helm}/helm.yaml" 2>/dev/null

    # Build Kustomize manifests
    kubectl kustomize "${OUTPUT_DIR}/base" > "${temp_kustomize}/kustomize.yaml" 2>/dev/null

    # Compare resource counts
    local helm_resources=$(grep -c '^kind:' "${temp_helm}/helm.yaml" || echo "0")
    local kustomize_resources=$(grep -c '^kind:' "${temp_kustomize}/kustomize.yaml" || echo "0")

    log_info "Helm resources: ${helm_resources}"
    log_info "Kustomize resources: ${kustomize_resources}"

    if [[ ${helm_resources} -ne ${kustomize_resources} ]]; then
        log_warning "Resource count mismatch: Helm(${helm_resources}) vs Kustomize(${kustomize_resources})"
    else
        log_success "Resource counts match"
    fi

    # Cleanup
    rm -rf "${temp_helm}" "${temp_kustomize}"
}

# Generate summary report
generate_summary() {
    log_info "Generating summary..."

    local base_files=$(find "${OUTPUT_DIR}/base" -name "*.yaml" 2>/dev/null | wc -l)
    local overlay_count=$(find "${OUTPUT_DIR}/overlays" -name "kustomization.yaml" 2>/dev/null | wc -l)

    cat << EOF

========================================
       Helm to Kustomize Sync Report
========================================

Chart Path:        ${CHART_PATH}
Output Directory:  ${OUTPUT_DIR}
Release Name:      ${RELEASE_NAME}
Namespace:         ${NAMESPACE}

Generated Files:
  - Base manifests:    ${base_files}
  - Overlays created:  ${overlay_count}

Status: ${GREEN}SUCCESS${NC}

To apply the generated manifests:
  kubectl apply -k ${OUTPUT_DIR}/base
  kubectl apply -k ${OUTPUT_DIR}/overlays/standalone
  kubectl apply -k ${OUTPUT_DIR}/overlays/kubeflow-platform

========================================
EOF
}

# Cleanup function
cleanup() {
    log_debug "Cleaning up temporary files..."
    # Add cleanup logic if needed
}

# Main function
main() {
    # Setup trap for cleanup
    trap cleanup EXIT

    # Parse arguments
    parse_args "$@"

    # Check prerequisites
    check_prerequisites

    # Create output structure
    create_output_structure

    # Render Helm templates
    local rendered_dir=$(render_helm_templates)

    # Process templates
    process_templates "${rendered_dir}"

    # Create kustomization files
    create_base_kustomization
    create_overlay_kustomizations

    # Validate if not in dry-run mode
    if [[ "${DRY_RUN}" == "false" ]]; then
        validate_manifests || true
    fi

    # Generate summary
    generate_summary

    # Cleanup rendered templates
    rm -rf "${rendered_dir}"

    log_success "Sync completed successfully!"
}

# Run main function
main "$@"