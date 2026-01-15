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

package flux

import (
	"fmt"
	"testing"

	"github.com/kubeflow/trainer/v2/pkg/constants"
)

func TestGenerateRange(t *testing.T) {
	cases := []struct {
		name  string
		size  int32
		start int32
		want  string
	}{
		{
			name:  "single node",
			size:  1,
			start: 0,
			want:  "0",
		},
		{
			name:  "multiple nodes starting at zero",
			size:  4,
			start: 0,
			want:  "0-3",
		},
		{
			name:  "multiple nodes with offset start",
			size:  3,
			start: 10,
			want:  "10-12",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateRange(tc.size, tc.start)
			if got != tc.want {
				t.Errorf("generateRange(%d, %d) = %q; want %q", tc.size, tc.start, got, tc.want)
			}
		})
	}
}

func TestGenerateHostlist(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		size   int32
		want   string
	}{
		{
			name:   "prefix with one node",
			prefix: "lammps-job",
			size:   1,
			want:   fmt.Sprintf("lammps-job-%s-0-[0]", constants.Node),
		},
		{
			name:   "prefix with four nodes",
			prefix: "flux-cluster",
			size:   4,
			want:   fmt.Sprintf("flux-cluster-%s-0-[0-3]", constants.Node),
		},
		{
			name:   "empty prefix handled",
			prefix: "",
			size:   2,
			want:   fmt.Sprintf("-%s-0-[0-1]", constants.Node),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateHostlist(tc.prefix, tc.size)
			if got != tc.want {
				t.Errorf("generateHostlist(%q, %d) = %q; want %q", tc.prefix, tc.size, got, tc.want)
			}
		})
	}
}
