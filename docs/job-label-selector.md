# Selecting training jobs by label

`--job-label-selector` restricts which training jobs an instance of Training
Operator reconciles. It applies to TFJob, PyTorchJob, MPIJob, XGBoostJob,
PaddleJob, and JAXJob, within the namespace scope already configured for the
operator. It selects labels on the **Job metadata**, not its Pod template.

The default is an empty selector: all jobs are reconciled, preserving existing
behavior. Invalid selectors cause startup to fail before connecting to the
cluster. Kubernetes equality, set-based, and existence expressions are supported.

For example, add this argument to the operator container:

```yaml
args:
  - --job-label-selector=routing.example.org/controller=new
```

Select a job using its metadata:

```yaml
apiVersion: kubeflow.org/v1
kind: PyTorchJob
metadata:
  name: example
  labels:
    routing.example.org/controller: new
# Supply spec as usual.
```

The selector can be combined with `--gang-scheduler-name=volcano`; it does not
change the scheduling backend or PodGroup API.

## Behavior and limits

- The controller checks the current cached Job labels on every reconciliation,
  including reconciliations triggered by dependent resources. Nonmatching jobs
  are skipped before defaulting, resource management, status updates, or cleanup.
  Their create events also skip the existing defaulting/status initialization.
- Pod, Service, and PodGroup watches remain intact. These resources do not need
  to repeat the selection label. This is a reconciliation filter, not a cache,
  RBAC, or security boundary; existing list/watch permissions are still needed.
- Removing a matching label does not delete existing resources or transfer
  ownership. There is no atomic handoff: an in-flight reconciliation can finish
  using previously read labels. Use stable routing labels for active jobs and
  coordinate controller shutdown before changing ownership.
- Admission webhooks are configured separately. This argument does not restrict
  validation/defaulting performed by a webhook. If admission must be scoped,
  configure its admission registration (for example, `objectSelector`) and
  account for certificate and webhook configuration ownership separately.

## Multiple controller instances

Every controller that can handle the same Job kind must have a disjoint scope.
For example, one selector can be `routing.example.org/controller=new` and the
other `routing.example.org/controller!=new`. The latter also matches jobs with
no routing label. A legacy operator that has no filtering support will still
process both groups; adding this argument to the new operator alone does **not**
make it safe to run alongside that legacy operator.

Instances managing different job groups need distinct leader-election IDs
when leader election is enabled. Replicas of the same logical instance should
share an ID. Separately plan admission webhook, certificate Secret, and other
shared installation resources; this flag is not a complete multi-install setup.

The operator does not assign routing labels. A submitter or a separately
configured admission mutator can assign them on creation. That integration,
legacy operator filtering, and migration of existing jobs are outside this
feature. No Arena CLI or training CRD schema changes are required by this flag.
