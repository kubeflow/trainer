#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
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

# This shell script is used to build a cluster and create a namespace from our
# argo workflow


set -o errexit
set -o nounset
set -o pipefail

CLUSTER_NAME="${CLUSTER_NAME}"
REGION="${AWS_REGION:-us-west-2}"
REGISTRY="${ECR_REGISTRY:-public.ecr.aws/j1r0q0g6/training/training-operator}"
VERSION="${PULL_BASE_SHA}"
GO_DIR=${GOPATH}/src/github.com/${REPO_OWNER}/${REPO_NAME}

echo "Configuring kubeconfig.."
aws eks update-kubeconfig --region=${REGION} --name=${CLUSTER_NAME}

echo "Update training operator manifest with new name $REGISTRY and tag $VERSION"
cd manifests/overlays/standalone
#kustomize edit set image public.ecr.aws/j1r0q0g6/training/training-operator=${REGISTRY}:${VERSION}
kustomize edit set image ghcr.io/kubeflow/training-v1/training-operator=${REGISTRY}:${VERSION}

echo "Installing training operator manifests"
kustomize build . | kubectl apply --server-side -f -

TIMEOUT=30
until kubectl get pods -n kubeflow | grep training-operator | grep 1/1 || [[ $TIMEOUT -eq 1 ]]; do
  sleep 10
  TIMEOUT=$(( TIMEOUT - 1 ))
done
kubectl describe all -n kubeflow
kubectl describe pods -n kubeflow
