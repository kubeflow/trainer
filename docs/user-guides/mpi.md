# MPI Guide

This guide describes how to run MPI on Kubernetes with TrainJob.

## Prerequisites

Before exploring this guide, make sure to follow [the Getting Started guide](../getting-started/index)
to understand the basics of Kubeflow Trainer.

## MPI Runtime Overview

An MPI runtime is a `ClusterTrainingRuntime` or `TrainingRuntime` with
[the `MPI` ML policy](../operator-guides/ml-policy). It defines two replicatedJobs:

- `launcher`: runs the TrainJob command, which calls `mpirun`. `mpirun` connects to the training
  nodes over SSH and starts the MPI processes there.
- `node`: runs the SSH server and waits for `mpirun` to start the MPI processes.

Kubeflow Trainer currently supports OpenMPI 4 and OpenMPI 5.

For every TrainJob, Kubeflow Trainer generates:

- An SSH key pair in the `<trainjob-name>-mpi-ssh-auth` Secret, mounted on the launcher and on the
  nodes at the `sshAuthMountPath` of the runtime.
- The MPI hostfile in the `<trainjob-name>-mpi-hostfile` ConfigMap, mounted on the launcher at
  `/etc/mpi/hostfile`. It has one line per training node, for example
  `my-mpi-job-node-0-0.my-mpi-job slots=4`.
- The OpenMPI environment variables of the launcher, which point `mpirun` to the hostfile and set
  the default number of slots, for both OpenMPI 4 (`OMPI_MCA_*`) and OpenMPI 5
  (`PRTE_MCA_*`). These variables are reserved, so the TrainJob cannot set them in
  `spec.trainer.env`.

Since the hostfile is already configured, the TrainJob command does not need the `-np` or
`--hostfile` flags: `mpirun` starts one process per slot on every node of the hostfile.

## Choose an MPI Runtime

Kubeflow Trainer includes two builtin MPI runtimes:

- [`deepspeed-distributed`](https://github.com/kubeflow/trainer/blob/master/manifests/base/runtimes/deepspeed_distributed.yaml),
  used in [the DeepSpeed guide](deepspeed).
- [`mlx-distributed`](https://github.com/kubeflow/trainer/blob/master/manifests/base/runtimes/mlx_distributed.yaml),
  used in [the MLX guide](mlx).

Both use OpenMPI, run the containers as the `mpiuser` user with UID 1000, mount the SSH keys at
`/home/mpiuser/.ssh` and enable `runLauncherAsNode`. Your platform administrator may install other
MPI runtimes. A runtime uses MPI when its `spec.mlPolicy` has the `mpi` field:

```sh
kubectl get clustertrainingruntime deepspeed-distributed -o jsonpath='{.spec.mlPolicy}'
```

You should see the MPI policy of the runtime as follows:

```json
{"mpi":{"mpiImplementation":"OpenMPI","numProcPerNode":1,"runLauncherAsNode":true,"sshAuthMountPath":"/home/mpiuser/.ssh"},"numNodes":1}
```

## Configure the Training Nodes

### Number of Nodes

`spec.trainer.numNodes` sets the number of training nodes. The number of Pods depends on
`mlPolicy.mpi.runLauncherAsNode` in the runtime:

- With `runLauncherAsNode: true`, as in the built-in runtimes, the launcher is one of the training
  nodes: it is listed in the hostfile and runs MPI processes like the other nodes. The TrainJob
  creates one launcher Pod and `max(numNodes - 1, 1)` node Pods, so `numNodes: 2` runs on two Pods
  and no Pod is left idle. Since the `node` replicatedJob keeps at least one Pod and both Pods are
  listed in the hostfile, `numNodes: 1` behaves like `numNodes: 2`.
- With `runLauncherAsNode: false`, the launcher only runs `mpirun` and is not listed in the
  hostfile. The TrainJob creates one launcher Pod and `numNodes` node Pods.

### Processes per Node

The `slots` value of each hostfile line is the number of MPI processes that `mpirun` starts on
that node. `spec.trainer.numProcPerNode` sets it and overrides the value of the runtime. When this value is omitted, Kubeflow Trainer sets it to the number of GPUs per node. For example, a TrainJob with 4 GPUs per node starts 4 processes per node.

You can check the generated hostfile with the following command:

```sh
kubectl get configmap <trainjob-name>-mpi-hostfile -o jsonpath='{.data.hostfile}'
```

For example, a TrainJob named `my-mpi-job` with `numNodes: 2` and 4 GPUs per node, using the
`deepspeed-distributed` runtime, gets one line for the node Pod and one for the launcher Pod:

```
my-mpi-job-node-0-0.my-mpi-job slots=4
my-mpi-job-launcher-0-0.my-mpi-job slots=4
```

### Trainer Fields on the Launcher and the Nodes

The `spec.trainer` fields of the TrainJob are set on the `node` container of the launcher and of
the nodes as follows:

| Field              | Launcher | Nodes                                                      |
| ------------------ | -------- | ---------------------------------------------------------- |
| `image`            | Yes      | Yes                                                        |
| `command`, `args`  | Yes      | No, the nodes keep the command of the runtime to start SSH |
| `env`              | Yes      | Only with `runLauncherAsNode: true`                        |
| `resourcesPerNode` | Yes      | Only with `runLauncherAsNode: true`                        |

The `env` of the nodes is set on the SSH server container, and the processes that `mpirun` starts
over SSH do not inherit it. Use `mpirun -x <NAME>` to forward an environment variable from the
launcher to all MPI processes.

With `runLauncherAsNode: false`, the nodes keep the resources of the runtime. You can set them with
[runtime patches](../operator-guides/runtime-patches) on the `node` container of the `node`
replicatedJob.

## Use a Custom Image

`spec.trainer.image` runs on the launcher and on the nodes, so all MPI processes run the same
application and the same MPI version. Besides OpenMPI and the training code, the image must
provide what the runtime expects on the nodes:

- The OpenSSH server and client, and their configuration. The built-in runtimes start `sshd` with
  `/home/mpiuser/.sshd_config`, listening on port 2222, and `/etc/ssh/ssh_config` makes the SSH
  client connect to port 2222 without host key checks. Both files come from the
  `mpioperator/base` image.
- A user that matches the security context of the runtime, and whose home holds the SSH keys mount
  path. The built-in runtimes use `mpiuser` with UID 1000 and `/home/mpiuser/.ssh`.
- OpenMPI and the training dependencies on the default `PATH` of SSH sessions, since the processes
  that `mpirun` starts over SSH do not inherit the `ENV` of the image.

The simplest way to meet these requirements is to extend the image of the runtime:

```dockerfile
FROM ghcr.io/kubeflow/trainer/deepspeed-runtime:latest

COPY train.py /home/mpiuser/train.py
```

## Create a TrainJob

The following TrainJob runs two MPI processes on each of two nodes, using the
`deepspeed-distributed` runtime, and prints the rank of every process and the Pod it runs on:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: my-mpi-job
spec:
  runtimeRef:
    name: deepspeed-distributed
    kind: ClusterTrainingRuntime
    apiGroup: trainer.kubeflow.org
  trainer:
    numNodes: 2
    numProcPerNode: 2
    command:
      - mpirun
      - python3
      - -c
      - |
        import os, socket
        rank, size = os.environ["OMPI_COMM_WORLD_RANK"], os.environ["OMPI_COMM_WORLD_SIZE"]
        print(f"Rank {rank} of {size} on {socket.gethostname()}")
```

Since the runtime enables `runLauncherAsNode`, the TrainJob creates one launcher Pod and one node
Pod, and `mpirun` starts 4 processes. To train on GPUs, set `resourcesPerNode`, for example with
`nvidia.com/gpu: 4`, and leave out `numProcPerNode`, so that each node gets one slot per GPU.

`mpirun` forwards the output of every process to the launcher, so read the logs from the launcher
Pod:

```sh
kubectl logs -l jobset.sigs.k8s.io/jobset-name=my-mpi-job,jobset.sigs.k8s.io/replicatedjob-name=launcher
```

You should see the output of the four processes as follows:

```
Warning: Permanently added '[my-mpi-job-node-0-0.my-mpi-job]:2222' (ECDSA) to the list of known hosts.
Rank 0 of 4 on my-mpi-job-launcher-0-0
Rank 1 of 4 on my-mpi-job-launcher-0-0
Rank 2 of 4 on my-mpi-job-node-0-0
Rank 3 of 4 on my-mpi-job-node-0-0
```

## Next Steps

- Run DeepSpeed and MLX with the Kubeflow SDK in [the DeepSpeed guide](deepspeed) and
  [the MLX guide](mlx).
- Learn more about [the `MPI` ML policy](../operator-guides/ml-policy).
- Migrate an `MPIJob` to TrainJob with [the migration guide](../operator-guides/migration).
