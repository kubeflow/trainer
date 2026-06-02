# Job Scheduling

Configure gang scheduling and integrate Kubeflow Trainer with Kubernetes schedulers.

----

:::::{grid} 1 1 2 2
:gutter: 3

::::{grid-item-card} Overview
:link: overview
:link-type: doc

Introduction to gang scheduling and PodGroupPolicy
::::

::::{grid-item-card} Coscheduling
:link: coscheduling
:link-type: doc

Gang scheduling with the Coscheduling plugin
::::

::::{grid-item-card} Volcano Scheduler
:link: volcano
:link-type: doc

Advanced batch scheduling with Volcano
::::

::::{grid-item-card} Kueue
:link: https://kueue.sigs.k8s.io/docs/tasks/run/trainjobs/
:link-type: url

Job queueing and resource management with Kueue
::::

::::{grid-item-card} KAI Scheduler
:link: kai
:link-type: doc

Gang scheduling with NVIDIA KAI Scheduler
::::

:::::

----

```{toctree}
:hidden:
:maxdepth: 1

overview
coscheduling
volcano
Kueue <https://kueue.sigs.k8s.io/docs/tasks/run/trainjobs/>
kai
```
