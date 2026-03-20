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

package trainingruntime

import (
	"testing"

	"github.com/kubeflow/trainer/v2/pkg/constants"
)

func TestIsSupportDeprecated(t *testing.T) {
	cases := map[string]struct {
		labels map[string]string
		want   bool
	}{
		"nil labels returns false": {
			labels: nil,
			want:   false,
		},
		"empty labels returns false": {
			labels: map[string]string{},
			want:   false,
		},
		"label key absent returns false": {
			labels: map[string]string{
				"some-other-label": "value",
			},
			want: false,
		},
		"label key present but value is not deprecated returns false": {
			labels: map[string]string{
				constants.LabelSupport: "supported",
			},
			want: false,
		},
		"label key present but value is empty returns false": {
			labels: map[string]string{
				constants.LabelSupport: "",
			},
			want: false,
		},
		"label key present with deprecated value returns true": {
			labels: map[string]string{
				constants.LabelSupport: constants.SupportDeprecated,
			},
			want: true,
		},
		"deprecated label alongside other labels returns true": {
			labels: map[string]string{
				"app":                  "kubeflow",
				constants.LabelSupport: constants.SupportDeprecated,
				"version":              "v2",
			},
			want: true,
		},
		"partial match of deprecated value returns false": {
			labels: map[string]string{
				constants.LabelSupport: constants.SupportDeprecated + "-extra",
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsSupportDeprecated(tc.labels)
			if got != tc.want {
				t.Errorf("IsSupportDeprecated(%v) = %v, want %v", tc.labels, got, tc.want)
			}
		})
	}
}