# TrainJob Network Policies

Platform administrators can opt in to automatically managed Kubernetes
`NetworkPolicy` resources for every TrainJob. Training workers and initializers
can communicate with their own job's pods and private data cache. Other ingress
and egress is blocked unless administrators explicitly allow it.

The feature is disabled by default and does not change the TrainJob API or SDK.
There is no per-job opt-out that would let tenants bypass an administrator's policy.

## Configuration

Set `networkPolicy` in the controller manager's Configuration ConfigMap. Configure
cluster-specific dependencies before enabling it. For example, to allow CoreDNS:

```yaml
networkPolicy:
  enabled: true
  ingress: []
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
```

For Helm, put the same configuration under `manager.config.networkPolicy` in a
values file. For Kustomize, patch
`manifests/base/manager/controller_manager_config.yaml` through your overlay.
Restart the controller after changing its configuration.

`ingress` and `egress` are additional standard Kubernetes NetworkPolicy rules.
They are applied to both training pods and private cache pods. An empty rule
allows all traffic in that direction; use narrow peers and ports when possible.
The Kubernetes API validates the generated policies. An invalid rule or a policy
creation error prevents creation of a new workload and is reported by the
TrainJob controller through its normal error and Event handling.

Allow the dependencies your runtime needs, for example:

- DNS: the example above assumes CoreDNS with the usual labels. NodeLocal DNS
  may require an explicit IP allowance instead.
- Kubernetes API: the cache initializer creates Kubernetes resources. Allow your
  actual API server destination IPs and ports. Depending on CNI service NAT,
  these may be endpoint IPs/ports rather than the Service IP/port. Allowing
  every address on TCP 443 is not an API-server-only rule.
- Storage and downloads: allow approved object-store or download destinations.
  Standard NetworkPolicy supports IP blocks, not DNS-name allowlists.
- Status reporting: allow the controller's configured status-server endpoint
  when the TrainJobStatus feature is used.
- Shared caches: allow the cache's pod and namespace selectors in job egress and
  allow the intended job clients in cache ingress. An independently managed
  shared cache needs its own ingress policy if it is isolated.
- Monitoring and service meshes: add explicit ingress/egress for their actual
  data paths. This feature does not discover mesh or monitoring installations.

## Resources and lifecycle

For each TrainJob the controller creates:

- `trainjob-<TrainJob UID>`, selecting JobSet pods through
  `jobset.sigs.k8s.io/jobset-name`.
- `trainjob-cache-<TrainJob UID>`, selecting its private cache through
  `leaderworkerset.sigs.k8s.io/name: <TrainJob name>-cache`.

Both policies allow same-namespace communication between these two pod sets.
The cache policy is harmless when no cache exists. If the generated cache name
cannot fit into a label value, no cache policy is created, since such a cache
cannot be represented by the initializer's LWS naming scheme.

Existing JobSet and LWS labels also allow policies to cover already-running
workloads when enabled. The operator applies policies before creating new
workload resources. Policies are repaired on active TrainJob reconciliation
when edited or deleted. They remain after a job finishes, since retained pods
must not become unrestricted simply because the job completed. Owner references
let Kubernetes garbage-collect the policies when the TrainJob is deleted.

Setting `enabled: false` and restarting the operator removes only policies owned
by each TrainJob on reconciliation, including for finished jobs. Policies with
conflicting names that are not owned by the TrainJob are never adopted or deleted.

## Security boundaries and testing

A NetworkPolicy-capable networking implementation is required. Verify real
allowed and denied connections; successfully creating policy objects is not a
network enforcement test. Existing connections and policy propagation are CNI
specific, and Kubernetes does not provide an atomic policy-installation barrier.

NetworkPolicies are additive: a separate broad policy can still allow traffic.
Administrators must prevent tenants from spoofing controller-owned pod labels,
editing the operator configuration, or creating policies that bypass isolation.
This feature provides pod-network isolation, not authentication, encryption, or
protection against a tenant with unrestricted namespace administration.

The real-network regression suite can be run against a disposable cluster with
this controller installed using `test/e2e/testdata/network-policy-values.yaml`:

```sh
TRAINER_NETWORK_POLICY_E2E=true go test ./test/e2e \
  -ginkgo.focus='TrainJob network isolation' -ginkgo.v
```

The test uses small HTTP workloads rather than ML frameworks or GPUs.
