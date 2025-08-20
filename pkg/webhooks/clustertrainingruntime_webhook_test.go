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
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestClusterTrainingRuntime_ValidateCreate_DeprecatedWarning(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	obj := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
		RuntimeSpec(trainer.TrainingRuntimeSpec{
			Template: trainer.JobSetTemplateSpec{
				Spec: func() jobsetv1alpha2.JobSetSpec {
					js := testingutil.MakeJobSetWrapper("", "")
					js.Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node)
					return js.Obj().Spec
				}(),
			},
		}).Obj()

	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[constants.LabelDeprecated] = constants.DeprecatedTrueValue

	validator := &ClusterTrainingRuntimeWebhook{}
	warnings, err := validator.ValidateCreate(ctx, obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var want admission.Warnings
	want = append(want, "ClusterTrainingRuntime \"test-runtime\" is marked deprecated (trainer.kubeflow.org/deprecated=true). See runtime deprecation policy: https://www.kubeflow.org/docs/components/trainer/operator-guides/runtime/#runtime-deprecation-policy")
	if diff := cmp.Diff(want, warnings); diff != "" {
		t.Fatalf("unexpected warnings (-want, +got): %s", diff)
	}
}
