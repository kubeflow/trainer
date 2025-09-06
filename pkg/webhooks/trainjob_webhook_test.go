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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	runtimecore "github.com/kubeflow/trainer/v2/pkg/runtime/core"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

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
			runtimes, err := runtimecore.New(context.Background(), clientBuilder.Build(), testingutil.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}

			validator := &TrainJobWebhook{
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
