# Changelog

# [v2.0.1](https://github.com/kubeflow/trainer/tree/v2.0.1) (2025-09-29)

## New Features

- [release-2.0] feat: Add a public function to create runtime info objects ([#2846](https://github.com/kubeflow/trainer/pull/2846) by [@kaisoz](https://github.com/kaisoz))

## Bug Fixes

- [release-2.0] fix(runtimes): Set numProcPerNode: 1 in DeepSpeed Runtime ([#2863](https://github.com/kubeflow/trainer/pull/2863) by [@andreyvelich](https://github.com/andreyvelich))
- [release-2.0] fix(ci): Add latest image tag only for the master branch ([#2862](https://github.com/kubeflow/trainer/pull/2862) by [@andreyvelich](https://github.com/andreyvelich))
- [release-2.0] fix: update examples to reflect func_args now being unpacked (#2815) ([#2853](https://github.com/kubeflow/trainer/pull/2853) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] fix(examples): Update get_job_logs() API in examples (#2813) ([#2852](https://github.com/kubeflow/trainer/pull/2852) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] feat(runtimes): Add Framework Label to the Runtimes (#2761) ([#2851](https://github.com/kubeflow/trainer/pull/2851) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] fix(examples): Update the argument for Runtime framework (#2766) ([#2850](https://github.com/kubeflow/trainer/pull/2850) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] fix: update kubeflow sdk reference (#2780) ([#2847](https://github.com/kubeflow/trainer/pull/2847) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] fix(api): Fix license path for Kubeflow Trainer Python API ([#2772](https://github.com/kubeflow/trainer/pull/2772) by [@andreyvelich](https://github.com/andreyvelich))

[Full Changelog](https://github.com/kubeflow/trainer/compare/v2.0.0...v2.0.1)

# [v2.0.0](https://github.com/kubeflow/trainer/tree/v2.0.0) (2025-07-17)

This is the major release of the Kubeflow Trainer 2.0 project.

For more information, please see the

- [Blog post announcement](https://blog.kubeflow.org/trainer/intro/)

- [Migration guide from the Training Operator v1](https://www.kubeflow.org/docs/components/trainer/operator-guides/migration/)

## Breaking Changes

- Migrate SDK to the `kubeflow/sdk` repository ([#2657](https://github.com/kubeflow/trainer/pull/2657) by [@eoinfennessy](https://github.com/eoinfennessy))
- KEP-2170: Change API Group Name to `trainer.kubeflow.org` ([#2413](https://github.com/kubeflow/trainer/pull/2413) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Move generated Python models into kubeflow_trainer_api package ([#2632](https://github.com/kubeflow/trainer/pull/2632) by [@kramaranya](https://github.com/kramaranya))
- Upgrade kubernetes Go module version to 1.32 ([#2450](https://github.com/kubeflow/trainer/pull/2450) by [@tenzen-y](https://github.com/tenzen-y))
- Remove kubeflow-trainer prefix from jobset resource names ([#2596](https://github.com/kubeflow/trainer/pull/2596) by [@ChenYi015](https://github.com/ChenYi015))
- Remove the Training Operator V1 Source Code ([#2389](https://github.com/kubeflow/trainer/pull/2389) by [@andreyvelich](https://github.com/andreyvelich))

## New Features

### LLM Trainer V2

- KEP-2401: Support loading local LLMs ([#2644](https://github.com/kubeflow/trainer/pull/2644) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Support mutating dataset preprocessing config in SDK ([#2638](https://github.com/kubeflow/trainer/pull/2638) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Create LLM Training Runtimes for Llama 3.2 model family ([#2590](https://github.com/kubeflow/trainer/pull/2590) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Complement torch plugin to support torchtune config mutation ([#2587](https://github.com/kubeflow/trainer/pull/2587) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Create `torchtune` trainer image ([#2516](https://github.com/kubeflow/trainer/pull/2516) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Refactor current `train()` API ([#2513](https://github.com/kubeflow/trainer/pull/2513) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Kubeflow LLM Trainer V2 ([#2410](https://github.com/kubeflow/trainer/pull/2410) by [@Electronic-Waste](https://github.com/Electronic-Waste))

### Runtime Framework

- feat(runtimes): Support MLX Distributed Runtime with OpenMPI ([#2565](https://github.com/kubeflow/trainer/pull/2565) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtimes): Support DeepSpeed Runtime with OpenMPI ([#2559](https://github.com/kubeflow/trainer/pull/2559) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtime): remove needless Launcher chainer. ([#2558](https://github.com/kubeflow/trainer/pull/2558) by [@IRONICBo](https://github.com/IRONICBo))
- Store the TrainingRuntime numNodes as runtime.Info.PodSet.Count ([#2539](https://github.com/kubeflow/trainer/pull/2539) by [@tenzen-y](https://github.com/tenzen-y))
- Add dependencies to RuntimeRegistrar ([#2476](https://github.com/kubeflow/trainer/pull/2476) by [@tenzen-y](https://github.com/tenzen-y))
- KEP: 2170: Adding cel validations on TrainingRuntime/ClusterTrainingRuntime CRDs ([#2313](https://github.com/kubeflow/trainer/pull/2313) by [@akshaychitneni](https://github.com/akshaychitneni))
- Implement trainer.kubeflow.org/resource-in-use finalizer mechanism to ClusterTrainingRuntime ([#2625](https://github.com/kubeflow/trainer/pull/2625) by [@tenzen-y](https://github.com/tenzen-y))
- Implement trainer.kubeflow.org/resource-in-use finalizer mechanism to TrainingRuntime ([#2608](https://github.com/kubeflow/trainer/pull/2608) by [@tenzen-y](https://github.com/tenzen-y))

### MPI Plugin

- [feature]:add validations for MPIRuntime with RunLauncherAsNode ([#2551](https://github.com/kubeflow/trainer/pull/2551) by [@Harshal292004](https://github.com/Harshal292004))
- Implement CustomValidation UT for MPI plugin ([#2555](https://github.com/kubeflow/trainer/pull/2555) by [@tenzen-y](https://github.com/tenzen-y))
- Implemenet MPI Plugin for OpenMPI ([#2493](https://github.com/kubeflow/trainer/pull/2493) by [@tenzen-y](https://github.com/tenzen-y))
- Implement MPI plugin UTs ([#2481](https://github.com/kubeflow/trainer/pull/2481) by [@tenzen-y](https://github.com/tenzen-y))
- Implement MPIImplementation Enum CRD validation ([#2482](https://github.com/kubeflow/trainer/pull/2482) by [@tenzen-y](https://github.com/tenzen-y))
- Implement MPI numProcPerNode defaulter ([#2483](https://github.com/kubeflow/trainer/pull/2483) by [@tenzen-y](https://github.com/tenzen-y))
- Add MPIMLPolicySource CRD defaulters ([#2474](https://github.com/kubeflow/trainer/pull/2474) by [@tenzen-y](https://github.com/tenzen-y))
- Make MPIMLPolicySource optional fields as a pointer ([#2472](https://github.com/kubeflow/trainer/pull/2472) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Implement MPI Plugin for Kubeflow Trainer ([#2394](https://github.com/kubeflow/trainer/pull/2394) by [@andreyvelich](https://github.com/andreyvelich))

### JobSet

- Retrieve JobSetSpec from runtime.Info in CustomValidations ([#2557](https://github.com/kubeflow/trainer/pull/2557) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Deploy JobSet in `kubeflow-system` namespace ([#2388](https://github.com/kubeflow/trainer/pull/2388) by [@andreyvelich](https://github.com/andreyvelich))
- Bump JobSet to v0.8.0 ([#2463](https://github.com/kubeflow/trainer/pull/2463) by [@andreyvelich](https://github.com/andreyvelich))
- Upgrade jobset SDK version to v0.7.3 ([#2445](https://github.com/kubeflow/trainer/pull/2445) by [@Electronic-Waste](https://github.com/Electronic-Waste))

### New Examples

- Add question-answer example for v2 trainer ([#2580](https://github.com/kubeflow/trainer/pull/2580) by [@solanyn](https://github.com/solanyn))
- KEP-2170: Add PyTorch DDP MNIST training example ([#2387](https://github.com/kubeflow/trainer/pull/2387) by [@astefanutti](https://github.com/astefanutti))

### SDK Updates

- feat(sdk): Get namespace from the provided context ([#2593](https://github.com/kubeflow/trainer/pull/2593) by [@andreyvelich](https://github.com/andreyvelich))
- feat(sdk): Support MPI-based TrainJobs ([#2545](https://github.com/kubeflow/trainer/pull/2545) by [@andreyvelich](https://github.com/andreyvelich))
- feat(sdk): Migrate to OpenAPI V3 ([#2490](https://github.com/kubeflow/trainer/pull/2490) by [@andreyvelich](https://github.com/andreyvelich))
- feat(sdk): Generate external Kubernetes and JobSet models ([#2466](https://github.com/kubeflow/trainer/pull/2466) by [@andreyvelich](https://github.com/andreyvelich))

## Bug Fixes

- [release-2.0] fix(manifests): add rbac config of events for event recorders ([#2733](https://github.com/kubeflow/trainer/pull/2733) by [@rudeigerc](https://github.com/rudeigerc))
- [release-2.0] fix(manifests): fix position of labels of dataset-initializer from pod to job ([#2720](https://github.com/kubeflow/trainer/pull/2720) by [@rudeigerc](https://github.com/rudeigerc))
- [release-2.0] fix(module): Change Go module name to v2 ([#2708](https://github.com/kubeflow/trainer/pull/2708) by [@andreyvelich](https://github.com/andreyvelich))
- [cherry-pick] fix(manifests): Update manifests to enable LLM fine-tuning workflow w… ([#2696](https://github.com/kubeflow/trainer/pull/2696) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- [release-2.0] fix(plugins): Fix some errors in torchtune mutation process. ([#2693](https://github.com/kubeflow/trainer/pull/2693) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- [release-2.0] fix(rbac): Add required RBAC to update ClusterTrainingRuntimes on OpenShift ([#2684](https://github.com/kubeflow/trainer/pull/2684) by [@astefanutti](https://github.com/astefanutti))
- Revert "fix(sdk): Fix type annotation for `train` method's `trainer` parameter" ([#2651](https://github.com/kubeflow/trainer/pull/2651) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(sdk): Fix bad arg passed to `get_args_using_torchtune_config` ([#2647](https://github.com/kubeflow/trainer/pull/2647) by [@eoinfennessy](https://github.com/eoinfennessy))
- fix(sdk): Fix type annotation for `train` method's `trainer` parameter ([#2646](https://github.com/kubeflow/trainer/pull/2646) by [@eoinfennessy](https://github.com/eoinfennessy))
- fix(controller): Fix RBAC permissions for TrainJob controller ([#2626](https://github.com/kubeflow/trainer/pull/2626) by [@andreyvelich](https://github.com/andreyvelich))
- Fix close-pr message in Stale GitHub Action ([#2622](https://github.com/kubeflow/trainer/pull/2622) by [@kramaranya](https://github.com/kramaranya))
- fix: remove redundant K8s version matrix from integration tests ([#2617](https://github.com/kubeflow/trainer/pull/2617) by [@tr33k](https://github.com/tr33k))
- fix(doc): tidy up KEP-2401. ([#2594](https://github.com/kubeflow/trainer/pull/2594) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Fix MPI Test runnable errors ([#2570](https://github.com/kubeflow/trainer/pull/2570) by [@tenzen-y](https://github.com/tenzen-y))
- Fix issue with fetching clustertrainingruntime for validations ([#2564](https://github.com/kubeflow/trainer/pull/2564) by [@akshaychitneni](https://github.com/akshaychitneni))
- fix(sdk): Add missing import types. ([#2566](https://github.com/kubeflow/trainer/pull/2566) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(sdk): Using correct entrypoint for mpirun ([#2552](https://github.com/kubeflow/trainer/pull/2552) by [@andreyvelich](https://github.com/andreyvelich))
- fix(sdk): add missing import type Initializer. ([#2541](https://github.com/kubeflow/trainer/pull/2541) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(ci): update `test-go` coverage ci config and replace trainer badge with new address. ([#2534](https://github.com/kubeflow/trainer/pull/2534) by [@IRONICBo](https://github.com/IRONICBo))
- fix(doc): Update `train()` API in KEP-2401 ([#2536](https://github.com/kubeflow/trainer/pull/2536) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(test): Update images for DockerHub publish ([#2535](https://github.com/kubeflow/trainer/pull/2535) by [@andreyvelich](https://github.com/andreyvelich))
- [hotfix] fix checkout on workflow ([#2531](https://github.com/kubeflow/trainer/pull/2531) by [@mahdikhashan](https://github.com/mahdikhashan))
- [hotfix] fix docker cred ([#2530](https://github.com/kubeflow/trainer/pull/2530) by [@mahdikhashan](https://github.com/mahdikhashan))
- fix: remove unused parameter name in default case of shouldUseCPU function ([#2521](https://github.com/kubeflow/trainer/pull/2521) by [@Diasker](https://github.com/Diasker))
- Fix #2407: Cap nproc_per_node based on CPU resources for PyTorch TrainJob ([#2492](https://github.com/kubeflow/trainer/pull/2492) by [@Diasker](https://github.com/Diasker))
- fix type in model initializer entrypoint ([#2489](https://github.com/kubeflow/trainer/pull/2489) by [@szaher](https://github.com/szaher))
- fix(runtime): fix error label name. ([#2487](https://github.com/kubeflow/trainer/pull/2487) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(sdk): resolve errors in deserialization ([#2457](https://github.com/kubeflow/trainer/pull/2457) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Fix missing external types in apply configurations ([#2429](https://github.com/kubeflow/trainer/pull/2429) by [@astefanutti](https://github.com/astefanutti))
- Fix API Group for Torch Runtime ([#2424](https://github.com/kubeflow/trainer/pull/2424) by [@andreyvelich](https://github.com/andreyvelich))
- Fix Kustomize patchesStrategicMerge deprecation warning ([#2405](https://github.com/kubeflow/trainer/pull/2405) by [@astefanutti](https://github.com/astefanutti))
- ControlPlane: Fix flaky integraion testings due to missing the latest version of object ([#2414](https://github.com/kubeflow/trainer/pull/2414) by [@tenzen-y](https://github.com/tenzen-y))

## Misc

- [release-2.0] chore: update github runners to oci gh arc runners ([#2741](https://github.com/kubeflow/trainer/pull/2741) by [@koksay](https://github.com/koksay))
- [release-2.0] feat(operator): force trainjob name to be compliant with RFC 1035 for jobset ([#2736](https://github.com/kubeflow/trainer/pull/2736) by [@rudeigerc](https://github.com/rudeigerc))
- [release-2.0] chore: Upgrade JobSet to version 0.8.2 ([#2727](https://github.com/kubeflow/trainer/pull/2727) by [@google-oss-robot](https://github.com/google-oss-robot))
- [release-2.0] chore: Copy generated CRDs into Helm charts ([#2704](https://github.com/kubeflow/trainer/pull/2704) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] feat: Add schedulingGates to PodSpecOverrides ([#2705](https://github.com/kubeflow/trainer/pull/2705) by [@astefanutti](https://github.com/astefanutti))
- [cherry-pick] feat(example): Add alpaca-trianjob-yaml.ipynb. (#2670) ([#2702](https://github.com/kubeflow/trainer/pull/2702) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- [release-2.0] feat: Mutable PodSpecOverrides for suspended TrainJob ([#2698](https://github.com/kubeflow/trainer/pull/2698) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] chore: Replace the deprecated intstr.FromInt with intstr.FromInt32 ([#2697](https://github.com/kubeflow/trainer/pull/2697) by [@tenzen-y](https://github.com/tenzen-y))
- [release-2.0] chore: Remove the vendor specific parameters ([#2694](https://github.com/kubeflow/trainer/pull/2694) by [@tenzen-y](https://github.com/tenzen-y))
- [Release 2.0] KEP-2170: Add the manifests overlay for Kubeflow Training V2 ([#2692](https://github.com/kubeflow/trainer/pull/2692) by [@Doris-xm](https://github.com/Doris-xm))
- [release-2.0] chore(runtime): Bump Torch to 2.7.1 and DeepSpeed to 0.17.1 ([#2687](https://github.com/kubeflow/trainer/pull/2687) by [@andreyvelich](https://github.com/andreyvelich))
- [release-2.0] chore(helm): Sync ClusterRule in Helm chart ([#2688](https://github.com/kubeflow/trainer/pull/2688) by [@astefanutti](https://github.com/astefanutti))
- Tag Docker images with GitHub release tags ([#2662](https://github.com/kubeflow/trainer/pull/2662) by [@kramaranya](https://github.com/kramaranya))
- feat(controller): Implement PodSpecOverride API ([#2614](https://github.com/kubeflow/trainer/pull/2614) by [@andreyvelich](https://github.com/andreyvelich))
- Nominate @Electronic-Waste as approver and @astefanutti as reviewer ([#2659](https://github.com/kubeflow/trainer/pull/2659) by [@andreyvelich](https://github.com/andreyvelich))
- chore(build): Support Podman to run OpenAPI generator ([#2656](https://github.com/kubeflow/trainer/pull/2656) by [@astefanutti](https://github.com/astefanutti))
- chore(docs): Add OpenSSF Best Practices Badge ([#2611](https://github.com/kubeflow/trainer/pull/2611) by [@andreyvelich](https://github.com/andreyvelich))
- [chore] update stale action version to latest ([#2642](https://github.com/kubeflow/trainer/pull/2642) by [@mahdikhashan](https://github.com/mahdikhashan))
- Remove TrainJobCreated condition ([#2621](https://github.com/kubeflow/trainer/pull/2621) by [@astefanutti](https://github.com/astefanutti))
- ci: refactor build-push-images workflow ([#2607](https://github.com/kubeflow/trainer/pull/2607) by [@milinddethe15](https://github.com/milinddethe15))
- Update Go to v1.24 (#2615) ([#2620](https://github.com/kubeflow/trainer/pull/2620) by [@vzamboulingame](https://github.com/vzamboulingame))
- test(runtime): add UT for IndexTrainJobTrainingRuntime ([#2603](https://github.com/kubeflow/trainer/pull/2603) by [@Harshal292004](https://github.com/Harshal292004))
- ci: add k8s `v1.32` for tests env ([#2613](https://github.com/kubeflow/trainer/pull/2613) by [@milinddethe15](https://github.com/milinddethe15))
- chore(deps): bump torch from 2.5.0 to 2.6.0 in /cmd/runtimes/deepspeed ([#2606](https://github.com/kubeflow/trainer/pull/2606) by [@dependabot[bot]](https://github.com/apps/dependabot))
- chore(deps): bump golang.org/x/net from 0.36.0 to 0.38.0 ([#2602](https://github.com/kubeflow/trainer/pull/2602) by [@dependabot[bot]](https://github.com/apps/dependabot))
- test(runtime): add UT for jobset runtime valid function. ([#2562](https://github.com/kubeflow/trainer/pull/2562) by [@Harshal292004](https://github.com/Harshal292004))
- Add Helm chart for kubeflow trainer ([#2435](https://github.com/kubeflow/trainer/pull/2435) by [@ChenYi015](https://github.com/ChenYi015))
- chore(test): Removed the no longer needed github-trigger-rerun-test.yaml ([#2589](https://github.com/kubeflow/trainer/pull/2589) by [@hbelmiro](https://github.com/hbelmiro))
- Add PodNetwork plugin to KEP-2170 Job Pipeline Framework description ([#2578](https://github.com/kubeflow/trainer/pull/2578) by [@tenzen-y](https://github.com/tenzen-y))
- chore(docs): Update Slack channel ([#2569](https://github.com/kubeflow/trainer/pull/2569) by [@andreyvelich](https://github.com/andreyvelich))
- docs: update CONTRIBUTING.md for Kubeflow Trainer V2 ([#2561](https://github.com/kubeflow/trainer/pull/2561) by [@muzzlol](https://github.com/muzzlol))
- test(runtime): add UT for torch runtime valid function. ([#2560](https://github.com/kubeflow/trainer/pull/2560) by [@IRONICBo](https://github.com/IRONICBo))
- feat(doc): add Runtime API design in KEP-2401. ([#2501](https://github.com/kubeflow/trainer/pull/2501) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Update CONTRIBUTING.md ([#2512](https://github.com/kubeflow/trainer/pull/2512) by [@MuhammedgitAli](https://github.com/MuhammedgitAli))
- feat: add replicatedJobs.replicas validations in validateReplicatedJobs function. ([#2533](https://github.com/kubeflow/trainer/pull/2533) by [@IRONICBo](https://github.com/IRONICBo))
- Construct Trainer based on trainer.kubeflow.org/trainjob-ancestor-step label ([#2548](https://github.com/kubeflow/trainer/pull/2548) by [@tenzen-y](https://github.com/tenzen-y))
- chore: Enable GCI for golangci-lint ([#2540](https://github.com/kubeflow/trainer/pull/2540) by [@tenzen-y](https://github.com/tenzen-y))
- [feature] merge GHCR and DockerHub CI jobs ([#2537](https://github.com/kubeflow/trainer/pull/2537) by [@ashwinr64](https://github.com/ashwinr64))
- feat(controller): Refactor the Initializer APIs of TrainJob ([#2523](https://github.com/kubeflow/trainer/pull/2523) by [@andreyvelich](https://github.com/andreyvelich))
- Migrate InfoOptions.podSpecReplias and info.Scheduler.TotalRequests to info.TemplateSpec.PodSet ([#2524](https://github.com/kubeflow/trainer/pull/2524) by [@tenzen-y](https://github.com/tenzen-y))
- [feature] pull images in manifest from ghcr ([#2529](https://github.com/kubeflow/trainer/pull/2529) by [@mahdikhashan](https://github.com/mahdikhashan))
- [feature] migrate images to ghcr ([#2455](https://github.com/kubeflow/trainer/pull/2455) by [@mahdikhashan](https://github.com/mahdikhashan))
- KEP-2170: Adding validation webhook for v2 trainjob ([#2307](https://github.com/kubeflow/trainer/pull/2307) by [@akshaychitneni](https://github.com/akshaychitneni))
- Migrate Info.Trainer to Info.TemplateSpec.PodSet ([#2520](https://github.com/kubeflow/trainer/pull/2520) by [@tenzen-y](https://github.com/tenzen-y))
- Implement E2E for OpenMPI workload ([#2500](https://github.com/kubeflow/trainer/pull/2500) by [@tenzen-y](https://github.com/tenzen-y))
- Bump golang.org/x/net from 0.33.0 to 0.36.0 ([#2514](https://github.com/kubeflow/trainer/pull/2514) by [@dependabot[bot]](https://github.com/apps/dependabot))
- Move TrainJob marker defaulting and validation integration tests to test/integration/webhooks pkg ([#2486](https://github.com/kubeflow/trainer/pull/2486) by [@tenzen-y](https://github.com/tenzen-y))
- feat(controller): Integrate DependsOn API ([#2484](https://github.com/kubeflow/trainer/pull/2484) by [@andreyvelich](https://github.com/andreyvelich))
- Store E2E manifests to artifacts directory ([#2478](https://github.com/kubeflow/trainer/pull/2478) by [@tenzen-y](https://github.com/tenzen-y))
- Use large runner for building container image ([#2475](https://github.com/kubeflow/trainer/pull/2475) by [@tenzen-y](https://github.com/tenzen-y))
- chore(test): Upload artifacts from dir ([#2473](https://github.com/kubeflow/trainer/pull/2473) by [@andreyvelich](https://github.com/andreyvelich))
- Implement UTs for PlainML plugin ([#2469](https://github.com/kubeflow/trainer/pull/2469) by [@tenzen-y](https://github.com/tenzen-y))
- chore(test): Add E2E tests for Kubeflow Trainer ([#2470](https://github.com/kubeflow/trainer/pull/2470) by [@andreyvelich](https://github.com/andreyvelich))
- KEP-2170: Add Kubeflow Trainer Pipeline Framework Design ([#2439](https://github.com/kubeflow/trainer/pull/2439) by [@tenzen-y](https://github.com/tenzen-y))
- Replace Kueue PodRequests helper with core k/k one ([#2461](https://github.com/kubeflow/trainer/pull/2461) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Use SSA to reconcile TrainJob components ([#2431](https://github.com/kubeflow/trainer/pull/2431) by [@astefanutti](https://github.com/astefanutti))
- Bump golang.org/x/net from 0.30.0 to 0.33.0 ([#2451](https://github.com/kubeflow/trainer/pull/2451) by [@dependabot[bot]](https://github.com/apps/dependabot))
- Use the correct apiVersion name ([#2444](https://github.com/kubeflow/trainer/pull/2444) by [@runzhen](https://github.com/runzhen))
- Add 'KEP Usage' KEP and template link ([#2423](https://github.com/kubeflow/trainer/pull/2423) by [@anishasthana](https://github.com/anishasthana))
- KEP-2170: Add validation to Torch `numProcPerNode` field ([#2409](https://github.com/kubeflow/trainer/pull/2409) by [@astefanutti](https://github.com/astefanutti))
- update migration url on readme file ([#2436](https://github.com/kubeflow/trainer/pull/2436) by [@varodrig](https://github.com/varodrig))
- IntegraionTests: Waiting for expected conditions before emulate JobSet controller manager ([#2425](https://github.com/kubeflow/trainer/pull/2425) by [@tenzen-y](https://github.com/tenzen-y))
- Nominate @Electronic-Waste as a reviewer ([#2427](https://github.com/kubeflow/trainer/pull/2427) by [@andreyvelich](https://github.com/andreyvelich))
- Update the naming conventions for Kubeflow Trainer ([#2415](https://github.com/kubeflow/trainer/pull/2415) by [@andreyvelich](https://github.com/andreyvelich))
- Rename paddlepaddle_defaults.go file name ([#2399](https://github.com/kubeflow/trainer/pull/2399) by [@ChristianZaccaria](https://github.com/ChristianZaccaria))
- Bump golang.org/x/net from 0.30.0 to 0.33.0 ([#2391](https://github.com/kubeflow/trainer/pull/2391) by [@dependabot[bot]](https://github.com/apps/dependabot))
- KEP-2170: Add unit and Integration tests for model and dataset initializers ([#2323](https://github.com/kubeflow/trainer/pull/2323) by [@seanlaii](https://github.com/seanlaii))
- Testing CI in JAX example ([#2385](https://github.com/kubeflow/trainer/pull/2385) by [@saileshd1402](https://github.com/saileshd1402))
- Upgrade huggingface_hub to v0.27.x in dataset initializer v2 ([#2379](https://github.com/kubeflow/trainer/pull/2379) by [@astefanutti](https://github.com/astefanutti))
- Add Changelog for Training Operator v1.9.0-rc.0 ([#2380](https://github.com/kubeflow/trainer/pull/2380) by [@andreyvelich](https://github.com/andreyvelich))
- Add release branch to the image push trigger ([#2376](https://github.com/kubeflow/trainer/pull/2376) by [@andreyvelich](https://github.com/andreyvelich))

[Full Changelog](https://github.com/kubeflow/trainer/compare/v1.8.1...v2.0.0)

# [v2.0.0-rc.1](https://github.com/kubeflow/trainer/tree/v2.0.0-rc.1) (2025-07-03)

## New Features

- [release-2.0] feat: Add schedulingGates to PodSpecOverrides ([#2705](https://github.com/kubeflow/trainer/pull/2705) by [@astefanutti](https://github.com/astefanutti))
- [release-2.0] feat: Mutable PodSpecOverrides for suspended TrainJob ([#2698](https://github.com/kubeflow/trainer/pull/2698) by [@astefanutti](https://github.com/astefanutti))
- [Release 2.0] KEP-2170: Add the manifests overlay for Kubeflow Training V2 ([#2692](https://github.com/kubeflow/trainer/pull/2692) by [@Doris-xm](https://github.com/Doris-xm))

## Bug Fixes

- [release-2.0] fix(module): Change Go module name to v2 ([#2708](https://github.com/kubeflow/trainer/pull/2708) by [@andreyvelich](https://github.com/andreyvelich))
- [cherry-pick] fix(manifests): Update manifests to enable LLM fine-tuning workflow w… ([#2696](https://github.com/kubeflow/trainer/pull/2696) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- [release-2.0] fix(plugins): Fix some errors in torchtune mutation process. ([#2693](https://github.com/kubeflow/trainer/pull/2693) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- [release-2.0] fix(rbac): Add required RBAC to update ClusterTrainingRuntimes on OpenShift ([#2684](https://github.com/kubeflow/trainer/pull/2684) by [@astefanutti](https://github.com/astefanutti))

## Misc

- [release-2.0] chore: Copy generated CRDs into Helm charts ([#2704](https://github.com/kubeflow/trainer/pull/2704) by [@astefanutti](https://github.com/astefanutti))
- [cherry-pick] feat(example): Add alpaca-trianjob-yaml.ipynb. (#2670) ([#2702](https://github.com/kubeflow/trainer/pull/2702) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- [release-2.0] chore: Replace the deprecated intstr.FromInt with intstr.FromInt32 ([#2697](https://github.com/kubeflow/trainer/pull/2697) by [@tenzen-y](https://github.com/tenzen-y))
- [release-2.0] chore: Remove the vendor specific parameters ([#2694](https://github.com/kubeflow/trainer/pull/2694) by [@tenzen-y](https://github.com/tenzen-y))
- [release-2.0] chore(runtime): Bump Torch to 2.7.1 and DeepSpeed to 0.17.1 ([#2687](https://github.com/kubeflow/trainer/pull/2687) by [@andreyvelich](https://github.com/andreyvelich))
- [release-2.0] chore(helm): Sync ClusterRule in Helm chart ([#2688](https://github.com/kubeflow/trainer/pull/2688) by [@astefanutti](https://github.com/astefanutti))

[Full Changelog](https://github.com/kubeflow/trainer/compare/v2.0.0-rc.0...v2.0.0-rc.1)

# [v2.0.0-rc.0](https://github.com/kubeflow/trainer/tree/v2.0.0-rc.0) (2025-06-10)

## Breaking Changes

- KEP-2170: Change API Group Name to `trainer.kubeflow.org` ([#2413](https://github.com/kubeflow/trainer/pull/2413) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Move generated Python models into kubeflow_trainer_api package ([#2632](https://github.com/kubeflow/trainer/pull/2632) by [@kramaranya](https://github.com/kramaranya))
- Upgrade kubernetes Go module version to 1.32 ([#2450](https://github.com/kubeflow/trainer/pull/2450) by [@tenzen-y](https://github.com/tenzen-y))
- Remove kubeflow-trainer prefix from jobset resource names ([#2596](https://github.com/kubeflow/trainer/pull/2596) by [@ChenYi015](https://github.com/ChenYi015))
- Remove the Training Operator V1 Source Code ([#2389](https://github.com/kubeflow/trainer/pull/2389) by [@andreyvelich](https://github.com/andreyvelich))

## New Features

### LLM Trainer V2

- KEP-2401: Support loading local LLMs ([#2644](https://github.com/kubeflow/trainer/pull/2644) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Support mutating dataset preprocessing config in SDK ([#2638](https://github.com/kubeflow/trainer/pull/2638) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Create LLM Training Runtimes for Llama 3.2 model family ([#2590](https://github.com/kubeflow/trainer/pull/2590) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Complement torch plugin to support torchtune config mutation ([#2587](https://github.com/kubeflow/trainer/pull/2587) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Create `torchtune` trainer image ([#2516](https://github.com/kubeflow/trainer/pull/2516) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Refactor current `train()` API ([#2513](https://github.com/kubeflow/trainer/pull/2513) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- KEP-2401: Kubeflow LLM Trainer V2 ([#2410](https://github.com/kubeflow/trainer/pull/2410) by [@Electronic-Waste](https://github.com/Electronic-Waste))

### Runtime Framework

- feat(runtimes): Support MLX Distributed Runtime with OpenMPI ([#2565](https://github.com/kubeflow/trainer/pull/2565) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtimes): Support DeepSpeed Runtime with OpenMPI ([#2559](https://github.com/kubeflow/trainer/pull/2559) by [@andreyvelich](https://github.com/andreyvelich))
- feat(runtime): remove needless Launcher chainer. ([#2558](https://github.com/kubeflow/trainer/pull/2558) by [@IRONICBo](https://github.com/IRONICBo))
- Store the TrainingRuntime numNodes as runtime.Info.PodSet.Count ([#2539](https://github.com/kubeflow/trainer/pull/2539) by [@tenzen-y](https://github.com/tenzen-y))
- Add dependencies to RuntimeRegistrar ([#2476](https://github.com/kubeflow/trainer/pull/2476) by [@tenzen-y](https://github.com/tenzen-y))
- KEP: 2170: Adding cel validations on TrainingRuntime/ClusterTrainingRuntime CRDs ([#2313](https://github.com/kubeflow/trainer/pull/2313) by [@akshaychitneni](https://github.com/akshaychitneni))
- Implement trainer.kubeflow.org/resource-in-use finalizer mechanism to ClusterTrainingRuntime ([#2625](https://github.com/kubeflow/trainer/pull/2625) by [@tenzen-y](https://github.com/tenzen-y))
- Implement trainer.kubeflow.org/resource-in-use finalizer mechanism to TrainingRuntime ([#2608](https://github.com/kubeflow/trainer/pull/2608) by [@tenzen-y](https://github.com/tenzen-y))

### MPI Plugin

- [feature]:add validations for MPIRuntime with RunLauncherAsNode ([#2551](https://github.com/kubeflow/trainer/pull/2551) by [@Harshal292004](https://github.com/Harshal292004))
- Implement CustomValidation UT for MPI plugin ([#2555](https://github.com/kubeflow/trainer/pull/2555) by [@tenzen-y](https://github.com/tenzen-y))
- Implemenet MPI Plugin for OpenMPI ([#2493](https://github.com/kubeflow/trainer/pull/2493) by [@tenzen-y](https://github.com/tenzen-y))
- Implement MPI plugin UTs ([#2481](https://github.com/kubeflow/trainer/pull/2481) by [@tenzen-y](https://github.com/tenzen-y))
- Implement MPIImplementation Enum CRD validation ([#2482](https://github.com/kubeflow/trainer/pull/2482) by [@tenzen-y](https://github.com/tenzen-y))
- Implement MPI numProcPerNode defaulter ([#2483](https://github.com/kubeflow/trainer/pull/2483) by [@tenzen-y](https://github.com/tenzen-y))
- Add MPIMLPolicySource CRD defaulters ([#2474](https://github.com/kubeflow/trainer/pull/2474) by [@tenzen-y](https://github.com/tenzen-y))
- Make MPIMLPolicySource optional fields as a pointer ([#2472](https://github.com/kubeflow/trainer/pull/2472) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Implement MPI Plugin for Kubeflow Trainer ([#2394](https://github.com/kubeflow/trainer/pull/2394) by [@andreyvelich](https://github.com/andreyvelich))

### JobSet

- Retrieve JobSetSpec from runtime.Info in CustomValidations ([#2557](https://github.com/kubeflow/trainer/pull/2557) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Deploy JobSet in `kubeflow-system` namespace ([#2388](https://github.com/kubeflow/trainer/pull/2388) by [@andreyvelich](https://github.com/andreyvelich))
- Bump JobSet to v0.8.0 ([#2463](https://github.com/kubeflow/trainer/pull/2463) by [@andreyvelich](https://github.com/andreyvelich))
- Upgrade jobset SDK version to v0.7.3 ([#2445](https://github.com/kubeflow/trainer/pull/2445) by [@Electronic-Waste](https://github.com/Electronic-Waste))

### New Examples

- Add question-answer example for v2 trainer ([#2580](https://github.com/kubeflow/trainer/pull/2580) by [@solanyn](https://github.com/solanyn))
- KEP-2170: Add PyTorch DDP MNIST training example ([#2387](https://github.com/kubeflow/trainer/pull/2387) by [@astefanutti](https://github.com/astefanutti))

### SDK Updates

- Remove SDK ([#2657](https://github.com/kubeflow/trainer/pull/2657) by [@eoinfennessy](https://github.com/eoinfennessy))
- feat(sdk): Get namespace from the provided context ([#2593](https://github.com/kubeflow/trainer/pull/2593) by [@andreyvelich](https://github.com/andreyvelich))
- feat(sdk): Support MPI-based TrainJobs ([#2545](https://github.com/kubeflow/trainer/pull/2545) by [@andreyvelich](https://github.com/andreyvelich))
- feat(sdk): Migrate to OpenAPI V3 ([#2490](https://github.com/kubeflow/trainer/pull/2490) by [@andreyvelich](https://github.com/andreyvelich))
- feat(sdk): Generate external Kubernetes and JobSet models ([#2466](https://github.com/kubeflow/trainer/pull/2466) by [@andreyvelich](https://github.com/andreyvelich))

## Bug Fixes

- Revert "fix(sdk): Fix type annotation for `train` method's `trainer` parameter" ([#2651](https://github.com/kubeflow/trainer/pull/2651) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(sdk): Fix bad arg passed to `get_args_using_torchtune_config` ([#2647](https://github.com/kubeflow/trainer/pull/2647) by [@eoinfennessy](https://github.com/eoinfennessy))
- fix(sdk): Fix type annotation for `train` method's `trainer` parameter ([#2646](https://github.com/kubeflow/trainer/pull/2646) by [@eoinfennessy](https://github.com/eoinfennessy))
- fix(controller): Fix RBAC permissions for TrainJob controller ([#2626](https://github.com/kubeflow/trainer/pull/2626) by [@andreyvelich](https://github.com/andreyvelich))
- Fix close-pr message in Stale GitHub Action ([#2622](https://github.com/kubeflow/trainer/pull/2622) by [@kramaranya](https://github.com/kramaranya))
- fix: remove redundant K8s version matrix from integration tests ([#2617](https://github.com/kubeflow/trainer/pull/2617) by [@tr33k](https://github.com/tr33k))
- fix(doc): tidy up KEP-2401. ([#2594](https://github.com/kubeflow/trainer/pull/2594) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Fix MPI Test runnable errors ([#2570](https://github.com/kubeflow/trainer/pull/2570) by [@tenzen-y](https://github.com/tenzen-y))
- Fix issue with fetching clustertrainingruntime for validations ([#2564](https://github.com/kubeflow/trainer/pull/2564) by [@akshaychitneni](https://github.com/akshaychitneni))
- fix(sdk): Add missing import types. ([#2566](https://github.com/kubeflow/trainer/pull/2566) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(sdk): Using correct entrypoint for mpirun ([#2552](https://github.com/kubeflow/trainer/pull/2552) by [@andreyvelich](https://github.com/andreyvelich))
- fix(sdk): add missing import type Initializer. ([#2541](https://github.com/kubeflow/trainer/pull/2541) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(ci): update `test-go` coverage ci config and replace trainer badge with new address. ([#2534](https://github.com/kubeflow/trainer/pull/2534) by [@IRONICBo](https://github.com/IRONICBo))
- fix(doc): Update `train()` API in KEP-2401 ([#2536](https://github.com/kubeflow/trainer/pull/2536) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(test): Update images for DockerHub publish ([#2535](https://github.com/kubeflow/trainer/pull/2535) by [@andreyvelich](https://github.com/andreyvelich))
- [hotfix] fix checkout on workflow ([#2531](https://github.com/kubeflow/trainer/pull/2531) by [@mahdikhashan](https://github.com/mahdikhashan))
- [hotfix] fix docker cred ([#2530](https://github.com/kubeflow/trainer/pull/2530) by [@mahdikhashan](https://github.com/mahdikhashan))
- fix: remove unused parameter name in default case of shouldUseCPU function ([#2521](https://github.com/kubeflow/trainer/pull/2521) by [@Diasker](https://github.com/Diasker))
- Fix #2407: Cap nproc_per_node based on CPU resources for PyTorch TrainJob ([#2492](https://github.com/kubeflow/trainer/pull/2492) by [@Diasker](https://github.com/Diasker))
- fix type in model initializer entrypoint ([#2489](https://github.com/kubeflow/trainer/pull/2489) by [@szaher](https://github.com/szaher))
- fix(runtime): fix error label name. ([#2487](https://github.com/kubeflow/trainer/pull/2487) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- fix(sdk): resolve errors in deserialization ([#2457](https://github.com/kubeflow/trainer/pull/2457) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Fix missing external types in apply configurations ([#2429](https://github.com/kubeflow/trainer/pull/2429) by [@astefanutti](https://github.com/astefanutti))
- Fix API Group for Torch Runtime ([#2424](https://github.com/kubeflow/trainer/pull/2424) by [@andreyvelich](https://github.com/andreyvelich))
- Fix Kustomize patchesStrategicMerge deprecation warning ([#2405](https://github.com/kubeflow/trainer/pull/2405) by [@astefanutti](https://github.com/astefanutti))
- ControlPlane: Fix flaky integraion testings due to missing the latest version of object ([#2414](https://github.com/kubeflow/trainer/pull/2414) by [@tenzen-y](https://github.com/tenzen-y))

## Misc

- Tag Docker images with GitHub release tags ([#2662](https://github.com/kubeflow/trainer/pull/2662) by [@kramaranya](https://github.com/kramaranya))
- feat(controller): Implement PodSpecOverride API ([#2614](https://github.com/kubeflow/trainer/pull/2614) by [@andreyvelich](https://github.com/andreyvelich))
- Nominate @Electronic-Waste as approver and @astefanutti as reviewer ([#2659](https://github.com/kubeflow/trainer/pull/2659) by [@andreyvelich](https://github.com/andreyvelich))
- chore(build): Support Podman to run OpenAPI generator ([#2656](https://github.com/kubeflow/trainer/pull/2656) by [@astefanutti](https://github.com/astefanutti))
- chore(docs): Add OpenSSF Best Practices Badge ([#2611](https://github.com/kubeflow/trainer/pull/2611) by [@andreyvelich](https://github.com/andreyvelich))
- [chore] update stale action version to latest ([#2642](https://github.com/kubeflow/trainer/pull/2642) by [@mahdikhashan](https://github.com/mahdikhashan))
- Remove TrainJobCreated condition ([#2621](https://github.com/kubeflow/trainer/pull/2621) by [@astefanutti](https://github.com/astefanutti))
- ci: refactor build-push-images workflow ([#2607](https://github.com/kubeflow/trainer/pull/2607) by [@milinddethe15](https://github.com/milinddethe15))
- Update Go to v1.24 (#2615) ([#2620](https://github.com/kubeflow/trainer/pull/2620) by [@vzamboulingame](https://github.com/vzamboulingame))
- test(runtime): add UT for IndexTrainJobTrainingRuntime ([#2603](https://github.com/kubeflow/trainer/pull/2603) by [@Harshal292004](https://github.com/Harshal292004))
- ci: add k8s `v1.32` for tests env ([#2613](https://github.com/kubeflow/trainer/pull/2613) by [@milinddethe15](https://github.com/milinddethe15))
- chore(deps): bump torch from 2.5.0 to 2.6.0 in /cmd/runtimes/deepspeed ([#2606](https://github.com/kubeflow/trainer/pull/2606) by [@dependabot[bot]](https://github.com/apps/dependabot))
- chore(deps): bump golang.org/x/net from 0.36.0 to 0.38.0 ([#2602](https://github.com/kubeflow/trainer/pull/2602) by [@dependabot[bot]](https://github.com/apps/dependabot))
- test(runtime): add UT for jobset runtime valid function. ([#2562](https://github.com/kubeflow/trainer/pull/2562) by [@Harshal292004](https://github.com/Harshal292004))
- Add Helm chart for kubeflow trainer ([#2435](https://github.com/kubeflow/trainer/pull/2435) by [@ChenYi015](https://github.com/ChenYi015))
- chore(test): Removed the no longer needed github-trigger-rerun-test.yaml ([#2589](https://github.com/kubeflow/trainer/pull/2589) by [@hbelmiro](https://github.com/hbelmiro))
- Add PodNetwork plugin to KEP-2170 Job Pipeline Framework description ([#2578](https://github.com/kubeflow/trainer/pull/2578) by [@tenzen-y](https://github.com/tenzen-y))
- chore(docs): Update Slack channel ([#2569](https://github.com/kubeflow/trainer/pull/2569) by [@andreyvelich](https://github.com/andreyvelich))
- docs: update CONTRIBUTING.md for Kubeflow Trainer V2 ([#2561](https://github.com/kubeflow/trainer/pull/2561) by [@muzzlol](https://github.com/muzzlol))
- test(runtime): add UT for torch runtime valid function. ([#2560](https://github.com/kubeflow/trainer/pull/2560) by [@IRONICBo](https://github.com/IRONICBo))
- feat(doc): add Runtime API design in KEP-2401. ([#2501](https://github.com/kubeflow/trainer/pull/2501) by [@Electronic-Waste](https://github.com/Electronic-Waste))
- Update CONTRIBUTING.md ([#2512](https://github.com/kubeflow/trainer/pull/2512) by [@MuhammedgitAli](https://github.com/MuhammedgitAli))
- feat: add replicatedJobs.replicas validations in validateReplicatedJobs function. ([#2533](https://github.com/kubeflow/trainer/pull/2533) by [@IRONICBo](https://github.com/IRONICBo))
- Construct Trainer based on trainer.kubeflow.org/trainjob-ancestor-step label ([#2548](https://github.com/kubeflow/trainer/pull/2548) by [@tenzen-y](https://github.com/tenzen-y))
- chore: Enable GCI for golangci-lint ([#2540](https://github.com/kubeflow/trainer/pull/2540) by [@tenzen-y](https://github.com/tenzen-y))
- [feature] merge GHCR and DockerHub CI jobs ([#2537](https://github.com/kubeflow/trainer/pull/2537) by [@ashwinr64](https://github.com/ashwinr64))
- feat(controller): Refactor the Initializer APIs of TrainJob ([#2523](https://github.com/kubeflow/trainer/pull/2523) by [@andreyvelich](https://github.com/andreyvelich))
- Migrate InfoOptions.podSpecReplias and info.Scheduler.TotalRequests to info.TemplateSpec.PodSet ([#2524](https://github.com/kubeflow/trainer/pull/2524) by [@tenzen-y](https://github.com/tenzen-y))
- [feature] pull images in manifest from ghcr ([#2529](https://github.com/kubeflow/trainer/pull/2529) by [@mahdikhashan](https://github.com/mahdikhashan))
- [feature] migrate images to ghcr ([#2455](https://github.com/kubeflow/trainer/pull/2455) by [@mahdikhashan](https://github.com/mahdikhashan))
- KEP-2170: Adding validation webhook for v2 trainjob ([#2307](https://github.com/kubeflow/trainer/pull/2307) by [@akshaychitneni](https://github.com/akshaychitneni))
- Migrate Info.Trainer to Info.TemplateSpec.PodSet ([#2520](https://github.com/kubeflow/trainer/pull/2520) by [@tenzen-y](https://github.com/tenzen-y))
- Implement E2E for OpenMPI workload ([#2500](https://github.com/kubeflow/trainer/pull/2500) by [@tenzen-y](https://github.com/tenzen-y))
- Bump golang.org/x/net from 0.33.0 to 0.36.0 ([#2514](https://github.com/kubeflow/trainer/pull/2514) by [@dependabot[bot]](https://github.com/apps/dependabot))
- Move TrainJob marker defaulting and validation integration tests to test/integration/webhooks pkg ([#2486](https://github.com/kubeflow/trainer/pull/2486) by [@tenzen-y](https://github.com/tenzen-y))
- feat(controller): Integrate DependsOn API ([#2484](https://github.com/kubeflow/trainer/pull/2484) by [@andreyvelich](https://github.com/andreyvelich))
- Store E2E manifests to artifacts directory ([#2478](https://github.com/kubeflow/trainer/pull/2478) by [@tenzen-y](https://github.com/tenzen-y))
- Use large runner for building container image ([#2475](https://github.com/kubeflow/trainer/pull/2475) by [@tenzen-y](https://github.com/tenzen-y))
- chore(test): Upload artifacts from dir ([#2473](https://github.com/kubeflow/trainer/pull/2473) by [@andreyvelich](https://github.com/andreyvelich))
- Implement UTs for PlainML plugin ([#2469](https://github.com/kubeflow/trainer/pull/2469) by [@tenzen-y](https://github.com/tenzen-y))
- chore(test): Add E2E tests for Kubeflow Trainer ([#2470](https://github.com/kubeflow/trainer/pull/2470) by [@andreyvelich](https://github.com/andreyvelich))
- KEP-2170: Add Kubeflow Trainer Pipeline Framework Design ([#2439](https://github.com/kubeflow/trainer/pull/2439) by [@tenzen-y](https://github.com/tenzen-y))
- Replace Kueue PodRequests helper with core k/k one ([#2461](https://github.com/kubeflow/trainer/pull/2461) by [@tenzen-y](https://github.com/tenzen-y))
- KEP-2170: Use SSA to reconcile TrainJob components ([#2431](https://github.com/kubeflow/trainer/pull/2431) by [@astefanutti](https://github.com/astefanutti))
- Bump golang.org/x/net from 0.30.0 to 0.33.0 ([#2451](https://github.com/kubeflow/trainer/pull/2451) by [@dependabot[bot]](https://github.com/apps/dependabot))
- Use the correct apiVersion name ([#2444](https://github.com/kubeflow/trainer/pull/2444) by [@runzhen](https://github.com/runzhen))
- Add 'KEP Usage' KEP and template link ([#2423](https://github.com/kubeflow/trainer/pull/2423) by [@anishasthana](https://github.com/anishasthana))
- KEP-2170: Add validation to Torch `numProcPerNode` field ([#2409](https://github.com/kubeflow/trainer/pull/2409) by [@astefanutti](https://github.com/astefanutti))
- update migration url on readme file ([#2436](https://github.com/kubeflow/trainer/pull/2436) by [@varodrig](https://github.com/varodrig))
- IntegraionTests: Waiting for expected conditions before emulate JobSet controller manager ([#2425](https://github.com/kubeflow/trainer/pull/2425) by [@tenzen-y](https://github.com/tenzen-y))
- Nominate @Electronic-Waste as a reviewer ([#2427](https://github.com/kubeflow/trainer/pull/2427) by [@andreyvelich](https://github.com/andreyvelich))
- Update the naming conventions for Kubeflow Trainer ([#2415](https://github.com/kubeflow/trainer/pull/2415) by [@andreyvelich](https://github.com/andreyvelich))
- Rename paddlepaddle_defaults.go file name ([#2399](https://github.com/kubeflow/trainer/pull/2399) by [@ChristianZaccaria](https://github.com/ChristianZaccaria))
- Bump golang.org/x/net from 0.30.0 to 0.33.0 ([#2391](https://github.com/kubeflow/trainer/pull/2391) by [@dependabot[bot]](https://github.com/apps/dependabot))
- KEP-2170: Add unit and Integration tests for model and dataset initializers ([#2323](https://github.com/kubeflow/trainer/pull/2323) by [@seanlaii](https://github.com/seanlaii))
- Testing CI in JAX example ([#2385](https://github.com/kubeflow/trainer/pull/2385) by [@saileshd1402](https://github.com/saileshd1402))
- Upgrade huggingface_hub to v0.27.x in dataset initializer v2 ([#2379](https://github.com/kubeflow/trainer/pull/2379) by [@astefanutti](https://github.com/astefanutti))
- Add Changelog for Training Operator v1.9.0-rc.0 ([#2380](https://github.com/kubeflow/trainer/pull/2380) by [@andreyvelich](https://github.com/andreyvelich))
- Add release branch to the image push trigger ([#2376](https://github.com/kubeflow/trainer/pull/2376) by [@andreyvelich](https://github.com/andreyvelich))

[Full Changelog](https://github.com/kubeflow/trainer/compare/v1.8.1...v2.0.0-rc.0)
