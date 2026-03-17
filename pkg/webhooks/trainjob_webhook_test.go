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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2/ktesting"
	clocktesting "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
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
		"UPDATE, patch changed with externally-set Time: Time preserved": {
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
				if !patches[0].Time.Equal(&oldTime) {
					t.Errorf("expected externally-set Time %v preserved, got %v", oldTime, patches[0].Time)
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
		wantError              field.ErrorList
		wantWarnings           admission.Warnings
	}{
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
					Field: "spec.RuntimeRef",
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)

			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.clusterTrainingRuntime)
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
