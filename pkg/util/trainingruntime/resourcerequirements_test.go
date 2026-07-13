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

package trainingruntime

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestMergeResourceRequirements(t *testing.T) {
	cases := map[string]struct {
		base     corev1.ResourceRequirements
		override corev1.ResourceRequirements
		want     corev1.ResourceRequirements
	}{
		"gpu only overlays runtime cpu and memory": {
			base: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
			override: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("4"),
				},
			},
			want: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
					"nvidia.com/gpu":      resource.MustParse("4"),
				},
			},
		},
		"trainjob cpu overrides runtime cpu": {
			base: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("8"),
				},
			},
			override: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				},
			},
			want: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				},
			},
		},
		"partial override keeps other runtime keys": {
			base: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("5"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
			override: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"nvidia.com/gpu":      resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			want: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("5"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
					"nvidia.com/gpu":      resource.MustParse("2"),
				},
			},
		},
		"empty override returns base": {
			base: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			override: corev1.ResourceRequirements{},
			want: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
		},
		"trainjob requests override runtime requests": {
			base: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
			override: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				},
			},
			want: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
		},
		"trainjob claims override runtime claims by name": {
			base: corev1.ResourceRequirements{
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "default"},
				},
			},
			override: corev1.ResourceRequirements{
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "large"},
				},
			},
			want: corev1.ResourceRequirements{
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "large"},
				},
			},
		},
		"trainjob claims merge keeps other runtime claims": {
			base: corev1.ResourceRequirements{
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "default"},
					{Name: "storage", Request: "fast"},
				},
			},
			override: corev1.ResourceRequirements{
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "large"},
				},
			},
			want: corev1.ResourceRequirements{
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "large"},
					{Name: "storage", Request: "fast"},
				},
			},
		},
		"unset override claims keep runtime claims": {
			base: corev1.ResourceRequirements{
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "default"},
				},
			},
			override: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				},
			},
			want: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				},
				Claims: []corev1.ResourceClaim{
					{Name: "gpu", Request: "default"},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := MergeResourceRequirements(tc.base, tc.override)
			if err != nil {
				t.Fatalf("MergeResourceRequirements(): %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected diff (-want,+got):\n%s", diff)
			}
		})
	}
}
