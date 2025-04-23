/*
Copyright 2024 The Kubeflow Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	"github.com/kubeflow/trainer/pkg/constants"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestValidateReplicatedJobs(t *testing.T) {
	cases := map[string]struct {
		rJobs     []jobsetv1alpha2.ReplicatedJob
		wantError field.ErrorList
	}{
		"valid replicatedJobs": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				LauncherReplica().
				Replicas(1, constants.Launcher, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				Obj().Spec.ReplicatedJobs,
		},
		"valid replicatedJobs with unknown user-specified ancestor": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				ReplicatedJobLabel(constants.LabelTrainJobAncestor, "user-specified", constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				Replicas(2, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				Obj().Spec.ReplicatedJobs,
		},
		"invalid replicas": {
			rJobs: testingutil.MakeJobSetWrapper("ns", "valid").
				LauncherReplica().
				Replicas(2, constants.Launcher, constants.Node, constants.DatasetInitializer, constants.ModelInitializer).
				Obj().Spec.ReplicatedJobs,
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(0).Child("replicas"),
					"2", ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(1).Child("replicas"),
					"2", ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(2).Child("replicas"),
					"2", ""),
				field.Invalid(field.NewPath("spec").Child("template").Child("spec").Child("replicatedJobs").Index(3).Child("replicas"),
					"2", ""),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateReplicatedJobs(tc.rJobs)
			if diff := cmp.Diff(tc.wantError, gotErr, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("validateReplicateJobs() mismatch (-want,+got):\n%s", diff)
			}
		})
	}
}
