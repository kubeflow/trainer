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

package trainjob

import (
	"testing"

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
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RuntimeRefIsTrainingRuntime(tc.ref)
			if got != tc.want {
				t.Errorf("Unexpected RuntimeRefIsTrainingRuntime()\nwant: %v\n, want: %v", got, tc.want)
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
		"runtimeRef is not ClusterTrainingRuntime": {
			ref: trainer.RuntimeRef{
				APIGroup: &trainer.GroupVersion.Group,
				Kind:     ptr.To(trainer.TrainingRuntimeKind),
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RuntimeRefIsClusterTrainingRuntime(tc.ref)
			if got != tc.want {
				t.Errorf("Unexpected RuntimeRefIsClusterTrainingRuntime()\nwant: %v\n, want: %v", got, tc.want)
			}
		})
	}
}
