# Flux Guide

This guide describes how to use TrainJob to train or fine-tune AI models with a [Flux Framework](https://flux-framework.org) High Performance Computing (HPC) cluster.

## Prerequisites

Before exploring this guide, make sure to read [the Getting Started guide](../getting-started/index)
to learn the basics of Kubeflow Trainer.

## Flux Framework Overview

As AI/ML workloads grow in scale and complexity, they often intersect with the needs of traditional High Performance Computing, which can include topology-aware scheduling, high throughput, and using the low latency Message Passing Interface (MPI).

To support these workloads, Kubeflow can be deployed with Flux, an HPC workload manager that offers several important features:

* **Robust MPI Bootstrapping:** Flux uses a tree-based overlay network combined with native bootstrapping that works across MPI variants. Bootstrap over SSH requiring a client and server, shared keys, consistent user IDs, and complicated permissions, is not required.
* **Topology Awareness:** Flux supports workloads that require fine-grained, topology-aware placement for both GPUs and CPUs.
* **Scheduling Features:** Flux is built with support for custom job queues, graph-based scheduling for complex workflows, and scheduling policies.
* **Throughput**: Kubernetes is limited by API interactions, and etcd performance. Throughput in standard Kubernetes clusters can range between 10-100 Pods per second, and it can be much higher for HPC workload managers, especially Flux. In Flux, high throughput is enabled via submitting jobs to a hierarchy of Flux instances.

The integration of Flux Framework with Kubeflow provides these features, and offers a solution for demanding distributed jobs that require features from High Performance Computing.
demanding distributed jobs. The Kubeflow Trainer can be deployed with a Flux Policy to execute workloads that use MPI with or without GPUs.

## Flux Policy Example: LAMMPS

This example demonstrates how to use the **Flux Framework** policy for the Kubeflow Trainer to run distributed HPC workloads.

The Flux plugin automatically handles:
- Cluster discovery and broker configuration.
- Shared encryption (CURVE certificate) generation.
- Flux installation via an init-container.
- Automatic wrapping of your training command.

The example here will show you how to run the [LAMMPS](https://www.lammps.org/) Molecular Dynamic Simulator. The design here emulates the [Flux Operator](https://flux-framework.org/flux-operator/), and you can learn more about Flux Framework and associated projects [here](https://flux-framework.org/). Do you have a question?
We are part of the High Performance Software Foundation ([HPSF](https://hpsf.io/)) and we hope that you reach out to us with questions or feature requests (The GitHub [flux-framework](https://github.com/flux-framework/) organization works well).

## Quick Start

This LAMMPS example assumes two small nodes. You can retrieve and alter the LAMMPS manifest to increase the problem size if you have more than that. Apply the `ClusterTrainingRuntime` and the `TrainJob`:

```bash
kubectl apply --server-side -f https://raw.githubusercontent.com/kubeflow/trainer/refs/heads/master/examples/flux/flux-runtime.yaml
kubectl apply -f https://raw.githubusercontent.com/kubeflow/trainer/refs/heads/master/examples/flux/lammps-train-job.yaml
```
If you get error messages about the webhook, you need to wait a little longer.

### 1. Monitor the Job

Watch for the pods to be created, and wait for them to be `Running`.

```bash
kubectl get pods --watch
```
```console
NAME                         READY   STATUS     RESTARTS   AGE
lammps-flux-node-0-0-w5pwc   0/1     Init:0/1   0          4s
lammps-flux-node-0-1-gxkqp   0/1     Init:0/1   0          4s
lammps-flux-node-0-0-w5pwc   0/1     Init:0/1   0          17s
lammps-flux-node-0-1-gxkqp   0/1     Init:0/1   0          17s
lammps-flux-node-0-0-w5pwc   0/1     PodInitializing   0          32s
lammps-flux-node-0-1-gxkqp   0/1     PodInitializing   0          32s
lammps-flux-node-0-0-w5pwc   1/1     Running           0          3m38s
lammps-flux-node-0-1-gxkqp   1/1     Running           0          3m46s
```

The times above may vary depending on the image you use.

### 2. Check Logs

To see the Flux broker initialization and the output of the LAMMPS job, check the logs of the lead broker (pod index `0-0`):

```bash
kubectl logs lammps-flux-node-0-0-mvjsf -c node -f
```
```console
...
Performance: 0.036 ns/day, 663.320 hours/ns, 4.188 timesteps/s
93.0% CPU use with 8 MPI tasks x 1 OpenMP threads

MPI task timing breakdown:
Section |  min time  |  avg time  |  max time  |%varavg| %total
---------------------------------------------------------------
Pair    | 11.542     | 13.167     | 14.846     |  27.5 | 55.14
Neigh   | 0.26428    | 0.2649     | 0.26568    |   0.1 |  1.11
Comm    | 0.12108    | 1.8002     | 3.4243     |  74.5 |  7.54
Output  | 0.0057505  | 0.006482   | 0.0079982  |   0.8 |  0.03
Modify  | 8.6376     | 8.6386     | 8.6397     |   0.0 | 36.18
Other   |            | 0.002142   |            |       |  0.01

Nlocal:        2432.00 ave        2436 max        2427 min
Histogram: 1 1 0 1 1 0 1 0 1 2
Nghost:        10687.4 ave       10697 max       10680 min
Histogram: 3 0 0 0 2 1 0 0 1 1
Neighs:        824028.0 ave      825512 max      822595 min
Histogram: 1 1 2 0 0 0 1 1 0 2

Total # of neighbors = 6592221
Ave neighs/atom = 338.82715
Neighbor list builds = 5
Dangerous builds not checked
Total wall time: 0:00:24
```

You can look at the second pod to see the follower broker bootstrap with the lead broker, and then cleanup when LAMMPS is done running.

```bash
kubectl logs lammps-flux-node-0-1-glj22 -c node -f
```
```console
Defaulted container "node" out of: node, flux-installer (init)
Python version: /mnt/flux/view/bin/python3.11
Python root: /mnt/flux/view/lib/python3.11
🌀 flux start  -o --config /mnt/flux/config/etc/flux/config -Scron.directory=/mnt/flux/config/etc/flux/system/cron.d   -Stbon.fanout=256   -Srundir=/mnt/flux/config/run/flux    -Sstatedir=/mnt/flux/config/var/lib/flux -Slocal-uri=local:///mnt/flux/config/run/flux/local   -Slog-stderr-level=0    -Slog-stderr-mode=local
The follower worker exited cleanly. Goodbye!
```

## Interactive Mode

A cool feature of the Flux plugin is the ability to launch an interactive HPC cluster for debugging or manual job submission.

### Switch to Interactive Mode

Create an interactive lammps cluster:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubeflow/trainer/refs/heads/master/examples/flux/flux-interactive.yaml
```

### Using the Flux Shell

Once the pods are `Running` in interactive mode, shell into the lead broker pod:

```bash
kubectl exec -it lammps-flux-interactive-node-0-0-gps5p -- bash
```

Once inside the container, follow these steps to interact with your cluster:

```bash
# 1. Source the environment to put Flux and software in your PATH
. /mnt/flux/flux-view.sh

# 2. Connect to the running lead broker socket
flux proxy $fluxsocket bash

# 3. Manually run LAMMPS across the cluster
flux run -N 4 -n 4 lmp -v x 2 -v y 2 -v z 2 -in in.reaxc.hns -nocite

# or run for a beefier node
flux run -N 4 -n 256 lmp -v x 2 -v y 2 -v z 2 -in in.reaxc.hns -nocite
```

For the above, the `WORKDIR` has the LAMMPS input file `in.reaxc.hns`.

## Configuration Details

- **Runtime Configuration**: The `flux-runtime.yaml` defines the base blueprint. Note that the `flux: {}` policy trigger must be present for the plugin to activate.
- **Environment Variables**: You can customize the Flux setup by adding `env` variables to the `TrainJob` spec (e.g., `FLUX_VIEW_IMAGE` to change the base OS or `FLUX_NETWORK_DEVICE` to specify the interface). See [this example](https://github.com/kubeflow/trainer/blob/master/examples/flux/lammps-train-job.yaml) for setting environment variables.
- **Volumes**: Binaries are installed to `/mnt/flux`, software is copied to `/opt/software`, and configurations are stored in `/etc/flux-config`.

For environment variables, we currently support a small set:

- FLUX_VIEW_IMAGE: The flux view base image, which defaults to `ghcr.io/converged-computing/flux-view-ubuntu:tag-jammy`
- FLUX_NETWORK_DEVICE: The network device for the Flux overlay network only (not necessarily your application). Defaults to `eth0`

This can be easily expanded. If you would like help creating a custom image, please open an issue in the [Flux GitHub organization](https://github.com/flux-framework).
For the view, you primarily want it to make the base container platform, OS and version. We currently also provide:

- ghcr.io/converged-computing/flux-view-rocky:arm-9
- ghcr.io/converged-computing/flux-view-rocky:arm-8
- ghcr.io/converged-computing/flux-view-rocky:tag-9
- ghcr.io/converged-computing/flux-view-rocky:tag-8
- ghcr.io/converged-computing/flux-view-ubuntu:tag-noble
- ghcr.io/converged-computing/flux-view-ubuntu:tag-jammy
- ghcr.io/converged-computing/flux-view-ubuntu:tag-focal
- ghcr.io/converged-computing/flux-view-ubuntu:arm-jammy
- ghcr.io/converged-computing/flux-view-ubuntu:arm-focal

Thanks for stopping by!

## Next Steps

- Check out [the Flux Operator](https://github.com/flux-framework/flux-operator).
- Learn more about [Flux Framework APIs](https://flux-framework.org).
- For more about Flux and its context for Kubeflow Trainer, see [KEP-2841](https://github.com/kubeflow/trainer/tree/master/proposals/2841-flux-hpc).
