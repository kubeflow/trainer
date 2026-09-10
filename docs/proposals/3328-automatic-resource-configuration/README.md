# KEP-3328: Automatic Resource Configuration

<!--
This is the title of your KEP. Keep it short, simple, and descriptive. A good
title can help communicate what the KEP is and should be considered as part of
any review.
-->

## Authors

- Vassilis Vassiliadis - [@VassilisVassiliadis](https://github.com/VassilisVassiliadis)

## Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap. It should be
possible to collect this information before implementation begins, in order to
avoid requiring implementors to split their attention between writing release
notes and implementing the feature itself. KEP editors should ensure that
the tone and content of the `Summary` section is useful for a wide audience.
A good summary is probably at least a paragraph in length.
Both in this section and below, follow the guidelines of the [documentation
style guide]. In particular, wrap lines to a reasonable length, to make it
easier for reviewers to cite specific portions, and to minimize diff churn on
updates.
[documentation style guide]: https://github.com/kubernetes/community/blob/master/contributors/guide/style-guide.md
-->

An extensible mechanism for automatically configuring TrainJob resource requests
(starting with GPUs, CPUs, memory, and replicas) before a TrainJob becomes eligible
for admission and scheduling. The mechanism delegates recommendations to plugin
external controllers that compute a recommended configuration and write it back to
the TrainJob.

At a high level:
1. Kubeflow Trainer owns the protocol for mutating the TrainJob (admission gating,
   API, limited observability via conditions, etc.)
2. Plugin external controllers own the logic for generating the resource
   configurations.

## Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP. Describe why the change is important and the benefits to users. The
motivation section can optionally provide links to [experience reports] to
demonstrate the interest in a KEP within the wider Kubeflow community.
[experience reports]: https://github.com/golang/go/wiki/ExperienceReports
-->

Resource configuration of TrainJobs is not a minor task. It is a manual decision
that directly affects queueing time, cost, and success rate. ML practitioners guess
GPU count and memory headroom following the recommendations of platform teams, but
these recommendations can go stale. The problem is exacerbated in multi-tenant
Kubernetes clusters where users compete against each other for shared, limited, and
expensive accelerators like GPUs.

There are two undesired scenarios:
- Under-requesting resources can cause GPU out-of-memory errors, potentially after
  the TrainJob has already executed for a while.
- Over-requesting increases queue time and reduces cluster utilization, which can
  make the platform feel slower and more expensive than necessary.

Intuitively, Kubernetes can schedule what the user asks for, but it cannot help the
user size their jobs. Fortunately, it is possible to implement methods that
automatically decide the resource requirements of jobs. For example, there are 
machine-learning based resource recommenders employing techniques that leverage 
the characteristics of a TrainJob or even the state of the cluster to automatically
decide the number of GPUs required for tasks like fine-tuning.

This KEP introduces the use of external Kubernetes controllers acting as plugins 
that produce resource recommendations while Kubeflow Trainer orchestrates applying 
these recommendations to TrainJob objects safely.


### Goals

<!--
List the specific goals of the KEP. What is it trying to achieve? How will we
know that this has succeeded?
-->

- Automatically configure TrainJob resources before the job is eligible for admission:
  initially once, right after creation. Starting with `spec.trainer.numNodes`.
- Support a catalog of plugins: platform engineers can enable/disable plugins per 
  namespace enabling users to pick the one they wish to use for their TrainJob.

### Non-Goals

<!--
What is out of scope for this KEP? Listing non-goals helps to focus discussion
and make progress.
-->

- Implement or standardize recommender policies inside Kubeflow Trainer: Kubeflow owns
  the protocol, plugins own the resource recommendation logic/policy.
- Continuously reconfigure resources while the job is pending or after it has started
  running: this is useful but more complex. It could be future work.
- Implement a GUI for the catalog: users can discover available plugins using
  kubectl, so we need not complicate things now.
- Integrate with the [`AdmissionGatedBy`](https://kueue.sigs.k8s.io/docs/reference/labels-and-annotations/#kueuex-k8sioadmission-gated-by)
  feature in Kueue. This is future work.
- Support TrainJobs that use DRAs: This is future work.
- Support modifying the per-node resources. This is future work.
- Provide ample observability signals (status updates, events, etc). This is future work.
- Provide guardrails e.g. time out and fallback strategies. This is future work.

## Proposal

<!--
This is where we get down to the specifics of what the proposal actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real
nitty-gritty.
-->

Kubeflow Trainer implements the protocol that external controllers acting as plugins
use to mutate the `spec.trainer.numNodes` field of TrainJobs that opt into this process.

TrainJobs that do not opt into automatic resource mutation are treated by Kubeflow
Trainer as usual.

At a high level, everything related to deciding and patching the resources of Jobs is 
the responsibility of the external Kubernetes Controller that acts as the autoconf-plugin.
Everything related to gating/ungating the execution of the TrainJob is left to Kubeflow 
Trainer.



### User Stories (Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

#### Story 1

As a TrainJob user fine-tuning models on my organization's multi-tenant Kubernetes cluster,
I want to delegate the configuration of my TrainJob's resource requirements to an AI agent.
The platform engineers managing my cluster have already deployed a Kubernetes controller
wrapping the autoconf-plugin with the id `ai-agent-recommender`.

It is an AI agent powered resource recommender for TrainJobs. The platform engineers 
provided the AI agent with Model Context Protocol (MCP) tools that can interact with
the cluster to get more information about it (e.g. the available GPUs, the running
workloads, etc).

I write my TrainJob like I usually do except for 2 changes:

- I include the `trainer.kubeflow.org/autoconf-plugin` annotation pointing to the autoconf-plugin I want to use
- I start the job suspended

```yaml
...
metadata:
  annotations:
    trainer.kubeflow.org/autoconf-plugin: ai-agent-recommender
  ...
spec:
  ...
  suspend: true
```

Soon after the TrainJob is submitted, the AI agent uses the Kubeflow API to patch my
TrainJob's `spec.trainer.numNodes` field. Eventually the job gets unsuspended.


### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go into as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

1. Mutating `spec.trainer.numNodes` requires lifting the immutability constraint of
  [`spec.trainer` field](https://github.com/kubeflow/trainer/blob/a433daeee5697bfacdbfa0451a042911fbeb4874/pkg/apis/trainer/v1alpha1/trainjob_types.go#L113-L116)
  
2. The resource requirements of JobSets are immutable, including the fields that `spec.trainer.numNodes` sets i.e.
  `spec.replicatedJobs[].template.spec.parallelism` and `spec.replicatedJobs[].template.spec.completions`.
  We could either not create a JobSet until the plugin controller has finished patching the TrainJob, 
  or we delete and recreate the JobSet after the plugin is done patching the TrainJob.
  In this KEP we are opting for the delete and recreation approach.
  


### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate? Think broadly.
For example, consider both security and how this will impact the larger
Kubeflow ecosystem.
How will security be reviewed, and by whom?
How will UX be reviewed, and by whom?
Consider including folks who also work outside the SIG or subproject.
-->

1. On clusters where Kueue is configured to manage the Trainjob object, Kueue injects the
   `kueue.x-k8s.io/queue-name` label to the TrainJob objects it manages. In these cases
   Kubeflow trainer should not un-suspend the TrainJob at all leaving that decision to Kueue.
   For MultiKueue TrainJobs, Kueue sets `spec.managedBy="kueue.x-k8s.io/multikueue"`.
2. There could be other systems like Volcano managing the TrainJob. The proposal will still
   function properly because Trainer will not un-suspend the TrainJob object until it is
   ready to be considered a candidate for scheduling.
3. There is a chance that while the TrainJob is being patched by the plugin, Kueue, Volcano,
   or an equivalent framework preempts a different running workload to make room for the
   new TrainJob based on the original resource requirements of the job, which the plugin
   controller may change. The chances of this happening should be small. Even if it does
   happen, this is not a new issue that this proposal introduces. This is something that
   would have happened anyway. In fact, an intelligent plugin controller could detect
   that the capacity of the cluster is not enough for a TrainJob and size the TrainJob
   in a way that avoids preempting other workloads.
    1. Kueue supports temporarily pausing the admission checks for managed jobs 
       via the [`AdmissionGatedBy`](https://kueue.sigs.k8s.io/docs/reference/labels-and-annotations/#kueuex-k8sioadmission-gated-by)
       feature. In a future update of this KEP we could instruct users to include 
       the `AdmissionGatedBy` annotation on creation or even have the Kueue webhook
       auto-inject it when it detects the presence of the `trainer.kubeflow.org/autoconf-plugin` 
       annotation. Kubeflow Trainer would then remove the `AdmissionGatedBy` annotation
       after the autoconf-plugin declares that it has finished patching the TrainJob.

## Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

We forbid the mutation of all fields under `spec.trainer` in a TrainJob except for
`spec.trainer.numNodes`.

The protocol for TrainJobs that opt into automatic resource mutation consists of the following rules.

### Rules that the user follows

1. TrainJobs that opt-into automatic resource mutation by an `autoconf-plugin` Kubernetes Controller
   must be created with `spec.suspend=True` and with the `trainer.kubeflow.org/autoconf-plugin=${plugin-id}`
   annotation. The webhook rejects TrainJobs that carry the annotation but are not
   created in suspended state.

### Rules that Kubeflow Trainer follows

1. The webhook forbids the creation of `TrainJobs` that contain the annotation 
   `trainer.kubeflow.org/autoconf-plugin=${plugin-id}` and are not suspended.
2. Upon reconciliation of a `TrainJob` containing the annotation `trainer.kubeflow.org/autoconf-plugin=${plugin-id}`
   if the TrainJob does not have the `AutoConfPending` condition, Trainer injects the 
   condition to the TrainJob.
3. Upon reconciliation of a `TrainJob` which has the `trainer.kubeflow.org/autoconf-plugin-done=yes`
   annotation and the condition `AutoConfPending`, Trainer builds the following patch and then applies it:
   1. Removes the `AutoConfPending` condition
   2. Inserts the `AutoConfDone` condition
   3. If the TrainJob does not have a non-empty `kueue.x-k8s.io/queue-name` label AND 
      the field `spec.managedBy="trainer.kubeflow.org/trainjob-controller"` then 
      Kubeflow Trainer also sets `spec.suspend=False`
   4. If the JobSet object exists, and  `spec.replicatedJobs[].template.spec.parallelism`
      differs from `spec.trainer.numNodes`, then Trainer deletes the JobSet object.
      1. The next reconciliation iteration will create the JobSet using the updated 
      `spec.trainer.numNodes` field
      

### Rules that Kubernetes Controllers operating as autoconf-plugin follow

1. Look for suspended TrainJobs that contain the `trainer.kubeflow.org/autoconf-plugin=${plugin-id}`
    annotation which is matching the id of the autoconf-plugin.
2. If the autoconf-plugin can decide the number of nodes that the TrainJob needs, it 
    should set the `spec.trainer.numNodes`
3. When the plugin is done patching a TrainJob, it sets the annotation 
    `trainer.kubeflow.org/autoconf-plugin-done=yes`. 
4. Must not un-suspend the TrainJob. The autoconf-plugin does not manage the `spec.suspend` field.

### Test Plan

<!--
The goal is to ensure that we don't accept enhancements with inadequate testing.
All code is expected to have adequate tests (eventually with coverage
expectations). Please adhere to the Kubeflow testing guidelines when drafting this test plan.
-->

[x] I/we understand the owners of the involved components may require updates to
existing tests to make this code solid enough prior to committing the changes necessary
to implement this enhancement.

#### Prerequisite testing updates

<!--
Based on reviewers feedback describe what additional tests need to be added prior
implementing this enhancement to ensure the enhancements have also solid foundations.
-->

#### Unit Tests

<!--
In principle every added code should have complete unit test coverage, so providing
the exact set of tests will not bring additional value.
However, if complete unit test coverage is not possible, explain the reason of it
together with explanation why this is acceptable.
-->

<!--
Additionally, try to enumerate the core package you will be touching
to implement this enhancement and provide the current unit coverage for those
in the form of:
- <package>: <date> - <current test coverage>
This can inform certain test coverage improvements that we want to do before
extending the production code to implement this enhancement.
-->

- `<package>`: `<date>` - `<test coverage>`

#### E2E tests

<!--
Describe what E2E tests will be added to ensure proper quality of the enhancement.
After the implementation PR is merged, add the names of the tests here.
-->

#### Integration tests

<!--
Describe what tests will be added to ensure proper quality of the enhancement.
After the implementation PR is merged, add the names of the tests here.
-->

- TrainJob created with autoconf annotation but `spec.suspend=false` is rejected by
  the webhook.
- TrainJob created with autoconf annotation and `spec.suspend=true` gets
  the `AutoConfPending` condition set; the JobSet is created.
- For a TrainJob without a queue-name label and with a default `spec.managedBy`.
  After the plugin patches `spec.trainer.numNodes` and sets the autoconf-done annotation, 
  Trainer transitions to `AutoConfDone`, deletes any existing JobSet, and on the next 
  reconciliation creates a new JobSet reflecting the updated node count; with no Kueue 
  label and `spec.suspend` is set to `false`.
- Same as above but with a non-empty `kueue.x-k8s.io/queue-name` label: Trainer completes
  the transition but still with `spec.suspend=true`.
- Same as above but with `spec.managedBy="kueue.x-k8s.io/multikueue"`

### Graduation Criteria

<!--
This section is optional until Kubeflow has formally defined graduation criteria,
feature gates, and a deprecation policy.

Clearly define what it means for the feature to be implemented and
considered stable.
If the feature you are introducing has high complexity, consider adding graduation
milestones with these graduation criteria:
- [Maturity levels (`alpha`, `beta`, `stable`)][maturity-levels]
- [Feature gate][feature gate] lifecycle
- [Deprecation policy][deprecation-policy]
[feature gate]: https://git.k8s.io/community/contributors/devel/sig-architecture/feature-gates.md
[maturity-levels]: https://git.k8s.io/community/contributors/devel/sig-architecture/api_changes.md#alpha-beta-and-stable-versions
[deprecation-policy]: https://kubernetes.io/docs/reference/using-api/deprecation-policy/
-->

## Implementation History

<!--
Major milestones in the lifecycle of a KEP should be tracked in this section.
Major milestones might include:
- KEP Creation
- KEP Update(s)
- Implementation Start
- First Component and Kubeflow version where the KEP is released
- Component and Kubeflow version where the KEP is graduated
- When the KEP was retired or superseded
-->

- 2026-05-28: KEP Creation
- 2026-07-01: Reduce scope to just enabling resource recommenders to set the number of nodes

## Drawbacks

<!--
Why should this KEP _not_ be implemented?
-->

The main disadvantage of this approach is that it requires introducing a different code
path for TrainJobs.

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

### Proposed approach: External plugin controllers

- **Pros:**
  - **Extensible**: platform engineers can develop and deploy custom plugins without modifying Kubeflow Trainer.
- **Cons:**
  - **Reduced potential**: This first phase of the protocol sacrifices powerful features (guardrails, 
     observability, configuration of number of GPUs and other resources) for simplicity.
  - **Adds complexity**: requires gating/un-gating mechanism in Trainer to coordinate with admission/scheduling
    frameworks like Kueue and Kubernetes.
  - **API extension needed**: requires extending `spec.trainer` API to support mutating `spec.trainer.numNodes`.

### Bake recommenders into Kubeflow Trainer

- **Pros:**
  - **Reduced complexity**: Kubeflow Trainer wouldn't have to gate and un-gate the TrainJob.
    On first reconciliation, it would only apply the policy and then mutate the TrainJob.
- **Cons:**
  - **Limited extensibility**: Policies are workload-specific and rapidly evolving. Users would need to fork Kubeflow Trainer
    in order to use their custom policies.

### Use a mutating webhook instead of external controllers to patch the TrainJobs

- **Pros:**
  - **No Trainer changes**: We wouldn't need to update Kubeflow Trainer at all.
  - **Guaranteed ordering**: The mutating webhook would ensure that the TrainJob is patched before
    it is considered for admission or scheduling.
- **Cons:**
  - **Server Stability**: A recommender could take too long to come up with a suggestion (for example, it could
    run canary tests that take minutes). We do not want to increase the critical path of
    mutating webhooks, as that could compromise the stability of the cluster.
  - **Higher complexity**: The barrier of entry for plugin developers would be much higher because it is much more
    complex, harder, and riskier to implement a webhook compared to a controller.

### Users set the resource requirements of jobs following best practices and documentation

- **Pros:**
  - **No Trainer changes**: We wouldn't need to update Kubeflow Trainer at all.
- **Cons:**
  - **Manual effort**: Users would have to know the best practices and set the resource requirements correctly.
  - **Operational pain**: We would still have repeated operational pain.

### Introduce a new CRD (e.g., `AutoConfiguredTrainJob`) which wraps a `TrainJob`

- **Pros:**
  - **Reduced complexity**: This would make the Trainer feature less complex because it wouldn't have to gate and
    un-gate the TrainJob. It would only need to create the TrainJob once after the plugin has
    finished patching the `AutoConfiguredTrainJob`.
- **Cons:**
  - **Additional CRD**: There would be yet another CRD to manage.
  - **Different API**: Users would need to create a different resource instead of a TrainJob.
