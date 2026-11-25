# Slurm Bridge

Slurm Bridge schedules a multi-node TrainJob as one Slurm job. Trainer creates a Kubernetes
Workload and a native PodGroup for the TrainJob. Bridge submits the gang to Slurm and binds the
Pods to the nodes Slurm allocates. Slurm applies its configured priority, backfill, and topology
policies to the job. On hybrid nodes, native Slurm jobs and Kubernetes workloads can take turns
using the same hardware. See the [Bridge architecture](https://github.com/SlinkyProject/slurm-bridge/blob/v1.3.0/docs/architecture.md).

## Prerequisites

This walkthrough was tested on Kubernetes 1.37.0 with Kubeflow Trainer 2.4.0, Slurm Bridge 1.3.0,
and Slurm 26.05.6. Bridge requires Slurm 25.11 or later with the `v0.0.44` REST data parser.
See the [Bridge compatibility matrix](https://github.com/SlinkyProject/slurm-bridge/blob/v1.3.0/README.md#compatibility) for the Kubernetes and Slurm versions Bridge supports.

- Use a Kubernetes 1.37 or later cluster that serves the native Workload and PodGroup APIs. See the
  [Workload-Aware Scheduling guide](../workload-aware-scheduling.md#prerequisites) for the required
  feature gates and API configuration.
- [Install Kubeflow Trainer](../installation.md) with
  `manager.config.featureGates.TrainJobWorkloadAwareScheduling=true` in the Trainer Helm values.
- [Install Slurm Bridge and connect it to Slurm](https://github.com/SlinkyProject/slurm-bridge/blob/v1.3.0/docs/quickstart.md).
- A Slurm partition with at least two eligible Kubernetes workers. These can be **external nodes**
  registered by Bridge or **hybrid nodes** running both `kubelet` and `slurmd`. If a hybrid
  node's names differ, set its Kubernetes Node label `slinky.slurm.net/slurm-nodename` to the
  Slurm node name. See [Bridge node configuration](https://github.com/SlinkyProject/slurm-bridge/blob/v1.3.0/docs/config.md#nodes).

Run `kubectl api-resources` to verify that your cluster serves the required APIs. The output must
list both `workloads` and `podgroups` at `scheduling.k8s.io/v1beta1`.

```bash
kubectl api-resources --api-group=scheduling.k8s.io
```

## Create a Slurm-enabled Runtime

Save the following as `runtime.yaml`. The TrainJob-level gang policy groups all training Pods.
Setting `schedulerName` on the Pod template routes this runtime's Pods to Bridge even outside
its managed namespaces. Use the name configured in `schedulerConfig.schedulerName` in the Bridge
Helm values. The example uses the default, `slurm-bridge-scheduler`.

To route a whole namespace, add `slurm-training` to the existing
`admission.managedNamespaces` list. If your site uses `admission.managedNamespaceSelector`,
label the namespace to match that selector instead. Within a managed namespace, admission redirects
Pods that select `default-scheduler` to Bridge and preserves other scheduler names.
See the [Bridge Helm values](https://github.com/SlinkyProject/slurm-bridge/blob/v1.3.0/helm/slurm-bridge/values.yaml).

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-slurm-gang
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  scheduling:
    schedulingPolicy:
      gang: {}
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
                metadata:
                  annotations:
                    slurmjob.slinky.slurm.net/partition: slurm-bridge
                spec:
                  schedulerName: slurm-bridge-scheduler
                  containers:
                    - name: node
                      image: python:3.12-slim
```

Set the partition annotation to your site's Slurm partition. For additional placement requirements,
use Slurm constraints on the Pod template. Bridge rejects Pod node selectors by default, and a
native PodGroup with `schedulingConstraints.topology` fails Slurm registration.
See [Bridge workload annotations](https://github.com/SlinkyProject/slurm-bridge/blob/v1.3.0/docs/workload.md#annotations).

```bash
kubectl apply -f runtime.yaml
kubectl create namespace slurm-training
```

## Run a TrainJob

Save the following as `trainjob.yaml`. This CPU-only example uses Python to print each worker's
Trainer-assigned rank and keeps the workers running for five minutes so you can inspect the allocation.
For a training run, set `spec.trainer.image` on the TrainJob to an image containing your framework
and code, and replace the example command with your training entry point. See the
[PyTorch guide](../../user-guides/pytorch.md) for distributed training examples.

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-slurm
  namespace: slurm-training
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    kind: ClusterTrainingRuntime
    name: torch-slurm-gang
  trainer:
    numNodes: 2
    command:
      - python3
      - -c
      - |
        import os
        import time

        print(f"Training node {os.environ['PET_NODE_RANK']} is running", flush=True)
        time.sleep(300)
    resourcesPerNode:
      requests:
        cpu: "1"
        memory: 1Gi
      limits:
        cpu: "1"
        memory: 1Gi
```

```bash
kubectl apply -f trainjob.yaml
```

Trainer sets the generated PodGroup's `gang.minCount` to two. Bridge requests exclusive nodes by
default, so Slurm must have two eligible nodes available at the same time. Ordinary CPU requests
reserve capacity in Slurm. To pin a container to the CPUs Slurm allocated, use
[Bridge CPU DRA](https://github.com/SlinkyProject/slurm-bridge/blob/v1.3.0/docs/workload.md#cpu-dra).

## Verify the allocation

While the TrainJob is active, inspect its Workload and PodGroup. Trainer generates their names
and sets the TrainJob as their owner.

```bash
kubectl get workloads.scheduling.k8s.io,podgroups.scheduling.k8s.io -n slurm-training
kubectl get pods -n slurm-training -l jobset.sigs.k8s.io/jobset-name=pytorch-slurm \
  -o custom-columns='POD:.metadata.name,SCHEDULER:.spec.schedulerName,GROUP:.spec.schedulingGroup.podGroupName,SLURM-JOB:.metadata.labels.scheduler\.slinky\.slurm\.net/slurm-jobid,NODE:.spec.nodeName'
```

In this Kind run, Slurm job 8 allocated two nodes to the same PodGroup:

```console
POD                            SCHEDULER                GROUP                                 SLURM-JOB   NODE
pytorch-slurm-node-0-0-279p6   slurm-bridge-scheduler   pytorch-slurm-b23d449c91-d1826bca72   8           trainer-bridge13-worker2
pytorch-slurm-node-0-1-vslng   slurm-bridge-scheduler   pytorch-slurm-b23d449c91-d1826bca72   8           trainer-bridge13-worker3
```

On a Slurm client, run `squeue -j <job-id>` with the ID from your Pods to see the two-node
allocation. A pending job can already have an ID while the Pods are waiting for nodes. If the ID
is empty, first check that the Pods selected the Bridge scheduler.

After the TrainJob completes, Trainer removes its native Workload and PodGroup. Check completion
and read the ranks from the Pods. The JobSet should report `Completed` and its Job should show
`2/2` completions.

```bash
kubectl wait -n slurm-training --for=condition=Complete trainjob/pytorch-slurm --timeout=10m
kubectl get trainjobs,jobsets,jobs -n slurm-training
kubectl logs -n slurm-training -l jobset.sigs.k8s.io/jobset-name=pytorch-slurm
```

```console
Training node 0 is running
Training node 1 is running
```
