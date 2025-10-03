# Helm to Kustomize Sync Documentation

## Overview

This document describes the automated synchronization system between Helm charts and Kustomize manifests for the Kubeflow Trainer project. The system ensures that both deployment methods remain consistent and up-to-date.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Installation](#installation)
- [Usage](#usage)
- [CI/CD Integration](#cicd-integration)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

## Architecture

### System Components

1. **Sync Script** (`scripts/helm_to_kustomize_sync.sh`)
   - Main synchronization logic
   - Helm template rendering
   - Kustomize manifest generation
   - Validation and error handling

2. **Test Suite** (`scripts/tests/test_helm_to_kustomize.sh`)
   - Unit tests for sync functions
   - Integration tests for end-to-end flow
   - Validation tests for output correctness

3. **CI/CD Workflow** (`.github/workflows/helm-kustomize-sync.yaml`)
   - Automated sync checks on PR/push
   - Scheduled drift detection
   - Auto-generation of sync PRs

### Directory Structure

```
trainer/
├── charts/
│   └── kubeflow-trainer/         # Helm chart source
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── crds/                 # Custom Resource Definitions
│       └── templates/             # Helm templates
│           ├── manager/
│           ├── rbac/
│           └── webhook/
├── manifests/
│   ├── base/                     # Kustomize base configuration
│   │   ├── crds/
│   │   ├── manager/
│   │   ├── rbac/
│   │   ├── webhook/
│   │   └── kustomization.yaml
│   └── overlays/                  # Environment-specific overlays
│       ├── standalone/
│       └── kubeflow-platform/
├── scripts/
│   ├── helm_to_kustomize_sync.sh # Main sync script
│   └── tests/
│       └── test_helm_to_kustomize.sh
└── .github/
    └── workflows/
        └── helm-kustomize-sync.yaml
```

## Installation

### Prerequisites

1. **Required Tools**:
   ```bash
   # Install Helm
   curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

   # Install kubectl (for Kustomize validation)
   curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
   chmod +x kubectl
   sudo mv kubectl /usr/local/bin/
   ```

2. **Optional Tools** (recommended):
   ```bash
   # Install yq for better YAML processing
   wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
   chmod +x /usr/local/bin/yq
   ```

### Setup

1. **Make scripts executable**:
   ```bash
   chmod +x scripts/helm_to_kustomize_sync.sh
   chmod +x scripts/tests/test_helm_to_kustomize.sh
   ```

2. **Verify installation**:
   ```bash
   ./scripts/helm_to_kustomize_sync.sh --help
   ```

## Usage

### Basic Sync

Run the sync with default settings:

```bash
./scripts/helm_to_kustomize_sync.sh
```

### Custom Configuration

Specify custom paths and values:

```bash
./scripts/helm_to_kustomize_sync.sh \
  --chart-path ./charts/kubeflow-trainer \
  --output-dir ./manifests \
  --namespace kubeflow \
  --values-file custom-values.yaml
```

### Dry Run Mode

Preview changes without writing files:

```bash
./scripts/helm_to_kustomize_sync.sh --dry-run --verbose
```

### Validation Mode

Validate generated manifests against Helm output:

```bash
./scripts/helm_to_kustomize_sync.sh --validate
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-h, --help` | Show help message | - |
| `-v, --version` | Show version | - |
| `-c, --chart-path PATH` | Path to Helm chart | `./charts/kubeflow-trainer` |
| `-o, --output-dir PATH` | Output directory for Kustomize manifests | `./manifests` |
| `-f, --values-file FILE` | Values file for Helm rendering | - |
| `-r, --release-name NAME` | Release name for Helm rendering | `kubeflow-trainer` |
| `-n, --namespace NAMESPACE` | Namespace for resources | `kubeflow` |
| `-d, --dry-run` | Perform dry run without writing files | `false` |
| `--validate` | Validate output against Helm rendering | `false` |
| `--verbose` | Enable verbose output | `false` |
| `--preserve-comments` | Preserve comments from Helm templates | `false` |
| `--check-crds` | Check CRD handling capability | - |

## CI/CD Integration

### Automatic Sync Checks

The CI/CD workflow automatically:

1. **Checks for changes** in Helm charts or manifests
2. **Validates synchronization** between Helm and Kustomize
3. **Runs tests** to ensure script functionality
4. **Creates PRs** when sync is needed

### GitHub Actions Workflow

The workflow triggers on:

- **Push** to paths: `charts/**`, `manifests/**`, sync script
- **Pull Requests** affecting the same paths
- **Schedule** (weekly drift detection)
- **Manual dispatch** via GitHub UI

### Workflow Jobs

1. **sync-check**: Validates synchronization status
2. **validate-helm-chart**: Lints and validates Helm chart
3. **sync-on-schedule**: Weekly drift detection

## Testing

### Run All Tests

```bash
./scripts/tests/test_helm_to_kustomize.sh
```

### Test Coverage

The test suite covers:

1. **Script existence and permissions**
2. **Help output and documentation**
3. **Helm validation**
4. **Directory structure creation**
5. **Kustomization file generation**
6. **CRD handling**
7. **Output validation**
8. **Error handling**

### Expected Test Output

```
========================================
  Helm to Kustomize Sync Script Tests
========================================

Running: Test 1: Script exists and is executable
✓ File exists: scripts/helm_to_kustomize_sync.sh
✓ Script is executable

Running: Test 2: Script shows help
✓ Script shows help with --help

[... more tests ...]

========================================
              Test Summary
========================================
Passed: 15
Failed: 0
All tests passed! ✓
```

## Troubleshooting

### Common Issues

1. **Helm not installed**
   ```
   [ERROR] Helm is not installed. Please install Helm first.
   ```
   **Solution**: Install Helm following the prerequisites section.

2. **Invalid chart path**
   ```
   [ERROR] Chart path does not exist: /path/to/chart
   ```
   **Solution**: Verify the chart path and ensure Chart.yaml exists.

3. **Kustomize validation fails**
   ```
   [ERROR] Base kustomization validation failed
   ```
   **Solution**: Check for YAML syntax errors and missing resources.

4. **Permission denied**
   ```
   bash: ./scripts/helm_to_kustomize_sync.sh: Permission denied
   ```
   **Solution**: Make the script executable: `chmod +x scripts/helm_to_kustomize_sync.sh`

### Debug Mode

Enable verbose output for troubleshooting:

```bash
./scripts/helm_to_kustomize_sync.sh --verbose --dry-run
```

### Manual Validation

Validate generated manifests manually:

```bash
# Validate base manifests
kubectl kustomize manifests/base

# Validate overlays
kubectl kustomize manifests/overlays/standalone
kubectl kustomize manifests/overlays/kubeflow-platform
```

## Contributing

### Adding New Resource Types

To add support for new resource types:

1. Edit `scripts/helm_to_kustomize_sync.sh`
2. Add new resource pattern in `process_templates()`:
   ```bash
   process_resource_type "${template_dir}" "newresource" "target_dir"
   ```

### Updating Tests

When adding new features:

1. Add corresponding tests in `scripts/tests/test_helm_to_kustomize.sh`
2. Follow the existing test pattern:
   ```bash
   test_new_feature() {
       print_test_header "Test X: New Feature"
       # Test implementation
       assert_condition "expected" "actual"
   }
   ```

### CI/CD Modifications

To modify the CI/CD workflow:

1. Edit `.github/workflows/helm-kustomize-sync.yaml`
2. Test changes in a feature branch
3. Ensure all existing tests pass

## Best Practices

1. **Always run tests** before committing changes
2. **Use dry-run mode** to preview changes
3. **Validate output** against Helm rendering
4. **Keep manifests in sync** by running the script after Helm chart updates
5. **Review auto-generated PRs** carefully before merging

## Advanced Usage

### Custom Overlays

Create custom overlays for specific environments:

1. Create overlay directory:
   ```bash
   mkdir -p manifests/overlays/production
   ```

2. Create kustomization.yaml:
   ```yaml
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization

   resources:
     - ../../base

   patchesStrategicMerge:
     - deployment-patch.yaml

   configMapGenerator:
     - name: production-config
       literals:
         - ENV=production
   ```

### Integration with ArgoCD

Use generated manifests with ArgoCD:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kubeflow-trainer
spec:
  source:
    repoURL: https://github.com/kubeflow/trainer
    targetRevision: HEAD
    path: manifests/overlays/production
  destination:
    server: https://kubernetes.default.svc
    namespace: kubeflow
```

## Support

For issues or questions:

1. Check the [Troubleshooting](#troubleshooting) section
2. Review existing [GitHub Issues](https://github.com/kubeflow/trainer/issues)
3. Create a new issue with the `area/deployment` label

## License

Copyright 2025 The Kubeflow authors.

Licensed under the Apache License, Version 2.0.