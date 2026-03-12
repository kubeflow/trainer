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

package preemption

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPodPreempted(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with DisruptionTarget condition and PreemptionByScheduler reason",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.DisruptionTarget,
							Status: corev1.ConditionTrue,
							Reason: PreemptionBySchedulerReason,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with DisruptionTarget condition but different reason",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.DisruptionTarget,
							Status: corev1.ConditionTrue,
							Reason: "EvictionByEvictionAPI",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with DisruptionTarget condition but status is False",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.DisruptionTarget,
							Status: corev1.ConditionFalse,
							Reason: PreemptionBySchedulerReason,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod without DisruptionTarget condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with no conditions",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPodPreempted(tc.pod)
			if result != tc.expected {
				t.Errorf("IsPodPreempted() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestGetPreemptionRestartCount(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected int32
	}{
		{
			name: "pod with restart count annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreemptionRestartCountAnnotation: "3",
					},
				},
			},
			expected: 3,
		},
		{
			name: "pod without annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: 0,
		},
		{
			name: "pod with invalid annotation value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreemptionRestartCountAnnotation: "invalid",
					},
				},
			},
			expected: 0,
		},
		{
			name: "pod with zero count",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						PreemptionRestartCountAnnotation: "0",
					},
				},
			},
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetPreemptionRestartCount(tc.pod)
			if result != tc.expected {
				t.Errorf("GetPreemptionRestartCount() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestCountNonPreemptedFailedPods(t *testing.T) {
	tests := []struct {
		name     string
		pods     []corev1.Pod
		expected int32
	}{
		{
			name: "mix of preempted and non-preempted failed pods",
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.DisruptionTarget,
								Status: corev1.ConditionTrue,
								Reason: PreemptionBySchedulerReason,
							},
						},
					},
				},
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
					},
				},
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			expected: 1,
		},
		{
			name:     "no pods",
			pods:     []corev1.Pod{},
			expected: 0,
		},
		{
			name: "all preempted",
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.DisruptionTarget,
								Status: corev1.ConditionTrue,
								Reason: PreemptionBySchedulerReason,
							},
						},
					},
				},
			},
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CountNonPreemptedFailedPods(tc.pods)
			if result != tc.expected {
				t.Errorf("CountNonPreemptedFailedPods() = %v, expected %v", result, tc.expected)
			}
		})
	}
}
