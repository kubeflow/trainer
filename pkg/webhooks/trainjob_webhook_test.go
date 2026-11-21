/*
Copyright 2025 The Kubeflow Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/klog/v2/ktesting"
	clocktesting "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/features"
	runtimecore "github.com/kubeflow/trainer/v2/pkg/runtime/core"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestDefault(t *testing.T) {
	fakeNow := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	fakeClock := clocktesting.NewFakeClock(fakeNow)
	expectedTime := metav1.NewTime(fakeNow)
	oldTime := metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	cases := map[string]struct {
		oldObj   *trainer.TrainJob
		newObj   *trainer.TrainJob
		wantTime func(patches []trainer.RuntimePatch)
	}{
		"CREATE: all patches get timestamped": {
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{
					{Manager: "acme.io/one"},
					{Manager: "acme.io/two"},
				}).Obj(),
			wantTime: func(patches []trainer.RuntimePatch) {
				for _, p := range patches {
					if !p.Time.Equal(&expectedTime) {
						t.Errorf("patch %q: expected Time %v, got %v", p.Manager, expectedTime, p.Time)
					}
				}
			},
		},
		"UPDATE, patch unchanged: Time preserved": {
			oldObj: func() *trainer.TrainJob {
				obj := testingutil.MakeTrainJobWrapper("default", "test").
					RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
					RuntimePatches([]trainer.RuntimePatch{
						{
							Manager: "acme.io/one",
							Time:    &oldTime,
							TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
								Template: &trainer.JobSetTemplatePatch{
									Spec: &trainer.JobSetSpecPatch{
										ReplicatedJobs: []trainer.ReplicatedJobPatch{{
											Name: "node",
											Template: &trainer.JobTemplatePatch{
												Spec: &trainer.JobSpecPatch{
													Template: &trainer.PodTemplatePatch{
														Spec: &trainer.PodSpecPatch{
															ServiceAccountName: ptr.To("sa"),
														},
													},
												},
											},
										}},
									},
								},
							},
						},
					}).Obj()
				return obj
			}(),
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{
					{
						Manager: "acme.io/one",
						TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
							Template: &trainer.JobSetTemplatePatch{
								Spec: &trainer.JobSetSpecPatch{
									ReplicatedJobs: []trainer.ReplicatedJobPatch{{
										Name: "node",
										Template: &trainer.JobTemplatePatch{
											Spec: &trainer.JobSpecPatch{
												Template: &trainer.PodTemplatePatch{
													Spec: &trainer.PodSpecPatch{
														ServiceAccountName: ptr.To("sa"),
													},
												},
											},
										},
									}},
								},
							},
						},
					},
				}).Obj(),
			wantTime: func(patches []trainer.RuntimePatch) {
				if !patches[0].Time.Equal(&oldTime) {
					t.Errorf("expected Time to be preserved as %v, got %v", oldTime, patches[0].Time)
				}
			},
		},
		"UPDATE, patch changed: Time updated": {
			oldObj: func() *trainer.TrainJob {
				obj := testingutil.MakeTrainJobWrapper("default", "test").
					RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
					RuntimePatches([]trainer.RuntimePatch{
						{
							Manager: "acme.io/one",
							Time:    &oldTime,
							TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
								Template: &trainer.JobSetTemplatePatch{
									Spec: &trainer.JobSetSpecPatch{
										ReplicatedJobs: []trainer.ReplicatedJobPatch{{
											Name: "node",
											Template: &trainer.JobTemplatePatch{
												Spec: &trainer.JobSpecPatch{
													Template: &trainer.PodTemplatePatch{
														Spec: &trainer.PodSpecPatch{
															ServiceAccountName: ptr.To("old-sa"),
														},
													},
												},
											},
										}},
									},
								},
							},
						},
					}).Obj()
				return obj
			}(),
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{
					{
						Manager: "acme.io/one",
						TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
							Template: &trainer.JobSetTemplatePatch{
								Spec: &trainer.JobSetSpecPatch{
									ReplicatedJobs: []trainer.ReplicatedJobPatch{{
										Name: "node",
										Template: &trainer.JobTemplatePatch{
											Spec: &trainer.JobSpecPatch{
												Template: &trainer.PodTemplatePatch{
													Spec: &trainer.PodSpecPatch{
														ServiceAccountName: ptr.To("new-sa"),
													},
												},
											},
										},
									}},
								},
							},
						},
					},
				}).Obj(),
			wantTime: func(patches []trainer.RuntimePatch) {
				if !patches[0].Time.Equal(&expectedTime) {
					t.Errorf("expected Time to be updated to %v, got %v", expectedTime, patches[0].Time)
				}
			},
		},
		"CREATE: externally-set Time is preserved": {
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{
					{Manager: "acme.io/one", Time: &oldTime},
					{Manager: "acme.io/two"},
				}).Obj(),
			wantTime: func(patches []trainer.RuntimePatch) {
				if !patches[0].Time.Equal(&oldTime) {
					t.Errorf("patch %q: expected externally-set Time %v preserved, got %v", patches[0].Manager, oldTime, patches[0].Time)
				}
				if !patches[1].Time.Equal(&expectedTime) {
					t.Errorf("patch %q: expected Time %v, got %v", patches[1].Manager, expectedTime, patches[1].Time)
				}
			},
		},
		"UPDATE, patch changed with pre-existing Time: Time updated": {
			oldObj: func() *trainer.TrainJob {
				obj := testingutil.MakeTrainJobWrapper("default", "test").
					RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
					RuntimePatches([]trainer.RuntimePatch{
						{Manager: "acme.io/one", Time: &oldTime},
					}).Obj()
				return obj
			}(),
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{
					{
						Manager: "acme.io/one",
						Time:    &oldTime,
						TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
							Template: &trainer.JobSetTemplatePatch{
								Spec: &trainer.JobSetSpecPatch{
									ReplicatedJobs: []trainer.ReplicatedJobPatch{{
										Name: "node",
									}},
								},
							},
						},
					},
				}).Obj(),
			wantTime: func(patches []trainer.RuntimePatch) {
				if !patches[0].Time.Equal(&expectedTime) {
					t.Errorf("expected Time to be updated to %v, got %v", expectedTime, patches[0].Time)
				}
			},
		},
		"UPDATE, new patch added alongside unchanged patch": {
			oldObj: func() *trainer.TrainJob {
				obj := testingutil.MakeTrainJobWrapper("default", "test").
					RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
					RuntimePatches([]trainer.RuntimePatch{
						{Manager: "acme.io/existing", Time: &oldTime},
					}).Obj()
				return obj
			}(),
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{
					{Manager: "acme.io/existing"},
					{Manager: "acme.io/new"},
				}).Obj(),
			wantTime: func(patches []trainer.RuntimePatch) {
				if !patches[0].Time.Equal(&oldTime) {
					t.Errorf("existing patch: expected Time preserved as %v, got %v", oldTime, patches[0].Time)
				}
				if !patches[1].Time.Equal(&expectedTime) {
					t.Errorf("new patch: expected Time %v, got %v", expectedTime, patches[1].Time)
				}
			},
		},
		"UPDATE, new patch added with externally-set Time: Time preserved": {
			oldObj: func() *trainer.TrainJob {
				obj := testingutil.MakeTrainJobWrapper("default", "test").
					RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
					RuntimePatches([]trainer.RuntimePatch{
						{Manager: "acme.io/existing", Time: &oldTime},
					}).Obj()
				return obj
			}(),
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{
					{Manager: "acme.io/existing"},
					{Manager: "acme.io/new", Time: &oldTime},
				}).Obj(),
			wantTime: func(patches []trainer.RuntimePatch) {
				if !patches[0].Time.Equal(&oldTime) {
					t.Errorf("existing patch: expected Time preserved as %v, got %v", oldTime, patches[0].Time)
				}
				if !patches[1].Time.Equal(&oldTime) {
					t.Errorf("new patch: expected externally-set Time %v preserved, got %v", oldTime, patches[1].Time)
				}
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)

			operation := admissionv1.Create
			req := admissionv1.AdmissionRequest{}
			if tc.oldObj != nil {
				operation = admissionv1.Update
				raw, err := json.Marshal(tc.oldObj)
				if err != nil {
					t.Fatal(err)
				}
				req.OldObject = apiruntime.RawExtension{Raw: raw}
			}
			req.Operation = operation
			ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: req})

			defaulter := &TrainJobDefaulter{clock: fakeClock}
			if err := defaulter.Default(ctx, tc.newObj); err != nil {
				t.Fatalf("Default returned unexpected error: %v", err)
			}
			tc.wantTime(tc.newObj.Spec.RuntimePatches)
		})
	}
}

func TestValidateCreate(t *testing.T) {
	cases := map[string]struct {
		obj                    *trainer.TrainJob
		clusterTrainingRuntime *trainer.ClusterTrainingRuntime
		trainingRuntime        *trainer.TrainingRuntime
		wantError              field.ErrorList
		wantWarnings           admission.Warnings
		enableFeatureGates     []featuregate.Feature
	}{
		"DRA fields are accepted with the TrainJobDynamicResourceAllocation gate enabled": {
			obj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						ResourceClaimsPerNode(trainer.TrainerResourceClaim{
							Name:                      "gpu",
							ResourceClaimTemplateName: "gpu-template",
						}).
						Obj(),
				).
				Obj(),
			clusterTrainingRuntime: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
					},
				}).Obj(),
			enableFeatureGates: []featuregate.Feature{features.TrainJobDynamicResourceAllocation},
		},
		"valid trainjob name compliant with RFC 1035": {
			obj: testingutil.MakeTrainJobWrapper("default", "valid-job-name").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			clusterTrainingRuntime: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
				RuntimeSpec(trainer.TrainingRuntimeSpec{
					Template: trainer.JobSetTemplateSpec{
						Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
					},
				}).Obj(),
			wantError:    nil,
			wantWarnings: nil,
		},
		"unsupported runtime": {
			obj: testingutil.MakeTrainJobWrapper("default", "valid-job-name").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "unsupported-runtime").
				Obj(),
			// clusterTrainingRuntime: nil (no such runtime exists)
			wantError: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "spec.runtimeRef",
					BadValue: trainer.RuntimeRef{
						Name:     "unsupported-runtime",
						APIGroup: ptr.To(trainer.SchemeGroupVersion.Group),
						Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
					},
				},
			},
			wantWarnings: nil,
		},
		"deprecated runtime referenced": {
			obj: testingutil.MakeTrainJobWrapper("default", "valid-job-name").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			clusterTrainingRuntime: func() *trainer.ClusterTrainingRuntime {
				rt := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
					RuntimeSpec(trainer.TrainingRuntimeSpec{
						Template: trainer.JobSetTemplateSpec{
							Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
						},
					}).Obj()
				if rt.Labels == nil {
					rt.Labels = map[string]string{}
				}
				rt.Labels[constants.LabelSupport] = constants.SupportDeprecated
				return rt
			}(),
			wantError:    nil,
			wantWarnings: admission.Warnings{"Referenced ClusterTrainingRuntime \"test-runtime\" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: " + constants.RuntimeDeprecationPolicyURL},
		},
		"deprecated namespaced TrainingRuntime referenced": {
			obj: testingutil.MakeTrainJobWrapper("default", "valid-job-name").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").
				Label(constants.LabelSupport, constants.SupportDeprecated).
				Obj(),
			wantError:    nil,
			wantWarnings: admission.Warnings{"Referenced TrainingRuntime \"test-runtime\" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: " + constants.RuntimeDeprecationPolicyURL},
		},
		"non-deprecated namespaced TrainingRuntime referenced": {
			obj: testingutil.MakeTrainJobWrapper("default", "valid-job-name").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			trainingRuntime: testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").
				Label(constants.LabelSupport, "stable").
				Obj(),
			wantError:    nil,
			wantWarnings: nil,
		},
		"gate disabled with trainer resourceClaimsPerNode is forbidden": {
			obj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						ResourceClaimsPerNode(trainer.TrainerResourceClaim{
							Name:                      "gpu",
							ResourceClaimTemplateName: "gpu-template",
						}).
						Obj(),
				).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "trainer", "resourceClaimsPerNode"), ""),
			},
		},
		"gate disabled with runtimePatch resourceClaims is forbidden": {
			obj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				RuntimePatches([]trainer.RuntimePatch{{
					Manager: "acme.io/mgr",
					TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
						Template: &trainer.JobSetTemplatePatch{
							Spec: &trainer.JobSetSpecPatch{
								ReplicatedJobs: []trainer.ReplicatedJobPatch{{
									Name: "node",
									Template: &trainer.JobTemplatePatch{
										Spec: &trainer.JobSpecPatch{
											Template: &trainer.PodTemplatePatch{
												Spec: &trainer.PodSpecPatch{
													ResourceClaims: []corev1.PodResourceClaim{{
														Name: "gpu",
													}},
												},
											},
										},
									},
								}},
							},
						},
					},
				}}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "runtimePatches").Index(0).
						Child("trainingRuntimeSpec", "template", "spec", "replicatedJobs").Index(0).
						Child("template", "spec", "template", "spec", "resourceClaims"),
					"",
				),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGateDuringTest mutates global state.
			for _, gate := range tc.enableFeatureGates {
				featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, gate, true)
			}
			_, ctx := ktesting.NewTestContext(t)

			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.clusterTrainingRuntime)
			}
			if tc.trainingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.trainingRuntime)
			}

			runtimes, err := runtimecore.New(context.Background(), clientBuilder.Build(), testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}

			validator := &TrainJobValidator{
				runtimes: runtimes,
			}

			warnings, err := validator.ValidateCreate(ctx, tc.obj)
			if diff := cmp.Diff(tc.wantWarnings, warnings); len(diff) != 0 {
				t.Errorf("Unexpected warnings from ValidateCreate (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail")); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateCreate (-want, +got): %s", diff)
			}
		})
	}
}

func TestValidateUpdate(t *testing.T) {
	runtimeSpec := trainer.TrainingRuntimeSpec{
		Template: trainer.JobSetTemplateSpec{
			Spec: testingutil.MakeJobSetWrapper("", "").Obj().Spec,
		},
	}
	clusterRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").RuntimeSpec(runtimeSpec).Obj()
	trainingRuntime := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").RuntimeSpec(runtimeSpec).Obj()
	volcanoRuntime := testingutil.MakeTrainingRuntimeWrapper("default", "test-runtime").
		RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(trainer.TrainingRuntimeSpec{
			Template: trainer.JobSetTemplateSpec{
				Spec: testingutil.MakeJobSetWrapper("", "").PodPriorityClassName("high-priority").Obj().Spec,
			},
		}).PodGroupPolicyVolcano(&trainer.VolcanoPodGroupPolicySource{}).Obj()).
		Obj()
	// A snapshot ConfigMap whose ownerReference was removed, as the garbage collector does
	// when the TrainJob is deleted with --cascade=orphan.
	orphanedSnapshot := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-runtime-snapshot", Namespace: "default"},
		Data:       map[string]string{"runtime": ""},
	}

	trainJob := func(runtimeKind string) *testingutil.TrainJobWrapper {
		return testingutil.MakeTrainJobWrapper("default", "test").
			UID("test-uid").
			RuntimeRef(trainer.SchemeGroupVersion.WithKind(runtimeKind), "test-runtime")
	}
	deletedAt := metav1.NewTime(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	terminating := func(job *trainer.TrainJob, finalizers ...string) *trainer.TrainJob {
		job.DeletionTimestamp = &deletedAt
		job.Finalizers = finalizers
		return job
	}
	withMetadata := func(job *trainer.TrainJob) *trainer.TrainJob {
		job.Labels = map[string]string{"app.kubernetes.io/managed-by": "argocd"}
		job.Annotations = map[string]string{"example.com/note": "changed"}
		job.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "owner", UID: "owner-uid"}}
		return job
	}
	patchTime := metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	labelPatch := func(value string, patchTime *metav1.Time) []trainer.RuntimePatch {
		return []trainer.RuntimePatch{{
			Manager: "acme.io/manager",
			Time:    patchTime,
			TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
				Template: &trainer.JobSetTemplatePatch{
					Metadata: &metav1.ObjectMeta{Labels: map[string]string{"key": value}},
				},
			},
		}}
	}
	trainingRuntimeNotFound := field.ErrorList{
		field.Invalid(field.NewPath("spec", "runtimeRef"), trainJob(trainer.TrainingRuntimeKind).Obj().Spec.RuntimeRef, ""),
	}
	clusterTrainingRuntimeNotFound := field.ErrorList{
		field.Invalid(field.NewPath("spec", "runtimeRef"), trainJob(trainer.ClusterTrainingRuntimeKind).Obj().Spec.RuntimeRef, ""),
	}
	invalidSnapshot := field.ErrorList{
		field.InternalError(field.NewPath("spec", "runtimeRef"), fmt.Errorf("unable to get runtime snapshot: %w",
			errors.New("invalid runtime snapshot: a snapshot configmap exists but is owned by another object"))),
	}

	// Feature-gated fields are rejected on create while their gate is disabled. On update they are
	// rejected only when the update introduces them, so a TrainJob created while the gate was
	// enabled stays updatable (for example, suspend) after the gate is disabled. The cases below
	// use TrainJobDynamicResourceAllocation fields; the same shape works for any gated field.
	cases := map[string]struct {
		// objs are the objects in the cluster, e.g. the referenced runtime and its snapshot.
		objs   []client.Object
		oldObj *trainer.TrainJob
		newObj *trainer.TrainJob
		// runDefaulter runs the TrainJobDefaulter on newObj first, as the API server does.
		runDefaulter       bool
		wantError          field.ErrorList
		wantWarnings       admission.Warnings
		enableFeatureGates []featuregate.Feature
	}{
		"update adds a feature-gated field while the gate is disabled: forbidden": {
			objs: []client.Object{clusterRuntime},
			wantError: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "trainer", "resourceClaimsPerNode"), ""),
			},
			oldObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						ResourceClaimsPerNode(trainer.TrainerResourceClaim{
							Name:                      "gpu",
							ResourceClaimTemplateName: "gpu-template",
						}).
						Obj(),
				).
				Obj(),
		},
		"update of a TrainJob that already uses a feature-gated field while the gate is disabled: passes": {
			objs: []client.Object{clusterRuntime},
			oldObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						ResourceClaimsPerNode(trainer.TrainerResourceClaim{
							Name:                      "gpu",
							ResourceClaimTemplateName: "gpu-template",
						}).
						Obj(),
				).
				Obj(),
			newObj: testingutil.MakeTrainJobWrapper("default", "test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						ResourceClaimsPerNode(trainer.TrainerResourceClaim{
							Name:                      "gpu",
							ResourceClaimTemplateName: "gpu-template",
						}).
						Obj(),
				).
				Suspend(true).
				Obj(),
		},
		"TrainingRuntime: removing the deletion finalizer passes when the runtime and snapshot are deleted": {
			oldObj: terminating(trainJob(trainer.TrainingRuntimeKind).Obj(), metav1.FinalizerDeleteDependents),
			newObj: terminating(trainJob(trainer.TrainingRuntimeKind).Obj()),
		},
		"ClusterTrainingRuntime: removing the deletion finalizer passes when the runtime and snapshot are deleted": {
			oldObj: terminating(trainJob(trainer.ClusterTrainingRuntimeKind).Obj(), metav1.FinalizerDeleteDependents),
			newObj: terminating(trainJob(trainer.ClusterTrainingRuntimeKind).Obj()),
		},
		"TrainingRuntime: no-op update passes when the runtime and snapshot are deleted": {
			oldObj: trainJob(trainer.TrainingRuntimeKind).Obj(),
			newObj: trainJob(trainer.TrainingRuntimeKind).Obj(),
		},
		"TrainingRuntime: label, annotation and ownerReference update passes when the runtime and snapshot are deleted": {
			oldObj: trainJob(trainer.TrainingRuntimeKind).Obj(),
			newObj: withMetadata(trainJob(trainer.TrainingRuntimeKind).Obj()),
		},
		"ClusterTrainingRuntime: label, annotation and ownerReference update passes when the runtime and snapshot are deleted": {
			oldObj: trainJob(trainer.ClusterTrainingRuntimeKind).Obj(),
			newObj: withMetadata(trainJob(trainer.ClusterTrainingRuntimeKind).Obj()),
		},
		"TrainingRuntime: removing the orphan finalizer passes when the snapshot has no owner": {
			objs:   []client.Object{trainingRuntime, orphanedSnapshot},
			oldObj: terminating(trainJob(trainer.TrainingRuntimeKind).Obj(), metav1.FinalizerOrphanDependents),
			newObj: terminating(trainJob(trainer.TrainingRuntimeKind).Obj()),
		},
		"ClusterTrainingRuntime: removing the orphan finalizer passes when the snapshot has no owner": {
			objs:   []client.Object{clusterRuntime, orphanedSnapshot},
			oldObj: terminating(trainJob(trainer.ClusterTrainingRuntimeKind).Obj(), metav1.FinalizerOrphanDependents),
			newObj: terminating(trainJob(trainer.ClusterTrainingRuntimeKind).Obj()),
		},
		"TrainingRuntime: spec update is rejected when the runtime and snapshot are deleted": {
			oldObj:    trainJob(trainer.TrainingRuntimeKind).Obj(),
			newObj:    trainJob(trainer.TrainingRuntimeKind).Suspend(true).Obj(),
			wantError: trainingRuntimeNotFound,
		},
		"ClusterTrainingRuntime: spec update is rejected when the runtime and snapshot are deleted": {
			oldObj:    trainJob(trainer.ClusterTrainingRuntimeKind).Obj(),
			newObj:    trainJob(trainer.ClusterTrainingRuntimeKind).Suspend(true).Obj(),
			wantError: clusterTrainingRuntimeNotFound,
		},
		"TrainingRuntime: spec update is rejected when the snapshot has no owner": {
			objs:      []client.Object{trainingRuntime, orphanedSnapshot},
			oldObj:    trainJob(trainer.TrainingRuntimeKind).Obj(),
			newObj:    trainJob(trainer.TrainingRuntimeKind).Suspend(true).Obj(),
			wantError: invalidSnapshot,
		},
		"terminating TrainJob: spec update is still validated": {
			oldObj:    terminating(trainJob(trainer.TrainingRuntimeKind).Obj(), metav1.FinalizerDeleteDependents),
			newObj:    terminating(trainJob(trainer.TrainingRuntimeKind).Suspend(true).Obj(), metav1.FinalizerDeleteDependents),
			wantError: trainingRuntimeNotFound,
		},
		"terminating TrainJob: update adds a feature-gated field while the gate is disabled: forbidden": {
			objs:   []client.Object{clusterRuntime},
			oldObj: terminating(trainJob(trainer.ClusterTrainingRuntimeKind).Obj(), metav1.FinalizerDeleteDependents),
			newObj: terminating(trainJob(trainer.ClusterTrainingRuntimeKind).
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						ResourceClaimsPerNode(trainer.TrainerResourceClaim{
							Name:                      "gpu",
							ResourceClaimTemplateName: "gpu-template",
						}).
						Obj(),
				).
				Obj(), metav1.FinalizerDeleteDependents),
			wantError: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "trainer", "resourceClaimsPerNode"), ""),
			},
		},
		"defaulted: unchanged runtimePatch without time passes when the runtime is deleted": {
			runDefaulter: true,
			oldObj:       trainJob(trainer.TrainingRuntimeKind).RuntimePatches(labelPatch("a", nil)).Obj(),
			newObj:       withMetadata(trainJob(trainer.TrainingRuntimeKind).RuntimePatches(labelPatch("a", nil)).Obj()),
		},
		"defaulted: unchanged runtimePatch with time passes when the runtime is deleted": {
			runDefaulter: true,
			oldObj:       trainJob(trainer.TrainingRuntimeKind).RuntimePatches(labelPatch("a", &patchTime)).Obj(),
			newObj:       withMetadata(trainJob(trainer.TrainingRuntimeKind).RuntimePatches(labelPatch("a", nil)).Obj()),
		},
		"defaulted: changed runtimePatch is validated": {
			runDefaulter: true,
			oldObj:       trainJob(trainer.TrainingRuntimeKind).RuntimePatches(labelPatch("a", &patchTime)).Obj(),
			newObj:       trainJob(trainer.TrainingRuntimeKind).RuntimePatches(labelPatch("b", &patchTime)).Obj(),
			wantError:    trainingRuntimeNotFound,
		},
		"defaulted: added runtimePatch is validated": {
			runDefaulter: true,
			oldObj:       trainJob(trainer.TrainingRuntimeKind).Obj(),
			newObj:       trainJob(trainer.TrainingRuntimeKind).RuntimePatches(labelPatch("a", nil)).Obj(),
			wantError:    trainingRuntimeNotFound,
		},
		"Volcano: metadata update passes when the referenced PriorityClass is deleted": {
			objs:   []client.Object{volcanoRuntime},
			oldObj: trainJob(trainer.TrainingRuntimeKind).Obj(),
			newObj: withMetadata(trainJob(trainer.TrainingRuntimeKind).Obj()),
		},
		"Volcano: spec update is validated when the referenced PriorityClass is deleted": {
			objs:   []client.Object{volcanoRuntime},
			oldObj: trainJob(trainer.TrainingRuntimeKind).Obj(),
			newObj: trainJob(trainer.TrainingRuntimeKind).Suspend(true).Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "templateSpec", "priorityClassName"), "high-priority", ""),
			},
		},
	}

	// field.InternalError stores the wrapped error in BadValue; compare such errors by message.
	equateBadValueErrors := cmp.FilterPath(func(p cmp.Path) bool {
		for _, step := range p {
			if sf, ok := step.(cmp.StructField); ok && sf.Name() == "BadValue" {
				return true
			}
		}
		return false
	}, cmp.Comparer(func(a, b error) bool { return a.Error() == b.Error() }))

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Do not use t.Parallel() — SetFeatureGateDuringTest mutates global state.
			for _, gate := range tc.enableFeatureGates {
				featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, gate, true)
			}
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder().WithObjects(tc.objs...)
			runtimes, err := runtimecore.New(context.Background(), clientBuilder.Build(), testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}
			validator := &TrainJobValidator{runtimes: runtimes}

			if tc.runDefaulter {
				raw, err := json.Marshal(tc.oldObj)
				if err != nil {
					t.Fatal(err)
				}
				defaultCtx := admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: apiruntime.RawExtension{Raw: raw},
				}})
				defaulter := &TrainJobDefaulter{clock: clocktesting.NewFakeClock(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))}
				if err := defaulter.Default(defaultCtx, tc.newObj); err != nil {
					t.Fatalf("Default returned unexpected error: %v", err)
				}
			}

			warnings, err := validator.ValidateUpdate(ctx, tc.oldObj, tc.newObj)
			if diff := cmp.Diff(tc.wantWarnings, warnings); len(diff) != 0 {
				t.Errorf("Unexpected warnings from ValidateUpdate (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantError.ToAggregate(), err, cmpopts.IgnoreFields(field.Error{}, "Detail"), equateBadValueErrors); len(diff) != 0 {
				t.Errorf("Unexpected error from ValidateUpdate (-want, +got): %s", diff)
			}
		})
	}
}
