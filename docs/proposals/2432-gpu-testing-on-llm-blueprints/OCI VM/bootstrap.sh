# These tools are required for setting up for OCI VM to act as self hosted GPU runner.
# This script is intended to be run as a startup script when creating the VM.

#!/bin/bash
set -eux

sudo apt-get update -y
sudo apt-get upgrade -y

# -------------------------------
# Install build tools & make
# -------------------------------
sudo apt-get install -y build-essential make git curl wget apt-transport-https ca-certificates gnupg lsb-release

# -------------------------------
# Install Go
# -------------------------------
GO_VERSION="1.24.0"
wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
echo "export PATH=\$PATH:/usr/local/go/bin:\$HOME/go/bin" | sudo tee /etc/profile.d/go.sh
source /etc/profile.d/go.sh
rm go${GO_VERSION}.linux-amd64.tar.gz

# -------------------------------
# Install Docker
# -------------------------------
sudo apt-get remove -y docker docker-engine docker.io containerd runc || true
sudo apt-get install -y ca-certificates curl gnupg

sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update -y
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# -------------------------------
# Install NVIDIA Container Toolkit
# -------------------------------
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg \
  && curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

sudo apt-get update -y

export NVIDIA_CONTAINER_TOOLKIT_VERSION=1.17.8-1
sudo apt-get install -y \
    nvidia-container-toolkit=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
    nvidia-container-toolkit-base=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
    libnvidia-container-tools=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
    libnvidia-container1=${NVIDIA_CONTAINER_TOOLKIT_VERSION}

sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker

# -------------------------------
# Install Helm
# -------------------------------
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# -------------------------------
# Install kubectl (latest stable)
# -------------------------------
KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
rm kubectl

# Verify installs
echo "✅ Installed versions:"
go version
sudo docker --version
nvidia-ctk --version
helm version
kubectl version --client=true --output=yaml

echo "🎉 Setup complete!"
