/*
Copyright The Kubeflow Authors.

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

package trainjob

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestRuntimeRefIsTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		ref  trainer.RuntimeRef
		want bool
	}{
		"runtimeRef is TrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: true,
		},
		"runtimeRef is not TrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has wrong APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: ptr.To("other.group.io"),
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: nil,
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil Kind": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     nil,
			},
			want: false,
		},
		"runtimeRef has both nil": {
			ref:  trainer.RuntimeRef{},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RuntimeRefIsTrainingRuntime(tc.ref)
			if got != tc.want {
				t.Errorf("RuntimeRefIsTrainingRuntime(%v) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestRuntimeRefIsClusterTrainingRuntime(t *testing.T) {
	cases := map[string]struct {
		ref  trainer.RuntimeRef
		want bool
	}{
		"runtimeRef is ClusterTrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: true,
		},
		"runtimeRef is TrainingRuntime not ClusterTrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has wrong APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: ptr.To("other.group.io"),
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil APIGroup": {
			ref: trainer.RuntimeRef{
				APIGroup: nil,
				Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
			},
			want: false,
		},
		"runtimeRef has nil Kind": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     nil,
			},
			want: false,
		},
		"runtimeRef has both nil": {
			ref:  trainer.RuntimeRef{},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RuntimeRefIsClusterTrainingRuntime(tc.ref)
			if got != tc.want {
				t.Errorf("RuntimeRefIsClusterTrainingRuntime(%v) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestIsTrainJobFinished(t *testing.T) {
	cases := map[string]struct {
		trainJob *trainer.TrainJob
		want     bool
	}{
		"completed TrainJob is finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobComplete,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		"failed TrainJob is finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobFailed,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		"running TrainJob with no conditions is not finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{},
				},
			},
			want: false,
		},
		"TrainJob with Complete=False is not finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobComplete,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		"TrainJob with Failed=False is not finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobFailed,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		"TrainJob with both Complete and Failed true is finished": {
			trainJob: &trainer.TrainJob{
				Status: trainer.TrainJobStatus{
					Conditions: []metav1.Condition{
						{
							Type:   trainer.TrainJobComplete,
							Status: metav1.ConditionTrue,
						},
						{
							Type:   trainer.TrainJobFailed,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsTrainJobFinished(tc.trainJob)
			if got != tc.want {
				t.Errorf("IsTrainJobFinished(%v) = %v, want %v", tc.trainJob, got, tc.want)
			}
		})
	}
}