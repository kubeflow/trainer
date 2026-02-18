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

package kaischeduler

import (
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	trainerv1alpha1 "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

func TestKAIScheduler(t *testing.T) {
	cases := map[string]struct {
		info     *runtime.Info
		trainJob *trainerv1alpha1.TrainJob
		wantInfo *runtime.Info
		wantErr  error
	}{
		"no action when info is nil": {
			info:     nil,
			trainJob: &trainerv1alpha1.TrainJob{},
			wantInfo: nil,
		},
		"no action when podGroupPolicy is nil": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: nil,
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
			},
		},
		"no action when KAIScheduler is nil": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: nil,
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{},
				},
			},
		},
		"no action when trainJob is nil": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{
								Queue: ptr.To("team-queue"),
							},
						},
					},
				},
			},
			trainJob: nil,
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{
								Queue: ptr.To("team-queue"),
							},
						},
					},
				},
			},
		},
		"queue from typed API field": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{
								Queue: ptr.To("team-queue"),
							},
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "train-job",
					Namespace: metav1.NamespaceDefault,
				},
			},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{
						"kai.scheduler/queue": "team-queue",
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{
								Queue: ptr.To("team-queue"),
							},
						},
					},
				},
			},
		},
		"no queue label when queue is empty string": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{
								Queue: ptr.To(""),
							},
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "train-job",
					Namespace: metav1.NamespaceDefault,
				},
			},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{
								Queue: ptr.To(""),
							},
						},
					},
				},
			},
		},
		"no queue label when both sources are empty": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{},
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "train-job",
					Namespace: metav1.NamespaceDefault,
				},
			},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							KAIScheduler: &trainerv1alpha1.KAISchedulerPodGroupPolicySource{},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plugin := &KAIScheduler{}
			err := plugin.EnforcePodGroupPolicy(tc.info, tc.trainJob)
			if diff := gocmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if diff := gocmp.Diff(tc.wantInfo, tc.info); len(diff) != 0 {
				t.Errorf("Unexpected info (-want,+got):\n%s", diff)
			}
		})
	}
}
