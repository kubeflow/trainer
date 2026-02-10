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

package controller

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestTrainJobFinishTime(t *testing.T) {
	now := metav1.Now()
	fiveMinutesAgo := metav1.NewTime(now.Add(-5 * time.Minute))

	testCases := map[string]struct {
		trainJob *trainer.TrainJob
		want     *metav1.Time
	}{
		"no conditions - returns nil": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: nil,
				},
			},
			want: nil,
		},
		"suspended condition only - returns nil": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobSuspended,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			want: nil,
		},
		"complete condition with status true - returns finish time": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobComplete,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			want: &fiveMinutesAgo,
		},
		"complete condition with status false - returns nil": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobComplete,
							Status:             metav1.ConditionFalse,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			want: nil,
		},
		"failed condition with status true - returns finish time": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobFailed,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			want: &fiveMinutesAgo,
		},
		"complete condition with zero time - returns nil": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobComplete,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Time{},
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := trainJobFinishTime(tc.trainJob)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("trainJobFinishTime() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNeedsTTLCleanup(t *testing.T) {
	now := metav1.Now()
	fiveMinutesAgo := metav1.NewTime(now.Add(-5 * time.Minute))

	testCases := map[string]struct {
		trainJob    *trainer.TrainJob
		runtimeSpec *trainer.TrainingRuntimeSpec
		want        bool
	}{
		"TTL not set - returns false": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobComplete,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			runtimeSpec: &trainer.TrainingRuntimeSpec{
				TTLSecondsAfterFinished: nil,
			},
			want: false,
		},
		"TTL set but job not finished - returns false": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobSuspended,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			runtimeSpec: &trainer.TrainingRuntimeSpec{
				TTLSecondsAfterFinished: ptr.To(int32(60)),
			},
			want: false,
		},
		"TTL set and job completed - returns true": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobComplete,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			runtimeSpec: &trainer.TrainingRuntimeSpec{
				TTLSecondsAfterFinished: ptr.To(int32(60)),
			},
			want: true,
		},
		"TTL set and job failed - returns true": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobFailed,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			runtimeSpec: &trainer.TrainingRuntimeSpec{
				TTLSecondsAfterFinished: ptr.To(int32(60)),
			},
			want: true,
		},
		"TTL = 0 and job completed - returns true": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobComplete,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			runtimeSpec: &trainer.TrainingRuntimeSpec{
				TTLSecondsAfterFinished: ptr.To(int32(0)),
			},
			want: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := needsTTLCleanup(tc.trainJob, tc.runtimeSpec)
			if got != tc.want {
				t.Errorf("needsTTLCleanup() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsDeadlineEnforcement(t *testing.T) {
	now := metav1.Now()
	fiveMinutesAgo := metav1.NewTime(now.Add(-5 * time.Minute))

	testCases := map[string]struct {
		trainJob    *trainer.TrainJob
		runtimeSpec *trainer.TrainingRuntimeSpec
		want        bool
	}{
		"deadline not set in either - returns false": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ActiveDeadlineSeconds: nil,
				},
				Status: trainer.TrainJobStatus{
					Conditions: nil,
				},
			},
			runtimeSpec: &trainer.TrainingRuntimeSpec{
				ActiveDeadlineSeconds: nil,
			},
			want: false,
		},
		"deadline set in TrainJob and job not finished - returns true": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ActiveDeadlineSeconds: ptr.To(int64(3600)),
				},
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobSuspended,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			runtimeSpec: nil,
			want:        true,
		},
		"deadline set in Runtime and job not finished - returns true": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ActiveDeadlineSeconds: nil,
				},
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobSuspended,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			runtimeSpec: &trainer.TrainingRuntimeSpec{
				ActiveDeadlineSeconds: ptr.To(int64(3600)),
			},
			want: true,
		},
		"deadline set but job completed - returns false": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ActiveDeadlineSeconds: ptr.To(int64(3600)),
				},
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobComplete,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			runtimeSpec: nil,
			want:        false,
		},
		"deadline set but job failed - returns false": {
			trainJob: &trainer.TrainJob{
				Spec: trainer.TrainJobSpec{
					ActiveDeadlineSeconds: ptr.To(int64(3600)),
				},
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:               trainer.TrainJobFailed,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: fiveMinutesAgo,
						},
					},
				},
			},
			runtimeSpec: nil,
			want:        false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := needsDeadlineEnforcement(tc.trainJob, tc.runtimeSpec)
			if got != tc.want {
				t.Errorf("needsDeadlineEnforcement() = %v, want %v", got, tc.want)
			}
		})
	}
}
