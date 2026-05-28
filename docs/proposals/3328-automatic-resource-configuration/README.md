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
   API, guardrails, etc.)
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

Resource configuration of TrainJobs is not a minor detail. It is a manual decision
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
user size their jobs. Vertical and horizontal pod autoscaling don't address this
scenario out of the box because they mutate only the resource requirement fields of
downstream Pod objects but not the TrainJob object that we want to right-size. What
they are missing is the ability to mutate the TrainJob object itself to ensure
proper integration with frameworks like Kueue, as well as update fields that depend
on the resource requirements. For example, `accelerate launch` has command line
arguments that specify the number of processes to use which typically map to the
number of GPUs.

Fortunately, it is possible to leverage the characteristics of a TrainJob or even
the state of the cluster to automate the resource configuration of a TrainJob. As
such, this KEP proposes the use of external Kubernetes controllers acting as
plugins that produce resource recommendations while Kubeflow Trainer orchestrates
applying these recommendations to TrainJob objects safely.


### Goals

<!--
List the specific goals of the KEP. What is it trying to achieve? How will we
know that this has succeeded?
-->

- Automatically configure TrainJob resources before the job is eligible for admission:
  initially once, right after creation. Starting with GPU, CPU, memory, and the number
  of node replicas.
- Enforce guardrails to the operations of plugins: timeouts, quota caps, fallback
  strategies, etc.
- Allow plugins to set TrainJob parameters that depend on resources:
  e.g., accelerate/torchrun derived flags.
- Integrate with Kueue so that the job is not eligible for Kueue's admission
  flow while it is being auto-configured.
- Support a catalog of plugins: platform engineers can enable/disable
  plugins per namespace and users can pick the one they'd like to use for their TrainJob.
- Provide observability: events and TrainJob status fields reflecting plugin activity.

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
- Support TrainJobs that use DRAs: It could be future work.

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
use to mutate the resource requirements of TrainJobs that opt into this process.

TrainJobs that do not opt into automatic resource mutation are treated by Kubeflow
Trainer as usual.

The protocol for TrainJobs that opt into automatic resource mutation consists of 4 steps:
1. Kubeflow Trainer gates the admission/scheduling of the TrainJob while the plugin
   is working on mutating the resource requirements of the TrainJob.
2. The plugin that the TrainJob opts into updates the resource requirements of the TrainJob
   as well as other fields that depend on the resource requirements (e.g., cmdline args, etc.).
3. Kubeflow Trainer applies guardrails on the patch from the plugin and emits
   observability events and status updates.
4. Kubeflow Trainer un-gates the TrainJob so that the underlying Kubernetes objects are
   eventually scheduled.

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
which has the id `ai-agent-recommender`.

It is an AI agent powered resource recommender for TrainJobs. The platform engineers 
provided the AI agent with Model Context Protocol (MCP) tools that can interact with
the cluster to get more information about it (e.g. the available GPUs, the running
workloads, etc).

I write my TrainJob like I usually do and simply add the following annotation to it:

```yaml
metadata:
  annotations:
    trainer.kubeflow.org/autoconf-plugin: ai-agent-recommender
```

Soon after the TrainJob is submitted, the AI agent uses the Kubeflow API to patch my
TrainJob's resource requirements and fields that depend on them. I can use
`kubectl describe trainjob my-trainjob` to see the changes that the agent made.


### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go into as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

1. Mutating the resources may require changing command-line arguments e.g.
   the number of processes for `accelerate launch` or `torchrun`.
2. The API that a plugin uses must enable associating the changes to the plugin: we
   can use the `spec.runtimePatches` API from KEP-2170; it requires extending it to
   support mutating the CLI args and the resource requirements.
3. JobSet objects do not support mutating resource requirements.
   We should either not create a JobSet until the plugin controller has finished patching
   the TrainJob, or we should delete and recreate the JobSet after the plugin
   has finished patching the TrainJob.


### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate? Think broadly.
For example, consider both security and how this will impact the larger
Kubeflow ecosystem.
How will security be reviewed, and by whom?
How will UX be reviewed, and by whom?
Consider including folks who also work outside the SIG or subproject.
-->

1. Kueue supports an alpha feature for gating the admission of jobs called [AdmissionGatedBy](https://kueue.sigs.k8s.io/docs/reference/labels-and-annotations/#kueuex-k8sioadmission-gated-by).
   AdmissionGatedBy is behind a feature gate and is off by default. Thus, Trainer cannot
   guarantee that Kueue will respect the `kueue.x-k8s.io/admission-gated-by` annotation.
   This means that there is a chance for Kueue to admit a TrainJob before a plugin has
   finished patching the TrainJob. To mitigate this risk, Kubeflow Trainer should not 
   un-suspend the JobSet object before the plugin has finished patching the associated TrainJob.
2. There could be other systems like Volcano managing the TrainJob. The proposal will still
   function properly because Trainer will not un-suspend the JobSet object until it is
   ready to be scheduled.
3. There is a chance that while the TrainJob is being patched by the plugin, Kueue, Volcano,
   or an equivalent framework preempts a different running workload to make room for the
   new TrainJob based on the original resource requirements of the job, which the plugin
   controller may change. The chances of this happening should be small. Even if it does
   happen, this is not a new issue that this proposal introduces. This is something that
   would have happened anyway. In fact, an intelligent plugin controller could detect
   that the capacity of the cluster is not enough for a TrainJob and size the TrainJob
   in a way that avoids preempting other workloads.

## Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

TBD

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

## Drawbacks

<!--
Why should this KEP _not_ be implemented?
-->

The main disadvantage of this approach is that it requires introducing a different code
path for TrainJobs. It also requires updating the Trainer API to account for changes
to the resource requirements of TrainJobs as well as CLI args.

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

### Proposed approach: External plugin controllers

- **Pros:**
  - **Extensible**: platform engineers can develop and deploy custom plugins without modifying Kubeflow Trainer.
  - **Safe**: Trainer enforces guardrails (timeouts, quota caps) on plugin operations.
  - **Observable**: `spec.runtimePatches` shows the exact fields that the plugin is mutating,
    events and status updates offer an additional layer of visibility into the plugin's actions.
- **Cons:**
  - **Adds complexity**: requires gating/un-gating mechanism in Trainer to coordinate with admission/scheduling
    frameworks like Kueue and Kubernetes.
  - **API extension needed**: requires extending `spec.runtimePatches` to support resource and CLI arg mutations.

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
