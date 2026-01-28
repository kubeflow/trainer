# Changelog

# [v2.1.0](https://github.com/kubeflow/trainer/tree/v2.1.0) (2025-11-07)

This is Kubeflow Trainer v2.1.0 release.

```bash
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/manager?ref=v2.1.0"
kubectl apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/overlays/runtimes?ref=v2.1.0"
```

You can now install controller manager with Helm charts 🚀

```bash
helm install kubeflow-trainer oci://ghcr.io/kubeflow/charts/kubeflow-trainer --version 2.1.0
```

For more information, please see [the Kubeflow Trainer docs](https://www.kubeflow.org/docs/components/trainer/overview/)

## Breaking Changes

- feat(api): Replace deprecated PodSpecOverrides API with PodTemplateOverrides in TrainJob ([#2785](https://github.com/kubeflow/trainer/pull/2785) by [@xigang](https://github.com/xigang))
- feat(operator): Replace TrainJob controller settings with the Config API ([#2879](https://github.com/kubeflow/trainer/pull/2879) by [@kapil27](https://github.com/kapil27))
- chore(operator): Upgrade JobSet to v0.10.1 ([#2875](https://github.com/kubeflow/trainer/pull/2875) by [@astefanutti](https://github.com/astefanutti))
- chore(operator): Upgrade Kubernetes to v1.34 ([#2804](https://github.com/kubeflow/trainer/pull/2804) by [@astefanutti](https://github.com/astefanutti))
- Upgrade Kubernetes to v1.33 ([#2756](https://github.com/kubeflow/trainer/pull/2756) by [@astefanutti](https://github.com/astefanutti))

## New Features

### Distributed AI Data Cache

- feat(cache): KEP-2655: Adding default runtime with cache and example ([#2928](https://github.com/kubeflow/trainer/pull/2928) by [@akshaychitneni](https://github.com/akshaychitneni))
- feat(cache): KEP-2655 - Supporting readiness probes on cache nodes ([#2920](https://github.com/kubeflow/trainer/pull/2920) by [@akshaychitneni](https://github.com/akshaychitneni))
- feat(cache): KEP-2655 - Add build pipeline and address vulnerabilities for data_cache ([#2890](https://github.com/kubeflow/trainer/pull/2890) by [@akshaychitneni](https://github.com/akshaychitneni))
- feat(cache): KEP-2655: Adding cache initializer ([#2793](https://github.com/kubeflow/trainer/pull/2793) by [@akshaychitneni](https://github.com/akshaychitneni))
- feat: KEP-2655: Add data cache system ([#2755](https://github.com/kubeflow/trainer/pull/2755) by [@akshaychitneni](https://github.com/akshaychitneni))

### LLM Post-Training

- feat(runtimes): Add LoRA/QLoRA/DoRA support in LLM Trainer V2 ([#2832](https://github.com/kubeflow/trainer/pull/2832) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- feat: Add Qwen 2.5 1.5b runtime, example and fix gpu e2e test ([#2835](https://github.com/kubeflow/trainer/pull/2835) by [@jaiakash](https://github.com/jaiakash))
- feat(runtimes): Support Distributed MLX on CUDA ([#2790](https://github.com/kubeflow/trainer/pull/2790) by [@andreyvelich](https://github.com/andreyvelich))

### Kueue Enhancements

- Support Topology Aware Scheduling for TrainJobs ([kubernetes-sigs/kueue#7249](https://github.com/kubernetes-sigs/kueue/pull/7249) by [@kaisoz](https://github.com/kaisoz))
- fix: Allow multiple podSpec overrides to target the same TargetJob ([#2880](https://github.com/kubeflow/trainer/pull/2880) by [@kaisoz](https://github.com/kaisoz))
- feat: support affinity in TrainJob pod spec overrides ([#2796](https://github.com/kubeflow/trainer/pull/2796) by [@toVersus](https://github.com/toVersus))
- feat: Add schedulingGates to PodSpecOverrides ([#2700](https://github.com/kubeflow/trainer/pull/2700) by [@astefanutti](https://github.com/astefanutti))

### Volcano Scheduler

- feat: KEP-2437 - PodGroup Creation for Volcano Scheduler ([#2729](https://github.com/kubeflow/trainer/pull/2729) by [@Doris-xm](https://github.com/Doris-xm))
- feat(docs): KEP-2437-Support Volcano Scheduler in Kubeflow Trainer V2 ([#2672](https://github.com/kubeflow/trainer/pull/2672) by [@Doris-xm](https://github.com/Doris-xm))

### API Updates

- feat(runtimes): add support for launcher resource allocation in MPI jobs ([#2653](https://github.com/kubeflow/trainer/pull/2653) by [@jskswamy](https://github.com/jskswamy))
- feat: Add PodTemplateOverrides into TrainJob V2 API ([#2882](https://github.com/kubeflow/trainer/pull/2882) by [@xigang](https://github.com/xigang))
- feat(api): Sync TrainJob JobsStatus from JobSet ReplicatedJobsStatus ([#2802](https://github.com/kubeflow/trainer/pull/2802) by [@astefanutti](https://github.com/astefanutti))
- feat: support imagePullSecrets in TrainJob pod spec overrides ([#2806](https://github.com/kubeflow/trainer/pull/2806) by [@toVersus](https://github.com/toVersus))
- feat(operator): enforce RFC 1035 validation for TrainJob name ([#2767](https://github.com/kubeflow/trainer/pull/2767) by [@juniemariam](https://github.com/juniemariam))

## Bug Fixes

- [release-2.1] fix(ci): Fix the Kubeflow SDK installation with Docker ([#2927](https://github.com/kubeflow/trainer/pull/2927) by [@andreyvelich](https://github.com/andreyvelich))
- fix(manifests): Add RBAC rules for Leases in Helm Charts ([#2901](https://github.com/kubeflow/trainer/pull/2901) by [@astefanutti](https://github.com/astefanutti))
- fix(docs): correct example usage in KEP-2437-Support-Volcano-Scheduler ([#2898](https://github.com/kubeflow/trainer/pull/2898) by [@Doris-xm](https://github.com/Doris-xm))
- fix(api): Keep mpiImplementation field a pointer ([#2897](https://github.com/kubeflow/trainer/pull/2897) by [@astefanutti](https://github.com/astefanutti))
- fix(api): Fix lint errors for the config API ([#2896](https://github.com/kubeflow/trainer/pull/2896) by [@astefanutti](https://github.com/astefanutti))
- fix: charts dependencies ([#2892](https://github.com/kubeflow/trainer/pull/2892) by [@ls-2018](https://github.com/ls-2018))
- fix(runtimes): fix missing dependency in torchtune trainer image. ([#2887](https://github.com/kubeflow/trainer/pull/2887) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(ci): Add latest image tag only for the master branch ([#2854](https://github.com/kubeflow/trainer/pull/2854) by [@andreyvelich](https://github.com/andreyvelich))
- fix: read only permission for PRs ([#2829](https://github.com/kubeflow/trainer/pull/2829) by [@jaiakash](https://github.com/jaiakash))
- fix: read only permission for PRs ([#2827](https://github.com/kubeflow/trainer/pull/2827) by [@jaiakash](https://github.com/jaiakash))
- fix: update examples to reflect func_args now being unpacked ([#2815](https://github.com/kubeflow/trainer/pull/2815) by [@briangallagher](https://github.com/briangallagher))
- fix(examples): Update get_job_logs() API in examples ([#2813](https://github.com/kubeflow/trainer/pull/2813) by [@andreyvelich](https://github.com/andreyvelich))
- fix: teraform for oci gpu based vm ([#2810](https://github.com/kubeflow/trainer/pull/2810) by [@jaiakash](https://github.com/jaiakash))
- fix(api): Regenerate TrainJob CRD ([#2805](https://github.com/kubeflow/trainer/pull/2805) by [@astefanutti](https://github.com/astefanutti))
- fix(ci): disable `Unit and Integration Test - Go` gh action in forked repos ([#2746](https://github.com/kubeflow/trainer/pull/2746) by [@milinddethe15](https://github.com/milinddethe15))
- fix(manifests): Add missing permissions for the RuntimeClass and LimitRange ([#2787](https://github.com/kubeflow/trainer/pull/2787) by [@tenzen-y](https://github.com/tenzen-y))
- fix: update kubeflow sdk reference ([#2780](https://github.com/kubeflow/trainer/pull/2780) by [@kramaranya](https://github.com/kramaranya))
- fix(api): update license path for kubeflow_trainer_api ([#2778](https://github.com/kubeflow/trainer/pull/2778) by [@kramaranya](https://github.com/kramaranya))
- fix(runtimes): Set numProcPerNode: 1 in DeepSpeed Runtime ([#2774](https://github.com/kubeflow/trainer/pull/2774) by [@andreyvelich](https://github.com/andreyvelich))
- fix(docs): update KEP-2401 according to current implementation. ([#2765](https://github.com/kubeflow/trainer/pull/2765) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(ci): Remove coverage from Go integration tests ([#2773](https://github.com/kubeflow/trainer/pull/2773) by [@andreyvelich](https://github.com/andreyvelich))
- fix(api): Fix license path for Kubeflow Trainer Python API ([#2771](https://github.com/kubeflow/trainer/pull/2771) by [@andreyvelich](https://github.com/andreyvelich))
- fix(examples): Update the argument for Runtime framework ([#2766](https://github.com/kubeflow/trainer/pull/2766) by [@andreyvelich](https://github.com/andreyvelich))
- fix(test): Fix Ginkgo command for integration tests ([#2758](https://github.com/kubeflow/trainer/pull/2758) by [@astefanutti](https://github.com/astefanutti))
- fix: fix the command for fetching Kubeflow Trainer version in the issue template ([#2732](https://github.com/kubeflow/trainer/pull/2732) by [@rudeigerc](https://github.com/rudeigerc))
- fix(manifests): add rbac config of events for event recorders ([#2731](https://github.com/kubeflow/trainer/pull/2731) by [@rudeigerc](https://github.com/rudeigerc))
- fix(manifests): fix position of labels of dataset-initializer from pod to job ([#2719](https://github.com/kubeflow/trainer/pull/2719) by [@rudeigerc](https://github.com/rudeigerc))
- fix(module): Change Go module name to v2 ([#2707](https://github.com/kubeflow/trainer/pull/2707) by [@andreyvelich](https://github.com/andreyvelich))
- fix(plugins): Fix some errors in torchtune mutation process. ([#2675](https://github.com/kubeflow/trainer/pull/2675) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(manifests): Update manifests to enable LLM fine-tuning workflow with CTR and TrainJob yaml files ([#2669](https://github.com/kubeflow/trainer/pull/2669) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(rbac): Add required RBAC to update ClusterTrainingRuntimes on OpenShift ([#2682](https://github.com/kubeflow/trainer/pull/2682) by [@astefanutti](https://github.com/astefanutti))

## Misc

- [release-2.1] feat: Adding local execution example notebook  ([#2924](https://github.com/kubeflow/trainer/pull/2924) by [@Fiona-Waters](https://github.com/Fiona-Waters))
- feat(manifests): Publish Kubeflow Trainer Helm charts ([#2917](https://github.com/kubeflow/trainer/pull/2917) by [@adity1raut](https://github.com/adity1raut))
- [release-2.1] chore(operator): Use SSA throughout runtime framework ([#2912](https://github.com/kubeflow/trainer/pull/2912) by [@astefanutti](https://github.com/astefanutti))
- [release-2.1] feat(initializer): add s3 model and dataset initializers ([#2911](https://github.com/kubeflow/trainer/pull/2911) by [@rudeigerc](https://github.com/rudeigerc))
- feat(operator): Add validation for required containers in replicatedJobs ([#2722](https://github.com/kubeflow/trainer/pull/2722) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- feat: add controller manager configuration helm chart ([#2895](https://github.com/kubeflow/trainer/pull/2895) by [@kapil27](https://github.com/kapil27))
- chore(ci): Enable Kubernetes API Linter ([#2858](https://github.com/kubeflow/trainer/pull/2858) by [@astefanutti](https://github.com/astefanutti))
- feat(runtimes): implement clusterTrainingRuntime deprecation process ([#2791](https://github.com/kubeflow/trainer/pull/2791) by [@tdn21](https://github.com/tdn21))
- feat: add HF token and allow gpu workflow to run from pull request target ([#2818](https://github.com/kubeflow/trainer/pull/2818) by [@jaiakash](https://github.com/jaiakash))
- feat(docs): KEP-2442-Support JAX Training Runtime ([#2643](https://github.com/kubeflow/trainer/pull/2643) by [@mahdikhashan](https://github.com/mahdikhashan))
- chore(test): Support e2e cluster setup with Podman ([#2861](https://github.com/kubeflow/trainer/pull/2861) by [@astefanutti](https://github.com/astefanutti))
- chore(runtimes): Upgrade torchtune version to v0.6.1 ([#2876](https://github.com/kubeflow/trainer/pull/2876) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- chore(operator): Upgrade JobSet to v0.10.1 ([#2875](https://github.com/kubeflow/trainer/pull/2875) by [@astefanutti](https://github.com/astefanutti))
- feat(docs): Update Trainer diagram and SDK release ([#2867](https://github.com/kubeflow/trainer/pull/2867) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): Add changelog for Kubeflow Trainer v2.0.1 ([#2864](https://github.com/kubeflow/trainer/pull/2864) by [@andreyvelich](https://github.com/andreyvelich))
- fix(docs): Update the release document to push all changes ([#2865](https://github.com/kubeflow/trainer/pull/2865) by [@andreyvelich](https://github.com/andreyvelich))
- chore: Install released version of Kubeflow SDK ([#2857](https://github.com/kubeflow/trainer/pull/2857) by [@kramaranya](https://github.com/kramaranya))
- chore(ci): Ignore generated files in .gitattributes ([#2855](https://github.com/kubeflow/trainer/pull/2855) by [@andreyvelich](https://github.com/andreyvelich))
- feat: Add a public function to create runtime info objects ([#2837](https://github.com/kubeflow/trainer/pull/2837) by [@kaisoz](https://github.com/kaisoz))
- chore(test): add uts for coscheduling plugin. ([#2582](https://github.com/kubeflow/trainer/pull/2582) by [@IRONICBo](https://github.com/IRONICBo))
- feat(ci): Add Trivy Vulnerability Scan ([#2826](https://github.com/kubeflow/trainer/pull/2826) by [@andreyvelich](https://github.com/andreyvelich))
- chore: merge test cases using PodSpecOverrides into a single case ([#2822](https://github.com/kubeflow/trainer/pull/2822) by [@toVersus](https://github.com/toVersus))
- chore(runtimes): update torchtune CTRs with multiple dependson feature in jobset v0.9.0 ([#2823](https://github.com/kubeflow/trainer/pull/2823) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- chore(operator): Bump JobSet to v0.9.0 version ([#2821](https://github.com/kubeflow/trainer/pull/2821) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): How to release Python API modules ([#2786](https://github.com/kubeflow/trainer/pull/2786) by [@andreyvelich](https://github.com/andreyvelich))
- feat: support for managing gpu enabled self runner infra ([#2762](https://github.com/kubeflow/trainer/pull/2762) by [@jaiakash](https://github.com/jaiakash))
- chore: Nominate @astefanutti as Kubeflow Trainer approver ([#2808](https://github.com/kubeflow/trainer/pull/2808) by [@andreyvelich](https://github.com/andreyvelich))
- chore: deflake test to ensure runtime is created before creating trainjob ([#2807](https://github.com/kubeflow/trainer/pull/2807) by [@toVersus](https://github.com/toVersus))
- feat: KEP-2432: GPU Testing for LLM Blueprints ([#2689](https://github.com/kubeflow/trainer/pull/2689) by [@jaiakash](https://github.com/jaiakash))
- chore(docs): Add license scan report and status ([#2788](https://github.com/kubeflow/trainer/pull/2788) by [@fossabot](https://github.com/fossabot))
- chore: Remove tool.hatch.build.targets.wheel from pyproject ([#2803](https://github.com/kubeflow/trainer/pull/2803) by [@kramaranya](https://github.com/kramaranya))
- chore: Add unit tests for `pkg/apply` ([#2479](https://github.com/kubeflow/trainer/pull/2479) by [@akagami-harsh](https://github.com/akagami-harsh))
- chore(runtimes): Remove MPI pi Runtime ([#2760](https://github.com/kubeflow/trainer/pull/2760) by [@andreyvelich](https://github.com/andreyvelich))
- chore(runtimes): Update packages in DeepSpeed runtime and fix T5 example ([#2781](https://github.com/kubeflow/trainer/pull/2781) by [@andreyvelich](https://github.com/andreyvelich))
- feat: run workflows on `/ok-to-test` label ([#2639](https://github.com/kubeflow/trainer/pull/2639) by [@milinddethe15](https://github.com/milinddethe15))
- feat: Add security contexts to controller managers ([#2759](https://github.com/kubeflow/trainer/pull/2759) by [@kunal-511](https://github.com/kunal-511))
- feat(docs): Introduce latest news to the README ([#2769](https://github.com/kubeflow/trainer/pull/2769) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtimes): Add Framework Label to the Runtimes ([#2761](https://github.com/kubeflow/trainer/pull/2761) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtimes): Remove command from the Runtimes with CustomTrainer ([#2754](https://github.com/kubeflow/trainer/pull/2754) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): Kubeflow Trainer ROADMAP 2025 ([#2748](https://github.com/kubeflow/trainer/pull/2748) by [@andreyvelich](https://github.com/andreyvelich))
- chore(docs): Add Changelog for Kubeflow Trainer v2.0.0 ([#2743](https://github.com/kubeflow/trainer/pull/2743) by [@andreyvelich](https://github.com/andreyvelich))
- chore: update github runners to oci gh arc runners ([#2739](https://github.com/kubeflow/trainer/pull/2739) by [@koksay](https://github.com/koksay))
- feat(operator): force trainjob name to be compliant with RFC 1035 for jobset ([#2734](https://github.com/kubeflow/trainer/pull/2734) by [@rudeigerc](https://github.com/rudeigerc))
- chore(ci): Add GitHub action to verify PR titles ([#2724](https://github.com/kubeflow/trainer/pull/2724) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): Guide to report security vulnerability ([#2718](https://github.com/kubeflow/trainer/pull/2718) by [@andreyvelich](https://github.com/andreyvelich))
- chore: Upgrade JobSet to version 0.8.2 ([#2726](https://github.com/kubeflow/trainer/pull/2726) by [@astefanutti](https://github.com/astefanutti))
- Add Red Hat to ADOPTERS.md ([#2714](https://github.com/kubeflow/trainer/pull/2714) by [@terrytangyuan](https://github.com/terrytangyuan))
- chore(docs): Add Changelog for v2.0.0-rc.1 ([#2709](https://github.com/kubeflow/trainer/pull/2709) by [@andreyvelich](https://github.com/andreyvelich))
- chore(docs): Update Release Guide ([#2710](https://github.com/kubeflow/trainer/pull/2710) by [@andreyvelich](https://github.com/andreyvelich))
- chore: Copy generated CRDs into Helm charts ([#2703](https://github.com/kubeflow/trainer/pull/2703) by [@astefanutti](https://github.com/astefanutti))
- feat(example): Add alpaca-trianjob-yaml.ipynb. ([#2670](https://github.com/kubeflow/trainer/pull/2670) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- feat: Mutable PodSpecOverrides for suspended TrainJob ([#2683](https://github.com/kubeflow/trainer/pull/2683) by [@astefanutti](https://github.com/astefanutti))
- chore: Replace the deprecated intstr.FromInt with intstr.FromInt32 ([#2695](https://github.com/kubeflow/trainer/pull/2695) by [@tenzen-y](https://github.com/tenzen-y))
- chore: Remove the vendor specific parameters ([#2691](https://github.com/kubeflow/trainer/pull/2691) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Add the manifests overlay for Kubeflow Training V2 ([#2382](https://github.com/kubeflow/trainer/pull/2382) by [@Doris-xm](https://github.com/Doris-xm))
- chore(runtime): Bump Torch to 2.7.1 and DeepSpeed to 0.17.1 ([#2685](https://github.com/kubeflow/trainer/pull/2685) by [@andreyvelich](https://github.com/andreyvelich))
- chore(helm): Sync ClusterRule in Helm chart ([#2686](https://github.com/kubeflow/trainer/pull/2686) by [@astefanutti](https://github.com/astefanutti))
- Add Changelog for Trainer v2.0.0-rc.0 ([#2666](https://github.com/kubeflow/trainer/pull/2666) by [@kramaranya](https://github.com/kramaranya))
- feat(initializer): Updated base image to Debian image and changed install commands compatible with Debian image ([#2528](https://github.com/kubeflow/trainer/pull/2528) by [@Debabrata47](https://github.com/Debabrata47))

[Full Changelog](https://github.com/kubeflow/trainer/compare/v2.0.1...v2.1.0)

# [v2.1.0-rc.1](https://github.com/kubeflow/trainer/tree/v2.1.0-rc.1) (2025-11-03)

## New Features

- feat(manifests): Publish Kubeflow Trainer Helm charts ([#2917](https://github.com/kubeflow/trainer/pull/2917) by [@adity1raut](https://github.com/adity1raut))
- [release-2.1] chore(operator): Use SSA throughout runtime framework ([#2912](https://github.com/kubeflow/trainer/pull/2912) by [@astefanutti](https://github.com/astefanutti))
- [release-2.1] feat(initializer): add s3 model and dataset initializers ([#2911](https://github.com/kubeflow/trainer/pull/2911) by [@rudeigerc](https://github.com/rudeigerc))

## Bug Fixes

- [release-2.1] fix(manifests): Fix boolean values defaulting in Helm charts ([#2914](https://github.com/kubeflow/trainer/pull/2914) by [@astefanutti](https://github.com/astefanutti))
- [release-2.1] fix(runtimes): Update pip version in the MLX runtime ([#2910](https://github.com/kubeflow/trainer/pull/2910) by [@andreyvelich](https://github.com/andreyvelich))

[Full Changelog](https://github.com/kubeflow/trainer/compare/v2.1.0-rc.0...v2.1.0-rc.1)

# [v2.1.0-rc.0](https://github.com/kubeflow/trainer/tree/v2.1.0-rc.0) (2025-10-21)

## Breaking Changes

- feat(api): Replace deprecated PodSpecOverrides API with PodTemplateOverrides in TrainJob ([#2785](https://github.com/kubeflow/trainer/pull/2785) by [@xigang](https://github.com/xigang))
- feat(operator): Replace TrainJob controller settings with the Config API ([#2879](https://github.com/kubeflow/trainer/pull/2879) by [@kapil27](https://github.com/kapil27))
- chore(operator): Upgrade JobSet to v0.10.1 ([#2875](https://github.com/kubeflow/trainer/pull/2875) by [@astefanutti](https://github.com/astefanutti))
- chore(operator): Upgrade Kubernetes to v1.34 ([#2804](https://github.com/kubeflow/trainer/pull/2804) by [@astefanutti](https://github.com/astefanutti))
- Upgrade Kubernetes to v1.33 ([#2756](https://github.com/kubeflow/trainer/pull/2756) by [@astefanutti](https://github.com/astefanutti))

## New Features

### Distributed AI Data Cache

- feat(cache): KEP-2655 - Add build pipeline and address vulnerabilities for data_cache ([#2890](https://github.com/kubeflow/trainer/pull/2890) by [@akshaychitneni](https://github.com/akshaychitneni))
- feat(cache): KEP-2655: Adding cache initializer ([#2793](https://github.com/kubeflow/trainer/pull/2793) by [@akshaychitneni](https://github.com/akshaychitneni))
- feat: KEP-2655: Add data cache system ([#2755](https://github.com/kubeflow/trainer/pull/2755) by [@akshaychitneni](https://github.com/akshaychitneni))

### LLM Post-Training

- feat(runtimes): Add LoRA/QLoRA/DoRA support in LLM Trainer V2 ([#2832](https://github.com/kubeflow/trainer/pull/2832) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- feat: Add Qwen 2.5 1.5b runtime, example and fix gpu e2e test ([#2835](https://github.com/kubeflow/trainer/pull/2835) by [@jaiakash](https://github.com/jaiakash))
- feat(runtimes): Support Distributed MLX on CUDA ([#2790](https://github.com/kubeflow/trainer/pull/2790) by [@andreyvelich](https://github.com/andreyvelich))

### Kueue Enhancements

- Support Topology Aware Scheduling for TrainJobs ([kubernetes-sigs/kueue#7249](https://github.com/kubernetes-sigs/kueue/pull/7249) by [@kaisoz](https://github.com/kaisoz))
- fix: Allow multiple podSpec overrides to target the same TargetJob ([#2880](https://github.com/kubeflow/trainer/pull/2880) by [@kaisoz](https://github.com/kaisoz))
- feat: support affinity in TrainJob pod spec overrides ([#2796](https://github.com/kubeflow/trainer/pull/2796) by [@toVersus](https://github.com/toVersus))
- feat: Add schedulingGates to PodSpecOverrides ([#2700](https://github.com/kubeflow/trainer/pull/2700) by [@astefanutti](https://github.com/astefanutti))

### Volcano Scheduler

- feat: KEP-2437 - PodGroup Creation for Volcano Scheduler ([#2729](https://github.com/kubeflow/trainer/pull/2729) by [@Doris-xm](https://github.com/Doris-xm))
- feat(docs): KEP-2437-Support Volcano Scheduler in Kubeflow Trainer V2 ([#2672](https://github.com/kubeflow/trainer/pull/2672) by [@Doris-xm](https://github.com/Doris-xm))

### API Updates

- feat(runtimes): add support for launcher resource allocation in MPI jobs ([#2653](https://github.com/kubeflow/trainer/pull/2653) by [@jskswamy](https://github.com/jskswamy))
- feat: Add PodTemplateOverrides into TrainJob V2 API ([#2882](https://github.com/kubeflow/trainer/pull/2882) by [@xigang](https://github.com/xigang))
- feat(api): Sync TrainJob JobsStatus from JobSet ReplicatedJobsStatus ([#2802](https://github.com/kubeflow/trainer/pull/2802) by [@astefanutti](https://github.com/astefanutti))
- feat: support imagePullSecrets in TrainJob pod spec overrides ([#2806](https://github.com/kubeflow/trainer/pull/2806) by [@toVersus](https://github.com/toVersus))
- feat(operator): enforce RFC 1035 validation for TrainJob name ([#2767](https://github.com/kubeflow/trainer/pull/2767) by [@juniemariam](https://github.com/juniemariam))

## Bug Fixes

- fix(manifests): Add RBAC rules for Leases in Helm Charts ([#2901](https://github.com/kubeflow/trainer/pull/2901) by [@astefanutti](https://github.com/astefanutti))
- fix(docs): correct example usage in KEP-2437-Support-Volcano-Scheduler ([#2898](https://github.com/kubeflow/trainer/pull/2898) by [@Doris-xm](https://github.com/Doris-xm))
- fix(api): Keep mpiImplementation field a pointer ([#2897](https://github.com/kubeflow/trainer/pull/2897) by [@astefanutti](https://github.com/astefanutti))
- fix(api): Fix lint errors for the config API ([#2896](https://github.com/kubeflow/trainer/pull/2896) by [@astefanutti](https://github.com/astefanutti))
- fix: charts dependencies ([#2892](https://github.com/kubeflow/trainer/pull/2892) by [@ls-2018](https://github.com/ls-2018))
- fix(runtimes): fix missing dependency in torchtune trainer image. ([#2887](https://github.com/kubeflow/trainer/pull/2887) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(ci): Add latest image tag only for the master branch ([#2854](https://github.com/kubeflow/trainer/pull/2854) by [@andreyvelich](https://github.com/andreyvelich))
- fix: read only permission for PRs ([#2829](https://github.com/kubeflow/trainer/pull/2829) by [@jaiakash](https://github.com/jaiakash))
- fix: read only permission for PRs ([#2827](https://github.com/kubeflow/trainer/pull/2827) by [@jaiakash](https://github.com/jaiakash))
- fix: update examples to reflect func_args now being unpacked ([#2815](https://github.com/kubeflow/trainer/pull/2815) by [@briangallagher](https://github.com/briangallagher))
- fix(examples): Update get_job_logs() API in examples ([#2813](https://github.com/kubeflow/trainer/pull/2813) by [@andreyvelich](https://github.com/andreyvelich))
- fix: teraform for oci gpu based vm ([#2810](https://github.com/kubeflow/trainer/pull/2810) by [@jaiakash](https://github.com/jaiakash))
- fix(api): Regenerate TrainJob CRD ([#2805](https://github.com/kubeflow/trainer/pull/2805) by [@astefanutti](https://github.com/astefanutti))
- fix(ci): disable `Unit and Integration Test - Go` gh action in forked repos ([#2746](https://github.com/kubeflow/trainer/pull/2746) by [@milinddethe15](https://github.com/milinddethe15))
- fix(manifests): Add missing permissions for the RuntimeClass and LimitRange ([#2787](https://github.com/kubeflow/trainer/pull/2787) by [@tenzen-y](https://github.com/tenzen-y))
- fix: update kubeflow sdk reference ([#2780](https://github.com/kubeflow/trainer/pull/2780) by [@kramaranya](https://github.com/kramaranya))
- fix(api): update license path for kubeflow_trainer_api ([#2778](https://github.com/kubeflow/trainer/pull/2778) by [@kramaranya](https://github.com/kramaranya))
- fix(runtimes): Set numProcPerNode: 1 in DeepSpeed Runtime ([#2774](https://github.com/kubeflow/trainer/pull/2774) by [@andreyvelich](https://github.com/andreyvelich))
- fix(docs): update KEP-2401 according to current implementation. ([#2765](https://github.com/kubeflow/trainer/pull/2765) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(ci): Remove coverage from Go integration tests ([#2773](https://github.com/kubeflow/trainer/pull/2773) by [@andreyvelich](https://github.com/andreyvelich))
- fix(api): Fix license path for Kubeflow Trainer Python API ([#2771](https://github.com/kubeflow/trainer/pull/2771) by [@andreyvelich](https://github.com/andreyvelich))
- fix(examples): Update the argument for Runtime framework ([#2766](https://github.com/kubeflow/trainer/pull/2766) by [@andreyvelich](https://github.com/andreyvelich))
- fix(test): Fix Ginkgo command for integration tests ([#2758](https://github.com/kubeflow/trainer/pull/2758) by [@astefanutti](https://github.com/astefanutti))
- fix: fix the command for fetching Kubeflow Trainer version in the issue template ([#2732](https://github.com/kubeflow/trainer/pull/2732) by [@rudeigerc](https://github.com/rudeigerc))
- fix(manifests): add rbac config of events for event recorders ([#2731](https://github.com/kubeflow/trainer/pull/2731) by [@rudeigerc](https://github.com/rudeigerc))
- fix(manifests): fix position of labels of dataset-initializer from pod to job ([#2719](https://github.com/kubeflow/trainer/pull/2719) by [@rudeigerc](https://github.com/rudeigerc))
- fix(module): Change Go module name to v2 ([#2707](https://github.com/kubeflow/trainer/pull/2707) by [@andreyvelich](https://github.com/andreyvelich))
- fix(plugins): Fix some errors in torchtune mutation process. ([#2675](https://github.com/kubeflow/trainer/pull/2675) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(manifests): Update manifests to enable LLM fine-tuning workflow with CTR and TrainJob yaml files ([#2669](https://github.com/kubeflow/trainer/pull/2669) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(rbac): Add required RBAC to update ClusterTrainingRuntimes on OpenShift ([#2682](https://github.com/kubeflow/trainer/pull/2682) by [@astefanutti](https://github.com/astefanutti))

## Misc

- feat(operator): Add validation for required containers in replicatedJobs ([#2722](https://github.com/kubeflow/trainer/pull/2722) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- feat: add controller manager configuration helm chart ([#2895](https://github.com/kubeflow/trainer/pull/2895) by [@kapil27](https://github.com/kapil27))
- chore(ci): Enable Kubernetes API Linter ([#2858](https://github.com/kubeflow/trainer/pull/2858) by [@astefanutti](https://github.com/astefanutti))
- feat(runtimes): implement clusterTrainingRuntime deprecation process ([#2791](https://github.com/kubeflow/trainer/pull/2791) by [@tdn21](https://github.com/tdn21))
- feat: add HF token and allow gpu workflow to run from pull request target ([#2818](https://github.com/kubeflow/trainer/pull/2818) by [@jaiakash](https://github.com/jaiakash))
- feat(docs): KEP-2442-Support JAX Training Runtime ([#2643](https://github.com/kubeflow/trainer/pull/2643) by [@mahdikhashan](https://github.com/mahdikhashan))
- chore(test): Support e2e cluster setup with Podman ([#2861](https://github.com/kubeflow/trainer/pull/2861) by [@astefanutti](https://github.com/astefanutti))
- chore(runtimes): Upgrade torchtune version to v0.6.1 ([#2876](https://github.com/kubeflow/trainer/pull/2876) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- chore(operator): Upgrade JobSet to v0.10.1 ([#2875](https://github.com/kubeflow/trainer/pull/2875) by [@astefanutti](https://github.com/astefanutti))
- feat(docs): Update Trainer diagram and SDK release ([#2867](https://github.com/kubeflow/trainer/pull/2867) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): Add changelog for Kubeflow Trainer v2.0.1 ([#2864](https://github.com/kubeflow/trainer/pull/2864) by [@andreyvelich](https://github.com/andreyvelich))
- fix(docs): Update the release document to push all changes ([#2865](https://github.com/kubeflow/trainer/pull/2865) by [@andreyvelich](https://github.com/andreyvelich))
- chore: Install released version of Kubeflow SDK ([#2857](https://github.com/kubeflow/trainer/pull/2857) by [@kramaranya](https://github.com/kramaranya))
- chore(ci): Ignore generated files in .gitattributes ([#2855](https://github.com/kubeflow/trainer/pull/2855) by [@andreyvelich](https://github.com/andreyvelich))
- feat: Add a public function to create runtime info objects ([#2837](https://github.com/kubeflow/trainer/pull/2837) by [@kaisoz](https://github.com/kaisoz))
- chore(test): add uts for coscheduling plugin. ([#2582](https://github.com/kubeflow/trainer/pull/2582) by [@IRONICBo](https://github.com/IRONICBo))
- feat(ci): Add Trivy Vulnerability Scan ([#2826](https://github.com/kubeflow/trainer/pull/2826) by [@andreyvelich](https://github.com/andreyvelich))
- chore: merge test cases using PodSpecOverrides into a single case ([#2822](https://github.com/kubeflow/trainer/pull/2822) by [@toVersus](https://github.com/toVersus))
- chore(runtimes): update torchtune CTRs with multiple dependson feature in jobset v0.9.0 ([#2823](https://github.com/kubeflow/trainer/pull/2823) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- chore(operator): Bump JobSet to v0.9.0 version ([#2821](https://github.com/kubeflow/trainer/pull/2821) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): How to release Python API modules ([#2786](https://github.com/kubeflow/trainer/pull/2786) by [@andreyvelich](https://github.com/andreyvelich))
- feat: support for managing gpu enabled self runner infra ([#2762](https://github.com/kubeflow/trainer/pull/2762) by [@jaiakash](https://github.com/jaiakash))
- chore: Nominate @astefanutti as Kubeflow Trainer approver ([#2808](https://github.com/kubeflow/trainer/pull/2808) by [@andreyvelich](https://github.com/andreyvelich))
- chore: deflake test to ensure runtime is created before creating trainjob ([#2807](https://github.com/kubeflow/trainer/pull/2807) by [@toVersus](https://github.com/toVersus))
- feat: KEP-2432: GPU Testing for LLM Blueprints ([#2689](https://github.com/kubeflow/trainer/pull/2689) by [@jaiakash](https://github.com/jaiakash))
- chore(docs): Add license scan report and status ([#2788](https://github.com/kubeflow/trainer/pull/2788) by [@fossabot](https://github.com/fossabot))
- chore: Remove tool.hatch.build.targets.wheel from pyproject ([#2803](https://github.com/kubeflow/trainer/pull/2803) by [@kramaranya](https://github.com/kramaranya))
- chore: Add unit tests for `pkg/apply` ([#2479](https://github.com/kubeflow/trainer/pull/2479) by [@akagami-harsh](https://github.com/akagami-harsh))
- chore(runtimes): Remove MPI pi Runtime ([#2760](https://github.com/kubeflow/trainer/pull/2760) by [@andreyvelich](https://github.com/andreyvelich))
- chore(runtimes): Update packages in DeepSpeed runtime and fix T5 example ([#2781](https://github.com/kubeflow/trainer/pull/2781) by [@andreyvelich](https://github.com/andreyvelich))
- feat: run workflows on `/ok-to-test` label ([#2639](https://github.com/kubeflow/trainer/pull/2639) by [@milinddethe15](https://github.com/milinddethe15))
- feat: Add security contexts to controller managers ([#2759](https://github.com/kubeflow/trainer/pull/2759) by [@kunal-511](https://github.com/kunal-511))
- feat(docs): Introduce latest news to the README ([#2769](https://github.com/kubeflow/trainer/pull/2769) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtimes): Add Framework Label to the Runtimes ([#2761](https://github.com/kubeflow/trainer/pull/2761) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtimes): Remove command from the Runtimes with CustomTrainer ([#2754](https://github.com/kubeflow/trainer/pull/2754) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): Kubeflow Trainer ROADMAP 2025 ([#2748](https://github.com/kubeflow/trainer/pull/2748) by [@andreyvelich](https://github.com/andreyvelich))
- chore(docs): Add Changelog for Kubeflow Trainer v2.0.0 ([#2743](https://github.com/kubeflow/trainer/pull/2743) by [@andreyvelich](https://github.com/andreyvelich))
- chore: update github runners to oci gh arc runners ([#2739](https://github.com/kubeflow/trainer/pull/2739) by [@koksay](https://github.com/koksay))
- feat(operator): force trainjob name to be compliant with RFC 1035 for jobset ([#2734](https://github.com/kubeflow/trainer/pull/2734) by [@rudeigerc](https://github.com/rudeigerc))
- chore(ci): Add GitHub action to verify PR titles ([#2724](https://github.com/kubeflow/trainer/pull/2724) by [@andreyvelich](https://github.com/andreyvelich))
- feat(docs): Guide to report security vulnerability ([#2718](https://github.com/kubeflow/trainer/pull/2718) by [@andreyvelich](https://github.com/andreyvelich))
- chore: Upgrade JobSet to version 0.8.2 ([#2726](https://github.com/kubeflow/trainer/pull/2726) by [@astefanutti](https://github.com/astefanutti))
- Add Red Hat to ADOPTERS.md ([#2714](https://github.com/kubeflow/trainer/pull/2714) by [@terrytangyuan](https://github.com/terrytangyuan))
- chore(docs): Add Changelog for v2.0.0-rc.1 ([#2709](https://github.com/kubeflow/trainer/pull/2709) by [@andreyvelich](https://github.com/andreyvelich))
- chore(docs): Update Release Guide ([#2710](https://github.com/kubeflow/trainer/pull/2710) by [@andreyvelich](https://github.com/andreyvelich))
- chore: Copy generated CRDs into Helm charts ([#2703](https://github.com/kubeflow/trainer/pull/2703) by [@astefanutti](https://github.com/astefanutti))
- feat(example): Add alpaca-trianjob-yaml.ipynb. ([#2670](https://github.com/kubeflow/trainer/pull/2670) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- feat: Mutable PodSpecOverrides for suspended TrainJob ([#2683](https://github.com/kubeflow/trainer/pull/2683) by [@astefanutti](https://github.com/astefanutti))
- chore: Replace the deprecated intstr.FromInt with intstr.FromInt32 ([#2695](https://github.com/kubeflow/trainer/pull/2695) by [@tenzen-y](https://github.com/tenzen-y))
- chore: Remove the vendor specific parameters ([#2691](https://github.com/kubeflow/trainer/pull/2691) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Add the manifests overlay for Kubeflow Training V2 ([#2382](https://github.com/kubeflow/trainer/pull/2382) by [@Doris-xm](https://github.com/Doris-xm))
- chore(runtime): Bump Torch to 2.7.1 and DeepSpeed to 0.17.1 ([#2685](https://github.com/kubeflow/trainer/pull/2685) by [@andreyvelich](https://github.com/andreyvelich))
- chore(helm): Sync ClusterRule in Helm chart ([#2686](https://github.com/kubeflow/trainer/pull/2686) by [@astefanutti](https://github.com/astefanutti))
- Add Changelog for Trainer v2.0.0-rc.0 ([#2666](https://github.com/kubeflow/trainer/pull/2666) by [@kramaranya](https://github.com/kramaranya))
- feat(initializer): Updated base image to Debian image and changed install commands compatible with Debian image ([#2528](https://github.com/kubeflow/trainer/pull/2528) by [@Debabrata47](https://github.com/Debabrata47))

[Full Changelog](https://github.com/kubeflow/trainer/compare/v2.0.0...v2.1.0-rc.0)
