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

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestTrainJobPrinterColumnConditions(t *testing.T) {
	cases := map[string]struct {
		trainJob      *TrainJob
		wantState     string
		wantSuspended bool
	}{
		"newly created running TrainJob with no conditions": {
			trainJob: &TrainJob{
				Spec: TrainJobSpec{
					Suspend: ptr.To(false),
				},
				Status: TrainJobStatus{},
			},
			wantState:     "",
			wantSuspended: false,
		},
		"suspended TrainJob": {
			trainJob: &TrainJob{
				Spec: TrainJobSpec{
					Suspend: ptr.To(true),
				},
				Status: TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    TrainJobSuspended,
							Status:  metav1.ConditionTrue,
							Reason:  TrainJobSuspendedReason,
							Message: "TrainJob is suspended",
						},
					},
				},
			},
			wantState:     TrainJobSuspended,
			wantSuspended: true,
		},
		"resumed TrainJob with Suspended condition False": {
			trainJob: &TrainJob{
				Spec: TrainJobSpec{
					Suspend: ptr.To(false),
				},
				Status: TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    TrainJobSuspended,
							Status:  metav1.ConditionFalse,
							Reason:  TrainJobResumedReason,
							Message: "TrainJob is resumed",
						},
					},
				},
			},
			wantState:     "",
			wantSuspended: false,
		},
		"completed TrainJob after resume": {
			trainJob: &TrainJob{
				Spec: TrainJobSpec{
					Suspend: ptr.To(false),
				},
				Status: TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    TrainJobSuspended,
							Status:  metav1.ConditionFalse,
							Reason:  TrainJobResumedReason,
							Message: "TrainJob is resumed",
						},
						{
							Type:    TrainJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  "AllJobsCompleted",
							Message: "All jobs completed successfully",
						},
					},
				},
			},
			wantState:     TrainJobComplete,
			wantSuspended: false,
		},
		"failed TrainJob after resume": {
			trainJob: &TrainJob{
				Spec: TrainJobSpec{
					Suspend: ptr.To(false),
				},
				Status: TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:    TrainJobSuspended,
							Status:  metav1.ConditionFalse,
							Reason:  TrainJobResumedReason,
							Message: "TrainJob is resumed",
						},
						{
							Type:    TrainJobFailed,
							Status:  metav1.ConditionTrue,
							Reason:  TrainJobDeadlineExceededReason,
							Message: "Deadline exceeded",
						},
					},
				},
			},
			wantState:     TrainJobFailed,
			wantSuspended: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Test active condition resolution matching .status.conditions[?(@.status=="True")].type
			var gotState string
			for _, cond := range tc.trainJob.Status.Conditions {
				if cond.Status == metav1.ConditionTrue {
					gotState = cond.Type
					break
				}
			}
			if gotState != tc.wantState {
				t.Errorf("active condition type = %q, want %q", gotState, tc.wantState)
			}

			// Test suspended spec resolution matching .spec.suspend
			gotSuspended := ptr.Deref(tc.trainJob.Spec.Suspend, false)
			if gotSuspended != tc.wantSuspended {
				t.Errorf("spec.suspend = %v, want %v", gotSuspended, tc.wantSuspended)
			}
		})
	}
}
