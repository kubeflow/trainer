#!/bin/bash
kind delete cluster
kind create cluster
make generate
make manifests
docker build -t ghcr.io/kubeflow/trainer/trainer-controller-manager -f ./cmd/trainer-controller-manager/Dockerfile .
kind load docker-image ghcr.io/kubeflow/trainer/trainer-controller-manager
kubectl apply --server-side -k ./manifests/overlays/manager
sleep 20
kubectl apply -f examples/flux/
