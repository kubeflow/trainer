# KEP-3416 Test Plan: Inject PET_* Envs into Trainer Init Containers

## Overview

This test plan validates the implementation of PET_* environment variable injection into trainer init containers via the `envInjection` configuration in `TorchMLPolicy`.

**PR under test**: [#3516](https://github.com/kubeflow/trainer/pull/3516)

## Results

| Layer | What it validates | Status |
|------:|-------------------|:------:|
| Unit: torch plugin | PET_* env injection into targeted init/sidecar containers via `torch.envInjection.targets` | ✅ Pass |
| Unit: jobset plugin | `PodSet.InitContainers` are carried into rendered JobSet PodSpec | ✅ Pass |
| E2E (manual) | Real user workflow: install → runtime → trainjob → inspect JobSet + init logs | ✅ Pass |

**Unit test commands**:
```bash
go test ./pkg/runtime/framework/plugins/torch/... -v -count=1
go test ./pkg/runtime/framework/plugins/jobset/... -v -count=1
```

## OrbStack E2E (Manual)

### Environment
- Kubernetes: OrbStack v1.31.6+orb1
- Cluster: single-node (local)

### Install Trainer (server-side apply)

Important: use server-side apply to avoid CRD size issues caused by `kubectl apply` adding the
`kubectl.kubernetes.io/last-applied-configuration` annotation to large CRDs.

```bash
kubectl apply --server-side -k manifests/overlays/manager
```

### Run controller from local code (imagePullPolicy=Never)

```bash
LOCAL_TAG=ghcr.io/kubeflow/trainer/trainer-controller-manager:pet-envinj-local
docker build . -f cmd/trainer-controller-manager/Dockerfile -t ${LOCAL_TAG}

kubectl -n kubeflow-system patch deploy kubeflow-trainer-controller-manager --type='strategic' -p "
spec:
  template:
    spec:
      containers:
        - name: manager
          image: ${LOCAL_TAG}
          imagePullPolicy: Never
"
kubectl -n kubeflow-system rollout restart deploy/kubeflow-trainer-controller-manager
kubectl -n kubeflow-system rollout status deploy/kubeflow-trainer-controller-manager --timeout=120s
```

### User workflow validation (ClusterTrainingRuntime + TrainJob)

1) Create a runtime that enables init-container injection:

```bash
cat > /tmp/torch-distributed-envinj.yaml <<'EOF'
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-envinj
spec:
  mlPolicy:
    numNodes: 1
    torch:
      envInjection:
        targets:
          - jobName: node
            containerNames:
              - preflight-check
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  initContainers:
                    - name: preflight-check
                      image: docker.m.daocloud.io/library/busybox:1.36.1
                      command: ["sh","-c"]
                      args:
                        - |
                          set -eu
                          echo "INIT PET_NNODES=${PET_NNODES:-}"
                          echo "INIT PET_NPROC_PER_NODE=${PET_NPROC_PER_NODE:-}"
                          echo "INIT PET_NODE_RANK=${PET_NODE_RANK:-}"
                          echo "INIT PET_MASTER_ADDR=${PET_MASTER_ADDR:-}"
                          echo "INIT PET_MASTER_PORT=${PET_MASTER_PORT:-}"
                          test -n "${PET_NNODES:-}"
                          test -n "${PET_NPROC_PER_NODE:-}"
                          test -n "${PET_NODE_RANK:-}"
                          test -n "${PET_MASTER_ADDR:-}"
                          test -n "${PET_MASTER_PORT:-}"
                  containers:
                    - name: node
                      image: docker.m.daocloud.io/library/busybox:1.36.1
                      command: ["sh","-c"]
                      args:
                        - |
                          set -eu
                          echo "MAIN PET_*:"
                          env | grep '^PET_' | sort
                          sleep 20
EOF
kubectl apply -f /tmp/torch-distributed-envinj.yaml
```

2) Create a 2-node TrainJob referencing that runtime:

```bash
cat > /tmp/pet-env-inj-e2e.yaml <<'EOF'
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pet-env-inj-e2e
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    kind: ClusterTrainingRuntime
    name: torch-distributed-envinj
  trainer:
    numNodes: 2
    numProcPerNode: 1
EOF
kubectl apply -f /tmp/pet-env-inj-e2e.yaml
```

3) Verify JobSet spec contains PET_* in both main container and init container:

```bash
kubectl get jobset pet-env-inj-e2e -o yaml | sed -n '/name: preflight-check/,+30p'
```

Observed snippet (init container has PET_* envs):

```
env:
- name: PET_NNODES
  value: "2"
- name: PET_NPROC_PER_NODE
  value: "1"
- name: PET_NODE_RANK
  valueFrom:
    fieldRef:
      fieldPath: metadata.annotations['batch.kubernetes.io/job-completion-index']
- name: PET_MASTER_ADDR
  value: pet-env-inj-e2e-node-0-0.pet-env-inj-e2e
- name: PET_MASTER_PORT
  value: "29500"
```

4) Verify init container actually sees PET_* at runtime:

```bash
pod=$(kubectl get pods -l jobset.sigs.k8s.io/jobset-name=pet-env-inj-e2e -o jsonpath='{.items[0].metadata.name}')
kubectl logs "${pod}" -c preflight-check
```

Observed:

```
INIT PET_NNODES=2
INIT PET_NPROC_PER_NODE=1
INIT PET_NODE_RANK=0
INIT PET_MASTER_ADDR=pet-env-inj-e2e-node-0-0.pet-env-inj-e2e
INIT PET_MASTER_PORT=29500
```
