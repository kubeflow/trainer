# GPU Runner Implementation with CNCF ARC

This document describes the implementation of GPU testing infrastructure for Kubeflow Trainer using CNCF's GitHub Actions Runner Controller (ARC) with NVIDIA A10 GPUs.

## Overview

PR [#3067](https://github.com/kubeflow/trainer/pull/3067) replaced the VM-based GPU runner with GPU-enabled ARC from CNCF. This transition enables automated GPU testing for LLM blueprints while addressing several technical challenges related to NVIDIA driver compatibility and nvkind limitations.

## Hardware Specifications

### CNCF GPU Runners

CNCF projects have access to Oracle Cloud Infrastructure (OCI) GPU instances by default:

- **Runner Labels**: `oracle-vm-gpu-a10-1`, `oracle-vm-gpu-a10-2`
- **Runner Group**: `GPUs`
- **GPU Model**: NVIDIA A10 Tensor Core GPU
- **Specifications** (VM.GPU.A10.1):
  - 1x NVIDIA A10 GPU (24GB GDDR6)
  - 15 OCPUs (AMD EPYC 7J13)
  - 240 GB RAM
  - Network bandwidth: 24.6 Gbps

**Reference**: [Oracle Cloud VM GPU Shapes](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm#vm-gpu)

## Key Changes in PR #3067

### 1. Workflow Configuration

**Before** (VM Runner):
```yaml
runs-on: oracle-vm-16cpu-a10gpu-240gb
```

**After** (CNCF ARC):
```yaml
runs-on:
  labels: oracle-vm-gpu-a10-1
  group: GPUs
```

### 2. Removed Separate Cleanup Job

Previously, a separate job deleted the Kind cluster after tests. This is now handled automatically by the ephemeral ARC runners.

## Technical Implementation

### NVIDIA Driver and CDI Compatibility Issues

#### Problem: CDI Blocker

Recent NVIDIA driver updates (v570+) introduced breaking changes with Container Device Interface (CDI) support in nvkind. This is tracked in:
- [NVIDIA/nvkind#20](https://github.com/NVIDIA/nvkind/issues/20) - CDI driver compatibility
- [NVIDIA/nvkind#57](https://github.com/NVIDIA/nvkind/issues/57) - Missing nvidia-container-runtime on newer systems

**Root Cause**: 
The NVIDIA Container Toolkit v1.18.0+ moved to CDI-first architecture, deprecating the legacy `nvidia-container-runtime`. However, nvkind still expects the older runtime configuration, causing cluster creation failures.

**Symptom**:
```bash
ERROR: Failed to create Kind cluster with GPU support
ERROR: nvidia-container-runtime not found
```

#### Solution: Pin NVIDIA Container Toolkit Version

The setup script pins to a known working version:

```bash
export NVIDIA_CONTAINER_TOOLKIT_VERSION=1.17.8-1
sudo apt-get install -y \
    nvidia-container-toolkit=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
    nvidia-container-toolkit-base=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
    libnvidia-container-tools=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
    libnvidia-container1=${NVIDIA_CONTAINER_TOOLKIT_VERSION}
```

**Configuration Changes**:

```bash
# Removed: CDI is disabled to avoid compatibility issues
# sudo nvidia-ctk runtime configure --runtime=docker --set-as-default --cdi.enabled

# Current: Use system drivers without CDI
sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts=true --in-place
sudo nvidia-ctk runtime configure --runtime=docker --set-as-default
sudo systemctl restart docker
```

### nvkind Limitations and Workarounds

#### Limitation 1: GPU Operator Driver Conflict

nvkind expects the host system to provide GPU drivers. Installing the NVIDIA GPU Operator with driver installation causes conflicts.

**Workaround**:
```bash
helm install --wait --generate-name \
  -n gpu-operator --create-namespace \
  nvidia/gpu-operator \
  --version="${GPU_OPERATOR_VERSION}" \
  --set driver.enabled=false  # ← Use system drivers
```

#### Limitation 2: Runtime Class Configuration

Due to above limitation, ClusterTrainingRuntimes can't specify the NVIDIA runtime class, causing GPU workloads to fail.

**Temporary Hotfix** (from PR #3067):
```bash
# Patch CRDs to run on GPU nodes
echo "Patch CRDs to run on GPU nodes"
kubectl get clustertrainingruntimes -o json | jq '
  .items[].spec.template.spec.replicatedJobs[].template.spec.template.spec.runtimeClassName = "nvidia"
' | kubectl apply -f -
```

> **Note**: This is a temporary workaround until nvkind supports proper runtime class configuration. Track progress at [NVIDIA/nvkind#20](https://github.com/NVIDIA/nvkind/issues/20).

## Tested Workloads

The following LLM blueprints are tested in the CI pipeline:

### 1. TorchTune - Qwen2.5-1.5B Fine-tuning
- **Notebook**: `examples/torchtune/qwen2_5/qwen2.5-1.5B-with-alpaca.ipynb`
- **Dataset**: Alpaca (1000 samples)
- **GPU**: 1x A10 (24GB)
- **Memory**: 128GB

### 2. JAX - Distributed MNIST Training
- **Notebook**: `examples/jax/image-classification/mnist.ipynb`
- **Configuration**: 8 CPU, 1 GPU, 1 node
- **Framework**: JAX with distributed training

## Migration from OCI VM Method

The KEP also has Terraform to provision OCI GPU VMs as self-hosted runners. These files are preserved in `docs/proposals/2432-gpu-testing-on-llm-blueprints/OCI VM/` for reference:

- `main.tf` - Terraform configuration for VM.GPU.A10.1 instance
- `bootstrap.sh` - Setup script for GPU drivers and dependencies
- `terraform.tfvars` - Configuration variables

**Why We Moved to ARC**:
1. **Simplified Management**: No manual VM provisioning or maintenance
2. **Cost Efficiency**: Pay only for active CI jobs
3. **Auto-scaling**: ARC handles runner scaling automatically
4. **Security**: Ephemeral runners reduce attack surface
5. **CNCF Support**: Built-in access to GPU infrastructure

## Future Work

1. **Permanent Runtime Class Solution**: Work with nvkind maintainers to support runtime class configuration in cluster creation
2. **CDI Migration**: Once nvkind supports CDI, migrate to the modern NVIDIA Container Toolkit stack
3. **Multi-GPU Testing**: Add test cases for multi-GPU distributed training
4. **Monitoring Dashboard**: Implement metrics collection for GPU utilization and job queue times

## References

- **Pull Request**: [kubeflow/trainer#3067](https://github.com/kubeflow/trainer/pull/3067)
- **Parent Issue**: [kubeflow/trainer#2674](https://github.com/kubeflow/trainer/issues/2674) - GPU Testing for LLM Blueprints
- **Related Issues**:
  - [kubeflow/trainer#2849](https://github.com/kubeflow/trainer/issues/2849) - GH ARC Setup Documentation
  - [kubeflow/trainer#2812](https://github.com/kubeflow/trainer/issues/2812) - GPU Cluster Setup Script Refactoring
- **nvkind Issues**:
  - [NVIDIA/nvkind#20](https://github.com/NVIDIA/nvkind/issues/20) - CDI Driver Compatibility
  - [NVIDIA/nvkind#57](https://github.com/NVIDIA/nvkind/issues/57) - Missing nvidia-container-runtime
- **CNCF Infrastructure**: [cncf/automation#115](https://github.com/cncf/automation/issues/115)
- **Oracle GPU Shapes**: [OCI Compute Shapes Documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm#vm-gpu)

