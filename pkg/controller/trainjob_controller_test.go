/*
Copyright 2026 The Kubeflow Authors.

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

package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestRemoveFailedCondition(t *testing.T) {
	cases := map[string]struct {
		conditions     []metav1.Condition
		wantConditions []metav1.Condition
	}{
		"no failed condition": {
			conditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobSuspended,
					Status: metav1.ConditionFalse,
				},
			},
			wantConditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobSuspended,
					Status: metav1.ConditionFalse,
				},
			},
		},
		"unsupported runtime is recoverable": {
			conditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobFailed,
					Status: metav1.ConditionTrue,
					Reason: trainer.TrainJobRuntimeNotSupportedReason,
				},
			},
			wantConditions: nil,
		},
		"jobset failure is terminal": {
			conditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobFailed,
					Status: metav1.ConditionTrue,
					Reason: "FailedJobs",
				},
			},
			wantConditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobFailed,
					Status: metav1.ConditionTrue,
					Reason: "FailedJobs",
				},
			},
		},
		"deadline exceeded is terminal": {
			conditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobFailed,
					Status: metav1.ConditionTrue,
					Reason: trainer.TrainJobDeadlineExceededReason,
				},
			},
			wantConditions: []metav1.Condition{
				{
					Type:   trainer.TrainJobFailed,
					Status: metav1.ConditionTrue,
					Reason: trainer.TrainJobDeadlineExceededReason,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trainJob := &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: tc.conditions,
				},
			}
			removeFailedCondition(trainJob)
			if diff := cmp.Diff(tc.wantConditions, trainJob.Status.Conditions, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("unexpected conditions (-want +got):\n%s", diff)
			}
			if tc.wantConditions != nil && !meta.IsStatusConditionTrue(trainJob.Status.Conditions, trainer.TrainJobFailed) &&
				meta.FindStatusCondition(tc.wantConditions, trainer.TrainJobFailed) != nil {
				t.Errorf("expected Failed condition to remain set")
			}
		})
	}
}
