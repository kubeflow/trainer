# Slurm Bridge

This guide describes how to schedule Kubeflow TrainJobs with
[Slurm Bridge](https://github.com/SlinkyProject/slurm-bridge) using Trainer's Coscheduling
`PodGroupPolicy`.

Trainer creates one `PodGroup` for the TrainJob and adds every generated Pod to
that group. Slurm Bridge submits the group as one external Slurm job, waits for Slurm to allocate
nodes, and then binds the Pods to those nodes.

Slurm applies its configured topology, priority, and backfill policies to the job. When `kubelet`
and `slurmd` run on the same compute nodes, Kubernetes and native Slurm workloads can share the
same pool.

This integration uses the scheduler-plugins `scheduling.x-k8s.io/v1alpha1` API. It does not require
the native Kubernetes 1.36 Workload APIs or their feature gates.

Slurm Bridge also supports the native Kubernetes PodGroup API on Kubernetes 1.36 and later. See the
Slurm Bridge
[PodGroup comparison](https://github.com/SlinkyProject/slurm-bridge/blob/main/docs/workload.md#podgroup-coscheduling)
for the distinction between the two APIs.

## Prerequisites

- [Kubeflow Trainer installed](../installation.md).
- [Slurm Bridge installed and connected to Slurm](https://github.com/SlinkyProject/slurm-bridge/blob/main/docs/quickstart.md).
- `kubelet` and `slurmd` running on the compute nodes used by Slurm Bridge.
- Matching Kubernetes and Slurm node names. If the names differ, label each Kubernetes Node with
  `slinky.slurm.net/slurm-nodename: <slurm-node-name>`.

Review the [Slurm Bridge compatibility requirements](https://github.com/SlinkyProject/slurm-bridge#compatibility)
before selecting Kubernetes, Slurm, Slurm Bridge, and scheduler-plugins versions.

## Install the Coscheduling API and Controller

Install the scheduler-plugins `PodGroup` CRD and CoScheduling controller:

```bash
helm install --repo https://scheduler-plugins.sigs.k8s.io scheduler-plugins scheduler-plugins \
  --namespace scheduler-plugins --create-namespace \
  --set 'plugins.enabled={CoScheduling}' \
  --set 'scheduler.replicaCount=0'
```

The scheduler replica count must remain zero. Slurm Bridge schedules the Pods; the
scheduler-plugins installation supplies only the `PodGroup` API and controller required by the
integration.

Trainer discovers the scheduler-plugins API when its controller starts. Install the `PodGroup` CRD
before installing Trainer or restart the Trainer controller after installing the CRD.

Verify that the API is available:

```bash
kubectl api-resources --api-group=scheduling.x-k8s.io
```

```console
NAME            SHORTNAMES   APIVERSION                     NAMESPACED   KIND
elasticquotas   eq,eqs       scheduling.x-k8s.io/v1alpha1   true         ElasticQuota
podgroups       pg,pgs       scheduling.x-k8s.io/v1alpha1   true         PodGroup
```

```bash
kubectl get pods -n scheduler-plugins
```

```console
NAME                                           READY   STATUS    RESTARTS   AGE
scheduler-plugins-controller-7469c866f-ngc7t   1/1     Running   0          21m
```

The API resource output should include `podgroups`.

## Create a Slurm-enabled Runtime

Create a dedicated runtime with the Coscheduling policy:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-slurm
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  podGroupPolicy:
    coscheduling: {}
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
                  containers:
                    - name: node
                      image: pytorch/pytorch:2.13.0-cuda13.0-cudnn9-runtime
```

```bash
kubectl apply -f runtime.yaml
```

Trainer calculates the `PodGroup` member count and resources from the runtime.

:::{note}
Trainer sets a default `scheduleTimeoutSeconds` on the generated scheduler-plugins `PodGroup`.
Slurm Bridge ignores that field, so it does not limit how long the external Slurm job can remain
pending.
:::

## Route TrainJobs to Slurm Bridge

Create a dedicated namespace for Slurm-scheduled TrainJobs:

```bash
kubectl create namespace trainer-slurm
```

Add the namespace to an existing Slurm Bridge helm installation:

```bash
helm upgrade slurm-bridge oci://ghcr.io/slinkyproject/charts/slurm-bridge \
  --namespace slurm \
  --reuse-values \
  --set 'admission.managedNamespaces={trainer-slurm}'
```

See the
[Slurm Bridge Helm values](https://github.com/SlinkyProject/slurm-bridge/blob/main/helm/slurm-bridge/values.yaml)
for additional configuration options.

The Slurm Bridge admission controller changes `spec.schedulerName` from `default-scheduler` to
`slurm-bridge-scheduler` for Pods in the managed namespace. Pods that explicitly select another
scheduler are not rewritten.

## Run a TrainJob

Create a TrainJob that uses the Slurm-enabled runtime:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: pytorch-slurm
  namespace: trainer-slurm
spec:
  runtimeRef:
    apiGroup: trainer.kubeflow.org
    kind: ClusterTrainingRuntime
    name: torch-distributed-slurm
  trainer:
    numNodes: 2
    command:
      - python3
      - -c
      - |
        import os
        import time

        print(f"Training node {os.environ['PET_NODE_RANK']} is running")
        time.sleep(60)
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
kubectl get podgroups.scheduling.x-k8s.io pytorch-slurm -n trainer-slurm
```

The generated `PodGroup` has the same name and namespace as the TrainJob.

:::{note}
Slurm Bridge uses exclusive whole-node allocations by default. Native Kubernetes CPU requests
reserve CPU capacity in Slurm but do not constrain the container to Slurm's allocated CPU set. Use
[Slurm Bridge CPU DRA](https://github.com/SlinkyProject/slurm-bridge/blob/main/docs/workload.md#cpu-dra)
when aligned CPU isolation is required.
:::

## Verify the TrainJob

Check the Trainer resources and scheduler-plugins PodGroup:

```bash
kubectl get trainjobs,jobsets,jobs -n trainer-slurm
```

```console
NAME                                          STATE      AGE
trainjob.trainer.kubeflow.org/pytorch-slurm   Complete   76s

NAME                                   TERMINALSTATE   RESTARTS   COMPLETED   SUSPENDED   AGE
jobset.jobset.x-k8s.io/pytorch-slurm   Completed       0          True        false       76s

NAME                             STATUS     COMPLETIONS   DURATION   AGE
job.batch/pytorch-slurm-node-0   Complete   2/2           71s        76s
```

```bash
kubectl get podgroups.scheduling.x-k8s.io -n trainer-slurm
```

```console
NAME            PHASE      MINMEMBER   RUNNING   SUCCEEDED   FAILED   AGE
pytorch-slurm   Finished   2                     2                    81s
```

Inspect the scheduler, Slurm job ID, allocated Slurm node, and bound Kubernetes Node:

```bash
kubectl get pods \
  -n trainer-slurm \
  -l jobset.sigs.k8s.io/jobset-name=pytorch-slurm \
  -o custom-columns='NAME:.metadata.name,SCHEDULER:.spec.schedulerName,SLURM-JOB:.metadata.labels.scheduler\.slinky\.slurm\.net/slurm-jobid,SLURM-NODE:.metadata.annotations.slinky\.slurm\.net/slurm-node,NODE:.spec.nodeName'
```

```console
NAME                           SCHEDULER                SLURM-JOB   SLURM-NODE                   NODE
pytorch-slurm-node-0-0-6cv72   slurm-bridge-scheduler   2           trainer-guide-3951-worker4   trainer-guide-3951-worker4
pytorch-slurm-node-0-1-kjn4j   slurm-bridge-scheduler   2           trainer-guide-3951-worker5   trainer-guide-3951-worker5
```

The `scheduler.slinky.slurm.net/slurm-jobid` label confirms that Slurm Bridge submitted the external
job. Non-empty Slurm and Kubernetes node columns confirm allocation. Use `squeue -j <job-id>` while
the external job is pending or running to see its entry in Slurm.

## Next Steps

- Review Slurm Bridge [workload and resource support](https://github.com/SlinkyProject/slurm-bridge/blob/main/docs/workload.md).
- Review the [Slurm Bridge architecture](https://github.com/SlinkyProject/slurm-bridge/blob/main/docs/architecture.md).
