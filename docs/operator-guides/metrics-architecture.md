# Trainer metrics architecture

This document summarizes how Trainer metrics are implemented and the design
decisions behind them.

## Scope

The metric names, labels, and query examples are defined in
[Trainer Controller Metrics](metrics.md). This document focuses on lifecycle,
ownership, and implementation decisions.

The proposed accelerator metric is not included yet. Accelerator requests can
come from runtime defaults, plugins, or TrainJob overrides. We should add this
metric once the effective-resource rules are clearly defined.

## Architecture

```text
Kubernetes API
     |
     +--> Runtime informers --> Runtime snapshot --> runtime_info
     |
     +--> TrainJob informer --> Elected leader --> TrainJob metrics
                                                     |
                                             Prometheus registry
                                                     |
                                               /metrics endpoint
```

The implementation uses informer data already held by the controller. Metric
collection does not call the Kubernetes API. This prevents a slow or
unavailable API server from blocking a scrape, although collection still
copies and iterates the local snapshot.

The runtime snapshot and TrainJob event metrics run only on the elected leader.
The metrics server remains available on every replica, but Trainer metrics are
owned and exposed by the leader to avoid duplicate series.

### Replica ownership

```text
Controller replica A                         Controller replica B
--------------------                         --------------------
Runtime snapshot: no                         Runtime snapshot: yes
TrainJob observer: no                        TrainJob observer: yes
                                             (elected leader)

                         Leader changes
                A becomes leader <----------> B becomes leader
                B removes observer            A removes observer
                A registers observer          B registers observer
```

## How metrics are recorded

1. Runtime informers watch TrainingRuntime and ClusterTrainingRuntime objects.
2. Add, update, and delete events maintain the in-memory runtime snapshot.
3. The runtime snapshot is synchronized before it is used for TrainJob metrics.
4. The elected leader registers the TrainJob observer.
5. New TrainJobs increment the creation counter.
6. The first terminal transition increments either the complete or failed
   counter and records the lifetime. Repeated terminal updates are ignored.

### TrainJob event flow

```text
TrainJob created
      |
      +--> creation counter
      |
      +--> resolve runtime and requested nodes
             |
             +--> requested-nodes histogram, when available

TrainJob status update
      |
      +--> terminal for the first time?
             |
             +-- no --> ignore
             |
             +-- yes -> complete/failed counter + lifetime histogram
```

TrainJobs already present during the initial cache listing are not counted as
newly created. These counters describe events observed by the current
controller process; they are not durable historical totals.

## Requested training nodes

The requested-node value is resolved in this order:

1. Explicit TrainJob `numNodes`, including zero.
2. Runtime ML policy `numNodes`.
3. The trainer replicated-job template `parallelism` when the runtime has no
   ML policy.
4. One node when no runtime value is configured.

If the runtime cannot be found or its value is invalid, the workload-size
observation is skipped instead of recording a guessed value.

## Labels and cardinality

Labels contain normalized values and bounded categories needed for
aggregation:

- runtime reference;
- namespace;
- runtime kind;
- result for lifetime observations;
- normalized failure reason;
- ML policy; and
- PodGroupPolicy.

TrainJob names, UIDs, child-job names, and raw error messages are not exported.
This avoids creating a new time series for every individual job. Runtime and
namespace labels are still user- or administrator-controlled dimensions, so
the total number of series grows with the number of configured runtimes.

The lifetime histogram covers ten minutes through 28 days. The
requested-node histogram covers one through 128 nodes, plus `+Inf`.

## Questions these metrics can answer

These metrics help answer:

- Are users creating TrainJobs, and is the creation rate increasing or
  decreasing?
- Which runtimes and namespaces are receiving TrainJobs?
- Which runtimes are most frequently used?
- How many TrainJobs finish successfully or unsuccessfully?
- What is the failure rate by runtime or failure reason?
- What failure reasons are most common?
- How long do successful and failed TrainJobs take to reach a terminal state?
- What are the typical and high-percentile workload sizes by runtime?
- Which runtimes, ML policies, and PodGroupPolicies are currently configured?
- Is the Trainer metrics endpoint being scraped successfully?

Use `rate()` or `increase()` for creation and completion activity, and use
`histogram_quantile()` for lifetime and workload-size percentiles.

Namespace is only a proxy for internal users; these metrics do not identify
individual customers. They also do not show currently running jobs, resource
utilization, scheduling wait time, checkpoints, training progress, or model
quality. Those limitations are described in [Out of scope](#out-of-scope).

### From metric to question

```text
Metric samples
      |
      +--> rate() / increase() ------> creation and completion trends
      |
      +--> histogram_quantile() ------> duration and workload percentiles
      |
      +--> group by runtime/reason --> usage and failure comparisons
      |
      +--> runtime_info --------------> current runtime-policy inventory
```

## Restarts and leadership changes

Counters and histograms are held in the controller process. Their in-memory
values reset when the controller restarts or leadership changes. Prometheus
queries should therefore use `rate()` or `increase()` instead of treating
them as permanent totals. A follower's metrics endpoint may remain healthy
while the leader-owned Trainer metric families are absent.

The runtime inventory is rebuilt from informer state after a restart. Events
that occur before the new leader is ready, or during a leadership transition,
may not be counted.

Metric collection remains local and does not make Kubernetes API calls. If
runtime metadata is not available, only the dependent workload-size
observation is skipped.

## Out of scope

This implementation does not provide:

- durable history across controller restarts;
- GPU utilization or actual resource consumption;
- Kueue wait times or scheduling reasons;
- checkpoint usage;
- training progress or estimated completion time;
- hyperparameters or model-quality metrics; or
- reliable customer identity.

These require additional instrumentation in Trainer, Kueue, JobSet, the
runtime, or the SDK.
